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
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ebuchman/fail-test"
	"github.com/gogo/protobuf/proto"

	cfg "github.com/kardiachain/go-kardia/configs"
	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/lib/common"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	kevents "github.com/kardiachain/go-kardia/lib/events"
	"github.com/kardiachain/go-kardia/lib/log"
	kos "github.com/kardiachain/go-kardia/lib/os"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/lib/service"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
	ktime "github.com/kardiachain/go-kardia/types/time"
)

var (
	msgQueueSize = 1000
)

// msgs from the manager which may update the state
type msgInfo struct {
	Msg    Message `json:"msg"`
	PeerID p2p.ID  `json:"peer_key"`
}

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration         `json:"duration"`
	Height   uint64                `json:"height"`
	Round    uint32                `json:"round"`
	Step     cstypes.RoundStepType `json:"step"`
}

// VoteTurn is used for simulate vote strategy
type VoteTurn struct {
	Height   int
	Round    int
	VoteType int
}

// interface to the evidence pool
type evidencePool interface {
	AddEvidenceFromConsensus(ev types.Evidence) error
}

func EmptyTimeoutInfo() *timeoutInfo {
	return &timeoutInfo{
		Duration: 0,
		Height:   0,
		Round:    1,
		Step:     1,
	}
}

// ConsensusState handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator,
// and from a timer.
type ConsensusState struct {
	service.BaseService

	config          *cfg.ConsensusConfig
	privValidator   types.PrivValidator // for signing votes
	blockOperations BaseBlockOperations
	blockExec       *cstate.BlockExecutor
	evpool          evidencePool // TODO(namdoh): Add mem pool.

	// internal state
	mtx sync.RWMutex
	cstypes.RoundState
	state cstate.LastestBlockState // State until height-1.
	// privValidator address, memoized for the duration of one block
	// to avoid extra requests to HSM
	privValidatorAddress *common.Address
	timeoutTicker        TimeoutTicker

	// State changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo

	// we use eventBus to trigger msg broadcasts in the manager,
	// and to notify external subscribers, eg. through a websocket
	eventBus *types.EventBus

	// For tests where we want to limit the number of transitions the state makes
	nSteps int

	// a Write-Ahead Log ensures we can recover from any kind of crash
	// and helps us avoid signing conflicting votes
	wal          WAL
	replayMode   bool // so we don't log signing errors during replay
	doWALCatchup bool // determines if we even try to do the catchup

	// Synchronous pubsub between consensus state and manager.
	// State only emits EventNewRoundStep, EventVote and EventProposalHeartbeat
	evsw kevents.EventSwitch

	// closed when we finish shutting down
	done chan struct{}
}

// NewConsensusState returns a new ConsensusState.
func NewConsensusState(
	logger log.Logger,
	config *cfg.ConsensusConfig,
	state cstate.LastestBlockState,
	blockOperations BaseBlockOperations,
	blockExec *cstate.BlockExecutor,
	evpool evidencePool,
) *ConsensusState {
	cs := &ConsensusState{
		config:           config,
		blockExec:        blockExec,
		blockOperations:  blockOperations,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(),
		done:             make(chan struct{}),
		evpool:           evpool,
		doWALCatchup:     true,
		wal:              nilWAL{},
		evsw:             kevents.NewEventSwitch(),
	}
	cs.SetLogger(logger)
	// We have no votes, so reconstruct LastCommit from SeenCommit.
	if state.LastBlockHeight > 0 {
		cs.reconstructLastCommit(state)
	}

	cs.updateToState(state)

	// Set first round
	cs.Round = 1

	cs.BaseService = *service.NewBaseService(nil, "State", cs)

	// Don't call scheduleRound0 yet. We do that upon Start().

	return cs
}

// String returns a string.
func (cs *ConsensusState) String() string {
	// better not to access shared variables
	return "ConsensusState"
}

// SetLogger implements Service.
func (cs *ConsensusState) SetLogger(l log.Logger) {
	cs.BaseService.Logger = l
	cs.timeoutTicker.SetLogger(l)
}

// SetEventBus sets event bus.
func (cs *ConsensusState) SetEventBus(b *types.EventBus) {
	cs.eventBus = b
	cs.blockExec.SetEventBus(b)
}

// SetPrivValidator sets the private validator account for signing votes.
func (cs *ConsensusState) SetPrivValidator(priv types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
	cs.updatePrivValidatorAddress()
}

// loadWalFile loads WAL data from file. It overwrites cs.wal.
func (cs *ConsensusState) loadWalFile() error {
	wal, err := cs.OpenWAL(cs.config.WalFile())
	if err != nil {
		cs.Logger.Error("Error loading State wal", "err", err)
		return err
	}
	cs.wal = wal
	return nil
}

// OpenWAL opens a file to log all consensus messages and timeouts for
// deterministic accountability.
func (cs *ConsensusState) OpenWAL(walFile string) (WAL, error) {
	wal, err := NewWAL(walFile)
	if err != nil {
		cs.Logger.Error("Failed to open WAL", "file", walFile, "err", err)
		return nil, err
	}
	wal.SetLogger(cs.Logger.New("wal", walFile))
	if err := wal.Start(); err != nil {
		cs.Logger.Error("Failed to start WAL", "err", err)
		return nil, err
	}
	return wal, nil
}

