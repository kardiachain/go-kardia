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
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"

	"github.com/kardiachain/go-kardia/common/chaindb"
	kaidb "github.com/kardiachain/go-kardia/common/storage"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/types"
)

const (
	DualStateAddressHex = "68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
)

var (
	evictionInterval    = time.Hour       // Time interval to check for evictable events
	statsReportInterval = 8 * time.Second // Time interval to report event pool stats
	dualStateAddress    = common.HexToAddress(DualStateAddressHex)
)

var (
	// ErrPoolFull is returned when the event pool is full and can't accept new dual's event.
	ErrPoolFull = errors.New("discard new event due to pool full")

	// ErrAddExistingEvent is returned if a dual's event already exists in the event pool.
	ErrAddExistingEvent = errors.New("adding event was previously inserted")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")
)

var (
	// Metrics for the pending pool
	pendingDiscardCounter = metrics.NewRegisteredCounter("eventpool/pending/discard", nil)

	// Metrics for the queued pool
	queuedDiscardCounter   = metrics.NewRegisteredCounter("eventpool/queued/discard", nil)
	queuedRateLimitCounter = metrics.NewRegisteredCounter("eventpool/queued/ratelimit", nil) // Dropped due to rate limiting

	// General dual's event metrics
	invalidEventCounter         = metrics.NewRegisteredCounter("eventpool/invalid", nil)
	discardEventFullPoolCounter = metrics.NewRegisteredCounter("eventpool/fullpool", nil)
)

// EventStatus is the current status of a event as seen by the pool.
type EventStatus uint

const (
	EventStatusUnknown EventStatus = iota
	EventStatusQueued
	EventStatusPending
)

// blockChain provides the state of blockchain and current gas limit to do
// some pre checks in dual's event pool and event subscribers.
type blockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)
	DB() kaidb.Database
	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
	StoreHash(hash *common.Hash)
	CheckHash(hash *common.Hash) bool
	StoreTxHash(hash *common.Hash)
	CheckTxHash(hash *common.Hash) bool
}

// EventPoolConfig are the configuration parameters of the event pool.
type EventPoolConfig struct {
	Journal   string        // Journal of dual's event to survive node restarts
	Rejournal time.Duration // Time interval to regenerate the dual's event journal

	QueueSize int           // Maximum number of queued events
	Lifetime  time.Duration // Maximum amount of time events are queued
}

// DefaultEventPoolConfig contains the default configurations for the event
// pool.
var DefaultEventPoolConfig = EventPoolConfig{
	Journal:   "dual_events.rlp",
	Rejournal: time.Hour,

	QueueSize: 4096,
	Lifetime:  3 * time.Hour,
}

// GetEventPoolConfig returns default eventPoolConfig with given dir path
func GetDefaultEventPoolConfig(path string) *EventPoolConfig {
	conf := DefaultEventPoolConfig
	if len(path) > 0 {
		conf.Journal = filepath.Join(path, conf.Journal)
	}
	return &conf
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *EventPoolConfig) sanitize(logger log.Logger) EventPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		logger.Warn("Sanitizing invalid eventpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	return conf
}

// EventPool contains all currently interesting events from both external or internal blockchains. Events enter the pool
// when dual nodes found events pertaining to what they care about at the moment (e.g. when nodes are listening to
// a transaction relating a specific sender or receiver). They exit the pool when they are included in the blockchain.
type EventPool struct {
	logger log.Logger

	config        EventPoolConfig
	chainconfig   *configs.ChainConfig
	chain         blockChain
	dualEventFeed event.Feed
	scope         event.SubscriptionScope
	chainHeadCh   chan ChainHeadEvent
	chainHeadSub  event.Subscription
	mu            sync.RWMutex

	currentState *state.StateDB      // Current state in the blockchain head
	pendingState *state.ManagedState // Pending state tracking virtual nonces

	journal *eventJournal // Journal of events to back up to disk

	pending *eventList   // All currently processable dual's events
	queue   *eventList   // Queued but non-processable dual's events
	all     *eventLookup // All dual's events to allow lookups

	wg sync.WaitGroup // for shutdown sync
}

