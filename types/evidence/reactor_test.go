package evidence

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
)

var (
	NumEvidence = 10
	Timeout     = 120 * time.Second // ridiculously high because CircleCI is slow
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

func sendEvidence(t *testing.T, evpool *Pool, valAddr common.Address, n int) types.EvidenceList {
	evList := make([]types.Evidence, n)
	for i := 0; i < n; i++ {
		ev := types.NewMockEvidence(uint64(i+1), time.Now().UTC(), 0, valAddr)
		err := evpool.AddEvidence(ev)
		assert.Nil(t, err)
		evList[i] = ev
	}
	return evList
}

func TestListMessageValidationBasic(t *testing.T) {

	testCases := []struct {
		testName          string
		malleateEvListMsg func(*ListMessage)
		expectErr         bool
	}{
		{"Good ListMessage", func(evList *ListMessage) {}, false},
		{"Invalid ListMessage", func(evList *ListMessage) {
			priv, _ := crypto.GenerateKey()
			evList.Evidence = append(evList.Evidence,
				&types.DuplicateVoteEvidence{Addr: crypto.PubkeyToAddress(priv.PublicKey)})
		}, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			evListMsg := &ListMessage{}
			n := 3
			valAddr := common.BytesToAddress([]byte("myval"))
			evListMsg.Evidence = make([]types.Evidence, n)
			for i := 0; i < n; i++ {
				evListMsg.Evidence[i] = types.NewMockEvidence(uint64(i+1), time.Now(), 0, valAddr)
			}
			tc.malleateEvListMsg(evListMsg)
			assert.Equal(t, tc.expectErr, evListMsg.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
