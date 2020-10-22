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
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

// PrivValidator defines the functionality of a local KAI validator
// that signs votes and proposals, and never double signs.
type PrivValidator interface {
	// TODO: Extend the interface to return errors too.
	GetPubKey() ecdsa.PublicKey
	GetAddress() common.Address
	SignVote(chainID string, vote *kproto.Vote) error
	SignProposal(chainID string, proposal *kproto.Proposal) error
}

// PrivValidatorsByAddress ...
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

// DefaultPrivValidator defines the functionality of a local Kardia validator
// that signs votes, proposals, and heartbeats, and never double signs.
type DefaultPrivValidator struct {
	privKey *ecdsa.PrivateKey
}

// NewDefaultPrivValidator ...
func NewDefaultPrivValidator(privKey *ecdsa.PrivateKey) *DefaultPrivValidator {
	return &DefaultPrivValidator{
		privKey: privKey,
	}
}

// GetAddress ...
func (privVal *DefaultPrivValidator) GetAddress() common.Address {
	return crypto.PubkeyToAddress(privVal.GetPubKey())
}

func (privVal *DefaultPrivValidator) GetPubKey() ecdsa.PublicKey {
	return privVal.privKey.PublicKey
}

func (privVal *DefaultPrivValidator) GetPrivKey() *ecdsa.PrivateKey {
	return privVal.privKey
}

func (privVal *DefaultPrivValidator) SignVote(chainID string, vote *kproto.Vote) error {
	signBytes := VoteSignBytes(chainID, vote)
	sig, err := crypto.Sign(crypto.Keccak256(signBytes), privVal.privKey)
	if err != nil {
		log.Trace("Signing vote failed", "err", err)
		return err
	}
	vote.Signature = sig
	return nil
}

func (privVal *DefaultPrivValidator) SignProposal(chainID string, proposal *kproto.Proposal) error {
	signBytes := ProposalSignBytes(chainID, proposal)
	sig, err := crypto.Sign(crypto.Keccak256(signBytes), privVal.privKey)
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
// MockPV

// MockPV implements PrivValidator without any safety or persistence.
// Only use it for testing.
type MockPV struct {
	privKey              *ecdsa.PrivateKey
	breakProposalSigning bool
	breakVoteSigning     bool
}

// NewMockPV new mock priv validator
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

// GetPubKey Implements PrivValidator.
func (pv *MockPV) GetPubKey() ecdsa.PublicKey {
	return pv.privKey.PublicKey
}

// GetAddress ...
func (pv *MockPV) GetAddress() common.Address {
	return crypto.PubkeyToAddress(pv.GetPubKey())
}

// SignVote Implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, vote *kproto.Vote) error {
	if pv.breakVoteSigning {
		chainID = "1"
	}
	voteSignBytes := VoteSignBytes(chainID, vote)
	sig, err := crypto.Sign(crypto.Keccak256(voteSignBytes), pv.privKey)
	if err != nil {
		return err
	}
	vote.Signature = sig
	return nil
}

// SignProposal Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, proposal *kproto.Proposal) error {
	if pv.breakProposalSigning {
		chainID = "1000"
	}
	signBytes := ProposalSignBytes(chainID, proposal)
	sig, err := crypto.Sign(crypto.Keccak256(signBytes), pv.privKey)
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

//DisableChecks ....
func (pv *MockPV) DisableChecks() {
	// Currently this does nothing,
	// as MockPV has no safety checks at all.
}
