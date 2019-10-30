/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package consensus

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	service "github.com/kardiachain/go-kardia/kai/service/const"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/types"
)

// ConsensusManager defines a manager for the consensus service.
type ConsensusManager struct {
	id     string     // Uniquely identifies this consensus service
	logger log.Logger // Please use this logger for all consensus activities.

	protocol BaseProtocol

	conS *ConsensusState

	mtx sync.RWMutex
	//eventBus *types.EventBus

	running bool
}

// NewConsensusManager returns a new ConsensusManager with the given
// consensusState.
func NewConsensusManager(id string, consensusState *ConsensusState) *ConsensusManager {
	return &ConsensusManager{
		id:     id,
		logger: consensusState.logger,
		conS:   consensusState,
	}
}

func (conR *ConsensusManager) SetProtocol(protocol BaseProtocol) {
	conR.protocol = protocol
}

func (conR *ConsensusManager) SetPrivValidator(priv *types.PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

func (conR *ConsensusManager) Validator() *types.Validator {
	if _, val := conR.conS.Validators.GetByAddress(conR.conS.privValidator.GetAddress()); val != nil {
		return val
	}
	return nil
}

func (conR *ConsensusManager) Validators() []*types.Validator {
	return conR.conS.Validators.CurrentValidators()
}

func (conR *ConsensusManager) Start() {
	conR.logger.Trace("Consensus manager starts!")

	if conR.running {
		conR.logger.Error("ConsensusManager already started. Shouldn't start again.")
		return
	}
	conR.running = true

	conR.subscribeToBroadcastEvents()
	conR.conS.Start()
}

func (conR *ConsensusManager) Stop() {
	if !conR.running {
		conR.logger.Error("ConsensusManager hasn't started yet. Shouldn't be asked to stop.")
	}

	conR.conS.Stop()
	conR.unsubscribeFromBroadcastEvents()

	conR.running = false
	conR.logger.Trace("Consensus manager stops!")
}

// AddPeer implements manager
func (conR *ConsensusManager) AddPeer(p *p2p.Peer, rw p2p.MsgReadWriter) {
	conR.logger.Info("Add peer to manager", "peer", p)
	conR.sendNewRoundStepMessages(rw)

	if !conR.running {
		return
	}

	//// Create peerState for peer
	peerState := NewPeerState(p, rw).SetLogger(conR.logger)
	p.Set(conR.GetPeerStateKey(), peerState)

	// Begin routines for this peer.
	go conR.gossipDataRoutine(p, peerState)
	go conR.gossipVotesRoutine(p, peerState)
	go conR.queryMaj23Routine(p, peerState)
}

func (conR *ConsensusManager) RemovePeer(p *p2p.Peer, reason interface{}) {
	conR.logger.Warn("ConsensusManager.RemovePeer - not yet implemented")
}

func (conR *ConsensusManager) GetPeerStateKey() string {
	return conR.id + "." + p2p.PeerStateKey
}

// subscribeToBroadcastEvents subscribes for new round steps, votes and
// proposal heartbeats using internal pubsub defined on state to broadcast
// them to peers upon receiving.
func (conR *ConsensusManager) subscribeToBroadcastEvents() {
	const subscriber = "consensus-manager"
	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventNewRoundStep,
		func(data EventData) {
			conR.broadcastNewRoundStepMessages(data.(*cstypes.RoundState))
		})

	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventVote,
		func(data EventData) {
			conR.broadcastHasVoteMessage(data.(*types.Vote))
		})
}

func (conR *ConsensusManager) unsubscribeFromBroadcastEvents() {
	const subscriber = "consensus-manager"
	conR.conS.evsw.RemoveListener(subscriber)
}

// ------------ Message handlers ---------

// Handles received NewRoundStepMessage
func (conR *ConsensusManager) ReceiveNewRoundStep(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received NewRoundStep", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg NewRoundStepMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid message", "msg", generalMsg, "err", err)
		return
	}
	conR.logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}

	ps.ApplyNewRoundStepMessage(&msg)
}

func (conR *ConsensusManager) ReceiveNewBlockPart(generalMsg p2p.Msg, src *p2p.Peer) {
	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}
	conR.logger.Trace("Consensus manager received Block Part", "peer", src)

	var msg BlockPartMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid block part message", "msg", generalMsg, "err", err)
		return
	}

	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}
	ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Index.Int32())
	conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
}

