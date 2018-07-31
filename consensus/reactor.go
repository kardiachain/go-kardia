package consensus

import (
	"sync"

	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/types"
)

const (
	StateChannel       = byte(0x20)
	DataChannel        = byte(0x21)
	VoteChannel        = byte(0x22)
	VoteSetBitsChannel = byte(0x23)

	maxMsgSize = 1048576 // 1MB; NOTE/TODO: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer = 10000
)

type PeerConnection struct {
	peer *p2p.Peer
	rw   p2p.MsgReadWriter
}

type consensusMessageAndChannel struct {
	channelID byte        `json:"channelID" gencodoc:"required"`
	msg       interface{} `json:"msg" gencodoc:"required"`
}

func (pc *PeerConnection) SendConsensusMessage(msg ConsensusMessage) error {
	//return p2p.Send(pc.rw, kcmn.CsMsg, msg)
	return p2p.Send(pc.rw, kcmn.CsMsg, &NewRoundStepMessage{
		Height: 0,
		Round:  0,
		Step:   0,
		SecondsSinceStartTime: 10,
		LastCommitRound:       0,
	})
}

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	//p2p.BaseReactor // BaseService + p2p.Switch

	conS *ConsensusState

	mtx sync.RWMutex
	//eventBus *types.EventBus
}

// NewConsensusReactor returns a new ConsensusReactor with the given
// consensusState.
func NewConsensusReactor(consensusState *ConsensusState) *ConsensusReactor {
	return &ConsensusReactor{
		conS: nil,
	}
	// TODO(namdoh): Re-anable this.
	//conR := &ConsensusReactor{
	//	conS:     consensusState,
	//	fastSync: fastSync,
	//}
	//conR.BaseReactor = *p2p.NewBaseReactor("ConsensusReactor", conR)
	//return conR
}

// Receive implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
// Messages affect either a peer state or the consensus state.
// Peer state updates can happen in parallel, but processing of
// proposals, block parts, and votes are ordered by the receiveRoutine
// NOTE: blocks on consensus state for proposals, block parts, and votes
func (conR *ConsensusReactor) Receive(msg ConsensusMessage, src *p2p.Peer) {
	log.Trace("Consensus reactor receive", "src", src, "msg", msg)
}

// AddPeer implements Reactor
func (conR *ConsensusReactor) AddPeer(p *p2p.Peer, rw p2p.MsgReadWriter) {
	log.Info("Add peer to reactor.")
	peerConnection := PeerConnection{peer: p, rw: rw}
	conR.sendNewRoundStepMessages(peerConnection)

	// TODO(namdoh): Re-anable this.
	//if !conR.IsRunning() {
	//	return
	//}
	//
	//// Create peerState for peer
	//peerState := NewPeerState(peer).SetLogger(conR.Logger)
	//peer.Set(types.PeerStateKey, peerState)
	//
	//// Begin routines for this peer.
	//go conR.gossipDataRoutine(peer, peerState)
	//go conR.gossipVotesRoutine(peer, peerState)
	//go conR.queryMaj23Routine(peer, peerState)
	//
	//// Send our state to peer.
	//// If we're fast_syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
	//if !conR.FastSync() {
	//	conR.sendNewRoundStepMessages(peer)
	//}
}

func (conR *ConsensusReactor) sendNewRoundStepMessages(pc PeerConnection) {
	log.Debug("reactor - sendNewRoundStepMessages")
	nrsMsg := &NewRoundStepMessage{
		Height: 0,
		Round:  0,
		Step:   0,
		SecondsSinceStartTime: 10,
		LastCommitRound:       0,
	}

	if err := pc.SendConsensusMessage(&consensusMessageAndChannel{
		channelID: StateChannel,
		msg:       nrsMsg,
	}); err != nil {
		log.Debug("sendNewRoundStepMessages failed", "err", err)
	}

	// TODO(namdoh): Re-anable this.
	//rs := conR.conS.GetRoundState()
	//nrsMsg, csMsg := makeRoundStepMessages(rs)
	//if nrsMsg != nil {
	//	peer.Send(StateChannel, cdc.MustMarshalBinaryBare(nrsMsg))
	//}
	//if csMsg != nil {
	//	peer.Send(StateChannel, cdc.MustMarshalBinaryBare(csMsg))
	//}
}

// -------- Consensus Messages ------------
// Specifies an event in BFT consensus to send between peers. We prefer to use
// oneof instead of flatten all events. However, we have to settle with this
// suboptimal choice since our serialization is done by RLP instead of protobuf.
// TODO(namdoh): Improve this with something cleaner/more efficient.
type ConsensusEvent struct {
	ChannelID byte             `json:"channelID" gencodoc:"required"`
	Msg       ConsensusMessage `json:"msg" gencodoc:"required"`
}

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface{}

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *types.Vote
}

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal *types.Proposal
}

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                uint64                `json:"height" gencodoc:"required"`
	Round                 uint64                `json:"round" gencodoc:"required"`
	Step                  cstypes.RoundStepType `json:"step" gencodoc:"required"`
	SecondsSinceStartTime uint64                `json:"elapsed" gencodoc:"required"`
	LastCommitRound       uint64                `json:"lastCommitRound" gencodoc:"required"`
}
