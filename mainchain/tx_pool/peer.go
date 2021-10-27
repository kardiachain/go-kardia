package tx_pool

import (
	"errors"
	"fmt"
	"sync"

	mapset "github.com/deckarep/golang-set"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/rlp"
	prototx "github.com/kardiachain/go-kardia/proto/kardiachain/txpool"
	"github.com/kardiachain/go-kardia/types"
)

const (
	maxKnownTxs = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transactions to queue up before dropping
	// older broadcasts.
	maxQueuedTxs = 4096

	// This is the target size for the packs of transactions or announcements. A
	// pack can get larger than this if a single transactions exceeds this size.
	maxTxPacketSize = 100 * 1024

	// maxQueuedTxAnns is the maximum number of transaction announcements to queue up
	// before dropping older announcements.
	maxQueuedTxAnns = 4096
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

	txpool      *TxPool            // Transaction pool used by the broadcasters for liveness checks
	knownTxs    mapset.Set         // Set of transaction hashes known to be known by this peer
	txBroadcast chan []common.Hash // Channel used to queue transaction propagation requests
	txAnnounce  chan []common.Hash // Channel used to queue transaction announcement requests

	terminated chan struct{} // Termination channel, close when peer close to stop the broadcast loop routine.
	Protocol   string
}

func newPeer(logger log.Logger, p p2p.Peer, txpool *TxPool) *peer {
	return &peer{
		logger:      logger,
		id:          p.ID(),
		peer:        p,
		knownTxs:    mapset.NewSet(),
		txBroadcast: make(chan []common.Hash),
		txAnnounce:  make(chan []common.Hash),
		txpool:      txpool,
		terminated:  make(chan struct{}),
	}
}