// OnStart loads the latest state via the WAL, and starts the timeout and receive routines.
func (cs *ConsensusState) OnStart() error {
	cs.Logger.Info("Consensus state starts!")

	// We may set the WAL in testing before calling Start, so only OpenWAL if its
	// still the nilWAL.
	if _, ok := cs.wal.(nilWAL); ok {
		if err := cs.loadWalFile(); err != nil {
			return err
		}
	}

	// We may have lost some votes if the process crashed reload from consensus
	// log to catchup.
	if cs.doWALCatchup {
		repairAttempted := false
	LOOP:
		for {
			err := cs.catchupReplay(cs.Height)
			switch {
			case err == nil:
				break LOOP
			case !IsDataCorruptionError(err):
				cs.Logger.Error("Error on catchup replay. Proceeding to start State anyway", "err", err)
				break LOOP
			case repairAttempted:
				return err
			}

			cs.Logger.Error("WAL file is corrupted, attempting repair", "err", err)

			// 1) prep work
			if err := cs.wal.Stop(); err != nil {
				return err
			}
			repairAttempted = true

			// 2) backup original WAL file
			corruptedFile := fmt.Sprintf("%s.CORRUPTED", cs.config.WalFile())
			if err := kos.CopyFile(cs.config.WalFile(), corruptedFile); err != nil {
				return err
			}
			cs.Logger.Info("Backed up WAL file", "src", cs.config.WalFile(), "dst", corruptedFile)

			// 3) try to repair (WAL file will be overwritten!)
			if err := repairWalFile(corruptedFile, cs.config.WalFile()); err != nil {
				cs.Logger.Error("WAL repair failed", "err", err)
				return err
			}
			cs.Logger.Info("Successful repair")

			// reload WAL file
			if err := cs.loadWalFile(); err != nil {
				return err
			}
		}
	}

	if err := cs.evsw.Start(); err != nil {
		return err
	}

	// we need the timeoutRoutine for replay so
	// we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	// firing on the tockChan until the receiveRoutine is started
	// to deal with them (by that point, at most one will be valid)
	if err := cs.timeoutTicker.Start(); err != nil {
		return err
	}

	// Double Signing Risk Reduction
	if err := cs.checkDoubleSigningRisk(cs.Height); err != nil {
		return err
	}

	// now start the receiveRoutine
	go cs.receiveRoutine(0)

	// schedule the first round!
	// use GetRoundState so we don't race the receiveRoutine for access
	cs.scheduleRound0(cs.GetRoundState())
	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
func (cs *ConsensusState) startRoutines(maxSteps int) {
	err := cs.timeoutTicker.Start()
	if err != nil {
		cs.Logger.Error("Error starting timeout ticker", "err", err)
		return
	}
	go cs.receiveRoutine(maxSteps)
}

// OnStop stops all routines and waits for the WAL to finish.
func (cs *ConsensusState) OnStop() {
	cs.timeoutTicker.Stop()
	cs.Logger.Trace("Consensus state stops!")
}

// Updates ConsensusState and increments height to match that of state.
// The round becomes 0 and cs.Step becomes cstypes.RoundStepNewHeight.
func (cs *ConsensusState) updateToState(state cstate.LastestBlockState) {
	if (cs.CommitRound >= 1) && (cs.Height > 0) && cs.Height != state.LastBlockHeight {
		cmn.PanicSanity(cmn.Fmt("updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight))
	}
	if !cs.state.IsEmpty() {
		if cs.state.LastBlockHeight > 0 && cs.state.LastBlockHeight+1 != cs.Height {
			// This might happen when someone else is mutating cs.state.
			// Someone forgot to pass in state.Copy() somewhere?!
			panic(fmt.Sprintf("Inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v",
				cs.state.LastBlockHeight+1, cs.Height))
		}
		// If state isn't further out than cs.state, just ignore.
		// This happens when SwitchToConsensus() is called in the reactor.
		// We don't want to reset e.g. the Votes, but we still want to
		// signal the new round step, because other services (eg. txNotifier)
		// depend on having an up-to-date peer state!
		if state.LastBlockHeight <= cs.state.LastBlockHeight {
			cs.Logger.Info(
				"Ignoring updateToState()",
				"newHeight",
				state.LastBlockHeight+1,
				"oldHeight",
				cs.state.LastBlockHeight+1)
			cs.newStep()
			return
		}
	}

	// Reset fields based on state.
	validators := state.Validators

	switch {
	case state.LastBlockHeight == 0: // Very first commit should be empty.
		cs.LastCommit = (*types.VoteSet)(nil)
	case cs.CommitRound > 0 && cs.Votes != nil: // Otherwise, use cs.Votes
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			panic(fmt.Sprintf("Wanted to form a Commit, but Precommits (H/R: %d/%d) didn't have 2/3+: %v",
				state.LastBlockHeight,
				cs.CommitRound,
				cs.Votes.Precommits(cs.CommitRound)))
		}
		cs.LastCommit = cs.Votes.Precommits(cs.CommitRound)
	case cs.LastCommit == nil:
		// NOTE: when Tendermint starts, it has no votes. reconstructLastCommit
		// must be called to reconstruct LastCommit from SeenCommit.
		// @lew: Uncomment after implement switchToConsensus
		//panic(fmt.Sprintf("LastCommit cannot be empty after initial block (H:%d)",
		//	state.LastBlockHeight+1,
		//))
	}

	// Next desired block height
	height := state.LastBlockHeight + 1

	// RoundState fields
	cs.updateHeight(height)
	cs.updateRoundStep(1, cstypes.RoundStepNewHeight)
	if cs.CommitTime.IsZero() {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		//  cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		//cs.Logger.Trace("cs.CommitTime is 0")
		cs.StartTime = cs.config.Commit(ktime.Now())
	} else {
		cs.StartTime = cs.config.Commit(cs.CommitTime)
	}
	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.ValidRound = 0
	cs.ValidBlock = nil
	cs.ValidBlockParts = nil
	cs.Votes = cstypes.NewHeightVoteSet(cs.Logger, state.ChainID, height, validators)
	cs.CommitRound = 0
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
func (cs *ConsensusState) AddVote(vote *types.Vote, peerID p2p.ID) (bool, error) {
	if peerID != "" {
		cs.internalMsgQueue <- msgInfo{&VoteMessage{vote}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&VoteMessage{vote}, peerID}
	}

	// TODO: wait for event?!
	return false, nil
}

func (cs *ConsensusState) decideProposal(height uint64, round uint32) {
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
			cs.Logger.Trace("Create proposal block failed")
			return
		}
	}

	// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
	// and the privValidator will refuse to sign anything.
	if err := cs.wal.FlushAndSync(); err != nil {
		cs.Logger.Error("Error flushing to disk")
	}

	// Make proposal
	propBlockID := types.BlockID{Hash: block.Hash(), PartsHeader: blockParts.Header()}
	proposal := types.NewProposal(height, round, cs.ValidRound, propBlockID)
	p := proposal.ToProto()
	if err := cs.privValidator.SignProposal(cs.state.ChainID, p); err == nil {
		proposal.Signature = p.Signature
		// Send proposal and blockparts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
		for i := 0; i < int(blockParts.Total()); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}
		cs.Logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
		cs.Logger.Debug(fmt.Sprintf("Signed proposal block: %s", block.Hash()))
	} else if !cs.replayMode {
		cs.Logger.Error("enterPropose: Error signing proposal", "height", height, "round", round, "err", err)
	}
}

