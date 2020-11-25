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
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

var (
	ErrNilValidator = errors.New("nil Validator")
)

type Delegator struct {
	Address      common.Address `json:"address"`
	StakedAmount *big.Int       `json:"stakedAmount"`
	Reward       *big.Int       `json:"reward"`
}

// Validator state for each Validator
type Validator struct {
	Address          common.Address `json:"address"`
	VotingPower      int64          `json:"votingPower"`
	ProposerPriority int64          `json:"proposerPriority"`
	StakedAmount     *big.Int       `json:"stakedAmount,omitempty"`
	Commission       *big.Int       `json:"commission,omitempty"`
	CommissionRate   *big.Int       `json:"commissionRate,omitempty"`
	MaxRate          *big.Int       `json:"maxRate,omitempty"`
	MaxChangeRate    *big.Int       `json:"maxChangeRate,omitempty"`
	Delegators       []*Delegator   `json:"delegators,omitempty"`
}

// NewValidator ...
func NewValidator(addr common.Address, votingPower int64) *Validator {
	return &Validator{
		Address:          addr,
		VotingPower:      votingPower,
		ProposerPriority: 0,
	}
}

// ValidateBasic performs basic validation.
func (v *Validator) ValidateBasic() error {
	if v == nil {
		return ErrNilValidator
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
func (v *Validator) Copy() *Validator {
	// Return empty object when v nil
	if v == nil {
		return nil
	}
	vCopy := &Validator{
		Address:          v.Address,
		VotingPower:      v.VotingPower,
		ProposerPriority: v.ProposerPriority,
	}
	return vCopy
}

// CompareProposerPriority Returns the one with higher ProposerPriority.
func (v *Validator) CompareProposerPriority(other *Validator) *Validator {
	if v == nil {
		return other
	}
	switch {
	case v.ProposerPriority > other.ProposerPriority:
		return v
	case v.ProposerPriority < other.ProposerPriority:
		return other
	default:
		result := bytes.Compare(v.Address.Bytes(), other.Address.Bytes())
		switch {
		case result < 0:
			return v
		case result > 0:
			return other
		default:
			panic("Cannot compare identical validators")
		}
	}
}

// String impl String interface and return validator object with
// 1. address
// 2. voting power
// 3. proposer priority
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

// FromProto sets a protobuf Validator to the given pointer.
// It returns an error if the public key is invalid.
func ValidatorFromProto(vp *kproto.Validator) (*Validator, error) {
	if vp == nil {
		return nil, ErrNilValidator
	}

	v := new(Validator)
	v.Address = common.BytesToAddress(vp.GetAddress())
	v.VotingPower = vp.GetVotingPower()
	v.ProposerPriority = vp.GetProposerPriority()

	return v, nil
}

//----------------------------------------
// RandValidator

// RandValidator returns a randomized validator, useful for testing.
func RandValidator(randPower bool, minPower int64) (*Validator, PrivValidator) {
	rand.Seed(time.Now().UnixNano())
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += int64(rand.Uint32())
	}
	pubKey := privVal.GetPubKey()
	val := NewValidator(crypto.PubkeyToAddress(pubKey), votePower)
	return val, privVal
}

// RandValidatorCS return a random validator for unit test
func RandValidatorCS(randPower bool, minPower int64) (*Validator, *DefaultPrivValidator) {
	rand.Seed(time.Now().UnixNano())
	privKey, _ := crypto.GenerateKey()
	votePower := minPower
	if randPower {
		votePower += int64(rand.Uint32())
	}
	privVal := NewDefaultPrivValidator(privKey)
	val := NewValidator(privVal.GetAddress(), votePower)
	return val, privVal
}

// GetProposerPriority ...
func (v *Validator) GetProposerPriority() int64 {
	if v != nil {
		return v.ProposerPriority
	}
	return 0
}

// ToProto converts Valiator to protobuf
func (v *Validator) ToProto() (*kproto.Validator, error) {
	if v == nil {
		return nil, ErrNilValidator
	}

	vp := kproto.Validator{
		Address:          v.Address.Bytes(),
		VotingPower:      v.VotingPower,
		ProposerPriority: v.ProposerPriority,
	}

	return &vp, nil
}
