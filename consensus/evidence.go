package evidence

import (
	"fmt"
	"sync"

	"github.com/kardiachain/go-kardia/log"
)

// EvidencePool maintains a pool of valid evidence
// in an EvidenceStore.
type EvidencePool struct {
	logger log.Logger

	evidenceStore *EvidenceStore
	// TODO(namdoh): Adds list of evidences.

	// latest state
	mtx   sync.Mutex
	state state.LastestBlockState
}

// TODO(namdoh): Implememnts EvidenceStore
