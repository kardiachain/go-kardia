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
	addressState map[common.Address]uint64 // address state will cache current state of addresses with the latest nonce

	currentMaxGas uint64 // Current gas limit for transaction caps
	totalPendingGas uint64

	journal *txJournal  // Journal of local transaction to back up to disk

	pendingSize uint // pendingSize is a counter, increased when adding new txs, decreased when remove txs
	pending  map[common.Address]types.Transactions   // All currently processable transactions
	all      *common.Set                        // All transactions to allow lookups
	promotableQueue *common.Set

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
		pending:     make(map[common.Address]types.Transactions),
		all:         common.NewSet(int64(config.GlobalQueue)),
		promotableQueue: common.NewSet(100000),
		addressState: make(map[common.Address]uint64),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
		totalPendingGas: uint64(0),
		txsCh: make(chan []interface{}, 100),
		pendingCh: make(chan []interface{}),
		allCh: make(chan []interface{}),
		precheckCh: make(chan []interface{}),
		numberOfWorkers: config.NumberOfWorkers,
		workerCap: config.WorkerCap,
		pendingSize: 0,
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

	collectTicker := time.NewTicker(100 * time.Millisecond)
	//cleanUpTicker := time.NewTicker(5 * time.Minute)

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			go pool.reset(head.Header(), ev.Block.Header())
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return
		//case txs := <-pool.pendingCh:
		//	pool.handlePendingTxs(txs)
		//case txs := <-pool.precheckCh:
		//	go pool.validateTxs(txs)
		case <-collectTicker.C:
			go pool.collectTxs()
		//case <-cleanUpTicker.C:
		//	pool.cleanUp()
		}
	}
}

func (pool *TxPool) collectTxs() {
	for {
		for i := 0; i < pool.numberOfWorkers; i++ {
			go pool.work(i, <-pool.txsCh)
		}
	}
}

func (pool *TxPool) work(index int, txs []interface{}) {
	go pool.AddRemotes(txs)
}

func (pool *TxPool) IsFull() (bool, int64) {
	pendingSize := pool.PendingSize()
	return int64(pendingSize) >= int64(pool.config.GlobalSlots), int64(pendingSize)
}

func (pool *TxPool) AddTxs(txs []interface{}) {
	//pool.precheckCh<-txs
	if len(txs) > 0 {
		to := pool.workerCap
		if len(txs) < to {
			to = len(txs)
		}
		pool.txsCh <- txs[0:to]
		go pool.AddTxs(txs[to:])
	}
}

func (pool *TxPool) validateTxs(txs []interface{}) {
	//if len(txs) > 0 {
	//	to := pool.workerCap
	//	if len(txs) < to {
	//		to = len(txs)
	//	}
	//	pool.txsCh <- txs[0:to]
	//	go pool.validateTxs(txs[to:])
	//}
}

func (pool *TxPool) ResetWorker(workers int, cap int) {
	pool.numberOfWorkers = workers
	pool.workerCap = cap
}