// Creates a new dual's event pool to gather, sort and filter inbound
// dual's events from other blockchains.
func NewEventPool(logger log.Logger, config EventPoolConfig, chainconfig *configs.ChainConfig, chain blockChain) *EventPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize(logger)

	// Create the dual's event pool with its initial settings
	pool := &EventPool{
		logger:      logger,
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		pending:     newEventList(),
		queue:       newEventList(),
		all:         newEventLookup(),
		chainHeadCh: make(chan ChainHeadEvent, chainHeadChanSize),
	}
	pool.reset(nil, chain.CurrentBlock().Header())

	if config.Journal != "" {
		pool.journal = newEventJournal(logger, config.Journal)

		if err := pool.journal.load(pool.AddEvents); err != nil {
			logger.Warn("Failed to load dual's event journal", "err", err)
		}
		if err := pool.journal.rotate(pool.getEvents()); err != nil {
			logger.Warn("Failed to rotate dual's event journal", "err", err)
		}
	}

	// Subscribe events from blockchain
	// TODO(namdoh@, #115): Move subsribing to external blockchains (e.g. Ether, Neo) or
	// Kardia's blockchain here.
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the dual's event pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and dual's event
// eviction events.
func (pool *EventPool) loop() {
	defer pool.wg.Done()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// Track the previous head headers for dual's events reorgs
	head := pool.chain.CurrentBlock()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			if ev.Block != nil {
				pool.mu.Lock()
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block

				pool.mu.Unlock()
			}
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return

			/*@huny
			// Handle stats reporting ticks
			case <-report.C:
				pool.mu.RLock()
				pending, queued := pool.stats()
				stales := pool.priced.stales
				pool.mu.RUnlock()

				if pending != prevPending || queued != prevQueued || stales != prevStales {
					pool.logger.Debug("Transaction pool status report", "executable", pending, "queued", queued, "stales", stales)
					prevPending, prevQueued, prevStales = pending, queued, stales
				}
			*/

		// Handle inactive account dual's event eviction
		case <-evict.C:
			// TODO(namdoh): Implement eviction policy here.

		// Handle dual's event journal rotation
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.getEvents()); err != nil {
					pool.logger.Warn("Failed to rotate event journal", "err", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}

// lockedReset is a wrapper around reset to allow calling it in a thread safe
// manner. This method is only ever used in the tester!
func (pool *EventPool) lockedReset(oldHead, newHead *types.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(oldHead, newHead)
}

// reset retrieves the current state of the blockchain and ensures the content
// of the dual's event pool is valid with regard to the chain state.
func (pool *EventPool) reset(oldHead, newHead *types.Header) {
	// Note: Disables feature of recreate dropped transaction, will evaluate this for mainnet.
	/*
		// If we're reorging an old state, reinject all dropped transactions
		var reinject types.Transactions

			if oldHead != nil && oldHead.Hash() != newHead.LastCommitHash {
				// If the reorg is too deep, avoid doing it (will happen during fast sync)
				oldHeight := oldHead.Height
				newHeight := newHead.Height

				if depth := uint64(math.Abs(float64(oldHeight) - float64(newHeight))); depth > 64 {
					pool.logger.Debug("Skipping deep transaction reorg", "depth", depth)
				} else {
					// Reorg seems shallow enough to pull in all transactions into memory
					var discarded, included types.Transactions

					var (
						rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Height)
						add = pool.chain.GetBlock(newHead.Hash(), newHead.Height)
					)
					for rem.Height() > add.Height() {
						discarded = append(discarded, rem.Transactions()...)
						if rem = pool.chain.GetBlock(rem.LastCommitHash(), rem.Height()-1); rem == nil {
							pool.logger.Error("Unrooted old chain seen by tx pool", "block", oldHead.Height, "hash", oldHead.Hash())
							return
						}
					}
					for add.Height() > rem.Height() {
						included = append(included, add.Transactions()...)
						if add = pool.chain.GetBlock(add.LastCommitHash(), add.Height()-1); add == nil {
							pool.logger.Error("Unrooted new chain seen by tx pool", "block", newHead.Height, "hash", newHead.Hash())
							return
						}
					}
					for rem.Hash() != add.Hash() {
						discarded = append(discarded, rem.Transactions()...)
						if rem = pool.chain.GetBlock(rem.LastCommitHash(), rem.Height()-1); rem == nil {
							pool.logger.Error("Unrooted old chain seen by tx pool", "block", oldHead.Height, "hash", oldHead.Hash())
							return
						}
						included = append(included, add.Transactions()...)
						if add = pool.chain.GetBlock(add.LastCommitHash(), add.Height()-1); add == nil {
							pool.logger.Error("Unrooted new chain seen by tx pool", "block", newHead.Height, "hash", newHead.Hash())
							return
						}
					}
					reinject = types.TxDifference(discarded, included)
				}
			}
	*/

	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // Special case during testing
	}

	statedb, err := pool.chain.StateAt(newHead.Root)
	pool.logger.Info("EventPool reset state to new head block", "height", newHead.Height, "root", newHead.Root)
	if err != nil {
		pool.logger.Error("Failed to reset eventPool state", "err", err)
		return
	}

	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)

	// Inject any transactions discarded due to reorgs
	//pool.logger.Debug("Reinjecting stale transactions", "count", len(reinject))
	//senderCacher.recover(reinject)
	//pool.addTxsLocked(reinject, false)

	// validate the pool of pending dual's events, this will remove
	// any events that have been included in the block or
	// have been invalidated because of another events.
	pool.demoteUnexecutables()

	// Update to the latest known pending nonce
	events := pool.pending.Flatten() // Heavy but will be cached and is needed by the miner anyway
	if len(events) > 0 {
		pool.currentState.SetNonce(dualStateAddress, events[len(events)-1].Nonce+1)
	}

	// Check the queue and move dual's events over to the pending if possible
	// or remove those that have become invalid
	pool.promoteExecutables()
}

