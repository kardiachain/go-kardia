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
	"path/filepath"
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

	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")

	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = errors.New("transaction underpriced")

	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")
)

// TxStatus is the current status of a transaction as seen by the pool.
type TxStatus uint

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
	Journal   string        // Journal of local transactions to survive node restarts
	Rejournal time.Duration // Time interval to regenerate the local transaction journal

	PriceLimit uint64 // Minimum gas price to enforce for acceptance into the pool
	PriceBump  uint64 // Minimum price bump percentage to replace an already existing transaction (nonce)

	AccountSlots uint64 // Minimum number of executable transaction slots guaranteed per account
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	AccountQueue uint64 // Maximum number of non-executable transaction slots permitted per account
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts

	Lifetime time.Duration // Maximum amount of time non-executable transaction are queued

	NumberOfWorkers int
	WorkerCap       int
	BlockSize       int
}

// DefaultTxPoolConfig contains the default configurations for the transaction
// pool.
var DefaultTxPoolConfig = TxPoolConfig{
	Journal:   "transactions.rlp",
	Rejournal: time.Hour,

	PriceLimit:   1,
	PriceBump:    10,
	AccountSlots: 16,
	GlobalSlots:  4096,
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime: 3 * time.Hour,
}

// GetDefaultTxPoolConfig returns default txPoolConfig with given dir path
func GetDefaultTxPoolConfig(path string) *TxPoolConfig {
	conf := DefaultTxPoolConfig
	if len(path) > 0 {
		conf.Journal = filepath.Join(path, conf.Journal)
	}
	return &conf
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *TxPoolConfig) sanitize(logger log.Logger) TxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		logger.Warn("Sanitizing invalid txpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	if conf.PriceLimit < 1 {
		logger.Warn("Sanitizing invalid txpool price limit", "provided", conf.PriceLimit, "updated", DefaultTxPoolConfig.PriceLimit)
		conf.PriceLimit = DefaultTxPoolConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		logger.Warn("Sanitizing invalid txpool price bump", "provided", conf.PriceBump, "updated", DefaultTxPoolConfig.PriceBump)
		conf.PriceBump = DefaultTxPoolConfig.PriceBump
	}
	return conf
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
	txsCh       chan []*types.Transaction
	pendingCh   chan map[common.Address][]interface{}
	allCh       chan []interface{}

	chainHeadCh  chan events.ChainHeadEvent
	chainHeadSub event.Subscription
	mu           sync.RWMutex

	numberOfWorkers int
	workerCap       int

	currentState *state.StateDB      // Current state in the blockchain head
	pendingState *state.ManagedState // Pending state tracking virtual nonces

	currentMaxGas uint64 // Current gas limit for transaction caps
	totalPendingGas uint64

	locals  *accountSet // Set of local transaction to exempt from eviction rules
	journal *txJournal  // Journal of local transaction to back up to disk

	pending map[common.Address]*common.Set   // All currently processable transactions
	//queue   map[common.Address]*txList   // Queued but non-processable transactions
	beats   map[common.Address]time.Time   // Last heartbeat from each known account
	all     *common.Set                        // All transactions to allow lookups

	wg sync.WaitGroup // for shutdown sync
}

// NewTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewTxPool(logger log.Logger, config TxPoolConfig, chainconfig *configs.ChainConfig, chain blockChain) *TxPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize(logger)

	// Create the transaction pool with its initial settings
	pool := &TxPool{
		logger:      logger,
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		pending:     make(map[common.Address]*common.Set),
		//queue:       make(map[common.Address]*txList),
		beats:       make(map[common.Address]time.Time),
		all:         common.NewSet(int64(config.GlobalQueue)),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
		gasPrice:    new(big.Int).SetUint64(config.PriceLimit),
		totalPendingGas: uint64(0),
		txsCh: make(chan []*types.Transaction, 100),
		pendingCh: make(chan map[common.Address][]interface{}),
		numberOfWorkers: config.NumberOfWorkers,
		workerCap: config.WorkerCap,
	}
	pool.locals = newAccountSet()
	//pool.priced = newTxPricedList(logger, pool.all)
	pool.reset(nil, chain.CurrentBlock().Header())

	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(logger, config.Journal)

		if err := pool.journal.load(pool.AddLocals); err != nil {
			logger.Warn("Failed to load transaction journal", "err", err)
		}
	}

	// Subscribe events from blockchain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *TxPool) loop() {
	defer pool.wg.Done()

	// Track the previous head headers for transaction reorgs
	head := pool.chain.CurrentBlock()

	// Keep waiting for and reacting to the various events
	for {
		go pool.collectTxs()
		select {
		// Handle ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			pool.wg.Add(1)
			go func() {
				if ev.Block != nil {
					pool.reset(head.Header(), ev.Block.Header())
				}
				pool.wg.Done()
			}()
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return
		case txs := <-pool.pendingCh:
			pool.handlePendingTxs(txs)
		case cachedTxs := <-pool.allCh:
			pool.handleCachingTxs(cachedTxs)
		}
	}
}

