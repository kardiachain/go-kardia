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
	"time"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"

	"github.com/kardiachain/go-kardiamain/lib/clist"
	"github.com/kardiachain/go-kardiamain/lib/log"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/types"
)

// Pool maintains a pool of valid evidence
// in an Store.
type Pool struct {
	logger log.Logger

	store        *Store
	evidenceList *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	stateDB kaidb.Database

	// latest state
	mtx   sync.Mutex
	state cstate.LastestBlockState
}

// NewPool creates an evidence pool. If using an existing evidence store,
// it will add all pending evidence to the concurrent list.
func NewPool(stateDB, evidenceDB kaidb.Database) *Pool {
	store := NewStore(evidenceDB)
	evpool := &Pool{
		stateDB:      stateDB,
		state:        cstate.LoadState(stateDB),
		logger:       log.New(),
		store:        store,
		evidenceList: clist.New(),
	}
	return evpool
}

// IsCommitted returns true if we have already seen this exact evidence and it is already marked as committed.
func (evpool *Pool) IsCommitted(evidence types.Evidence) bool {
	ei := evpool.store.getInfo(evidence)
	return ei.Evidence != nil && ei.Committed
}

// EvidenceFront ...
func (evpool *Pool) EvidenceFront() *clist.CElement {
	return evpool.evidenceList.Front()
}

// EvidenceWaitChan ...
func (evpool *Pool) EvidenceWaitChan() <-chan struct{} {
	return evpool.evidenceList.WaitChan()
}

// SetLogger sets the Logger.
func (evpool *Pool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// PriorityEvidence returns the priority evidence.
func (evpool *Pool) PriorityEvidence() []types.Evidence {
	return evpool.store.PriorityEvidence()
}

// PendingEvidence returns up to maxNum uncommitted evidence.
// If maxNum is -1, all evidence is returned.
func (evpool *Pool) PendingEvidence(maxNum int64) []types.Evidence {
	return evpool.store.PendingEvidence(maxNum)
}

// State returns the current state of the evpool.
func (evpool *Pool) State() cstate.LastestBlockState {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return evpool.state
}

// Update loads the latest
func (evpool *Pool) Update(block *types.Block, state cstate.LastestBlockState) {

	// sanity check
	if state.LastBlockHeight != block.Height() {
		panic(
			fmt.Sprintf("Failed EvidencePool.Update sanity check: got state.Height=%d with block.Height=%d",
				state.LastBlockHeight,
				block.Height(),
			),
		)
	}

	// update the state
	evpool.mtx.Lock()
	evpool.state = state
	evpool.mtx.Unlock()

	// remove evidence from pending and mark committed
	evpool.MarkEvidenceAsCommitted(block.Height(), time.Now(), block.Evidence().Evidence)
}

// MarkEvidenceAsCommitted marks all the evidence as committed and removes it from the queue.
func (evpool *Pool) MarkEvidenceAsCommitted(height uint64, lastBlockTime time.Time, evidence []types.Evidence) {
	// make a map of committed evidence to remove from the clist
	blockEvidenceMap := make(map[string]struct{})
	for _, ev := range evidence {
		evpool.store.MarkEvidenceAsCommitted(ev)
		blockEvidenceMap[evMapKey(ev)] = struct{}{}
	}

	// remove committed evidence from the clist
	evidenceParams := evpool.State().ConsensusParams.Evidence
	evpool.removeEvidence(height, lastBlockTime, evidenceParams, blockEvidenceMap)
}

func (evpool *Pool) removeEvidence(
	height uint64,
	lastBlockTime time.Time,
	params kproto.EvidenceParams,
	blockEvidenceMap map[string]struct{}) {

	for e := evpool.evidenceList.Front(); e != nil; e = e.Next() {
		var (
			ev           = e.Value.(types.Evidence)
			ageDuration  = lastBlockTime.Sub(ev.Time())
			ageNumBlocks = int64(uint64(height) - ev.Height())
		)

		// Remove the evidence if it's already in a block or if it's now too old.
		if _, ok := blockEvidenceMap[evMapKey(ev)]; ok ||
			(ageDuration > time.Duration(params.MaxAgeDuration)*time.Millisecond && ageNumBlocks > params.MaxAgeNumBlocks) {
			// remove from clist
			evpool.evidenceList.Remove(e)
			e.DetachPrev()
		}
	}
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *Pool) AddEvidence(evidence types.Evidence) error {

	// check if evidence is already stored
	if evpool.store.Has(evidence) {
		return ErrEvidenceAlreadyStored{}
	}

	if err := cstate.VerifyEvidence(evpool.stateDB, evpool.State(), evidence); err != nil {
		return ErrInvalidEvidence{err}
	}

	// fetch the validator and return its voting power as its priority
	// TODO: something better ?
	valset, err := cstate.LoadValidators(evpool.stateDB, evidence.Height())
	if err != nil {
		return err
	}
	_, val := valset.GetByAddress(evidence.Address())
	priority := val.VotingPower

	_, err = evpool.store.AddNewEvidence(evidence, int64(priority))
	if err != nil {
		return err
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	// add evidence to clist
	evpool.evidenceList.PushBack(evidence)

	return nil
}

func evMapKey(ev types.Evidence) string {
	return ev.Hash().String()
}
