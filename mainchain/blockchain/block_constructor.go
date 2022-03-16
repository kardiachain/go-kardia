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
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/kvm"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
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
	logger log.Logger

	blockchain *BlockChain
	txPool     *tx_pool.TxPool

	// Channels
	startCh chan struct{}
	exitCh  chan struct{}
	wg      sync.WaitGroup

	// Subscriptions
	txsCh        chan events.NewTxsEvent
	chainHeadCh  chan events.ChainHeadEvent
	txsSub       event.Subscription
	chainHeadSub event.Subscription

	// consensus state
	proposerAddr common.Address
	lastState    cstate.LatestBlockState
	commit       *types.Commit
	evidence     []types.Evidence

	// snapshot block and mutex
	snapshotBlock   *types.Block
	snapshotPartSet *types.PartSet
	snapshotMu      sync.RWMutex

	// atomic status counters
	status int32
}

// newblockConstructor creates a new block constructor
func newBlockConstructor(blockchain *BlockChain, txPool *tx_pool.TxPool) *blockConstructor {
	bcs := &blockConstructor{
		logger:      log.New("blockConstructor"),
		blockchain:  blockchain,
		txPool:      txPool,
		exitCh:      make(chan struct{}),
		startCh:     make(chan struct{}, 1),
		txsCh:       make(chan events.NewTxsEvent, txChanSize),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
	}

	// Subscribe NewTxsEvent for tx pool
	bcs.txsSub = txPool.SubscribeNewTxsEvent(bcs.txsCh)
	// Subscribe events for blockchain
	bcs.chainHeadSub = blockchain.SubscribeChainHeadEvent(bcs.chainHeadCh)

	bcs.start()

	bcs.wg.Add(1)
	go bcs.constructionLoop()

	return bcs
}

// start sets the status to 1 (running)
func (bcs *blockConstructor) start() {
	atomic.StoreInt32(&bcs.status, 1)
	bcs.startCh <- struct{}{}
}

// isRunning returns the running status
func (bcs *blockConstructor) isRunning() bool {
	return atomic.LoadInt32(&bcs.status) == 1
}

// constructionLoop is a standalone goroutine to regenerate the sealing task based on the received event.
func (bcs *blockConstructor) constructionLoop() {
	defer bcs.wg.Done()
	defer bcs.txsSub.Unsubscribe()
	defer bcs.chainHeadSub.Unsubscribe()

	var taskCtx context.Context
	var cancel context.CancelFunc
	var wg sync.WaitGroup

	defer wg.Wait()
	txsCh := make(chan events.NewTxsEvent, txChanSize)

	generateBlock := func() {
		if cancel != nil {
			cancel()
		}
		wg.Wait()
		taskCtx, cancel = context.WithCancel(context.Background())
		wg.Add(1)

		if bcs.isRunning() {
			go func() {
				bcs.constructProposalBlock(taskCtx, txsCh)
				wg.Done()
			}()
		} else {
			go func() {
				bcs.constructPendingBlock(taskCtx, txsCh)
				wg.Done()
			}()
		}
	}

	for {
		select {
		case <-bcs.startCh:
			generateBlock()

		case <-bcs.chainHeadCh:
			generateBlock()

		case ev := <-bcs.txsCh:
			if !bcs.isRunning() {
				select {
				case txsCh <- ev:
				default:
				}
			}
		// System stopped
		case <-bcs.exitCh:
			if cancel != nil {
				cancel()
			}
			return
		case <-bcs.chainHeadSub.Err():
			if cancel != nil {
				cancel()
			}
			return
		case <-bcs.txsSub.Err():
			if cancel != nil {
				cancel()
			}
			return
		}
	}
}

// newProposalBlock prepare a new block state to propose
func newProposalBlock(bcs *blockConstructor) (*proposalBlock, error) {
	lastBlock := bcs.blockchain.CurrentBlock()
	lastHeight := lastBlock.Height()
	lastState, err := bcs.blockchain.StateAt(lastHeight)
	if err != nil {
		bcs.logger.Error("Failed to get blockchain head state", "err", err)
		return nil, err
	}

	// prepare a new header
	header := &types.Header{
		Height:   lastHeight + 1,
		GasLimit: configs.BlockGasLimitGalaxias,
	}

	pb := &proposalBlock{
		logger:   log.New(),
		signer:   types.LatestSigner(bcs.blockchain.chainConfig),
		state:    lastState,
		tcount:   0,
		gasLimit: header.GasLimit,
		usedGas:  new(uint64),
		header:   header,
		txs:      []*types.Transaction{},
		receipts: []*types.Receipt{},
	}
	pb.gasPool = new(types.GasPool).AddGas(pb.gasLimit)

	if bcs.isRunning() {
		pb.state.IntermediateRoot(true)
	}

	return pb, nil
}

