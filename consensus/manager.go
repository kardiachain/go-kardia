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
	"reflect"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	cstypes "github.com/kardiachain/go-kardiamain/consensus/types"
	service "github.com/kardiachain/go-kardiamain/kai/service/const"
	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p"
	"github.com/kardiachain/go-kardiamain/types"

	tmcons "github.com/kardiachain/go-kardiamain/proto/kardiachain/consensus"
	tmproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

const (
	StateChannel       = byte(0x20)
	DataChannel        = byte(0x21)
	VoteChannel        = byte(0x22)
	VoteSetBitsChannel = byte(0x23)

	maxMsgSize = 1048576 // 1MB; NOTE/TODO: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer = 10000
	votesToContributeToBecomeGoodPeer  = 10000
)

// ConsensusManager defines a manager for the consensus service.
type ConsensusManager struct {
	p2p.BaseReactor // BaseService + p2p.Switch
	conS            *ConsensusState
	mtx             sync.RWMutex
}

// NewConsensusManager returns a new ConsensusManager with the given
// consensusState.
func NewConsensusManager(consensusState *ConsensusState) *ConsensusManager {
	conR := &ConsensusManager{
		conS: consensusState,
	}
	conR.BaseReactor = *p2p.NewBaseReactor("Consensus", conR)
	return conR
}

