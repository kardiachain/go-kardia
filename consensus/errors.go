// Package consensus
package consensus

import (
	"errors"
)

var (
	// Look like not used (14/11/2020)
	//ErrInvalidProposalSignature = errors.New("error invalid proposal signature")
	ErrInvalidProposalPOLRound = errors.New("error invalid proposal POL round")
	ErrAddingVote              = errors.New("error adding vote")
	ErrVoteHeightMismatch      = errors.New("error vote height mismatch")
)