func (cs *ConsensusState) setProposal(proposal *types.Proposal) error {

	// Already have one
	// TODO: possibly catch double proposals
	if cs.Proposal != nil {
		return nil
	}

	// Does not apply
	if (proposal.Height != cs.Height) || (proposal.Round != cs.Round) {
		cs.Logger.Trace(fmt.Sprintf("CS[%v/%v] doesn't match Proposal[%v/%v]", cs.Height, cs.Round, proposal.Height, proposal.Round))
		return nil
	}

	// Verify POLRound, which must be -1 or between 0 and proposal.Round exclusive.
	if (proposal.POLRound < 1) && ((proposal.POLRound > 0) || (proposal.POLRound > proposal.Round)) {
		cs.Logger.Trace("Invalid proposal POLRound", "proposal.POLRound", proposal.POLRound, "proposal.Round", proposal.Round)
		return ErrInvalidProposalPOLRound
	}

	proposalAddress := cs.Validators.GetProposer().Address
	signBytes := types.ProposalSignBytes(cs.state.ChainID, proposal.ToProto())
	if !types.VerifySignature(proposalAddress, crypto.Keccak256(signBytes), proposal.Signature) {
		return ErrInvalidProposalPOLRound
	}
	cs.Proposal = proposal
	// We don't update cs.ProposalBlockParts if it is already set.
	// This happens if we're already in cstypes.RoundStepCommit or if there is a valid block in the current round.
	// TODO: We can check if Proposal is for a different block as this is a sign of misbehavior!
	if cs.ProposalBlockParts == nil {
		cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.POLBlockID.PartsHeader)
	}
	cs.Logger.Info("Received proposal", "proposal", proposal)
	return nil
}

// ------- HELPER METHODS -------- //

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(rs *cstypes.RoundState) {
	cs.Logger.Info("scheduleRound0", "now", time.Now(), "startTime", cs.StartTime)
	sleepDuration := rs.StartTime.Sub(ktime.Now())
	cs.scheduleTimeout(sleepDuration, rs.Height, 1, cstypes.RoundStepNewHeight)
}

// Attempt to schedule a timeout (by sending timeoutInfo on the tickChan)
func (cs *ConsensusState) scheduleTimeout(duration time.Duration, height uint64, round uint32, step cstypes.RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

// Send a msg into the receiveRoutine regarding our own proposal, or vote
func (cs *ConsensusState) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		cs.Logger.Info("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastCommit(state cstate.LastestBlockState) {
	if state.LastBlockHeight == 0 {
		return
	}
	seenCommit := cs.blockOperations.LoadSeenCommit(state.LastBlockHeight)

	lastPrecommits := types.CommitToVoteSet(state.ChainID, seenCommit, state.LastValidators)
	if !lastPrecommits.HasTwoThirdsMajority() {
		cmn.PanicSanity("Failed to reconstruct LastCommit: Does not have +2/3 maj")
	}

	cs.LastCommit = lastPrecommits
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *ConsensusState) tryAddVote(vote *types.Vote, peerID p2p.ID) (bool, error) {
	added, err := cs.addVote(vote, peerID)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, add it to the cs.evpool.
		// If it's otherwise invalid, punish peer.
		if voteErr, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if cs.privValidatorAddress == nil {
				return false, errAddressIsNotSet
			}
			if vote.ValidatorAddress.Equal(*cs.privValidatorAddress) {
				cs.Logger.Error("Found conflicting vote from ourselves. Did you unsafe_reset a validator?",
					"height", vote.Height,
					"round", vote.Round,
					"type", vote.Type)
				return added, err
			}

			var timestamp time.Time
			if voteErr.VoteA.Height == 1 {
				timestamp = cs.state.LastBlockTime // genesis time
			} else {
				timestamp = cstate.MedianTime(cs.LastCommit.MakeCommit(), cs.LastValidators)
			}

			evidence := types.NewDuplicateVoteEvidence(voteErr.VoteA, voteErr.VoteB, timestamp, cs.Validators)
			evidenceErr := cs.evpool.AddEvidenceFromConsensus(evidence)
			if evidenceErr != nil {
				cs.Logger.Error("Failed to add evidence to the evidence pool", "err", evidenceErr)
			} else {
				cs.Logger.Debug("Added evidence to the evidence pool", "evidence", evidence)
			}
			return added, err
		} else if err == types.ErrVoteNonDeterministicSignature {
			cs.Logger.Debug("Vote has non-deterministic signature", "err", err)
		} else {
			// Either
			// 1) bad peer OR
			// 2) not a bad peer? this can also err sometimes with "Unexpected step" OR
			// 3) use with multiple validators connecting to a single instance
			cs.Logger.Info("Error attempting to add vote", "err", err)
			return added, ErrAddingVote
		}
	}
	return added, nil
}