func (conR *ConsensusManager) SetPrivValidator(priv types.PrivValidator) {
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

func (conR *ConsensusManager) OnStart() error {
	conR.Logger.Trace("Consensus manager starts!")
	conR.subscribeToBroadcastEvents()
	conR.conS.Start()
	return nil
}

func (conR *ConsensusManager) OnStop() {
	conR.conS.Stop()
	conR.unsubscribeFromBroadcastEvents()
}

// GetChannels implements Reactor
func (conR *ConsensusManager) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		{
			ID:                  StateChannel,
			Priority:            5,
			SendQueueCapacity:   100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID: DataChannel, // maybe split between gossiping current block and catchup stuff
			// once we gossip the whole block there's nothing left to send until next height or round
			Priority:            10,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  VoteChannel,
			Priority:            5,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  100 * 100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  VoteSetBitsChannel,
			Priority:            1,
			SendQueueCapacity:   2,
			RecvBufferCapacity:  1024,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// InitPeer implements Reactor by creating a state for the peer.
func (conR *ConsensusManager) InitPeer(peer p2p.Peer) p2p.Peer {
	peerState := NewPeerState(peer).SetLogger(conR.Logger)
	peer.Set(types.PeerStateKey, peerState)
	return peer
}

// AddPeer implements manager
func (conR *ConsensusManager) AddPeer(peer p2p.Peer) {
	conR.Logger.Info("Add peer to manager", "peer", peer)

	if !conR.IsRunning() {
		return
	}

	peerState, ok := peer.Get(types.PeerStateKey).(*PeerState)
	if !ok {
		panic(fmt.Sprintf("peer %v has no state", peer))
	}

	// Begin routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)
	go conR.queryMaj23Routine(peer, peerState)

	// Send our state to peer.
	// If we're fast_syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
	conR.sendNewRoundStepMessage(peer)

}

// RemovePeer is a noop.
func (conR *ConsensusManager) RemovePeer(p p2p.Peer, reason interface{}) {
	conR.Logger.Warn("ConsensusManager.RemovePeer - not yet implemented")
}

// Receive implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
// Messages affect either a peer state or the consensus state.
// Peer state updates can happen in parallel, but processing of
// proposals, block parts, and votes are ordered by the receiveRoutine
// NOTE: blocks on consensus state for proposals, block parts, and votes
func (conR *ConsensusManager) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	if !conR.IsRunning() {
		conR.Logger.Debug("Receive", "src", src, "chId", chID, "bytes", msgBytes)
		return
	}

	msg, err := decodeMsg(msgBytes)
	if err != nil {
		conR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		conR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		conR.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		conR.Switch.StopPeerForError(src, err)
		return
	}

	conR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps, ok := src.Get(types.PeerStateKey).(*PeerState)
	if !ok {
		panic(fmt.Sprintf("Peer %v has no state", src))
	}

	switch chID {
	case StateChannel:
		switch msg := msg.(type) {
		case *NewRoundStepMessage:
			ps.ApplyNewRoundStepMessage(msg)
		case *NewValidBlockMessage:
			ps.ApplyNewValidBlockMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		case *VoteSetMaj23Message:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()
			if height != msg.Height {
				return
			}
			// Peer claims to have a maj23 for some BlockID at H,R,S,
			err := votes.SetPeerMaj23(msg.Round, msg.Type, ps.peer.ID(), msg.BlockID)
			if err != nil {
				conR.Switch.StopPeerForError(src, err)
				return
			}
			// Respond with a VoteSetBitsMessage showing which votes we have.
			// (and consequently shows which we don't have)
			var ourVotes *cmn.BitArray
			switch msg.Type {
			case tmproto.PrevoteType:
				ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
			case tmproto.PrecommitType:
				ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
			default:
				panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
			}
			src.TrySend(VoteSetBitsChannel, MustEncode(&VoteSetBitsMessage{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: msg.BlockID,
				Votes:   ourVotes,
			}))
		default:
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}
	case DataChannel:
		switch msg := msg.(type) {
		case *ProposalMessage:
			ps.SetHasProposal(msg.Proposal)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		case *ProposalPOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, int(msg.Part.Index))
			//conR.Metrics.BlockParts.With("peer_id", string(src.ID())).Add(1)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		default:
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}
	case VoteChannel:
		switch msg := msg.(type) {
		case *VoteMessage:
			cs := conR.conS
			cs.mtx.RLock()
			height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
			cs.mtx.RUnlock()
			ps.EnsureVoteBitArrays(height, valSize)
			ps.EnsureVoteBitArrays(height-1, lastCommitSize)
			ps.SetHasVote(msg.Vote)

			cs.peerMsgQueue <- msgInfo{msg, src.ID()}

		default:
			// don't punish (leave room for soft upgrades)
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}
	case VoteSetBitsChannel:
		switch msg := msg.(type) {
		case *VoteSetBitsMessage:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()

			if height == msg.Height {
				var ourVotes *cmn.BitArray
				switch msg.Type {
				case tmproto.PrevoteType:
					ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
				case tmproto.PrecommitType:
					ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
				default:
					panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
				}
				ps.ApplyVoteSetBitsMessage(msg, ourVotes)
			} else {
				ps.ApplyVoteSetBitsMessage(msg, nil)
			}
		default:
			// don't punish (leave room for soft upgrades)
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}
	default:
		conR.Logger.Error(fmt.Sprintf("Unknown chId %X", chID))
	}
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

	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventValidBlock,
		func(data EventData) {
			conR.broadcastNewValidBlockMessage(data.(*cstypes.RoundState))
		})
}

func (conR *ConsensusManager) unsubscribeFromBroadcastEvents() {
	const subscriber = "consensus-manager"
	conR.conS.evsw.RemoveListener(subscriber)
}

// ------------ Broadcast messages ------------

func (conR *ConsensusManager) broadcastNewRoundStepMessages(rs *cstypes.RoundState) {
	nrsMsg := makeRoundStepMessage(rs)
	conR.Logger.Trace("broadcastNewRoundStepMessage", "nrsMsg", nrsMsg, "height", rs.Height)
	conR.Switch.Broadcast(StateChannel, MustEncode(nrsMsg))
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusManager) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	conR.Logger.Trace("broadcastHasVoteMessage", "msg", msg)
	conR.Switch.Broadcast(StateChannel, MustEncode(msg))
}

func (conR *ConsensusManager) broadcastNewValidBlockMessage(rs *cstypes.RoundState) {
	msg := &NewValidBlockMessage{
		Height:           rs.Height,
		Round:            rs.Round,
		BlockPartsHeader: rs.ProposalBlockParts.Header(),
		BlockParts:       rs.ProposalBlockParts.BitArray(),
		IsCommit:         rs.Step == cstypes.RoundStepCommit,
	}
	conR.Switch.Broadcast(StateChannel, MustEncode(msg))
}

