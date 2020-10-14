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
	"testing"
	"time"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	cState "github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/lib/common"
	tmproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
)

func initializeValidatorState(valAddr common.Address, height uint64) kaidb.Database {
	stateDB := memorydb.New()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			&types.Validator{
				Address:          valAddr,
				VotingPower:      100,
				ProposerPriority: 1,
			},
		},
	}
	valSet.IncrementProposerPriority(1)
	nextVal := valSet.Copy()
	nextVal.IncrementProposerPriority(1)
	state := cState.LastestBlockState{
		LastBlockHeight:             0,
		LastBlockTime:               time.Now(),
		Validators:                  valSet,
		NextValidators:              nextVal,
		LastHeightValidatorsChanged: 1,
		ConsensusParams: tmproto.ConsensusParams{
			Evidence: tmproto.EvidenceParams{
				MaxAgeNumBlocks: 10000,
				MaxAgeDuration:  48 * 60 * 60,
			},
		},
	}
	// save all states up to height
	for i := uint64(0); i < height; i++ {
		state.LastBlockHeight = i
		cState.SaveState(stateDB, state)
	}

	return stateDB
}

func TestEvidencePool(t *testing.T) {

	var (
		valAddr      = common.BytesToAddress([]byte("val1"))
		height       = uint64(100002)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = memorydb.New()
		pool         = NewPool(stateDB, evidenceDB)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	goodEvidence := types.NewMockEvidence(uint64(height), time.Now(), 0, valAddr)
	badEvidence := types.NewMockEvidence(1, evidenceTime, 0, valAddr)

	// bad evidence
	err := pool.AddEvidence(badEvidence)
	assert.Error(t, err)
	// err: evidence created at 2019-01-01 00:00:00 +0000 UTC has expired. Evidence can not be older than: ...

	if err := pool.AddEvidence(goodEvidence); err != nil {
		t.Fatal("Fail to add goodEvidence:", err)
	}

	<-pool.EvidenceWaitChan()

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt change the size
	err = pool.AddEvidence(goodEvidence)
	assert.Error(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}

func TestEvidencePoolIsCommitted(t *testing.T) {
	// Initialization:
	var (
		valAddr       = common.BytesToAddress([]byte("validator_address"))
		height        = uint64(42)
		lastBlockTime = time.Now()
		stateDB       = initializeValidatorState(valAddr, height)
		evidenceDB    = memorydb.New()
		pool          = NewPool(stateDB, evidenceDB)
	)

	// evidence not seen yet:
	evidence := types.NewMockEvidence(uint64(height), time.Now(), 0, valAddr)
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(evidence))
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, lastBlockTime, []types.Evidence{evidence})
	assert.True(t, pool.IsCommitted(evidence))
}

func TestAddEvidence(t *testing.T) {

	var (
		valAddr      = common.BytesToAddress([]byte("val1"))
		height       = uint64(100002)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = memorydb.New()
		pool         = NewPool(stateDB, evidenceDB)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	testCases := []struct {
		evHeight      uint64
		evTime        time.Time
		expErr        bool
		evDescription string
	}{
		{height, time.Now(), false, "valid evidence"},
		{uint64(1), evidenceTime, true,
			"evidence from height 1 is too old & evidence created at 2019-01-01 00:00:00 +0000 UTC has expired"},
	}

	for _, tc := range testCases {
		tc := tc
		ev := types.NewMockEvidence(tc.evHeight, tc.evTime, 0, valAddr)
		err := pool.AddEvidence(ev)
		if tc.expErr {
			assert.Error(t, err)
		}
	}
}
