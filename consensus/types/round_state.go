package types

import (
	"time"

	"github.com/kardiachain/go-kardia/types"
)

//-----------------------------------------------------------------------------
// RoundStepType enum type

// RoundStepType enumerates the state of the consensus state machine
type RoundStepType uint8 // These must be numeric, ordered.

// RoundStepType
const (
	RoundStepNewHeight     = RoundStepType(0x01) // Wait til CommitTime + timeoutCommit
	RoundStepNewRound      = RoundStepType(0x02) // Setup new round and go to RoundStepPropose
	RoundStepPropose       = RoundStepType(0x03) // Did propose, gossip proposal
	RoundStepPrevote       = RoundStepType(0x04) // Did prevote, gossip prevotes
	RoundStepPrevoteWait   = RoundStepType(0x05) // Did receive any +2/3 prevotes, start timeout
	RoundStepPrecommit     = RoundStepType(0x06) // Did precommit, gossip precommits
	RoundStepPrecommitWait = RoundStepType(0x07) // Did receive any +2/3 precommits, start timeout
	RoundStepCommit        = RoundStepType(0x08) // Entered commit state machine
	// NOTE: RoundStepNewHeight acts as RoundStepCommitWait.
)

// String returns a string
func (rs RoundStepType) String() string {
	switch rs {
	case RoundStepNewHeight:
		return "RoundStepNewHeight"
	case RoundStepNewRound:
		return "RoundStepNewRound"
	case RoundStepPropose:
		return "RoundStepPropose"
	case RoundStepPrevote:
		return "RoundStepPrevote"
	case RoundStepPrevoteWait:
		return "RoundStepPrevoteWait"
	case RoundStepPrecommit:
		return "RoundStepPrecommit"
	case RoundStepPrecommitWait:
		return "RoundStepPrecommitWait"
	case RoundStepCommit:
		return "RoundStepCommit"
	default:
		return "RoundStepUnknown" // Cannot panic.
	}
}

//-----------------------------------------------------------------------------

// RoundState defines the internal consensus state.
// NOTE: Not thread safe. Should only be manipulated by functions downstream
// of the cs.receiveRoutine
type RoundState struct {
	Height        int64               `json:"height"` // Height we are working on
	Round         int                 `json:"round"`
	Step          RoundStepType       `json:"step"`
	StartTime     time.Time           `json:"start_time"`
	CommitTime    time.Time           `json:"commit_time"` // Subjective time when +2/3 precommits for Block at Round were found
	Validators    *types.ValidatorSet `json:"validators"`  // TODO(huny@): Assume static validator set for now
	Proposal      *types.Proposal     `json:"proposal"`
	ProposalBlock *types.Block        `json:"proposal_block"`
	LockedRound   int                 `json:"locked_round"`
	LockedBlock   *types.Block        `json:"locked_block"`
	ValidRound    int                 `json:"valid_round"` // Last known round with POL for non-nil valid block.
	ValidBlock    *types.Block        `json:"valid_block"` // Last known block of POL mentioned above.
	Votes         *HeightVoteSet      `json:"votes"`
	CommitRound   int                 `json:"commit_round"` //
	LastCommit    *types.VoteSet      `json:"last_commit"`  // Last precommits at Height-1
}

// RoundStateEvent returns the H/R/S of the RoundState as an event.
func (rs *RoundState) RoundStateEvent() types.EventDataRoundState {
	// XXX: copy the RoundState
	// if we want to avoid this, we may need synchronous events after all
	rsCopy := *rs
	edrs := types.EventDataRoundState{
		Height:     rs.Height,
		Round:      rs.Round,
		Step:       rs.Step.String(),
		RoundState: &rsCopy,
	}
	return edrs
}