// ------------ Send message helpers -----------

func (conR *ConsensusManager) sendNewRoundStepMessage(peer p2p.Peer) {
	conR.Logger.Debug("manager - sendNewRoundStepMessages")
	rs := conR.conS.GetRoundState()
	nrsMsg := makeRoundStepMessage(rs)
	peer.Send(StateChannel, MustEncode(nrsMsg))
}

// ------------ Helpers to create messages -----
func makeRoundStepMessage(rs *cstypes.RoundState) (nrsMsg *NewRoundStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  rs.Step,
		SecondsSinceStartTime: uint64(time.Since(time.Unix(int64(rs.StartTime), 0)).Seconds()),
		LastCommitRound:       rs.LastCommit.GetRound(),
	}
	return
}

// ----------- Gossip routines ---------------
func (conR *ConsensusManager) gossipDataRoutine(peer p2p.Peer, ps *PeerState) {
	logger := conR.Logger.New("peer", peer)
	logger.Trace("Start gossipDataRoutine for peer")

OuterLoop:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
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
				logger.Info("Sending block part", "height", prs.Height, "round", prs.Round, "msg code", service.CsProposalBlockPartMsg)
				if peer.Send(DataChannel, MustEncode(msg)) {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				}
				continue OuterLoop
			}
		}

		// If the peer is on a previous height, help catch up.
		if prs.Height > 0 && prs.Height < rs.Height {

			// if we never received the commit message from the peer, the block parts wont be initialized
			if prs.ProposalBlockParts == nil {
				blockMeta := conR.conS.blockOperations.LoadBlockMeta(prs.Height)
				if blockMeta == nil {
					panic(fmt.Sprintf("Failed to load block %d when blockOperations is at %d",
						prs.Height, conR.conS.blockOperations.Height()))
				}
				ps.InitProposalBlockParts(blockMeta.BlockID.PartsHeader)
				continue OuterLoop
			}
			conR.gossipDataForCatchup(rs, prs, ps, peer)
			continue OuterLoop
		}

		// If height and round don't match, sleep.
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			logger.Trace("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
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
				msg := &ProposalMessage{Proposal: rs.Proposal}
				logger.Debug("Sending proposal", "height", prs.Height, "round", prs.Round)
				if peer.Send(DataChannel, MustEncode(msg)) {
					// NOTE[ZM]: A peer might have received different proposal msg so this Proposal msg will be rejected!
					ps.SetHasProposal(rs.Proposal)
				}
			}
			// ProposalPOL: lets peer know which POL votes we have so far.
			// Peer must receive ProposalMessage first.
			// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
			// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
			if rs.Proposal.POLRound > 0 {
				msg := &ProposalPOLMessage{
					Height:           rs.Height,
					ProposalPOLRound: rs.Proposal.POLRound,
					ProposalPOL:      rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray(),
				}
				logger.Debug("Sending POL", "height", prs.Height, "round", prs.Round)
				peer.Send(DataChannel, MustEncode(msg))
			}
			continue OuterLoop
		}

		// Nothing to do. Sleep.
		time.Sleep(conR.conS.config.PeerGossipSleep())
		continue OuterLoop
	}
}

