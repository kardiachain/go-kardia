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

package dual_proxy

import (
	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/dualchain/event_pool"
	"github.com/kardiachain/go-kardiamain/dualnode/utils"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/ksml"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
	"github.com/kardiachain/go-kardiamain/types"
	"sync"
)

type Proxy struct {

	// name is name of proxy, or type that proxy connects to (eg: NEO, TRX, ETH, KARDIA)
	name   string

	logger log.Logger // Logger for proxy service

	kardiaBc   base.BaseBlockChain
	txPool     *tx_pool.TxPool

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.Pool // Event pool of DUAL service.

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// Chain head subscription for new blocks.
	chainHeadCh  chan events.ChainHeadEvent
	chainHeadSub event.Subscription

	// Queue configuration
	publishedEndpoint string
	subscribedEndpoint string

	mtx sync.Mutex
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
func (p *Proxy) DualEventPool() *event_pool.Pool {
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

func NewProxy(
	serviceName string,
	kardiaBc base.BaseBlockChain,
	txPool *tx_pool.TxPool,
	dualBc base.BaseBlockChain,
	dualEventPool *event_pool.Pool,
	publishedEndpoint string,
	subscribedEndpoint string,
) (*Proxy, error) {

	// Create a specific logger for DUAL service.
	logger := log.New()
	logger.AddTag(serviceName)

	processor := &Proxy{
		name:       serviceName,
		logger:     logger,
		kardiaBc:   kardiaBc,
		txPool:     txPool,
		dualBc:     dualBc,
		eventPool:  dualEventPool,
		chainHeadCh: make(chan events.ChainHeadEvent, 5),
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

func (p *Proxy) Start() {
	// Start event
	go utils.StartSubscribe(p)
}

func (p *Proxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	p.internalChain = internalChain
}

func (p *Proxy) RegisterExternalChain(externalChain base.BlockChainAdapter) {
	panic("this function is not implemented")
}

// SubmitTx reads event data and submits data to Kardia or Target chain (TRON, NEO) based on specific logic. (eg: AddOrderFunction)
func (p *Proxy) SubmitTx(event *types.EventData) error {
	msg, err := event.GetEventMessage()
	if err != nil {
		return err
	}
	if event.Actions != nil && len(event.Actions) > 0 {
		smc := common.HexToAddress(msg.MasterSmartContract)
		parser := ksml.NewParser(p.Name(), p.PublishedEndpoint(), utils.PublishMessage, p.kardiaBc, p.txPool, &smc, event.Actions, msg, true)
		return parser.ParseParams()
	}
	return nil
}

func (p *Proxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	return &types.TxMetadata{
		TxHash: event.Hash(),
		Target: types.KARDIA,
	}, nil
}

func (p *Proxy) Lock() {
	p.mtx.Lock()
}

func (p *Proxy) UnLock() {
	p.mtx.Unlock()
}
