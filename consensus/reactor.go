package consensus

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	kcmn "github.com/kardiachain/go-kardia/kai/common" // TODO(namdoh): Remove kai/common dependency
	cmn "github.com/kardiachain/go-kardia/lib/common"
	libevents "github.com/kardiachain/go-kardia/lib/events"
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

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	protocol BaseProtocol

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
}

func (conR *ConsensusReactor) SetProtocol(protocol BaseProtocol) {
	conR.protocol = protocol
}

func (conR *ConsensusReactor) SetPrivValidator(priv *types.PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

func (conR *ConsensusReactor) Start() {
	conR.conS.Logger.Trace("Consensus reactor starts!")

	if conR.running {
		conR.conS.Logger.Error("ConsensusReactor already started. Shouldn't start again.")
		return
	}
	conR.running = true

	conR.subscribeToBroadcastEvents()
	conR.conS.Start()
}

func (conR *ConsensusReactor) Stop() {
	if !conR.running {
		conR.conS.Logger.Error("ConsensusReactor hasn't started yet. Shouldn't be asked to stop.")
	}

	conR.conS.Stop()
	conR.unsubscribeFromBroadcastEvents()

	conR.running = false
	conR.conS.Logger.Trace("Consensus reactor stops!")
}

// AddPeer implements Reactor
func (conR *ConsensusReactor) AddPeer(p *p2p.Peer, rw p2p.MsgReadWriter) {
	log.Info("Add peer to reactor.")
	conR.conS.Logger.Error("Find out why passing p.GetRW() doesn't work.")
	conR.sendNewRoundStepMessages(rw)

	if !conR.running {
		return
	}

	//// Create peerState for peer
	peerState := NewPeerState(p, rw).SetLogger(conR.conS.Logger)
	p.Set(p2p.PeerStateKey, peerState)

	// Begin routines for this peer.
	go conR.gossipDataRoutine(p, peerState)
	go conR.gossipVotesRoutine(p, peerState)
	//go conR.queryMaj23Routine(p, peerState)

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

	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventVote,
		func(data libevents.EventData) {
			conR.broadcastHasVoteMessage(data.(*types.Vote))
		})

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

	ps.ApplyNewRoundStepMessage(&msg)
}

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
	// TODO(namdo,issues#73): Remove this hack, which address one of RLP's diosyncrasies.
	msg.Proposal.MakeEmptyNil()
	conR.conS.Logger.Trace("Decoded msg", "msg", msg)
	if msg.Proposal.Block.LastCommit() == nil {
		msg.Proposal.Block.SetLastCommit(&types.Commit{})
	}

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		conR.conS.Logger.Error("Downcast failed!!")
		return
	}

	ps.SetHasProposal(msg.Proposal)
	conR.conS.peerMsgQueue <- msgInfo{&msg, src.ID()}
}

func (conR *ConsensusReactor) ReceiveNewVote(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received NewVote", "src", src, "msg", generalMsg)

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

	cs := conR.conS
	cs.mtx.Lock()
	height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
	cs.mtx.Unlock()
	ps.EnsureVoteBitArrays(height, valSize)
	ps.EnsureVoteBitArrays(height.Add(-1), lastCommitSize)
	ps.SetHasVote(msg.Vote)
	// TODO(namdoh): Reanble this to mark peer as good.
	conR.conS.Logger.Error("RecordVote has yet implemented.")
	//if blocks := ps.RecordVote(msg.Vote); blocks%blocksToContributeToBecomeGoodPeer == 0 {
	//	conR.Switch.MarkPeerAsGood(src)
	//}

	cs.peerMsgQueue <- msgInfo{&msg, src.ID()}
}

func (conR *ConsensusReactor) ReceiveHasVote(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received HasVote", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg HasVoteMessage
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

	ps.ApplyHasVoteMessage(&msg)
}

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

	ps.ApplyCommitStepMessage(&msg)
}

// ------------ Broadcast messages ------------