func (conR *ConsensusManager) gossipDataForCatchup(rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState, ps *PeerState, peer p2p.Peer) {

	if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
		// Ensure that the peer's PartSetHeader is correct
		blockMeta := conR.conS.blockOperations.LoadBlockMeta(prs.Height)
		if blockMeta == nil {
			conR.Logger.Error("Failed to load block meta",
				"ourHeight", rs.Height, "blockstoreHeight", conR.conS.blockOperations.Height())
			time.Sleep(conR.conS.config.PeerGossipSleep())
			return
		}
		if !blockMeta.BlockID.PartsHeader.Equals(prs.ProposalBlockPartsHeader) {
			conR.Logger.Info("Peer ProposalBlockPartsHeader mismatch, sleeping",
				"blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleep())
			return
		}
		// Load the part
		part := conR.conS.blockOperations.LoadBlockPart(prs.Height, index)
		if part == nil {
			conR.Logger.Error("Could not load part", "index", index,
				"blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleep())
			return
		}

		// Send the part
		msg := &BlockPartMessage{
			Height: prs.Height, // Not our height, so it doesn't matter.
			Round:  prs.Round,  // Not our height, so it doesn't matter.
			Part:   part,
		}
		conR.Logger.Debug("Sending block part for catchup", "round", prs.Round, "index", index)
		if peer.Send(DataChannel, MustEncode(msg)) {
			ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
		} else {
			conR.Logger.Debug("Sending block part for catchup failed")
		}
		return
	}
	//logger.Info("No parts to send in catch-up, sleeping")
	time.Sleep(conR.conS.config.PeerGossipSleep())
}

func (conR *ConsensusManager) gossipVotesRoutine(peer p2p.Peer, ps *PeerState) {
	logger := conR.Logger.New("peer", peer)
	logger.Trace("Start gossipVotesRoutine for peer")

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Info("Stopping gossipDataRoutine for peer")
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
		if rs.Height == prs.Height {
			heightLogger := logger.New("height", prs.Height)
			if conR.gossipVotesForHeight(heightLogger, rs, prs, ps) {
				continue OUTER_LOOP
			}
		}

		// Special catchup logic.
		// If peer is lagging by height 1, send LastCommit.
		if (prs.Height != 0) && (rs.Height == prs.Height+1) {
			if ps.PickSendVote(rs.LastCommit) {
				logger.Debug("Picked rs.LastCommit to send", "height", prs.Height)
				continue OUTER_LOOP
			}
		}

		// Catchup logic
		// If peer is lagging by more than 1, send Commit.
		if (prs.Height != 0) && (rs.Height >= prs.Height+2) {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			commit := conR.conS.blockOperations.LoadBlockCommit(prs.Height)
			if ps.PickSendVote(commit) {
				logger.Debug("Picked Catchup commit to send", "height", prs.Height)
				continue OUTER_LOOP
			}
		}

		if sleeping == 0 {
			// We sent nothing. Sleep...
			sleeping = 1
			logger.Debug("No votes to send, sleeping", "rs.Height", rs.Height, "prs.Height", prs.Height,
				"localPV", rs.Votes.Prevotes(rs.Round).BitArray(), "peerPV", prs.Prevotes,
				"localPC", rs.Votes.Precommits(rs.Round).BitArray(), "peerPC", prs.Precommits)
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
	if (prs.Step <= cstypes.RoundStepPropose) && (prs.Round != 0) && (prs.Round <= rs.Round) && (prs.ProposalPOLRound != 0) {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}
	// If there are prevotes to send...
	if (prs.Step <= cstypes.RoundStepPrevoteWait) && (prs.Round != 0) && (prs.Round <= rs.Round) {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommitWait && (prs.Round != 0) && (prs.Round <= rs.Round) {
		if ps.PickSendVote(rs.Votes.Precommits(prs.Round)) {
			logger.Debug("Picked rs.Precommits(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are prevotes to send...Needed because of validBlock mechanism
	if (prs.Round != 0) && (prs.Round <= rs.Round) {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are POLPrevotes to send...
	if prs.ProposalPOLRound != 0 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}

	return false
}

func (conR *ConsensusManager) queryMaj23Routine(peer p2p.Peer, ps *PeerState) {
	logger := conR.Logger.New("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Info("Stopping gossipDataRoutine for peer")
			return
		}

		// Send Height/Round/Prevotes
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    tmproto.PrevoteType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/Round/Precommits
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    tmproto.PrecommitType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/Round/ProposalPOL
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if (rs.Height == prs.Height) && (prs.ProposalPOLRound >= 0) {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    tmproto.PrevoteType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Send Height/CatchupCommitRound/CatchupCommit.
		{
			prs := ps.GetRoundState()
			if (prs.CatchupCommitRound != 0) && (prs.Height > 0) && (prs.Height <= conR.conS.blockOperations.Height()) {
				commit := conR.conS.LoadCommit(prs.Height)
				if commit != nil {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   commit.Round,
						Type:    tmproto.PrecommitType,
						BlockID: commit.BlockID,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}

			}
		}

		time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())

		continue OUTER_LOOP
	}
}

//-----------------------------------------------------------------------------
// Messages

// Message is a message that can be sent and received on the Reactor
type Message interface {
	ValidateBasic() error
}

// ----------- Consensus Messages ------------

// ConsensusMessage is a message that can be sent and received on the ConsensusManager
type ConsensusMessage interface{}

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *types.Vote
}

// ValidateBasic performs basic validation.
func (m *VoteMessage) ValidateBasic() error {
	return m.Vote.ValidateBasic()
}

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal *types.Proposal
}

// ValidateBasic performs basic validation.
func (m *ProposalMessage) ValidateBasic() error {
	return nil
}

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           uint64
	ProposalPOLRound uint32
	ProposalPOL      *cmn.BitArray
}

// String returns a string representation.
func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

// ValidateBasic performs basic validation.
func (m *ProposalPOLMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.ProposalPOLRound < 0 {
		return errors.New("negative ProposalPOLRound")
	}
	if m.ProposalPOL.Size() == 0 {
		return errors.New("empty ProposalPOL bit array")
	}
	return nil
}

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                uint64                `json:"height" gencodoc:"required"`
	Round                 uint32                `json:"round" gencodoc:"required"`
	Step                  cstypes.RoundStepType `json:"step" gencodoc:"required"`
	SecondsSinceStartTime uint64                `json:"elapsed" gencodoc:"required"`
	LastCommitRound       uint32                `json:"lastCommitRound" gencodoc:"required"`
}

// ValidateBasic performs basic validation.
func (m *NewRoundStepMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	// if !m.Step.IsValid() {
	// 	return errors.New("invalid Step")
	// }

	// NOTE: SecondsSinceStartTime may be negative

	// LastCommitRound will be -1 for the initial height, but we don't know what height this is
	// since it can be specified in genesis. The reactor will have to validate this via
	// ValidateHeight().
	// if m.LastCommitRound < -1 {
	// 	return errors.New("invalid LastCommitRound (cannot be < -1)")
	// }

	return nil
}

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height uint64
	Round  uint32
	Type   tmproto.SignedMsgType
	Index  uint32
}

