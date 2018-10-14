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

package types

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/rlp"
)

var (
	ErrVoteUnexpectedStep            = errors.New("Unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("Invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("Invalid validator address")
	ErrVoteInvalidSignature          = errors.New("Invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("Invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("Non-deterministic signature")
	ErrVoteNil                       = errors.New("Nil vote")
)

type ErrVoteConflictingVotes struct {
	*DuplicateVoteEvidence
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("Conflicting votes from validator %v", crypto.PubkeyToAddress(err.PubKey))
}

func NewConflictingVoteError(val *Validator, voteA, voteB *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		&DuplicateVoteEvidence{
			PubKey: val.PubKey,
			VoteA:  voteA,
			VoteB:  voteB,
		},
	}
}

// Types of votes
// TODO Make a new type "VoteType"
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
)

func IsVoteTypeValid(type_ byte) bool {
	switch type_ {
	case VoteTypePrevote:
		return true
	case VoteTypePrecommit:
		return true
	default:
		return false
	}
}

func GetReadableVoteTypeString(type_ byte) string {
	var typeString string
	switch type_ {
	case VoteTypePrevote:
		typeString = "Prevote"
	case VoteTypePrecommit:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return typeString
}

// Represents a prevote, precommit, or commit vote from validators for consensus.
type Vote struct {
	ValidatorAddress cmn.Address `json:"validator_address"`
	ValidatorIndex   *cmn.BigInt `json:"validator_index"`
	Height           *cmn.BigInt `json:"height"`
	Round            *cmn.BigInt `json:"round"`
	Timestamp        *big.Int    `json:"timestamp"` // TODO(thientn/namdoh): epoch seconds, change to milis.
	Type             byte        `json:"type"`
	BlockID          BlockID     `json:"block_id"` // zero if vote is nil.
	Signature        []byte      `json:"signature"`
}

func CreateEmptyVote() *Vote {
	return &Vote{
		ValidatorIndex: cmn.NewBigInt64(-1),
		Height:         cmn.NewBigInt64(-1),
		Round:          cmn.NewBigInt64(-1),
		Timestamp:      big.NewInt(0),
	}
}

func (vote *Vote) IsEmpty() bool {
	return vote.ValidatorIndex.EqualsInt(-1) && vote.Height.EqualsInt(-1) && vote.Height.EqualsInt(-1) && vote.Timestamp.Int64() == 0
}

func (vote *Vote) SignBytes(chainID string) []byte {
	bz, err := rlp.EncodeToBytes(CreateCanonicalVote(chainID, vote))
	if err != nil {
		panic(err)
	}
	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	voteCopy.ValidatorIndex = vote.ValidatorIndex.Copy()
	voteCopy.Height = vote.Height.Copy()
	voteCopy.Round = vote.Round.Copy()
	voteCopy.Timestamp = big.NewInt(vote.Timestamp.Int64())
	return &voteCopy
}

// StringLong returns a long string representing full info about Vote
func (vote *Vote) StringLong() string {
	if vote == nil {
		return "nil-Vote"
	}
	if vote.IsEmpty() {
		return "empty-Vote"
	}

	return fmt.Sprintf("Vote{%v:%X %v/%v/%v(%v) %X , %v @ %v}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress[:]),
		vote.Height, vote.Round, vote.Type, GetReadableVoteTypeString(vote.Type),
		cmn.Fingerprint(vote.BlockID[:]), vote.Signature,
		time.Unix(vote.Timestamp.Int64(), 0))
}

// String simplifies vote.Signature, array of bytes, to hex and gets the first 14 characters
func (vote *Vote) String() string {
	if vote == nil {
		return "nil-vote"
	}
	if vote.IsEmpty() {
		return "empty-vote"
	}

	return fmt.Sprintf("Vote{%v:%X %v/%v/%v(%v) %v , %X @%v}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress[:]),
		vote.Height, vote.Round, vote.Type, GetReadableVoteTypeString(vote.Type),
		vote.BlockID.FingerPrint(), cmn.Fingerprint(vote.Signature[:]),
		time.Unix(vote.Timestamp.Int64(), 0))
}

// UNSTABLE
// XXX: duplicate of p2p.ID to avoid dependence between packages.
// Perhaps we can have a minimal types package containing this (and other things?)
// that both `types` and `p2p` import ?
type P2PID string