func (cs *ConsensusState) addVote(vote *types.Vote, peerID p2p.ID) (bool, error) {
	var (
		added bool
		err   error
	)
	cs.Logger.Debug(
		"addVote",
		"voteHeight",
		vote.Height,
		"voteType",
		vote.Type,
		"valIndex",
		vote.ValidatorIndex,
		"csHeight",
		cs.Height,
	)
	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height+1 == cs.Height && vote.Type == kproto.PrecommitType {
		if cs.Step != cstypes.RoundStepNewHeight {
			// Late precommit at prior height is ignored
			cs.Logger.Debug("Precommit vote came in after commit timeout and has been ignored", "vote", vote)
			return false, nil
		}

		added, err = cs.LastCommit.AddVote(vote)
		if !added {
			return false, nil
		}

		cs.Logger.Info(cmn.Fmt("Added to lastPrecommits: %v", cs.LastCommit.StringShort()))
		if err2 := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err2 != nil {
			return added, err2
		}

		cs.evsw.FireEvent(types.EventVote, vote)
		// if we can skip timeoutCommit and have all the votes now,
		if cs.config.IsSkipTimeoutCommit && cs.LastCommit.HasAll() {
			// go straight to new round (skip timeout commit)
			// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
			cs.enterNewRound(cs.Height, 1)
		}

		return false, nil
	}

	// Height mismatch is ignored.
	// Not necessarily a bad peer, but not favourable behaviour.
	if vote.Height != cs.Height {
		cs.Logger.Debug("Vote ignored and not added", "voteHeight", vote.Height, "csHeight", cs.Height, "peerID", peerID)
		return false, nil
	}

	height := cs.Height
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return false, nil
	}

	if err3 := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err3 != nil {
		return added, err3
	}
	cs.evsw.FireEvent(types.EventVote, vote)

	switch vote.Type {
	case kproto.PrevoteType:
		prevotes := cs.Votes.Prevotes(vote.Round)
		cs.Logger.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())

		// If +2/3 prevotes for a block or nil for *any* round:
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {
			// There was a polka!
			// If we're locked but this is a recent polka, unlock.
			// If it matches our ProposalBlock, update the ValidBlock

			// Unlock if `cs.LockedRound < vote.Round <= cs.Round`
			// NOTE: If vote.Round > cs.Round, we'll deal with it when we get to vote.Round
			if (cs.LockedBlock != nil) &&
				(cs.LockedRound < vote.Round) &&
				(vote.Round <= cs.Round) &&
				!cs.LockedBlock.HashesTo(blockID.Hash) {

				cs.Logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
				cs.LockedRound = 0
				cs.LockedBlock = nil
				cs.LockedBlockParts = nil
				if err4 := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err4 != nil {
					return added, err4
				}
			}

			// Update Valid* if we can.
			// NOTE: our proposal block may be nil or not what received a polka..
			if !blockID.Hash.IsZero() && (cs.ValidRound < vote.Round) && (vote.Round == cs.Round) {
				if cs.ProposalBlock.HashesTo(blockID.Hash) {
					cs.Logger.Info("Updating ValidBlock because of POL.", "validRound", cs.ValidRound, "POLRound", vote.Round)
					cs.ValidRound = vote.Round
					cs.ValidBlock = cs.ProposalBlock
					cs.ValidBlockParts = cs.ProposalBlockParts
				} else {
					cs.Logger.Info(
						"Valid block we don't know about. Set ProposalBlock=nil",
						"proposal", cs.ProposalBlock.Hash(), "blockId", blockID.Hash)
					// We're getting the wrong block.
					cs.ProposalBlock = nil
				}

				if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
					cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
				}

				cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
				if err5 := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err5 != nil {
					return added, err
				}
			}
		}

		// If +2/3 prevotes for *anything* for future round:
		switch {
		case (cs.Round < vote.Round) && prevotes.HasTwoThirdsAny():
			// Round-skip if there is any 2/3+ of votes ahead of us
			cs.enterNewRound(height, vote.Round)
		case (cs.Round == vote.Round) && (cstypes.RoundStepPrevote <= cs.Step):
			blockID, ok := prevotes.TwoThirdsMajority()
			if ok && (cs.isProposalComplete() || blockID.Hash.IsZero()) {
				cs.enterPrecommit(height, vote.Round)
			} else if prevotes.HasTwoThirdsAny() {
				cs.enterPrevoteWait(height, vote.Round)
			}
		case cs.Proposal != nil && (1 <= cs.Proposal.POLRound) && (cs.Proposal.POLRound == vote.Round):
			// If the proposal is now complete, enter prevote of cs.Round.
			if cs.isProposalComplete() {
				cs.enterPrevote(height, cs.Round)
			}
		}

	case kproto.PrecommitType:
		precommits := cs.Votes.Precommits(vote.Round)

		cs.Logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			// Executed as TwoThirdsMajority could be from a higher round
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommit(height, vote.Round)
			if !blockID.Hash.IsZero() {
				cs.enterCommit(height, vote.Round)
				if cs.config.IsSkipTimeoutCommit && precommits.HasAll() {
					cs.enterNewRound(cs.Height, 1)
				}
			} else {
				cs.enterPrecommitWait(height, vote.Round)
			}
		} else if (cs.Round <= vote.Round) && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}
	default:
		panic(cmn.Fmt("Unexpected vote type %X", vote.Type)) // go-wire should prevent this.
	}

	return added, err
}

// Signs vote.
func (cs *ConsensusState) signVote(signedMsgType kproto.SignedMsgType, hash cmn.Hash, header types.PartSetHeader) (*types.Vote, error) {
	// Flush the WAL. Otherwise, we may not recompute the same vote to sign,
	// and the privValidator will refuse to sign anything.
	if err := cs.wal.FlushAndSync(); err != nil {
		return nil, err
	}

	addr := cs.privValidatorAddress
	valIndex, _ := cs.Validators.GetByAddress(*addr)

	vote := &types.Vote{
		ValidatorAddress: *addr,
		ValidatorIndex:   uint32(valIndex),
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        cs.voteTime(),
		Type:             signedMsgType,
		BlockID:          types.BlockID{Hash: hash, PartsHeader: header},
	}
	v := vote.ToProto()
	err := cs.privValidator.SignVote(cs.state.ChainID, v)
	vote.Signature = v.Signature
	return vote, err
}

func (cs *ConsensusState) voteTime() time.Time {
	now := ktime.Now()
	minVoteTime := now
	// TODO: We should remove next line in case we don't vote for v in case cs.ProposalBlock == nil,
	// even if cs.LockedBlock != nil. See https://github.com/tendermint/spec.
	timeIotaMs := time.Duration(cs.state.ConsensusParams.Block.TimeIotaMs) * time.Millisecond
	if cs.LockedBlock != nil {
		// See the BFT time spec https://tendermint.com/docs/spec/consensus/bft-time.html
		minVoteTime = cs.LockedBlock.Time().Add(timeIotaMs)
	} else if cs.ProposalBlock != nil {
		minVoteTime = cs.ProposalBlock.Time().Add(timeIotaMs)
	}

	if now.After(minVoteTime) {
		return now
	}
	return minVoteTime
}

