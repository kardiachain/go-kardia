package event_pool

import (
	"fmt"
	"sync"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10

	// promotableQueueSize is the size for promotableQueue
	promotableQueueSize = 1000000
)

// blockChain provides the state of blockchain and current gas limit to do
// some pre checks in tx pool and event subscribers.
type blockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	DB() types.StoreDB
	SubscribeChainHeadEvent(ch chan<- events.ChainHeadEvent) event.Subscription
}

// Config are the configuration parameters of the event pool.
type Config struct {
	GlobalSlots  uint64
	GlobalQueue  uint64
	AccountSlots uint64
	AccountQueue uint64
}

// EventPool contains all currently interesting events from both external or internal blockchains. Events enter the pool
// when dual nodes found events pertaining to what they care about at the moment (e.g. when nodes are listening to
// a transaction relating a specific sender or receiver). They exit the pool when they are included in the blockchain.
type Pool struct {
	logger log.Logger

	chain  blockChain
	config Config

	eventsCh chan []interface{}               // eventsCh is used for pending events
	allCh    chan []interface{}               // allCh is used to cache processed events
	pending  map[common.Hash]*types.DualEvent // current processable events
	all      map[common.Hash]*types.DualEvent // All events

	numberOfWorkers int
	workerCap       int

	chainHeadCh  chan events.ChainHeadEvent
	chainHeadSub event.Subscription
	eventFeed    event.Feed

	mu sync.RWMutex
	wg sync.WaitGroup

	// notify listeners (ie. consensus) when txs are available
	notifiedTxsAvailable bool
	txsAvailable         chan struct{} // fires once for each height, when the mempool is not empty
}

func NewPool(logger log.Logger, config Config, chain blockChain) *Pool {
	pool := &Pool{
		logger:      logger,
		eventsCh:    make(chan []interface{}, 100),
		allCh:       make(chan []interface{}),
		pending:     make(map[common.Hash]*types.DualEvent),
		all:         make(map[common.Hash]*types.DualEvent),
		chainHeadCh: make(chan events.ChainHeadEvent, chainHeadChanSize),
		chain:       chain,
		config:      config,
	}

	pool.reset(nil, chain.CurrentBlock().Header())

	// Subscribe events from dual block chain
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the event pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *Pool) loop() {
	// Track the previous head headers for transaction reorgs
	head := pool.chain.CurrentBlock()
	collectTicker := time.NewTicker(2000 * time.Millisecond)
	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-pool.chainHeadCh:
			go pool.reset(head.Header(), ev.Block.Header())
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return
		case <-collectTicker.C:
			go pool.collectEvents()
		}
	}
}

// NOTE: not thread safe - should only be called once, on startup
func (pool *Pool) EnableTxsAvailable() {
	pool.txsAvailable = make(chan struct{}, 1)
}

func (pool *Pool) TxsAvailable() <-chan struct{} {
	return pool.txsAvailable
}

func (pool *Pool) notifyTxsAvailable() {
	if pool.txsAvailable != nil && !pool.notifiedTxsAvailable {
		// channel cap is 1, so this will send once
		pool.notifiedTxsAvailable = true
		select {
		case pool.txsAvailable <- struct{}{}:
		default:
		}
	}
}

// collectEvents is called periodically to add events from eventsCh to pending pool
func (pool *Pool) collectEvents() {
	for i := 0; i < pool.numberOfWorkers; i++ {
		go pool.work(i, <-pool.eventsCh)
	}
}

// work is called by workers to add txs into pool
func (pool *Pool) work(index int, txs []interface{}) {
	go pool.addEvents(txs)
}

func (pool *Pool) AddEvents(events []interface{}) {
	if len(events) > 0 {
		to := pool.workerCap
		if len(events) < to {
			to = len(events)
		}
		pool.eventsCh <- events[0:to]
		pool.AddEvents(events[to:])
		pool.notifyTxsAvailable()
	}
}

