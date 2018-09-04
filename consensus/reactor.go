package consensus

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	kcmn "github.com/kardiachain/go-kardia/kai/common"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	libevents "github.com/kardiachain/go-kardia/lib/events"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p"
	"github.com/kardiachain/go-kardia/types"
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

func (conR *ConsensusReactor) Validator() *types.Validator {
	if _, val := conR.conS.Validators.GetByAddress(conR.conS.privValidator.GetAddress()); val != nil {
		return val
	}
	return nil
}

func (conR *ConsensusReactor) Validators() []*types.Validator {
	return conR.conS.Validators.Validators
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
	go conR.queryMaj23Routine(p, peerState)
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
	conR.conS.Logger.Warn("Implement RecordVote here to mark peer as good.")

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
		conR.conS.Logger.Error("Invalid HasVoteMessage", "msg", generalMsg, "err", err)
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

func (conR *ConsensusReactor) ReceiveProposalPOL(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received ProposalPOLMessage", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg ProposalPOLMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid ProposalPOLMessage", "msg", generalMsg, "err", err)
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

	if !ps.PRS.Height.Equals(msg.Height) {
		return
	}
	if !ps.PRS.ProposalPOLRound.Equals(msg.ProposalPOLRound) {
		return
	}

	ps.PRS.ProposalPOL = msg.ProposalPOL
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

func (conR *ConsensusReactor) ReceiveBlock(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received block", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg BlockMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid BlockMessage", "msg", generalMsg, "err", err)
		return
	}
	conR.conS.Logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(p2p.PeerStateKey).(*PeerState)
	if !ok {
		conR.conS.Logger.Error("Downcast failed!!")
		return
	}

	ps.SetProposalBlock(msg.Height, msg.Round, msg.Block.Header().Hash())
	conR.conS.peerMsgQueue <- msgInfo{&msg, src.ID()}
}

func (conR *ConsensusReactor) ReceiveVoteSetMaj23(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received VoteSetMaj23", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg VoteSetMaj23Message
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid VoteSetMaj23Message", "msg", generalMsg, "err", err)
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
	height, votes := cs.Height, cs.Votes
	cs.mtx.Unlock()
	if !height.Equals(msg.Height) {
		return
	}
	// Peer claims to have a maj23 for some BlockID at H,R,S,
	err := votes.SetPeerMaj23(msg.Round.Int32(), msg.Type, ps.peer.ID(), msg.BlockID)
	if err != nil {
		conR.conS.Logger.Error("SetPeerMaj23 failed", "err", err)
		return
	}
	// Respond with a VoteSetBitsMessage showing which votes we have.
	// (and consequently shows which we don't have)
	var ourVotes *cmn.BitArray
	switch msg.Type {
	case types.VoteTypePrevote:
		ourVotes = votes.Prevotes(msg.Round.Int32()).BitArrayByBlockID(msg.BlockID)
	case types.VoteTypePrecommit:
		ourVotes = votes.Precommits(msg.Round.Int32()).BitArrayByBlockID(msg.BlockID)
	default:
		conR.conS.Logger.Error("Bad VoteSetBitsMessage field Type")
		return
	}
	p2p.Send(ps.rw, kcmn.CsVoteSetBitsMessage, &VoteSetBitsMessage{
		Height:  msg.Height,
		Round:   msg.Round,
		Type:    msg.Type,
		BlockID: msg.BlockID,
		Votes:   ourVotes,
	})
}

func (conR *ConsensusReactor) ReceiveVoteSetBits(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.conS.Logger.Trace("Consensus reactor received VoteSetBits", "src", src, "msg", generalMsg)

	if !conR.running {
		conR.conS.Logger.Trace("Consensus reactor isn't running.")
		return
	}

	var msg VoteSetBitsMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.conS.Logger.Error("Invalid VoteSetBitsMessage", "msg", generalMsg, "err", err)
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
	height, votes := cs.Height, cs.Votes
	cs.mtx.Unlock()

	if height.Equals(msg.Height) {
		var ourVotes *cmn.BitArray
		switch msg.Type {
		case types.VoteTypePrevote:
			ourVotes = votes.Prevotes(msg.Round.Int32()).BitArrayByBlockID(msg.BlockID)
		case types.VoteTypePrecommit:
			ourVotes = votes.Precommits(msg.Round.Int32()).BitArrayByBlockID(msg.BlockID)
		default:
			conR.conS.Logger.Error("Bad VoteSetBitsMessage field Type")
			return
		}
		ps.ApplyVoteSetBitsMessage(&msg, ourVotes)
	} else {
		ps.ApplyVoteSetBitsMessage(&msg, nil)
	}
}

// ------------ Broadcast messages ------------

