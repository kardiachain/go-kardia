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
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	stypes "github.com/kardiachain/go-kardia/mainchain/staking/types"

	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

//-----------------------------------------------------------------------------
// evidence pool

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
// Get/Set/Commit
type EvidencePool interface {
	PendingEvidence(int64) ([]types.Evidence, int64)
}

// BlockOperations
type BlockOperations struct {
	logger log.Logger

	mtx sync.RWMutex

	blockchain *BlockChain
	txPool     *tx_pool.TxPool
	evPool     EvidencePool
	base       uint64
	height     uint64
	staking    *staking.StakingSmcUtil
}

// NewBlockOperations returns a new BlockOperations with reference to the latest state of blockchain.
func NewBlockOperations(logger log.Logger, blockchain *BlockChain, txPool *tx_pool.TxPool, evpool EvidencePool, staking *staking.StakingSmcUtil) *BlockOperations {
	return &BlockOperations{
		logger:     logger,
		blockchain: blockchain,
		txPool:     txPool,
		height:     blockchain.CurrentBlock().Height(),
		evPool:     evpool,
		staking:    staking,
	}
}

// Base returns the first known contiguous block height, or 0 for empty block stores.
func (bo *BlockOperations) Base() uint64 {
	bo.mtx.RLock()
	defer bo.mtx.RUnlock()
	return bo.base
}

// Height returns latest height of blockchain.
func (bo *BlockOperations) Height() uint64 {
	bo.mtx.RLock()
	defer bo.mtx.RUnlock()
	return bo.height
}

// CreateProposalBlock creates a new proposal block with all current pending txs in pool.
func (bo *BlockOperations) CreateProposalBlock(
	height uint64, lastState cstate.LastestBlockState,
	proposerAddr common.Address, commit *types.Commit) (block *types.Block, blockParts *types.PartSet) {
	// Gets all transactions in pending pools and execute them to get new account states.
	// Tx execution can happen in parallel with voting or precommitted.
	// For simplicity, this code executes & commits txs before sending proposal,
	// so statedb of proposal node already contains the new state and txs receipts of this proposal block.
	//maxBytes := lastState.ConsensusParams.Block.MaxBytes
	// Fetch a limited amount of valid evidence
	maxNumEvidence, _ := types.MaxEvidencePerBlock(lastState.ConsensusParams.Evidence.MaxBytes)
	evidence, _ := bo.evPool.PendingEvidence(maxNumEvidence)

	txs := bo.txPool.ProposeTransactions()
	bo.logger.Debug("Collected transactions", "txs count", len(txs))

	// Set time.
	var timestamp time.Time
	if height == 1 {
		timestamp = lastState.LastBlockTime // genesis time
	} else {
		timestamp = cstate.MedianTime(commit, lastState.LastValidators)
	}

	header := bo.newHeader(timestamp, height, uint64(len(txs)), lastState.LastBlockID, proposerAddr, lastState.Validators.Hash(),
		lastState.NextValidators.Hash(), lastState.AppHash)
	header.GasLimit = lastState.ConsensusParams.Block.MaxGas
	bo.logger.Info("Creates new header", "header", header)

	block = bo.newBlock(header, txs, commit, evidence)
	bo.logger.Trace("Make block to propose", "block", block)
	return block, block.MakePartSet(types.BlockPartSizeBytes)
}

// CommitAndValidateBlockTxs executes and commits the transactions in the given block.
// New calculated state root is validated against the root field in block.
// Transactions, new state and receipts are saved to storage.
func (bo *BlockOperations) CommitAndValidateBlockTxs(block *types.Block, lastCommit stypes.LastCommitInfo, byzVals []stypes.Evidence) ([]*types.Validator, common.Hash, error) {
	vals, root, blockInfo, _, err := bo.commitTransactions(block.Transactions(), block.Header(), lastCommit, byzVals)
	if err != nil {
		return nil, common.Hash{}, err
	}
	bo.saveBlockInfo(blockInfo, block)
	bo.blockchain.DB().WriteHeadBlockHash(block.Hash())
	bo.blockchain.DB().WriteTxLookupEntries(block)
	bo.blockchain.DB().WriteAppHash(block.Height(), root)
	bo.blockchain.InsertHeadBlock(block)

	return vals, root, nil
}

// CommitBlockTxsIfNotFound executes and commits block txs if the block state root is not found in storage.
// Proposer and validators should already commit the block txs, so this function prevents double tx execution.
func (bo *BlockOperations) CommitBlockTxsIfNotFound(block *types.Block, lastCommit stypes.LastCommitInfo, byzVals []stypes.Evidence) ([]*types.Validator, common.Hash, error) {
	root := bo.blockchain.DB().ReadAppHash(block.Height())
	if !bo.blockchain.CheckCommittedStateRoot(root) {
		bo.logger.Trace("Block has unseen state root, execute & commit block txs", "height", block.Height())
		return bo.CommitAndValidateBlockTxs(block, lastCommit, byzVals)
	}

	return nil, common.Hash{}, nil
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
	if !blockParts.IsComplete() {
		panic("BlockOperations can only save complete block part sets")
	}
	bo.blockchain.SaveBlock(block, blockParts, seenCommit)

	bo.mtx.Lock()
	bo.height = height
	bo.mtx.Unlock()
}

