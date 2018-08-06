package kai

import (
	"github.com/kardiachain/go-kardia/p2p"
)

type Reactor interface {
	// SetSwitch allows setting a switch.
	SetProtocolManager(*ProtocolManager)

	// AddPeer is called by the protocol manager when a new peer is added.
	AddPeer(p *p2p.Peer, rw p2p.MsgReadWriter)

	// RemovePeer is called by the switch when the peer is stopped (due to error
	// or other reason).
	RemovePeer(p *p2p.Peer, reason interface{})

	// Receive is called when msgBytes is received from peer.
	//
	// NOTE reactor can not keep msgBytes around after Receive completes without
	// copying.
	ReceiveNewRoundStep(msg p2p.Msg, src *p2p.Peer)

	Start()
	Stop()
}

type BaseReactor struct {
	name string

	ProtocolManager *ProtocolManager
}

func NewBaseReactor(name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		name:            name,
		ProtocolManager: nil,
	}
}

func (br *BaseReactor) SetProtocolManager(pm *ProtocolManager) {
	br.ProtocolManager = pm
}

func (*BaseReactor) AddPeer(p *p2p.Peer, rw p2p.MsgReadWriter)  {}
func (*BaseReactor) RemovePeer(p *p2p.Peer, reason interface{}) {}
func (*BaseReactor) Start()                                     {}
func (*BaseReactor) Stop()                                      {}
func (*BaseReactor) Receive(msg p2p.Msg, src *p2p.Peer)         {}
