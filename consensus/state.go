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
	"bytes"
	"errors"
	"fmt"
	"github.com/kardiachain/go-kardiamain/kai/base"
	"github.com/kardiachain/go-kardiamain/kvm"
	"math/big"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/rlp"

	"github.com/ebuchman/fail-test"

	cfg "github.com/kardiachain/go-kardiamain/configs"
	cstypes "github.com/kardiachain/go-kardiamain/consensus/types"
	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/lib/p2p/discover"
	"github.com/kardiachain/go-kardiamain/types"
)

var (
	msgQueueSize = 1000
)

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
)

// interface to the mempool
type txNotifier interface {
	TxsAvailable() <-chan struct{}
}

// msgs from the manager which may update the state
type msgInfo struct {
	Msg    ConsensusMessage `json:"msg"`
	PeerID discover.NodeID  `json:"peer_key"`
}

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration         `json:"duration"`
	Height   *cmn.BigInt           `json:"height"`
	Round    *cmn.BigInt           `json:"round"`
	Step     cstypes.RoundStepType `json:"step"`
}

// VoteTurn is used for simulate vote strategy
type VoteTurn struct {
	Height   int
	Round    int
	VoteType int
}

func EmptyTimeoutInfo() *timeoutInfo {
	return &timeoutInfo{
		Duration: 0,
		Height:   cmn.NewBigInt32(0),
		Round:    cmn.NewBigInt32(0),
		Step:     0,
	}
}

// ConsensusState handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator,
// and from a timer.
type ConsensusState struct {
	logger log.Logger

	config          *cfg.ConsensusConfig
	privValidator   *types.PrivValidator // for signing votes
	blockOperations BaseBlockOperations
	//evpool evidence.EvidencePool 	// TODO(namdoh): Add mem pool.

	// notify us if txs are available
	txNotifier txNotifier

	// internal state
	mtx sync.RWMutex
	cstypes.RoundState
	state         LastestBlockState // State until height-1.
	timeoutTicker TimeoutTicker

	// State changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo

	// we use eventBus to trigger msg broadcasts in the manager,
	// and to notify external subscribers, eg. through a websocket
	eventBus *types.EventBus

	// For tests where we want to limit the number of transitions the state makes
	nSteps int

	// Synchronous pubsub between consensus state and manager.
	// State only emits EventNewRoundStep, EventVote and EventProposalHeartbeat
	evsw EventSwitch

	// closed when we finish shutting down
	done chan struct{}

	// Simulate voting strategy
	votingStrategy map[VoteTurn]int

	updateVals bool
}

// NewConsensusState returns a new ConsensusState.
func NewConsensusState(
	logger log.Logger,
	config *cfg.ConsensusConfig,
	state LastestBlockState,
	blockOperations BaseBlockOperations,
	txNotifier txNotifier,
) *ConsensusState {
	cs := &ConsensusState{
		logger: logger,
		config: config,
		//namdoh@ blockExec:        blockExec,
		blockOperations:  blockOperations,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(),
		done:             make(chan struct{}),
		evsw:             NewEventSwitch(),
		txNotifier:       txNotifier,
		RoundState: cstypes.RoundState{
			CommitRound: cmn.NewBigInt32(0),
			Height:      cmn.NewBigInt32(0),
			StartTime:   big.NewInt(0),
			CommitTime:  big.NewInt(0),
		},
		updateVals: false,
	}

	cs.updateToState(state)
	cs.timeoutTicker.SetLogger(cs.logger)

	// Reconstruct LastCommit from db after a crash.
	cs.reconstructLastCommit(state)

	// Don't call scheduleRound0 yet. We do that upon Start().

	return cs
}

// SetPrivValidator sets the private validator account for signing votes.
func (cs *ConsensusState) SetPrivValidator(priv *types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
}

// It loads the latest state via the WAL, and starts the timeout and receive routines.
func (cs *ConsensusState) Start() {
	cs.logger.Info("Consensus state starts!")

	// we need the timeoutRoutine for replay so
	// we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	// firing on the tockChan until the receiveRoutine is started
	// to deal with them (by that point, at most one will be valid)
	if err := cs.timeoutTicker.Start(); err != nil {
		cs.logger.Error("ConsensusState - Start", "err", err)
		return
	}

	// now start the receiveRoutine
	go cs.receiveRoutine(0)

	// schedule the first round!
	// use GetRoundState so we don't race the receiveRoutine for access
	cs.scheduleRound0(cs.GetRoundState())
}

// It stops all routines and waits for the WAL to finish.
func (cs *ConsensusState) Stop() {
	cs.timeoutTicker.Stop()
	cs.logger.Trace("Consensus state stops!")
}

