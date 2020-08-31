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

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/rlp"
)

// EvidenceType enum type
type EvidenceType uint8

// EvidenceType
const (
	EvidenceDuplicateVote = EvidenceType(0x01)
)

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int64                                    // height of the equivocation
	Time() int64                                      // time of the equivocation
	Address() common.Address                          // address of the equivocating validator
	Bytes() []byte                                    // bytes which comprise the evidence
	Hash() common.Hash                                // hash of the evidence
	Verify(chainID string, addr common.Address) error // verify the evidence
	Equal(Evidence) bool                              // check equality of evidence

	ValidateBasic() error
	String() string
}

//-------------------------------------------

// EvidenceInfo ...
type EvidenceInfo struct {
	Type    EvidenceType
	Payload []byte
}

// EvidenceToBytes ...
func EvidenceToBytes(evidence Evidence) ([]byte, error) {
	if evidence == nil {
		return nil, errors.New("nil evidence")
	}

	info := &EvidenceInfo{}

	switch evi := evidence.(type) {
	case *DuplicateVoteEvidence:
		info.Type = EvidenceDuplicateVote
		b, err := rlp.EncodeToBytes(evi)
		if err != nil {
			return nil, err
		}
		info.Payload = b
		break
	default:
		return nil, fmt.Errorf("evidence is not recognized: %T", evidence)
	}

	return rlp.EncodeToBytes(info)
}

// EvidenceFromBytes ...
func EvidenceFromBytes(b []byte) (Evidence, error) {
	info := &EvidenceInfo{}

	if err := rlp.DecodeBytes(b, info); err != nil {
		return nil, err
	}

	switch info.Type {
	case EvidenceDuplicateVote:
		duplicateVoteEvidence := &DuplicateVoteEvidence{}
		if err := rlp.DecodeBytes(info.Payload, duplicateVoteEvidence); err != nil {
			return nil, err
		}
		return duplicateVoteEvidence, nil
	default:
		return nil, errors.New("evidence is not recognized")
	}
}

//-------------------------------------------

// DuplicateVoteEvidence contains evidence a validator signed two conflicting
// votes.
type DuplicateVoteEvidence struct {
	Addr  common.Address
	VoteA *Vote
	VoteB *Vote
}

// NewDuplicateVoteEvidence creates DuplicateVoteEvidence with right ordering given
// two conflicting votes. If one of the votes is nil, evidence returned is nil as well
func NewDuplicateVoteEvidence(addr common.Address, vote1 *Vote, vote2 *Vote) *DuplicateVoteEvidence {
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
		Addr:  addr,
		VoteA: voteA,
		VoteB: voteB,
	}
}

// String returns a string representation of the evidence.
func (dve *DuplicateVoteEvidence) String() string {
	return fmt.Sprintf("VoteA: %v; VoteB: %v", dve.VoteA, dve.VoteB)

}

// Height returns the height this evidence refers to.
func (dve *DuplicateVoteEvidence) Height() int64 {
	return dve.VoteA.Height.Int64()
}

// Time return the time the evidence was created
func (dve *DuplicateVoteEvidence) Time() int64 {
	return dve.VoteA.Timestamp.Int64()
}

// Address returns the address of the validator.
func (dve *DuplicateVoteEvidence) Address() common.Address {
	return dve.Addr
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

// Bytes Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Bytes() []byte {
	b, _ := rlp.EncodeToBytes(dve)
	return b
}

// Hash returns the hash of the evidence.
func (dve *DuplicateVoteEvidence) Hash() common.Hash {
	return rlpHash(dve.Bytes())
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

	if !VerifySignature(addr, crypto.Keccak256(dve.VoteA.SignBytes(chainID)), dve.VoteA.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteA: %v", ErrVoteInvalidSignature)
	}

	if !VerifySignature(addr, crypto.Keccak256(dve.VoteB.SignBytes(chainID)), dve.VoteB.Signature) {
		return fmt.Errorf("duplicateVoteEvidence Error verifying VoteB: %v", ErrVoteInvalidSignature)
	}

	return nil
}

// ValidateBasic performs basic validation.
func (dve *DuplicateVoteEvidence) ValidateBasic() error {
	// if dve.VoteA == nil || dve.VoteB == nil {
	// 	return fmt.Errorf("one or both of the votes are empty %v, %v", dve.VoteA, dve.VoteB)
	// }
	// if err := dve.VoteA.ValidateBasic(); err != nil {
	// 	return fmt.Errorf("invalid VoteA: %v", err)
	// }
	// if err := dve.VoteB.ValidateBasic(); err != nil {
	// 	return fmt.Errorf("invalid VoteB: %v", err)
	// }
	// // Enforce Votes are lexicographically sorted on blockID
	// if strings.Compare(dve.VoteA.BlockID.Key(), dve.VoteB.BlockID.Key()) >= 0 {
	// 	return errors.New("duplicate votes in invalid order")
	// }
	return nil
}

//-------------------------------------------

// EvidenceList is a list of Evidence. Evidences is not a word.
type EvidenceList []Evidence

// Hash returns the simple merkle root hash of the EvidenceList.
func (evl EvidenceList) Hash() common.Hash {
	return DeriveSha(evl)
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
		if ev.Equal(evidence) {
			return true
		}
	}
	return false
}

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (evl EvidenceList) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(evl[i])
	return enc
}

// Len ...
func (evl EvidenceList) Len() int {
	return len(evl)
}
