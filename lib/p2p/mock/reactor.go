package mock

import (
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/p2p/conn"
)

type Reactor struct {
	p2p.BaseReactor
}

func NewReactor() *Reactor {
	r := &Reactor{}
	r.BaseReactor = *p2p.NewBaseReactor("Mock-PEX", r)
	r.SetLogger(log.TestingLogger())
	return r
}

func (r *Reactor) GetChannels() []*conn.ChannelDescriptor             { return []*conn.ChannelDescriptor{} }
func (r *Reactor) AddPeer(peer p2p.Peer) error                        { return nil }
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) error { return nil }
func (r *Reactor) Receive(chID byte, peer p2p.Peer, msgBytes []byte)  {}