func (conR *ConsensusReactor) broadcastNewRoundStepMessages(rs *cstypes.RoundState) {
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		conR.conS.Logger.Trace("broadcastNewRoundStepMessages", "nrsMsg", nrsMsg)
		conR.protocol.Broadcast(nrsMsg, kcmn.CsNewRoundStepMsg)
	}
	if csMsg != nil {
		conR.conS.Logger.Trace("broadcastCommitStepMessages", "csMsg", csMsg)
		conR.protocol.Broadcast(csMsg, kcmn.CsCommitStepMsg)
	}
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusReactor) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	conR.conS.Logger.Trace("broadcastHasVoteMessage", "msg", msg)
	conR.protocol.Broadcast(msg, kcmn.CsHasVoteMsg)
	/*
		// TODO: Make this broadcast more selective.
		for _, peer := range conR.Switch.Peers().List() {
			ps := peer.Get(PeerStateKey).(*PeerState)
			prs := ps.GetRoundState()
			if prs.Height == vote.Height {
				// TODO: Also filter on round?
				peer.TrySend(StateChannel, struct{ ConsensusMessage }{msg})
			} else {
				// Height doesn't match
				// TODO: check a field, maybe CatchupCommitRound?
				// TODO: But that requires changing the struct field comment.
			}
		}
	*/
}

// ------------ Send message helpers -----------

func (conR *ConsensusReactor) sendNewRoundStepMessages(rw p2p.MsgReadWriter) {
	conR.conS.Logger.Debug("reactor - sendNewRoundStepMessages")

	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	conR.conS.Logger.Trace("makeRoundStepMessages", "nrsMsg", nrsMsg)
	if nrsMsg != nil {
		if err := p2p.Send(rw, kcmn.CsNewRoundStepMsg, nrsMsg); err != nil {
			conR.conS.Logger.Warn("send NewRoundStepMessage failed", "err", err)
		} else {
			conR.conS.Logger.Trace("send NewRoundStepMessage success")
		}
	}

	if csMsg != nil {
		if err := p2p.Send(rw, kcmn.CsCommitStepMsg, csMsg); err != nil {
			conR.conS.Logger.Warn("send CommitStepMessage failed", "err", err)
		} else {
			conR.conS.Logger.Trace("send CommitStepMessage success")
		}
	}
}

