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
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/kai/state"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

// TODO(thientn/namdoh): this is similar to execution.go & validation.go in state/
// These files should be consolidated in the future.

type BlockOperations struct {
	logger log.Logger

	mtx sync.RWMutex

	blockchain *BlockChain
	txPool     *tx_pool.TxPool
	height     uint64
}

// NewBlockOperations returns a new BlockOperations with reference to the latest state of blockchain.
func NewBlockOperations(logger log.Logger, blockchain *BlockChain, txPool *tx_pool.TxPool) *BlockOperations {
	return &BlockOperations{
		logger:     logger,
		blockchain: blockchain,
		txPool:     txPool,
		height:     blockchain.CurrentBlock().Height(),
	}
}

// Height returns latest height of blockchain.
func (bo *BlockOperations) Height() uint64 {
	return bo.height
}

// CreateProposalBlock creates a new proposal block with all current pending txs in pool.
func (bo *BlockOperations) CreateProposalBlock(
	height int64, lastState state.LastestBlockState,
	proposerAddr common.Address, commit *types.Commit) (block *types.Block, blockParts *types.PartSet) {
	// Gets all transactions in pending pools and execute them to get new account states.
	// Tx execution can happen in parallel with voting or precommitted.
	// For simplicity, this code executes & commits txs before sending proposal,
	// so statedb of proposal node already contains the new state and txs receipts of this proposal block.
	txs := bo.txPool.ProposeTransactions()
	bo.logger.Debug("Collected transactions", "txs count", len(txs))

	header := bo.newHeader(height, uint64(len(txs)), lastState.LastBlockID, proposerAddr, lastState.LastValidators.Hash())
	header.AppHash = lastState.AppHash

	block = bo.newBlock(header, txs, commit)
	bo.logger.Info("Make block to propose", "height", block.Height(), "AppHash", block.AppHash(), "hash", block.Hash())

	// claim reward
	if bo.blockchain.CurrentBlock().Height() > 1 {
		st, err := bo.blockchain.State()
		if err != nil {
			bo.logger.Error("Fail to get blockchain head state", "err", err)
			return nil, nil
		}
		tx, err := kvm.ClaimReward(bo.blockchain, st, bo.txPool)
		if err != nil {
			bo.logger.Error("fail to claim reward", "err", err, "sender", bo.blockchain.Config().BaseAccount.Address.Hex())
			return nil, nil
		}
		if err = bo.txPool.AddTx(tx); err != nil {
			bo.logger.Error("fail to add claim reward transaction", "err", err)
			return nil, nil
		}
	}
	return block, block.MakePartSet(types.BlockPartSizeBytes)
}

// CommitAndValidateBlockTxs executes and commits the transactions in the given block.
// New calculated state root is validated against the root field in block.
// Transactions, new state and receipts are saved to storage.
func (bo *BlockOperations) CommitAndValidateBlockTxs(block *types.Block) (common.Hash, error) {
	root, receipts, _, err := bo.commitTransactions(block.Transactions(), block.Header())
	if err != nil {
		return common.Hash{}, err
	}
	bo.saveReceipts(receipts, block)
	bo.blockchain.WriteAppHash(block.Height(), root)
	return root, nil
}

// SaveBlock saves the given block, blockParts, and seenCommit to the underlying storage.
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bo *BlockOperations) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
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

	if !blockParts.IsComplete() {
		panic(fmt.Sprintf("BlockOperations can only save complete block part sets"))
	}

	// TODO(kiendn): WriteBlockWithoutState returns an error, write logic check if error appears
	if err := bo.blockchain.WriteBlockWithoutState(block, blockParts, seenCommit); err != nil {
		common.PanicSanity(common.Fmt("WriteBlockWithoutState fails with error %v", err))
	}

	bo.mtx.Lock()
	bo.height = height
	bo.mtx.Unlock()
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (bo *BlockOperations) LoadBlock(height uint64) *types.Block {
	return bo.blockchain.GetBlockByHeight(height)
}