// Terminates the dual's event pool.
func (pool *EventPool) Stop() {
	// Unsubscribe all subscriptions registered from txpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	pool.logger.Info("Event pool stopped")
}

// Registers a subscription of NewTxsEvent and
// starts sending event to the given channel.
func (pool *EventPool) SubscribeNewTxsEvent(ch chan<- NewDualEventsEvent) event.Subscription {
	return pool.scope.Track(pool.dualEventFeed.Subscribe(ch))
}

// State returns the virtual managed state of the event pool.
func (pool *EventPool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

/*
// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}
*/

// Retrieves the data content of the dual's event pool, returning all the
// pending as well as queued dual's events, sorted by nonce.
func (pool *EventPool) Content() (types.DualEvents, types.DualEvents) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := pool.pending.Flatten()
	queued := pool.queue.Flatten()
	return pending, queued
}

// Retrieves all currently processable dual's events, sorted by nonce. The returned dual's event set is a copy and
// can be freely modified by calling code.
func (pool *EventPool) Pending() (types.DualEvents, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := pool.pending.Flatten()
	return pending, nil
}

// Retrieves all currently known dual's events, sorted by nonce. The returned dual's event set is a copy and
// can be freely modified by calling code.
func (pool *EventPool) getEvents() types.DualEvents {
	events := pool.pending.Flatten()
	events = append(events, pool.queue.Flatten()...)
	return events
}

// validateTx checks whether a dual's event is valid according to the consensus
// rules.
func (pool *EventPool) validateTx(tx *types.DualEvent) error {
	pool.logger.Error("Not yet implement EventPool.validateTx()")
	return nil
	/*
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
		from, err := types.Sender(tx)
		if err != nil {
			return ErrInvalidSender
		}
		// Drop non-local transactions under our own minimal accepted gas price
		local = local || pool.locals.contains(from) // account may be local even if the transaction arrived from the network
		if !local && pool.gasPrice.Cmp(tx.GasPrice()) > 0 {
			return ErrUnderpriced
		}

		// Ensure the transaction adheres to nonce ordering
		if pool.currentState.GetNonce(from) > tx.Nonce() {
			return ErrNonceTooLow
		}

		// Transactor should have enough funds to cover the costs
		// cost == V + GP * GL
		if pool.currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
			pool.logger.Error("Bad txn cost", "balance", pool.currentState.GetBalance(from), "cost", tx.Cost(), "from", from)
			return ErrInsufficientFunds
		}

		return nil
	*/
}

