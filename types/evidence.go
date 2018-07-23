package types

import (
	"github.com/kardiachain/go-kardia/crypto"
)

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Height() int64                                     // height of the equivocation
	Address() []byte                                   // address of the equivocating validator
	Hash() []byte                                      // hash of the evidence
	Verify(chainID string, pubKey crypto.PubKey) error // verify the evidence
	Equal(Evidence) bool                               // check equality of evidence

	String() string
}

// ---------- EvidenceStore -----------
// EvidenceStore is a store of all the evidence we've seen, including
// evidence that has been committed, evidence that has been verified but not broadcast,
// and evidence that has been broadcast but not yet committed.
type EvidenceStore struct {
	// TODO(namdoh): Switch to use permanent storage
	pendEvidences []Evidence
}

// PendingEvidence returns all known uncommitted evidence.
func (store *EvidenceStore) PendingEvidence() []Evidence {
	return store.pendEvidences
}