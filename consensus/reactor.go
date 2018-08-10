package consensus

import (
	"sync"
	"time"

	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	"github.com/kardiachain/go-kardia/kai"
	// TODO(namdoh): Remove kai/common dependency
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	libevents "github.com/kardiachain/go-kardia/lib/events"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/p2p/discover"
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

func (pc *PeerConnection) SendConsensusMessage(msg ConsensusMessage) error {
	return p2p.Send(pc.rw, kcmn.CsNewRoundStepMsg, msg)
}

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	kai.BaseReactor

	conS *ConsensusState

	mtx sync.RWMutex
	//eventBus *types.EventBus

	running bool
}

// NewConsensusReactor returns a new ConsensusReactor with the given
// consensusState.
func NewConsensusReactor(consensusState *ConsensusState) *ConsensusReactor {
	return &ConsensusReactor{
		conS: consensusState,
	}
	// TODO(namdoh): Re-anable this.
	//conR := &ConsensusReactor{
	//	conS:     consensusState,
	//	fastSync: fastSync,
	//}
	//conR.BaseReactor = *p2p.NewBaseReactor("ConsensusReactor", conR)
	//r eturn conR
}

func (conR *ConsensusReactor) SetNodeID(nodeID discover.NodeID) {
	conR.conS.SetNodeID(nodeID)
}

func (conR *ConsensusReactor) SetPrivValidator(priv *types.PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

func (conR *ConsensusReactor) Start() {
	conR.running = true

	conR.subscribeToBroadcastEvents()
	conR.conS.Start()
}

func (conR *ConsensusReactor) Stop() {

	conR.conS.Stop()
	conR.unsubscribeFromBroadcastEvents()

	conR.running = false
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
	peerState := NewPeerState(p)
	p.Set(p2p.PeerStateKey, peerState)
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

func (conR *ConsensusReactor) RemovePeer(p *p2p.Peer, reason interface{}) {
	log.Error("ConsensusReactor.RemovePeer - not yet implemented")
}

// subscribeToBroadcastEvents subscribes for new round steps, votes and
// proposal heartbeats using internal pubsub defined on state to broadcast
// them to peers upon receiving.
func (conR *ConsensusReactor) subscribeToBroadcastEvents() {
	const subscriber = "consensus-reactor"
	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventNewRoundStep,
		func(data libevents.EventData) {
			conR.broadcastNewRoundStepMessages(data.(*cstypes.RoundState))
		})

	//namdoh@ conR.conS.evsw.AddListenerForEvent(subscriber, types.EventVote,
	//namdoh@ 	func(data libevents.EventData) {
	//namdoh@ 		conR.broadcastHasVoteMessage(data.(*types.Vote))
	//namdoh@ 	})
	//namdoh@
	//namdoh@ conR.conS.evsw.AddListenerForEvent(subscriber, types.EventProposalHeartbeat,
	//namdoh@ 	func(data libevents.EventData) {
	//namdoh@ 		conR.broadcastProposalHeartbeatMessage(data.(*types.Heartbeat))
	//namdoh@ 	})
}

func (conR *ConsensusReactor) unsubscribeFromBroadcastEvents() {
	const subscriber = "consensus-reactor"
	conR.conS.evsw.RemoveListener(subscriber)
}

// ------------ Message handlers ---------

// Handles received NewRoundStepMessage
func (conR *ConsensusReactor) ReceiveNewRoundStep(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received NewRoundStep", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg NewRoundStepMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid message", "msg", generalMsg, "err", err)
		return
	}
	conR.conS.Logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		conR.conS.Logger.Error("Downcast failed!!")
		return
	}

	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Ignore duplicates or decreases
	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.PRS.Height, ps.PRS.Round, ps.PRS.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.PRS.Height
	psRound := ps.PRS.Round
	//psStep := ps.PRS.Step
	psCatchupCommitRound := ps.PRS.CatchupCommitRound
	psCatchupCommit := ps.PRS.CatchupCommit

	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.PRS.Height = msg.Height
	ps.PRS.Round = msg.Round
	ps.PRS.Step = msg.Step
	ps.PRS.StartTime = startTime
	if !psHeight.Equals(msg.Height) || !psRound.Equals(msg.Round) {
		ps.PRS.Proposal = false
		ps.PRS.ProposalBlockHeader = cmn.Hash{}
		ps.PRS.ProposalPOLRound = cmn.NewBigInt(-1)
		ps.PRS.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.PRS.Prevotes = nil
		ps.PRS.Precommits = nil
	}
	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.PRS.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Precommits to LastCommit.
		if psHeight.Add(1).Equals(msg.Height) && psRound.Equals(msg.LastCommitRound) {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = ps.PRS.Precommits
		} else {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = nil
		}
		// We'll update the BitArray capacity later.
		ps.PRS.CatchupCommitRound = cmn.NewBigInt(-1)
		ps.PRS.CatchupCommit = nil
	}
}

