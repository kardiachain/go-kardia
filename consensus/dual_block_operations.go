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

package consensus

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/dual"
	dualbc "github.com/kardiachain/go-kardia/dual/blockchain"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

var (
	ErrNilDualBlockChainManager = errors.New("DualBlockChainManager isn't set yet")
)

// TODO(thientn/namdoh): this is similar to execution.go & validation.go in state/
// These files should be consolidated in the future.
type DualBlockOperations struct {
	logger log.Logger

	mtx sync.RWMutex

	blockchain *dualbc.DualBlockChain
	eventPool  *dualbc.EventPool

	bcManager *dual.DualBlockChainManager

	height uint64
}

// Returns a new DualBlockOperations with latest chain & ,
// initialized to the last height that was committed to the DB.
func NewDualBlockOperations(logger log.Logger, blockchain *dualbc.DualBlockChain, eventPool *dualbc.EventPool) *DualBlockOperations {
	return &DualBlockOperations{
		logger:     logger,
		blockchain: blockchain,
		eventPool:  eventPool,
		height:     blockchain.CurrentHeader().Height,
	}
}

func (dbo *DualBlockOperations) SetDualBlockChainManager(bcManager *dual.DualBlockChainManager) {
	dbo.bcManager = bcManager
}

func (dbo *DualBlockOperations) Height() uint64 {
	return dbo.height
}

// Proposes a new block for dual's blockchain.
func (dbo *DualBlockOperations) CreateProposalBlock(height int64, lastBlockID types.BlockID, lastValidatorHash common.Hash, commit *types.Commit) (block *types.Block) {
	// Gets all dual's events in pending pools and them to the new block.
	// TODO(namdoh@): Since there may be a small latency for other dual peers to see the same set of
	// dual's events, we may need to wait a bit here.
	events := dbo.collectDualEvents()
	dbo.logger.Debug("Collected dual's events", "events", events)

	header := dbo.newHeader(height, uint64(len(events)), lastBlockID, lastValidatorHash)
	dbo.logger.Info("Creates new header", "header", header)

	if height > 0 {
		previousBlock := dbo.blockchain.GetBlockByHeight(uint64(height) - 1)
		if previousBlock == nil {
			dbo.logger.Error("Get previous block N-1 failed", "proposedHeight", height)
			return nil
		}
		// TODO(#169,namdoh): Break this propose step into two passes--first is to propose
		// pending DualEvents, second is to propose submission receipts of N-1 DualEvent-derived Txs
		// to other blockchains.
		dbo.logger.Debug("Submitting dual's events from N-1", "events", previousBlock.DualEvents())
		stateRoot, err := dbo.submitDualEvents(previousBlock.DualEvents())
		if err != nil {
			dbo.logger.Error("Fail to submit dual events", "err", err)
			return nil
		}
		header.Root = stateRoot
	}

	block = dbo.newBlock(header, events, commit)
	dbo.logger.Trace("Make block to propose", "block", block)

	dbo.logger.Error("Not yet implement -- save the receipt of dual's event submission to other blockchain to the proposed block")

	return block
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// Executes and commits the transactions in the given block.
// Transactions & receipts are saved to storage.
// This also validate the new state root against the block root.
func (dbo *DualBlockOperations) CommitAndValidateBlockTxs(block *types.Block) error {
	return nil
}

// Persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (dbo *DualBlockOperations) SaveBlock(block *types.Block, seenCommit *types.Commit) {
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

	// Save block commit (duplicate and separate from the Block)
	dbo.blockchain.WriteCommit(height-1, block.LastCommit())

	// (@kiendn, issue#73)Use this function to prevent nil commits
	seenCommit.MakeNilEmpty()

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	dbo.blockchain.WriteCommit(height, seenCommit)

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
func (dbo *DualBlockOperations) newHeader(height int64, numEvents uint64, blockId types.BlockID, validatorsHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		NumDualEvents:  numEvents,
		Time:           big.NewInt(time.Now().Unix()),
		LastBlockID:    blockId,
		ValidatorsHash: validatorsHash,
	}
}

// Creates new block from given data.
func (dbo *DualBlockOperations) newBlock(header *types.Header, events types.DualEvents, commit *types.Commit) *types.Block {
	return types.NewDualBlock(dbo.logger, header, events, commit)
}

// Queries list of pending dual's events from EventPool.
func (dbo *DualBlockOperations) collectDualEvents() []*types.DualEvent {
	pending, err := dbo.eventPool.Pending()
	if err != nil {
		dbo.logger.Error("Fail to get pending events", "err", err)
		return nil
	}
	return pending
}

// Submits txs derived from a dual events list to other blockchain.
func (dbo *DualBlockOperations) submitDualEvents(events types.DualEvents) (common.Hash, error) {
	if len(events) == 0 {
		return common.Hash{}, nil
	}

	if dbo.bcManager == nil {
		dbo.logger.Error("DualBlockChainManager isn't set yet.")
		return common.Hash{}, ErrNilDualBlockChainManager
	}

	for _, event := range events {
		err := dbo.bcManager.SubmitTx(event.TriggeredEvent)
		if err != nil {
			return common.Hash{}, err
		}
		// TODO(namdoh): Properly handle error here.
	}
	dbo.logger.Error("Not yet implemented - getting submit DualEvent receipt")
	return common.Hash{}, nil
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// saveReceipts saves receipts of block transactions to storage.
func (dbo *DualBlockOperations) saveReceipts(receipts types.Receipts, block *types.Block) {
	dbo.logger.Error("Not yet implement DualBlockOperations.submitDualEvents()")
}
