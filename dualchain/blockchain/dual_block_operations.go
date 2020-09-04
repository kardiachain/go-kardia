/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package blockchain

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

var (
	ErrNilDualBlockChainManager = errors.New("DualBlockChainManager isn't set yet")
)

// TODO(thientn/namdoh): this is similar to execution.go & validation.go in state/
// These files should be consolidated in the future.
type DualBlockOperations struct {
	logger log.Logger

	mtx sync.RWMutex

	blockchain *DualBlockChain
	eventPool  *event_pool.Pool

	bcManager *DualBlockChainManager

	height uint64
}

// Returns a new DualBlockOperations with latest chain & ,
// initialized to the last height that was committed to the DB.
func NewDualBlockOperations(logger log.Logger, blockchain *DualBlockChain, eventPool *event_pool.Pool) *DualBlockOperations {
	return &DualBlockOperations{
		logger:     logger,
		blockchain: blockchain,
		eventPool:  eventPool,
		height:     blockchain.CurrentHeader().Height,
	}
}

func (dbo *DualBlockOperations) SetDualBlockChainManager(bcManager *DualBlockChainManager) {
	dbo.bcManager = bcManager
}

func (dbo *DualBlockOperations) Height() uint64 {
	return dbo.height
}

// Proposes a new block for dual's blockchain.
func (dbo *DualBlockOperations) CreateProposalBlock(height int64, lastState cstate.LastestBlockState, proposerAddr common.Address, commit *types.Commit) (block *types.Block, blockParts *types.PartSet) {
	// Gets all dual's events in pending pools and them to the new block.
	// TODO(namdoh@): Since there may be a small latency for other dual peers to see the same set of
	// dual's events, we may need to wait a bit here.
	events := dbo.collectDualEvents()
	dbo.logger.Info("Collected dual's events", "events", events)

	header := dbo.newHeader(height, uint64(len(events)), lastState.LastBlockID, proposerAddr, lastState.LastValidators.Hash())
	dbo.logger.Info("Creates new header", "header", header)

	stateRoot, err := dbo.commitDualEvents(events)
	if err != nil {
		dbo.logger.Error("Fail to commit dual's events", "err", err)
		return nil, nil
	}

	header.Root = stateRoot

	if height > 0 {
		previousBlock := dbo.blockchain.GetBlockByHeight(uint64(height) - 1)
		if previousBlock == nil {
			dbo.logger.Error("Get previous block N-1 failed", "proposedHeight", height)
			return nil, nil
		}
		// TODO(#169,namdoh): Break this propose step into two passes--first is to propose
		//  pending DualEvents, second is to propose submission receipts of N-1 DualEvent-derived Txs
		//  to other blockchains.
		dbo.logger.Debug("Submitting dual events from N-1", "events", previousBlock.DualEvents())
		if err := dbo.submitDualEvents(previousBlock.DualEvents()); err != nil {
			dbo.logger.Error("Fail to submit dual events", "err", err)
			return nil, nil
		}
		dbo.logger.Info("Not yet implemented - Update state root with the DualEvent's submission receipt")
	}

	block = dbo.newBlock(header, events, commit)
	dbo.logger.Trace("Make block to propose", "block", block)

	return block, block.MakePartSet(types.BlockPartSizeBytes)
}

// Executes and commits the new state from events in the given block.
// This also validate the new state root against the block root.
func (dbo *DualBlockOperations) CommitAndValidateBlockTxs(block *types.Block) error {
	root, err := dbo.commitDualEvents(block.DualEvents())
	if err != nil {
		return err
	}
	if root != block.Root() {
		return fmt.Errorf("different new dualchain state root: Block root: %s, Execution result: %s", block.Root().Hex(), root.Hex())
	}
	return nil
}

// CommitBlockTxsIfNotFound executes and commits block txs if the block state root is not found in storage.
// Proposer and validators should already commit the block txs, so this function prevents double tx execution.
func (dbo *DualBlockOperations) CommitBlockTxsIfNotFound(block *types.Block) error {
	if !dbo.blockchain.CheckCommittedStateRoot(block.Root()) {
		dbo.logger.Trace("Block has unseen state root, execute & commit block txs", "height", block.Height())
		return dbo.CommitAndValidateBlockTxs(block)
	}

	return nil
}

// Persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (dbo *DualBlockOperations) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	if block == nil {
		common.PanicSanity("DualBlockOperations try to save a nil block")
	}
	height := block.Height()
	if g, w := height, dbo.Height()+1; g != w {
		common.PanicSanity(common.Fmt("DualBlockOperations can only save contiguous blocks. Wanted %v, got %v", w, g))
	}

	// Save block
	if height != dbo.Height()+1 {
		common.PanicSanity(common.Fmt("DualBlockOperations can only save contiguous blocks. Wanted %v, got %v", dbo.Height()+1, height))
	}

	// TODO(kiendn): WriteBlockWithoutState returns an error, write logic check if error appears
	if err := dbo.blockchain.WriteBlockWithoutState(block); err != nil {
		common.PanicSanity(common.Fmt("WriteBlockWithoutState fails with error %v", err))
	}

	dbo.blockchain.WriteBlock(block, blockParts, seenCommit)

	dbo.logger.Trace("After commited to blockchain, removing these DualEvent's", "events", block.DualEvents())
	dbo.eventPool.RemoveEvents(block.DualEvents())

	dbo.mtx.Lock()
	dbo.height = height

	dbo.mtx.Unlock()
}

// Returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (dbo *DualBlockOperations) LoadBlock(height uint64) *types.Block {
	return dbo.blockchain.GetBlockByHeight(height)
}

// Return blockpart by given height and part's index
func (bo *DualBlockOperations) LoadBlockPart(height uint64, index int) *types.Part {
	return bo.blockchain.LoadBlockPart(height, index)
}

func (bo *DualBlockOperations) LoadBlockMeta(height uint64) *types.BlockMeta {
	return bo.blockchain.LoadBlockMeta(height)
}

// Returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (dbo *DualBlockOperations) LoadBlockCommit(height uint64) *types.Commit {
	block := dbo.blockchain.GetBlockByHeight(height + 1)
	if block == nil {
		return nil
	}

	return block.LastCommit()
}

// Returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (dbo *DualBlockOperations) LoadSeenCommit(height uint64) *types.Commit {
	commit := dbo.blockchain.ReadCommit(height)
	if commit == nil {
		dbo.logger.Error("LoadSeenCommit return nothing", "height", height)
	}

	return commit
}

// Creates new block header from given data.
// Some header fields are not ready at this point.
func (dbo *DualBlockOperations) newHeader(height int64, numEvents uint64, blockId types.BlockID, validator common.Address, validatorsHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		NumDualEvents:  numEvents,
		Time:           big.NewInt(time.Now().Unix()),
		LastBlockID:    blockId,
		Validator:      validator,
		ValidatorsHash: validatorsHash,
	}
}

// Creates new block from given data.
func (dbo *DualBlockOperations) newBlock(header *types.Header, events types.DualEvents, commit *types.Commit) *types.Block {
	return types.NewDualBlock(header, events, commit)
}

// Queries list of pending dual's events from EventPool.
func (dbo *DualBlockOperations) collectDualEvents() []*types.DualEvent {
	return dbo.eventPool.ProposeEvents()
}

// Submits txs derived from a dual events list to other blockchain.
func (dbo *DualBlockOperations) submitDualEvents(events types.DualEvents) error {
	if len(events) == 0 {
		return nil
	}
	if dbo.bcManager == nil {
		dbo.logger.Error("DualBlockChainManager isn't set yet.")
		return ErrNilDualBlockChainManager
	}
	dbo.logger.Debug("submitting dual event", "events", len(events))
	for _, event := range events {
		sender, err := types.EventSender(event)
		if err != nil {
			dbo.logger.Error("error while getting sender from event", "err", err, "event", event.Hash().Hex())
			continue
		}
		dbo.logger.Debug("processing event",
			"hash", event.Hash().Hex(),
			"sender", sender.Hash().Hex(),
			"txSource", event.TriggeredEvent.TxSource,
			"txHash", event.TriggeredEvent.TxHash.Hex(),
		)

		if len(event.KardiaSmcs) != 0 {
			dbo.bcManager.HandleKardiaSmcs(event.KardiaSmcs)
			continue
		}

		if err := dbo.bcManager.SubmitTx(event.TriggeredEvent); err != nil {
			// TODO(sontranrad, namdoh): add logic for handling error when submitting TX, currrently just log error here
			dbo.logger.Error("Error submit dual event", "err", err)
		} else {
			dbo.logger.Info("Submit dual event successfully",
				"sender", sender.Hex(), "txSource", event.TriggeredEvent.TxSource,
				"txHash", event.TriggeredEvent.TxHash.Hex(),
				"eventHash", event.Hash().Hex(),
			)
		}

		// TODO(namdoh): Properly handle error here.
	}
	dbo.logger.Info("Not yet implemented - getting submit DualEvent receipt")
	return nil
}

// Commit dual's events result stateDB to disk.
func (dbo *DualBlockOperations) commitDualEvents(events types.DualEvents) (common.Hash, error) {
	// Blockchain state at head block.
	state, err := dbo.blockchain.State()
	if err != nil {
		dbo.logger.Error("Fail to get blockchain head state", "err", err)
		return common.Hash{}, err
	}

	counter := 0
	for _, event := range events {
		state.Prepare(event.Hash(), common.Hash{}, counter)
		state.Finalise(true)
		counter++
	}
	root, err := state.Commit(true)
	if err != nil {
		dbo.logger.Error("Fail to commit new statedb", "err", err)
		return common.Hash{}, err
	}
	err = dbo.blockchain.CommitTrie(root)
	if err != nil {
		dbo.logger.Error("Fail to write statedb trie to disk", "err", err)
		return common.Hash{}, err
	}

	return root, nil
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// saveReceipts saves receipts of block transactions to storage.
func (dbo *DualBlockOperations) saveReceipts(receipts types.Receipts, block *types.Block) {
	dbo.logger.Info("Not yet implement DualBlockOperations.submitDualEvents()")
}
