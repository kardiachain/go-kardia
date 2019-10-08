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
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"sync"
	"time"

	"github.com/kardiachain/go-kardia/consensus"
	serviceconst "github.com/kardiachain/go-kardia/kai/service/const"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/types"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
	errDiffChainID       = errors.New("diff chain id")
	errUnAuthorizedPeer  = errors.New("peer is not authorized")
)

const (
	handshakeTimeout = 5 * time.Second
	maxKnownTxs      = 5120 // Maximum transactions hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 5120
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

	knownTxs  *common.Set                  // Set of transaction hashes known to be known by this peer
	queuedTxs chan types.Transactions // Queue of transactions to broadcast to the peer

	csReactor *consensus.ConsensusManager

	terminated chan struct{} // Termination channel, close when peer close to stop the broadcast loop routine.
	Protocol string
	IsValidator bool
}

func newPeer(logger log.Logger, version int, p *p2p.Peer, rw p2p.MsgReadWriter, csReactor *consensus.ConsensusManager) *peer {
	isValidator := false
	validators := csReactor.Validators()
	pubKey, err := crypto.StringToPublicKey(hex.EncodeToString(p.ID().Bytes()))
	if err != nil {
		logger.Error("invalid peer", "id", p.ID().String())
		return nil
	}
	address := crypto.PubkeyToAddress(*pubKey)

	for _, val := range validators {
		if val.Address.Equal(address) {
			isValidator = true
			break
		}
	}

	return &peer{
		logger:     logger,
		Peer:       p,
		rw:         rw,
		version:    version,
		id:         fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		queuedTxs:  make(chan types.Transactions, maxQueuedTxs),
		knownTxs:   common.NewSet(maxKnownTxs),
		csReactor:  csReactor,
		terminated: make(chan struct{}),
		IsValidator: isValidator,
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
// Handshake can return error, or nil error but accept=false when peer is valid but gracefully rejected.
func (p *peer) Handshake(network uint64, chainID uint64, height uint64, head common.Hash, genesis common.Hash, hasPermission bool) (accept bool, err error) {
	p.logger.Trace("Handshake starts...")
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, serviceconst.StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			ChainID:         chainID,
			Height:          height,
			CurrentBlock:    head,
			GenesisBlock:    genesis,
		})
	}()
	go func() {
		errc <- p.readStatus(network, chainID, &status, genesis, hasPermission)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				if err == errDiffChainID {
					p.logger.Info("Reject peer with different ChainID", "peer", p.Name())
					return false, nil
				}
				p.logger.Warn("Handshake return err", "err", err)
				return false, err
			}
			p.logger.Trace("Handshake returns no err")
		case <-timeout.C:
			p.logger.Warn("Handshake return read timeout")
			return false, p2p.DiscReadTimeout
		}
	}
	return true, nil
}

func (p *peer) readStatus(network uint64, chainID uint64, status *statusData, genesis common.Hash, hasPermission bool) (err error) {
	msg, err := p.rw.ReadMsg()
	p.logger.Info("Read Peer handshake Status", "peer", p.Name(), "Code", msg.Code, "err", err, "NodeID", p.Peer.ID())
	if err != nil {
		return err
	}
	if msg.Code != serviceconst.StatusMsg {
		return errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, serviceconst.StatusMsg)
	}
	if msg.Size > serviceconst.ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, serviceconst.ProtocolMaxMsgSize)
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
	if status.ChainID != chainID {
		// FIXME(#211): have to use error handling path to reject mismatch chainID, but this is expected for some peer.
		return errDiffChainID
	}
	if !hasPermission {
		return errUnAuthorizedPeer
	}

	return nil
}

// String implements fmt.Stringer.
func (p *peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("kai/%2d", p.version),
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
			if len(txs) > 0 {
				go func() {
					if err := p.SendTransactions(&txs); err != nil {
						p.logger.Error("Send txs failed", "err", err, "count", len(txs), "peer", p.Name())
						return
					}
					p.logger.Trace("Transactions sent", "count", len(txs), "peer", p.Name())
				}()
			}
		case <-p.terminated:
			return
		}
	}
}

// MarkTransactions marks a list of transaction as known for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *peer) MarkTransactions(txs types.Transactions) []*types.Transaction {
	queueTxs := make([]*types.Transaction, 0)
	txHashes := make([]interface{}, 0)
	for _, tx := range txs {
		if p.knownTxs.Has(tx.Hash()) {
			continue
		}
		queueTxs = append(queueTxs, tx)
		txHashes = append(txHashes, tx.Hash())
	}

	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Size() >= maxKnownTxs {
		p.knownTxs.Pop()
	}

	if len(queueTxs) > 0 {
		p.knownTxs.Add(txHashes...)
	}
	return queueTxs
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *peerSet) PeersWithoutTx(tx *types.Transaction) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	peers := ps.peers
	list := make([]*peer, 0, len(ps.peers))
	for _, p := range peers {

		if !p.IsValidator {
			continue
		}

		if !p.knownTxs.Has(tx.Hash()) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutTxs retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *peerSet) PeersWithoutTxs(txs types.Transactions) map[*peer]types.Transactions {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	peers := ps.peers
	set := make(map[*peer]types.Transactions)
	for _, tx := range txs {
		for _, p := range peers {
			if !p.IsValidator {
				continue
			}

			if _, ok := set[p]; !ok {
				set[p] = make(types.Transactions, 0)
			}

			if !p.knownTxs.Has(tx.Hash()) {
				set[p] = append(set[p], tx)
			}
		}
	}
	return set
}

// SendTransactions sends transactions to the peer, adds the txn hashes to known txn set.
func (p *peer) SendTransactions(txs *types.Transactions) error {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Size() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	return p2p.Send(p.rw, serviceconst.TxMsg, txs)
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendTransactions(txs types.Transactions) {
	// Tx will be actually sent in SendTransactions() trigger by broadcast() routine
	select {
	case p.queuedTxs <- txs:
		go p.MarkTransactions(txs)
	default:
		p.logger.Debug("Dropping transaction propagation", "count", len(txs))
	}
}