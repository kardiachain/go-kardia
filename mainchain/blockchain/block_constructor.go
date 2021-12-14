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

// proposalBlock
type proposalBlock struct {
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

	blockState *proposalBlock
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
	bcs.blockState = &proposalBlock{
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

	pendingTxs, _ := bcs.txPool.Pending()
	txs := types.NewTransactionsByPriceAndNonce(bcs.blockState.signer, pendingTxs)
	bcs.commitTransactions(txs)
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
				txSet := types.NewTransactionsByPriceAndNonce(bcs.blockState.signer, txs)
				bcs.commitTransactions(txSet)
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
func (bcs *blockConstructor) newProposalBlock() (*proposalBlock, error) {
	state, err := bcs.blockchain.State()
	if err != nil {
		bcs.logger.Error("Failed to get blockchain head state", "err", err)
		return nil, err
	}

	pb := &proposalBlock{
		logger:   log.New("BLOCK"),
		signer:   types.LatestSigner(bcs.chainConfig),
		state:    state,
		tcount:   0,
		gasLimit: configs.BlockGasLimit,
		usedGas:  new(uint64),
		header:   bcs.blockState.header,
		txs:      []*types.Transaction{},
		receipts: []*types.Receipt{},
	}
	pb.gasPool = new(types.GasPool).AddGas(pb.gasLimit)
	if err := pb.organizeTransactions(bcs); err != nil {
		return nil, err
	}

	return pb, nil
}

// organizeTransaction organize transactions in tx pool and try to apply into block state
func (bs *proposalBlock) organizeTransactions(bcs *blockConstructor) error {
	pending, err := bcs.txPool.Pending()
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
	for _, account := range bcs.txPool.Locals() {
		if txs := remoteTxs[account]; len(txs) > 0 {
			delete(remoteTxs, account)
			localTxs[account] = txs
		}
	}
	if len(localTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(bs.signer, localTxs)
		if err := bcs.commitTransactions(txs); err != nil {
			return fmt.Errorf("failed to commit local transactions: %w", err)
		}
	}
	if len(remoteTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(bs.signer, remoteTxs)
		if err := bcs.commitTransactions(txs); err != nil {
			return fmt.Errorf("failed to commit remote transactions: %w", err)
		}
	}
	return nil
}

// commitBlockInfo commit block info to current state
func (bcs *blockConstructor) commitBlockInfo(bc *BlockChain, state *state.StateDB, header *types.Header, lastCommit stypes.LastCommitInfo, byzVals []stypes.Evidence) ([]*types.Validator, common.Hash, *types.BlockInfo, error) {
	kvmConfig := kvm.Config{}
	blockReward, err := bcs.staking.Mint(state, header, bc, kvmConfig)
	if err != nil {
		bcs.logger.Error("Fail to mint", "err", err)
		return nil, common.Hash{}, nil, err
	}

	if err := bcs.staking.FinalizeCommit(state, header, bc, kvmConfig, lastCommit); err != nil {
		bcs.logger.Error("Fail to finalize commit", "err", err)
		return nil, common.Hash{}, nil, err
	}

	if err := bcs.staking.DoubleSign(state, header, bc, kvmConfig, byzVals); err != nil {
		bcs.logger.Error("Fail to apply double sign", "err", err)
		return nil, common.Hash{}, nil, err
	}

	vals, err := bcs.staking.ApplyAndReturnValidatorSets(state, header, bc, kvmConfig)
	if err != nil {
		return nil, common.Hash{}, nil, err
	}

	root, err := state.Commit(true)

	if err != nil {
		bcs.logger.Error("Fail to commit new statedb after txs", "err", err)
		return nil, common.Hash{}, nil, err
	}
	err = bcs.blockchain.CommitTrie(root)
	if err != nil {
		bcs.logger.Error("Fail to write statedb trie to disk", "err", err)
		return nil, common.Hash{}, nil, err
	}

	blockInfo := &types.BlockInfo{
		GasUsed:  *bcs.blockState.usedGas,
		Receipts: bcs.blockState.receipts,
		Rewards:  blockReward,
		Bloom:    types.CreateBloom(bcs.blockState.receipts),
	}

	return vals, root, blockInfo, nil
}

// tryApplyTransaction attempts to appply a single transaction. If the transaction fails, it's modifications are reverted.
func (bcs *blockConstructor) commitTransaction(tx *types.Transaction) error {
	snap := bcs.blockState.state.Snapshot()
	kvmConfig := kvm.Config{}

	receipt, _, err := ApplyTransaction(bcs.chainConfig, bcs.blockState.logger, bcs.blockchain, bcs.blockState.gasPool, bcs.blockState.state, bcs.blockState.header, tx, bcs.blockState.usedGas, kvmConfig)
	if err != nil {
		bcs.blockState.state.RevertToSnapshot(snap)
		return err
	}
	bcs.blockState.txs = append(bcs.blockState.txs, tx)
	bcs.blockState.receipts = append(bcs.blockState.receipts, receipt)
	return nil
}

// tryCommitTransactions validate and try commit transactions into block to propose
func (bcs *blockConstructor) commitTransactions(txs *types.TransactionsByPriceAndNonce) error {
	for {
		// If we don't have enough gas for any further transactions then we're done
		if bcs.blockState.gasPool.Gas() < configs.TxGas {
			log.Error("tryApplyTransaction Not enough gas for further transactions", "have", bcs.blockState.gasPool, "want", configs.TxGas)
			break
		}

		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}

		if bcs.blockState.gasPool.Gas() < configs.TxGas {
			log.Trace("tryApplyTransaction Skipping transaction which requires more gas than is left in the block", "hash", tx.Hash(), "gas", bcs.blockState.gasPool.Gas(), "txgas", tx.Gas())
			txs.Pop()
			continue
		}

		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		from, _ := types.Sender(bcs.blockState.signer, tx)

		bcs.blockState.state.Prepare(tx.Hash(), common.Hash{}, bcs.blockState.tcount)
		err := bcs.commitTransaction(tx)
		switch {
		case errors.Is(err, tx_pool.ErrGasLimitReached):
			// Pop the current out-of-gas transaction without shifting in the next from the account
			log.Error("tryApplyTransaction Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case errors.Is(err, tx_pool.ErrNonceTooLow):
			// New head notification data race between the transaction pool and miner, shift
			log.Error("tryApplyTransaction Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case errors.Is(err, tx_pool.ErrNonceTooHigh):
			// Reorg notification data race between the transaction pool and miner, skip account =
			log.Error("tryApplyTransaction Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case errors.Is(err, nil):
			bcs.blockState.tcount++
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
