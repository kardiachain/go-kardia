package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ebuchman/fail-test"

	"github.com/kardiachain/go-kardia/blockchain"
	cfg "github.com/kardiachain/go-kardia/configs"
	cstypes "github.com/kardiachain/go-kardia/consensus/types"
	"github.com/kardiachain/go-kardia/kai/dev"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	libevents "github.com/kardiachain/go-kardia/lib/events"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/p2p/discover"
	"github.com/kardiachain/go-kardia/state"
	"github.com/kardiachain/go-kardia/types"
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

func EmptyTimeoutInfo() *timeoutInfo {
	return &timeoutInfo{
		Duration: 0,
		Height:   cmn.NewBigInt(0),
		Round:    cmn.NewBigInt(0),
		Step:     0,
	}
}

// ConsensusState handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator,
// and from a timer.
type ConsensusState struct {
	Logger log.Logger

	config          *cfg.ConsensusConfig
	privValidator   *types.PrivValidator // for signing votes
	blockOperations *BlockOperations
	//evpool evidence.EvidencePool 	// TODO(namdoh): Add mem pool.

	// internal state
	mtx sync.RWMutex
	cstypes.RoundState
	state         state.LastestBlockState // State until height-1.
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
	evsw libevents.EventSwitch

	// closed when we finish shutting down
	done chan struct{}

	// Simulate voting strategy
	votingStrategy map[dev.VoteTurn]int

	// Development config, only used in dev/test environment
	devConfig *dev.DevEnvironmentConfig
}

// NewConsensusState returns a new ConsensusState.
func NewConsensusState(
	config *cfg.ConsensusConfig,
	state state.LastestBlockState,
	blockchain *blockchain.BlockChain,
	txPool *blockchain.TxPool,
	votingStrategy map[dev.VoteTurn]int,
) *ConsensusState {
	cs := &ConsensusState{
		Logger: log.New("module", "consensus"),
		config: config,
		//namdoh@ blockExec:        blockExec,
		blockOperations: NewBlockOperations(blockchain, txPool),
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(),
		done:             make(chan struct{}),
		evsw:             libevents.NewEventSwitch(),
		RoundState: cstypes.RoundState{
			CommitRound: cmn.NewBigInt(0),
			Height:      cmn.NewBigInt(0),
			StartTime:   big.NewInt(0),
			CommitTime:  big.NewInt(0),
		},
		votingStrategy: votingStrategy,
	}

	cs.updateToState(state)
	cs.timeoutTicker.SetLogger(cs.Logger)

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
	cs.Logger.Trace("Consensus state starts!")

	// we need the timeoutRoutine for replay so
	// we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	// firing on the tockChan until the receiveRoutine is started
	// to deal with them (by that point, at most one will be valid)
	if err := cs.timeoutTicker.Start(); err != nil {
		cs.Logger.Error("ConsensusState - Start", "err", err)
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
	cs.Logger.Trace("Consensus state stops!")
}

// Updates ConsensusState and increments height to match that of state.
// The round becomes 0 and cs.Step becomes cstypes.RoundStepNewHeight.
func (cs *ConsensusState) updateToState(state state.LastestBlockState) {
	cs.Logger.Trace("ConsensusState - updateToState")
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
		cs.Logger.Info("Ignoring updateToState()", "newHeight", state.LastBlockHeight.Int64()+1, "oldHeight", cs.state.LastBlockHeight.Int64()+1)
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
	cs.updateRoundStep(cmn.NewBigInt(0), cstypes.RoundStepNewHeight)
	if cs.CommitTime.Int64() == 0 {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		//  cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		cs.Logger.Trace("cs.CommitTime is 0")
		cs.StartTime = big.NewInt(cs.config.Commit(time.Now()).Unix() + 10)
	} else {
		cs.StartTime = big.NewInt(cs.config.Commit(time.Unix(cs.CommitTime.Int64(), 0)).Unix() + 10)
	}

	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockID = types.NewZeroBlockID()
	cs.LockedRound = cmn.NewBigInt(0)
	cs.LockedBlock = nil
	cs.ValidRound = cmn.NewBigInt(0)
	cs.ValidBlock = nil
	cs.Votes = cstypes.NewHeightVoteSet(state.ChainID, height, validators)
	cs.CommitRound = cmn.NewBigInt(-1)
	cs.LastCommit = lastPrecommits
	cs.LastValidators = state.LastValidators

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

	// Decide on block
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block = cs.LockedBlock
	} else if cs.ValidBlock != nil {
		// If there is valid block, choose that.
		block = cs.ValidBlock
	} else {
		// Create a new proposal block from state/txs from the mempool.
		// Decide on block
		block = cs.createProposalBlock()
		if block == nil { // on error
			cs.Logger.Trace("Create proposal block failed")
			return
		}
	}

	// Make proposal
	polRound, polBlockID := cs.Votes.POLInfo()
	proposal := types.NewProposal(height, round, block, cmn.NewBigInt(int64(polRound)), polBlockID)
	if err := cs.privValidator.SignProposal(cs.state.ChainID, proposal); err == nil {
		cs.Logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
		// Send proposal on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, discover.ZeroNodeID()})
	}
}