// ValidateBasic performs basic validation.
func (m *HasVoteMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if m.Index < 0 {
		return errors.New("negative Index")
	}
	return nil
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%v/%v(%v)}]", m.Index, m.Height, m.Round, m.Type, types.GetReadableVoteTypeString(m.Type))
}

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  uint64
	Round   uint32
	Type    tmproto.SignedMsgType
	BlockID types.BlockID
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%v/%v(%v) %v]", m.Height, m.Round, m.Type, types.GetReadableVoteTypeString(m.Type), m.BlockID)
}

// ValidateBasic performs basic validation.
func (m *VoteSetMaj23Message) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	return nil
}

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  uint64
	Round   uint32
	Type    tmproto.SignedMsgType
	BlockID types.BlockID
	Votes   *cmn.BitArray
}

// ValidateBasic performs basic validation.
func (m *VoteSetBitsMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// NOTE: Votes.Size() can be zero if the node does not have any
	if m.Votes.Size() > types.MaxVotesCount {
		return fmt.Errorf("votes bit array is too big: %d, max: %d", m.Votes.Size(), types.MaxVotesCount)
	}
	return nil
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02v/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

// ---------  PeerState ---------
// PeerState contains the known state of a peer, including its connection and
// threadsafe access to its PeerRoundState.
// NOTE: THIS GETS DUMPED WITH rpc/core/consensus.go.
// Be mindful of what you Expose.
type PeerState struct {
	peer   p2p.Peer
	logger log.Logger

	mtx sync.Mutex             // NOTE: Modify below using setters, never directly.
	PRS cstypes.PeerRoundState `json:"round_state"` // Exposed.
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer p2p.Peer) *PeerState {
	return &PeerState{
		peer: peer,
		PRS: cstypes.PeerRoundState{
			Height:             0,
			Round:              0,
			ProposalPOLRound:   0,
			LastCommitRound:    0,
			CatchupCommitRound: 0,
			StartTime:          0,
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

	if (ps.PRS.Height != proposal.Height) || (ps.PRS.Round != proposal.Round) {
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
	ps.PRS.ProposalBlockParts = cmn.NewBitArray(int(proposal.POLBlockID.PartsHeader.Total))
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
	ps.PRS.ProposalBlockParts = cmn.NewBitArray(int(partsHeader.Total))
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
func (ps *PeerState) SetHasProposalBlockPart(height uint64, round uint32, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if (ps.PRS.Height != height) || (ps.PRS.Round != round) {
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
		if ps.peer.Send(VoteChannel, MustEncode(msg)) {
			ps.SetHasVote(vote)
			return true
		}
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

	height, round, signedMsgType, size := votes.GetHeight(), votes.GetRound(), votes.Type(), votes.Size()

	// Lazily set data using 'votes'.
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, signedMsgType)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		ps.setHasVote(height, round, signedMsgType, uint32(index))
		return votes.GetByIndex(uint32(index)), true
	}
	return nil, false
}

// ApplyNewValidBlockMessage updates the peer state for the new valid block.
func (ps *PeerState) ApplyNewValidBlockMessage(msg *NewValidBlockMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}

	if ps.PRS.Round != msg.Round && !msg.IsCommit {
		return
	}

	ps.PRS.ProposalBlockPartsHeader = msg.BlockPartsHeader
	ps.PRS.ProposalBlockParts = msg.BlockParts
}

func (ps *PeerState) getVoteBitArray(height uint64, round uint32, signedMsgType tmproto.SignedMsgType) *cmn.BitArray {
	if !types.IsVoteTypeValid(signedMsgType) {
		return nil
	}

	if ps.PRS.Height == height {
		if ps.PRS.Round == round {
			switch signedMsgType {
			case tmproto.PrevoteType:
				return ps.PRS.Prevotes
			case tmproto.PrecommitType:
				return ps.PRS.Precommits
			}
		}
		if ps.PRS.CatchupCommitRound == round {
			switch signedMsgType {
			case tmproto.PrevoteType:
				return nil
			case tmproto.PrecommitType:
				return ps.PRS.CatchupCommit
			}
		}
		if ps.PRS.ProposalPOLRound == round {
			switch signedMsgType {
			case tmproto.PrevoteType:
				return ps.PRS.ProposalPOL
			case tmproto.PrecommitType:
				return nil
			}
		}
		return nil
	}
	if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommitRound == round {
			switch signedMsgType {
			case tmproto.PrevoteType:
				return nil
			case tmproto.PrecommitType:
				return ps.PRS.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height uint64, round uint32, numValidators int) {
	if ps.PRS.Height != height {
		return
	}
	if ps.PRS.CatchupCommitRound == round {
		return
	}
	ps.PRS.CatchupCommitRound = round
	if round == ps.PRS.Round {
		ps.PRS.CatchupCommit = ps.PRS.Precommits
	} else {
		ps.PRS.CatchupCommit = cmn.NewBitArray(numValidators)
	}
}

// EnsureVoteBitArrays ensures the bit-arrays have been allocated for tracking
// what votes this peer has received.
// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height uint64, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height uint64, numValidators int) {
	if ps.PRS.Height == height {
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
	} else if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommit == nil {
			ps.PRS.LastCommit = cmn.NewBitArray(numValidators)
		}
	}
}

// SetHasVote sets the given vote as known by the peer
func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(uint64(vote.Height), vote.Round, vote.Type, vote.ValidatorIndex)
}

func (ps *PeerState) setHasVote(height uint64, round uint32, signedMsgType tmproto.SignedMsgType, index uint32) {
	//logger := ps.logger.New("peerH/R", cmn.Fmt("%v/%v", ps.PRS.Height, ps.PRS.Round))
	ps.logger.Debug("setHasVote", "H/R", cmn.Fmt("%v/%v", height, round), "type", types.GetReadableVoteTypeString(signedMsgType), "index", index)

	psVotes := ps.getVoteBitArray(height, round, signedMsgType)
	if psVotes != nil {
		psVotes.SetIndex(int(index), true)
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

	startTime := time.Now().Unix() - int64(msg.SecondsSinceStartTime)
	ps.PRS.Height = msg.Height
	ps.PRS.Round = msg.Round
	ps.PRS.Step = msg.Step
	ps.PRS.StartTime = uint64(startTime)
	if (psHeight != msg.Height) || (psRound != msg.Round) {
		ps.PRS.Proposal = false
		ps.PRS.ProposalBlockPartsHeader = types.PartSetHeader{}
		ps.PRS.ProposalBlockParts = nil
		ps.PRS.ProposalPOLRound = 0
		ps.PRS.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.PRS.Prevotes = nil
		ps.PRS.Precommits = nil
	}
	if (psHeight == msg.Height) && (psRound != msg.Round) && (msg.Round == psCatchupCommitRound) {
		ps.PRS.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Precommits to LastCommit.
		if (psHeight+1 == msg.Height) && (psRound == msg.LastCommitRound) {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = ps.PRS.Precommits
		} else {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = nil
		}
		ps.PRS.CatchupCommitRound = 0
		ps.PRS.CatchupCommit = nil
	}
}

// ApplyHasVoteMessage updates the peer state for the new vote.
func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
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

// ApplyProposalPOLMessage updates the peer state for the new proposal POL.
func (ps *PeerState) ApplyProposalPOLMessage(msg *ProposalPOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}
	if ps.PRS.ProposalPOLRound != msg.ProposalPOLRound {
		return
	}

	// TODO: Merge onto existing ps.PRS.ProposalPOL?
	// We might have sent some prevotes in the meantime.
	ps.PRS.ProposalPOL = msg.ProposalPOL
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
	Height uint64
	Round  uint32
	Part   *types.Part
}

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
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

//-------------------------------------

// NewValidBlockMessage is sent when a validator observes a valid block B in some round r,
//i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type NewValidBlockMessage struct {
	Height           uint64
	Round            uint32
	BlockPartsHeader types.PartSetHeader
	BlockParts       *cmn.BitArray
	IsCommit         bool
}

// ValidateBasic performs basic validation.
func (m *NewValidBlockMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if err := m.BlockPartsHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockPartsHeader: %v", err)
	}
	if m.BlockParts.Size() == 0 {
		return errors.New("Empty BlockParts")
	}
	if m.BlockParts.Size() != int(m.BlockPartsHeader.Total) {
		return fmt.Errorf("BlockParts bit array size %d not equal to BlockPartsHeader.Total %d",
			m.BlockParts.Size(),
			m.BlockPartsHeader.Total)
	}
	if m.BlockParts.Size() > types.MaxBlockPartsCount {
		return fmt.Errorf("BlockParts bit array is too big: %d, max: %d", m.BlockParts.Size(), types.MaxBlockPartsCount)
	}
	return nil
}

// String returns a string representation.
func (m *NewValidBlockMessage) String() string {
	return fmt.Sprintf("[ValidBlockMessage H:%v R:%v BP:%v BA:%v IsCommit:%v]",
		m.Height, m.Round, m.BlockPartsHeader, m.BlockParts, m.IsCommit)
}

func decodeMsg(bz []byte) (msg Message, err error) {
	pb := &tmcons.Message{}
	if err = proto.Unmarshal(bz, pb); err != nil {
		return msg, err
	}

	return MsgFromProto(pb)
}