func (conR *ConsensusManager) ReceiveNewProposal(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received Proposal", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg ProposalMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid proposal message", "msg", generalMsg, "err", err)
		return
	}
	msg.Proposal.Block.SetLogger(conR.logger)
	proposal := msg.Proposal
	conR.logger.Trace("Decoded msg",
		"proposalHeight", proposal.Height,
		"blockHeight", proposal.Block.Height(),
		"round", proposal.Round,
		"POLRound", proposal.POLRound,
	)
	if msg.Proposal.Block.LastCommit() == nil {
		msg.Proposal.Block.SetLastCommit(&types.Commit{})
	}

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}

	ps.SetHasProposal(msg.Proposal)
	conR.conS.peerMsgQueue <- msgInfo{&msg, src.ID()}
}

func (conR *ConsensusManager) ReceiveBlock(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received block", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg BlockMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid BlockMessage", "msg", generalMsg, "err", err)
		return
	}
	msg.Block.SetLogger(conR.logger)

	conR.logger.Trace("Decoded msg", "msg", fmt.Sprintf("Height:%v   Round:%v   Block:%v", msg.Height, msg.Round, msg.Block.Height()))

	conR.conS.peerMsgQueue <- msgInfo{&msg, src.ID()}
}

func (conR *ConsensusManager) ReceiveNewVote(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received NewVote", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg VoteMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid vote message", "msg", generalMsg, "err", err)
		return
	}
	conR.logger.Trace("Decoded msg", "msg", msg.Vote)

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}

	cs := conR.conS
	cs.mtx.Lock()
	height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
	cs.mtx.Unlock()
	ps.EnsureVoteBitArrays(height, valSize)
	ps.EnsureVoteBitArrays(height.Add(-1), lastCommitSize)
	ps.SetHasVote(msg.Vote)
	conR.logger.Trace("Implement RecordVote here to mark peer as good.")

	cs.peerMsgQueue <- msgInfo{&msg, src.ID()}
}

func (conR *ConsensusManager) ReceiveHasVote(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received HasVote", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg HasVoteMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid HasVoteMessage", "msg", generalMsg, "err", err)
		return
	}
	conR.logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}

	ps.ApplyHasVoteMessage(&msg)
}

func (conR *ConsensusManager) ReceiveProposalPOL(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received ProposalPOLMessage", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg ProposalPOLMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid ProposalPOLMessage", "msg", generalMsg, "err", err)
		return
	}
	conR.logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
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

func (conR *ConsensusManager) ReceiveNewCommit(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received vote", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg CommitStepMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid commit step message", "msg", generalMsg, "err", err)
		return
	}
	msg.Block.SetLogger(conR.logger)

	conR.logger.Trace("Decoded msg", "msg", fmt.Sprintf("{Height:%v  Block:%v}", msg.Height, msg.Block.Height()))

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}

	ps.ApplyCommitStepMessage(&msg)
}

func (conR *ConsensusManager) ReceiveVoteSetMaj23(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received VoteSetMaj23Message", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg VoteSetMaj23Message
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid VoteSetMaj23Message", "msg", generalMsg, "err", err)
		return
	}
	conR.logger.Trace("Decoded msg", "msg", &msg)

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
		return
	}

	cs := conR.conS
	cs.mtx.Lock()
	height, votes := cs.Height, cs.Votes
	cs.mtx.Unlock()
	if !height.Equals(msg.Height) {
		conR.logger.Trace("ReceiveVoteSetMaj23 - height doesn't match", "height", height, "msg.Height", msg.Height)
		return
	}
	// Peer claims to have a maj23 for some BlockID at H,R,S,
	err := votes.SetPeerMaj23(msg.Round.Int32(), msg.Type, ps.peer.ID(), msg.BlockID)
	if err != nil {
		conR.logger.Error("SetPeerMaj23 failed", "err", err)
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
		conR.logger.Error("Bad VoteSetMaj23Message field Type")
		return
	}
	go p2p.Send(ps.rw, service.CsVoteSetBitsMessage, &VoteSetBitsMessage{
		Height:  msg.Height,
		Round:   msg.Round,
		Type:    msg.Type,
		BlockID: msg.BlockID,
		Votes:   ourVotes,
	})
}

func (conR *ConsensusManager) ReceiveVoteSetBits(generalMsg p2p.Msg, src *p2p.Peer) {
	conR.logger.Trace("Consensus manager received VoteSetBits", "peer", src)

	if !conR.running {
		conR.logger.Trace("Consensus manager isn't running.")
		return
	}

	var msg VoteSetBitsMessage
	if err := generalMsg.Decode(&msg); err != nil {
		conR.logger.Error("Invalid VoteSetBitsMessage", "msg", generalMsg, "err", err)
		return
	}
	conR.logger.Trace("Decoded msg", "msg", msg)

	// Get peer states
	ps, ok := src.Get(conR.GetPeerStateKey()).(*PeerState)
	if !ok {
		conR.logger.Error("Downcast failed!!")
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
			conR.logger.Error("Bad VoteSetBitsMessage field Type")
			return
		}
		ps.ApplyVoteSetBitsMessage(&msg, ourVotes)
	} else {
		ps.ApplyVoteSetBitsMessage(&msg, nil)
	}
}

