package types

import (
	"crypto/ecdsa"
)

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int64                                       // height of the equivocation
	Address() []byte                                     // address of the equivocating validator
	Hash() []byte                                        // hash of the evidence
	Verify(chainID string, pubKey ecdsa.PublicKey) error // verify the evidence
	Equal(Evidence) bool                                 // check equality of evidence

	String() string
}

// DuplicateVoteEvidence contains evidence a validator signed two conflicting votes.
type DuplicateVoteEvidence struct {
	PubKey ecdsa.PublicKey
	VoteA  *Vote
	VoteB  *Vote
}
