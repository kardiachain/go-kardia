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
	"sync/atomic"

	serviceconst "github.com/kardiachain/go-kardia/common/service/const"
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/consensus"
	"github.com/kardiachain/go-kardia/kardia/blockchain"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"github.com/kardiachain/go-kardia/types"
)

const (
	softResponseLimit = 2 * 1024 * 1024 // Target maximum size of returned blocks, headers or node data.
	estHeaderRlpSize  = 500             // Approximate size of an RLP encoded block header
	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
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

	acceptTxs uint32 // Flag whether we're considered synchronised (enables transaction processing)

	maxPeers int

	peers *peerSet

	txpool *blockchain.TxPool

	blockchain  blockchain.BaseBlockChain
	chainconfig *configs.ChainConfig

	SubProtocols []p2p.Protocol

	// channels for fetcher, syncer, txsyncLoop
	newPeerCh   chan *peer
	noMorePeers chan struct{}

	// transaction channel and subscriptions
	txsCh  chan blockchain.NewTxsEvent
	txsSub event.Subscription

	// Consensus stuff
	csReactor *consensus.ConsensusManager
	//csCh    chan consensus.NewCsEvent
	csSub event.Subscription

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup
}

// NewProtocolManager returns a new Kardia sub protocol manager. The Kardia sub protocol manages peers capable
// with the Kardia network.
func NewProtocolManager(protocolName string, logger log.Logger, networkID uint64, blockchain blockchain.BaseBlockChain, config *configs.ChainConfig, txpool *blockchain.TxPool, csReactor *consensus.ConsensusManager) (*ProtocolManager, error) {

	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		logger:      logger,
		networkID:   networkID,
		txpool:      txpool,
		blockchain:  blockchain,
		chainconfig: config,
		peers:       newPeerSet(),
		newPeerCh:   make(chan *peer),
		noMorePeers: make(chan struct{}),
		csReactor:   csReactor,
	}

	// Initiate a sub-protocol for every implemented version we can handle
	manager.SubProtocols = make([]p2p.Protocol, 0, len(serviceconst.ProtocolVersions))
	for i, version := range serviceconst.ProtocolVersions {
		// Compatible; initialise the sub-protocol
		version := version // Closure for the run
		manager.SubProtocols = append(manager.SubProtocols, p2p.Protocol{
			Name:    protocolName,
			Version: version,
			Length:  serviceconst.ProtocolLengths[i],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := manager.newPeer(int(version), p, rw)
				select {
				case manager.newPeerCh <- peer:
					manager.wg.Add(1)
					defer manager.wg.Done()
					return manager.handle(peer)
				}
			},
			NodeInfo: func() interface{} {
				return manager.NodeInfo()
			},
			PeerInfo: func(id discover.NodeID) interface{} {
				if p := manager.peers.Peer(fmt.Sprintf("%x", id[:8])); p != nil {
					return p.Info()
				}
				return nil
			},
		})
	}
	if len(manager.SubProtocols) == 0 {
		return nil, errIncompatibleConfig
	}

	return manager, nil
}

func (pm *ProtocolManager) removePeer(id string) {
	// Short circuit if the peer was already removed
	peer := pm.peers.Peer(id)
	if peer == nil {
		return
	}
	pm.logger.Debug("Removing Kardia peer", "peer", id)

	// Unregister the peer from the Kardia peer set
	if err := pm.peers.Unregister(id); err != nil {
		pm.logger.Error("Peer removal failed", "peer", id, "err", err)
	}
	// Hard disconnect at the networking layer
	if peer != nil {
		peer.Peer.Disconnect(p2p.DiscUselessPeer)
	}
}

func (pm *ProtocolManager) Start(maxPeers int) {
	pm.logger.Info("Start Kardia Protocol Manager", "maxPeers", maxPeers)
	pm.maxPeers = maxPeers

	// TODO(namdoh@,thientn@): Refactor this so we won't have to check this for dual service.
	if pm.txpool != nil {
		// broadcast transactions
		pm.txsCh = make(chan blockchain.NewTxsEvent, txChanSize)
		pm.txsSub = pm.txpool.SubscribeNewTxsEvent(pm.txsCh)

		//namdoh@ pm.csCh = make(chan consensus.NewCsEvent, csChanSize)

		go pm.txBroadcastLoop()
	}
	//namdoh@ go pm.csBroadcastLoop()
	go syncNetwork(pm)
}

