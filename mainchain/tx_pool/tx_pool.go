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

package tx_pool

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/kai/state"
	kaidb "github.com/kardiachain/go-kardia/kai/storage"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
)

var (
	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")
)

// blockChain provides the state of blockchain and current gas limit to do
// some pre checks in tx pool and event subscribers.
type blockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)
	DB() kaidb.Database
	SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription
}

// TxPoolConfig are the configuration parameters of the transaction pool.
type TxPoolConfig struct {
	NoLocals  bool          // Whether local transaction handling should be disabled
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts
	NumberOfWorkers int
	WorkerCap       int
	BlockSize       int
}

// DefaultTxPoolConfig contains the default configurations for the transaction
// pool.
var DefaultTxPoolConfig = TxPoolConfig{
	GlobalSlots:  4096,
	GlobalQueue:  1024,
}

// GetDefaultTxPoolConfig returns default txPoolConfig with given dir path
func GetDefaultTxPoolConfig(path string) *TxPoolConfig {
	conf := DefaultTxPoolConfig
	return &conf
}

// TxPool contains all currently known transactions. Transactions
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are included in the blockchain.
//
// The pool separates processable transactions (which can be applied to the
// current state) and future transactions. Transactions move between those
// two states over time as they are received and processed.
type TxPool struct {
	logger log.Logger

	config       TxPoolConfig
	chainconfig  *configs.ChainConfig
	chain        blockChain
	gasPrice     *big.Int
	txFeed       event.Feed
	scope        event.SubscriptionScope

	// 2 channels: workerCh and pendingCh
	txsCh       chan []interface{}
	pendingCh   chan []interface{}
	allCh       chan []interface{}
	precheckCh  chan []interface{}

	chainHeadCh  chan events.ChainHeadEvent
	chainHeadSub event.Subscription
	mu           sync.RWMutex

	numberOfWorkers int
	workerCap       int

	currentState *state.StateDB      // Current state in the blockchain head
	pendingState *state.ManagedState // Pending state tracking virtual nonces

	currentMaxGas uint64 // Current gas limit for transaction caps
	totalPendingGas uint64

	journal *txJournal  // Journal of local transaction to back up to disk

	pending  *common.Set   // All currently processable transactions
	all      *common.Set                        // All transactions to allow lookups

	wg sync.WaitGroup // for shutdown sync
}

// NewTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewTxPool(logger log.Logger, config TxPoolConfig, chainconfig *configs.ChainConfig, chain blockChain) *TxPool {
	// Create the transaction pool with its initial settings
	pool := &TxPool{
		logger:      logger,
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		pending:     common.NewSet(int64(config.GlobalSlots)),
		all:         common.NewSet(int64(config.GlobalQueue)),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
		totalPendingGas: uint64(0),
		txsCh: make(chan []interface{}, 100),
		pendingCh: make(chan []interface{}),
		allCh: make(chan []interface{}),
		precheckCh: make(chan []interface{}),
		numberOfWorkers: config.NumberOfWorkers,
		workerCap: config.WorkerCap,
	}
	//pool.priced = newTxPricedList(logger, pool.all)
	pool.reset(nil, chain.CurrentBlock().Header())

	// Subscribe events from blockchain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	go pool.loop()

	return pool
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *TxPool) loop() {
	// Track the previous head headers for transaction reorgs
	head := pool.chain.CurrentBlock()

	collectTicker := time.NewTicker(500 * time.Millisecond)

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			go pool.reset(head.Header(), ev.Block.Header())
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return
		case txs := <-pool.pendingCh:
			go pool.handlePendingTxs(txs)
		case txs := <-pool.precheckCh:
			go pool.validateTxs(txs)
		case <-collectTicker.C:
			go pool.collectTxs()
		}
	}
}

func (pool *TxPool) collectTxs() {
	for i := 0; i < pool.numberOfWorkers; i++ {
		go pool.work(i, pool.txsCh)
	}
}

func (pool *TxPool) work(id int, jobs <-chan []interface{}) {
	for job := range jobs {
		go pool.AddRemotes(job)
	}
}

func (pool *TxPool) IsFull() (bool, int64) {
	pendingSize := pool.PendingSize()
	return int64(pendingSize) >= int64(pool.config.GlobalSlots), int64(pendingSize)
}

func (pool *TxPool) AddTxs(txs []interface{}) {
	pool.precheckCh<-txs
}

func (pool *TxPool) validateTxs(txs []interface{}) {
	isFull, size := pool.IsFull()
	if isFull {
		pool.logger.Error(fmt.Sprintf("pool has reached its limit %v/%v", size, pool.config.GlobalSlots))
		return
	}

	if len(txs) > 0 {
		to := pool.workerCap
		if len(txs) < to {
			to = len(txs)
		}
		pool.txsCh <- txs[0:to]
		go pool.validateTxs(txs[to:])
	}
}

