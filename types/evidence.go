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
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/merkle"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
)

// EvidenceType enum type
type EvidenceType uint8

// EvidenceType
const (
	EvidenceDuplicateVote       = EvidenceType(0x01)
	EvidenceMock                = EvidenceType(0x02)
	MaxEvidenceBytesDenominator = 10
	// MaxEvidenceBytes is a maximum size of any evidence
	MaxEvidenceBytes int64 = 484
)

// MaxEvidencePerBlock returns the maximum number of evidences
// allowed in the block and their maximum total size (limitted to 1/10th
// of the maximum block size).
// TODO: change to a constant, or to a fraction of the validator set size.
func MaxEvidencePerBlock(blockMaxBytes int64) (int64, int64) {
	maxBytes := blockMaxBytes / MaxEvidenceBytesDenominator
	maxNum := maxBytes / MaxEvidenceBytes
	return maxNum, maxBytes
}

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() uint64    // height of the equivocation
	Bytes() []byte     // bytes which comprise the evidence
	Hash() common.Hash // hash of the evidence
	ValidateBasic() error
	String() string
}

//-------------------------------------------
//------------------------------------------ PROTO --------------------------------------

// EvidenceToProto is a generalized function for encoding evidence that conforms to the
// evidence interface to protobuf
func EvidenceToProto(evidence Evidence) (*kproto.Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.(type) {
	case *DuplicateVoteEvidence:
		pbev := evi.ToProto()
		return &kproto.Evidence{
			Sum: &kproto.Evidence_DuplicateVoteEvidence{
				DuplicateVoteEvidence: pbev,
			},
		}, nil

	default:
		return nil, fmt.Errorf("toproto: evidence is not recognized: %T", evi)
	}
}

// EvidenceFromProto is a generalized function for decoding protobuf into the
// evidence interface
func EvidenceFromProto(evidence *kproto.Evidence) (Evidence, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	switch evi := evidence.Sum.(type) {
	case *kproto.Evidence_DuplicateVoteEvidence:
		return DuplicateVoteEvidenceFromProto(evi.DuplicateVoteEvidence)
	default:
		return nil, errors.New("evidence is not recognized")
	}
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	VoteA *Vote
	VoteB *Vote
}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(vote1 *Vote, vote2 *Vote) *DuplicateVoteEvidence {
	var voteA, voteB *Vote
	if vote1 == nil || vote2 == nil {
		return nil
	}
	if strings.Compare(vote1.BlockID.Key(), vote2.BlockID.Key()) == -1 {
		voteA = vote1
		voteB = vote2
	} else {
		voteA = vote2
		voteB = vote1
	}
	return &DuplicateVoteEvidence{
		VoteA: voteA,
		VoteB: voteB,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("VoteA: %v; VoteB: %v", dve.VoteA, dve.VoteB)

}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() uint64 {
	return dve.VoteA.Height
}

// Time return the time the evidence was created
func (dve *DuplicateVoteEvidence) Time() time.Time {
	return dve.VoteA.Timestamp
}

// Equal checks if two pieces of evidence are equal.
func (dve *DuplicateVoteEvidence) Equal(ev Evidence) bool {
	if _, ok := ev.(*DuplicateVoteEvidence); !ok {
		return false
	}

	// just check their hashes
	dveHash := dve.Hash()
	evHash := ev.Hash()
	return bytes.Equal(dveHash.Bytes(), evHash.Bytes())
}

// Bytes returns the proto-encoded evidence as a byte array.
func (dve *DuplicateVoteEvidence) Bytes() []byte {
	pbe := dve.ToProto()
	bz, err := pbe.Marshal()
	if err != nil {
		panic(err)
	}

	return bz
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() common.Hash {
	return hash(dve.Bytes())
}

// Verify returns an error if the two votes aren't conflicting.
// To be conflicting, they must be from the same validator, for the same H/R/S, but for different blocks.
func (dve *DuplicateVoteEvidence) Verify(chainID string, addr common.Address) error {
	// H/R/S must be the same
	if dve.VoteA.Height != dve.VoteB.Height ||
		dve.VoteA.Round != dve.VoteB.Round ||
		dve.VoteA.Type != dve.VoteB.Type {
		return fmt.Errorf("duplicateVoteEvidence Error: H/R/S does not match. Got %v and %v", dve.VoteA, dve.VoteB)
	}

	// Address must be the same
	if !bytes.Equal(dve.VoteA.ValidatorAddress.Bytes(), dve.VoteB.ValidatorAddress.Bytes()) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator addresses do not match. Got %X and %X",
			dve.VoteA.ValidatorAddress,
			dve.VoteB.ValidatorAddress,
		)
	}

	// Index must be the same
	if dve.VoteA.ValidatorIndex != dve.VoteB.ValidatorIndex {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: Validator indices do not match. Got %d and %d",
			dve.VoteA.ValidatorIndex,
			dve.VoteB.ValidatorIndex,
		)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equal(dve.VoteB.BlockID) {
		return fmt.Errorf(
			"duplicateVoteEvidence Error: BlockIDs are the same (%v) - not a real duplicate vote",
			dve.VoteA.BlockID,
		)
	}

	// pubkey must match address (this should already be true, sanity check)
	vaddr := dve.VoteA.ValidatorAddress
	if !bytes.Equal(vaddr.Bytes(), addr.Bytes()) {
		return fmt.Errorf("duplicateVoteEvidence FAILED SANITY CHECK - address doesn't match validator addr (%v - %X)",
			vaddr, addr)
	}
	signBytes := VoteSignBytes(chainID, dve.VoteA.ToProto())
	if !VerifySignature(addr, crypto.Keccak256(signBytes), dve.VoteA.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteA: %v", ErrVoteInvalidSignature)
	}
	signBytes = VoteSignBytes(chainID, dve.VoteB.ToProto())
	if !VerifySignature(addr, crypto.Keccak256(signBytes), dve.VoteB.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteB: %v", ErrVoteInvalidSignature)
	}

	return nil
}

