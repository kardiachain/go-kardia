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
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/vm"
)

// TODO(thientn/namdoh): this is similar to execution.go & validation.go in state/
// These files should be consolidated in the future.

type BlockOperations struct {
	logger log.Logger

	mtx sync.RWMutex

	blockchain *blockchain.BlockChain
	txPool     *blockchain.TxPool
	height     uint64
}

// NewBlockOperations returns a new BlockOperations with latest chain & ,
// initialized to the last height that was committed to the DB.
func NewBlockOperations(logger log.Logger, blockchain *blockchain.BlockChain, txPool *blockchain.TxPool) *BlockOperations {
	return &BlockOperations{
		logger:     logger,
		blockchain: blockchain,
		txPool:     txPool,
		height:     blockchain.CurrentHeader().Height,
	}
}

func (bo *BlockOperations) Height() uint64 {
	return bo.height
}

// newHeader creates new block header from given data.
// Some header fields are not ready at this point.
func (bo *BlockOperations) newHeader(height int64, numTxs uint64, blockId types.BlockID, validatorsHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		Time:           big.NewInt(time.Now().Unix()),
		NumTxs:         numTxs,
		LastBlockID:    blockId,
		ValidatorsHash: validatorsHash,
		GasLimit:       10000000,
	}
}

// NewBlock creates new block from given data.
func (bo *BlockOperations) newBlock(header *types.Header, txs []*types.Transaction, receipts types.Receipts, commit *types.Commit) *types.Block {
	block := types.NewBlock(bo.logger, header, txs, receipts, commit)

	// TODO(namdoh): Fill the missing header info: AppHash, ConsensusHash,
	// LastResultHash.

	return block
}

// Proposal a new block.
func (bo *BlockOperations) CreateProposalBlock(height int64, lastBlockID types.BlockID, lastValidatorHash common.Hash, commit *types.Commit) (block *types.Block) {
	// Gets all transactions in pending pools and execute them to get new account states.
	// Tx execution can happen in parallel with voting or precommitted.
	// For simplicity, this code executes & commits txs before sending proposal,
	// so statedb of proposal node already contains the new state and txs receipts of this proposal block.
	txs := bo.collectTransactions()
	bo.logger.Debug("Collected transactions", "txs", txs)

	header := bo.newHeader(height, uint64(len(txs)), lastBlockID, lastValidatorHash)
	bo.logger.Info("Creates new header", "header", header)

	stateRoot, receipts, err := bo.commitTransactions(txs, header)
	if err != nil {
		bo.logger.Error("Fail to commit transactions", "err", err)
		return nil
	}
	header.Root = stateRoot

	block = bo.newBlock(header, txs, receipts, commit)
	bo.logger.Trace("Make block to propose", "block", block)

	bo.saveReceipts(receipts, block)

	return block
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bo *BlockOperations) SaveBlock(block *types.Block, seenCommit *types.Commit) {
	if block == nil {
		common.PanicSanity("BlockOperations try to save a nil block")
	}
	height := block.Height()
	if g, w := height, bo.Height()+1; g != w {
		common.PanicSanity(common.Fmt("BlockOperations can only save contiguous blocks. Wanted %v, got %v", w, g))
	}

	// Save block
	if height != bo.Height()+1 {
		common.PanicSanity(common.Fmt("BlockOperations can only save contiguous blocks. Wanted %v, got %v", bo.Height()+1, height))
	}

	// TODO(kiendn): WriteBlockWithoutState returns an error, write logic check if error appears
	if err := bo.blockchain.WriteBlockWithoutState(block); err != nil {
		common.PanicSanity(common.Fmt("WriteBlockWithoutState fails with error %v", err))
	}

	// Save block commit (duplicate and separate from the Block)
	bo.blockchain.WriteCommit(height-1, block.LastCommit())

	// (@kiendn, issue#73)Use this function to prevent nil commits
	seenCommit.MakeNilEmpty()

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	bo.blockchain.WriteCommit(height, seenCommit)

	// TODO(thientn/kiendn): Evaluates remove txs directly here, or depending on txPool.reset() when receiving new block event.
	bo.txPool.RemoveTxs(block.Transactions())

	bo.mtx.Lock()
	bo.height = height
	bo.mtx.Unlock()
}