// ------------ Broadcast messages ------------

func (conR *ConsensusManager) broadcastNewRoundStepMessages(rs *cstypes.RoundState) {
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		conR.logger.Trace("broadcastNewRoundStepMessage", "nrsMsg", nrsMsg)
		go conR.protocol.Broadcast(nrsMsg, service.CsNewRoundStepMsg)
	}
	if csMsg != nil {
		conR.logger.Trace("broadcastCommitStepMessage", "csMsg", fmt.Sprintf("{Height:%v  Block:%v}", csMsg.Height, csMsg.Block.Hash().Hex()))
		go conR.protocol.Broadcast(csMsg, service.CsCommitStepMsg)
	}
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusManager) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	conR.logger.Trace("broadcastHasVoteMessage", "msg", msg)
	conR.protocol.Broadcast(msg, service.CsHasVoteMsg)
}

// ------------ Send message helpers -----------

func (conR *ConsensusManager) sendNewRoundStepMessages(rw p2p.MsgReadWriter) {
	conR.logger.Debug("manager - sendNewRoundStepMessages")

	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	conR.logger.Trace("makeRoundStepMessages", "nrsMsg", nrsMsg)
	if nrsMsg != nil {
		go func() {
			if err := p2p.Send(rw, service.CsNewRoundStepMsg, nrsMsg); err != nil {
				conR.logger.Warn("send NewRoundStepMessage failed", "err", err)
			} else {
				conR.logger.Trace("send NewRoundStepMessage success")
			}
		}()
	}

	if csMsg != nil {
		go func() {
			conR.logger.Trace("Send CommitStepMsg", "csMsg", csMsg)
			if err := p2p.Send(rw, service.CsCommitStepMsg, csMsg); err != nil {
				conR.logger.Warn("send CommitStepMessage failed", "err", err)
			} else {
				conR.logger.Trace("send CommitStepMessage success")
			}
		}()
	}
}

// ------------ Helpers to create messages -----
func makeRoundStepMessages(rs *cstypes.RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  rs.Step,
		SecondsSinceStartTime: uint(time.Now().Unix() - rs.StartTime.Int64()),
		LastCommitRound:       rs.LastCommit.Round(),
	}
	if rs.Step == cstypes.RoundStepCommit && rs.ProposalBlock != nil {
		csMsg = &CommitStepMessage{
			Height: rs.Height,
			Block:  rs.ProposalBlock,
		}
	}
	return
}

// ----------- Gossip routines ---------------
func (conR *ConsensusManager) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {
	logger := conR.logger.New("peer", peer)
	logger.Trace("Start gossipDataRoutine for peer")

OuterLoop:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsAlive || !conR.running {
			logger.Info("Stopping gossipDataRoutine for peer")
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartsHeader) {
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)
				msg := &BlockPartMessage{
					Height: rs.Height, // This tells peer that this part applies to us.
					Round:  rs.Round,  // This tells peer that this part applies to us.
					Part:   part,
				}
				logger.Debug("Sending block part", "height", prs.Height, "round", prs.Round)
				if err := p2p.Send(ps.rw, service.CsProposalBlockPartMsg, msg); err != nil {
					logger.Trace("Sending block part failed", "err", err)
				}
				ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				continue OuterLoop
			}
		}

		// If the peer is on a previous height, help catch up.
		if prs.Height.IsGreaterThanInt(0) && prs.Height.IsLessThan(rs.Height) {
			// if we never received the commit message from the peer, the block parts wont be initialized
			if prs.ProposalBlockParts == nil {
				lastCommit := conR.conS.blockOperations.LoadBlockCommit(prs.Height.Uint64())
				if lastCommit == nil {
					panic(fmt.Sprintf("Failed to load block %d when blockStore is at %d",
						prs.Height.Int64(), conR.conS.blockOperations.Height()))
				}
				ps.InitProposalBlockParts(lastCommit.BlockID.PartsHeader)
				continue OuterLoop
			}
			conR.gossipDataForCatchup(rs, prs, ps)
			continue OuterLoop
		}

		// If height and round don't match, sleep.
		if !rs.Height.Equals(prs.Height) || !rs.Round.Equals(prs.Round) {
			//logger.Trace("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(conR.conS.config.PeerGossipSleep())
			continue OuterLoop
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
				ps.SetHasProposal(rs.Proposal)

				// proposal contains block data, therefore, it will cause bottle neck here if there are thounsands of txs inside.
				// add it into goroutine to prevent bottleneck
				go func() {
					if err := p2p.Send(ps.rw, service.CsProposalMsg, &ProposalMessage{Proposal: rs.Proposal}); err != nil {
						logger.Trace("Sending proposal failed", "err", err)
					}
				}()
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
				go func() {
					if err := p2p.Send(ps.rw, service.CsProposalPOLMsg, msg); err != nil {
						logger.Error("Sending proposalPOLMsg failed", "err", err)
					}
				}()
			}
			continue OuterLoop
		}

		// Nothing to do. Sleep.
		time.Sleep(conR.conS.config.PeerGossipSleep())
		continue OuterLoop
	}
}

