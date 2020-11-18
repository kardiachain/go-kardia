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
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"
	cmn "github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/protoio"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

// IsVoteTypeValid returns true if t is a valid vote type.
func IsVoteTypeValid(t kproto.SignedMsgType) bool {
	switch t {
	case kproto.PrevoteType, kproto.PrecommitType:
		return true
	default:
		return false
	}
}
func GetReadableVoteTypeString(type_ kproto.SignedMsgType) string {
	var typeString string
	switch type_ {
	case kproto.PrevoteType:
		typeString = "Prevote"
	case kproto.PrecommitType:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return typeString
}

// Vote Represents a prevote, precommit, or commit vote from validators for consensus.
type Vote struct {
	ValidatorAddress cmn.Address          `json:"validator_address"`
	ValidatorIndex   uint32               `json:"validator_index"`
	Height           uint64               `json:"height"`
	Round            uint32               `json:"round"`
	Timestamp        time.Time            `json:"timestamp"`
	Type             kproto.SignedMsgType `json:"type"`
	BlockID          BlockID              `json:"block_id"` // zero if vote is nil.
	Signature        []byte               `json:"signature"`
}

// CreateEmptyVote ...
func CreateEmptyVote() *Vote {
	return &Vote{}
}

// CommitSig converts the Vote to a CommitSig.
func (vote *Vote) CommitSig() CommitSig {
	if vote == nil {
		return NewCommitSigAbsent()
	}

	var blockIDFlag BlockIDFlag
	switch {
	case vote.BlockID.IsComplete():
		blockIDFlag = BlockIDFlagCommit
	case vote.BlockID.IsZero():
		blockIDFlag = BlockIDFlagNil
	default:
		panic(fmt.Sprintf("Invalid vote %v - expected BlockID to be either empty or complete", vote))
	}

	return CommitSig{
		BlockIDFlag:      blockIDFlag,
		ValidatorAddress: vote.ValidatorAddress,
		Timestamp:        vote.Timestamp,
		Signature:        vote.Signature,
	}
}

// VoteSignBytes returns the proto-encoding of the canonicalized Vote, for
// signing. Panics is the marshaling fails.
//
// The encoded Protobuf message is varint length-prefixed (using MarshalDelimited)
// for backwards-compatibility with the Amino encoding, due to e.g. hardware
// devices that rely on this encoding.
//
// See CanonicalizeVote
func VoteSignBytes(chainID string, vote *kproto.Vote) []byte {
	pb := CreateCanonicalVote(chainID, vote)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	voteCopy.ValidatorIndex = vote.ValidatorIndex
	voteCopy.Height = vote.Height
	voteCopy.Round = vote.Round
	voteCopy.Timestamp = vote.Timestamp
	return &voteCopy
}

// StringLong returns a long string representing full info about Vote
func (vote *Vote) StringLong() string {
	if vote == nil {
		return "nil-Vote"
	}

	return fmt.Sprintf("Vote{%v:%X %v/%v/%v(%v) %X , %v @ %v}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress[:]),
		vote.Height, vote.Round, vote.Type, GetReadableVoteTypeString(vote.Type),
		vote.BlockID.Hash.Fingerprint(), vote.Signature,
		vote.Timestamp)
}

// String simplifies vote.Signature, array of bytes, to hex and gets the first 14 characters
func (vote *Vote) String() string {
	if vote == nil {
		return "nil-vote"
	}
	return fmt.Sprintf("Vote{%v:%X %v/%v/%v(%v) %v , %X @%v}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress[:]),
		vote.Height, vote.Round, vote.Type, GetReadableVoteTypeString(vote.Type),
		vote.BlockID, cmn.Fingerprint(vote.Signature[:]),
		vote.Timestamp)
}

// Verify ...
func (vote *Vote) Verify(chainID string, address common.Address) error {
	if !vote.ValidatorAddress.Equal(address) {
		return ErrVoteInvalidValidatorAddress
	}
	v := vote.ToProto()
	signBytes := VoteSignBytes(chainID, v)
	if !VerifySignature(address, crypto.Keccak256(signBytes), vote.Signature) {
		return ErrVoteInvalidSignature
	}
	return nil
}

// ValidateBasic performs basic validation.
func (vote *Vote) ValidateBasic() error {
	if !IsVoteTypeValid(vote.Type) {
		return errors.New("invalid Type")
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if err := vote.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// BlockID.ValidateBasic would not err if we for instance have an empty hash but a
	// non-empty PartsSetHeader:
	if !vote.BlockID.IsZero() && !vote.BlockID.IsComplete() {
		return fmt.Errorf("blockID must be either empty or complete, got: %v", vote.BlockID)
	}

	if len(vote.Signature) == 0 {
		return errors.New("signature is missing")
	}
	return nil
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *kproto.Vote {
	if vote == nil {
		return nil
	}

	return &kproto.Vote{
		Type:             vote.Type,
		Height:           vote.Height,
		Round:            vote.Round,
		BlockID:          vote.BlockID.ToProto(),
		Timestamp:        vote.Timestamp,
		ValidatorAddress: vote.ValidatorAddress.Bytes(),
		ValidatorIndex:   vote.ValidatorIndex,
		Signature:        vote.Signature,
	}
}

// FromProto converts a proto generetad type to a handwritten type
// return type, nil if everything converts safely, otherwise nil, error
func VoteFromProto(pv *kproto.Vote) (*Vote, error) {
	if pv == nil {
		return nil, errors.New("nil vote")
	}

	blockID, err := BlockIDFromProto(&pv.BlockID)
	if err != nil {
		return nil, err
	}

	vote := new(Vote)
	vote.Type = pv.Type
	vote.Height = pv.Height
	vote.Round = pv.Round
	vote.BlockID = *blockID
	vote.Timestamp = pv.Timestamp
	vote.ValidatorAddress = common.BytesToAddress(pv.ValidatorAddress)
	vote.ValidatorIndex = pv.ValidatorIndex
	vote.Signature = pv.Signature

	return vote, vote.ValidateBasic()
}

// UNSTABLE
// XXX: duplicate of p2p.ID to avoid dependence between packages.
// Perhaps we can have a minimal types package containing this (and other things?)
// that both `types` and `p2p` import ?
type P2PID string
