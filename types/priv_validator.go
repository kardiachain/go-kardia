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
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
)

// IPrivValidator defines the functionality of a local KAI validator
// that signs votes and proposals, and never double signs.
type IPrivValidator interface {
	// TODO: Extend the interface to return errors too.
	// Ref: https://github.com/tendermint/tendermint/issues/3602
	GetPubKey() ecdsa.PublicKey

	SignVote(chainID string, vote *Vote) error
	SignProposal(chainID string, proposal *Proposal) error
}

// PrivValidator defines the functionality of a local Kardia validator
// that signs votes, proposals, and heartbeats, and never double signs.
type PrivValidator struct {
	privKey *ecdsa.PrivateKey
}

// NewPrivValidator ...
func NewPrivValidator(privKey *ecdsa.PrivateKey) *PrivValidator {
	return &PrivValidator{
		privKey: privKey,
	}
}

// GetAddress ...
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

// SignVote Implements PrivValidator.
func (pv *MockPV) SignVote(chainID string, vote *Vote) error {
	if pv.breakVoteSigning {
		chainID = "1"
	}
	sig, err := crypto.Sign(crypto.Keccak256(vote.SignBytes(chainID)), pv.privKey)
	if err != nil {
		return err
	}
	vote.Signature = sig
	return nil
}

// SignProposal Implements PrivValidator.
func (pv *MockPV) SignProposal(chainID string, proposal *Proposal) error {
	if pv.breakProposalSigning {
		chainID = "1000"
	}
	sig, err := crypto.Sign(crypto.Keccak256(proposal.SignBytes(chainID)), pv.privKey)
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