// collectTransactions queries list of pending transactions from tx pool.
func (bo *BlockOperations) collectTransactions() []*types.Transaction {
	pending, err := bo.txPool.Pending()
	if err != nil {
		bo.logger.Error("Fail to get pending txs", "err", err)
		return nil
	}

	// TODO: do basic verification & check with gas & sort by nonce
	// check code NewTransactionsByPriceAndNonce
	pendingTxs := make([]*types.Transaction, 0)
	for _, txs := range pending {
		for _, tx := range txs {
			pendingTxs = append(pendingTxs, tx)
		}
	}
	return pendingTxs
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (bo *BlockOperations) LoadBlock(height uint64) *types.Block {
	return bo.blockchain.GetBlockByHeight(height)
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (bo *BlockOperations) LoadBlockCommit(height uint64) *types.Commit {
	block := bo.blockchain.GetBlockByHeight(height + 1)
	if block == nil {
		return nil
	}

	return block.LastCommit()
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bo *BlockOperations) LoadSeenCommit(height uint64) *types.Commit {
	commit := bo.blockchain.ReadCommit(height)
	if commit == nil {
		bo.logger.Error("LoadSeenCommit return nothing", "height", height)
	}

	return commit
}

// commitTransactions executes the given transactions and commits the result stateDB to disk.
func (bo *BlockOperations) commitTransactions(txs types.Transactions, header *types.Header) (common.Hash, types.Receipts, error) {
	var (
		receipts = types.Receipts{}
		usedGas  = new(uint64)
	)
	counter := 0

	// Blockchain state at head block.
	state, err := bo.blockchain.State()
	if err != nil {
		bo.logger.Error("Fail to get blockchain head state", "err", err)
		return common.Hash{}, nil, err
	}

	// GasPool
	bo.logger.Info("header gas limit", "limit", header.GasLimit)
	gasPool := new(blockchain.GasPool).AddGas(header.GasLimit)

	// TODO(thientn): verifies the list is sorted by nonce so tx with lower nonce is execute first.
	for _, tx := range txs {
		state.Prepare(tx.Hash(), common.Hash{}, counter)
		snap := state.Snapshot()
		// TODO(thientn): confirms nil coinbase is acceptable.
		receipt, _, err := blockchain.ApplyTransaction(bo.logger, bo.blockchain, gasPool, state, header, tx, usedGas, vm.Config{})
		if err != nil {
			state.RevertToSnapshot(snap)
			// TODO(thientn): check error type and jump to next tx if possible
			return common.Hash{}, nil, err
		}
		counter++
		receipts = append(receipts, receipt)
	}
	root, err := state.Commit(true)

	if err != nil {
		bo.logger.Error("Fail to commit new statedb after txs", "err", err)
		return common.Hash{}, nil, err
	}
	err = bo.blockchain.CommitTrie(root)
	if err != nil {
		bo.logger.Error("Fail to write statedb trie to disk", "err", err)
		return common.Hash{}, nil, err
	}

	return root, receipts, nil
}

// saveReceipts saves receipts of block transactions to storage.
func (bo *BlockOperations) saveReceipts(receipts types.Receipts, block *types.Block) {
	bo.blockchain.WriteReceipts(receipts, block)
}

// CommitAndValidateBlockTxs executes and commits the transactions in the given block.
// Transactions & receipts are saved to storage.
// This also validate the new state root against the block root.
func (bo *BlockOperations) CommitAndValidateBlockTxs(block *types.Block) error {
	root, receipts, err := bo.commitTransactions(block.Transactions(), block.Header())
	if err != nil {
		return err
	}
	if root != block.Root() {
		return fmt.Errorf("different new state root: Block root: %s, Execution result: %s", block.Root().Hex(), root.Hex())
	}
	receiptsHash := types.DeriveSha(receipts)
	if receiptsHash != block.ReceiptHash() {
		return fmt.Errorf("different receipt hash: Block receipt: %s, receipt from execution: %s", block.ReceiptHash().Hex(), receiptsHash.Hex())
	}
	bo.saveReceipts(receipts, block)

	return nil
}
