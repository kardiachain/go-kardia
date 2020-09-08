package channel

import (
	"sync"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
)

type ChainHeadEventChannel struct {
	blockchain *blockchain.BlockChain

	mtx           sync.RWMutex
	chainHeadChan chan events.ChainHeadEvent
	chainHeadSub  event.Subscription

	checkpoint uint64
}

func newChainHeadEventChannel(bc *blockchain.BlockChain, currentHeight uint64) *ChainHeadEventChannel {
	chann := ChainHeadEventChannel{
		blockchain:    bc,
		chainHeadChan: make(chan events.ChainHeadEvent, chainHeadChanSize),
		checkpoint:    currentHeight,
	}
	chann.chainHeadSub = bc.SubscribeChainHeadEvent(chann.chainHeadChan)
	return &chann
}

func (c *ChainHeadEventChannel) readCurrentEventsInChannel() events.ChainHeadEvent {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return <-c.chainHeadChan
}

// loop is the main event loop, waiting for and reacting to chainHeadEvents
func (c *ChainHeadEventChannel) loop() {
	// Track the previous head headers for transaction reorgs
	head := c.chain.CurrentBlock()
	collectTicker := time.NewTicker(2000 * time.Millisecond)
	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-c.chainHeadCh:
			go c.reset(head.Header(), ev.Block.Header())
		// Be unsubscribed due to system stopped
		case <-c.chainHeadSub.Err():
			return
		case <-collectTicker.C:
			go c.collectEvents()
		}
	}
}

// func (c *ChainHeadEventChannel) stop() {

// }

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
// func (c *ChainHeadEventChanne) reset(oldHead, newHead *types.Header) {
// 	// Initialize the internal state to the current head
// 	currentBlock := pool.chain.CurrentBlock()

// 	if newHead == nil {
// 		newHead = currentBlock.Header() // Special case during testing
// 	}

// 	// remove current block's txs from pending
// 	pool.RemoveEvents(currentBlock.DualEvents())
// 	pool.saveEvents(currentBlock.DualEvents())
// }