func (bo *BlockOperations) LoadBlockPart(height uint64, index int) *types.Part {
	return bo.blockchain.LoadBlockPart(height, index)
}

func (bo *BlockOperations) LoadBlockMeta(height uint64) *types.BlockMeta {
	return bo.blockchain.LoadBlockMeta(height)
}

// LoadBlockCommit returns the Commit for the given height.
// If no block is found for the given height, it returns nil.
func (bo *BlockOperations) LoadBlockCommit(height uint64) *types.Commit {
	return bo.blockchain.LoadBlockCommit(height)
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bo *BlockOperations) LoadSeenCommit(height uint64) *types.Commit {
	commit := bo.blockchain.LoadSeenCommit(height)
	if commit == nil {
		bo.logger.Error("LoadSeenCommit return nothing", "height", height)
	}

	return commit
}

// newHeader creates new block header from given data.
// Some header fields are not ready at this point.
func (bo *BlockOperations) newHeader(height int64, numTxs uint64, blockId types.BlockID, validator common.Address, validatorsHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		Time:           big.NewInt(time.Now().Unix()),
		NumTxs:         numTxs,
		LastBlockID:    blockId,
		Validator:      validator,
		ValidatorsHash: validatorsHash,
		GasLimit:       215040000,
	}
}

// newBlock creates new block from given data.
func (bo *BlockOperations) newBlock(header *types.Header, txs []*types.Transaction, commit *types.Commit) *types.Block {
	block := types.NewBlock(header, txs, commit)

	// TODO(namdoh): Fill the missing header info: AppHash, ConsensusHash,
	// LastResultHash.

	return block
}

// commitTransactions executes the given transactions and commits the result stateDB to disk.
func (bo *BlockOperations) commitTransactions(txs types.Transactions, header *types.Header) (common.Hash, types.Receipts,
	types.Transactions, error) {
	var (
		newTxs   = types.Transactions{}
		receipts = types.Receipts{}
		usedGas  = new(uint64)
	)
	counter := 0

	// Blockchain state at head block.
	state, err := bo.blockchain.State()
	if err != nil {
		bo.logger.Error("Fail to get blockchain head state", "err", err)
		return common.Hash{}, nil, nil, err
	}

	// GasPool
	bo.logger.Info("header gas limit", "limit", header.GasLimit)
	gasPool := new(types.GasPool).AddGas(header.GasLimit)

	// TODO(thientn): verifies the list is sorted by nonce so tx with lower nonce is execute first.
LOOP:
	for _, tx := range txs {
		state.Prepare(tx.Hash(), common.Hash{}, counter)
		snap := state.Snapshot()
		// TODO(thientn): confirms nil coinbase is acceptable.
		receipt, _, err := ApplyTransaction(bo.logger, bo.blockchain, gasPool, state, header, tx, usedGas, kvm.Config{
			IsZeroFee: bo.blockchain.IsZeroFee,
		})
		if err != nil {
			bo.logger.Error("ApplyTransaction failed", "tx", tx.Hash().Hex(), "nonce", tx.Nonce(), "err", err)
			state.RevertToSnapshot(snap)
			// TODO(thientn): check error type and jump to next tx if possible
			// kiendn: instead of return nil and err, jump to next tx
			//return common.Hash{}, nil, nil, err
			continue LOOP
		}
		counter++
		receipts = append(receipts, receipt)
		newTxs = append(newTxs, tx)
	}

	root, err := state.Commit(true)

	if err != nil {
		bo.logger.Error("Fail to commit new statedb after txs", "err", err)
		return common.Hash{}, nil, nil, err
	}
	err = bo.blockchain.CommitTrie(root)
	if err != nil {
		bo.logger.Error("Fail to write statedb trie to disk", "err", err)
		return common.Hash{}, nil, nil, err
	}

	return root, receipts, newTxs, nil
}

// saveReceipts saves receipts of block transactions to storage.
func (bo *BlockOperations) saveReceipts(receipts types.Receipts, block *types.Block) {
	bo.blockchain.WriteReceipts(receipts, block)
}