// ValidateBasic performs basic validation.
func (dve *DuplicateVoteEvidence) ValidateBasic() error {
	if dve.VoteA == nil || dve.VoteB == nil {
		return fmt.Errorf("one or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	}
	if err := dve.VoteA.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteA: %v", err)
	}
	if err := dve.VoteB.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid VoteB: %v", err)
	}
	// Enforce Votes are lexicographically sorted on blockID
	if strings.Compare(dve.VoteA.BlockID.Key(), dve.VoteB.BlockID.Key()) >= 0 {
		return errors.New("duplicate votes in invalid order")
	}
	return nil
}

// ToProto encodes DuplicateVoteEvidence to protobuf
func (dve *DuplicateVoteEvidence) ToProto() *kproto.DuplicateVoteEvidence {
	voteB := dve.VoteB.ToProto()
	voteA := dve.VoteA.ToProto()
	tp := kproto.DuplicateVoteEvidence{
		VoteA: voteA,
		VoteB: voteB,
	}
	return &tp
}

// DuplicateVoteEvidenceFromProto decodes protobuf into DuplicateVoteEvidence
func DuplicateVoteEvidenceFromProto(pb *kproto.DuplicateVoteEvidence) (*DuplicateVoteEvidence, error) {
	if pb == nil {
		return nil, errors.New("nil duplicate vote evidence")
	}

	vA, err := VoteFromProto(pb.VoteA)
	if err != nil {
		return nil, err
	}

	vB, err := VoteFromProto(pb.VoteB)
	if err != nil {
		return nil, err
	}

	dve := NewDuplicateVoteEvidence(vA, vB)

	return dve, dve.ValidateBasic()
}

//-------------------------------------------- MOCKING --------------------------------------

// unstable - use only for testing

func makeMockVote(height uint64, round, index uint32, addr common.Address,
	blockID BlockID, time time.Time) *Vote {
	return &Vote{
		Type:             kproto.SignedMsgType(2),
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        time,
		ValidatorAddress: addr,
		ValidatorIndex:   index,
	}
}

func createBlockIDRandom() BlockID {
	return BlockID{
		Hash: common.BytesToHash(common.RandBytes(32)),
		PartsHeader: PartSetHeader{
			Total: 1,
			Hash:  common.BytesToHash(common.RandBytes(32)),
		},
	}
}

// assumes the round to be 0 and the validator index to be 0
func NewMockDuplicateVoteEvidence(height uint64, time time.Time, chainID string) *DuplicateVoteEvidence {
	val := NewMockPV()
	return NewMockDuplicateVoteEvidenceWithValidator(height, time, val, chainID)
}

func NewMockDuplicateVoteEvidenceWithValidator(height uint64, time time.Time, pv PrivValidator, chainID string) *DuplicateVoteEvidence {
	addr := pv.GetAddress()
	voteA := makeMockVote(height, 0, 0, addr, createBlockIDRandom(), time)
	vA := voteA.ToProto()
	_ = pv.SignVote(chainID, vA)
	voteA.Signature = vA.Signature
	voteB := makeMockVote(height, 0, 0, addr, createBlockIDRandom(), time)
	vB := voteB.ToProto()
	_ = pv.SignVote(chainID, vB)
	voteB.Signature = vB.Signature
	return NewDuplicateVoteEvidence(voteA, voteB)
}

//-------------------------------------------

// EvidenceList is a list of Evidence. Evidences is not a word.
type EvidenceList []Evidence

// Hash returns the simple merkle root hash of the EvidenceList.
func (evl EvidenceList) Hash() common.Hash {
	if len(evl) == 0 {
		return common.NewZeroHash()
	}

	bze := make([][]byte, len(evl))
	for i, ev := range evl {
		bze[i] = ev.Hash().Bytes()
	}
	proof := merkle.SimpleHashFromByteSlices(bze)
	return common.BytesToHash(proof)
}

func (evl EvidenceList) String() string {
	s := ""
	for _, e := range evl {
		s += fmt.Sprintf("%s\t\t", e)
	}
	return s
}

// Has returns true if the evidence is in the EvidenceList.
func (evl EvidenceList) Has(evidence Evidence) bool {
	for _, ev := range evl {
		if evidence.Hash().Equal(ev.Hash()) {
			return true
		}
	}
	return false
}