func (pool *TxPool) ResetWorker(workers int, cap int) {
	pool.numberOfWorkers = workers
	pool.workerCap = cap
}

// ClearPending is used to clear pending data. Note: this function is only for testing only
func (pool *TxPool) ClearPending() {
	pool.pending = common.NewSet(int64(pool.config.GlobalSlots))
}

// lockedReset is a wrapper around reset to allow calling it in a thread safe
// manner. This method is only ever used in the tester!
func (pool *TxPool) lockedReset(oldHead, newHead *types.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(oldHead, newHead)
}

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
func (pool *TxPool) reset(oldHead, newHead *types.Header) {
	// Initialize the internal state to the current head
	currentBlock := pool.chain.CurrentBlock()

	if newHead == nil {
		newHead = currentBlock.Header() // Special case during testing
	}

	statedb, err := pool.chain.StateAt(newHead.Root)
	pool.logger.Info("TxPool reset state to new head block", "height", newHead.Height, "root", newHead.Root)
	if err != nil {
		pool.logger.Error("Failed to reset txpool state", "err", err)
		return
	}
	pool.mu.Lock()
	pool.pendingState = state.ManageState(statedb)
	pool.mu.Unlock()

	go pool.saveTxs(currentBlock.Transactions())
	// remove current block's txs from pending
	//pool.RemoveTxs(txs)
	//if _, err := pool.Pending(0, false); err != nil {
	//	pool.logger.Error("error while remove invalid txs from reset", "err", err)
	//}
}

// Stop terminates the transaction pool.
func (pool *TxPool) Stop() {
	// Unsubscribe all subscriptions registered from txpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()
	pool.logger.Info("Transaction pool stopped")
}

// SubscribeNewTxsEvent registers a subscription of NewTxsEvent and
// starts sending event to the given channel.
func (pool *TxPool) SubscribeNewTxsEvent(ch chan<- events.NewTxsEvent) event.Subscription {
	return pool.scope.Track(pool.txFeed.Subscribe(ch))
}

// GasPrice returns the current gas price enforced by the transaction pool.
func (pool *TxPool) GasPrice() *big.Int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return new(big.Int).Set(pool.gasPrice)
}

// State returns the virtual managed state of the transaction pool.
func (pool *TxPool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

//func (pool *TxPool) CurrentState() *state.StateDB {
//	return pool.currentState
//}

func (pool *TxPool) pendingValidation(tx *types.Transaction) error {
	_, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}
	// Ensure the transaction adheres to nonce ordering
	//currentState := pool.currentState
	//senderNonce := currentState.GetNonce(from)
	//if senderNonce > tx.Nonce() {
	//	return fmt.Errorf("nonce too low expected %v found %v", senderNonce, tx.Nonce())
	//}
	//if pool.currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
	//	pool.logger.Error("Bad txn cost", "balance", pool.currentState.GetBalance(from), "cost", tx.Cost(), "from", from)
	//	return ErrInsufficientFunds
	//}
	// TODO: this can be moved to execute transactions step or may not need
	//readTimeStart := getTime()
	//if t, _, _, _ := chaindb.ReadTransaction(pool.chain.DB(), tx.Hash()); t != nil {
	//	errs[i] =  fmt.Errorf("known transaction: %x", tx.Hash())
	//}
	return nil
}

func (pool *TxPool) ProposeTransactions() types.Transactions {
	txs, _ := pool.Pending(pool.config.BlockSize, true)
	return txs
}

func getTime() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// Pending retrieves all currently processable transactions, groupped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Pending(limit int, removeResult bool) (types.Transactions, error) {

	//pool.mu.Lock()
	//defer pool.mu.Unlock()

	startTime := getTime()
	pending := make(types.Transactions, 0)
	removedTxs := make(TxInterfaceByNonce, 0)
	results := make(TxInterfaceByNonce, 0)

	// get pending list
	pendingList := TxInterfaceByNonce(pool.pending.List())
	count := 0

	// sort pending list
	sort.Sort(pendingList)

	for _, pendingTx := range pendingList {
		if limit > 0 && count == limit {
			break
		}
		tx := pendingTx.(*types.Transaction)

		// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
		if tx.Size() > 32*1024 || tx.Value().Sign() < 0 || pool.all.Has(tx.Hash()) {
			removedTxs = append(removedTxs, pendingTx)
			continue
		}

		pending = append(pending, tx)
		if removeResult {
			results = append(results, pendingTx)
		}
		count++
	}

	if len(removedTxs) > 0 {
		go pool.pending.Remove(removedTxs...)
	}

	if len(results) > 0 {
		go pool.pending.Remove(results...)
	}
	endTime := getTime()
	pool.logger.Error("get pending txs", "txs", len(pending), "total time", endTime-startTime, "limit", limit)
	return pending, nil
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxPool) ValidateTx(tx *types.Transaction, local bool) error {

	if uint64(pool.PendingSize()) >= pool.config.GlobalSlots {
		return fmt.Errorf("pool has reached its limit")
	}
	return nil
}

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
//
// If a newly added transaction is marked as local, its sending account will be
// whitelisted, preventing any associated transaction from being dropped out of
// the pool due to pricing constraints.
func (pool *TxPool) add(tx *types.Transaction, local bool) (bool, error) {
	if err := pool.ValidateTx(tx, local); err != nil {
		return false, err
	}
	return true, nil
}


// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *TxPool) AddLocal(tx *types.Transaction) error {
	if err := pool.addTx(tx, !pool.config.NoLocals); err != nil {
		return err
	}
	_, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	//pool.all.Add(tx.Hash())
	pool.pending.Add(tx)
	go pool.txFeed.Send(events.NewTxsEvent{Txs: []*types.Transaction{tx}})
	return nil
}

// AddRemote enqueues a single transaction into the pool if it is valid. If the
// sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *TxPool) AddRemote(tx *types.Transaction) error {
	if err := pool.addTx(tx, false); err != nil {
		return err
	}
	_, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	//pool.all.Add(tx.Hash())
	pool.pending.Add(tx)
	go pool.txFeed.Send(events.NewTxsEvent{Txs: []*types.Transaction{tx}})
	return nil
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *TxPool) AddLocals(txs []*types.Transaction) error {
	//return pool.addTxs(txs, !pool.config.NoLocals)
	return nil
}

// AddRemotes enqueues a batch of transactions into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *TxPool) AddRemotes(txs []interface{}) {
	pool.addTxs(txs, false)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx *types.Transaction, local bool) error {

	// Try to inject the transaction and update any state
	if err := pool.ValidateTx(tx, local); err != nil {
		pool.logger.Trace("Discarding invalid transaction", "hash", tx.Hash().Hex(), "err", err)
		return err
	}

	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs []interface{}, local bool) {
	//pool.mu.Lock()
	//defer pool.mu.Unlock()

	promoted := make([]*types.Transaction, 0)
	pendings := make(TxInterfaceByNonce, 0)

	for _, txInterface := range txs {
		if txInterface == nil {
			continue
		}
		tx := txInterface.(*types.Transaction)
		if !pool.all.Has(tx.Hash()) {
			promoted = append(promoted, tx)
		}
		pendings = append(pendings, tx)
	}

	if len(promoted) > 0 {
		go pool.txFeed.Send(events.NewTxsEvent{Txs: promoted})
		pool.pendingCh <- pendings
	}
}

func (pool *TxPool) CachedTxs() *common.Set {
	return pool.all
}

func (pool *TxPool) saveTxs(txs []*types.Transaction) {
	if len(txs) > 0 {
		hashes := make([]interface{}, len(txs))
		for i, tx := range txs {
			hashes[i] = tx.Hash()
		}
		go pool.all.Add(hashes...)
	}
}

func (pool *TxPool) handlePendingTxs(txs TxInterfaceByNonce) {
	pool.pending.Add(txs...)
}

// RemoveTx removes transactions from pending queue.
// This function is mainly for caller in blockchain/consensus to directly remove committed txs.
//
func (pool *TxPool) RemoveTxs(txs types.Transactions) {
	pool.logger.Trace("Removing Txs from pending", "txs", len(txs))
	startTime := getTime()
	txsInterfaces := make([]interface{}, len(txs))
	for i, tx := range txs {
		txsInterfaces[i] = tx
	}
	go pool.pending.Remove(txsInterfaces...)
	diff := getTime() - startTime
	pool.logger.Trace("total time to finish removing txs from pending", "time", diff)
}

func (pool *TxPool) PendingSize() int {
	return pool.pending.Size()
}

func (pool *TxPool) GetPendingData() *types.Transactions {
	txs := make(types.Transactions, 0)
	for _, txInterface := range pool.pending.List() {
		txs = append(txs, txInterface.(*types.Transaction))
	}
	return &txs
}

// TxInterfaceByNonce implements the sort interface to allow sorting a list of transactions (in interface)
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxInterfaceByNonce []interface{}
func (s TxInterfaceByNonce) Len() int           { return len(s) }
func (s TxInterfaceByNonce) Less(i, j int) bool {
	return s[i].(*types.Transaction).Nonce() < s[j].(*types.Transaction).Nonce()
}
func (s TxInterfaceByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