func (conR *ConsensusManager) gossipDataForCatchup(rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState, ps *PeerState) {

	if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
		// Ensure that the peer's PartSetHeader is correct
		commit := conR.conS.blockOperations.LoadBlockCommit(prs.Height.Uint64())
		if commit == nil {
			conR.logger.Error("Failed to load block meta",
				"ourHeight", rs.Height, "blockstoreHeight", conR.conS.blockOperations.Height())
			time.Sleep(time.Duration(conR.conS.config.PeerGossipSleepDuration))
			return
		} else if !commit.BlockID.PartsHeader.Equals(prs.ProposalBlockPartsHeader) {
			conR.logger.Info("Peer ProposalBlockPartsHeader mismatch, sleeping",
				"blockPartsHeader", commit.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		}
		// Load the part
		part := conR.conS.blockOperations.LoadBlockPart(prs.Height.Int64(), index)
		if part == nil {
			conR.logger.Error("Could not load part", "index", index,
				"blockPartsHeader", commit.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		}

		// Send the part
		msg := &BlockPartMessage{
			Height: prs.Height, // Not our height, so it doesn't matter.
			Round:  prs.Round,  // Not our height, so it doesn't matter.
			Part:   part,
		}
		conR.logger.Debug("Sending block part for catchup", "round", prs.Round, "index", index)
		if err := p2p.Send(ps.rw, service.CsProposalBlockPartMsg, msg); err != nil {
			conR.logger.Trace("Sending block part failed", "err", err)
		}
		return
	}
	//logger.Info("No parts to send in catch-up, sleeping")
	time.Sleep(time.Duration(conR.conS.config.PeerGossipSleepDuration))
}

func (conR *ConsensusManager) gossipVotesRoutine(peer *p2p.Peer, ps *PeerState) {
	logger := conR.logger.New("peer", peer)
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
		if !prs.Height.EqualsInt(0) && rs.Height.IsGreaterThanOrEqualToInt64(prs.Height.Int64()+2) {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			commit := conR.conS.blockOperations.LoadBlockCommit(prs.Height.Uint64())
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

func (conR *ConsensusManager) gossipVotesForHeight(logger log.Logger, rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) bool {
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

func (conR *ConsensusManager) queryMaj23Routine(peer *p2p.Peer, ps *PeerState) {
	logger := conR.logger.New("peer", peer)

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
					go func() {
						if err := p2p.Send(ps.rw, service.CsVoteSetMaj23Message, &VoteSetMaj23Message{
							Height:  prs.Height,
							Round:   prs.Round,
							Type:    types.VoteTypePrevote,
							BlockID: maj23,
						}); err != nil {
							logger.Error("error while sending Height/Round/Prevotes", "err", err)
						}
					}()
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
					go func() {
						if err := p2p.Send(ps.rw, service.CsVoteSetMaj23Message, &VoteSetMaj23Message{
							Height:  prs.Height,
							Round:   prs.Round,
							Type:    types.VoteTypePrecommit,
							BlockID: maj23,
						}); err != nil {
							logger.Error("error while sending Height/Round/Precommits", "err", err)
						}
					}()
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
					go func() {
						if err := p2p.Send(ps.rw, service.CsVoteSetMaj23Message, &VoteSetMaj23Message{
							Height:  prs.Height,
							Round:   prs.ProposalPOLRound,
							Type:    types.VoteTypePrevote,
							BlockID: maj23,
						}); err != nil {
							logger.Error("error while sending Height/Round/ProposalPOL", "err", err)
						}
					}()
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/CatchupCommitRound/CatchupCommit.
		{
			prs := ps.GetRoundState()
			if !prs.CatchupCommitRound.EqualsInt(-1) && prs.Height.IsGreaterThanInt(0) && prs.Height.IsLessThanOrEqualsUint64(conR.conS.blockOperations.Height()) {
				commit := conR.conS.LoadCommit(prs.Height)
				if commit != nil {
					go func() {
						if err := p2p.Send(ps.rw, service.CsVoteSetMaj23Message, &VoteSetMaj23Message{
							Height:  prs.Height,
							Round:   commit.Round(),
							Type:    types.VoteTypePrecommit,
							BlockID: commit.BlockID,
						}); err != nil {
							logger.Error("error while sending Height/CatchupCommitRound/CatchupCommit", "err", err)
						}
					}()
				}
				time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
			}
		}

		time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())

		continue OUTER_LOOP
	}
}

// ----------- Consensus Messages ------------

// ConsensusMessage is a message that can be sent and received on the ConsensusManager
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
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%v/%v(%v)}]", m.Index, m.Height, m.Round, m.Type, types.GetReadableVoteTypeString(m.Type))
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
	return fmt.Sprintf("[VSM23 %v/%v/%v(%v) %v]", m.Height, m.Round, m.Type, types.GetReadableVoteTypeString(m.Type), m.BlockID)
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
	peer   *p2p.Peer
	rw     p2p.MsgReadWriter
	logger log.Logger

	mtx sync.Mutex             // NOTE: Modify below using setters, never directly.
	PRS cstypes.PeerRoundState `json:"round_state"` // Exposed.
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer *p2p.Peer, rw p2p.MsgReadWriter) *PeerState {
	return &PeerState{
		peer: peer,
		rw:   rw,
		PRS: cstypes.PeerRoundState{
			Height:             cmn.NewBigInt32(0),
			Round:              cmn.NewBigInt32(-1),
			ProposalPOLRound:   cmn.NewBigInt32(-1),
			LastCommitRound:    cmn.NewBigInt32(-1),
			CatchupCommitRound: cmn.NewBigInt32(-1),
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

	if ps.PRS.Height != proposal.Height || ps.PRS.Round != proposal.Round {
		return
	}

	if ps.PRS.Proposal {
		return
	}

	ps.PRS.Proposal = true

	// ps.PRS.ProposalBlockParts is set due to NewValidBlockMessage
	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartsHeader = proposal.POLBlockID.PartsHeader
	ps.PRS.ProposalBlockParts = cmn.NewBitArray(proposal.POLBlockID.PartsHeader.Total.Int32())
	ps.PRS.ProposalPOLRound = proposal.POLRound
	ps.PRS.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

// InitProposalBlockParts initializes the peer's proposal block parts header and bit array.
func (ps *PeerState) InitProposalBlockParts(partsHeader types.PartSetHeader) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartsHeader = partsHeader
	ps.PRS.ProposalBlockParts = cmn.NewBitArray(partsHeader.Total.Int32())
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
func (ps *PeerState) SetHasProposalBlockPart(height *cmn.BigInt, round *cmn.BigInt, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if !ps.PRS.Height.Equals(height) || !ps.PRS.Round.Equals(round) {
		return
	}

	ps.PRS.ProposalBlockParts.SetIndex(index, true)
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) bool {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}

		ps.logger.Trace("Sending vote message", "peer", ps.peer, "prs", ps.PRS, "vote", vote)
		return p2p.Send(ps.rw, service.CsVoteMsg, msg) == nil
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
		ps.setHasVote(height, round, type_, cmn.NewBigInt32(index))
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
	//logger := ps.logger.New("peerH/R", cmn.Fmt("%v/%v", ps.PRS.Height, ps.PRS.Round))
	ps.logger.Debug("setHasVote", "H/R", cmn.Fmt("%v/%v", height, round), "type", types.GetReadableVoteTypeString(type_), "index", index)

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
		ps.PRS.ProposalBlockParts = nil
		ps.PRS.ProposalPOLRound = cmn.NewBigInt32(-1)
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
		ps.PRS.CatchupCommitRound = cmn.NewBigInt32(-1)
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

//-------------------------------------

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height *cmn.BigInt
	Round  *cmn.BigInt
	Part   *types.Part
}

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height.IsLessThanInt(0) {
		return errors.New("Negative Height")
	}
	if m.Round.IsLessThanInt(0) {
		return errors.New("Negative Round")
	}
	if err := m.Part.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong Part: %v", err)
	}
	return nil
}

// String returns a string representation.
func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}