// Updates ConsensusState and increments height to match that of state.
// The round becomes 0 and cs.Step becomes cstypes.RoundStepNewHeight.
func (cs *ConsensusState) updateToState(state LastestBlockState) {
	cs.logger.Trace("ConsensusState - updateToState")
	if cs.CommitRound.IsGreaterThanInt(-1) && cs.Height.IsGreaterThanInt(0) && !cs.Height.Equals(state.LastBlockHeight) {
		cmn.PanicSanity(cmn.Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight))
	}
	if !cs.state.IsEmpty() && !cs.state.LastBlockHeight.Add(1).Equals(cs.Height) {
		// This might happen when someone else is mutating cs.state.
		// Someone forgot to pass in state.Copy() somewhere?!
		cmn.PanicSanity(cmn.Fmt("Inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v",
			cs.state.LastBlockHeight.Int64()+1, cs.Height))
	}

	// If state isn't further out than cs.state, just ignore.
	// This happens when SwitchToConsensus() is called in the manager.
	// We don't want to reset e.g. the Votes, but we still want to
	// signal the new round step, because other services (eg. mempool)
	// depend on having an up-to-date peer state!
	if !cs.state.IsEmpty() && state.LastBlockHeight.IsLessThanOrEquals(cs.state.LastBlockHeight) {
		cs.logger.Info("Ignoring updateToState()", "newHeight", state.LastBlockHeight.Int64()+1, "oldHeight", cs.state.LastBlockHeight.Int64()+1)
		cs.newStep()
		return
	}

	// Reset fields based on state.
	validators := state.Validators
	lastPrecommits := (*types.VoteSet)(nil)
	if cs.CommitRound.IsGreaterThanInt(-1) && cs.Votes != nil {
		if !cs.Votes.Precommits(cs.CommitRound.Int32()).HasTwoThirdsMajority() {
			cmn.PanicSanity("updateToState(state) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = cs.Votes.Precommits(cs.CommitRound.Int32())
	}

	// Next desired block height
	height := state.LastBlockHeight.Add(1)

	// RoundState fields
	cs.updateHeight(height)
	cs.updateRoundStep(cmn.NewBigInt32(0), cstypes.RoundStepNewHeight)
	if cs.CommitTime.Int64() == 0 {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		//  cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		cs.logger.Trace("cs.CommitTime is 0")
		cs.StartTime = big.NewInt(cs.config.Commit(time.Now()).Unix())
	} else {
		commitTime := time.Unix(cs.CommitTime.Int64(), 0)
		cs.StartTime = big.NewInt(cs.config.Commit(commitTime).Unix())
	}
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = cmn.NewBigInt32(-1)
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.ValidRound = cmn.NewBigInt32(-1)
	cs.ValidBlock = nil
	cs.ValidBlockParts = nil
	cs.Votes = cstypes.NewHeightVoteSet(cs.logger, state.ChainID, height, validators)
	cs.CommitRound = cmn.NewBigInt32(-1)
	cs.LastCommit = lastPrecommits
	cs.LastValidators = state.LastValidators
	cs.TriggeredTimeoutPrecommit = false

	cs.state = state

	// Finally, broadcast RoundState
	cs.newStep()
}

//------------------------------------------------------------
// Public interface for passing messages into the consensus state, possibly causing a state transition.
// If peerID == "", the msg is considered internal.
// Messages are added to the appropriate queue (peer or internal).
// If the queue is full, the function may block.
// TODO: should these return anything or let callers just use events?

// AddVote inputs a vote.
func (cs *ConsensusState) AddVote(vote *types.Vote, peerID discover.NodeID) (added bool, err error) {
	if peerID.IsZero() {
		cs.internalMsgQueue <- msgInfo{&VoteMessage{vote}, discover.ZeroNodeID()}
	} else {
		cs.peerMsgQueue <- msgInfo{&VoteMessage{vote}, peerID}
	}

	// TODO: wait for event?!
	return false, nil
}

func (cs *ConsensusState) decideProposal(height *cmn.BigInt, round *cmn.BigInt) {
	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block
	if cs.ValidBlock != nil {
		// If there is valid block, choose that.
		block, blockParts = cs.ValidBlock, cs.ValidBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		// Decide on block
		block, blockParts = cs.createProposalBlock()
		if block == nil { // on error
			cs.logger.Trace("Create proposal block failed")
			return
		}
	}

	// Make proposal
	propBlockID := types.BlockID{Hash: block.Hash(), PartsHeader: blockParts.Header()}
	proposal := types.NewProposal(height, round, cs.ValidRound, propBlockID)
	if err := cs.privValidator.SignProposal(cs.state.ChainID, proposal); err == nil {
		cs.logger.Info("Signed proposal", "height", height, "round", round, "proposal", propBlockID.Hash)
		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, discover.ZeroNodeID()})
		for i := 0; i < blockParts.Total(); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, discover.ZeroNodeID()})
		}
		cs.logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
		cs.logger.Debug(fmt.Sprintf("Signed proposal block: %s", block.Hash()))
	}
}

func (cs *ConsensusState) setProposal(proposal *types.Proposal) error {
	// Already have one
	// TODO: possibly catch double proposals
	if cs.Proposal != nil {
		return nil
	}

	// Does not apply
	if !proposal.Height.Equals(cs.Height) || !proposal.Round.Equals(cs.Round) {
		cs.logger.Trace(fmt.Sprintf("CS[%v/%v] doesn't match Proposal[%v/%v]", cs.Height, cs.Round, proposal.Height, proposal.Round))
		return nil
	}

	// Verify POLRound, which must be -1 or between 0 and proposal.Round exclusive.
	if proposal.POLRound.Int64() < -1 &&
		(proposal.POLRound.Int64() > 0 || proposal.POLRound.IsGreaterThan(proposal.Round)) {
		cs.logger.Trace("Invalid proposal POLRound", "proposal.POLRound", proposal.POLRound, "proposal.Round", proposal.Round)
		return ErrInvalidProposalPOLRound
	}

	// Verify signature
	if !cs.Validators.GetProposer().VerifyProposalSignature(cs.state.ChainID, proposal) {
		cs.logger.Trace("Verify proposal signature failed.")
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	// We don't update cs.ProposalBlockParts if it is already set.
	// This happens if we're already in cstypes.RoundStepCommit or if there is a valid block in the current round.
	// TODO: We can check if Proposal is for a different block as this is a sign of misbehavior!
	if cs.ProposalBlockParts == nil {
		cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.POLBlockID.PartsHeader)
	}
	cs.logger.Info("Received proposal", "proposal", proposal)
	return nil
}

// ------- HELPER METHODS -------- //

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(rs *cstypes.RoundState) {
	cs.logger.Info("scheduleRound0", "now", time.Now(), "startTime", time.Unix(cs.StartTime.Int64(), 0))
	sleepDuration := time.Duration(rs.StartTime.Int64() - time.Now().Unix()) // nolint: gotype, gosimple
	cs.scheduleTimeout(sleepDuration, rs.Height, cmn.NewBigInt32(0), cstypes.RoundStepNewHeight)
}

// Attempt to schedule a timeout (by sending timeoutInfo on the tickChan)
func (cs *ConsensusState) scheduleTimeout(duration time.Duration, height *cmn.BigInt, round *cmn.BigInt, step cstypes.RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height.Copy(), round.Copy(), step})
}