// dummy handler to handle new proposal
func (conR *ConsensusReactor) ReceiveNewProposal(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received Proposal", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg ProposalMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid proposal message", "msg", generalMsg, "err", err)
		return
	}
	conR.conS.Logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		conR.conS.Logger.Error("Downcast failed!!")
		return
	}
	ps.mtx.Lock()
	//handle proposal logic
	return
	defer ps.mtx.Unlock()
}

// dummy handler to handle new vote
func (conR *ConsensusReactor) ReceiveNewVote(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received vote", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg VoteMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid vote message", "msg", generalMsg, "err", err)
		return
	}
	conR.conS.Logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		conR.conS.Logger.Error("Downcast failed!!")
		return
	}
	ps.mtx.Lock()
	//handle vote logic
	return
	defer ps.mtx.Unlock()
}

// dummy handler to handle new commit
func (conR *ConsensusReactor) ReceiveNewCommit(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received vote", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg CommitStepMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid commit step message", "msg", generalMsg, "err", err)
		return
	}
	conR.conS.Logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		conR.conS.Logger.Error("Downcast failed!!")
		return
	}
	ps.mtx.Lock()
	//handle commit logic
	return
	defer ps.mtx.Unlock()
}

// ------------ Broadcast messages ------------

func (conR *ConsensusReactor) broadcastNewRoundStepMessages(rs *cstypes.RoundState) {
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	conR.conS.Logger.Trace("broadcastNewRoundStepMessages", "nrsMsg", nrsMsg)
	if nrsMsg != nil {
		conR.ProtocolManager.Broadcast(nrsMsg)
	}
	if csMsg != nil {
		conR.ProtocolManager.Broadcast(csMsg)
	}
}

// ------------ Send message helpers -----------

func (conR *ConsensusReactor) sendNewRoundStepMessages(pc PeerConnection) {
	conR.conS.Logger.Debug("reactor - sendNewRoundStepMessages")

	rs := conR.conS.GetRoundState()
	nrsMsg, _ := makeRoundStepMessages(rs)
	conR.conS.Logger.Trace("makeRoundStepMessages", "nrsMsg", nrsMsg)
	if nrsMsg != nil {
		if err := pc.SendConsensusMessage(nrsMsg); err != nil {
			conR.conS.Logger.Debug("sendNewRoundStepMessages failed", "err", err)
		} else {
			conR.conS.Logger.Debug("sendNewRoundStepMessages success")
		}
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

// ------------ Helpers to create messages -----
func makeRoundStepMessages(rs *cstypes.RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step,
		SecondsSinceStartTime: uint(time.Since(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.Round(),
	}
	if rs.Step == cstypes.RoundStepCommit {
		csMsg = &CommitStepMessage{
			Height: rs.Height,
			Block:  rs.ProposalBlock,
		}
	}
	return
}

// ----------- Consensus Messages ------------

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
	Height                *cmn.BigInt           `json:"height" gencodoc:"required"`
	Round                 *cmn.BigInt           `json:"round" gencodoc:"required"`
	Step                  cstypes.RoundStepType `json:"step" gencodoc:"required"`
	SecondsSinceStartTime uint                  `json:"elapsed" gencodoc:"required"`
	LastCommitRound       *cmn.BigInt           `json:"lastCommitRound" gencodoc:"required"`
}

// CommitStepMessage is sent when a block is committed.
type CommitStepMessage struct {
	Height *cmn.BigInt  `json:"height" gencodoc:"required"`
	Block  *types.Block `json:"block" gencodoc:"required"`
}

// ---------  PeerState ---------
// PeerState contains the known state of a peer, including its connection and
// threadsafe access to its PeerRoundState.
// NOTE: THIS GETS DUMPED WITH rpc/core/consensus.go.
// Be mindful of what you Expose.
type PeerState struct {
	peer p2p.Peer

	mtx sync.Mutex             `json:"-"`           // NOTE: Modify below using setters, never directly.
	PRS cstypes.PeerRoundState `json:"round_state"` // Exposed.
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{
		peer: *peer,
		PRS: cstypes.PeerRoundState{
			Height:             cmn.NewBigInt(0),
			Round:              cmn.NewBigInt(-1),
			ProposalPOLRound:   cmn.NewBigInt(-1),
			LastCommitRound:    cmn.NewBigInt(-1),
			CatchupCommitRound: cmn.NewBigInt(-1),
		},
	}
}
