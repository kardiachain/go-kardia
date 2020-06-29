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
	"crypto/ecdsa"
	"errors"
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
)

type PrivV interface {
	GetPubKey() ecdsa.PublicKey
	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
}

// PrivValidator defines the functionality of a local Kardia validator
// that signs votes, proposals, and heartbeats, and never double signs.
type PrivValidator struct {
	privKey *ecdsa.PrivateKey
}

func NewPrivValidator(privKey *ecdsa.PrivateKey) *PrivValidator {
	return &PrivValidator{
		privKey: privKey,
	}
}

func (privVal *PrivValidator) GetAddress() common.Address {
	return crypto.PubkeyToAddress(privVal.GetPubKey())
}

func (privVal *PrivValidator) GetPubKey() ecdsa.PublicKey {
	return privVal.privKey.PublicKey
}

func (privVal *PrivValidator) GetPrivKey() *ecdsa.PrivateKey {
	return privVal.privKey
}

func (privVal *PrivValidator) SignVote(chainID string, vote *Vote) error {
	hash := rlpHash(vote.SignBytes(chainID))
	sig, err := crypto.Sign(hash[:], privVal.privKey)
	if err != nil {
		log.Trace("Signing vote failed", "err", err)
		return err
	}
	vote.Signature = sig
	return nil
}

func (privVal *PrivValidator) SignProposal(chainID string, proposal *Proposal) error {
	hash := rlpHash(proposal.SignBytes(chainID))
	sig, err := crypto.Sign(hash[:], privVal.privKey)
	if err != nil {
		log.Trace("Signing proposal failed", "err", err)
		return err
	}
	proposal.Signature = sig
	return nil
}

//func (privVal *PrivValidator) SignHeartbeat(chainID string, heartbeat *Heartbeat) error {
//	panic("SignHeartbeat - not yet implemented")
//}

//----------------------------------------
// Misc.

type PrivValidatorsByAddress []PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(pvs[i].GetAddress().Bytes(), pvs[j].GetAddress().Bytes()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}

//----------------------------------------
// MockPV

// MockPV implements PrivValidator without any safety or persistence.
// Only use it for testing.
type MockPV struct {
	privKey              *ecdsa.PrivateKey
	breakProposalSigning bool
	breakVoteSigning     bool
}

func NewMockPV() *MockPV {
	priv, err := crypto.GenerateKey()

	if err != nil {
		panic(err)
	}

	return &MockPV{priv, false, false}
}

// NewMockPVWithParams allows one to create a MockPV instance, but with finer
// grained control over the operation of the mock validator. This is useful for
// mocking test failures.
func NewMockPVWithParams(privKey *ecdsa.PrivateKey, breakProposalSigning, breakVoteSigning bool) *MockPV {
	return &MockPV{privKey, breakProposalSigning, breakVoteSigning}
}

// Implements PrivValidator.
func (pv *MockPV) GetPubKey() ecdsa.PublicKey {
	return pv.privKey.PublicKey
}

// Implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, vote *Vote) error {
	if pv.breakVoteSigning {
		chainID = "incorrect-chain-id"
	}
	//hash := rlpHash(vote.SignBytes(chainID))
	sig, err := crypto.Sign(rlpHash(vote.SignBytes(chainID)).Bytes(), pv.privKey)
	if err != nil {
		return err
	}
	vote.Signature = sig
	return nil
}

// Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, proposal *Proposal) error {
	if pv.breakProposalSigning {
		chainID = "incorrect-chain-id"
	}
	hash := rlpHash(proposal.SignBytes(chainID))
	sig, err := crypto.Sign(hash.Bytes(), pv.privKey)
	if err != nil {
		return err
	}
	proposal.Signature = sig
	return nil

}

// String returns a string representation of the MockPV.
func (pv *MockPV) String() string {
	addr := crypto.PubkeyToAddress(pv.privKey.PublicKey)
	return fmt.Sprintf("MockPV{%v}", addr)
}

// XXX: Implement.
func (pv *MockPV) DisableChecks() {
	// Currently this does nothing,
	// as MockPV has no safety checks at all.
}

type erroringMockPV struct {
	*MockPV
}

var ErroringMockPVErr = errors.New("erroringMockPV always returns an error")

// Implements PrivValidator.
func (pv *erroringMockPV) SignVote(chainID string, vote *Vote) error {
	return ErroringMockPVErr
}

// Implements PrivValidator.
func (pv *erroringMockPV) SignProposal(chainID string, proposal *Proposal) error {
	return ErroringMockPVErr
}

// NewErroringMockPV returns a MockPV that fails on each signing request. Again, for testing only.
func NewErroringMockPV() *erroringMockPV {
	priv, _ := crypto.GenerateKey()
	return &erroringMockPV{&MockPV{priv, false, false}}
}