func (pool *TxPool) collectTxs() {
	var wg sync.WaitGroup
	for i := 0; i < pool.numberOfWorkers; i++ {
		wg.Add(1)
		go pool.work(i, pool.txsCh, &wg)
	}
}

func (pool *TxPool) work(id int, jobs <-chan []*types.Transaction, wg *sync.WaitGroup) {
	for job := range jobs {
		if errs := pool.AddRemotes(job); errs != nil && len(errs) > 0 {
			for _,err := range errs {
				if err != nil {
					pool.logger.Error("error while add tx to pool", "err", err, "worker", id)
				}
			}
		}
	}
	wg.Done()
}

func (pool *TxPool) AddTxs(txs []*types.Transaction) error {

	if pool.PendingSize() >= int64(pool.config.GlobalSlots) {
		return fmt.Errorf("pool has reached its limit")
	}

	if len(txs) > 0 {
		to := pool.workerCap
		if len(txs) < to {
			to = len(txs)
		}
		pool.txsCh <- txs[0:to]
		go pool.AddTxs(txs[to:])
	}
	return nil
}

func (pool *TxPool) ResetWorker(workers int, cap int) {
	pool.numberOfWorkers = workers
	pool.workerCap = cap
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
	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)
	pool.currentMaxGas = newHead.GasLimit
	pool.mu.Unlock()

	// remove current block's txs from pending
	pool.RemoveTxs(currentBlock.Transactions())
}

// Stop terminates the transaction pool.
func (pool *TxPool) Stop() {
	// Unsubscribe all subscriptions registered from txpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
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

func (pool *TxPool) CurrentState() *state.StateDB {
	return pool.currentState
}

func (pool *TxPool) pendingValidation(tx *types.Transaction) error {
	from, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}
	// Ensure the transaction adheres to nonce ordering
	currentState := pool.currentState
	senderNonce := currentState.GetNonce(from)
	if pool.currentState.GetNonce(from) > tx.Nonce() {
		return fmt.Errorf("nonce too low expected %v found %v", senderNonce, tx.Nonce())
	}
	if pool.currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		pool.logger.Error("Bad txn cost", "balance", pool.currentState.GetBalance(from), "cost", tx.Cost(), "from", from)
		return ErrInsufficientFunds
	}
	// TODO: this can be moved to execute transactions step or may not need
	//readTimeStart := getTime()
	//if t, _, _, _ := chaindb.ReadTransaction(pool.chain.DB(), tx.Hash()); t != nil {
	//	errs[i] =  fmt.Errorf("known transaction: %x", tx.Hash())
	//}
	return nil
}

func (pool *TxPool) ProposeTransactions() types.Transactions {
	txs, _ := pool.Pending(pool.config.BlockSize)
	if err := pool.RemoveTxsFromPending(txs); err != nil {
		pool.logger.Error("cannot remove txs from pending", "err", err)
	}
	return txs
}

func (pool *TxPool) RemoveTxsFromPending(txs types.Transactions) error {
	removedTxs := make(map[common.Address][]interface{})
	for _, tx := range txs {
		addr, err := types.Sender(tx)
		if err != nil {
			return err
		}
		if _, ok := removedTxs[addr]; !ok {
			removedTxs[addr] = make([]interface{}, 0)
		}
		removedTxs[addr] = append(removedTxs[addr], tx)
	}

	for addr, txs := range removedTxs {
		pool.removePending(addr, txs)
	}

	return nil
}

func (pool *TxPool) removePending(addr common.Address, txs []interface{}) {
	pool.mu.Lock()
	if pool.pending[addr] != nil && len(txs) > 0 {
		pool.pending[addr].Remove(txs...)
	}
	pool.mu.Unlock()
}

