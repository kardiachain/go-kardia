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

package kai

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/consensus"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/types"
	"gopkg.in/fatih/set.v0"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

const (
	handshakeTimeout = 5 * time.Second
	maxKnownTxs      = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 128
)

// PeerInfo represents a short summary of the Kai sub-protocol metadata known
// about a connected peer.
type PeerInfo struct {
	Version int `json:"version"` // Kai protocol version negotiated
}

type peer struct {
	// TODO(namdoh): De-dup this logger duplicates with the log in p2p.Peer
	logger log.Logger

	id string

	lock sync.RWMutex

	*p2p.Peer
	rw p2p.MsgReadWriter

	version int // Protocol version negotiated

	knownTxs  *set.Set                  // Set of transaction hashes known to be known by this peer
	queuedTxs chan []*types.Transaction // Queue of transactions to broadcast to the peer

	csReactor *consensus.ConsensusManager

	terminated chan struct{} // Termination channel, close when peer close to stop the broadcast loop routine.

}

func newPeer(logger log.Logger, version int, p *p2p.Peer, rw p2p.MsgReadWriter, csReactor *consensus.ConsensusManager) *peer {
	return &peer{
		logger:     logger,
		Peer:       p,
		rw:         rw,
		version:    version,
		id:         fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		queuedTxs:  make(chan []*types.Transaction, maxQueuedTxs),
		knownTxs:   set.New(),
		csReactor:  csReactor,
		terminated: make(chan struct{}),
	}
}

// close signals the broadcast goroutine to terminate.
func (p *peer) close() {
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *peer) Info() *PeerInfo {

	return &PeerInfo{
		Version: p.version,
	}
}

// Handshake executes the kardia protocol handshake, negotiating version number,
// network IDs, head and genesis blocks.
func (p *peer) Handshake(network uint64, height uint64, head common.Hash, genesis common.Hash) error {
	p.logger.Trace("Handshake starts...")
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, kcmn.StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			Height:          height,
			CurrentBlock:    head,
			GenesisBlock:    genesis,
		})
	}()
	go func() {
		errc <- p.readStatus(network, &status, genesis)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				p.logger.Warn("Handshake return err", "err", err)
				return err
			}
			p.logger.Trace("Handshake returns no err")
		case <-timeout.C:
			p.logger.Warn("Handshake return read timeout")
			return p2p.DiscReadTimeout
		}
	}
	return nil
}

func (p *peer) readStatus(network uint64, status *statusData, genesis common.Hash) (err error) {
	msg, err := p.rw.ReadMsg()
	p.logger.Info("Read Status", "msg.Code", msg.Code, "err", err, "status", fmt.Sprintf("{ProtocolVersion:%v NetworkId:%v Height:%v CurrentBlock:%X GenesisBlock:%X",
		status.ProtocolVersion, status.NetworkId, status.Height, status.CurrentBlock[:12], status.GenesisBlock[:12]))
	if err != nil {
		return err
	}
	if msg.Code != kcmn.StatusMsg {
		return errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, kcmn.StatusMsg)
	}
	if msg.Size > kcmn.ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, kcmn.ProtocolMaxMsgSize)
	}
	// Decode the handshake and make sure everything matches
	if err := msg.Decode(&status); err != nil {
		return errResp(ErrDecode, "msg %v: %v", msg, err)
	}
	if status.GenesisBlock != genesis {
		return errResp(ErrGenesisBlockMismatch, "%x (!= %x)", status.GenesisBlock[:8], genesis[:8])
	}

	p.logger.Info("Decoded data", "msg", msg, "status", fmt.Sprintf("{ProtocolVersion:%v NetworkId:%v Height:%v CurrentBlock:%X GenesisBlock:%X",
		status.ProtocolVersion, status.NetworkId, status.Height, status.CurrentBlock[:12], status.GenesisBlock[:12]))

	if status.NetworkId != network {
		return errResp(ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	if int(status.ProtocolVersion) != p.version {
		return errResp(ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	return nil
}

// String implements fmt.Stringer.
func (p *peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("eth/%2d", p.version),
	)
}

// peerSet represents the collection of active peers currently participating in
// the Kardia sub-protocol.
type peerSet struct {
	peers  map[string]*peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known. If a new peer it registered, its broadcast loop is also
// started.
func (ps *peerSet) Register(p *peer) error {
	p.logger.Debug("Registering a peer to peer set", "name", p.Name())
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	go p.broadcast()
	p.csReactor.AddPeer(p.Peer, p.rw)
	p.IsAlive = true

	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	p.IsAlive = false
	p.csReactor.RemovePeer(p.Peer, nil)
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *peerSet) Peer(id string) *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len returns if the current number of peers in the set.
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}

// broadcast is a async write loop that send messages to remote peers.
func (p *peer) broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			if err := p.SendTransactions(txs); err != nil {
				p.logger.Error("Send txs failed", "err", err, "count", len(txs))
				return
			}
			p.logger.Trace("Transactions sent", "count", len(txs))

		case <-p.terminated:
			return
		}
	}
}

// MarkTransaction marks a transaction as known for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *peer) MarkTransaction(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Size() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *peerSet) PeersWithoutTx(hash common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownTxs.Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

// SendTransactions sends transactions to the peer, adds the txn hashes to known txn set.
func (p *peer) SendTransactions(txs types.Transactions) error {
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	return p2p.Send(p.rw, kcmn.TxMsg, txs)
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendTransactions(txs []*types.Transaction) {
	// Tx will be actually sent in SendTransactions() trigger by broadcast() routine
	select {
	case p.queuedTxs <- txs:
		for _, tx := range txs {
			p.knownTxs.Add(tx.Hash())
		}
	default:
		p.logger.Debug("Dropping transaction propagation", "count", len(txs))
	}
}
