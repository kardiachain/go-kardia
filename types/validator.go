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
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
)

// Volatile state for each Validator
type Validator struct {
	Address          common.Address `json:"address"`
	VotingPower      uint64         `json:"voting_power"`
	ProposerPriority *common.BigInt `json:"accum"`
}

func NewValidator(addr common.Address, votingPower uint64) *Validator {
	return &Validator{
		Address:          addr,
		VotingPower:      votingPower,
		ProposerPriority: common.NewBigInt(0),
	}
}

// ValidateBasic performs basic validation.
func (v *Validator) ValidateBasic() error {
	if v == nil {
		return errors.New("nil validator")
	}

	if v.VotingPower < 0 {
		return errors.New("validator has negative voting power")
	}

	if !common.IsHexAddress(v.Address.Hex()) {
		return fmt.Errorf("wrong validator address: %v", v.Address)
	}

	return nil
}

// Hash computes the unique ID of a validator with a given voting power.
func (v *Validator) Hash() common.Hash {
	return rlpHash(v)
}

// Creates a new copy of the validator.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

// Returns the one with higher ProposerPriority.
func (v *Validator) CompareProposerPriority(other *Validator) *Validator {
	if v == nil {
		return other
	}
	switch {
	case v.ProposerPriority.IsGreaterThan(other.ProposerPriority):
		return v
	case v.ProposerPriority.IsLessThan(other.ProposerPriority):
		return other
	default:
		result := v.Address.Equal(other.Address)
		switch {
		case result == false:
			return v
		case result == true:
			return other
		default:
			panic("Cannot compare identical validators")
		}
	}
}

// VerifyProposalSignature ...
func (v *Validator) VerifyProposalSignature(chainID string, proposal *Proposal) bool {
	hash := rlpHash(proposal.SignBytes(chainID))
	return VerifySignature(v.Address, hash[:], proposal.Signature[:])
}

// VerifyVoteSignature ...
func (v *Validator) VerifyVoteSignature(chainID string, vote *Vote) bool {
	hash := rlpHash(vote.SignBytes(chainID))
	return VerifySignature(v.Address, hash[:], vote.Signature[:])
}

// String
// String returns a string representation of String.
//
// 1. address
// 2. public key
// 3. voting power
// 4. proposer priority
func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v VP:%v A:%v}",
		v.Address,
		v.VotingPower,
		v.ProposerPriority)
}

// ValidatorListString returns a prettified validator list for logging purposes.
func ValidatorListString(vals []*Validator) string {
	chunks := make([]string, len(vals))
	for i, val := range vals {
		chunks[i] = fmt.Sprintf("%s:%d", val.Address, val.VotingPower)
	}

	return strings.Join(chunks, ",")
}

//----------------------------------------
// RandValidator

// RandValidator returns a randomized validator, useful for testing.
// UNSTABLE
// EXPOSED FOR TESTING.
func RandValidator(randPower bool, minPower uint64) (*Validator, IPrivValidator) {
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += rand.Uint64()
	}
	pubKey := privVal.GetPubKey()
	val := NewValidator(crypto.PubkeyToAddress(pubKey), votePower)
	return val, privVal
}

// RandValidatorCS returns a randomized validator, useful for testing.
// UNSTABLE
// EXPOSED FOR TESTING.
func RandValidatorCS(randPower bool, minPower uint64) (*Validator, *ecdsa.PrivateKey) {
	privVal, _ := crypto.GenerateKey()
	votePower := minPower
	if randPower {
		votePower += uint64(rand.Intn(1000))
	}
	priv := NewPrivValidator(privVal)
	address := priv.GetAddress()
	val := NewValidator(address, votePower)
	return val, privVal
}
