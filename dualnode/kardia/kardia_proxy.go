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

package kardia

import (
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/dualnode/kardia/dual_logic_handler"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/abi"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/types"
	"github.com/pkg/errors"
	"math/big"
	"strings"
)

var errNilLogicHandler = errors.New("no logic handler available")

// Proxy of Kardia's chain to interface with dual's node, responsible for listening to the chain's
// new block and submiting Kardia's transaction.
type KardiaProxy struct {
	// Kardia's mainchain stuffs.
	kardiaBc     base.BaseBlockChain
	txPool       *tx_pool.TxPool
	chainHeadCh  chan base.ChainHeadEvent // Used to subscribe for new blocks.
	chainHeadSub event.Subscription

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool
	// isPrivateDual indicates whether this proxy is between Kardia and a private blockchain, used to determine which
	// handler is attached
	isPrivateDual bool

	// The external blockchain that this dual node's interacting with.
	externalChain base.BlockChainAdapter
	logicHandler  dual_logic_handler.KardiaTxHandlerAdapter
}

type MatchedRequest struct {
	MatchedRequestID *big.Int `abi:"matchedRequestID"`
	DestAddress      string   `abi:"destAddress"`
	SendAmount       *big.Int `abi:"sendAmount"`
}

func NewKardiaProxy(kardiaBc base.BaseBlockChain, txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, dualEventPool *event_pool.EventPool, isPrivateDual bool, smcAddr *common.Address, smcABIStr string) (*KardiaProxy, error) {
	var handler dual_logic_handler.KardiaTxHandlerAdapter
	var err error
	smcABI, err := abi.JSON(strings.NewReader(smcABIStr))
	if err != nil {
		return nil, err
	}
	// TODO(@sontranrad): This if-else logic should be removed soon and KardiaTxHandlerAdapter should be passed in dynamically
	if !isPrivateDual {
		// KardiaProxy runs on public dual node, attach CurrencyExchangeLogicHandler for currency exchange demo
		handler, err = dual_logic_handler.NewCurrencyExchangeLogicHandler(smcAddr, &smcABI)
		if err != nil {
			return nil, err
		}
	} else {
		// Kardia Proxy runs on private dual node, attach CandidateExchangeLogicHandler for candidate exchange demo
		handler, err = dual_logic_handler.NewCandidateExchangeLogicHandler(smcAddr, &smcABI)
		if err != nil {
			return nil, err
		}
	}

	processor := &KardiaProxy{
		kardiaBc:      kardiaBc,
		txPool:        txPool,
		dualBc:        dualBc,
		eventPool:     dualEventPool,
		chainHeadCh:   make(chan base.ChainHeadEvent, 5),
		logicHandler:  handler,
		isPrivateDual: isPrivateDual,
	}

	// Start subscription to blockchain head event.
	processor.chainHeadSub = kardiaBc.SubscribeChainHeadEvent(processor.chainHeadCh)

	return processor, nil
}

func (p *KardiaProxy) SubmitTx(event *types.EventData) error {
	if p.logicHandler == nil {
		return errNilLogicHandler
	}
	return p.logicHandler.SubmitTx(event, p.kardiaBc, p.txPool)
}

// ComputeTxMetadata precomputes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (p *KardiaProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	if p.logicHandler == nil {
		return nil, errNilLogicHandler
	}
	return p.logicHandler.ComputeTxMetadata(event, p.txPool)
}

func (p *KardiaProxy) Start(initRate bool) {
	// Start event
	go p.loop()
	if initRate {
		go p.logicHandler.Init(p.txPool)
	}
}

func (p *KardiaProxy) RegisterExternalChain(externalChain base.BlockChainAdapter) {
	p.externalChain = externalChain
	if p.logicHandler != nil {
		p.logicHandler.RegisterExternalChain(externalChain)
	}
}

func (p *KardiaProxy) loop() {
	if p.externalChain == nil {
		panic("External chain needs not to be nil.")
	}
	for {
		select {
		case ev := <-p.chainHeadCh:
			if ev.Block != nil {
				// New block
				// TODO(thietn): concurrency improvement. Consider call new go routine, or have height atomic counter.
				p.handleBlock(ev.Block)
			}
		case err := <-p.chainHeadSub.Err():
			log.Error("Error while listening to new blocks", "error", err)
			return
		}
	}
}

func (p *KardiaProxy) handleBlock(block *types.Block) {
	if p.logicHandler == nil {
		log.Error("Error handle Kardia block", "err", errNilLogicHandler)
	}
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == p.logicHandler.GetSmcAddress() {
			err := p.logicHandler.HandleKardiaTx(tx, p.eventPool, p.txPool)
			if err != nil {
				log.Error("Error handling tx", "txHash", tx.Hash(), "err", err)
			}
		}
	}
}