func (pm *ProtocolManager) Stop() {
	pm.logger.Info("Stopping Kardia protocol")

	if pm.txpool != nil {
		pm.txsSub.Unsubscribe() // quits txBroadcastLoop
	}

	// Quit the sync loop.
	// After this send has completed, no new peers will be accepted.
	pm.noMorePeers <- struct{}{}

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.peers.Close()

	// Wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()

	pm.logger.Info("Kardia protocol stopped")
}

func (pm *ProtocolManager) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) *peer {
	//@huny return newPeer(pv, p, newMeteredMsgWriter(rw))
	return newPeer(pm.logger, pv, p, rw, pm.csReactor)
}

// handle is the callback invoked to manage the life cycle of a kai peer. When
// this function terminates, the peer is disconnected.
func (pm *ProtocolManager) handle(p *peer) error {
	// Ignore maxPeers if this is a trusted peer
	if pm.peers.Len() >= pm.maxPeers && !p.Peer.Info().Network.Trusted {
		return p2p.DiscTooManyPeers
	}
	pm.logger.Debug("Kardia peer connected", "name", p.Name())

	// Execute the Kardia handshake
	var (
		genesis = pm.blockchain.Genesis()
		hash    = pm.blockchain.CurrentHeader().Hash()
		height  = pm.blockchain.CurrentBlock().Height()
	)

	if err := p.Handshake(pm.networkID, height, hash, genesis.Hash()); err != nil {
		pm.logger.Debug("Kardia handshake failed", "err", err)
		return err
	}

	// Register the peer locally
	if err := pm.peers.Register(p); err != nil {
		pm.logger.Error("Kardia peer registration failed", "err", err)
		return err
	}
	defer pm.removePeer(p.id)

	// TODO(thientn): performance optimization. This function should be reliable since it's before the main loop.
	pm.syncTransactions(p)

	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			pm.logger.Warn("Kardia message handling failed", "err", err)
			return err
		}
	}
}

// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *ProtocolManager) handleMsg(p *peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > serviceconst.ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, serviceconst.ProtocolMaxMsgSize)
	}
	defer msg.Discard()

	// Handle the message depending on its contents
	switch {
	case msg.Code == serviceconst.StatusMsg:
		// Status messages should never arrive after the handshake
		return errResp(ErrExtraStatusMsg, "uncontrolled status message")
	case msg.Code == serviceconst.TxMsg:
		pm.logger.Trace("Transactions received")
		// TODO(namdoh@,thientn@): Refactor this so we won't have to check this for dual service.
		if pm.txpool == nil {
			pm.logger.Info("This service doesn't accept incoming transactions")
			return nil
		}

		// Transactions arrived, make sure we have a valid and fresh chain to handle them
		if atomic.LoadUint32(&pm.acceptTxs) == 0 {
			pm.logger.Trace("Skip received txs, acceptTxs flag is false")
			break
		}
		// Transactions can be processed, parse all of them and deliver to the pool
		var txs []*types.Transaction
		if err := msg.Decode(&txs); err != nil {
			return errResp(ErrDecode, "msg %v: %v", msg, err)
		}
		for i, tx := range txs {
			// Validate and mark the remote transaction
			if tx == nil {
				return errResp(ErrDecode, "transaction %d is nil", i)
			}
			p.MarkTransaction(tx.Hash())
		}
		pm.txpool.AddRemotes(txs)
		pm.logger.Trace("Transactions added to pool", "txs", txs)
	case msg.Code == serviceconst.CsNewRoundStepMsg:
		pm.logger.Trace("NewRoundStep message received")
		pm.csReactor.ReceiveNewRoundStep(msg, p.Peer)
	case msg.Code == serviceconst.CsProposalMsg:
		pm.logger.Trace("Proposal message received")
		pm.csReactor.ReceiveNewProposal(msg, p.Peer)
	case msg.Code == serviceconst.CsVoteMsg:
		pm.logger.Trace("Vote messsage received")
		pm.csReactor.ReceiveNewVote(msg, p.Peer)
	case msg.Code == serviceconst.CsHasVoteMsg:
		pm.logger.Trace("HasVote messsage received")
		pm.csReactor.ReceiveHasVote(msg, p.Peer)
	case msg.Code == serviceconst.CsProposalPOLMsg:
		pm.logger.Trace("ProposalPOL messsage received")
		pm.csReactor.ReceiveProposalPOL(msg, p.Peer)
	case msg.Code == serviceconst.CsCommitStepMsg:
		pm.logger.Trace("CommitStep message received")
		pm.csReactor.ReceiveNewCommit(msg, p.Peer)
	case msg.Code == serviceconst.CsBlockMsg:
		pm.logger.Trace("Block message received")
		pm.csReactor.ReceiveBlock(msg, p.Peer)
	case msg.Code == serviceconst.CsVoteSetMaj23Message:
		pm.logger.Trace("VoteSetMaj23 message received")
		pm.csReactor.ReceiveVoteSetMaj23(msg, p.Peer)
	case msg.Code == serviceconst.CsVoteSetBitsMessage:
		pm.logger.Trace("VoteSetBits message received")
		pm.csReactor.ReceiveVoteSetBits(msg, p.Peer)
	default:
		return errResp(ErrInvalidMsgCode, "%v", msg.Code)
	}
	return nil
}