// AddEvent adds a single event into event pool
func (pool *Pool) AddEvent(event *types.DualEvent) error {
	if err := pool.addEvent(event); err != nil {
		return err
	}
	if event.TriggeredEvent.TxSource == types.KARDIA {
		go pool.eventFeed.Send(events.NewDualEventsEvent{Events: []*types.DualEvent{event}})
	}
	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *Pool) addEvents(evts []interface{}) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	internalEvents := make([]*types.DualEvent, 0)
	for _, evtInterface := range evts {
		if evtInterface == nil {
			continue
		}
		evt := evtInterface.(*types.DualEvent)

		// validate and add tx to pool
		if err := pool.addEvent(evt); err != nil {
			pool.logger.Error("error while adding event", "err", err, "event", evt.Hash().Hex())
			continue
		}
		if evt.TriggeredEvent.TxSource == types.KARDIA {
			internalEvents = append(internalEvents, evt)
		}
	}

	// if event is internal then broadcast to all subscribers
	if len(internalEvents) > 0 {
		go pool.eventFeed.Send(events.NewDualEventsEvent{Events: internalEvents})
	}
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *Pool) addEvent(evt *types.DualEvent) error {
	if err := pool.validateEvent(evt); err != nil {
		return err
	}
	pool.pending[evt.TriggeredEvent.TxHash] = evt
	return nil
}

// validateEvent checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *Pool) validateEvent(event *types.DualEvent) error {

	// check sender and duplicated pending tx
	_, err := types.EventSender(event)
	if err != nil {
		return err
	}

	pendingSize := len(pool.pending)
	if uint64(pendingSize) >= pool.config.GlobalSlots {
		return fmt.Errorf("eventPool has reached its limit %v/%v", pendingSize, pool.config.GlobalSlots)
	}

	// if event has been added into memory then reject it
	_, hasAll := pool.all[event.TriggeredEvent.TxHash]
	_, hasPending := pool.pending[event.TriggeredEvent.TxHash]
	if hasAll || hasPending {
		return fmt.Errorf("transaction %v existed", event.TriggeredEvent.TxHash)
	}

	// TODO(kiendn): base on blockNumber check current block height in external chain or internal chain,
	//  if it is less than current blockNumber, return error.

	return nil
}

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
func (pool *Pool) reset(oldHead, newHead *types.Header) {
	pool.notifiedTxsAvailable = false
	// Initialize the internal state to the current head
	currentBlock := pool.chain.CurrentBlock()

	if newHead == nil {
		newHead = currentBlock.Header() // Special case during testing
	}

	// remove current block's txs from pending
	pool.RemoveEvents(currentBlock.DualEvents())
	pool.saveEvents(currentBlock.DualEvents())
}

// saveEvents saves events to all
func (pool *Pool) saveEvents(events types.DualEvents) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(events) > 0 {
		for _, evt := range events {
			pool.all[evt.TriggeredEvent.TxHash] = evt
		}
	}
}

// getTime gets current time in milliseconds
func getTime() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// RemoveEvents removes transactions from pending queue.
func (pool *Pool) RemoveEvents(events types.DualEvents) {

	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(events) == 0 {
		return
	}

	pool.logger.Trace("Removing Txs from pending", "events", len(events))
	startTime := getTime()

	for _, evt := range events {
		delete(pool.pending, evt.TriggeredEvent.TxHash)
	}

	diff := getTime() - startTime
	pool.logger.Trace("total time to finish removing txs from pending", "time", diff)
}

// ProposeEvents collects events from pending and remove them.
func (pool *Pool) ProposeEvents() types.DualEvents {
	des, _ := pool.Pending(true)
	return des
}

// Pending collects pending transactions with limit number, if removeResult is marked to true then remove results after all.
func (pool *Pool) Pending(removeResult bool) (types.DualEvents, error) {

	pool.mu.Lock()
	pending := make(types.DualEvents, 0)
	addedEvents := make(types.DualEvents, 0)

	for _, evt := range pool.pending {
		pending = append(pending, evt)
		if removeResult {
			addedEvents = append(addedEvents, evt)
		}
	}
	pool.mu.Unlock()
	// remove events in pending if addedEvents is not empty
	if len(addedEvents) > 0 {
		pool.RemoveEvents(addedEvents)
	}
	return pending, nil
}

// GetPendingData get all pending data in event pool
func (pool *Pool) GetPendingData() *types.DualEvents {
	evts := make(types.DualEvents, 0)
	for _, pending := range pool.pending {
		evts = append(evts, pending)
	}
	return &evts
}