func getTime() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// Pending retrieves all currently processable transactions, groupped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Pending(limit int) (types.Transactions, error) {
	pending := make(types.Transactions, 0)
	// indexes is found txs indexes in pool.pending
	//pool.mu.Lock()
	//defer pool.mu.Unlock()

	count := 0
	removedHashes := make([]interface{}, 0)

	pool.mu.RLock()
	pendings := pool.pending
	pool.mu.RUnlock()

	for addr, pendingTxs := range pendings {
		if pendingTxs.IsEmpty() {
			continue
		}
		removedPendings := make([]interface{}, 0)
		// txs is a list of valid txs, txs will be sorted after loop
		txs := make(types.Transactions, 0)
		for _, txInterface := range pendingTxs.List() {
			tx := txInterface.(*types.Transaction)
			if err := pool.pendingValidation(tx); err != nil {
				removedHashes = append(removedHashes, tx.Hash())
				removedPendings = append(removedPendings, tx)
			} else {
				txs = append(txs, tx)
				count++
			}
			if limit > 0 && count >= limit {
				break
			}
		}

		if len(txs) > 0 {
			pending = append(pending, txs...)
			// update pending state for address
			pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
		}
		pool.removePending(addr, removedPendings)
	}

	if len(removedHashes) > 0 {
		go pool.all.Remove(removedHashes...)
	}

	if len(pending) > 0 {
		sort.Sort(types.TxByNonce(pending))
	}
	return pending, nil
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxPool) validateTx(tx *types.Transaction, local bool) error {

	if pool.PendingSize() >= int64(pool.config.GlobalSlots) {
		return fmt.Errorf("pool has reached its limit")
	}

	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}
	// Ensure the transaction doesn't exceed the current block limit gas.
	if pool.currentMaxGas < tx.Gas() {
		return ErrGasLimit
	}
	// Make sure the transaction is signed properly
	_, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}
	// Drop non-local transactions under our own minimal accepted gas price
	//local = local || pool.locals.contains(from) // account may be local even if the transaction arrived from the network
	if pool.gasPrice.Cmp(tx.GasPrice()) > 0 {
		return ErrUnderpriced
	}

	if pool.all.Has(tx.Hash()) {
		return fmt.Errorf("known transaction %v", tx.Hash().Hex())
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
	if err := pool.validateTx(tx, local); err != nil {
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
	sender, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	pool.all.Add(tx.Hash())

	pool.mu.RLock()
	_, ok := pool.pending[sender]
	pool.mu.RUnlock()

	if !ok {
		pool.mu.Lock()
		pool.pending[sender] = common.NewSet(0)
		pool.mu.Unlock()
	}

	pool.pending[sender].Add(tx)
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

	sender, err := types.Sender(tx)
	if err != nil {
		return ErrInvalidSender
	}

	pool.all.Add(tx.Hash())

	pool.mu.RLock()
	_, ok := pool.pending[sender]
	pool.mu.RUnlock()

	if !ok {
		pool.mu.Lock()
		pool.pending[sender] = common.NewSet(0)
		pool.mu.Unlock()
	}
	pool.pending[sender].Add(tx)
	go pool.txFeed.Send(events.NewTxsEvent{Txs: []*types.Transaction{tx}})
	return nil
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *TxPool) AddLocals(txs []*types.Transaction) []error {
	return pool.addTxs(txs, !pool.config.NoLocals)
}

// AddRemotes enqueues a batch of transactions into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *TxPool) AddRemotes(txs []*types.Transaction) []error {
	return pool.addTxs(txs, false)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx *types.Transaction, local bool) error {

	// Try to inject the transaction and update any state
	if err := pool.validateTx(tx, local); err != nil {
		pool.logger.Trace("Discarding invalid transaction", "hash", tx.Hash().Hex(), "err", err)
		return err
	}

	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs []*types.Transaction, local bool) []error {
	//pool.mu.Lock()
	//defer pool.mu.Unlock()

	errs := make([]error, len(txs))
	promoted := make([]*types.Transaction, 0)
	pendings := make(map[common.Address][]interface{})
	hashes := make([]interface{}, 0)

	for i, tx := range txs {
		if tx == nil {
			continue
		}
		errs[i] = pool.addTx(tx, local)
		if errs[i] == nil {
			promoted = append(promoted, tx)
			from, _ := types.Sender(tx)
			if _, ok := pendings[from]; !ok {
				pendings[from] = make([]interface{}, 0)
			}
			pendings[from] = append(pendings[from], tx)
			hashes = append(hashes, tx.Hash())
		}
	}

	if len(promoted) > 0 {
		go pool.txFeed.Send(events.NewTxsEvent{Txs: promoted})
		pool.pendingCh <- pendings
		pool.allCh <- hashes
	}
	return errs
}


func (pool *TxPool) handleCachingTxs(hashes []interface{}) {
	pool.all.Add(hashes...)
}


