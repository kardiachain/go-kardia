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

	"github.com/kardiachain/go-kardia/kai/state/cstate"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/types/evidence/mocks"

	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/p2p"
	"github.com/kardiachain/go-kardia/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	numEvidence = 10
	Timeout     = 5 * time.Second // ridiculously high because CircleCI is slow
)

// We have N evidence reactors connected to one another. The first reactor
// receives a number of evidence at varying heights. We test that all
// other reactors receive the evidence and add it to their own respective
// evidence pools.
func TestReactorBroadcastEvidence(t *testing.T) {
	config := configs.DefaultP2PConfig()
	N := 7

	// create statedb for everyone
	stateDBs := make([]cstate.Store, N)
	val := types.NewMockPV()
	// we need validators saved for heights at least as high as we have evidence for
	height := uint64(numEvidence) + 10
	for i := 0; i < N; i++ {
		stateDBs[i] = initializeValidatorState(val, height)
	}

	// make reactors from statedb
	reactors := makeAndConnectReactors(config, stateDBs)

	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			ps := peerState{height}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// send a bunch of valid evidence to the first reactor's evpool
	// and wait for them all to be received in the others
	evList := sendEvidence(t, reactors[0].evpool, val, numEvidence)
	waitForEvidence(t, evList, reactors)
}

// We have two evidence reactors connected to one another but are at different heights.
// Reactor 1 which is ahead receives a number of evidence. It should only send the evidence
// that is below the height of the peer to that peer.
func TestReactorSelectiveBroadcast(t *testing.T) {
	config := configs.DefaultP2PConfig()

	val := types.NewMockPV()
	height1 := uint64(numEvidence) + 10
	height2 := uint64(numEvidence) / 2

	// DB1 is ahead of DB2
	stateDB1 := initializeValidatorState(val, height1)
	stateDB2 := initializeValidatorState(val, height2)

	// make reactors from statedb
	reactors := makeAndConnectReactors(config, []cstate.Store{stateDB1, stateDB2})

	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			ps := peerState{height1}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// update the first reactor peer's height to be very small
	peer := reactors[0].Switch.Peers().List()[0]
	ps := peerState{height2}
	peer.Set(types.PeerStateKey, ps)

	// send a bunch of valid evidence to the first reactor's evpool
	evList := sendEvidence(t, reactors[0].evpool, val, numEvidence)

	// only ones less than the peers height should make it through
	waitForEvidence(t, evList[:numEvidence/2-1], []*Reactor{reactors[1]})

	// peers should still be connected
	peers := reactors[1].Switch.Peers().List()
	assert.Equal(t, 1, len(peers))
}

// This tests aims to ensure that reactors don't send evidence that they have committed or that ar
// not ready for the peer through three scenarios.
// First, committed evidence to a newly connected peer
// Second, evidence to a peer that is behind
// Third, evidence that was pending and became committed just before the peer caught up
func TestReactorsGossipNoCommittedEvidence(t *testing.T) {
	config := configs.DefaultP2PConfig()

	val := types.NewMockPV()
	var height uint64 = 10

	// DB1 is ahead of DB2
	stateDB1 := initializeValidatorState(val, height)
	stateDB2 := initializeValidatorState(val, height-2)

	// make reactors from statedb
	reactors := makeAndConnectReactors(config, []cstate.Store{stateDB1, stateDB2})

	evList := sendEvidence(t, reactors[0].evpool, val, 2)
	vmEvs := reactors[0].evpool.VMEvidence(height, evList)
	require.EqualValues(t, 2, len(vmEvs))
	require.EqualValues(t, uint32(0), reactors[0].evpool.Size())

	time.Sleep(100 * time.Millisecond)

	peer := reactors[0].Switch.Peers().List()[0]
	ps := peerState{height - 2}
	peer.Set(types.PeerStateKey, ps)

	peer = reactors[1].Switch.Peers().List()[0]
	ps = peerState{height}
	peer.Set(types.PeerStateKey, ps)

	// wait to see that no evidence comes through
	time.Sleep(300 * time.Millisecond)

	// the second pool should not have received any evidence because it has already been committed
	assert.Equal(t, uint32(0), reactors[1].evpool.Size(), "second reactor should not have received evidence")

	// the first reactor receives three more evidence
	evList = make([]types.Evidence, 3)
	for i := 0; i < 3; i++ {
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(height-3+uint64(i),
			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC), val, "kai")
		err := reactors[0].evpool.AddEvidence(ev)
		require.NoError(t, err)
		evList[i] = ev
	}

	// wait to see that only one evidence is sent
	time.Sleep(300 * time.Millisecond)

	// the second pool should only have received the first evidence because it is behind
	peerEv, _ := reactors[1].evpool.PendingEvidence(1000)
	assert.EqualValues(t, []types.Evidence{evList[0]}, peerEv)

	// the last evidence is committed and the second reactor catches up in state to the first
	// reactor. We therefore expect that the second reactor only receives one more evidence, the
	// one that is still pending and not the evidence that has already been committed.
	_ = reactors[0].evpool.VMEvidence(height, []types.Evidence{evList[2]})
	// the first reactor should have the two remaining pending evidence
	require.EqualValues(t, uint32(2), reactors[0].evpool.Size())

	// now update the state of the second reactor
	reactors[1].evpool.Update(cstate.LastestBlockState{LastBlockHeight: height})
	peer = reactors[0].Switch.Peers().List()[0]
	ps = peerState{height}
	peer.Set(types.PeerStateKey, ps)

	// wait to see that only two evidence is sent
	time.Sleep(300 * time.Millisecond)

	peerEv, _ = reactors[1].evpool.PendingEvidence(1000)
	assert.EqualValues(t, evList[0:1], peerEv)
}

type peerState struct {
	height uint64
}

func (ps peerState) GetHeight() uint64 {
	return ps.height
}

// connect N evidence reactors through N switches
func makeAndConnectReactors(p2pConfig *configs.P2PConfig, stateDBs []cstate.Store) []*Reactor {
	N := len(stateDBs)
	reactors := make([]*Reactor, N)
	logger := log.New()
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < N; i++ {

		evidenceDB := memorydb.New()
		blockStore := &mocks.BlockStore{}
		blockStore.On("LoadBlockMeta", mock.AnythingOfType("uint64")).Return(
			&types.BlockMeta{Header: &types.Header{Time: evidenceTime}},
		)
		pool, err := NewPool(stateDBs[i], evidenceDB, blockStore)
		if err != nil {
			panic(err)
		}
		reactors[i] = NewReactor(pool)
		reactors[i].SetLogger(logger.New("validator", i))
	}

	p2p.MakeConnectedSwitches(p2pConfig, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("EVIDENCE", reactors[i])
		return s

	}, p2p.Connect2Switches)

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
	var evList []types.Evidence
	currentPoolSize := 0
	fmt.Println(reactorIdx)
	for currentPoolSize != len(evs) {
		evList, _ = evpool.PendingEvidence(int64(len(evs) * 500)) // each evidence should not be more than 500 bytes
		currentPoolSize = len(evList)
		time.Sleep(time.Millisecond * 100)
	}

	// put the reaped evidence in a map so we can quickly check we got everything
	evMap := make(map[string]types.Evidence)
	for _, e := range evList {
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
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(uint64(i+1),
			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC), privVal, "kai")
		err := evpool.AddEvidence(ev)
		assert.Nil(t, err, err)
		evList[i] = ev
	}
	return evList
}
