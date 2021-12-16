/*
 *  Copyright 2021 KardiaChain
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
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/staking"
	stypes "github.com/kardiachain/go-kardia/mainchain/staking/types"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
)

const (
	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
)

// blockState
type blockState struct {
	logger log.Logger

	signer types.Signer

	state    *state.StateDB // apply state changes here
	tcount   int            // tx count in cycle
	gasPool  *types.GasPool // available gas used to pack transactions
	gasLimit uint64
	usedGas  *uint64

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt
}

// blockConstructor
type blockConstructor struct {
	logger      log.Logger
	chainConfig *configs.ChainConfig

	blockState *blockState
	blockchain *BlockChain
	txPool     *tx_pool.TxPool

	staking *staking.StakingSmcUtil

	// Subscriptions
	mux          *event.TypeMux
	txsCh        chan events.NewTxsEvent
	chainHeadCh  chan events.ChainHeadEvent
	txsSub       event.Subscription
	chainHeadSub event.Subscription

	exitCh chan struct{}
}

// newblockConstructor creates a new block constructor
func newblockConstructor(blockchain *BlockChain, txPool *tx_pool.TxPool, cfg *configs.ChainConfig) *blockConstructor {
	bcs := &blockConstructor{
		logger:      log.New("blockConstructor"),
		chainConfig: cfg,
		blockchain:  blockchain,
		txPool:      txPool,
		txsCh:       make(chan events.NewTxsEvent, txChanSize),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
		exitCh:      make(chan struct{}),
	}

	// Subscribe NewTxsEvent for tx pool
	bcs.txsSub = txPool.SubscribeNewTxsEvent(bcs.txsCh)
	// Subscribe events for blockchain
	bcs.chainHeadSub = blockchain.SubscribeChainHeadEvent(bcs.chainHeadCh)

	go bcs.constructionLoop()
	return bcs
}

// renew the blockchain state
func (bcs *blockConstructor) renew() {
	current := bcs.blockchain.CurrentBlock()
	currentState, _ := bcs.blockchain.State()
	bcs.logger.Info("Txs", "total", len(bcs.blockState.txs))
	bcs.blockState = &blockState{
		signer: types.LatestSigner(bcs.chainConfig),
		state:  currentState.Copy(),
		tcount: 0,
		txs:    []*types.Transaction{},
		header: &types.Header{
			Height:   current.Height() + 1,
			GasLimit: current.GasLimit(),
			Time:     time.Now(),
		},
	}

	// pendingTxs, _ := bcs.txPool.Pending()
	// txs := types.NewTransactionsByPriceAndNonce(bcs.blockState.signer, pendingTxs)
	// bcs.commitTransactions(txs)
}

// constructionLoop is a standalone goroutine to regenerate the sealing task based on the received event.
func (bcs *blockConstructor) constructionLoop() {
	defer bcs.txsSub.Unsubscribe()
	defer bcs.chainHeadSub.Unsubscribe()
	for {
		select {
		case <-bcs.chainHeadCh:
			bcs.renew()
		case ev := <-bcs.txsCh:
			// System stopped
			if bcs.blockState != nil {
				txs := make(map[common.Address]types.Transactions)
				for _, tx := range ev.Txs {
					acc, _ := types.Sender(bcs.blockState.signer, tx)
					txs[acc] = append(txs[acc], tx)
				}
				// txSet := types.NewTransactionsByPriceAndNonce(bcs.blockState.signer, txs)
				// bcs.commitTransactions(txSet)
			}
		case <-bcs.exitCh:
			return
		case <-bcs.txsSub.Err():
			return
		case <-bcs.chainHeadSub.Err():
			return
		}
	}
}

// newProposalBlock prepare a new block state to propose
func (bo *BlockOperations) newProposalBlock(header *types.Header) (*blockState, error) {
	state, err := bo.blockchain.State()
	if err != nil {
		bo.logger.Error("Failed to get blockchain head state", "err", err)
		return nil, err
	}

	pb := &blockState{
		logger:   log.New("ProposalBlock"),
		signer:   types.LatestSigner(bo.blockchain.chainConfig),
		state:    state,
		tcount:   0,
		gasLimit: configs.BlockGasLimit,
		usedGas:  new(uint64),
		header:   header,
		txs:      []*types.Transaction{},
		receipts: []*types.Receipt{},
	}
	pb.gasPool = new(types.GasPool).AddGas(pb.gasLimit)
	if err := pb.organizeTransactions(bo); err != nil {
		return nil, err
	}

	return pb, nil
}

// organizeTransaction organize transactions in tx pool and try to apply into block state
func (bs *blockState) organizeTransactions(bo *BlockOperations) error {
	pending, err := bo.txPool.Pending()
	if err != nil {
		// @lewtran: panic here?
		bs.logger.Error("Cannot fetch pending transactions", "err", err)
		return nil
	}

	if len(pending) == 0 {
		return nil
	}

	// Split pending transactions to local and remote
	localTxs, remoteTxs := make(map[common.Address]types.Transactions), pending
	for _, account := range bo.txPool.Locals() {
		if txs := remoteTxs[account]; len(txs) > 0 {
			delete(remoteTxs, account)
			localTxs[account] = txs
		}
	}
	if len(localTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(bs.signer, localTxs)
		if err := bs.commitTransactions(bo, txs); err != nil {
			return fmt.Errorf("failed to commit local transactions: %w", err)
		}
	}
	if len(remoteTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(bs.signer, remoteTxs)
		if err := bs.commitTransactions(bo, txs); err != nil {
			return fmt.Errorf("failed to commit remote transactions: %w", err)
		}
	}
	return nil
}

// commitBlockInfo commit block info to current state
func (bo *BlockOperations) commitBlockInfo(bc *BlockChain, state *state.StateDB, header *types.Header, lastCommit stypes.LastCommitInfo, byzVals []stypes.Evidence) ([]*types.Validator, common.Hash, *types.BlockInfo, error) {
	kvmConfig := kvm.Config{}
	blockReward, err := bo.staking.Mint(state, header, bc, kvmConfig)
	if err != nil {
		bo.logger.Error("Fail to mint", "err", err)
		return nil, common.Hash{}, nil, err
	}

	if err := bo.staking.FinalizeCommit(state, header, bc, kvmConfig, lastCommit); err != nil {
		bo.logger.Error("Fail to finalize commit", "err", err)
		return nil, common.Hash{}, nil, err
	}

	if err := bo.staking.DoubleSign(state, header, bc, kvmConfig, byzVals); err != nil {
		bo.logger.Error("Fail to apply double sign", "err", err)
		return nil, common.Hash{}, nil, err
	}

	vals, err := bo.staking.ApplyAndReturnValidatorSets(state, header, bc, kvmConfig)
	if err != nil {
		return nil, common.Hash{}, nil, err
	}

	root, err := state.Commit(true)

	if err != nil {
		bo.logger.Error("Fail to commit new statedb after txs", "err", err)
		return nil, common.Hash{}, nil, err
	}
	err = bo.blockchain.CommitTrie(root)
	if err != nil {
		bo.logger.Error("Fail to write statedb trie to disk", "err", err)
		return nil, common.Hash{}, nil, err
	}

	blockInfo := &types.BlockInfo{
		GasUsed:  *bo.blockState.usedGas,
		Receipts: bo.blockState.receipts,
		Rewards:  blockReward,
		Bloom:    types.CreateBloom(bo.blockState.receipts),
	}

	return vals, root, blockInfo, nil
}

// tryApplyTransaction attempts to appply a single transaction. If the transaction fails, it's modifications are reverted.
func (bs *blockState) commitTransaction(bo *BlockOperations, tx *types.Transaction) error {
	snap := bs.state.Snapshot()
	kvmConfig := kvm.Config{}

	receipt, _, err := ApplyTransaction(bo.blockchain.chainConfig, bs.logger, bo.blockchain, bs.gasPool, bs.state, bs.header, tx, bs.usedGas, kvmConfig)
	if err != nil {
		bs.state.RevertToSnapshot(snap)
		return err
	}
	bs.txs = append(bs.txs, tx)
	bs.receipts = append(bs.receipts, receipt)
	return nil
}

// tryCommitTransactions validate and try commit transactions into block to propose
func (bs *blockState) commitTransactions(bo *BlockOperations, txs *types.TransactionsByPriceAndNonce) error {
	for {
		// If we don't have enough gas for any further transactions then we're done
		if bs.gasPool.Gas() < configs.TxGas {
			log.Error("Not enough gas for further transactions", "have", bs.gasPool, "want", configs.TxGas)
			break
		}

		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}

		if bs.gasPool.Gas() < configs.TxGas {
			log.Trace("Skipping transaction which requires more gas than is left in the block", "hash", tx.Hash(), "gas", bs.gasPool.Gas(), "txgas", tx.Gas())
			txs.Pop()
			continue
		}

		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		from, _ := types.Sender(bs.signer, tx)

		bs.state.Prepare(tx.Hash(), common.Hash{}, bs.tcount)
		err := bs.commitTransaction(bo, tx)
		switch {
		case errors.Is(err, tx_pool.ErrGasLimitReached):
			// Pop the current out-of-gas transaction without shifting in the next from the account
			log.Error("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case errors.Is(err, tx_pool.ErrNonceTooLow):
			// New head notification data race between the transaction pool and miner, shift
			log.Error("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case errors.Is(err, tx_pool.ErrNonceTooHigh):
			// Reorg notification data race between the transaction pool and miner, skip account =
			log.Error("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case errors.Is(err, nil):
			bs.tcount++
			txs.Shift()
		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			log.Error("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}
	return nil
}