func (cs *ConsensusState) setProposal(proposal *types.Proposal) error {
	cs.Logger.Trace("setProposal()", "proposal", proposal)

	if cs.Proposal != nil {
		cs.Logger.Trace("cs.Proposal isn't nil. Returns early.")
		return nil
	}

	// Does not apply
	if !proposal.Height.Equals(cs.Height) || !proposal.Round.Equals(cs.Round) {
		cs.Logger.Trace(fmt.Sprintf("CS[%v/%v] doesn't match Proposal[%v/%v]", cs.Height, cs.Round, proposal.Height, proposal.Round))
		return nil
	}

	// We don't care about the proposal if we're already in cstypes.RoundStepCommit.
	if cstypes.RoundStepCommit <= cs.Step {
		cs.Logger.Trace("cs.Step is already beyond RoundStepCommit", "cs.Step", cs.Step)
		return nil
	}

	// Verify POLRound, which must be -1 or between 0 and proposal.Round exclusive.
	if !proposal.POLRound.EqualsInt(-1) &&
		(proposal.POLRound.IsLessThanInt(0) || proposal.Round.IsLessThanOrEquals(proposal.POLRound)) {
		cs.Logger.Trace("Invalid proposal POLRound", "proposal.POLRound", proposal.POLRound, "proposal.Round", proposal.Round)
		return ErrInvalidProposalPOLRound
	}

	// Verify signature
	if !cs.Validators.GetProposer().VerifyProposalSignature(cs.state.ChainID, proposal) {
		cs.Logger.Trace("Verify proposal signature failed.")
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	_, err := cs.setProposalBlock(proposal.Block)
	return err
}

// ------- HELPER METHODS -------- //

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(rs *cstypes.RoundState) {
	cs.Logger.Info("scheduleRound0", "now", time.Now(), "startTime", time.Unix(cs.StartTime.Int64(), 0))
	sleepDuration := time.Duration(rs.StartTime.Int64() - time.Now().Unix()) // nolint: gotype, gosimple
	cs.scheduleTimeout(sleepDuration, rs.Height, cmn.NewBigInt(0), cstypes.RoundStepNewHeight)
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
		cs.Logger.Info("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastCommit(state state.LastestBlockState) {
	if state.LastBlockHeight.EqualsInt64(0) {
		return
	}
	seenCommit := cs.blockOperations.LoadSeenCommit(uint64(state.LastBlockHeight.Int64()))
	lastPrecommits := types.NewVoteSet(state.ChainID, state.LastBlockHeight, seenCommit.Round(), types.VoteTypePrecommit, state.LastValidators)
	for _, precommit := range seenCommit.Precommits {
		if precommit == nil {
			continue
		}
		added, err := lastPrecommits.AddVote(precommit)
		if !added || err != nil {
			cmn.PanicCrisis(cmn.Fmt("Failed to reconstruct LastCommit: %v", err))
		}
	}
	if !lastPrecommits.HasTwoThirdsMajority() {
		cmn.PanicSanity("Failed to reconstruct LastCommit: Does not have +2/3 maj")
	}
	cs.LastCommit = lastPrecommits
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit, once we have the full block.
func (cs *ConsensusState) handleBlockMessage(msg *BlockMessage, peerID discover.NodeID) (added bool, err error) {
	cs.Logger.Trace("setProposalBlock", "msg", msg, "peerID", peerID)

	// Blocks might be reused, so round mismatch is OK
	if !cs.Height.Equals(msg.Height) {
		cs.Logger.Debug("Received block from wrong height", "msg.height", msg.Height, "msg.Round", msg.Round, "cs.Height", cs.Height, "cs.Round", cs.Round)
		return false, nil
	}

	if cs.ProposalBlockID.IsZero() {
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.Logger.Info("Received a block when we're not expecting any",
			"height", msg.Height, "round", msg.Round, "peer", peerID)
		return false, nil
	}

	return cs.setProposalBlock(msg.Block)
}

func (cs *ConsensusState) setProposalBlock(block *types.Block) (added bool, err error) {
	cs.ProposalBlock = block
	cs.ProposalBlockID = block.BlockID()
	// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
	cs.Logger.Info("Received complete proposal block", "height", cs.ProposalBlock.Height(), "hash", cs.ProposalBlock.Hash())

	// Update Valid* if we can.
	prevotes := cs.Votes.Prevotes(cs.Round.Int32())
	blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
	if hasTwoThirds && !blockID.IsZero() && cs.ValidRound.IsLessThan(cs.Round) {
		if cs.ProposalBlock.HashesTo(blockID) {
			cs.Logger.Info("Updating valid block to new proposal block",
				"valid-round", cs.Round, "valid-block-hash", cs.ProposalBlock.Hash())
			cs.ValidRound = cs.Round
			cs.ValidBlock = cs.ProposalBlock
		}
		// TODO: In case there is +2/3 majority in Prevotes set for some
		// block and cs.ProposalBlock contains different block, either
		// proposer is faulty or voting power of faulty processes is more
		// than 1/3. We should trigger in the future accountability
		// procedure at this point.
	}

	if cs.Step <= cstypes.RoundStepPropose && cs.isProposalComplete() {
		// Move onto the next step
		cs.enterPrevote(cmn.NewBigInt(int64(block.Height())), cs.Round)
	} else if cs.Step == cstypes.RoundStepCommit {
		// If we're waiting on the proposal block...
		cs.tryFinalizeCommit(cmn.NewBigInt(int64(block.Height())))
	}
	return true, nil
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *ConsensusState) tryAddVote(vote *types.Vote, peerID discover.NodeID) error {
	_, err := cs.addVote(vote, peerID)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, add it to the cs.evpool.
		// If it's otherwise invalid, punish peer.
		if err == ErrVoteHeightMismatch {
			return err
		} else if _, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if vote.ValidatorAddress.Equal(cs.privValidator.GetAddress()) {
				cs.Logger.Error("Found conflicting vote from ourselves. Did you unsafe_reset a validator?", "height", vote.Height, "round", vote.Round, "type", vote.Type)
				return err
			}
			// TODO(namdoh): Re-enable this later.
			cs.Logger.Warn("Add vote error to evidence pool later")
			return err
		} else {
			// Probably an invalid signature / Bad peer.
			// Seems this can also err sometimes with "Unexpected step" - perhaps not from a bad peer ?
			cs.Logger.Error("Error attempting to add vote", "err", err)
			return ErrAddingVote
		}
	}
	return nil
}

func (cs *ConsensusState) addVote(vote *types.Vote, peerID discover.NodeID) (added bool, err error) {
	cs.Logger.Debug("addVote", "voteHeight", vote.Height, "voteType", vote.Type, "valIndex", vote.ValidatorIndex, "csHeight", cs.Height)

	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height.Add(1).Equals(cs.Height) {
		if !(cs.Step == cstypes.RoundStepNewHeight && vote.Type == types.VoteTypePrecommit) {
			return added, ErrVoteHeightMismatch
		}
		added, err = cs.LastCommit.AddVote(vote)
		if !added {
			return added, err
		}

		cs.Logger.Info(cmn.Fmt("Added to lastPrecommits: %v", cs.LastCommit.StringShort()))
		cs.eventBus.PublishEventVote(types.EventDataVote{vote})
		cs.evsw.FireEvent(types.EventVote, vote)

		// if we can skip timeoutCommit and have all the votes now,
		if cs.config.SkipTimeoutCommit && cs.LastCommit.HasAll() {
			// go straight to new round (skip timeout commit)
			// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
			cs.enterNewRound(cs.Height, cmn.NewBigInt(0))
		}

		return
	}

	// Height mismatch is ignored.
	// Not necessarily a bad peer, but not favourable behaviour.
	if !vote.Height.Equals(cs.Height) {
		err = ErrVoteHeightMismatch
		cs.Logger.Info("Vote ignored and not added", "voteHeight", vote.Height, "csHeight", cs.Height, "err", err)
		return
	}

	height := cs.Height
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	}

	cs.eventBus.PublishEventVote(types.EventDataVote{vote})
	cs.evsw.FireEvent(types.EventVote, vote)

	switch vote.Type {
	case types.VoteTypePrevote:
		prevotes := cs.Votes.Prevotes(vote.Round.Int32())
		cs.Logger.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())

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
				!cs.LockedBlock.HashesTo(blockID) {

				cs.Logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
				cs.LockedRound = cmn.NewBigInt(0)
				cs.LockedBlock = nil
				cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
			}

			// Update Valid* if we can.
			// NOTE: our proposal block may be nil or not what received a polka..
			// TODO: we may want to still update the ValidBlock and obtain it via gossipping
			if !blockID.IsZero() &&
				cs.ValidRound.IsLessThan(vote.Round) &&
				vote.Round.IsLessThanOrEquals(cs.Round) &&
				cs.ProposalBlock.HashesTo(blockID) {

				cs.Logger.Info("Updating ValidBlock because of POL.", "validRound", cs.ValidRound, "POLRound", vote.Round)
				cs.ValidRound = vote.Round
				cs.ValidBlock = cs.ProposalBlock
			}
		}

		// If +2/3 prevotes for *anything* for this or future round:
		if cs.Round.IsLessThanOrEquals(vote.Round) && prevotes.HasTwoThirdsAny() {
			// Round-skip over to PrevoteWait or goto Precommit.
			cs.enterNewRound(height, vote.Round) // if the vote is ahead of us
			if prevotes.HasTwoThirdsMajority() {
				cs.enterPrecommit(height, vote.Round)
			} else {
				cs.enterPrevote(height, vote.Round) // if the vote is ahead of us
				cs.enterPrevoteWait(height, vote.Round)
			}
		} else if cs.Proposal != nil && cs.Proposal.POLRound.IsGreaterThanOrEqualToInt(0) && cs.Proposal.POLRound.Equals(vote.Round) {
			// If the proposal is now complete, enter prevote of cs.Round.
			if cs.isProposalComplete() {
				cs.enterPrevote(height, cs.Round)
			}
		}

	case types.VoteTypePrecommit:
		precommits := cs.Votes.Precommits(vote.Round.Int32())
		cs.Logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			if blockID.IsZero() {
				cs.enterNewRound(height, vote.Round.Add(1))
			} else {
				cs.enterNewRound(height, vote.Round)
				cs.enterPrecommit(height, vote.Round)
				cs.enterCommit(height, vote.Round)

				if cs.config.SkipTimeoutCommit && precommits.HasAll() {
					// if we have all the votes now,
					// go straight to new round (skip timeout commit)
					// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
					cs.enterNewRound(cs.Height, cmn.NewBigInt(0))
				}

			}
		} else if cs.Round.IsLessThanOrEquals(vote.Round) && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommit(height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}
	default:
		panic(cmn.Fmt("Unexpected vote type %X", vote.Type)) // go-wire should prevent this.
	}

	return
}

