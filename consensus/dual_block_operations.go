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
	"math/big"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/blockchain/dual"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// TODO(thientn/namdoh): this is similar to execution.go & validation.go in state/
// These files should be consolidated in the future.

type DualBlockOperations struct {
	logger log.Logger

	mtx sync.RWMutex

	blockchain *dual.DualBlockChain
	height     uint64
}

// NewBlockOperations returns a new BlockOperations with latest chain & ,
// initialized to the last height that was committed to the DB.
func NewDualBlockOperations(logger log.Logger, blockchain *dual.DualBlockChain) *DualBlockOperations {
	return &DualBlockOperations{
		logger:     logger,
		blockchain: blockchain,
		height:     blockchain.CurrentHeader().Height,
	}
}

func (dbo *DualBlockOperations) Height() uint64 {
	return dbo.height
}

// newHeader creates new block header from given data.
// Some header fields are not ready at this point.
func (dbo *DualBlockOperations) newHeader(height int64, numTxs uint64, blockId types.BlockID, validatorsHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		Time:           big.NewInt(time.Now().Unix()),
		LastBlockID:    blockId,
		ValidatorsHash: validatorsHash,
	}
}

// newBlock creates new block from given data.
func (dbo *DualBlockOperations) newBlock(header *types.Header, txs []*types.Transaction, receipts types.Receipts, commit *types.Commit) *types.Block {
	block := types.NewDualBlock(dbo.logger, header, commit)

	// TODO(namdoh): Fill the missing header info: AppHash, ConsensusHash,
	// LastResultHash.

	return block
}

// Proposes a new block.
func (dbo *DualBlockOperations) CreateProposalBlock(height int64, lastBlockID types.BlockID, lastValidatorHash common.Hash, commit *types.Commit) (block *types.Block) {
	// Gets all transactions in pending pools and execute them to get new account states.
	// Tx execution can happen in parallel with voting or precommitted.
	// For simplicity, this code executes & commits txs before sending proposal,
	// so statedb of proposal node already contains the new state and txs receipts of this proposal block.
	txs := dbo.collectTransactions()
	dbo.logger.Debug("Collected transactions", "txs", txs)

	header := dbo.newHeader(height, uint64(len(txs)), lastBlockID, lastValidatorHash)
	dbo.logger.Info("Creates new header", "header", header)

	stateRoot, receipts, err := dbo.commitTransactions(txs, header)
	if err != nil {
		dbo.logger.Error("Fail to commit transactions", "err", err)
		return nil
	}
	header.Root = stateRoot

	block = dbo.newBlock(header, txs, receipts, commit)
	dbo.logger.Trace("Make block to propose", "block", block)

	dbo.saveReceipts(receipts, block)

	return block
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (dbo *DualBlockOperations) SaveBlock(block *types.Block, seenCommit *types.Commit) {
	if block == nil {
		common.PanicSanity("BlockOperations try to save a nil block")
	}
	height := block.Height()
	if g, w := height, dbo.Height()+1; g != w {
		common.PanicSanity(common.Fmt("BlockOperations can only save contiguous blocks. Wanted %v, got %v", w, g))
	}

	// Save block
	if height != dbo.Height()+1 {
		common.PanicSanity(common.Fmt("BlockOperations can only save contiguous blocks. Wanted %v, got %v", dbo.Height()+1, height))
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

	dbo.mtx.Lock()
	dbo.height = height
	dbo.mtx.Unlock()
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// collectTransactions queries list of pending transactions from tx pool.
func (dbo *DualBlockOperations) collectTransactions() []*types.Transaction {
	return []*types.Transaction{}
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (dbo *DualBlockOperations) LoadBlock(height uint64) *types.Block {
	return dbo.blockchain.GetBlockByHeight(height)
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (dbo *DualBlockOperations) LoadBlockCommit(height uint64) *types.Commit {
	block := dbo.blockchain.GetBlockByHeight(height + 1)
	if block == nil {
		return nil
	}

	return block.LastCommit()
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (dbo *DualBlockOperations) LoadSeenCommit(height uint64) *types.Commit {
	commit := dbo.blockchain.ReadCommit(height)
	if commit == nil {
		dbo.logger.Error("LoadSeenCommit return nothing", "height", height)
	}

	return commit
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// LoadBlock returns the Block for the given height.
// commitTransactions executes the given transactions and commits the result stateDB to disk.
func (dbo *DualBlockOperations) commitTransactions(txs types.Transactions, header *types.Header) (common.Hash, types.Receipts, error) {
	return common.Hash{}, types.Receipts{}, nil
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// saveReceipts saves receipts of block transactions to storage.
func (dbo *DualBlockOperations) saveReceipts(receipts types.Receipts, block *types.Block) {
}

// TODO(namdoh@): This isn't needed. Figure out how to remove this.
// CommitAndValidateBlockTxs executes and commits the transactions in the given block.
// Transactions & receipts are saved to storage.
// This also validate the new state root against the block root.
func (dbo *DualBlockOperations) CommitAndValidateBlockTxs(block *types.Block) error {
	return nil
}
