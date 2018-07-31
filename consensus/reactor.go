package consensus

import (
	"sync"
	"time"

	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	cmn "github.com/kardiachain/go-kardia/lib/common"
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

func (pc *PeerConnection) SendConsensusMessage(msg ConsensusMessage) error {
	return p2p.Send(pc.rw, kcmn.CsMsg, msg)
}

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	//p2p.BaseReactor // BaseService + p2p.Switch

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
	//return conR
}

func (conR *ConsensusReactor) Start() {
	log.Error("ConsensusReactor.Start - not yet implemented.")

	conR.running = true
}

func (conR *ConsensusReactor) Stop() {
	log.Error("ConsensusReactor.Stop - not yet implemented.")

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

// ------------ Message handlers ---------

// Handles received NewRoundStepMessage
func (conR *ConsensusReactor) ReceiveNewRoundStepMessage(msg NewRoundStepMessage, src *p2p.Peer) {
	log.Trace("Consensus reactor receive", "src", src, "msg", msg)

	if !conR.running {
		log.Trace("Consensus reactor isn't running.")
		return
	}

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		log.Error("Downcast failed!!")
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

// ------------ Message Helpers -----------

func (conR *ConsensusReactor) sendNewRoundStepMessages(pc PeerConnection) {
	log.Debug("reactor - sendNewRoundStepMessages")
	nrsMsg := &NewRoundStepMessage{
		Height: cmn.NewBigInt(0),
		Round:  cmn.NewBigInt(0),
		Step:   0,
		SecondsSinceStartTime: 10,
		LastCommitRound:       cmn.NewBigInt(0),
	}

	if err := pc.SendConsensusMessage(nrsMsg); err != nil {
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