func (conR *ConsensusReactor) broadcastNewRoundStepMessages(rs *cstypes.RoundState) {
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		conR.conS.Logger.Trace("broadcastNewRoundStepMessage", "nrsMsg", nrsMsg)
		conR.protocol.Broadcast(nrsMsg, kcmn.CsNewRoundStepMsg)
	}
	if csMsg != nil {
		conR.conS.Logger.Trace("broadcastCommitStepMessage", "csMsg", csMsg)
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
		conR.conS.Logger.Trace("Send CommitStepMsg", "csMsg", csMsg)
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

		// If the peer is on a previous height, help catch up.
		if prs.Height.IsGreaterThanInt(0) && prs.Height.IsLessThan(rs.Height) {
			block := conR.conS.blockOperations.LoadBlock(uint64(prs.Height.Int64()))
			logger.Trace("Sending BlockMessage", "height", prs.Height, "block", block)
			if err := p2p.Send(ps.rw, kcmn.CsBlockMsg, &BlockMessage{Height: prs.Height, Round: prs.Round, Block: block}); err != nil {
				logger.Trace("Sending block message failed", "err", err)
			}
			time.Sleep(conR.conS.config.PeerGossipSleep())
			continue OUTER_LOOP
		}

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
				logger.Debug("Sending proposal", "height", prs.Height, "round", prs.Round)
				if err := p2p.Send(ps.rw, kcmn.CsProposalMsg, &ProposalMessage{Proposal: rs.Proposal}); err != nil {
					logger.Trace("Sending proposal failed", "err", err)
				}
				ps.SetHasProposal(rs.Proposal)
			}
			// ProposalPOL: lets peer know which POL votes we have so far.
			// Peer must receive ProposalMessage first.
			// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
			// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
			if rs.Proposal.POLRound.IsGreaterThanOrEqualToInt(0) {
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
		if !prs.Height.EqualsInt(0) && rs.Height.IsGreaterThanInt64(prs.Height.Int64()+2) {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			commit := conR.conS.blockOperations.LoadBlockCommit(uint64(prs.Height.Int64()))
			if ps.PickSendVote(commit) {
				logger.Debug("Picked Catchup commit to send", "height", prs.Height)
				continue OUTER_LOOP
			}
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

func (conR *ConsensusReactor) queryMaj23Routine(peer *p2p.Peer, ps *PeerState) {
	logger := conR.conS.Logger.New("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsAlive || !conR.running {
			logger.Info("Stopping queryMaj23Routine for peer")
			return
		}

		// Send Height/Round/Prevotes
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height.Equals(prs.Height) {
				if maj23, ok := rs.Votes.Prevotes(prs.Round.Int32()).TwoThirdsMajority(); ok {
					p2p.Send(ps.rw, kcmn.CsVoteSetMaj23Message, &VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.VoteTypePrevote,
						BlockID: maj23,
					})
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/Round/Precommits
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height.Equals(prs.Height) {
				if maj23, ok := rs.Votes.Precommits(prs.Round.Int32()).TwoThirdsMajority(); ok {
					p2p.Send(ps.rw, kcmn.CsVoteSetMaj23Message, &VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.VoteTypePrecommit,
						BlockID: maj23,
					})
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/Round/ProposalPOL
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height.Equals(prs.Height) && prs.ProposalPOLRound.IsGreaterThanOrEqualToInt(0) {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound.Int32()).TwoThirdsMajority(); ok {
					p2p.Send(ps.rw, kcmn.CsVoteSetMaj23Message, &VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    types.VoteTypePrevote,
						BlockID: maj23,
					})
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/CatchupCommitRound/CatchupCommit.
		{
			prs := ps.GetRoundState()
			if !prs.CatchupCommitRound.EqualsInt(-1) && prs.Height.IsGreaterThanInt(0) && prs.Height.IsLessThanInt64(int64(conR.conS.blockOperations.Height())) {
				commit := conR.conS.blockOperations.LoadBlockCommit(uint64(prs.Height.Int64()))
				p2p.Send(ps.rw, kcmn.CsVoteSetMaj23Message, &VoteSetMaj23Message{
					Height:  prs.Height,
					Round:   commit.Round(),
					Type:    types.VoteTypePrecommit,
					BlockID: commit.BlockID,
				})
				time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
			}
		}

		time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())

		continue OUTER_LOOP
	}
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

// BlockMessage is sent when gossipping block.
type BlockMessage struct {
	Height *cmn.BigInt
	Round  *cmn.BigInt
	Block  *types.Block
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

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  *cmn.BigInt
	Round   *cmn.BigInt
	Type    byte
	BlockID types.BlockID
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02v/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  *cmn.BigInt
	Round   *cmn.BigInt
	Type    byte
	BlockID types.BlockID
	Votes   *cmn.BitArray
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02v/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
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

// SetProposalBlock sets the given block as known for the peer.
func (ps *PeerState) SetProposalBlock(height *cmn.BigInt, round *cmn.BigInt, blockHeader cmn.Hash) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if !ps.PRS.Height.Equals(height) || !ps.PRS.Round.Equals(round) {
		return
	}

	ps.PRS.ProposalBlockHeader = blockHeader
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
		if ps.PRS.CatchupCommitRound.Equals(round) {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.PRS.CatchupCommit
			}
		}
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
	if ps.PRS.CatchupCommitRound.Equals(round) {
		return
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

// ApplyVoteSetBitsMessage updates the peer state for the bit-array of votes
// it claims to have for the corresponding BlockID.
// `ourVotes` is a BitArray of votes we have for msg.BlockID
// NOTE: if ourVotes is nil (e.g. msg.Height < rs.Height),
// we conservatively overwrite ps's votes w/ msg.Votes.
func (ps *PeerState) ApplyVoteSetBitsMessage(msg *VoteSetBitsMessage, ourVotes *cmn.BitArray) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	votes := ps.getVoteBitArray(msg.Height, msg.Round, msg.Type)
	if votes != nil {
		if ourVotes == nil {
			votes.Update(msg.Votes)
		} else {
			otherVotes := votes.Sub(ourVotes)
			hasVotes := otherVotes.Or(msg.Votes)
			votes.Update(hasVotes)
		}
	}
}

// String returns a string representation of the PeerState
func (ps *PeerState) String() string {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf("PeerState{Key:%v  RoundState:%v}",
		ps.peer.ID(),
		ps.PRS)
}