// Adds validates a dual's event and inserts it into the non-executable queue for
// later pending promotion and execution. If the event is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
func (pool *EventPool) add(event *types.DualEvent) (bool, error) {
	// If the dual's event is already known, discard it
	hash := event.Hash()

	// If dual's event exists in pool or DB, discard it
	if e, _, _, _ := chaindb.ReadDualEvent(pool.chain.DB(), hash); pool.all.Get(hash) != nil || e != nil {
		pool.logger.Trace("Discarding already known dual's event", "hash", hash)
		return false, fmt.Errorf("known dual's event: %x", hash)
	}
	// If the dual's event fails basic validation, discard it
	if err := pool.validateTx(event); err != nil {
		pool.logger.Trace("Discarding invalid dual's event", "hash", hash, "err", err)
		invalidEventCounter.Inc(1)
		return false, err
	}
	// If the event pool is full, discard the new dual's event
	if pool.all.Count() >= pool.config.QueueSize {
		pool.logger.Error("EventPool is full. Discard incoming events.")
		discardEventFullPoolCounter.Inc(1)
		return false, ErrPoolFull
	}
	// If the dual's event is replacing an already pending one, do directly
	if pool.pending.Overlaps(event) {
		pendingDiscardCounter.Inc(1)
		return false, ErrAddExistingEvent
	}
	// New dual's event hasn't pushed yet, push into queue
	replace, err := pool.enqueueEvent(hash, event)
	if err != nil {
		return false, err
	}

	pool.journalEvent(event)

	pool.logger.Trace("Pooled new future dual's event", "hash", hash)
	return replace, nil
}

// Inserts a new dual' event into the non-executable event queue.
//
// Note, this method assumes the pool lock is held!
func (pool *EventPool) enqueueEvent(hash common.Hash, event *types.DualEvent) (bool, error) {
	inserted := pool.queue.Add(event)
	if !inserted {
		// An older dual's event was better, discard this
		queuedDiscardCounter.Inc(1)
		return false, ErrAddExistingEvent
	}
	if pool.all.Get(hash) == nil {
		pool.all.Add(event)
	}
	return true, nil
}

// Adds the specified dual's event to the disk journal.
func (pool *EventPool) journalEvent(event *types.DualEvent) {
	// Only journal if it's enabled
	if pool.journal == nil {
		return
	}
	if err := pool.journal.insert(event); err != nil {
		pool.logger.Warn("Failed to journal dual's event", "err", err)
	}
}

// Adds a dual's event to the pending (processable) list of transactions
// and returns whether it was inserted or an older was better.
//
// Note, this method assumes the pool lock is held!
func (pool *EventPool) promoteEvent(hash common.Hash, event *types.DualEvent) bool {
	// Try to insert the dual's event into the pending queue
	inserted := pool.pending.Add(event)
	if !inserted {
		// An older dual's event was better, discard this
		pool.all.Remove(hash)

		pendingDiscardCounter.Inc(1)
		return false
	}
	// Failsafe to work around direct pending inserts (tests)
	if pool.all.Get(hash) == nil {
		pool.all.Add(event)
	}
	// Set the potentially new pending nonce and notify any subsystems of the new dual's event
	pool.pendingState.SetNonce(dualStateAddress, event.Nonce+1)

	return true
}

// Enqueues a single dual's event into the pool if it is valid.
func (pool *EventPool) AddEvent(event *types.DualEvent) error {
	return pool.addEvent(event)
}

// Enqueues a batch of dual's events into the pool if they are valid.
func (pool *EventPool) AddEvents(events []*types.DualEvent) []error {
	return pool.addEvents(events)
}

// Enqueues a single dua's event into the pool if it is valid.
func (pool *EventPool) addEvent(event *types.DualEvent) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.addEventLocked(event)
	pool.promoteExecutables()

	return nil
}

// Attempts to queue a batch of dual's events if they are valid.
func (pool *EventPool) addEvents(events []*types.DualEvent) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addEventsLocked(events)
}

// Attempts to queue a batch of dual's events if they are valid,
// whilst assuming the event pool lock is already held.
func (pool *EventPool) addEventsLocked(events []*types.DualEvent) []error {
	// Add the batch of dual's events, tracking the accepted ones
	errs := make([]error, len(events))

	for i, event := range events {
		errs[i] = pool.addEventLocked(event)
	}
	pool.promoteExecutables()

	return errs
}