// LoadBlock returns the Block for the given height.
// If no block is found for the given height, it returns nil.
func (bo *BlockOperations) LoadBlock(height uint64) *types.Block {
	return bo.blockchain.GetBlockByHeight(height)
}

// LoadBlockPart load block part
func (bo *BlockOperations) LoadBlockPart(height uint64, index int) *types.Part {
	return bo.blockchain.LoadBlockPart(height, index)
}

// LoadBlockMeta load block meta
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
func (bo *BlockOperations) newHeader(time time.Time, height uint64, numTxs uint64, blockID types.BlockID,
	proposer common.Address, validatorsHash common.Hash, nextValidatorHash common.Hash, appHash common.Hash) *types.Header {
	return &types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:             height,
		Time:               time,
		NumTxs:             numTxs,
		LastBlockID:        blockID,
		ProposerAddress:    proposer,
		ValidatorsHash:     validatorsHash,
		NextValidatorsHash: nextValidatorHash,
		AppHash:            appHash,
	}
}

// newBlock creates new block from given data.
func (bo *BlockOperations) newBlock(header *types.Header, txs []*types.Transaction, commit *types.Commit, ev []types.Evidence) *types.Block {
	block := types.NewBlock(header, txs, commit, ev)
	return block
}

// commitTransactions executes the given transactions and commits the result stateDB to disk.
func (bo *BlockOperations) commitTransactions(txs types.Transactions, header *types.Header,
	lastCommit stypes.LastCommitInfo, byzVals []stypes.Evidence) ([]*types.Validator, common.Hash, *types.BlockInfo,
	types.Transactions, error) {
	var (
		newTxs   = types.Transactions{}
		receipts = types.Receipts{}
		usedGas  = new(uint64)
	)

	// Blockchain state at head block.
	state, err := bo.blockchain.State()
	if err != nil {
		bo.logger.Error("Fail to get blockchain head state", "err", err)
		return nil, common.Hash{}, nil, nil, err
	}

	// GasPool
	bo.logger.Info("header gas limit", "limit", header.GasLimit)
	gasPool := new(types.GasPool).AddGas(header.GasLimit)

	kvmConfig := kvm.Config{}

	blockReward, err := bo.staking.Mint(state, header, bo.blockchain, kvmConfig)
	if err != nil {
		bo.logger.Error("Fail to mint", "err", err)
		return nil, common.Hash{}, nil, nil, err
	}

	if err := bo.staking.FinalizeCommit(state, header, bo.blockchain, kvmConfig, lastCommit); err != nil {
		bo.logger.Error("Fail to finalize commit", "err", err)
		return nil, common.Hash{}, nil, nil, err
	}

	if err := bo.staking.DoubleSign(state, header, bo.blockchain, kvmConfig, byzVals); err != nil {
		bo.logger.Error("Fail to apply double sign", "err", err)
		return nil, common.Hash{}, nil, nil, err
	}

	// TODO(thientn): verifies the list is sorted by nonce so tx with lower nonce is execute first.
LOOP:
	for i, tx := range txs {
		state.Prepare(tx.Hash(), header.Hash(), i)
		snap := state.Snapshot()
		// TODO(thientn): confirms nil coinbase is acceptable.
		receipt, _, err := ApplyTransaction(bo.logger, bo.blockchain, gasPool, state, header, tx, usedGas, kvmConfig)
		if err != nil {
			bo.logger.Error("ApplyTransaction failed", "tx", tx.Hash().Hex(), "nonce", tx.Nonce(), "err", err)
			state.RevertToSnapshot(snap)
			// TODO(thientn): check error type and jump to next tx if possible
			// kiendn: instead of return nil and err, jump to next tx
			//return common.Hash{}, nil, nil, err
			continue LOOP
		}
		i++
		receipts = append(receipts, receipt)
		newTxs = append(newTxs, tx)
		// send logs of emitted events to logs feed for collecting
		if len(receipt.Logs) > 0 {
			bo.blockchain.logsFeed.Send(receipt.Logs)
		}
	}

	vals, err := bo.staking.ApplyAndReturnValidatorSets(state, header, bo.blockchain, kvmConfig)
	if err != nil {
		return nil, common.Hash{}, nil, nil, err
	}

	root, err := state.Commit(true)

	if err != nil {
		bo.logger.Error("Fail to commit new statedb after txs", "err", err)
		return nil, common.Hash{}, nil, nil, err
	}
	err = bo.blockchain.CommitTrie(root)
	if err != nil {
		bo.logger.Error("Fail to write statedb trie to disk", "err", err)
		return nil, common.Hash{}, nil, nil, err
	}

	blockInfo := &types.BlockInfo{
		GasUsed:  *usedGas,
		Receipts: receipts,
		Rewards:  blockReward,
		Bloom:    types.CreateBloom(receipts),
	}

	return vals, root, blockInfo, newTxs, nil
}

// saveReceipts saves receipts of block transactions to storage.
func (bo *BlockOperations) saveBlockInfo(blockInfo *types.BlockInfo, block *types.Block) {
	bo.blockchain.WriteBlockInfo(block, blockInfo)
}
