package tx_pool

import (
	"errors"
	"fmt"

	"github.com/kardiachain/go-kardiamain/kai/events"
	"github.com/kardiachain/go-kardiamain/lib/event"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	prototx "github.com/kardiachain/go-kardiamain/proto/kardiachain/txpool"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	TxpoolChannel = byte(0x30)

	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 8192
)

// Reactor handles mempool tx broadcasting amongst peers.
// It maintains a map from peer ID to counter, to prevent gossiping txs to the
// peers you received it from.
type Reactor struct {
	p2p.BaseReactor
	config TxPoolConfig
	txpool *TxPool

	// transaction channel and subscriptions
	txsCh  chan events.NewTxsEvent
	txsSub event.Subscription

	peers *peerSet
}

// NewReactor returns a new Reactor with the given config and txpool.
func NewReactor(config TxPoolConfig, txpool *TxPool) *Reactor {
	txR := &Reactor{
		config: config,
		txpool: txpool,
		peers:  newPeerSet(),
	}
	txR.BaseReactor = *p2p.NewBaseReactor("txpool", txR)
	return txR
}

// OnStart implements p2p.BaseReactor.
func (txR *Reactor) OnStart() error {
	if !txR.config.Broadcast {
		txR.Logger.Info("Tx broadcasting is disabled")
		return nil
	}
	go txR.broadcastTxRoutine()
	return nil
}

// GetChannels implements Reactor by returning the list of channels for this
// reactor.
func (txR *Reactor) GetChannels() []*p2p.ChannelDescriptor {
	maxMsgSize := txR.config.MaxBatchBytes
	return []*p2p.ChannelDescriptor{
		{
			ID:                  TxpoolChannel,
			Priority:            5,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (txR *Reactor) AddPeer(peer p2p.Peer) {
	if err := txR.peers.Register(newPeer(txR.Logger, peer)); err != nil {
		txR.Logger.Error("register peer err: %s", err)
	}
}

// RemovePeer implements Reactor.
func (txR *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	if err := txR.peers.Unregister(peer.ID()); err != nil {
		txR.Logger.Error("unregister peer err: %s", err)
	}
	// broadcast routine checks if peer is gone and returns
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
	txR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)
	txR.txpool.AddRemotes(msg.Txs)
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

// Send new txpool txs to peer.
func (txR *Reactor) broadcastTxRoutine() {
	txR.txsCh = make(chan events.NewTxsEvent, txChanSize)
	txR.txsSub = txR.txpool.SubscribeNewTxsEvent(txR.txsCh)
	for {
		// In case of both next.NextWaitChan() and peer.Quit() are variable at the same time
		if !txR.IsRunning() {
			return
		}

		select {
		case txEvent := <-txR.txsCh:
			for peer, txs := range txR.peers.PeersWithoutTxs(txEvent.Txs) {
				// only send to validators
				peer.AsyncSendTransactions(txs)
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
}

//-----------------------------------------------------------------------------
// Messages

func decodeMsg(bz []byte) (TxsMessage, error) {
	msg := prototx.Message{}
	err := msg.Unmarshal(bz)
	if err != nil {
		return TxsMessage{}, err
	}

	var message TxsMessage

	if i, ok := msg.Sum.(*prototx.Message_Txs); ok {
		txs := i.Txs.GetTxs()

		if len(txs) == 0 {
			return message, errors.New("empty TxsMessage")
		}

		decoded := make([]*types.Transaction, len(txs))
		for j, txBytes := range txs {
			tx := &types.Transaction{}
			if err := rlp.DecodeBytes(txBytes, tx); err != nil {
				return message, err
			}

			decoded[j] = tx
		}

		message = TxsMessage{
			Txs: decoded,
		}
		return message, nil
	}
	return message, fmt.Errorf("msg type: %T is not supported", msg)
}

//-------------------------------------

// TxsMessage is a Message containing transactions.
type TxsMessage struct {
	Txs []*types.Transaction
}

// String returns a string representation of the TxsMessage.
func (m *TxsMessage) String() string {
	return fmt.Sprintf("[TxsMessage %v]", m.Txs)
}