// ClearPending is used to clear pending data. Note: this function is only for testing only
func (pool *TxPool) ClearPending() {
	pool.pending = make(map[common.Address]types.Transactions)
	pool.pendingSize = 0
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
	//if newHead == nil {
	//	newHead = currentBlock.Header() // Special case during testing
	//}

	//statedb, err := pool.chain.StateAt(newHead.Root)
	//pool.logger.Info("TxPool reset state to new head block", "height", newHead.Height, "root", newHead.Root)
	//if err != nil {
	//	pool.logger.Error("Failed to reset txpool state", "err", err)
	//	return
	//}
	//pool.mu.Lock()
	//pool.pendingState = state.ManageState(statedb)
	//pool.mu.Unlock()

	pool.RemoveTxs(currentBlock.Transactions())
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
//func (pool *TxPool) Pending(limit int, removeResult bool) (types.Transactions, error) {
//
//	//pool.mu.Lock()
//	//defer pool.mu.Unlock()
//
//	startTime := getTime()
//	pending := make(types.Transactions, 0)
//	removedTxs := make(TxInterfaceByNonce, 0)
//	results := make(TxInterfaceByNonce, 0)
//
//	// get pending list
//	pendingList := TxInterfaceByNonce(pool.pending.List())
//	count := 0
//
//	// sort pending list
//	sort.Sort(pendingList)
//
//	for _, pendingTx := range pendingList {
//		if limit > 0 && count == limit {
//			break
//		}
//		tx := pendingTx.(*types.Transaction)
//
//		// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
//		if tx.Size() > 32*1024 || tx.Value().Sign() < 0 || pool.all.Has(tx.Hash()) {
//			removedTxs = append(removedTxs, pendingTx)
//			continue
//		}
//
//		pending = append(pending, tx)
//		if removeResult {
//			results = append(results, pendingTx)
//		}
//		count++
//	}
//
//	if len(removedTxs) > 0 {
//		go pool.pending.Remove(removedTxs...)
//	}
//
//	if len(results) > 0 {
//		go pool.pending.Remove(results...)
//	}
//	endTime := getTime()
//	pool.logger.Error("get pending txs", "txs", len(pending), "total time", endTime-startTime, "limit", limit)
//	return pending, nil
//}
func (pool *TxPool) Pending(limit int, removeResult bool) (types.Transactions, error) {

	pool.mu.Lock()
	defer pool.mu.Unlock()

	startTime := getTime()
	pending := make(types.Transactions, 0)
	addedTx := make(map[*types.Transaction]struct{})

	// get pending list
	promotableAddresses := pool.promotableQueue.List()

	// loop through pending
	for _, addrInterface := range promotableAddresses {

		if len(pending) >= limit {
			break
		}

		addr := addrInterface.(common.Address)
		txs := pool.pending[addr]

		if len(txs) > 0 {
			// latest txs must be the highest nonce
			// update addressState here
			pool.addressState[addr] = txs[len(txs)-1].Nonce()
			for _, tx := range txs {

				if _, ok := addedTx[tx]; ok {
					continue
				}

				if pool.all.Has(tx.Hash()) {
					continue
				}

				pending = append(pending, tx)
				addedTx[tx] = struct{}{}
			}
			// delete all txs in address if removeResult is true
			if removeResult {
				delete(pool.pending, addr)
				pool.pendingSize -= uint(len(txs))
			}
		}

		// remove addr from queue
		pool.promotableQueue.Remove(addrInterface)
	}
	// sort pending list
	endTime := getTime()
	pool.logger.Error("get pending txs", "txs", len(pending), "total time", endTime-startTime, "limit", limit)
	return pending, nil
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxPool) ValidateTx(tx *types.Transaction) (*common.Address, error) {

	// check sender and duplicated pending tx
	sender, err := pool.getSender(tx)
	if err != nil {
		return nil, err
	}

	if pool.pending[*sender] != nil && uint64(len(pool.pending[*sender])) >= pool.config.GlobalSlots {
		return nil, fmt.Errorf("%v has reached its limit %v/%v", sender.Hex(), len(pool.pending[*sender]), pool.config.GlobalSlots)
	}

	if tx.Nonce() <= pool.addressState[*sender] && pool.addressState[*sender] > 0 {
		return nil, fmt.Errorf("invalid nonce with sender %v %v <= %v", sender.Hex(), tx.Nonce(), pool.addressState[*sender])
	}

	// if tx has been added into db then reject it
	if pool.all.Has(tx.Hash()) {
		return nil, fmt.Errorf("transaction %v existed", tx.Hash().Hex())
	}

	return sender, nil
}

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
//
// If a newly added transaction is marked as local, its sending account will be
// whitelisted, preventing any associated transaction from being dropped out of
// the pool due to pricing constraints.
func (pool *TxPool) add(tx *types.Transaction) (bool, error) {
	if _, err := pool.ValidateTx(tx); err != nil {
		return false, err
	}
	return true, nil
}


// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *TxPool) AddLocal(tx *types.Transaction) error {
	if err := pool.addTx(tx); err != nil {
		return err
	}
	_, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	//pool.all.Add(tx.Hash())
	//pool.pending.Add(tx)
	go pool.txFeed.Send(events.NewTxsEvent{Txs: []*types.Transaction{tx}})
	return nil
}