// Get script vote
func (cs *ConsensusState) scriptedVote(height int, round int, voteType int) (int, bool) {
	if val, ok := cs.votingStrategy[dev.VoteTurn{height, round, voteType}]; ok {
		return val, ok
	}
	return 0, false
}

// Signs vote.
func (cs *ConsensusState) signVote(type_ byte, hash types.BlockID) (*types.Vote, error) {
	addr := cs.privValidator.GetAddress()
	valIndex, _ := cs.Validators.GetByAddress(addr)
	// Simulate voting strategy
	if cs.votingStrategy != nil {
		if votingStrategy, ok := cs.scriptedVote(cs.Height.Int32(), cs.Round.Int32(), int(type_)); ok {
			if ok && votingStrategy == -1 {
				log.Info("Simulate voting strategy", "Height", cs.Height, "Round", cs.Round, "VoteType", cs.Step, "VotingStrategy", votingStrategy)
				hash = types.NewZeroBlockID()
			}
		}
	}

	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   cmn.NewBigInt(int64(valIndex)),
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        big.NewInt(time.Now().Unix()),
		Type:             type_,
		BlockID:          hash,
	}
	err := cs.privValidator.SignVote(cs.state.ChainID, vote)
	return vote, err
}

// Signs the vote and publish on internalMsgQueue
func (cs *ConsensusState) signAddVote(type_ byte, hash types.BlockID) *types.Vote {
	// if we don't have a key or we're not in the validator set, do nothing
	if cs.privValidator == nil || !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
		return nil
	}
	vote, err := cs.signVote(type_, hash)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, discover.ZeroNodeID()})
		cs.Logger.Info("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
		return vote
	}
	//if !cs.replayMode {
	cs.Logger.Error("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
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
	cs.Logger.Trace("enter newStep()")

	cs.nSteps++
	cs.evsw.FireEvent(types.EventNewRoundStep, &cs.RoundState)
}

