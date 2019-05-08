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

package neo

import (
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode/utils"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/kai/events"
)

const ServiceName = "NEO"

// NeoProxy provides interfaces for Neo Dual node, responsible for detecting updates
// that relates to NEO from Kardia, sending tx to NEO network and check tx status
type Proxy struct {

	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH)
	name   string

	logger log.Logger // Logger for Tron service

	kardiaBc   base.BaseBlockChain
	txPool     *tx_pool.TxPool
	smcAddress *common.Address
	smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool // Event pool of DUAL service.

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// Chain head subscription for new blocks.
	chainHeadCh  chan events.ChainHeadEvent
	chainHeadSub event.Subscription

	// Queue configuration
	publishedEndpoint string
	subscribedEndpoint string
	queueTopic string
}

func NewProxy(
	kardiaBc base.BaseBlockChain,
	txPool *tx_pool.TxPool,
	dualBc base.BaseBlockChain,
	dualEventPool *event_pool.EventPool,
	publishedEndpoint string,
	subscribedEndpoint string,
) (*Proxy, error) {

	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(ServiceName)

	processor := &Proxy{
		name: configs.NEO,
		logger: logger,
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,

		chainHeadCh: make(chan events.ChainHeadEvent, 5),
		queueTopic: ServiceName,
	}

	processor.publishedEndpoint = publishedEndpoint
	if publishedEndpoint == "" {
		processor.publishedEndpoint = configs.DefaultPublishedEndpoint
	}

	processor.subscribedEndpoint = subscribedEndpoint
	if subscribedEndpoint == "" {
		processor.subscribedEndpoint = configs.DefaultSubscribedEndpoint
	}

	return processor, nil
}


// PublishedEndpoint returns publishedEndpoint
func (p *Proxy) PublishedEndpoint() string {
	return p.publishedEndpoint
}

// SubscribedEndpoint returns subscribedEndpoint
func (p *Proxy) SubscribedEndpoint() string {
	return p.subscribedEndpoint
}

// InternalChain returns internalChain which is internal proxy (eg:kardiaProxy)
func (p *Proxy) InternalChain() base.BlockChainAdapter {
	return p.internalChain
}

func (p *Proxy) ExternalChain() base.BlockChainAdapter {
	return nil
}

// DualEventPool returns dual's eventPool
func (p *Proxy) DualEventPool() *event_pool.EventPool {
	return p.eventPool
}

// DualBlockChain returns dual blockchain
func (p *Proxy) DualBlockChain() base.BaseBlockChain {
	return p.dualBc
}

// KardiaBlockChain returns kardia blockchain
func (p *Proxy) KardiaBlockChain() base.BaseBlockChain {
	return p.kardiaBc
}

// KardiaTxPool returns Kardia Blockchain's tx pool
func (p *Proxy) KardiaTxPool() *tx_pool.TxPool {
	return p.txPool
}

func (p *Proxy) Logger() log.Logger {
	return p.logger
}

func (p *Proxy) Name() string {
	return p.name
}

func (n *Proxy) Start() {
	// Start event
	go utils.StartSubscribe(n)
}

func (n *Proxy) AddEvent(dualEvent *types.DualEvent) error {
	return n.eventPool.AddEvent(dualEvent)
}

func (n *Proxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	n.internalChain = internalChain
}

// SubmitTx submit corresponding tx to NEO or Kardia basing on Data in EventData, include release NEO
// and upgrade Kardia smart contract. In case of matching event, we find the matched request here to release NEO to.
func (n *Proxy) SubmitTx(event *types.EventData) error {
	// Only allow TxSource from Kardia
	if event.TxSource == types.KARDIA {
		switch event.Data.TxMethod {
		case configs.AddOrderFunction:
			return utils.HandleAddOrderFunction(n, event)
		default:
			log.Warn("Unexpected method in NEO SubmitTx", "method", event.Data.TxMethod)
			return configs.ErrUnsupportedMethod
		}
	}
	return configs.ErrUnsupportedMethod
}

// In case it's an exchange event (matchOrder), we will calculate matching order later
// when we submitTx to externalChain, so I simply return a basic metadata here basing on target and event hash,
// to differentiate TxMetadata inferred from events
func (n *Proxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.KARDIA,
	}, nil
}
