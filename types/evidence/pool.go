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

package evidence

import (
	"fmt"
	"sync"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/lib/clist"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

// Pool maintains a pool of valid evidence
// in an EvidenceStore.
type Pool struct {
	logger log.Logger

	evidenceStore kaidb.Database

	evidenceList *clist.CList // concurrent linked-list of evidence

	// latest state
	state state.LastestBlockState

	// needed to load validators to verify evidence
	stateDB StateStore
	// needed to load headers to verify evidence
	blockStore BlockStore

	mtx sync.Mutex

	// This is the closest height where at one or more of the current trial periods
	// will have ended and we will need to then upgrade the evidence to amnesia evidence.
	// It is set to -1 when we don't have any evidence on trial.
	nextEvidenceTrialEndedHeight int64
}

// NewPool creates an evidence pool. If using an existing evidence store,
// it will add all pending evidence to the concurrent list.
func NewPool(evidenceDB kaidb.Database, stateDB StateStore, blockStore BlockStore) (*Pool, error) {
	state := stateDB.LoadState()
	pool := &Pool{
		blockStore:                   blockStore,
		stateDB:                      stateDB,
		state:                        state,
		logger:                       log.New(),
		evidenceStore:                evidenceDB,
		nextEvidenceTrialEndedHeight: -1,
	}
	return pool, nil
}

// PendingEvidence is used primarily as part of block proposal and returns up to maxNum of uncommitted evidence.
// If maxNum is -1, all evidence is returned. Pending evidence is prioritized based on time.
func (evpool *Pool) PendingEvidence(maxNum uint32) []types.Evidence {
	return nil
}

// listEvidence lists up to maxNum pieces of evidence for the given prefix key.
// If maxNum is -1, there's no cap on the size of returned evidence.
func (evpool *Pool) listEvidence(prefixKey byte, maxNum int64) ([]types.Evidence, error) {
	var count int64
	var evidence []types.Evidence
	iter := evpool.evidenceStore.NewIteratorWithPrefix([]byte{prefixKey})
	for iter.Next() {
		if count == maxNum {
			return evidence, nil
		}
		count++
		val := iter.Value()
		ev, err := types.EvidenceFromBytes(val)
		if err != nil {
			return nil, err
		}
		evidence = append(evidence, ev)
	}
	return evidence, nil
}

func (evpool *Pool) addPendingEvidence(evidence types.Evidence) error {
	ev, err := types.EvidenceToBytes(evidence)
	if err != nil {
		return fmt.Errorf("unable to encode evidence: %s", err)
	}
	key := keyPending(evidence)
	return evpool.evidenceStore.Put(key, ev)
}

func (evpool *Pool) removePendingEvidence(evidence types.Evidence) {
	key := keyPending(evidence)
	if err := evpool.evidenceStore.Delete(key); err != nil {
		evpool.logger.Error("unable to delete pending evidence", "err", err)
	} else {
		evpool.logger.Info("Deleted pending evidence", "evidence", evidence)
	}
}