// Attempts to queue a dual's event if they are valid, whilst assuming the event pool lock is
// already held.
func (pool *EventPool) addEventLocked(event *types.DualEvent) error {
	eventHash := event.TriggeredEvent.TxHash
	if pool.chain.CheckHash(&eventHash) {
		// TODO(#121): Consider removing this error when we move to beta net.
		pool.logger.Error("Attempting to add a dual's event that was previously added to EventPool. Abort adding event.", "event", event)
		return ErrAddExistingEvent
	}

	// Try to inject the dual's event and update any state
	_, err := pool.add(event)
	if err != nil {
		return err
	}
	pool.chain.StoreHash(&eventHash)
	// TODO(namdoh@): Consider storing this under a different key (from the event hash) to avoid
	// collision.
	if !event.PendingTx.TxHash.IsZero() {
		pool.chain.StoreTxHash(&event.PendingTx.TxHash)
	}
	return nil
}

// Status returns the status (unknown/pending/queued) of a batch of dual's events
// identified by their hashes.
func (pool *EventPool) Status(hashes []common.Hash) []EventStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]EventStatus, len(hashes))
	for i, hash := range hashes {
		if event := pool.all.Get(hash); event != nil {
			if pool.pending.events.items[event.Nonce] != nil {
				status[i] = EventStatusPending
			} else {
				status[i] = EventStatusQueued
			}
		}
	}
	return status
}

// Returns a dual's event if it is contained in the pool
// and nil otherwise.
func (pool *EventPool) Get(hash common.Hash) *types.DualEvent {
	return pool.all.Get(hash)
}

// Removes dual's events from pending queue.
// This function is mainly for caller in blockchain/consensus to directly remove committed events.
//
func (pool *EventPool) RemoveEvents(events types.DualEvents) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for _, event := range events {
		pool.all.Remove(event.Hash())
	}
}

// Removes a single dual's event from the queue, moving all subsequent
// dual's events back to the future queue. Pool pendingState is also reset.
// Caller is assumed to hold pool.mu.Lock()
func (pool *EventPool) removeEventInternal(hash common.Hash) {
	// Fetch the dual's event we wish to delete
	event := pool.all.Get(hash)
	if event == nil {
		return
	}

	// Remove it from the list of known dual's events
	pool.all.Remove(hash)
	// Remove the dual's event from the pending lists and reset the account nonce
	if removed := pool.pending.Remove(event); removed {
		// If no more pending dual's events are left, remove the list
		if pool.pending.Empty() {
			pool.pending = nil
		}

		// Update the account nonce if needed
		if pool.pendingState.GetNonce(dualStateAddress) > event.Nonce {
			pool.pendingState.SetNonce(dualStateAddress, event.Nonce)
		}

		return
	}
	// Dual's event is in the future queue
	pool.queue.Remove(event)
}

// Moves dual's events that have become processable from the
// future queue to the set of pending dual's events. During this process, all
// invalidated dual's events (low nonce) are deleted.
func (pool *EventPool) promoteExecutables() {
	// Track the promoted transactions to broadcast them at once
	var promoted []*types.DualEvent

	// Drop all dual's events that are deemed too old (low nonce)
	for _, event := range pool.queue.Forward(pool.currentState.GetNonce(dualStateAddress)) {
		hash := event.Hash()
		pool.logger.Info("Removed old queued dual's event", "hash", hash)
		pool.all.Remove(hash)
	}

	// Gather all executable dual's events and promote them
	for _, event := range pool.queue.Ready(pool.pendingState.GetNonce(dualStateAddress)) {
		hash := event.Hash()
		pool.logger.Info("Promoting event", "event", event)
		if pool.promoteEvent(hash, event) {
			pool.logger.Info("Promoting queued dual's event", "hash", hash)
			promoted = append(promoted, event)
		} else {
			pool.logger.Error("Fail to promote event", "event", event)
		}
	}

	// Notify subsystem for new promoted dual's events.
	if len(promoted) > 0 {
		go pool.dualEventFeed.Send(NewDualEventsEvent{promoted})
	}
	// If we've queued more dual's events than the hard limit, drop oldest ones
	if pool.queue.Len() > pool.config.QueueSize {
		// Drop dual's events until the total is below the limit
		drop := pool.queue.Len() - pool.config.QueueSize
		events := pool.queue.Flatten()
		for i := len(events) - 1; i >= 0 && drop > 0; i-- {
			pool.removeEventInternal(events[i].Hash())
			drop--
			queuedRateLimitCounter.Inc(1)
		}
	}
}