// syncTransactions sends all pending transactions to the new peer.
func (pm *ProtocolManager) syncTransactions(p *peer) {
	// TODO(namdoh@,thientn@): Refactor this so we won't have to check this for dual service.
	if pm.txpool == nil {
		return
	}
	pm.logger.Trace("Sync txns to new peer", "peer", p)
	// TODO(thientn): sends transactions in chunks. This may send a large number of transactions.
	// Breaks them to chunks here or inside AsyncSend to not overload the pipeline.
	txsMap, _ := pm.txpool.Pending()
	var txs types.Transactions

	for _, list := range txsMap {
		txs = append(txs, list...)
	}
	if len(txs) == 0 {
		return
	}
	pm.logger.Trace("Start sending pending transactions", "count", len(txs))
	p.AsyncSendTransactions(txs)
}

func (pm *ProtocolManager) txBroadcastLoop() {
	for {
		select {
		case txEvent := <-pm.txsCh:
			pm.BroadcastTxs(txEvent.Txs)

		// Err() channel will be closed when unsubscribing.
		case <-pm.txsSub.Err():
			return
		}
	}
}

// A loop for broadcasting consensus events.
func (pm *ProtocolManager) Broadcast(msg interface{}, msgType uint64) {

	// If msg's type is *consensus.CommitStepMessage, v will hold the instance and ok will be true
	v, ok := msg.(*consensus.CommitStepMessage)

	// If ok is true, then simplify the log
	if ok {
		pm.logger.Info("Start broadcast consensus message", "Height", v.Height, "Block", v.Block.String(), "msgType", msgType)
	} else {
		pm.logger.Info("Start broadcast consensus message", "msg", msg, "msgType", msgType)
	}

	for _, p := range pm.peers.peers {
		pm.wg.Add(1)
		go func(p *peer) {
			defer pm.wg.Done()
			p2p.Send(p.rw, msgType, msg)
		}(p)
	}
}

// BroadcastTxs will propagate a batch of transactions to all peers which are not known to
// already have the given transaction.
func (pm *ProtocolManager) BroadcastTxs(txs types.Transactions) {
	var txset = make(map[*peer]types.Transactions)
	pm.logger.Info("Start broadcast txn", "txn", txs)
	// Broadcast transactions to a batch of peers not knowing about it
	for _, tx := range txs {
		peers := pm.peers.PeersWithoutTx(tx.Hash())
		for _, peer := range peers {
			txset[peer] = append(txset[peer], tx)
		}
		pm.logger.Trace("Broadcast transaction", "hash", tx.Hash(), "recipients", len(peers))
	}
	// FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	for peer, txs := range txset {
		peer.AsyncSendTransactions(txs)
	}
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
