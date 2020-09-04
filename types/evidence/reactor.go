package evidence

import (
	"fmt"
	"io"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/clist"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/lib/service"

	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/types"
)

const (
	// EvListMsg ..
	EvListMsg = 0x13 // EvListMsg

	broadcastEvidenceIntervalS = 60  // broadcast uncommitted evidence this often
	peerCatchupSleepIntervalMS = 100 // If peer is behind, sleep this amount
)

// Reactor handles evpool evidence broadcasting amongst peers.
type Reactor struct {
	service.BaseService
	evpool   *Pool
	eventBus *types.EventBus
	protocol Protocol
}

// NewReactor returns a new Reactor with the given config and evpool.
func NewReactor(evpool *Pool) *Reactor {
	evR := &Reactor{
		evpool: evpool,
	}
	return evR
}

// SetProtocol ...
func (evR *Reactor) SetProtocol(protocol Protocol) {
	evR.protocol = protocol
}

// SetLogger sets the Logger on the reactor and the underlying Evidence.
func (evR *Reactor) SetLogger(l log.Logger) {
	evR.Logger = l
	evR.evpool.SetLogger(l)
}

// AddPeer implements Reactor.
func (evR *Reactor) AddPeer(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	go evR.broadcastEvidenceRoutine(peer, rw)
}

// Receive implements Reactor.
// It adds any received evidence to the evpool.
func (evR *Reactor) Receive(src *p2p.Peer, msg p2p.Msg) error {
	evis, err := decodeMsg(msg)
	if err != nil {
		evR.Logger.Error("Error decoding message", "src", src, "err", err)
		evR.protocol.StopPeerForError(src, err)
		return nil
	}

	for _, ev := range evis {
		err := evR.evpool.AddEvidence(ev)
		switch err.(type) {
		case *types.ErrEvidenceInvalid:
			evR.Logger.Error(err.Error())
			// punish peer
			evR.protocol.StopPeerForError(src, err)
			return nil
		case nil:
		default:
			// continue to the next piece of evidence
			evR.Logger.Error("Evidence has not been added", "evidence", evis, "err", err)
		}
	}
	return nil
}

// Modeled after the mempool routine.
// - Evidence accumulates in a clist.
// - Each peer has a routine that iterates through the clist,
// sending available evidence to the peer.
// - If we're waiting for new evidence and the list is not empty,
// start iterating from the beginning again.
func (evR *Reactor) broadcastEvidenceRoutine(peer *p2p.Peer, rw p2p.MsgReadWriter) {
	var next *clist.CElement
	for {

		if !peer.IsAlive || !evR.IsRunning() {
			return
		}

		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-evR.evpool.EvidenceWaitChan(): // Wait until evidence is available
				if next = evR.evpool.EvidenceFront(); next == nil {
					continue
				}
			}
		}

		ev := next.Value.(types.Evidence)
		msg, retry := evR.checkSendEvidenceMessage(peer, ev)
		if msg != nil {
			err := p2p.Send(rw, EvListMsg, msg)
			retry = err != nil
		}

		if retry {
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		afterCh := time.After(time.Second * broadcastEvidenceIntervalS)
		select {
		case <-afterCh:
			// start from the beginning every tick.
			// TODO: only do this if we're at the end of the list!
			next = nil
		case <-next.NextWaitChan():
			// see the start of the for loop for nil check
			next = next.Next()
		}
	}
}

// Returns the message to send the peer, or nil if the evidence is invalid for the peer.
// If message is nil, return true if we should sleep and try again.
func (evR Reactor) checkSendEvidenceMessage(
	peer *p2p.Peer,
	ev types.Evidence,
) (msg Message, retry bool) {

	// make sure the peer is up to date
	evHeight := ev.Height()
	peerState, ok := peer.Get(p2p.PeerStateKey).(PeerState)
	if !ok {
		// Peer does not have a state yet. We set it in the consensus reactor, but
		// when we add peer in Switch, the order we call reactors#AddPeer is
		// different every time due to us using a map. Sometimes other reactors
		// will be initialized before the consensus reactor. We should wait a few
		// milliseconds and retry.
		return nil, true
	}

	// NOTE: We only send evidence to peers where
	// peerHeight - maxAge < evidenceHeight < peerHeight
	// and
	// lastBlockTime - maxDuration < evidenceTime
	var (
		peerHeight = peerState.GetHeight()

		params = evR.evpool.State().ConsensusParams.Evidence

		ageDuration  = evR.evpool.State().LastBlockTime - ev.Time()
		ageNumBlocks = peerHeight - evHeight
	)

	if peerHeight < evHeight { // peer is behind. sleep while he catches up
		return nil, true
	} else if ageNumBlocks > params.MaxAgeNumBlocks ||
		ageDuration > uint64(params.MaxAgeDuration) { // evidence is too old, skip

		// NOTE: if evidence is too old for an honest peer, then we're behind and
		// either it already got committed or it never will!
		evR.Logger.Info("Not sending peer old evidence",
			"peerHeight", peerHeight,
			"evHeight", evHeight,
			"maxAgeNumBlocks", params.MaxAgeNumBlocks,
			"lastBlockTime", evR.evpool.State().LastBlockTime,
			"evTime", ev.Time(),
			"maxAgeDuration", params.MaxAgeDuration,
			"peer", peer,
		)

		return nil, false
	}

	// send evidence
	msg = &ListMessage{[]types.Evidence{ev}}
	return msg, false
}

// Protocol ...
type Protocol interface {
	StopPeerForError(*p2p.Peer, error)
}

// PeerList ...
type PeerList interface {
	List() []*p2p.Peer
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() uint64
}

//-----------------------------------------------------------------------------
// Messages

// Message is a message sent or received by the Reactor.
type Message interface {
	ValidateBasic() error
}

//-------------------------------------

// ListMessage contains a list of evidence.
type ListMessage struct {
	Evidence []types.Evidence
}

// ValidateBasic performs basic validation.
func (m *ListMessage) ValidateBasic() error {
	for i, ev := range m.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}
	return nil
}

type storageListMsg struct {
	Evidence [][]byte
}

// EncodeRLP implement rlp
func (m *ListMessage) EncodeRLP(w io.Writer) error {
	smsg := &storageListMsg{Evidence: make([][]byte, len(m.Evidence))}
	for i, ev := range m.Evidence {
		evBytes, err := types.EvidenceToBytes(ev)
		if err != nil {
			return err
		}
		smsg.Evidence[i] = evBytes
	}
	return rlp.Encode(w, smsg)
}

// DecodeRLP implement rlp
func (m *ListMessage) DecodeRLP(s *rlp.Stream) error {
	var err error
	smsg := &storageListMsg{Evidence: make([][]byte, 0)}
	if err := s.Decode(smsg); err != nil {
		return err
	}
	evd := make([]types.Evidence, len(smsg.Evidence))
	for i, evBytes := range smsg.Evidence {
		evd[i], err = types.EvidenceFromBytes(evBytes)
		if err != nil {
			return err
		}
	}
	m.Evidence = evd
	return nil
}

// String returns a string representation of the ListMessage.
func (m *ListMessage) String() string {
	return fmt.Sprintf("[ListMessage %v]", m.Evidence)
}

// decodemsg takes an array of bytes
// returns an array of evidence
func decodeMsg(msg p2p.Msg) (evis []types.Evidence, err error) {
	list := ListMessage{}
	if err := msg.Decode(&list); err != nil {
		return nil, err
	}
	return list.Evidence, nil
}
