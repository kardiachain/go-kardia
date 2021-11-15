/*
 *  Copyright 2020 KardiaChain
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

package tx_pool

import (
	"math"

	"github.com/kardiachain/go-kardia/kai/events"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/event"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/mainchain/fetcher"
	txpoolproto "github.com/kardiachain/go-kardia/proto/kardiachain/txpool"
	"github.com/kardiachain/go-kardia/types"
)

const (
	TxpoolChannel = byte(0x30)

	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 8192

	// softResponseLimit is the target maximum size of replies to data retrievals.
	softResponseLimit = 2 * 1024 * 1024
)

// Reactor handles mempool tx broadcasting amongst peers.
// It maintains a map from peer ID to counter, to prevent gossiping txs to the
// peers you received it from.
type Reactor struct {
	p2p.BaseReactor
	config TxPoolConfig
	txpool *TxPool

	// transaction channel and subscriptions
	txsCh     chan events.NewTxsEvent
	txsSub    event.Subscription
	txFetcher *fetcher.TxFetcher

	peers *peerSet
}

// NewReactor returns a new Reactor with the given config and txpool.
func NewReactor(config TxPoolConfig, txpool *TxPool) *Reactor {
	txR := &Reactor{
		config: config,
		txpool: txpool,
		peers:  newPeerSet(),
	}

	txR.txFetcher = fetcher.NewTxFetcher(txpool.Has, txpool.AddRemotes, txR.fetchTx)
	txR.BaseReactor = *p2p.NewBaseReactor("txpool", txR)
	return txR
}

func (txR *Reactor) fetchTx(peer string, hashes []common.Hash) error {
	p := txR.peers.Peer(p2p.ID(peer))
	return p.RequestTxs(hashes)
}

// OnStart implements p2p.BaseReactor.
func (txR *Reactor) OnStart() error {
	if !txR.config.Broadcast {
		txR.Logger.Info("Tx broadcasting is disabled")
		return nil
	}
	txR.txFetcher.Start()
	go txR.broadcastTxRoutine()
	return nil
}

// GetChannels implements Reactor by returning the list of channels for this
// reactor.
func (txR *Reactor) GetChannels() []*p2p.ChannelDescriptor {
	largestTx := make([]byte, DefaultTxPoolConfig.MaxTxBytes)
	batchMsg := txpoolproto.Message{
		Sum: &txpoolproto.Message_Txs{
			Txs: &txpoolproto.Txs{Txs: [][]byte{largestTx}},
		},
	}

	return []*p2p.ChannelDescriptor{
		{
			ID:                  TxpoolChannel,
			Priority:            5,
			RecvMessageCapacity: batchMsg.Size(),
			RecvBufferCapacity:  128,
			MaxSendBytes:        5000,
		},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (txR *Reactor) AddPeer(peer p2p.Peer) {
	if err := txR.peers.Register(newPeer(txR.Logger, peer, txR.txpool)); err != nil {
		txR.Logger.Error("register peer err: %s", err)
		return
	}

	// Propagate existing transactions. new transactions appearing
	// after this will be sent via broadcasts.
	txR.syncTransactions(peer)
}

// syncTransactions starts sending all currently pending transactions to the given peer.
func (txR *Reactor) syncTransactions(peer p2p.Peer) {
	// Assemble the set of transaction to broadcast or announce to the remote
	// peer. Fun fact, this is quite an expensive operation as it needs to sort
	// the transactions if the sorting is not cached yet. However, with a random
	// order, insertions could overflow the non-executable queues and get dropped.
	var txs types.Transactions
	pending, _ := txR.txpool.Pending()
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	if len(txs) == 0 {
		return
	}
	// Send the entire transactions list as an announcement and let the remote side
	// decide what they need (likely nothing).
	hashes := make([]common.Hash, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.Hash()
	}

	p := txR.peers.Peer(peer.ID())
	if p != nil && len(hashes) > 0 {
		p.AsyncSendPooledTransactionHashes(hashes)
	}
}

// RemovePeer implements Reactor.
func (txR *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {

	if err := txR.peers.Unregister(peer.ID()); err != nil {
		txR.Logger.Error("unregister peer err: %s", err)
	}

	if err := txR.txFetcher.Drop(string(peer.ID())); err != nil {
		txR.Logger.Error("txFetcher drop err: %s", err)
	}
}

// Receive implements Reactor.
// It adds any received transactions to the txpool.
func (txR *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		txR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		txR.Switch.StopPeerForError(src, err)
		return
	}

	peerID := string(src.ID())
	p := txR.peers.Peer(src.ID())
	if p == nil {
		return
	}

	switch m := msg.(type) {
	case TxsMessage:
		for _, tx := range m.Txs {
			p.markTransaction(tx.Hash())
		}
		if err := txR.txFetcher.Enqueue(peerID, m.Txs, false); err != nil {
			txR.Logger.Info("Receive TxsMessage error", err)
		}
	case PooledTransactions:
		for _, tx := range m {
			p.markTransaction(tx.Hash())
		}
		if err := txR.txFetcher.Enqueue(peerID, m, true); err != nil {
			txR.Logger.Info("Receive PooledTransactions error", err)
		}
	case NewPooledTransactionHashes:
		// Schedule all the unknown hashes for retrieval
		for _, hash := range m {
			p.markTransaction(hash)
		}
		if err := txR.txFetcher.Notify(peerID, m); err != nil {
			txR.Logger.Info("Receive NewPooledTransactionHashes error", err)
		}
	case RequestPooledTransactionHashes:
		txR.handleRequestPooledTransactions(src, m)
	default:
		txR.Switch.StopPeerForError(src, err)
		return
	}

}

func (txR *Reactor) handleRequestPooledTransactions(src p2p.Peer, msg RequestPooledTransactionHashes) {
	var (
		bytes int
		txs   []*types.Transaction
	)
	for _, hash := range msg {
		if bytes >= softResponseLimit {
			break
		}
		tx := txR.txpool.Get(hash)
		if tx == nil {
			continue
		}

		bytes += int(tx.Size())
		txs = append(txs, tx)
	}

	if len(txs) > 0 {
		src.Send(TxpoolChannel, MustEncode(PooledTransactions(txs)))
	}
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

// Send new txpool txs to peer.
func (txR *Reactor) broadcastTxRoutine() {
	txset := make(map[*peer][]common.Hash)
	txR.txsCh = make(chan events.NewTxsEvent, txChanSize)
	txR.txsSub = txR.txpool.SubscribeNewTxsEvent(txR.txsCh)
	for {
		// In case of both next.NextWaitChan() and peer.Quit() are variable at the same time
		if !txR.IsRunning() {
			return
		}

		select {
		case txEvent := <-txR.txsCh:
			for _, tx := range txEvent.Txs {
				peers := txR.peers.PeersWithoutTx(tx.Hash())
				// Send the txset to a subset of our peers
				subset := peers[:int(math.Sqrt(float64(len(peers))))]
				for _, peer := range subset {
					txset[peer] = append(txset[peer], tx.Hash())
				}
				txR.Logger.Trace("Broadcast transaction", "hash", tx.Hash(), "recipients", len(peers))
			}

			for peer, hashes := range txset {
				// only send to validators
				peer.AsyncSendTransactions(hashes)
			}
		case <-txR.txsSub.Err():
			return
		}
	}

}

func (txR *Reactor) OnStop() {
	if txR.txsSub != nil {
		txR.txsSub.Unsubscribe()
	}
	txR.txFetcher.Stop()
}