// close signals the broadcast goroutine to terminate.
func (p *peer) close() {
	close(p.terminated)
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
	go p.announceTransactions()
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
		queue  []common.Hash         // Queue of hashes to broadcast as full transactions
		done   chan struct{}         // Non-nil if background broadcaster is running
		fail   = make(chan error, 1) // Channel used to receive network error
		failed bool                  // Flag whether a send failed, discard everything onward
	)
	for {
		// If there's no in-flight broadcast running, check if a new one is needed
		if done == nil && len(queue) > 0 {
			// Pile transaction until we reach our allowed network limit
			var (
				hashes []common.Hash
				txs    []*types.Transaction
				size   common.StorageSize
			)
			for i := 0; i < len(queue) && size < maxTxPacketSize; i++ {
				if tx := p.txpool.Get(queue[i]); tx != nil {
					txs = append(txs, tx)
					size += tx.Size()
				}
				hashes = append(hashes, queue[i])
			}
			queue = queue[:copy(queue, queue[len(hashes):])]

			// If there's anything available to transfer, fire up an async writer
			if len(txs) > 0 {
				done = make(chan struct{})
				go func() {
					if err := p.sendTransactions(txs); err != nil {
						fail <- err
						return
					}
					close(done)
					p.logger.Trace("Sent transactions", "count", len(txs))
				}()
			}
		}
		// Transfer goroutine may or may not have been started, listen for events
		select {
		case hashes := <-p.txBroadcast:
			// If the connection failed, discard all transaction events
			if failed {
				continue
			}
			// New batch of transactions to be broadcast, queue them (with cap)
			queue = append(queue, hashes...)
			if len(queue) > maxQueuedTxs {
				// Fancy copy and resize to ensure buffer doesn't grow indefinitely
				queue = queue[:copy(queue, queue[len(queue)-maxQueuedTxs:])]
			}

		case <-done:
			done = nil

		case <-fail:
			failed = true

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
func (p *peer) sendTransactions(txs types.Transactions) error {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Cardinality() > max(0, maxKnownTxs-len(txs)) {
		p.knownTxs.Pop()
	}

	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}

	encoded := make([][]byte, len(txs))
	for idx, tx := range txs {
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

	ok := p.peer.Send(TxpoolChannel, bz)
	if !ok {
		return errors.New("sendTransactions not success")
	}
	return nil
}

// sendPooledTransactionHashes sends transaction hashes to the peer and includes
// them in its transaction hash set for future reference.
//
// This method is a helper used by the async transaction announcer. Don't call it
// directly as the queueing (memory) and transmission (bandwidth) costs should
// not be managed directly.
func (p *peer) sendPooledTransactionHashes(hashes []common.Hash) error {
	// Mark all the transactions as known, but ensure we don't overflow our limits
	for _, hash := range hashes {
		p.knownTxs.Add(hash)
	}
	ok := p.peer.Send(TxpoolChannel, MustEncode(NewPooledTransactionHashes(hashes)))
	if !ok {
		return errors.New("send NewPooledTransactionHashes not success")
	}
	return nil
}

// markTransaction marks a transaction as known for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *peer) markTransaction(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known transaction hash
	p.knownTxs.Add(hash)
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

// AsyncSendPooledTransactionHashes queues a list of transactions hashes to eventually
// announce to a remote peer.  The number of pending sends are capped (new ones
// will force old sends to be dropped)
func (p *peer) AsyncSendPooledTransactionHashes(hashes []common.Hash) {
	select {
	case p.txAnnounce <- hashes:
		// Mark all the transactions as known, but ensure we don't overflow our limits
		for _, hash := range hashes {
			p.knownTxs.Add(hash)
		}

	case <-p.terminated:
		p.logger.Debug("Dropping transaction announcement", "count", len(hashes))
	}
}

// RequestTxs fetches a batch of transactions from a remote node.
func (p *peer) RequestTxs(hashes []common.Hash) error {
	p.logger.Debug("Fetching batch of transactions", "count", len(hashes))
	p.peer.Send(TxpoolChannel, MustEncode(RequestPooledTransactionHashes(hashes)))
	return nil
}

// announceTransactions is a write loop that schedules transaction broadcasts
// to the remote peer. The goal is to have an async writer that does not lock up
// node internals and at the same time rate limits queued data.
func (p *peer) announceTransactions() {
	var (
		queue  []common.Hash         // Queue of hashes to announce as transaction stubs
		done   chan struct{}         // Non-nil if background announcer is running
		fail   = make(chan error, 1) // Channel used to receive network error
		failed bool                  // Flag whether a send failed, discard everything onward
	)

	for {
		// If there's no in-flight announce running, check if a new one is needed
		if done == nil && len(queue) > 0 {
			// Pile transaction hashes until we reach our allowed network limit
			var (
				count   int
				pending []common.Hash
				size    common.StorageSize
			)
			for count = 0; count < len(queue) && size < maxTxPacketSize; count++ {
				if p.txpool.Get(queue[count]) != nil {
					pending = append(pending, queue[count])
					size += common.HashLength
				}
			}
			// Shift and trim queue
			queue = queue[:copy(queue, queue[count:])]

			// If there's anything available to transfer, fire up an async writer
			if len(pending) > 0 {
				done = make(chan struct{})
				go func() {
					if err := p.sendPooledTransactionHashes(pending); err != nil {
						fail <- err
						return
					}
					close(done)
					p.logger.Trace("Sent transaction announcements", "count", len(pending))
				}()
			}
		}
		// Transfer goroutine may or may not have been started, listen for events
		select {
		case hashes := <-p.txAnnounce:

			// If the connection failed, discard all transaction events
			if failed {
				continue
			}
			// New batch of transactions to be broadcast, queue them (with cap)
			queue = append(queue, hashes...)
			if len(queue) > maxQueuedTxAnns {
				// Fancy copy and resize to ensure buffer doesn't grow indefinitely
				queue = queue[:copy(queue, queue[len(queue)-maxQueuedTxAnns:])]
			}

		case <-done:
			done = nil

		case <-fail:
			failed = true

		case <-p.terminated:
			return
		}
	}
}
