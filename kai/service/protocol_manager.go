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

package service

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/types/evidence"

	"github.com/kardiachain/go-kardiamain/consensus"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/tx_pool"
)

const (
	softResponseLimit = 2 * 1024 * 1024 // Target maximum size of returned blocks, headers or node data.
	estHeaderRlpSize  = 500             // Approximate size of an RLP encoded block header
	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 8192
	csChanSize = 4096 // Consensus channel size.
)

// errIncompatibleConfig is returned if the requested protocols and configs are
// not compatible (low protocol version restrictions and high requirements).
var errIncompatibleConfig = errors.New("incompatible configuration")

func errResp(code errCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

type ProtocolManager struct {
	logger log.Logger

	consensus.BaseProtocol

	networkID uint64

	chainID uint64

	acceptTxs uint32 // Flag whether we're considered synchronised (enables transaction processing)

	maxPeers int

	txpool *tx_pool.TxPool

	blockchain  base.BaseBlockChain
	chainconfig *configs.ChainConfig

	// channels for fetcher, syncer, txsyncLoop

	noMorePeers chan struct{}
	quitSync    chan struct{}

	// transaction channel and subscriptions
	txsCh  chan events.NewTxsEvent
	txsSub event.Subscription

	// Consensus stuff
	csReactor *consensus.ConsensusManager
	//csCh    chan consensus.NewCsEvent
	csSub event.Subscription

	// Evidence Reactor
	evReactor *evidence.Reactor

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup
}

// NewProtocolManager returns a new Kardia sub protocol manager. The Kardia sub protocol manages peers capable
// with the Kardia network.
func NewProtocolManager(
	protocolName string,
	logger log.Logger,
	networkID uint64,
	chainID uint64,
	blockchain base.BaseBlockChain,
	config *configs.ChainConfig,
	txpool *tx_pool.TxPool,
	csReactor *consensus.ConsensusManager,
	evReactor *evidence.Reactor,
) (*ProtocolManager, error) {

	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		logger:      logger,
		networkID:   networkID,
		chainID:     chainID,
		txpool:      txpool,
		blockchain:  blockchain,
		chainconfig: config,
		noMorePeers: make(chan struct{}),
		csReactor:   csReactor,
		quitSync:    make(chan struct{}),
		evReactor:   evReactor,
	}

	return manager, nil
}

func (pm *ProtocolManager) removeServicePeer(id string) {

}

func (pm *ProtocolManager) removePeer(id string) {

}

// Start ...
func (pm *ProtocolManager) Start(maxPeers int) {
	pm.logger.Info("Start Kardia Protocol Manager", "maxPeers", maxPeers)
	pm.maxPeers = maxPeers

	// TODO(namdoh@,thientn@): Refactor this so we won't have to check this for dual service.
	if pm.txpool != nil {
		// broadcast transactions
		pm.txsCh = make(chan events.NewTxsEvent, txChanSize)
		pm.txsSub = pm.txpool.SubscribeNewTxsEvent(pm.txsCh)

		//namdoh@ pm.csCh = make(chan consensus.NewCsEvent, csChanSize)

	}
}

// Stop stop service
func (pm *ProtocolManager) Stop() {
	pm.logger.Info("Stopping Kardia protocol")

	if pm.txpool != nil {
		pm.txsSub.Unsubscribe() // quits txBroadcastLoop
	}

	// Wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()

	pm.logger.Info("Kardia protocol stopped")
}

// NodeInfo represents a short summary of the Kardia sub-protocol metadata
// known about the host peer.
type NodeInfo struct {
	Network uint64               `json:"network"` // Kardia network ID
	Height  uint64               `json:"height"`  // Height of the blockchain
	Genesis common.Hash          `json:"genesis"` // SHA3 hash of the host's genesis block
	Config  *configs.ChainConfig `json:"config"`  // Chain configuration for the fork rules
	Head    common.Hash          `json:"head"`    // SHA3 hash of the host's best owned block
}

// NodeInfo retrieves some protocol metadata about the running host node.
func (pm *ProtocolManager) NodeInfo() *NodeInfo {
	return &NodeInfo{
		Network: pm.networkID,
		Height:  pm.blockchain.CurrentBlock().Height(),
		Genesis: pm.blockchain.Genesis().Hash(),
		Config:  pm.blockchain.Config(),
		Head:    pm.blockchain.CurrentBlock().Hash(),
	}
}

func (pm *ProtocolManager) AcceptTxs() uint32 {
	return pm.acceptTxs
}

func (pm *ProtocolManager) SetAcceptTxs(acceptTxs uint32) {
	pm.acceptTxs = acceptTxs
}