// Removes invalid and processed dual's events from the pools
// executable/pending queue and any subsequent dual's events that become unexecutable
// are moved back into the future queue.
func (pool *EventPool) demoteUnexecutables() {
	nonce := pool.currentState.GetNonce(dualStateAddress)

	// TODO(thientn): Evaluate this for future phases.
	// These txs should also dropped by below loop because of low nonce.
	// Drop transactions included in latest block, assume it's committed and saved.
	// This function is only called when TxPool detect new height.
	for _, event := range pool.chain.CurrentBlock().DualEvents() {
		hash := event.Hash()
		pool.logger.Info("EventPool to remove committed dual's event", "hash", hash.Fingerprint())
		pool.all.Remove(hash)
	}

	// Drop all transactions that are deemed too old (low nonce)
	for _, event := range pool.pending.Forward(nonce) {
		hash := event.Hash()
		pool.logger.Info("Removed old pending dual's event", "hash", hash)
		pool.all.Remove(hash)
	}
	// TODO(thientn): Evaluates enable this.
	/*
		// Drop all transactions that are too costly (low balance or out of gas), and queue any invalids back for later
		drops, invalids := list.Filter(pool.currentState.GetBalance(addr), pool.currentMaxGas)
		for _, tx := range drops {
			hash := tx.Hash()
			pool.logger.Info("Removed unpayable pending transaction", "hash", hash)
			pool.all.Remove(hash)
			pool.priced.Removed()
			pendingNofundsCounter.Inc(1)
		}
		for _, tx := range invalids {
			hash := tx.Hash()
			pool.logger.Info("Demoting pending transaction", "hash", hash)
			pool.enqueueTx(hash, tx)
		}
	*/

	// If there's a gap in front, alert (should never happen) and postpone all transactions
	if pool.pending.Len() > 0 && pool.pending.events.Get(nonce) == nil {
		for _, event := range pool.pending.Cap(0) {
			hash := event.Hash()
			pool.logger.Error("Demoting invalidated dual's event", "hash", hash)
			pool.enqueueEvent(hash, event)
		}
	}
}

// eventLookup is used internally by EventPool to track dual's events while allowing lookup without
// mutex contention.
//
// Note, although this type is properly protected against concurrent access, it
// is **not** a type that should ever be mutated or even exposed outside of the
// event pool, since its internal state is tightly coupled with the pools
// internal mechanisms. The sole purpose of the type is to permit out-of-bound
// peeking into the pool in EventPool.Get without having to acquire the widely scoped
// EventPool.mu mutex.
type eventLookup struct {
	all  map[common.Hash]*types.DualEvent
	lock sync.RWMutex
}

// Returns a new eventLookup structure.
func newEventLookup() *eventLookup {
	return &eventLookup{
		all: make(map[common.Hash]*types.DualEvent),
	}
}

// Range calls f on each key and value present in the map.
func (e *eventLookup) Range(f func(hash common.Hash, event *types.DualEvent) bool) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	for key, value := range e.all {
		if !f(key, value) {
			break
		}
	}
}

// Get returns a dual'event if it exists in the lookup, or nil if not found.
func (e *eventLookup) Get(hash common.Hash) *types.DualEvent {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.all[hash]
}

// Count returns the current number of items in the lookup.
func (e *eventLookup) Count() int {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return len(e.all)
}

// Add adds a dual's event to the lookup.
func (e *eventLookup) Add(event *types.DualEvent) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.all[event.Hash()] = event
}

// Remove removes a dual's event from the lookup.
func (e *eventLookup) Remove(hash common.Hash) {
	e.lock.Lock()
	defer e.lock.Unlock()

	delete(e.all, hash)
}