// ------------ Helpers to create messages -----
func makeRoundStepMessages(rs *cstypes.RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step,
		SecondsSinceStartTime: uint(time.Now().Unix() - rs.StartTime.Int64()),
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

// ----------- Gossip routines ---------------
func (conR *ConsensusReactor) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {
	logger := conR.conS.Logger.New("peer", peer)
	logger.Trace("Start gossipDataRoutine for peer")

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsAlive || !conR.running {
			logger.Info("Stopping gossipDataRoutine for peer")
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// If height and round don't match, sleep.
		if !rs.Height.Equals(prs.Height) || !rs.Round.Equals(prs.Round) {
			//logger.Trace("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(conR.conS.config.PeerGossipSleep())
			continue OUTER_LOOP
		}

		// By here, height and round match.
		// Proposal block were already matched and sent if it was wanted.
		// (These can match on hash so the round doesn't matter)
		// Now consider sending other things, like the Proposal itself.

		// Send Proposal && ProposalPOL BitArray?
		if rs.Proposal != nil && !prs.Proposal {
			// Proposal: share the proposal metadata with peer.
			{
				// TODO(namdo,issues#73): Remove this hack, which address one of RLP's diosyncrasies.
				rs.Proposal.MakeNilEmpty()
				logger.Debug("Sending proposal", "height", prs.Height, "round", prs.Round)
				if err := p2p.Send(ps.rw, kcmn.CsProposalMsg, &ProposalMessage{Proposal: rs.Proposal}); err != nil {
					logger.Trace("Sending proposal failed", "err", err)
				}
				// TODO(namdo,issues#73): Remove this hack, which address one of RLP's diosyncrasies.
				rs.Proposal.MakeEmptyNil()
				ps.SetHasProposal(rs.Proposal)
			}
			// ProposalPOL: lets peer know which POL votes we have so far.
			// Peer must receive ProposalMessage first.
			// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
			// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
			if rs.Proposal.POLRound.IsGreaterThanOrEqualThanInt(0) {
				msg := &ProposalPOLMessage{
					Height:           rs.Height,
					ProposalPOLRound: rs.Proposal.POLRound,
					ProposalPOL:      rs.Votes.Prevotes(rs.Proposal.POLRound.Int32()).BitArray(),
				}
				logger.Debug("Sending POL", "height", prs.Height, "round", prs.Round)
				p2p.Send(ps.rw, kcmn.CsProposalPOLMsg, msg)
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(conR.conS.config.PeerGossipSleep())
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesRoutine(peer *p2p.Peer, ps *PeerState) {
	logger := conR.conS.Logger.New("peer", peer)
	logger.Trace("Start gossipVotesRoutine for peer")

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsAlive || !conR.running {
			logger.Info("Stopping gossipVotesRoutine for peer")
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		switch sleeping {
		case 1: // First sleep
			sleeping = 2
		case 2: // No more sleep
			sleeping = 0
		}

		//logger.Trace("gossipVotesRoutine", "rsHeight", rs.Height, "rsRound", rs.Round,
		//	"prsHeight", prs.Height, "prsRound", prs.Round, "prsStep", prs.Step)

		// If height matches, then send LastCommit, Prevotes, Precommits.
		if rs.Height.Equals(prs.Height) {
			heightLogger := logger.New("height", prs.Height)
			if conR.gossipVotesForHeight(heightLogger, rs, prs, ps) {
				continue OUTER_LOOP
			}
		}

		// Special catchup logic.
		// If peer is lagging by height 1, send LastCommit.
		if !prs.Height.EqualsInt(0) && rs.Height.EqualsInt(prs.Height.Int32()+1) {
			if ps.PickSendVote(rs.LastCommit) {
				logger.Debug("Picked rs.LastCommit to send", "height", prs.Height)
				continue OUTER_LOOP
			}
		}

		// Catchup logic
		// If peer is lagging by more than 1, send Commit.
		if !prs.Height.EqualsInt(0) && rs.Height.IsGreaterThanInt(prs.Height.Int32()+2) {
			logger.Error("gossipVotesRoutine- ERROR!!!")
			panic("Catchup isn't implemented yet.")
			// TODO(namdoh): Re-enable this.
			//// Load the block commit for prs.Height,
			//// which contains precommit signatures for prs.Height.
			//commit := conR.conS.blockStore.LoadBlockCommit(prs.Height)
			//if ps.PickSendVote(commit) {
			//	logger.Debug("Picked Catchup commit to send", "height", prs.Height)
			//	continue OUTER_LOOP
			//}
		}

		if sleeping == 0 {
			// We sent nothing. Sleep...
			sleeping = 1
			logger.Debug("No votes to send, sleeping", "rs.Height", rs.Height, "prs.Height", prs.Height,
				"localPV", rs.Votes.Prevotes(rs.Round.Int32()).BitArray(), "peerPV", prs.Prevotes,
				"localPC", rs.Votes.Precommits(rs.Round.Int32()).BitArray(), "peerPC", prs.Precommits)
		} else if sleeping == 2 {
			// Continued sleep...
			sleeping = 1
		}

		time.Sleep(conR.conS.config.PeerGossipSleep())
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesForHeight(logger log.Logger, rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) bool {
	//logger.Trace("Start gossipVotesForHeight for peer")

	// If there are lastCommits to send...
	if prs.Step == cstypes.RoundStepNewHeight {
		if ps.PickSendVote(rs.LastCommit) {
			logger.Debug("Picked rs.LastCommit to send")
			return true
		}
	}
	// If there are POL prevotes to send...
	if prs.Step <= cstypes.RoundStepPropose && !prs.Round.EqualsInt(-1) && prs.Round.IsLessThanOrEquals(rs.Round) && !prs.ProposalPOLRound.EqualsInt(-1) {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound.Int32()); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}
	// If there are prevotes to send...
	if prs.Step <= cstypes.RoundStepPrevoteWait && !prs.Round.EqualsInt(-1) && prs.Round.IsLessThanOrEquals(rs.Round) {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round.Int32())) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommitWait && !prs.Round.EqualsInt(-1) && prs.Round.IsLessThanOrEquals(rs.Round) {
		if ps.PickSendVote(rs.Votes.Precommits(prs.Round.Int32())) {
			logger.Debug("Picked rs.Precommits(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are prevotes to send...Needed because of validBlock mechanism
	if !prs.Round.EqualsInt(-1) && prs.Round.IsLessThanOrEquals(rs.Round) {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round.Int32())) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are POLPrevotes to send...
	if !prs.ProposalPOLRound.EqualsInt(-1) {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound.Int32()); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}

	return false
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

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           *cmn.BigInt
	ProposalPOLRound *cmn.BigInt
	ProposalPOL      *cmn.BitArray
}

// String returns a string representation.
func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
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

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height *cmn.BigInt
	Round  *cmn.BigInt
	Type   byte
	Index  *cmn.BigInt
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%v/%v}]", m.Index, m.Height, m.Round, m.Type)
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
	peer   p2p.Peer
	rw     p2p.MsgReadWriter
	logger log.Logger

	mtx sync.Mutex             `json:"-"`           // NOTE: Modify below using setters, never directly.
	PRS cstypes.PeerRoundState `json:"round_state"` // Exposed.
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer *p2p.Peer, rw p2p.MsgReadWriter) *PeerState {
	return &PeerState{
		peer: *peer,
		rw:   rw,
		PRS: cstypes.PeerRoundState{
			Height:             cmn.NewBigInt(0),
			Round:              cmn.NewBigInt(-1),
			ProposalPOLRound:   cmn.NewBigInt(-1),
			LastCommitRound:    cmn.NewBigInt(-1),
			CatchupCommitRound: cmn.NewBigInt(-1),
			StartTime:          big.NewInt(0),
		},
	}
}

// SetLogger allows to set a logger on the peer state. Returns the peer state
// itself.
func (ps *PeerState) SetLogger(logger log.Logger) *PeerState {
	ps.logger = logger
	return ps
}

// GetRoundState returns an shallow copy of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *cstypes.PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PRS // copy
	return &prs
}

// SetHasProposal sets the given proposal as known for the peer.
func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if !ps.PRS.Height.Equals(proposal.Height) || !ps.PRS.Round.Equals(proposal.Round) {
		return
	}
	if ps.PRS.Proposal {
		return
	}

	ps.PRS.Proposal = true
	ps.PRS.ProposalBlockHeader = proposal.Block.Header().Hash()
	ps.PRS.ProposalPOLRound = proposal.POLRound
	ps.PRS.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) bool {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		ps.logger.Debug("Sending vote message", "ps", ps, "vote", vote)
		return p2p.Send(ps.rw, kcmn.CsVoteMsg, msg) == nil
	}
	return false
}

