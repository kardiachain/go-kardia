/*
 *  Copyright 2019 KardiaChain
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
	"math/big"
	"math/rand"
	"testing"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidatorSetValidateBasic(t *testing.T) {
	val, _ := RandValidator(false, 1)

	testCases := []struct {
		vals ValidatorSet
		err  bool
		msg  string
	}{
		{
			vals: ValidatorSet{},
			err:  true,
			msg:  "validator set is nil or empty",
		},
		{
			vals: ValidatorSet{
				Validators: []*Validator{},
			},
			err: true,
			msg: "validator set is nil or empty",
		},
		{
			vals: ValidatorSet{
				Validators: []*Validator{val},
			},
			err: true,
			msg: "proposer failed validate basic, error: nil validator",
		},
		{
			vals: ValidatorSet{
				Validators: []*Validator{val},
				Proposer:   val,
			},
			err: false,
			msg: "",
		},
	}

	for _, tc := range testCases {
		err := tc.vals.ValidateBasic()
		if tc.err {
			if assert.Error(t, err) {
				assert.Equal(t, tc.msg, err.Error())
			}
		} else {
			assert.NoError(t, err)
		}
	}

}

func randValidator(totalVotingPower uint64) *Validator {
	// this modulo limits the ProposerPriority/VotingPower to stay in the
	// bounds of MaxTotalVotingPower minus the already existing voting power:
	val := NewValidator(generateAddress(), uint64(rand.Uint64()%uint64(MaxTotalVotingPower-totalVotingPower)))
	proposerPriority := rand.Uint64() % (MaxTotalVotingPower - totalVotingPower)
	val.ProposerPriority = big.NewInt(int64(proposerPriority))
	return val
}

func randValidatorSet(numValidators int) *ValidatorSet {
	validators := make([]*Validator, numValidators)
	totalVotingPower := uint64(0)
	for i := 0; i < numValidators; i++ {
		validators[i] = randValidator(totalVotingPower)
		totalVotingPower += validators[i].VotingPower
	}
	return NewValidatorSet(validators)
}

func TestCopy(t *testing.T) {
	vset := randValidatorSet(10)
	vsetHash := vset.Hash()
	if len(vsetHash) == 0 {
		t.Fatalf("ValidatorSet had unexpected zero hash")
	}

	vsetCopy := vset.Copy()
	vsetCopyHash := vsetCopy.Hash()

	if !vsetHash.Equal(vsetCopyHash) {
		t.Fatalf("ValidatorSet copy had wrong hash. Orig: %X, Copy: %X", vsetHash, vsetCopyHash)
	}
}

// Test that IncrementProposerPriority requires positive times.
func TestIncrementProposerPriorityPositiveTimes(t *testing.T) {
	vset := NewValidatorSet([]*Validator{
		newValidator(generateAddress(), 1000),
		newValidator(generateAddress(), 300),
		newValidator(generateAddress(), 330),
	})

	assert.Panics(t, func() { vset.IncrementProposerPriority(-1) })
	assert.Panics(t, func() { vset.IncrementProposerPriority(0) })
	vset.IncrementProposerPriority(1)
}

func newValidator(address common.Address, power uint64) *Validator {
	return &Validator{Address: address, VotingPower: power}
}

func generateAddress() common.Address {
	privateKey, _ := crypto.GenerateKey()
	publicKey := privateKey.Public()
	publicKeyECDSA, _ := publicKey.(*ecdsa.PublicKey)

	return crypto.PubkeyToAddress(*publicKeyECDSA)
}