// Signs the vote and publish on internalMsgQueue
func (cs *ConsensusState) signAddVote(signedMsgType kproto.SignedMsgType, hash cmn.Hash, header types.PartSetHeader) *types.Vote {
	if cs.privValidator == nil { // the node does not have a key
		return nil
	}

	if cs.privValidatorAddress == nil {
		// Vote won't be signed, but it's not critical.
		cs.Logger.Error(fmt.Sprintf("signAddVote: %v", errAddressIsNotSet))
		return nil
	}

	// if we don't have a key or we're not in the validator set, do nothing
	if !cs.Validators.HasAddress(*cs.privValidatorAddress) {
		return nil
	}

	vote, err := cs.signVote(signedMsgType, hash, header)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, ""})
		cs.Logger.Info("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
		return vote
	}
	//if !cs.replayMode {
	cs.Logger.Error("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
	//}
	return nil
}

// Updates ConsensusState to the current round and round step.
func (cs *ConsensusState) updateRoundStep(round uint32, step cstypes.RoundStepType) {
	cs.Round = round
	cs.Step = step
}

// Advances to a new step.
func (cs *ConsensusState) newStep() {
	rs := cs.RoundStateEvent()
	cs.Logger.Trace("enter newStep()")

	if err := cs.wal.Write(rs); err != nil {
		cs.Logger.Error("Error writing to wal", "err", err)
	}

	cs.nSteps++

	if cs.eventBus != nil {
		if err := cs.eventBus.PublishEventNewRoundStep(rs); err != nil {
			cs.Logger.Error("Error publishing new round step", "err", err)
		}
		cs.evsw.FireEvent(types.EventNewRoundStep, &cs.RoundState)
	}
}

// SetProposal inputs a proposal.
func (cs *ConsensusState) SetProposal(proposal *types.Proposal, peerID p2p.ID) error {

	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&ProposalMessage{proposal}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&ProposalMessage{proposal}, peerID}
	}

	// TODO: wait for event?!
	return nil
}

// AddProposalBlockPart inputs a part of the proposal block.
func (cs *ConsensusState) AddProposalBlockPart(height uint64, round uint32, part *types.Part, peerID p2p.ID) error {

	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerID}
	}

	// TODO: wait for event?!
	return nil
}

