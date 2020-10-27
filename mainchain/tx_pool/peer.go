package tx_pool

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	prototx "github.com/kardiachain/go-kardiamain/proto/kardiachain/txpool"
	"github.com/kardiachain/go-kardiamain/types"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

const (
	maxKnownTxs = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 8192
)

// PeerInfo represents a short summary of the Kai sub-protocol metadata known
// about a connected peer.
type PeerInfo struct {
	Version int `json:"version"` // Kai protocol version negotiated
}

type peer struct {
	// TODO(namdoh): De-dup this logger duplicates with the log in p2p.Peer
	logger log.Logger

	id p2p.ID

	peer p2p.Peer

	version int // Protocol version negotiated

	knownTxs  *common.Set             // Set of transaction hashes known to be known by this peer
	queuedTxs chan types.Transactions // Queue of transactions to broadcast to the peer

	terminated chan struct{} // Termination channel, close when peer close to stop the broadcast loop routine.
	Protocol   string
}

func newPeer(logger log.Logger, p p2p.Peer) *peer {
	// isValidator := false
	// validators := csReactor.Validators()
	// pubKey, err := crypto.StringToPublicKey(hex.EncodeToString(p.ID().Bytes()))
	// fmt.Println("!!!", p.ID(), pubKey)
	// if err != nil {
	// 	logger.Error("invalid peer", "id", p.ID().String())
	// 	return nil
	// }
	// address := crypto.PubkeyToAddress(*pubKey)

	// for _, val := range validators {
	// 	if val.Address.Equal(address) {
	// 		isValidator = true
	// 		break
	// 	}
	// }

	// TODO(Lew):
	// We shouldn't check if a peer is a validator here,
	// we should check it when the consensus starts and collect
	// all the actives node then filter whom is a validator from
	// staking smartcontract.
	// Now we treat all node as a validator for testing purpose

	return &peer{
		id:         p.ID(),
		queuedTxs:  make(chan types.Transactions, maxQueuedTxs),
		knownTxs:   common.NewSet(maxKnownTxs),
		terminated: make(chan struct{}),
		peer:       p,
		logger:     logger,
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
	go p.broadcast()

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

// broadcast is a async write loop that send messages to remote peers.
func (p *peer) broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			if err := p.SendTransactions(txs); err != nil {
				p.logger.Error("Send txs failed", "err", err, "count", len(txs), "peer", p.id)
				return
			}
			p.logger.Trace("Broadcast transactions", "count", len(txs))
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
func (p *peer) SendTransactions(txs types.Transactions) error {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Size() >= maxKnownTxs {
		p.knownTxs.Pop()
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

	p.peer.Send(TxpoolChannel, bz)
	return nil
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendTransactions(txs types.Transactions) {
	// Tx will be actually sent in SendTransactions() trigger by broadcast() routine
	select {
	case p.queuedTxs <- txs:
		p.MarkTransactions(txs)
	default:
		p.logger.Debug("Dropping transaction propagation", "count", len(txs))
	}
}
