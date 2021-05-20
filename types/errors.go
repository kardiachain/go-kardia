// Package types
package types

import (
	"errors"
	"fmt"
)

// ErrEvidenceOverflow is for when there is too much evidence in a block.
type ErrEvidenceOverflow struct {
	MaxNum int64
	GotNum int64
}

// NewErrEvidenceOverflow returns a new ErrEvidenceOverflow where got > max.
func NewErrEvidenceOverflow(max, got int64) *ErrEvidenceOverflow {
	return &ErrEvidenceOverflow{max, got}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceOverflow) Error() string {
	return fmt.Sprintf("Too much evidence: Max %d, got %d", err.MaxNum, err.GotNum)
}

// ErrEvidenceInvalid wraps a piece of evidence and the error denoting how or why it is invalid.
type ErrEvidenceInvalid struct {
	Evidence Evidence
	Reason   error
}

// NewErrEvidenceInvalid returns a new EvidenceInvalid with the given err.
func NewErrInvalidEvidence(ev Evidence, err error) *ErrEvidenceInvalid {
	return &ErrEvidenceInvalid{ev, err}
}

// Error returns a string representation of the error.
func (err *ErrEvidenceInvalid) Error() string {
	return fmt.Sprintf("Invalid evidence: %v. Evidence: %v", err.Reason, err.Evidence)
}

// Part set error
var (
	ErrPartSetUnexpectedIndex = errors.New("error part set unexpected index")
	ErrPartSetInvalidProof    = errors.New("error part set invalid proof")
)

// Validator

// ErrTotalVotingPowerOverflow is returned if the total voting power of the
// resulting validator set exceeds MaxTotalVotingPower.
var ErrTotalVotingPowerOverflow = fmt.Errorf("total voting power of resulting valset exceeds max %d", MaxTotalVotingPower)

//-----------------

// IsErrNotEnoughVotingPowerSigned returns true if err is
// ErrNotEnoughVotingPowerSigned.
func IsErrNotEnoughVotingPowerSigned(err error) bool {
	return errors.As(err, &ErrNotEnoughVotingPowerSigned{})
}

// ErrNotEnoughVotingPowerSigned is returned when not enough validators signed
// a commit.
type ErrNotEnoughVotingPowerSigned struct {
	Got    int64
	Needed int64
}

func (e ErrNotEnoughVotingPowerSigned) Error() string {
	return fmt.Sprintf("invalid commit -- insufficient voting power: got %d, needed more than %d", e.Got, e.Needed)
}

// Vote

var (
	ErrVoteUnexpectedStep            = errors.New("unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("invalid validator address")
	ErrVoteInvalidSignature          = errors.New("invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("non-deterministic signature")
	ErrVoteNil                       = errors.New("nil vote")
	ErrZeroHeight                    = errors.New("error zero height")
)

type ErrVoteConflictingVotes struct {
	*DuplicateVoteEvidence
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("Conflicting votes from validator %v", "val addr")
}

// NewConflictingVoteError ...
func NewConflictingVoteError(voteA, voteB *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		&DuplicateVoteEvidence{
			VoteA: voteA,
			VoteB: voteB,
		},
	}
}

// Validator set error
var (
	ErrNilValidatorSet = errors.New("nil validator set")
	ErrNilCommit       = errors.New("nil commit")
)