// PickVoteToSend picks a vote to send to the peer.
// Returns true if a vote was picked.
// NOTE: `votes` must be the correct Size() for the Height().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (vote *types.Vote, ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return nil, false
	}

	height, round, type_, size := votes.Height(), votes.Round(), votes.Type(), votes.Size()

	// Lazily set data using 'votes'.
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		ps.setHasVote(height, round, type_, cmn.NewBigInt(int64(index)))
		return votes.GetByIndex(uint(index)), true
	}
	return nil, false
}

func (ps *PeerState) getVoteBitArray(height *cmn.BigInt, round *cmn.BigInt, type_ byte) *cmn.BitArray {
	if !types.IsVoteTypeValid(type_) {
		return nil
	}

	if ps.PRS.Height.Equals(height) {
		if ps.PRS.Round.Equals(round) {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.PRS.Prevotes
			case types.VoteTypePrecommit:
				return ps.PRS.Precommits
			}
		}
		// TODO(namdoh): Re-eable this once catchup is turned on.
		//if ps.PRS.CatchupCommitRound.Equals(round) {
		//	switch type_ {
		//	case types.VoteTypePrevote:
		//		return nil
		//	case types.VoteTypePrecommit:
		//		return ps.PRS.CatchupCommit
		//	}
		//}
		if ps.PRS.ProposalPOLRound.Equals(round) {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.PRS.ProposalPOL
			case types.VoteTypePrecommit:
				return nil
			}
		}
		return nil
	}
	if ps.PRS.Height.Equals(height.Add(1)) {
		if ps.PRS.LastCommitRound.Equals(round) {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.PRS.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height *cmn.BigInt, round *cmn.BigInt, numValidators int) {
	if !ps.PRS.Height.Equals(height) {
		return
	}
	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommit round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			cmn.PanicSanity(cmn.Fmt("Conflicting CatchupCommitRound. Height: %v, Orig: %v, New: %v", height, ps.CatchupCommitRound, round))
		}
	*/
	if ps.PRS.CatchupCommitRound.Equals(round) {
		return // Nothing to do!
	}
	ps.PRS.CatchupCommitRound = round
	if round.Equals(ps.PRS.Round) {
		ps.PRS.CatchupCommit = ps.PRS.Precommits
	} else {
		ps.PRS.CatchupCommit = cmn.NewBitArray(numValidators)
	}
}

