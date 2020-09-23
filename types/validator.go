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
	"math/rand"
	"strings"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
)

// ErrTotalVotingPowerOverflow is returned if the total voting power of the
// resulting validator set exceeds MaxTotalVotingPower.
var ErrTotalVotingPowerOverflow = fmt.Errorf("total voting power of resulting valset exceeds max %d",
	MaxTotalVotingPower)

//-----------------

// IsErrNotEnoughVotingPowerSigned returns true if err is
// ErrNotEnoughVotingPowerSigned.
func IsErrNotEnoughVotingPowerSigned(err error) bool {
	return errors.As(err, &ErrNotEnoughVotingPowerSigned{})
}

// ErrNotEnoughVotingPowerSigned is returned when not enough validators signed
// a commit.
type ErrNotEnoughVotingPowerSigned struct {
	Got    uint64
	Needed uint64
}

func (e ErrNotEnoughVotingPowerSigned) Error() string {
	return fmt.Sprintf("invalid commit -- insufficient voting power: got %d, needed more than %d", e.Got, e.Needed)
}

// Validator state for each Validator
type Validator struct {
	Address          common.Address `json:"address"`
	VotingPower      uint64         `json:"voting_power"`
	ProposerPriority *common.BigInt `json:"accum"`
}

// NewValidator ...
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

// Copy Creates a new copy of the validator.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

// CompareProposerPriority Returns the one with higher ProposerPriority.
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
		v.Address.String(),
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
func RandValidator(randPower bool, minPower uint64) (*Validator, PrivValidator) {
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += uint64(rand.Uint32())
	}
	pubKey := privVal.GetPubKey()
	val := NewValidator(crypto.PubkeyToAddress(pubKey), votePower)
	return val, privVal
}

func RandValidatorCS(randPower bool, minPower uint64) (*Validator, *DefaultPrivValidator) {
	privKey, _ := crypto.GenerateKey()
	votePower := minPower
	if randPower {
		votePower += uint64(rand.Uint32())
	}
	privVal := NewDefaultPrivValidator(privKey)
	val := NewValidator(privVal.GetAddress(), votePower)
	return val, privVal
}

// GetProposerPriority ...
func (v *Validator) GetProposerPriority() *common.BigInt {
	if v != nil {
		return v.ProposerPriority
	}
	return common.NewBigInt(0)
}
