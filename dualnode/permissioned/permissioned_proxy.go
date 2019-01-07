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

package permissioned

import (
	"fmt"
	"github.com/kardiachain/go-kardia/mainchain/tx_pool"
	"github.com/kardiachain/go-kardia/dualchain/event_pool"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/node"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
	"github.com/kardiachain/go-kardia/mainchain"
)

const SERVICE_NAME = "PRIVATE_DUAL"

// PermissionedProxy provides interfaces for PrivateChain Dual node
type PermissionedProxy struct {
	kardiaBc   base.BaseBlockChain
	txPool     *tx_pool.TxPool
	// TODO: uncomment these lines when we have specific watched smartcontract
	//smcAddress *common.Address
	//smcABI     *abi.ABI

	// Dual blockchain related fields
	dualBc    base.BaseBlockChain
	eventPool *event_pool.EventPool // Event pool of DUAL service.

	privateService *kai.KardiaService

	// The internal blockchain (i.e. Kardia's mainchain) that this dual node's interacting with.
	internalChain base.BlockChainAdapter

	// Chain head subscription for privatechain new blocks.
	chainHeadCh  chan base.ChainHeadEvent
	chainHeadSub event.Subscription

	// Chain head subscription for kardia new blocks.
	kardiaChainHeadCh chan base.ChainHeadEvent
	kardiaChainHeadSub event.Subscription

	logger log.Logger

}

// NewPermissionedProxy initiates a new private proxy
func NewPermissionedProxy(config *Config, internalBlockchain base.BaseBlockChain,
	txPool *tx_pool.TxPool, dualBc base.BaseBlockChain, eventPool *event_pool.EventPool) (*PermissionedProxy, error) {

	logger := log.New()
	logger.AddTag(SERVICE_NAME)

	// Setup nodeConfig for privatechain
	nodeConfig, err := SetUp(config)
	if err != nil {
		return nil, err
	}

	// New node based on nodeConfig
	n, err := node.NewNode(nodeConfig)
	if err != nil {
		return nil, err
	}

	n.RegisterService(kai.NewKardiaService)

	// Start node
	if err := n.Start(); err != nil {
		return nil, err
	}

	// Get privateService
	var kardiaService *kai.KardiaService
	if err := n.Service(&kardiaService); err != nil {
		return nil, fmt.Errorf("cannot get privateService: %v", err)
	}

	for i := 0; i < nodeConfig.EnvConfig.GetNodeSize(); i++ {
		peerURL := nodeConfig.EnvConfig.GetNodeMetadata(i).NodeID()
		logger.Info("Adding static peer", "peerURL", peerURL)
		success, err := n.AddPeer(peerURL)
		if !success {
			return nil, err
		}
	}

	processor := &PermissionedProxy{
		kardiaBc: internalBlockchain,
		dualBc: dualBc,
		eventPool: eventPool,
		txPool: txPool,
		privateService: kardiaService,
		chainHeadCh: make(chan base.ChainHeadEvent, 5),
		kardiaChainHeadCh: make(chan base.ChainHeadEvent, 5),
		logger: logger,
	}

	processor.kardiaChainHeadSub = internalBlockchain.SubscribeChainHeadEvent(processor.kardiaChainHeadCh)
	processor.chainHeadSub = kardiaService.BlockChain().SubscribeChainHeadEvent(processor.chainHeadCh)
	return processor, nil
}

func (p *PermissionedProxy) Start() {
	// Start event
	go p.loop()
}

func (p *PermissionedProxy) loop() {
	for {
		select {
		case privateEvent := <-p.chainHeadCh:
			if privateEvent.Block != nil {
				p.handlePrivateBlock(privateEvent.Block)
			}
		case kardiaEvent := <-p.kardiaChainHeadCh:
			if kardiaEvent.Block != nil {
				p.handleKardiaBlock(kardiaEvent.Block)
			}
		case err := <-p.chainHeadSub.Err():
			p.logger.Error("Error while listening to new blocks from privatechain", "error", err)
			return
		case err := <-p.kardiaChainHeadSub.Err():
			p.logger.Error("Error while listening to new blocks from kardiachain", "error", err)
			return
		}
	}
}

// handleBlock handles privatechain coming blocks and processes watched smc
func (p *PermissionedProxy) handlePrivateBlock(block *types.Block) {
	p.logger.Info("Received block from privatechain", "blockHeight", block.Height())
}

// handleKardiaBlock handles kardia coming blocks and processes watched smc
// FIXME(kiendn) should we need this function?
func (p *PermissionedProxy) handleKardiaBlock(block *types.Block) {
	p.logger.Info("Received block from kardiachain", "blockHeight", block.Height())
}

func (p *PermissionedProxy) SubmitTx(event *types.EventData) error {
	return nil
}

// ComputeTxMetadata pre-computes the tx metadata that will be submitted to another blockchain
// In case of error, this will return nil so that DualEvent won't be added to EventPool for further processing
func (p *PermissionedProxy) ComputeTxMetadata(event *types.EventData) (*types.TxMetadata, error) {
	return nil, nil
}

func (p *PermissionedProxy) RegisterInternalChain(internalChain base.BlockChainAdapter) {
	p.internalChain = internalChain
}
