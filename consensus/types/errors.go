// Package types
package types

import (
	"errors"
)

var (
	ErrGotVoteFromUnwantedRound = errors.New("peer has sent a vote that does not match our round for more than one round")
	ErrNilVoteType              = errors.New("voteType is Nil")
)
