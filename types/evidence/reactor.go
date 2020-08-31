package evidence

import (
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/types"
)

// Reactor handles evpool evidence broadcasting amongst peers.
type Reactor struct {
	evpool   *Pool
	eventBus *types.EventBus
}

// NewReactor returns a new Reactor with the given config and evpool.
func NewReactor(evpool *Pool) *Reactor {
	evR := &Reactor{
		evpool: evpool,
	}
	return evR
}

// Receive implements Reactor.
// It adds any received evidence to the evpool.
func (evR *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {

}
