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

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/crypto"
	"github.com/kardiachain/go-kardia/lib/log"
)

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
	return pvs[i].GetAddress().Equal(pvs[j].GetAddress())
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	it := pvs[i]
	pvs[i] = pvs[j]
	pvs[j] = it
}