func (cs *ConsensusState) updateHeight(height *cmn.BigInt) {
	//namdoh@ cs.metrics.Height.Set(float64(height))
	cs.Height = height
}

// GetRoundState returns a shallow copy of the internal consensus state.
func (cs *ConsensusState) GetRoundState() *cstypes.RoundState {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	rs := cs.RoundState // copy
	return &rs
}

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
// 	or, if SkipTimeout==true, after receiving all precommits from (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) enterNewRound(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.Logger.New("height", height, "round", round)

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
		validators = validators.Copy()
		validators.IncrementAccum(int(round.Int64() - cs.Round.Int64()))
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
		cs.ProposalBlockID = types.NewZeroBlockID()
	}
	cs.Votes.SetRound(round.Int32() + 1) // also track next round (round+1) to allow round-skipping

	cs.eventBus.PublishEventNewRound(cs.RoundStateEvent())

	cs.enterPropose(height, round)
}

// Enter (CreateEmptyBlocks): from enterNewRound(height,round)
// Enter (CreateEmptyBlocks, CreateEmptyBlocksInterval > 0 ): after enterNewRound(height,round), after timeout of CreateEmptyBlocksInterval
// Enter (!CreateEmptyBlocks) : after enterNewRound(height,round), once txs are in the mempool
func (cs *ConsensusState) enterPropose(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.Logger.New("height", height, "round", round)

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
	//if cs.privValidator == nil {
	//	logger.Debug("This node is not a validator")
	//	return
	//}
	// if not a validator, we're done
	//if !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
	//	logger.Debug("This node is not a validator", "addr", cs.privValidator.GetAddress(), "vals", cs.Validators)
	//	return
	//}

	logger.Trace("This node is a validator")
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
		cs.Logger.Debug(cmn.Fmt("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, cstypes.RoundStepPrevote)
		cs.newStep()
	}()

	// fire event for how we got here
	if cs.isProposalComplete() {
		cs.eventBus.PublishEventCompleteProposal(cs.RoundStateEvent())
	} else {
		// we received +2/3 prevotes for a future round
		// TODO: catchup event?
	}

	cs.Logger.Info(cmn.Fmt("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *ConsensusState) doPrevote(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.Logger.New("height", height, "round", round)
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Block was locked")
		cs.signAddVote(types.VoteTypePrevote, cs.LockedBlock.BlockID())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Info("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.VoteTypePrevote, types.NewZeroBlockID())
		return
	}

	// Validate proposal block
	// This checks the block contents without executing txs.
	if err := state.ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: ProposalBlock is invalid", "err", err)
		cs.signAddVote(types.VoteTypePrevote, types.NewZeroBlockID())
		return
	}
	// Executes txs to verify the block state root. New statedb is committed if success.
	if err := cs.blockOperations.CommitAndValidateBlockTxs(cs.ProposalBlock); err != nil {
		logger.Error("enterPrevote: fail to commit & verify txs", "err", err)
		cs.signAddVote(types.VoteTypePrevote, types.NewZeroBlockID())
		return
	} else {
		logger.Info("Successfully executes and commits block txs")
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block is validated as it is received (against the merkle hash in the proposal)
	logger.Info("enterPrevote: ProposalBlock is valid")
	cs.signAddVote(types.VoteTypePrevote, cs.ProposalBlock.BlockID())
}

// Enter: any +2/3 prevotes at next round.
func (cs *ConsensusState) enterPrevoteWait(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.Logger.New("height", height, "round", round)

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
	cs.Logger.Trace("enterPrecommit", "height", height, "round", round)
	logger := cs.Logger.New("height", height, "round", round)

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
		cs.signAddVote(types.VoteTypePrecommit, types.NewZeroBlockID())
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
			cs.LockedRound = cmn.NewBigInt(0)
			cs.LockedBlock = nil
			cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}
		cs.signAddVote(types.VoteTypePrecommit, types.NewZeroBlockID())
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock != nil && cs.LockedBlock.HashesTo(blockID) {
		logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		cs.eventBus.PublishEventRelock(cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID) {
		logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID)
		// Validate the block.
		if err := state.ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
			cmn.PanicConsensus(cmn.Fmt("enterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.eventBus.PublishEventLock(cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	// TODO: In the future save the POL prevotes for justification.
	cs.LockedRound = cmn.NewBigInt(0)
	cs.LockedBlock = nil
	if !cs.ProposalBlockID.Equal(blockID) {
		cs.ProposalBlock = nil
		cs.ProposalBlockID = blockID
	}
	cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
	cs.signAddVote(types.VoteTypePrecommit, types.NewZeroBlockID())
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) enterPrecommitWait(height *cmn.BigInt, round *cmn.BigInt) {
	logger := cs.Logger.New("height", height, "round", round)

	if !cs.Height.Equals(height) || round.IsLessThan(cs.Round) || (cs.Round.Equals(round) && cstypes.RoundStepPrecommitWait <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Precommits(round.Int32()).HasTwoThirdsAny() {
		cmn.PanicSanity(cmn.Fmt("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}
	logger.Info(cmn.Fmt("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrecommitWait)
		cs.newStep()
	}()

	// Wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.config.Precommit(round.Int32()), height, round, cstypes.RoundStepPrecommitWait)
}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height *cmn.BigInt, commitRound *cmn.BigInt) {
	logger := cs.Logger.New("height", height, "commitRound", commitRound)

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
	if cs.LockedBlock != nil && cs.LockedBlock.HashesTo(blockID) {
		logger.Info("Commit is for locked block. Set ProposalBlock=LockedBlock", "blockHash", blockID)
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockID = blockID
	}

	// If we don't have the block being committed, set up to get it.
	// cs.ProposalBlock is confirmed not nil from caller.
	if cs.ProposalBlock == nil || !cs.ProposalBlock.HashesTo(blockID) {
		logger.Info("Commit is for a block we don't know about. Set ProposalBlock=nil", "proposal", cs.ProposalBlock.Hash(), "commit", blockID)
		// We're getting the wrong block.
		// Set up ProposalBlock and keep waiting.
		if !cs.ProposalBlockID.Equal(blockID) {
			cs.ProposalBlock = nil
			cs.ProposalBlockID = blockID
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) tryFinalizeCommit(height *cmn.BigInt) {
	logger := cs.Logger.New("height", height)

	if !cs.Height.Equals(height) {
		cmn.PanicSanity(cmn.Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound.Int32()).TwoThirdsMajority()
	if !ok || blockID.IsZero() {
		logger.Error("Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.")
		return
	}
	if cs.ProposalBlock == nil {
		logger.Info("Attempt to finalize failed. Proposed block is nil.")
		return
	}

	if !cs.ProposalBlock.HashesTo(blockID) {
		logger.Info("Attempt to finalize failed. We don't have the commit block.", "proposal-block", cs.ProposalBlock.BlockID(), "commit-block", blockID)
		return
	}

	cs.finalizeCommit(height)
}

// Increment height and goto cstypes.RoundStepNewHeight
func (cs *ConsensusState) finalizeCommit(height *cmn.BigInt) {
	if !cs.Height.Equals(height) || cs.Step != cstypes.RoundStepCommit {
		cs.Logger.Debug(cmn.Fmt("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound.Int32()).TwoThirdsMajority()
	block, proposalBlockID := cs.ProposalBlock, cs.ProposalBlockID

	if !ok {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, commit does not have two thirds majority"))
	}
	if !proposalBlockID.Equal(blockID) {
		cmn.PanicSanity(cmn.Fmt("Expected ProposalBlockID to match the commiting block"))
	}
	if !block.HashesTo(blockID) {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	if err := state.ValidateBlock(cs.state, block); err != nil {
		cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block: %v", err))
		panic("Block validation failed")
	}

	cs.Logger.Info(cmn.Fmt("Finalizing commit of block with %d txs", block.NumTxs),
		"height", block.Height, "hash", block.Hash())
	cs.Logger.Info(cmn.Fmt("%v", block))

	fail.Fail() // XXX

	// Save block.
	if cs.blockOperations.Height() < block.Height() {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.Votes.Precommits(cs.CommitRound.Int32())
		seenCommit := precommits.MakeCommit()
		cs.Logger.Trace("Save new block", "block", block, "seenCommit", seenCommit)
		cs.blockOperations.SaveBlock(block, seenCommit)
	} else {
		// Happens during replay if we already saved the block but didn't commit
		cs.Logger.Info("Calling finalizeCommit on already stored block", "height", block.Height())
	}

	fail.Fail() // XXX

	// Create a copy of the state for staging and an event cache for txs.
	stateCopy := cs.state.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	var err error
	stateCopy, err = state.ApplyBlock(stateCopy, block.BlockID(), block)
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
func (cs *ConsensusState) createProposalBlock() (block *types.Block) {
	cs.Logger.Trace("createProposalBlock")
	var commit *types.Commit
	if cs.Height.EqualsInt(1) {
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
		return
	}

	// Gets all transactions in pending pools and execute them to get new account states.
	// Tx execution can happen in parallel with voting or precommitted.
	// For simplicity, this code executes & commits txs before sending proposal,
	// so statedb of proposal node already contains the new state and txs receipts of this proposal block.
	txs := cs.blockOperations.CollectTransactions()
	log.Debug("Collected transactions", "txs", txs)

	header := cs.blockOperations.NewHeader(cs.Height.Int64(), uint64(len(txs)), cs.state.LastBlockID, cs.state.LastValidators.Hash())
	log.Info("Creates new header", "header", header)

	stateRoot, receipts, err := cs.blockOperations.CommitTransactions(txs, header)
	if err != nil {
		log.Error("Fail to commit transactions", "err", err)
	}
	header.Root = stateRoot

	block = cs.blockOperations.NewBlock(header, txs, receipts, commit)
	cs.Logger.Trace("Make block to propose", "block", block)

	cs.blockOperations.SaveReceipts(receipts, block)

	return block
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
			cs.Logger.Error("CONSENSUS FAILURE!!!", "err", r, "stack", string(debug.Stack()))
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
			// handles proposals, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
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
	cs.Logger.Trace("handleMsg", "msgInfo", mi)
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	var err error
	msg, peerID := mi.Msg, mi.PeerID
	switch msg := msg.(type) {
	case *ProposalMessage:
		cs.Logger.Trace("handling ProposalMessage", "ProposalMessage", msg)
		err = cs.setProposal(msg.Proposal)
	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		cs.Logger.Trace("handling AddVote", "VoteMessage", msg)
		err := cs.tryAddVote(msg.Vote, peerID)
		if err == ErrAddingVote {
			cs.Logger.Trace("trying to add vote failed", "err", err)
			cs.Logger.Warn("TODO - punish peer.")
		}
	case *BlockMessage:
		cs.Logger.Trace("handling BlockMessage", "msg", msg)
		_, err = cs.handleBlockMessage(msg, peerID)
		if err != nil && !msg.Round.Equals(cs.Round) {
			cs.Logger.Debug("Received block from wrong round", "height", cs.Height, "csRound", cs.Round, "blockRound", msg.Round)
			err = nil
		}
	default:
		cs.Logger.Error("Unknown msg type", "msg_type", reflect.TypeOf(msg))
	}
	if err != nil {
		cs.Logger.Error("Error with msg", "height", cs.Height, "round", cs.Round, "type", reflect.TypeOf(msg), "peer", peerID, "err", err, "msg", msg)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs cstypes.RoundState) {
	cs.Logger.Debug("Received tock", "timeout", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)

	//// timeouts must be for current height, round, step
	if !ti.Height.Equals(rs.Height) || ti.Round.IsLessThan(rs.Round) || (ti.Round.Equals(rs.Round) && ti.Step < rs.Step) {
		cs.Logger.Debug("Ignoring tick because we're ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case cstypes.RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		cs.enterNewRound(ti.Height, cmn.NewBigInt(0))
	case cstypes.RoundStepNewRound:
		cs.enterPropose(ti.Height, cmn.NewBigInt(0))
	case cstypes.RoundStepPropose:
		cs.enterPrevote(ti.Height, ti.Round)
	case cstypes.RoundStepPrevoteWait:
		cs.enterPrecommit(ti.Height, ti.Round)
	case cstypes.RoundStepPrecommitWait:
		cs.enterNewRound(ti.Height, ti.Round.Add(1))
	default:
		panic(cmn.Fmt("Invalid timeout step: %v", ti.Step))
	}
}