func (bcs *blockConstructor) createSnapshotBlock(pb *proposalBlock) {
	bcs.snapshotMu.Lock()
	defer bcs.snapshotMu.Unlock()

	bcs.snapshotBlock = types.NewBlock(
		pb.header,
		pb.txs,
		nil,
		nil,
	)

	bcs.snapshotPartSet = bcs.snapshotBlock.MakePartSet(types.BlockPartSizeBytes)
}

// updateHeader update the block header from given data.
func (bcs *blockConstructor) updateHeader(time time.Time, blockID types.BlockID,
	proposer common.Address, validatorsHash common.Hash, nextValidatorHash common.Hash, appHash common.Hash) *types.Header {
	bcs.snapshotMu.Lock()
	defer bcs.snapshotMu.Unlock()

	bcs.snapshotBlock.Header().Time = time
	bcs.snapshotBlock.Header().LastBlockID = blockID
	bcs.snapshotBlock.Header().ProposerAddress = proposer
	bcs.snapshotBlock.Header().ValidatorsHash = validatorsHash
	bcs.snapshotBlock.Header().NextValidatorsHash = nextValidatorHash
	bcs.snapshotBlock.Header().AppHash = appHash

	return bcs.snapshotBlock.Header()
}

func (bcs *blockConstructor) constructProposalBlock(ctx context.Context, txsCh chan events.NewTxsEvent) {
	pb, err := newProposalBlock(bcs)
	if err != nil {
		return
	}
	if err = pb.organizeTransactions(ctx, bcs); err != nil {
		bcs.logger.Error("Failed to apply transactions to the block", "err", err)
		return
	}
}

func (bcs *blockConstructor) constructPendingBlock(ctx context.Context, txsCh chan events.NewTxsEvent) {
	pb, err := newProposalBlock(bcs)
	if err != nil {
		return
	}
	if err = pb.organizeTransactions(ctx, bcs); err != nil {
		bcs.logger.Error("Failed to apply transactions to the block", "err", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-txsCh:
			if !bcs.isRunning() {
				if gp := pb.gasPool; gp != nil && gp.Gas() < configs.TxGas {
					return
				}
				txs := make(map[common.Address]types.Transactions)
				for _, tx := range ev.Txs {
					acc, _ := types.Sender(pb.signer, tx)
					txs[acc] = append(txs[acc], tx)
				}

				txSet := types.NewTransactionsByPriceAndNonce(pb.signer, txs)
				pb.commitTransactions(ctx, bcs, txSet)
			}
		}
	}
}

// organizeTransaction organize transactions in tx pool and try to apply into block state
func (pb *proposalBlock) organizeTransactions(ctx context.Context, bcs *blockConstructor) error {
	pending, err := bcs.txPool.Pending()
	if err != nil {
		bcs.logger.Error("Cannot fetch pending transactions", "err", err)
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
		txs := types.NewTransactionsByPriceAndNonce(pb.signer, localTxs)
		if err := pb.commitTransactions(ctx, bcs, txs); err != nil {
			return fmt.Errorf("failed to commit local transactions: %w", err)
		}
	}
	if len(remoteTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(pb.signer, remoteTxs)
		if err := pb.commitTransactions(ctx, bcs, txs); err != nil {
			return fmt.Errorf("failed to commit remote transactions: %w", err)
		}
	}
	return nil
}

// commitTransaction attempts to appply a single transaction. If the transaction fails, it's modifications are reverted.
func (pb *proposalBlock) commitTransaction(bcs *blockConstructor, tx *types.Transaction) error {
	snap := pb.state.Snapshot()
	kvmConfig := kvm.Config{}

	receipt, _, err := ApplyTransaction(bcs.blockchain.chainConfig, bcs.logger, bcs.blockchain, pb.gasPool, pb.state, pb.header, tx, pb.usedGas, kvmConfig)
	if err != nil {
		pb.state.RevertToSnapshot(snap)
		return err
	}
	pb.txs = append(pb.txs, tx)
	pb.receipts = append(pb.receipts, receipt)
	return nil
}

// commitTransactions validate and commit transactions into block to propose
func (pb *proposalBlock) commitTransactions(ctx context.Context, bcs *blockConstructor, txs *types.TransactionsByPriceAndNonce) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// pass ctx
		}

		// If we don't have enough gas for any further transactions then we're done
		if pb.gasPool.Gas() < configs.TxGas {
			log.Error("Not enough gas for further transactions", "have", pb.gasPool, "want", configs.TxGas)
			break
		}

		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}

		if pb.gasPool.Gas() < configs.TxGas {
			log.Trace("Skipping transaction which requires more gas than is left in the block", "hash", tx.Hash(), "gas", pb.gasPool.Gas(), "txgas", tx.Gas())
			txs.Pop()
			continue
		}

		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		from, _ := types.Sender(pb.signer, tx)

		pb.state.Prepare(tx.Hash(), common.Hash{}, pb.tcount)
		err := pb.commitTransaction(bcs, tx)
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
			pb.tcount++
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