func (pool *TxPool) handlePendingTxs(txs map[common.Address][]interface{}) {
	for addr, txs := range txs {

		pool.mu.RLock()
		_, ok := pool.pending[addr]
		pool.mu.RUnlock()

		if !ok {
			pool.mu.Lock()
			pool.pending[addr] = common.NewSet(0)
			pool.mu.Unlock()
		}
		pool.pending[addr].Add(txs...)
	}
}

// RemoveTx removes transactions from pending queue.
// This function is mainly for caller in blockchain/consensus to directly remove committed txs.
//
func (pool *TxPool) RemoveTxs(txs types.Transactions) {
	pool.logger.Trace("Removing Txs from pending", "txs", len(txs))
	startTime := getTime()
	if err := pool.RemoveTxsFromPending(txs); err != nil {
		pool.logger.Error("error while trying remove pending Txs", "err", err)
	}
	diff := getTime() - startTime
	pool.logger.Trace("total time to finish removing txs from pending", "time", diff)
}

func (pool *TxPool) PendingSize() int64 {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	results := 0
	for _, txs := range pool.pending {
		results += txs.Size()
	}

	return int64(results)
}

func (pool *TxPool) GetPendingData() *types.Transactions {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	txs := make(types.Transactions, 0)
	for _, txsInterface := range pool.pending {
		for _, txInterface := range txsInterface.List() {
			txs = append(txs, txInterface.(*types.Transaction))
		}
	}

	return &txs
}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addresssByHeartbeat []addressByHeartbeat

func (a addresssByHeartbeat) Len() int           { return len(a) }
func (a addresssByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addresssByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence
type accountSet struct {
	accounts map[common.Address]struct{}
}

// newAccountSet creates a new address set
func newAccountSet() *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
	}
}

// contains checks if a given address is contained within the set.
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

// containsTx checks if the sender of a given tx is within the set. If the sender
// cannot be derived, this method returns false.
func (as *accountSet) containsTx(tx *types.Transaction) bool {
	if addr, err := types.Sender(tx); err == nil {
		return as.contains(addr)
	}
	return false
}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
}

// txLookup is used internally by TxPool to track transactions while allowing lookup without
// mutex contention.
//
// Note, although this type is properly protected against concurrent access, it
// is **not** a type that should ever be mutated or even exposed outside of the
// transaction pool, since its internal state is tightly coupled with the pools
// internal mechanisms. The sole purpose of the type is to permit out-of-bound
// peeking into the pool in TxPool.Get without having to acquire the widely scoped
// TxPool.mu mutex.
type txLookup struct {
	limit int
	all  map[common.Hash]*types.Transaction
	heap txLookupHeap
	lock sync.RWMutex
}

// newTxLookup returns a new txLookup structure.
func newTxLookup(limit int) *txLookup {
	return &txLookup{
		all: make(map[common.Hash]*types.Transaction),
		heap: make(txLookupHeap, 0),
		limit: limit,
	}
}

// Range calls f on each key and value present in the map.
func (t *txLookup) Range(f func(hash common.Hash, tx *types.Transaction) bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for key, value := range t.all {
		if !f(key, value) {
			break
		}
	}
}

// Get returns a transaction if it exists in the lookup, or nil if not found.
func (t *txLookup) Get(hash common.Hash) *types.Transaction {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.all[hash]
}

// Count returns the current number of items in the lookup.
func (t *txLookup) Count() int {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return len(t.all)
}

// Add adds a transaction to the lookup.
func (t *txLookup) Add(tx *types.Transaction) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if _, ok := t.all[tx.Hash()]; !ok {
		t.all[tx.Hash()] = tx
		t.heap.Push(tx.Hash())

		// loop until heap <= limit
		for {
			if len(t.heap) <= t.limit {
				break
			}

			txHash := t.heap.Pop().(common.Hash)
			delete(t.all, txHash)
		}
	}
}

// Remove removes a transaction from the lookup.
func (t *txLookup) Remove(hash common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()

	delete(t.all, hash)
	for i, txHash := range t.heap {
		if txHash == hash {
			newHeap := make(txLookupHeap, 0)
			if i < len(t.heap) - 1 {
				newHeap = t.heap[0:i]
				newHeap = append(newHeap, t.heap[i+1:len(t.heap)]...)
			} else {
				newHeap = t.heap[0:len(t.heap) - 1]
			}
			t.heap = newHeap
			break
		}
	}
}

type txLookupHeap []common.Hash

func (h txLookupHeap) Len() int      { return len(h) }
func (h txLookupHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *txLookupHeap) Push(x interface{}) {
	*h = append(*h, x.(common.Hash))
}

func (h *txLookupHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
