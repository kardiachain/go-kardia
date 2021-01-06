// Package consensus
package consensus

import (
	"errors"
)

var (
	ErrInvalidProposalPOLRound    = errors.New("error invalid proposal POL round")
	ErrAddingVote                 = errors.New("error adding vote")
	ErrNegativeHeight             = errors.New("negative Height")
	ErrNegativeRound              = errors.New("negative Round")
	ErrNegativeIndex              = errors.New("negative Index")
	ErrNegativeProposalPOLRound   = errors.New("negative ProposalPOLRound")
	ErrEmptyProposalPOL           = errors.New("empty ProposalPOL bit array")
	ErrInvalidMsgType             = errors.New("invalid message Type")
	ErrEmptyBlockPart             = errors.New("empty BlockParts")
	ErrNilMsg                     = errors.New("message is Nil")
	ErrConsensusMgrNotRunning     = errors.New("consensus manager is not running")
	ErrSignatureFoundInPastBlocks = errors.New("found signature from the same key")
)