// Send a msg into the receiveRoutine regarding our own proposal, or vote
func (cs *ConsensusState) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		cs.logger.Info("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastCommit(state LastestBlockState) {
	if state.LastBlockHeight.EqualsInt64(0) {
		return
	}
	seenCommit := cs.blockOperations.LoadSeenCommit(state.LastBlockHeight.Uint64())
	lastPrecommits := types.CommitToVoteSet(state.ChainID, seenCommit, state.LastValidators)
	if !lastPrecommits.HasTwoThirdsMajority() {
		cmn.PanicSanity("Failed to reconstruct LastCommit: Does not have +2/3 maj")
	}
	cs.LastCommit = lastPrecommits
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *ConsensusState) tryAddVote(vote *types.Vote, peerID discover.NodeID) (bool, error) {
	added, err := cs.addVote(vote, peerID)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, add it to the cs.evpool.
		// If it's otherwise invalid, punish peer.
		if err == ErrVoteHeightMismatch {
			return added, err
		} else if _, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if vote.ValidatorAddress.Equal(cs.privValidator.GetAddress()) {
				cs.logger.Error("Found conflicting vote from ourselves. Did you unsafe_reset a validator?", "height", vote.Height, "round", vote.Round, "type", vote.Type)
				return added, err
			}
			// TODO(namdoh): Re-enable this later.
			cs.logger.Warn("Add vote error to evidence pool later")
			return added, err
		} else {
			// Probably an invalid signature / Bad peer.
			// Seems this can also err sometimes with "Unexpected step" - perhaps not from a bad peer ?
			cs.logger.Error("Error attempting to add vote", "err", err)
			return added, ErrAddingVote
		}
	}
	return added, nil
}

func (cs *ConsensusState) addVote(vote *types.Vote, peerID discover.NodeID) (added bool, err error) {
	cs.logger.Debug(
		"addVote",
		"voteHeight",
		vote.Height,
		"voteType",
		vote.Type,
		"valIndex",
		vote.ValidatorIndex,
		"csHeight",
		cs.Height)

	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height.Add(1).Equals(cs.Height) {
		if !(cs.Step == cstypes.RoundStepNewHeight && vote.Type == types.PrevoteType) {
			return added, ErrVoteHeightMismatch
		}
		added, err = cs.LastCommit.AddVote(vote)
		if !added {
			return
		}

		cs.logger.Info(cmn.Fmt("Added to lastPrecommits: %v", cs.LastCommit.StringShort()))
		cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote})
		cs.evsw.FireEvent(types.EventVote, vote)

		// if we can skip timeoutCommit and have all the votes now,
		if cs.config.SkipTimeoutCommit && cs.LastCommit.HasAll() {
			// go straight to new round (skip timeout commit)
			// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
			cs.enterNewRound(cs.Height, cmn.NewBigInt32(0))
		}

		return
	}

	// Height mismatch is ignored.
	// Not necessarily a bad peer, but not favourable behaviour.
	if !vote.Height.Equals(cs.Height) {
		err = ErrVoteHeightMismatch
		cs.logger.Info("Vote ignored and not added", "voteHeight", vote.Height, "csHeight", cs.Height, "err", err)
		return
	}

	height := cs.Height
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	}

	cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote})
	cs.evsw.FireEvent(types.EventVote, vote)

	switch vote.Type {
	case types.PrevoteType:
		prevotes := cs.Votes.Prevotes(vote.Round.Int32())

		cs.logger.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())

		// If +2/3 prevotes for a block or nil for *any* round:
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {
			// There was a polka!
			// If we're locked but this is a recent polka, unlock.
			// If it matches our ProposalBlock, update the ValidBlock

			// Unlock if `cs.LockedRound < vote.Round <= cs.Round`
			// NOTE: If vote.Round > cs.Round, we'll deal with it when we get to vote.Round
			if (cs.LockedBlock != nil) &&
				cs.LockedRound.IsLessThan(vote.Round) &&
				vote.Round.IsLessThanOrEquals(cs.Round) &&
				!cs.LockedBlock.HashesTo(blockID.Hash) {

				cs.logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
				cs.LockedRound = cmn.NewBigInt32(-1)
				cs.LockedBlock = nil
				cs.LockedBlockParts = nil
				cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
			}

			// Update Valid* if we can.
			// NOTE: our proposal block may be nil or not what received a polka..
			if !blockID.IsZero() &&
				cs.ValidRound.IsLessThan(vote.Round) &&
				vote.Round.Equals(cs.Round) {

				if cs.ProposalBlock.HashesTo(blockID.Hash) {
					cs.logger.Info("Updating ValidBlock because of POL.", "validRound", cs.ValidRound, "POLRound", vote.Round)
					cs.ValidRound = vote.Round
					cs.ValidBlock = cs.ProposalBlock
					cs.ValidBlockParts = cs.ProposalBlockParts
				} else {
					cs.logger.Info(
						"Valid block we don't know about. Set ProposalBlock=nil",
						"proposal", cs.ProposalBlock.Hash(), "blockId", blockID.Hash)
					// We're getting the wrong block.
					cs.ProposalBlock = nil
				}

				if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
					cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
				}

				cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
				cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent())
			}
		}

		// If +2/3 prevotes for *anything* for future round:
		switch {
		case cs.Round.IsLessThan(vote.Round) && prevotes.HasTwoThirdsAny():
			cs.enterNewRound(height, vote.Round)
		case cs.Round.Equals(vote.Round) && cstypes.RoundStepPrevote <= cs.Step:
			blockID, ok := prevotes.TwoThirdsMajority()
			if ok && (cs.isProposalComplete() || blockID.Hash.IsZero()) {
				cs.enterPrecommit(height, vote.Round)
			} else if prevotes.HasTwoThirdsAny() {
				cs.enterPrevoteWait(height, vote.Round)
			}
		case cs.Proposal != nil && cs.Proposal.POLRound.IsGreaterThanOrEqualToInt(0) && cs.Proposal.POLRound.Equals(vote.Round):
			// If the proposal is now complete, enter prevote of cs.Round.
			if cs.isProposalComplete() {
				cs.enterPrevote(height, cs.Round)
			}
		}

	case types.PrecommitType:
		precommits := cs.Votes.Precommits(vote.Round.Int32())

		cs.logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			// Executed as TwoThirdsMajority could be from a higher round
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommit(height, vote.Round)

			if !blockID.Hash.IsZero() {
				cs.enterCommit(height, vote.Round)
				if cs.config.SkipTimeoutCommit && precommits.HasAll() {
					cs.enterNewRound(cs.Height, cmn.NewBigInt32(0))
				}
			} else {
				// if we have all the votes now,
				// go straight to new round (skip timeout commit)
				// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
				cs.enterPrecommitWait(height, vote.Round)
			}
		} else if cs.Round.IsLessThanOrEquals(vote.Round) && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}
	default:
		panic(cmn.Fmt("Unexpected vote type %X", vote.Type)) // go-wire should prevent this.
	}

	return
}

