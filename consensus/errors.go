// Package consensus
package consensus

import (
	"errors"
)

var (
	ErrInvalidProposalPOLRound  = errors.New("error invalid proposal POL round")
	ErrAddingVote               = errors.New("error adding vote")
	ErrVoteHeightMismatch       = errors.New("error vote height mismatch")
	ErrNegativeHeight           = errors.New("negative Height")
	ErrNegativeRound            = errors.New("negative Round")
	ErrNegativeIndex            = errors.New("negative Index")
	ErrNegativeLastCommitRound  = errors.New("negative LastCommitRound")
	ErrNegativeProposalPOLRound = errors.New("negative ProposalPOLRound")
	ErrEmptyProposalPOL         = errors.New("empty ProposalPOL bit array")
	ErrInvalidMsgType           = errors.New("invalid message Type")
	ErrInvalidStep              = errors.New("invalid step")
	ErrEmptyBlockPart           = errors.New("empty BlockParts")
	ErrNilMsg                   = errors.New("message is Nil")
)
