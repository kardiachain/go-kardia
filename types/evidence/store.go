package evidence

import (
	"github.com/kardiachain/go-kardia/types"
)

// EvidenceStore is a store of all the evidence we've seen, including
// evidence that has been committed, evidence that has been verified but not broadcast,
// and evidence that has been broadcast but not yet committed.
type EvidenceStore struct {
	// TODO(namdoh): Switch to use permanent storage
	pendEvidences []types.Evidence
}

// PendingEvidence returns all known uncommitted evidence.
func (store *EvidenceStore) PendingEvidence() []types.Evidence {
	return store.pendEvidences
}
