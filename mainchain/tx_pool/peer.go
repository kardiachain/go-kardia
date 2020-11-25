package tx_pool

import (
	"fmt"
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	prototx "github.com/kardiachain/go-kardiamain/proto/kardiachain/txpool"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	maxKnownTxs = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transactions to queue up before dropping
	// older broadcasts.
	maxQueuedTxs = 4096

	// This is the target size for the packs of transactions sent while broadcasting transactions.
	// A pack can get larger than this if a single transactions exceeds this size.
	txsyncPackSize = 100 * 1024
)

// max is a helper function which returns the larger of the two given integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PeerInfo represents a short summary of the Kardia sub-protocol metadata known
// about a connected peer.
type PeerInfo struct {
	Version int `json:"version"` // Kardia protocol version negotiated
}

type peer struct {
	logger log.Logger

	id p2p.ID

	peer p2p.Peer

	version int // Protocol version negotiated

	knownTxs    mapset.Set                           // Set of transaction hashes known to be known by this peer
	txBroadcast chan []common.Hash                   // Channel used to queue transaction propagation requests
	getPooledTx func(common.Hash) *types.Transaction // Callback used to retrieve transaction from txpool

	terminated chan struct{} // Termination channel, close when peer close to stop the broadcast loop routine.
	Protocol   string
}

func newPeer(logger log.Logger, p p2p.Peer, getPooledTx func(hash common.Hash) *types.Transaction) *peer {
	return &peer{
		logger:      logger,
		id:          p.ID(),
		peer:        p,
		knownTxs:    mapset.NewSet(),
		txBroadcast: make(chan []common.Hash),
		getPooledTx: getPooledTx,
		terminated:  make(chan struct{}),
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

// String implements fmt.Stringer.
func (p *peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("kai/%2d", p.version),
	)
}

// peerSet represents the collection of active peers currently participating in
// the Kardia sub-protocol.
type peerSet struct {
	peers  map[p2p.ID]*peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[p2p.ID]*peer),
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known. If a new peer it registered, its broadcast loop is also
// started.
func (ps *peerSet) Register(p *peer) error {
	p.logger.Debug("Registering a peer to peer set", "id", p.id)
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	go p.broadcastTransactions()

	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *peerSet) Unregister(id p2p.ID) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *peerSet) Peer(id p2p.ID) *peer {
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
	ps.closed = true
}

// broadcastTransactions is a async write loop that broadcast txs to remote peers.
func (p *peer) broadcastTransactions() {
	var (
		queue []common.Hash         // Queue of hashes to broadcast as full transactions
		done  chan struct{}         // Non-nil if background broadcaster is running
		fail  = make(chan error, 1) // Channel used to receive network error
	)
	for {
		// If there's no in-flight broadcast running, check if a new one is needed
		if done == nil && len(queue) > 0 {
			// Pile transaction until we reach our allowed network limit
			var (
				hashes  []common.Hash
				pending []common.Hash
				size    common.StorageSize
			)
			for i := 0; i < len(queue) && size < txsyncPackSize; i++ {
				if p.getPooledTx(queue[i]) != nil {
					pending = append(pending, queue[i])
					size += common.HashLength
				}
				hashes = append(hashes, queue[i])
			}
			queue = queue[:copy(queue, queue[len(hashes):])]

			// If there's anything available to transfer, fire up an async writer
			if len(pending) > 0 {
				done = make(chan struct{})
				go func() {
					if err := p.sendTransactions(pending); err != nil {
						p.logger.Error("Send txs failed", "err", err, "count", len(pending), "peer", p.id)
						fail <- err
						return
					}
					close(done)
					p.logger.Trace("Sent transactions", "count", len(pending))
				}()
			}
		}
		// Transfer goroutine may or may not have been started, listen for events
		select {
		case hashes := <-p.txBroadcast:
			// New batch of transactions to be broadcast, queue them (with capped capcacity)
			queue = append(queue, hashes...)
			if len(queue) > maxQueuedTxs {
				// Copy and resize queue to ensure buffer doesn't grow indefinitely
				queue = queue[:copy(queue, queue[len(queue)-maxQueuedTxs:])]
			}
		case <-done:
			done = nil
		case <-fail:
			// Consider remove peers here. Not yet implementation
			return
		case <-p.terminated:
			return
		}
	}
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *peerSet) PeersWithoutTx(hash common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	peers := ps.peers
	list := make([]*peer, 0, len(ps.peers))
	for _, p := range peers {
		if !p.knownTxs.Contains(hash) {
			list = append(list, p)
		}
	}
	return list
}

// SendTransactions sends transactions to the peer, adds the txn hashes to known txn set.
func (p *peer) sendTransactions(hashes []common.Hash) error {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Cardinality() > max(0, maxKnownTxs-len(hashes)) {
		p.knownTxs.Pop()
	}

	encoded := make([][]byte, len(hashes))
	for idx, tx := range hashes {
		txBytes, err := rlp.EncodeToBytes(tx)
		if err != nil {
			panic(err)
		}
		encoded[idx] = txBytes
	}
	msg := prototx.Message{
		Sum: &prototx.Message_Txs{
			Txs: &prototx.Txs{Txs: encoded},
		},
	}
	bz, err := msg.Marshal()
	if err != nil {
		panic(err)
	}

	p.peer.Send(TxpoolChannel, bz)
	return nil
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendTransactions(hashes []common.Hash) {
	// Tx will be actually sent in SendTransactions() trigger by broadcast() routine
	select {
	case p.txBroadcast <- hashes:
		// Mark all the transactions as known, but ensure we don't overflow our limits
		for p.knownTxs.Cardinality() > max(0, maxKnownTxs-len(hashes)) {
			p.knownTxs.Pop()
		}
		for _, hash := range hashes {
			p.knownTxs.Add(hash)
		}
	case <-p.terminated:
		p.logger.Debug("Dropping transaction propagation", "count", len(hashes))
	}
}
