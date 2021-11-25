package blockchain

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/params"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
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

// environment is the worker's current environment and holds all of the current state information.
type environment struct {
	signer types.Signer

	state   *state.StateDB // apply state changes here
	tcount  int            // tx count in cycle
	gasPool *types.GasPool // available gas used to pack transactions

	header  *types.Header
	txs     []*types.Transaction
	gasUsed uint64
}

type worker struct {
	current      *environment
	txsCh        chan events.NewTxsEvent
	chainHeadCh  chan events.ChainHeadEvent
	txsSub       event.Subscription
	chainHeadSub event.Subscription
	chainConfig  *params.ChainConfig
	logger       log.Logger
	chain        *BlockChain
	txpool       *tx_pool.TxPool
	exitCh       chan struct{}
}

func newWorker(chain *BlockChain, txpool *tx_pool.TxPool, cfg *params.ChainConfig) *worker {
	w := &worker{
		txsCh:       make(chan events.NewTxsEvent, txChanSize),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
		chain:       chain,
		chainConfig: cfg,
		logger:      log.New("Worker"),
		exitCh:      make(chan struct{}),
		txpool:      txpool,
	}

	// Subscribe NewTxsEvent for tx pool
	w.txsSub = txpool.SubscribeNewTxsEvent(w.txsCh)
	// Subscribe events for blockchain
	w.chainHeadSub = chain.SubscribeChainHeadEvent(w.chainHeadCh)

	go w.mainLoop()

	return w
}

func (w *worker) commitTransaction(tx *types.Transaction, coinbase common.Address) error {
	snap := w.current.state.Snapshot()

	_, _, err := ApplyTransaction(w.logger, w.chain, w.current.gasPool, w.current.state, w.current.header, tx, &w.current.gasUsed, *w.chain.GetVMConfig())
	if err != nil {
		w.current.state.RevertToSnapshot(snap)
		return err
	}
	w.current.txs = append(w.current.txs, tx)

	return nil
}

func (w *worker) commitTransactions(txs *types.TransactionsByPriceAndNonce, coinbase common.Address, interrupt *int32) bool {
	// Short circuit if current is nil
	if w.current == nil {
		return true
	}

	gasLimit := w.current.header.GasLimit
	if w.current.gasPool == nil {
		w.current.gasPool = new(types.GasPool).AddGas(gasLimit)
	}
	for {
		// If we don't have enough gas for any further transactions then we're done
		if w.current.gasPool.Gas() < params.TxGas {
			log.Trace("Not enough gas for further transactions", "have", w.current.gasPool, "want", params.TxGas)
			break
		}
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		//
		// We use the eip155 signer regardless of the current hf.
		from, _ := types.Sender(w.current.signer, tx)
		// Start executing the transaction
		w.current.state.Prepare(tx.Hash(), w.current.header.Hash(), w.current.tcount)

		err := w.commitTransaction(tx, coinbase)
		switch {
		case errors.Is(err, tx_pool.ErrGasLimitReached):
			// Pop the current out-of-gas transaction without shifting in the next from the account
			log.Trace("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case errors.Is(err, tx_pool.ErrNonceTooLow):
			// New head notification data race between the transaction pool and miner, shift
			log.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case errors.Is(err, tx_pool.ErrNonceTooHigh):
			// Reorg notification data race between the transaction pool and miner, skip account =
			log.Trace("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case errors.Is(err, nil):
			w.current.tcount++
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			log.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}
	return false
}

func (w *worker) renew() {
	parent := w.chain.CurrentBlock()
	currentState, _ := w.chain.State()
	w.current = &environment{
		signer: types.HomesteadSigner{},
		state:  currentState,
		tcount: 0,
		txs:    []*types.Transaction{},
		header: &types.Header{
			Height:   parent.Height() + 1,
			GasLimit: parent.GasLimit(),
			Time:     time.Now(),
		},
	}

	pendingTxs, _ := w.txpool.Pending()
	if len(pendingTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(w.current.signer, pendingTxs)
		w.commitTransactions(txs, common.Address{}, nil)
	}
}

// mainLoop is a standalone goroutine to regenerate the sealing task based on the received event.
func (w *worker) mainLoop() {
	defer w.txsSub.Unsubscribe()
	defer w.chainHeadSub.Unsubscribe()
	for {
		select {
		case <-w.chainHeadCh:
			w.renew()
		case ev := <-w.txsCh:
			// System stopped
			if w.current != nil {
				txs := make(map[common.Address]types.Transactions)
				for _, tx := range ev.Txs {
					acc, _ := types.Sender(w.current.signer, tx)
					txs[acc] = append(txs[acc], tx)
				}
				txset := types.NewTransactionsByPriceAndNonce(w.current.signer, txs)
				w.commitTransactions(txset, common.Address{}, nil)
			}

		case <-w.exitCh:
			return
		case <-w.txsSub.Err():
			return
		case <-w.chainHeadSub.Err():
			return
		}
	}
}