// SetProposalAndBlock inputs the proposal and all block parts.
func (cs *ConsensusState) SetProposalAndBlock(
	proposal *types.Proposal,
	block *types.Block,
	parts *types.PartSet,
	peerID p2p.ID,
) error {
	if err := cs.SetProposal(proposal, peerID); err != nil {
		return err
	}
	for i := 0; i < int(parts.Total()); i++ {
		part := parts.GetPart(i)
		if err := cs.AddProposalBlockPart(proposal.Height, proposal.Round, part, peerID); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ConsensusState) updateHeight(height uint64) {
	//namdoh@ cs.metrics.Height.Set(float64(height))
	cs.Height = height
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit,
// once we have the full block.
func (cs *ConsensusState) addProposalBlockPart(msg *BlockPartMessage, peerID p2p.ID) (bool, error) {
	height, round, part := msg.Height, msg.Round, msg.Part
	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		cs.Logger.Debug("Received block part from wrong height", "height", height, "round", round)
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.Logger.Info("Received a block part when we're not expecting any",
			"height", height, "round", round, "index", part.Index, "peer", peerID)
		return false, nil
	}

	added, err := cs.ProposalBlockParts.AddPart(part)
	if err != nil {
		return added, err
	}

	if added && cs.ProposalBlockParts.IsComplete() {
		bz, err := ioutil.ReadAll(cs.ProposalBlockParts.GetReader())
		if err != nil {
			return added, err
		}

		var pbb = new(kproto.Block)
		err = proto.Unmarshal(bz, pbb)
		if err != nil {
			return added, err
		}
		block, err := types.BlockFromProto(pbb)
		if err != nil {
			return added, err
		}

		cs.ProposalBlock = block
		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		cs.Logger.Info("Received complete proposal block", "height", cs.ProposalBlock.Height(), "hash", cs.ProposalBlock.Hash())
		if err := cs.eventBus.PublishEventCompleteProposal(cs.CompleteProposalEvent()); err != nil {
			cs.Logger.Error("Error publishing event complete proposal", "err", err)
		}

		// Update Valid* if we can.
		prevotes := cs.Votes.Prevotes(cs.Round)
		blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
		if hasTwoThirds && !blockID.IsZero() && (cs.ValidRound < cs.Round) {
			if cs.ProposalBlock.HashesTo(blockID.Hash) {
				cs.Logger.Info("Updating valid block to new proposal block",
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
func (cs *ConsensusState) LoadCommit(height uint64) *types.Commit {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	if height == cs.blockOperations.Height() {
		return cs.blockOperations.LoadSeenCommit(height)
	}
	return cs.blockOperations.LoadBlockCommit(height)
}

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
// 	or, if SkipTimeout==true, after receiving all precommits from (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) enterNewRound(height uint64, round uint32) {
	logger := cs.Logger.New("height", height, "round", round)

	if (cs.Height != height) || (round < cs.Round) || (cs.Round == round) && (cs.Step != cstypes.RoundStepNewHeight) {
		logger.Debug(cmn.Fmt("enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	if now := time.Now(); cs.StartTime.After(now) {
		logger.Info("Need to set a buffer and log message here for sanity.", "startTime", cs.StartTime, "now", now)
	}

	logger.Info(cmn.Fmt("enterNewRound(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {
		// TODO(namdoh): Revisit to see if we need to copy validators here.
		validators = validators.Copy()
		validators.IncrementProposerPriority(int64(round - cs.Round))
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, cstypes.RoundStepNewRound)
	cs.Validators = validators
	if round == 1 {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		logger.Info("Resetting Proposal info")
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
	}
	cs.Votes.SetRound(round + 1) // also track next round (round+1) to allow round-skipping
	cs.TriggeredTimeoutPrecommit = false
	if err := cs.eventBus.PublishEventNewRound(cs.NewRoundEvent()); err != nil {
		cs.Logger.Error("Error publishing new round", "err", err)
	}

	// Wait for txs to be available in the mempool
	// before we enterPropose in round 0. If the last block changed the app hash,
	// we may need an empty "proof" block, and enterPropose immediately.
	waitForTxs := cs.config.WaitForTxs() && (round == 1)
	if waitForTxs {
		if cs.config.CreateEmptyBlocksInterval > 0 {
			cs.scheduleTimeout(cs.config.CreateEmptyBlocksInterval, height, round,
				cstypes.RoundStepNewRound)
		}

	} else {
		cs.enterPropose(height, round)
	}

}

// Enter (IsCreateEmptyBlocks): from enterNewRound(height,round)
// Enter (IsCreateEmptyBlocks, CreateEmptyBlocksInterval > 0 ): after enterNewRound(height,round), after timeout of CreateEmptyBlocksInterval
// Enter (!IsCreateEmptyBlocks) : after enterNewRound(height,round), once txs are in the mempool
func (cs *ConsensusState) enterPropose(height uint64, round uint32) {
	logger := cs.Logger.New("height", height, "round", round)
	if (cs.Height != height) || (round < cs.Round) || (cs.Round == round && cstypes.RoundStepPropose <= cs.Step) {
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
	cs.scheduleTimeout(cs.config.Propose(round), height, round, cstypes.RoundStepPropose)

	// TODO(namdoh): For now this any node is a validator. Remove it once we
	// restrict who can be validator.
	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("This node is not a validator")
		return
	}
	logger.Debug("This node is a validator")

	if cs.privValidatorAddress == nil {
		// If this node is a validator & proposer in the current round, it will
		// miss the opportunity to create a block.
		logger.Error(fmt.Sprintf("enterPropose: %v", errAddressIsNotSet))
		return
	}
	// if not a validator, we're done
	if !cs.Validators.HasAddress(*cs.privValidatorAddress) {
		logger.Debug("This node is not a validator", "addr", *cs.privValidatorAddress, "vals", cs.Validators)
		return
	}

	if cs.isProposer() {
		logger.Info("enterPropose: Our turn to propose",
			"proposer",
			*cs.privValidatorAddress,
			"privValidator",
			cs.privValidator)
		cs.decideProposal(height, round)
	} else {
		logger.Trace("enterPropose: Not our turn to propose",
			"proposer",
			cs.Validators.GetProposer().Address,
			"privValidator",
			cs.privValidator)
	}
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for future round.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) enterPrevote(height uint64, round uint32) {
	if (cs.Height != height) || (round < cs.Round) || (cs.Round == round && cstypes.RoundStepPrevote <= cs.Step) {
		cs.Logger.Debug(cmn.Fmt("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, cstypes.RoundStepPrevote)
		cs.newStep()
	}()

	cs.Logger.Info(cmn.Fmt("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *ConsensusState) doPrevote(height uint64, round uint32) {
	logger := cs.Logger.New("height", height, "round", round)
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Block was locked")
		cs.signAddVote(kproto.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Info("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(kproto.PrevoteType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// Validate proposal block
	// This checks the block contents without executing txs.
	if err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: ProposalBlock is invalid", "err", err)
		cs.signAddVote(kproto.PrevoteType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block is validated as it is received (against the merkle hash in the proposal)
	logger.Info("enterPrevote: ProposalBlock is valid")
	cs.signAddVote(kproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
}

// Enter: any +2/3 prevotes at next round.
func (cs *ConsensusState) enterPrevoteWait(height uint64, round uint32) {
	logger := cs.Logger.New("height", height, "round", round)

	if (cs.Height != height) || (round < cs.Round) || (cs.Round == round && cstypes.RoundStepPrevoteWait <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		cmn.PanicSanity(cmn.Fmt("enterPrevoteWait(%v/%v), but Prevotes does not have any +2/3 votes", height, round))
	}
	logger.Info(cmn.Fmt("enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.config.Prevote(round), height, round, cstypes.RoundStepPrevoteWait)
}

// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: +2/3 precomits for block or nil.
// Enter: any +2/3 precommits for next round.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (cs *ConsensusState) enterPrecommit(height uint64, round uint32) {
	cs.Logger.Trace("enterPrecommit", "height", height, "round", round)
	logger := cs.Logger.New("height", height, "round", round)

	if (cs.Height != height) || (round < cs.Round) || (cs.Round == round && cstypes.RoundStepPrecommit <= cs.Step) {
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
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(kproto.PrecommitType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	if err := cs.eventBus.PublishEventPolka(cs.RoundStateEvent()); err != nil {
		cs.Logger.Error("Error publishing polka", "err", err)
	}

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		cmn.PanicSanity(cmn.Fmt("This POLRound should be %v but got %", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if blockID.IsZero() {
		if cs.LockedBlock == nil {
			logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = 0
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
				cs.Logger.Error("Error publishing event unlock", "err", err)
			}
		}
		cs.signAddVote(kproto.PrecommitType, cmn.Hash{}, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		if err := cs.eventBus.PublishEventRelock(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("Error publishing event relock", "err", err)
		}
		cs.signAddVote(kproto.PrecommitType, blockID.Hash, blockID.PartsHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID)
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		if err := cs.eventBus.PublishEventLock(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("Error publishing event lock", "err", err)
		}
		cs.signAddVote(kproto.PrecommitType, blockID.Hash, blockID.PartsHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	// TODO: In the future save the POL prevotes for justification.
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
	}
	if err := cs.eventBus.PublishEventUnlock(cs.RoundStateEvent()); err != nil {
		cs.Logger.Error("Error publishing event unlock", "err", err)
	}
	cs.signAddVote(kproto.PrecommitType, cmn.Hash{}, types.PartSetHeader{})
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) enterPrecommitWait(height uint64, round uint32) {
	logger := log.New("height", height, "round", round)

	if (cs.Height != height) || (round != cs.Round) || (cs.Round == round && cs.TriggeredTimeoutPrecommit) {
		logger.Debug(
			fmt.Sprintf(
				"enterPrecommitWait(%v/%v): Invalid args. "+
					"Current state is Height/Round: %v/%v/, TriggeredTimeoutPrecommit:%v",
				height, round, cs.Height, cs.Round, cs.TriggeredTimeoutPrecommit))
		return
	}
	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}
	logger.Info(fmt.Sprintf("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.TriggeredTimeoutPrecommit = true
		cs.newStep()
	}()

	// Wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.config.Precommit(round), height, round, cstypes.RoundStepPrecommitWait)
}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height uint64, commitRound uint32) {
	logger := cs.Logger.New("height", height, "commitRound", commitRound)

	if (cs.Height != height) || cstypes.RoundStepCommit <= cs.Step {
		logger.Debug(cmn.Fmt("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(cmn.Fmt("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		cs.updateRoundStep(cs.Round, cstypes.RoundStepCommit)
		cs.CommitRound = commitRound
		cs.CommitTime = ktime.Now()
		cs.newStep()
		// Maybe finalize immediately.
		cs.tryFinalizeCommit(height)
	}()

	blockID, ok := cs.Votes.Precommits(commitRound).TwoThirdsMajority()
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
			_ = cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent())
			cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) tryFinalizeCommit(height uint64) {
	logger := cs.Logger.New("height", height)

	if cs.Height != height {
		cmn.PanicSanity(cmn.Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
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
func (cs *ConsensusState) finalizeCommit(height uint64) {
	if (cs.Height != height) || cs.Step != cstypes.RoundStepCommit {
		cs.Logger.Debug(cmn.Fmt("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
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
	if err := cs.blockExec.ValidateBlock(cs.state, block); err != nil {
		cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block: %v", err))
		panic("Block validation failed")
	}

	cs.Logger.Info("Finalizing commit of block", "tx number", block.NumTxs(),
		"height", block.Height(), "hash", block.Hash().String())

	fail.Fail() // XXX

	// Save block.
	if cs.blockOperations.Height() < block.Height() {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.Votes.Precommits(cs.CommitRound)
		seenCommit := precommits.MakeCommit()
		cs.Logger.Trace("Save new block", "block", block.Height(), "seenCommit", seenCommit)
		cs.blockOperations.SaveBlock(block, blockParts, seenCommit)
	} else {
		// Happens during replay if we already saved the block but didn't commit
		cs.Logger.Info("Calling finalizeCommit on already stored block", "height", block.Height())
	}

	fail.Fail() // XXX

	// Write EndHeightMessage{} for this height, implying that the blockstore
	// has saved the block.
	//
	// If we crash before writing this EndHeightMessage{}, we will recover by
	// running ApplyBlock during the ABCI handshake when we restart.  If we
	// didn't save the block to the blockstore before writing
	// EndHeightMessage{}, we'd have to change WAL replay -- currently it
	// complains about replaying for heights where an #ENDHEIGHT entry already
	// exists.
	//
	// Either way, the State should not be resumed until we
	// successfully call ApplyBlock (ie. later here, or in Handshake after
	// restart).
	endMsg := EndHeightMessage{Height: int64(height)}
	if err := cs.wal.WriteSync(endMsg); err != nil { // NOTE: fsync
		panic(fmt.Sprintf("Failed to write %v msg to consensus wal due to %v. Check your FS and restart the node",
			endMsg, err))
	}

	fail.Fail() // XXX

	// Create a copy of the state for staging and an event cache for txs.
	stateCopy := cs.state.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	var err error
	stateCopy, err = cs.blockExec.ApplyBlock(
		cs.Logger, stateCopy,
		types.BlockID{Hash: block.Hash(), PartsHeader: blockParts.Header()},
		block,
	)
	if err != nil {
		cs.Logger.Error("Error on ApplyBlock. Did the application crash? Please restart node", "err", err)
		err := cmn.Kill()
		if err != nil {
			cs.Logger.Error("Failed to kill this process - please do so manually", "err", err)
		}
		return
	}

	fail.Fail() // XXX
	// NewHeightStep!
	cs.updateToState(stateCopy)

	fail.Fail() // XXX

	// Private validator might have changed it's key pair => refetch address
	cs.updatePrivValidatorAddress()

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
func (cs *ConsensusState) createProposalBlock() (*types.Block, *types.PartSet) {
	if cs.privValidator == nil {
		panic("entered createProposalBlock with privValidator being nil")
	}
	cs.Logger.Trace("createProposalBlock")
	var commit *types.Commit
	if cs.Height == 1 {
		// We're creating a proposal for the first block.
		commit = &types.Commit{}
		cs.Logger.Trace("enterPropose: First height, use empty Commit.")
	} else if cs.LastCommit.HasTwoThirdsMajority() {
		// Make the commit from LastCommit
		commit = cs.LastCommit.MakeCommit()
		cs.Logger.Trace("enterPropose: Subsequent height, use last commit.", "commit", commit)
	} else {
		// This shouldn't happen.
		cs.Logger.Error("enterPropose: Cannot propose anything: No commit for the previous block.")
		return nil, nil
	}
	if cs.privValidatorAddress == nil {
		// If this node is a validator & proposer in the current round, it will
		// miss the opportunity to create a block.
		cs.Logger.Error(fmt.Sprintf("enterPropose: %v", errAddressIsNotSet))
		return nil, nil
	}
	return cs.blockOperations.CreateProposalBlock(
		cs.Height,
		cs.state,
		*cs.privValidatorAddress,
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
	if cs.Proposal.POLRound < 1 {
		return true
	}
	// if this is false the proposer is lying or we haven't received the POL yet
	return cs.Votes.Prevotes(cs.Proposal.POLRound).HasTwoThirdsMajority()
}

func (cs *ConsensusState) isProposer() bool {
	return cs.Validators.GetProposer().Address.Equal(*cs.privValidatorAddress)
}

// updatePrivValidatorAddress get's the private validator public key and
// memoizes it. This func returns an error if the private validator is not
// responding or responds with an error.
func (cs *ConsensusState) updatePrivValidatorAddress() {
	if cs.privValidator == nil {
		return
	}
	privValidatorAddress := cs.privValidator.GetAddress()
	cs.privValidatorAddress = &privValidatorAddress
	cs.Logger.Info("Private validator address updated", "privValidatorAddress", privValidatorAddress.Hex())
}

// look back to check existence of the node's consensus votes before joining consensus
func (cs *ConsensusState) checkDoubleSigningRisk(height uint64) error {
	if cs.privValidator != nil && cs.privValidatorAddress != nil && cs.config.DoubleSignCheckHeight > 0 && height > 0 {
		doubleSignCheckHeight := cs.config.DoubleSignCheckHeight
		if doubleSignCheckHeight > height {
			doubleSignCheckHeight = height
		}
		for i := uint64(1); i < doubleSignCheckHeight; i++ {
			lastCommit := cs.blockOperations.LoadSeenCommit(height - i)
			if lastCommit != nil {
				for sigIdx, s := range lastCommit.Signatures {
					if s.BlockIDFlag == types.BlockIDFlagCommit && s.ValidatorAddress.Equal(*cs.privValidatorAddress) {
						cs.Logger.Info("Found signature from the same key", "sig", s, "idx", sigIdx, "height", height-i)
						return ErrSignatureFoundInPastBlocks
					}
				}
			}
		}
	}
	return nil
}

// ----------- Other helpers -----------

func CompareHRS(h1 uint64, r1 uint32, s1 cstypes.RoundStepType, h2 uint64, r2 uint32, s2 cstypes.RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
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
	onExit := func(cs *ConsensusState) {
		// NOTE: the internalMsgQueue may have signed messages from our
		// priv_val that haven't hit the WAL, but its ok because
		// priv_val tracks LastSig

		// close wal now that we're done writing to it
		if err := cs.wal.Stop(); err != nil {
			cs.Logger.Error("error trying to stop wal", "error", err)
		}
		cs.wal.Wait()

		close(cs.done)
	}

	defer func() {
		if r := recover(); r != nil {
			cs.Logger.Error("CONSENSUS FAILURE!!!", "err", r, "stack", string(debug.Stack()))
			// stop gracefully
			//
			// NOTE: We most probably shouldn't be running any further when there is
			// some unexpected panic. Some unknown error happened, and so we don't
			// know if that will result in the validator signing an invalid thing. It
			// might be worthwhile to explore a mechanism for manual resuming via
			// some console or secure RPC system, but for now, halting the chain upon
			// unexpected consensus bugs sounds like the better option.
			onExit(cs)
		}
	}()
	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				cs.Logger.Info("reached max steps. exiting receive routine")
				cs.nSteps = 0
				return
			}
		}
		rs := cs.RoundState
		var mi msgInfo
		select {
		case mi = <-cs.peerMsgQueue:
			if err := cs.wal.Write(mi); err != nil {
				cs.Logger.Error("Error writing to wal", "err", err)
			}
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			cs.handleMsg(mi)
		case mi = <-cs.internalMsgQueue:
			err := cs.wal.WriteSync(mi) // NOTE: fsync
			if err != nil {
				panic(fmt.Sprintf("Failed to write %v msg to consensus wal due to %v. Check your FS and restart the node", mi, err))
			}
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
		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			if err := cs.wal.Write(ti); err != nil {
				cs.Logger.Error("Error writing to wal", "err", err)
			}

			// if the timeout is relevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)
		case <-cs.Quit():
			onExit(cs)
			return
		}
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *ConsensusState) handleMsg(mi msgInfo) {
	cs.Logger.Trace("handleMsg", "msgInfo", mi)
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
			cs.Logger.Debug(
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
		cs.Logger.Trace("handling AddVote", "VoteMessage", msg.Vote)
		_, err := cs.tryAddVote(msg.Vote, peerID)
		if err != nil {
			cs.Logger.Trace("trying to add vote failed", "err", err)
			if err == ErrAddingVote {
				cs.Logger.Warn("TODO - punish peer.")
			}
		}

	default:
		cs.Logger.Error("Unknown msg type", "msg_type", reflect.TypeOf(msg))
	}
	if err != nil {
		cs.Logger.Error("Error with msg", "height", cs.Height, "round", cs.Round, "type", reflect.TypeOf(msg), "peer", peerID, "err", err)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs cstypes.RoundState) {
	cs.Logger.Debug("Received tock", "timeout", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)

	// timeouts must be for current height, round, step
	if (ti.Height != rs.Height) || (ti.Round < rs.Round) || (ti.Round == rs.Round && ti.Step < rs.Step) {
		cs.Logger.Debug("Ignoring tick because we're ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case cstypes.RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		cs.enterNewRound(ti.Height, 1)
	case cstypes.RoundStepNewRound:
		cs.enterPropose(ti.Height, 1)
	case cstypes.RoundStepPropose:
		if err := cs.eventBus.PublishEventTimeoutPropose(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("Error publishing timeout propose", "err", err)
		}
		cs.enterPrevote(ti.Height, ti.Round)
	case cstypes.RoundStepPrevoteWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("Error publishing timeout wait", "err", err)
		}
		cs.enterPrecommit(ti.Height, ti.Round)
	case cstypes.RoundStepPrecommitWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.Logger.Error("Error publishing timeout wait", "err", err)
		}
		cs.enterPrecommit(ti.Height, ti.Round)
		cs.enterNewRound(ti.Height, ti.Round+1)
	default:
		panic(cmn.Fmt("Invalid timeout step: %v", ti.Step))
	}
}

// repairWalFile decodes messages from src (until the decoder errors) and
// writes them to dst.
func repairWalFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	var (
		dec = NewWALDecoder(in)
		enc = NewWALEncoder(out)
	)

	// best-case repair (until first error is encountered)
	for {
		msg, err := dec.Decode()
		if err != nil {
			break
		}

		err = enc.Encode(msg)
		if err != nil {
			return fmt.Errorf("failed to encode msg: %w", err)
		}
	}

	return nil
}
