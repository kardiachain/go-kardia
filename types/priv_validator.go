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
