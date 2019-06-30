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
	"crypto/elliptic"
	"crypto/rand"
	"sort"
	"testing"
)

func TestGetProposerUniformVotingPower(t *testing.T) {
	val1 := NewValidator(generatePublicKey(), 1)
	val2 := NewValidator(generatePublicKey(), 1)
	val3 := NewValidator(generatePublicKey(), 1)
	vals := [...]*Validator{val1, val2, val3}
	sort.Sort(ValidatorsByAddress(vals[:]))
	valSet := NewValidatorSet(vals[:], 0, 1000000)
	var proposer *Validator

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[0].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[0])
	}

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[1].Address) {
		t.Errorf("Wrong proposer is selected. Get \n%x, and expect \n%x", proposer, vals[1])
	}

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[2].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[2])
	}

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[0].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[0])
	}
}

func TestGetProposerMixedVotingPower(t *testing.T) {
	val1 := NewValidator(generatePublicKey(), 1)
	val2 := NewValidator(generatePublicKey(), 2)
	val3 := NewValidator(generatePublicKey(), 4)
	vals := [...]*Validator{val1, val2, val3}
	valSet := NewValidatorSet(vals[:], 0, 1000000)
	var proposer *Validator

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[2].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[2])
	}

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[1].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[1])
	}

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[2].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[2])
	}

	valSet.AdvanceProposer(1)
	proposer = valSet.GetProposer()
	if !proposer.Address.Equal(vals[0].Address) {
		t.Errorf("Wrong proposer is selected. Get %v, and expect %v", proposer, vals[0])
	}
}

func generatePublicKey() ecdsa.PublicKey {
	p256 := elliptic.P256()
	priv, _ := ecdsa.GenerateKey(p256, rand.Reader)
	return priv.PublicKey
}