func (pool *TxPool) getSender(tx *types.Transaction) (*common.Address, error) {
	sender, err := types.Sender(tx)
	if err != nil {
		return nil, ErrInvalidSender
	}
	return &sender, nil
}

// AddRemote enqueues a single transaction into the pool if it is valid. If the
// sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *TxPool) AddRemote(tx *types.Transaction) error {
	if err := pool.addTx(tx); err != nil {
		return err
	}
	_, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	//pool.all.Add(tx.Hash())
	//pool.pending.Add(tx)
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
	pool.addTxs(txs)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx *types.Transaction) error {

	// Try to inject the transaction and update any state
	sender, err := pool.ValidateTx(tx)
	if err != nil {
		return err
	}

	pendingTxs := pool.pending[*sender]
	if pendingTxs == nil {
		pendingTxs = make(types.Transactions, 0)
	}

	pendingTxs = append(pendingTxs, tx)
	sort.Sort(types.TxByNonce(pendingTxs))
	pool.pending[*sender] = pendingTxs

	// add sender to queue if it does not exist in queue
	if !pool.promotableQueue.Has(*sender) {
		pool.promotableQueue.Add(*sender)
	}

	pool.pendingSize++

	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs []interface{}) {
	pool.mu.Lock()
	promoted := make([]*types.Transaction, 0)
	addedTx := make(map[*types.Transaction]struct{})
	for _, txInterface := range txs {
		if txInterface == nil {
			continue
		}
		tx := txInterface.(*types.Transaction)

		if _, ok := addedTx[tx]; ok {
			continue
		}

		// validate and add tx to pool
		if err := pool.addTx(tx); err == nil {
			promoted = append(promoted, tx)
			addedTx[tx] = struct{}{}
		}
	}
	pool.mu.Unlock()

	if len(promoted) > 0 {
		go pool.txFeed.Send(events.NewTxsEvent{Txs: promoted})
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

// RemoveTx removes transactions from pending queue.
// This function is mainly for caller in blockchain/consensus to directly remove committed txs.
//
func (pool *TxPool) RemoveTxs(txs types.Transactions) {

	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.logger.Trace("Removing Txs from pending", "txs", len(txs))
	startTime := getTime()
	for _, tx := range txs {
		sender, _ := pool.getSender(tx)
		pendings := pool.pending[*sender]

		if pendings != nil && len(pendings) > 0 {
			newTxs := make(types.Transactions, 0)
			for _, pending := range pendings {
				if pending.Nonce() != tx.Nonce() {
					newTxs = append(newTxs, pending)
				} else {
					pool.pendingSize -= 1
				}
			}
			pool.pending[*sender] = newTxs
		}
	}
	diff := getTime() - startTime
	pool.logger.Trace("total time to finish removing txs from pending", "time", diff)
}

func (pool *TxPool) cleanUp() {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for addr, txs := range pool.pending {
		newTxs := make(types.Transactions, 0)
		if len(txs) > 0 {
			for _, tx := range txs {
				if !pool.all.Has(tx.Hash()) {
					newTxs = append(newTxs, tx)
				}
			}
		}
		pool.pending[addr] = newTxs
	}
}

func (pool *TxPool) PendingSize() int {
	//return int(pool.pendingSize)
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	count := 0
	for _, txs := range pool.pending {
		count += len(txs)
	}
	return count
}

func (pool *TxPool) GetPendingData() *types.Transactions {
	txs := make(types.Transactions, 0)
	for _, pendings := range pool.pending {
		txs = append(txs, pendings...)
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