// EnsureVoteBitArrays ensures the bit-arrays have been allocated for tracking
// what votes this peer has received.
// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height *cmn.BigInt, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height *cmn.BigInt, numValidators int) {
	if ps.PRS.Height.Equals(height) {
		if ps.PRS.Prevotes == nil {
			ps.PRS.Prevotes = cmn.NewBitArray(numValidators)
		}
		if ps.PRS.Precommits == nil {
			ps.PRS.Precommits = cmn.NewBitArray(numValidators)
		}
		if ps.PRS.CatchupCommit == nil {
			ps.PRS.CatchupCommit = cmn.NewBitArray(numValidators)
		}
		if ps.PRS.ProposalPOL == nil {
			ps.PRS.ProposalPOL = cmn.NewBitArray(numValidators)
		}
	} else if ps.PRS.Height.EqualsInt(height.Int32() + 1) {
		if ps.PRS.LastCommit == nil {
			ps.PRS.LastCommit = cmn.NewBitArray(numValidators)
		}
	}
}

// SetHasVote sets the given vote as known by the peer
func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, vote.ValidatorIndex)
}

func (ps *PeerState) setHasVote(height *cmn.BigInt, round *cmn.BigInt, type_ byte, index *cmn.BigInt) {
	logger := ps.logger.New("peerH/R", cmn.Fmt("%v/%v", ps.PRS.Height, ps.PRS.Round), "H/R", cmn.Fmt("%v/%v", height, round))
	logger.Debug("setHasVote", "type", type_, "index", index)

	// NOTE: some may be nil BitArrays -> no side effects.
	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes != nil {
		psVotes.SetIndex(index.Int32(), true)
	}
}

// ApplyNewRoundStepMessage updates the peer state for the new round.
func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) {
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

	startTime := big.NewInt(time.Now().Unix() - int64(msg.SecondsSinceStartTime))
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
	if psHeight.Equals(msg.Height) && !psRound.Equals(msg.Round) && msg.Round.Equals(psCatchupCommitRound) {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.PRS.Precommits = psCatchupCommit
	}
	if !psHeight.Equals(msg.Height) {
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

// ApplyCommitStepMessage updates the peer state for the new commit.
func (ps *PeerState) ApplyCommitStepMessage(msg *CommitStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if !ps.PRS.Height.Equals(msg.Height) {
		return
	}

	ps.PRS.ProposalBlockHeader = msg.Block.Header().Hash()
}

// ApplyHasVoteMessage updates the peer state for the new vote.
func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if !ps.PRS.Height.Equals(msg.Height) {
		return
	}

	ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

// String returns a string representation of the PeerState
func (ps *PeerState) String() string {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf("PeerState{Key:%v  RoundState:%v}",
		ps.peer.ID(),
		ps.PRS)
}
