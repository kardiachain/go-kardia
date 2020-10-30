/*
 *  Copyright 2020 KardiaChain
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
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
)

var (
	Timeout = 120 * time.Second // ridiculously high because CircleCI is slow
)

// connect N evidence reactors through N switches
func makeAndConnectReactors(stateDBs []kaidb.Database) []*Reactor {
	N := len(stateDBs)
	reactors := make([]*Reactor, N)
	logger := log.New()
	for i := 0; i < N; i++ {

		evidenceDB := memorydb.New()
		pool := NewPool(stateDBs[i], evidenceDB)
		reactors[i] = NewReactor(pool)
		reactors[i].SetLogger(logger)
	}
	return reactors
}

// wait for all evidence on all reactors
func waitForEvidence(t *testing.T, evs types.EvidenceList, reactors []*Reactor) {
	// wait for the evidence in all evpools
	wg := new(sync.WaitGroup)
	for i := 0; i < len(reactors); i++ {
		wg.Add(1)
		go _waitForEvidence(t, wg, evs, i, reactors)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(Timeout)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for evidence")
	case <-done:
	}
}

// wait for all evidence on a single evpool
func _waitForEvidence(
	t *testing.T,
	wg *sync.WaitGroup,
	evs types.EvidenceList,
	reactorIdx int,
	reactors []*Reactor,
) {
	evpool := reactors[reactorIdx].evpool
	for len(evpool.PendingEvidence(-1)) != len(evs) {
		time.Sleep(time.Millisecond * 100)
	}

	reapedEv := evpool.PendingEvidence(-1)
	// put the reaped evidence in a map so we can quickly check we got everything
	evMap := make(map[string]types.Evidence)
	for _, e := range reapedEv {
		evMap[e.Hash().String()] = e
	}
	for i, expectedEv := range evs {
		gotEv := evMap[expectedEv.Hash().String()]
		assert.Equal(t, expectedEv, gotEv,
			fmt.Sprintf("evidence at index %d on reactor %d don't match: %v vs %v",
				i, reactorIdx, expectedEv, gotEv))
	}

	wg.Done()
}

func sendEvidence(t *testing.T, evpool *Pool, privVal types.PrivValidator, n int) types.EvidenceList {
	evList := make([]types.Evidence, n)
	for i := 0; i < n; i++ {
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(uint64(i+1), time.Now(), privVal, "kai")
		err := evpool.AddEvidence(ev)
		assert.Nil(t, err)
		evList[i] = ev
	}
	return evList
}