// Get script vote
func (cs *ConsensusState) scriptedVote(height int, round int, voteType int) (int, bool) {
	if val, ok := cs.votingStrategy[VoteTurn{Height: height, Round: round, VoteType: voteType}]; ok {
		return val, ok
	}
	return 0, false
}

// Signs vote.
func (cs *ConsensusState) signVote(type_ types.SignedMsgType, hash cmn.Hash, header types.PartSetHeader) (*types.Vote, error) {
	addr := cs.privValidator.GetAddress()
	valIndex, _ := cs.Validators.GetByAddress(addr)

	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   cmn.NewBigInt32(valIndex),
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        cs.voteTime(),
		Type:             type_,
		BlockID:          types.BlockID{Hash: hash, PartsHeader: header},
	}

	err := cs.privValidator.SignVote(cs.state.ChainID, vote)
	return vote, err
}

func (cs *ConsensusState) voteTime() *big.Int {
	now := time.Now()
	minVoteTime := now.Unix()
	// TODO: We should remove next line in case we don't vote for v in case cs.ProposalBlock == nil,
	// even if cs.LockedBlock != nil. See https://github.com/tendermint/spec.
	timeIotaMs := int64(1000)
	if cs.LockedBlock != nil {
		// See the BFT time spec https://tendermint.com/docs/spec/consensus/bft-time.html
		minVoteTime = cs.LockedBlock.Time().Int64()
	} else if cs.ProposalBlock != nil {
		minVoteTime = cs.ProposalBlock.Time().Int64()
	}

	return big.NewInt(minVoteTime + timeIotaMs)
}

// Signs the vote and publish on internalMsgQueue
func (cs *ConsensusState) signAddVote(type_ types.SignedMsgType, hash cmn.Hash, header types.PartSetHeader) *types.Vote {
	// if we don't have a key or we're not in the validator set, do nothing
	if cs.privValidator == nil || !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
		return nil
	}
	vote, err := cs.signVote(type_, hash, header)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, discover.ZeroNodeID()})
		cs.logger.Info("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
		return vote
	}
	//if !cs.replayMode {
	cs.logger.Error("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
	//}
	return nil
}

// Updates ConsensusState to the current round and round step.
func (cs *ConsensusState) updateRoundStep(round *cmn.BigInt, step cstypes.RoundStepType) {
	cs.Round = round
	cs.Step = step
}

// Advances to a new step.
func (cs *ConsensusState) newStep() {
	cs.logger.Info("enter newStep()")
	//rs := cs.RoundStateEvent()
	cs.nSteps++

	cs.evsw.FireEvent(types.EventNewRoundStep, &cs.RoundState)
}

func (cs *ConsensusState) updateHeight(height *cmn.BigInt) {
	//namdoh@ cs.metrics.Height.Set(float64(height))
	cs.Height = height
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit,
// once we have the full block.
func (cs *ConsensusState) addProposalBlockPart(msg *BlockPartMessage, peerID discover.NodeID) (added bool, err error) {
	height, round, part := msg.Height, msg.Round, msg.Part

	// Blocks might be reused, so round mismatch is OK
	if !cs.Height.Equals(height) {
		cs.logger.Debug("Received block part from wrong height", "height", height, "round", round)
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.logger.Info("Received a block part when we're not expecting any",
			"height", height, "round", round, "index", part.Index, "peer", peerID)
		return false, nil
	}

	added, err = cs.ProposalBlockParts.AddPart(part)
	if err != nil {
		return added, err
	}

	if added && cs.ProposalBlockParts.IsComplete() {
		// Added and completed!
		if err := rlp.Decode(cs.ProposalBlockParts.GetReader(), &cs.ProposalBlock); err != nil {
			return added, err
		}
		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		cs.logger.Info("Received complete proposal block", "height", cs.ProposalBlock.Height(), "hash", cs.ProposalBlock.Hash())
		cs.eventBus.PublishEventCompleteProposal(cs.CompleteProposalEvent())

		// Update Valid* if we can.
		prevotes := cs.Votes.Prevotes(cs.Round.Int32())
		blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
		if hasTwoThirds && !blockID.IsZero() && (cs.ValidRound.IsLessThan(cs.Round)) {
			if cs.ProposalBlock.HashesTo(blockID.Hash) {
				cs.logger.Info("Updating valid block to new proposal block",
					"valid-round", cs.Round, "valid-block-hash", cs.ProposalBlock.Hash())
				cs.ValidRound = cs.Round
				cs.ValidBlock = cs.ProposalBlock
				cs.ValidBlockParts = cs.ProposalBlockParts
			}
			// TODO: In case there is +2/3 majority in Prevotes set for some
			// block and cs.ProposalBlock contains different block, either
			// proposer is faulty or voting power of faulty processes is more
			// than 1/3. We should trigger in the future accountability
			// procedure at this point.
		}
		if cs.Step <= cstypes.RoundStepPropose && cs.isProposalComplete() {
			// Move onto the next step
			cs.enterPrevote(height, cs.Round)
			if hasTwoThirds { // this is optimisation as this will be triggered when prevote is added
				cs.enterPrecommit(height, cs.Round)
			}
		} else if cs.Step == cstypes.RoundStepCommit {
			// If we're waiting on the proposal block...
			cs.tryFinalizeCommit(height)
		}
		return added, nil
	}
	return added, nil
}

// GetRoundState returns a shallow copy of the internal consensus state.
func (cs *ConsensusState) GetRoundState() *cstypes.RoundState {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	rs := cs.RoundState // copy
	return &rs
}

// LoadCommit loads the commit for a given height.
func (cs *ConsensusState) LoadCommit(height *cmn.BigInt) *types.Commit {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	if height.EqualsUint64(cs.blockOperations.Height()) {
		return cs.blockOperations.LoadSeenCommit(height.Uint64())
	}
	return cs.blockOperations.LoadBlockCommit(height.Uint64())
}

func (cs *ConsensusState) handleTxsAvailable() {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// We only need to do this for round 0.
	if !cs.Round.EqualsInt(0) {
		return
	}

	switch cs.Step {
	case cstypes.RoundStepNewHeight: // timeoutCommit phase
		if cs.needProofBlock(cs.Height) {
			// enterPropose will be called by enterNewRound
			return
		}

		// +1ms to ensure RoundStepNewRound timeout always happens after RoundStepNewHeight
		timeoutCommit := time.Duration(cs.StartTime.Int64()-time.Now().Unix()) + 1*time.Millisecond
		cs.scheduleTimeout(timeoutCommit, cs.Height, cmn.NewBigInt32(0), cstypes.RoundStepNewRound)
	case cstypes.RoundStepNewRound: // after timeoutCommit
		cs.enterPropose(cs.Height, cmn.NewBigInt32(0))
	}
}

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
// 	or, if SkipTimeout==true, after receiving all precommits from (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) enterNewRound(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.logger.New("height", height, "round", round)

	if !cs.Height.Equals(height) || round.IsLessThan(cs.Round) || (cs.Round.Equals(round) && cs.Step != cstypes.RoundStepNewHeight) {
		logger.Debug(cmn.Fmt("enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	if now := time.Now().Unix(); cs.StartTime.Int64() > now {
		logger.Info("Need to set a buffer and log message here for sanity.", "startTime", time.Unix(cs.StartTime.Int64(), 0), "now", time.Unix(now, 0))
	}

	logger.Info(cmn.Fmt("enterNewRound(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Increment validators if necessary
	validators := cs.Validators
	if cs.Round.IsLessThan(round) {
		// TODO(namdoh): Revisit to see if we need to copy validators here.
		validators = validators.Copy()
		validators.AdvanceProposer(round.Int64() - cs.Round.Int64())
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, cstypes.RoundStepNewRound)
	cs.Validators = validators
	if round.EqualsInt(0) {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		logger.Info("Resetting Proposal info")
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
	}
	cs.Votes.SetRound(round.Int32() + 1) // also track next round (round+1) to allow round-skipping
	cs.TriggeredTimeoutPrecommit = false
	cs.eventBus.PublishEventNewRound(cs.RoundStateEvent())

	// Wait for txs to be available in the mempool
	// before we enterPropose in round 0. If the last block changed the app hash,
	// we may need an empty "proof" block, and enterPropose immediately.
	waitForTxs := cs.config.WaitForTxs() && round.EqualsInt(0) && !cs.needProofBlock(height)
	if waitForTxs {
		if cs.config.CreateEmptyBlocksInterval > 0 {
			cs.scheduleTimeout(cs.config.CreateEmptyBlocksInterval, height, round,
				cstypes.RoundStepNewRound)
		}
	} else {
		cs.enterPropose(height, round)
	}
}

// needProofBlock returns true on the first height (so the genesis app hash is signed right away)
// and where the last block (height-1) caused the app hash to change
// addition: last proposer always sends a tx to claim block reward, therefore pendingSize should greater than 1
func (cs *ConsensusState) needProofBlock(height *cmn.BigInt) bool {
	if height.EqualsInt(1) {
		return true
	}

	lastBlockMeta := cs.blockOperations.LoadBlockMeta(height.Uint64() - 1)
	return !cs.state.AppHash.Equal(lastBlockMeta.Header.AppHash) && cs.blockOperations.TxPool().PendingSize() > 1
}

// Enter (CreateEmptyBlocks): from enterNewRound(height,round)
// Enter (CreateEmptyBlocks, CreateEmptyBlocksInterval > 0 ): after enterNewRound(height,round), after timeout of CreateEmptyBlocksInterval
// Enter (!CreateEmptyBlocks) : after enterNewRound(height,round), once txs are in the mempool
func (cs *ConsensusState) enterPropose(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.logger.New("height", height, "round", round)

	if !cs.Height.Equals(height) || round.IsLessThan(cs.Round) || (cs.Round.Equals(round) && cstypes.RoundStepPropose <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPropose(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(cmn.Fmt("enterPropose(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPropose:
		cs.updateRoundStep(round, cstypes.RoundStepPropose)
		cs.newStep()

		// If we have the proposal + POL, then goto Prevote now.
		// Else after timeoutPropose
		if cs.isProposalComplete() {
			cs.enterPrevote(height, cs.Round)
		}
	}()

	// If we don't get the proposal quick enough, enterPrevote
	cs.scheduleTimeout(cs.config.Propose(round.Int32()), height, round, cstypes.RoundStepPropose)

	// TODO(namdoh): For now this any node is a validator. Remove it once we
	// restrict who can be validator.
	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("This node is not a validator")
		return
	}
	// if not a validator, we're done
	if !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
		logger.Debug("This node is not a validator", "addr", cs.privValidator.GetAddress(), "vals", cs.Validators)
		return
	}

	logger.Debug("This node is a validator")
	if cs.isProposer() {
		logger.Trace("Our turn to propose")
		//namdoh@ logger.Info("enterPropose: Our turn to propose", "proposer", cs.Validators.GetProposer().Address, "privValidator", cs.privValidator)
		cs.decideProposal(height, round)
	} else {
		logger.Trace("Not our turn to propose")
		//namdoh@ logger.Info("enterPropose: Not our turn to propose", "proposer", cs.Validators.GetProposer().Address, "privValidator", cs.privValidator)
	}
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for future round.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) enterPrevote(height *cmn.BigInt, round *cmn.BigInt) {
	if !cs.Height.Equals(height) || round.IsLessThan(cs.Round) || (cs.Round.Equals(round) && cstypes.RoundStepPrevote <= cs.Step) {
		cs.logger.Debug(cmn.Fmt("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, cstypes.RoundStepPrevote)
		cs.newStep()
	}()

	cs.logger.Info(cmn.Fmt("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *ConsensusState) doPrevote(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.logger.New("height", height, "round", round)
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Block was locked")
		cs.signAddVote(types.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Info("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.PrevoteType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// Validate proposal block
	// This checks the block contents without executing txs.
	if err := ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: ProposalBlock is invalid", "err", err)
		cs.signAddVote(types.PrevoteType, cmn.Hash{}, types.PartSetHeader{})
		return
	}
	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block is validated as it is received (against the merkle hash in the proposal)
	logger.Info("enterPrevote: ProposalBlock is valid")
	cs.signAddVote(types.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
}

// Enter: any +2/3 prevotes at next round.
func (cs *ConsensusState) enterPrevoteWait(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.logger.New("height", height, "round", round)

	if !cs.Height.Equals(height) || round.IsLessThan(cs.Round) || (cs.Round.Equals(round) && cstypes.RoundStepPrevoteWait <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Prevotes(round.Int32()).HasTwoThirdsAny() {
		cmn.PanicSanity(cmn.Fmt("enterPrevoteWait(%v/%v), but Prevotes does not have any +2/3 votes", height, round))
	}
	logger.Info(cmn.Fmt("enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.config.Prevote(round.Int32()), height, round, cstypes.RoundStepPrevoteWait)
}

// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: +2/3 precomits for block or nil.
// Enter: any +2/3 precommits for next round.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (cs *ConsensusState) enterPrecommit(height *cmn.BigInt, round *cmn.BigInt) {
	cs.logger.Trace("enterPrecommit", "height", height, "round", round)
	logger := cs.logger.New("height", height, "round", round)

	if !cs.Height.Equals(height) || round.IsLessThan(cs.Round) || (cs.Round.Equals(round) && cstypes.RoundStepPrecommit <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	logger.Info(cmn.Fmt("enterPrecommit(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, cstypes.RoundStepPrecommit)
		cs.newStep()
	}()

	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round.Int32()).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(types.PrecommitType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	cs.eventBus.PublishEventPolka(cs.RoundStateEvent())

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round.Int32() {
		cmn.PanicSanity(cmn.Fmt("This POLRound should be %v but got %", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if blockID.IsZero() {
		if cs.LockedBlock == nil {
			logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = cmn.NewBigInt32(-1)
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}
		cs.signAddVote(types.PrecommitType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		cs.eventBus.PublishEventRelock(cs.RoundStateEvent())
		cs.signAddVote(types.PrecommitType, blockID.Hash, blockID.PartsHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID)
		// Validate the block.
		if err := ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
			cmn.PanicConsensus(cmn.Fmt("enterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.eventBus.PublishEventLock(cs.RoundStateEvent())
		cs.signAddVote(types.PrecommitType, blockID.Hash, blockID.PartsHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	// TODO: In the future save the POL prevotes for justification.
	cs.LockedRound = cmn.NewBigInt32(-1)
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
	}
	cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
	cs.signAddVote(types.PrecommitType, cmn.Hash{}, types.PartSetHeader{})
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) enterPrecommitWait(height *cmn.BigInt, round *cmn.BigInt) {
	logger := log.New("height", height.Int64(), "round", round.Int64())

	if !cs.Height.Equals(height) || !round.Equals(cs.Round) || (cs.Round.Equals(round) && cs.TriggeredTimeoutPrecommit) {
		logger.Debug(
			fmt.Sprintf(
				"enterPrecommitWait(%v/%v): Invalid args. "+
					"Current state is Height/Round: %v/%v/, TriggeredTimeoutPrecommit:%v",
				height, round, cs.Height, cs.Round, cs.TriggeredTimeoutPrecommit))
		return
	}
	if !cs.Votes.Precommits(round.Int32()).HasTwoThirdsAny() {
		panic(fmt.Sprintf("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}
	logger.Info(fmt.Sprintf("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.TriggeredTimeoutPrecommit = true
		cs.newStep()
	}()

	// Wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.config.Precommit(round.Int32()), height, round, cstypes.RoundStepPrecommitWait)
}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height *cmn.BigInt, commitRound *cmn.BigInt) {
	logger := cs.logger.New("height", height, "commitRound", commitRound)

	if !cs.Height.Equals(height) || cstypes.RoundStepCommit <= cs.Step {
		logger.Debug(cmn.Fmt("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(cmn.Fmt("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		cs.updateRoundStep(cs.Round, cstypes.RoundStepCommit)
		cs.CommitRound = commitRound
		cs.CommitTime = big.NewInt(time.Now().Unix())
		cs.newStep()

		// Maybe finalize immediately.
		cs.tryFinalizeCommit(height)
	}()

	blockID, ok := cs.Votes.Precommits(commitRound.Int32()).TwoThirdsMajority()
	if !ok {
		cmn.PanicSanity("RunActionCommit() expects +2/3 precommits")
	}

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they'll be cleared in updateToState.
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Info("Commit is for locked block. Set ProposalBlock=LockedBlock", "blockHash", blockID)
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
	}

	// If we don't have the block being committed, set up to get it.
	// cs.ProposalBlock is confirmed not nil from caller.
	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
			logger.Info("Commit is for a block we don't know about. Set ProposalBlock=nil", "commit", blockID)
			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
			cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent())
			cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) tryFinalizeCommit(height *cmn.BigInt) {
	logger := cs.logger.New("height", height)

	if !cs.Height.Equals(height) {
		cmn.PanicSanity(cmn.Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound.Int32()).TwoThirdsMajority()
	if !ok || blockID.IsZero() {
		logger.Error("Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.")
		return
	}

	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Info("Attempt to finalize failed. We don't have the commit block.", "proposal-block", cs.ProposalBlock.Hash(), "commit-block", blockID)
		return
	}

	cs.finalizeCommit(height)
}

// Increment height and goto cstypes.RoundStepNewHeight
func (cs *ConsensusState) finalizeCommit(height *cmn.BigInt) {
	if !cs.Height.Equals(height) || cs.Step != cstypes.RoundStepCommit {
		cs.logger.Debug(cmn.Fmt("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound.Int32()).TwoThirdsMajority()
	block, blockParts := cs.ProposalBlock, cs.ProposalBlockParts

	if !ok {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, commit does not have two thirds majority"))
	}
	if !blockParts.HasHeader(blockID.PartsHeader) {
		cmn.PanicSanity(cmn.Fmt("Expected ProposalBlockParts header to be commit header"))
	}
	if !block.HashesTo(blockID.Hash) {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	if err := ValidateBlock(cs.state, block); err != nil {
		cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block: %v", err))
		panic("Block validation failed")
	}

	cs.logger.Info("Finalizing commit of block", "tx number", block.NumTxs(),
		"height", block.Height(), "hash", block.Hash().String())

	fail.Fail() // XXX

	// Create a copy of the state for staging and an event cache for txs.
	stateCopy := cs.state.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	var err error
	stateCopy, err = ApplyBlock(
		cs.logger,
		stateCopy,
		cs.blockOperations,
		blockID,
		block,
		cs.blockOperations.Blockchain(),
	)
	if err != nil {
		cs.logger.Error("Error on ApplyBlock. Did the application crash? Please restart node", "err", err)
		err := cmn.Kill()
		if err != nil {
			cs.logger.Error("Failed to kill this process - please do so manually", "err", err)
		}
		return
	}

	// Save block.
	if cs.blockOperations.Height() < block.Height() {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.Votes.Precommits(cs.CommitRound.Int32())
		seenCommit := precommits.MakeCommit()
		cs.blockOperations.SaveBlock(block, blockParts, seenCommit)
	} else {
		// Happens during replay if we already saved the block but didn't commit
		cs.logger.Info("Calling finalizeCommit on already stored block", "height", block.Height())
	}

	fail.Fail() // XXX

	// NewHeightStep!
	cs.updateToState(stateCopy)

	fail.Fail() // XXX

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0(&cs.RoundState)

	// By now,
	// - cs.Height has been increment to height+1
	// - cs.Step is now cstypes.RoundStepNewHeight
	// - cs.StartTime is set to when we will start round0.
}

// Creates the next block to propose and returns it. Returns nil block upon
// error.
func (cs *ConsensusState) createProposalBlock() (block *types.Block, blockParts *types.PartSet) {
	cs.logger.Trace("createProposalBlock")
	var commit *types.Commit
	if cs.Height.EqualsInt(1) {
		// We're creating a proposal for the first block.
		commit = types.NewCommit(types.BlockID{}, nil)
		cs.logger.Trace("enterPropose: First height, use empty Commit.")
	} else if cs.LastCommit.HasTwoThirdsMajority() {
		// Make the commit from LastCommit
		commit = cs.LastCommit.MakeCommit()
		cs.logger.Trace("enterPropose: Subsequent height, use last commit.", "commit", commit)
	} else {
		// This shouldn't happen.
		cs.logger.Error("enterPropose: Cannot propose anything: No commit for the previous block.")
		return nil, nil
	}
	proposerAddr := cs.privValidator.GetAddress()
	return cs.blockOperations.CreateProposalBlock(
		cs.Height.Int64(),
		cs.state,
		proposerAddr,
		commit,
	)
}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there).
func (cs *ConsensusState) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if cs.Proposal.POLRound.IsLessThanInt(0) {
		return true
	}
	// if this is false the proposer is lying or we haven't received the POL yet
	return cs.Votes.Prevotes(cs.Proposal.POLRound.Int32()).HasTwoThirdsMajority()
}

func (cs *ConsensusState) isProposer() bool {
	privValidatorAddress := cs.privValidator.GetAddress()
	return bytes.Equal(cs.Validators.GetProposer().Address[:], privValidatorAddress[:])
}

// ----------- Other helpers -----------

func CompareHRS(h1 *cmn.BigInt, r1 *cmn.BigInt, s1 cstypes.RoundStepType, h2 *cmn.BigInt, r2 *cmn.BigInt, s2 cstypes.RoundStepType) int {
	if h1.IsLessThan(h2) {
		return -1
	} else if h1.IsGreaterThan(h2) {
		return 1
	}
	if r1.IsLessThan(r2) {
		return -1
	} else if r1.IsGreaterThan(r2) {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}

// receiveRoutine handles messages which may cause state transitions.
// it's argument (n) is the number of messages to process before exiting - use 0 to run forever
// It keeps the RoundState and is the only thing that updates it.
// Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities.
// ConsensusState must be locked before any internal state is updated.
func (cs *ConsensusState) receiveRoutine(maxSteps int) {
	defer func() {
		if r := recover(); r != nil {
			cs.logger.Error("CONSENSUS FAILURE!!!", "err", r, "stack", string(debug.Stack()))
		}
	}()

	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				cs.logger.Info("reached max steps. exiting receive routine")
				cs.nSteps = 0
				return
			}
		}
		rs := cs.RoundState
		var mi msgInfo

		select {
		case <-cs.txNotifier.TxsAvailable():
			cs.handleTxsAvailable()
		case mi = <-cs.peerMsgQueue:
			// handles proposals, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)

			if _, ok := mi.Msg.(*VoteMessage); ok {
				// we actually want to simulate failing during
				// the previous WriteSync, but this isn't easy to do.
				// Equivalent would be to fail here and manually remove
				// some bytes from the end of the wal.
				fail.Fail() // XXX
			}

			cs.handleMsg(mi)
		case mi = <-cs.internalMsgQueue:
			// handles proposals, votes
			cs.handleMsg(mi)
		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			// if the timeout is relevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)
		}
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *ConsensusState) handleMsg(mi msgInfo) {
	cs.logger.Trace("handleMsg", "msgInfo", mi)
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	var err error
	msg, peerID := mi.Msg, mi.PeerID
	switch msg := msg.(type) {
	case *ProposalMessage:
		err = cs.setProposal(msg.Proposal)
	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		_, err := cs.addProposalBlockPart(msg, peerID)
		if err != nil && msg.Round != cs.Round {
			cs.logger.Debug(
				"Received block part from wrong round",
				"height",
				cs.Height,
				"csRound",
				cs.Round,
				"blockRound",
				msg.Round)
			err = nil
		}
	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		cs.logger.Trace("handling AddVote", "VoteMessage", msg.Vote)
		_, err := cs.tryAddVote(msg.Vote, peerID)
		if err == ErrAddingVote {
			cs.logger.Trace("trying to add vote failed", "err", err)
			cs.logger.Warn("TODO - punish peer.")
		}

	default:
		cs.logger.Error("Unknown msg type", "msg_type", reflect.TypeOf(msg))
	}
	if err != nil {
		cs.logger.Error("Error with msg", "height", cs.Height, "round", cs.Round, "type", reflect.TypeOf(msg), "peer", peerID, "err", err)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs cstypes.RoundState) {
	cs.logger.Debug("Received tock", "timeout", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)

	//// timeouts must be for current height, round, step
	if !ti.Height.Equals(rs.Height) || ti.Round.IsLessThan(rs.Round) || (ti.Round.Equals(rs.Round) && ti.Step < rs.Step) {
		cs.logger.Debug("Ignoring tick because we're ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case cstypes.RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		cs.enterNewRound(ti.Height, cmn.NewBigInt32(0))
	case cstypes.RoundStepNewRound:
		cs.enterPropose(ti.Height, cmn.NewBigInt32(0))
	case cstypes.RoundStepPropose:
		cs.enterPrevote(ti.Height, ti.Round)
	case cstypes.RoundStepPrevoteWait:
		cs.enterPrecommit(ti.Height, ti.Round)
	case cstypes.RoundStepPrecommitWait:
		cs.enterPrecommit(ti.Height, ti.Round)
		cs.enterNewRound(ti.Height, ti.Round.Add(1))
	default:
		panic(cmn.Fmt("Invalid timeout step: %v", ti.Step))
	}
}

// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type LastestBlockState struct {
	// Immutable
	ChainID string

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight  *cmn.BigInt
	LastBlockTotalTx *cmn.BigInt
	LastBlockID      types.BlockID
	LastBlockTime    *big.Int

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	PrefetchedFutureValidators  *types.ValidatorSet
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged *cmn.BigInt

	// TODO(namdoh): Add consensus parameters used for validating blocks.

	// Merkle root of the results from executing prev block
	//namdoh@ LastResultsHash []byte

	// The latest AppHash we've received from calling abci.Commit()
	AppHash cmn.Hash
}

// Copy makes a copy of the State for mutating.
func (state LastestBlockState) Copy() LastestBlockState {
	var futureVals *types.ValidatorSet
	if state.PrefetchedFutureValidators != nil {
		futureVals = state.PrefetchedFutureValidators.Copy()
	}
	return LastestBlockState{
		ChainID: state.ChainID,

		LastBlockHeight:  state.LastBlockHeight,
		LastBlockTotalTx: state.LastBlockTotalTx,
		LastBlockID:      state.LastBlockID,
		LastBlockTime:    state.LastBlockTime,

		Validators:                  state.Validators.Copy(),
		LastValidators:              state.LastValidators.Copy(),
		PrefetchedFutureValidators:  futureVals,
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,

		AppHash: state.AppHash,

		//namdoh@ LastResultsHash: state.LastResultsHash,
	}
}

// IsEmpty returns true if the State is equal to the empty State.
func (state LastestBlockState) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}

// Stringshort returns a short string representing State
func (state LastestBlockState) String() string {
	return fmt.Sprintf("{ChainID:%v LastBlockHeight:%v LastBlockTotalTx:%v LastBlockID:%v LastBlockTime:%v Validators:%v LastValidators:%v LastHeightValidatorsChanged:%v",
		state.ChainID, state.LastBlockHeight, state.LastBlockTotalTx, state.LastBlockID, time.Unix(state.LastBlockTime.Int64(), 0),
		state.Validators, state.LastValidators, state.LastHeightValidatorsChanged)
}

// May fresh current or future validator sets, or both.
// Note: This must be called before commiting a block.
func (state *LastestBlockState) mayRefreshValidatorSet(bc base.BaseBlockChain) {
	height := state.LastBlockHeight.Int64()

	currentVals := state.Validators
	nextVals := state.PrefetchedFutureValidators


	// if height is between currentVals's endHeight and EndHeight-bc.GetFetchNewValidatorsTime(), fetch new validators.
	if nextVals == nil && height < currentVals.EndHeight && height >= currentVals.EndHeight-int64(bc.GetFetchNewValidatorsTime()) {
		state.PrefetchedFutureValidators = state.fetchValidatorSet(bc)
	}

	// if height is greater or equal current validators set end height, update current validators set, clear PrefetchedFutureValidators
	currentVals = state.Validators
	nextVals = state.PrefetchedFutureValidators
	if height >= currentVals.EndHeight {
		state.Validators = nextVals.Copy()
		state.PrefetchedFutureValidators = nil
	}
}

// Fetches the validator set at a given height.
func (state *LastestBlockState) fetchValidatorSet(bc base.BaseBlockChain) *types.ValidatorSet {
	var (
		valSet *types.ValidatorSet
		err error
	)
	if valSet, err = kvm.CollectValidatorSet(bc); err != nil || valSet == nil {
		log.Error("error while fetching validator set", "err", err)
		return nil
	}
	if valSet.StartHeight == state.Validators.StartHeight && valSet.EndHeight == state.Validators.EndHeight {
		return nil
	}
	return valSet
}
