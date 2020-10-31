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
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/kardiachain/go-kardiamain/types/evidence/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func initializeValidatorState(prival types.PrivValidator, height uint64) kaidb.Database {
	stateDB := memorydb.New()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			&types.Validator{
				Address:          prival.GetAddress(),
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
		LastValidators:              valSet,
		LastHeightValidatorsChanged: 1,
		ChainID:                     "kai",
		ConsensusParams: kproto.ConsensusParams{
			Evidence: kproto.EvidenceParams{
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
	_, privVals := types.RandValidatorSet(3, 10)
	var (
		height       = uint64(100002)
		stateDB      = initializeValidatorState(privVals[0], height)
		evidenceDB   = memorydb.New()
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("uint64")).Return(
		&types.BlockMeta{Header: &types.Header{Time: evidenceTime}},
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	goodEvidence := types.NewMockDuplicateVoteEvidenceWithValidator(height, time.Now(), privVals[0], "kai")
	badEvidence := types.NewMockDuplicateVoteEvidenceWithValidator(1, evidenceTime, privVals[0], "kai")
	// bad evidence
	err = pool.AddEvidence(badEvidence)
	assert.Error(t, err)
	// err: evidence created at 2019-01-01 00:00:00 +0000 UTC has expired. Evidence can not be older than: ...

	if err := pool.AddEvidence(goodEvidence); err != nil {
		t.Fatal("Fail to add goodEvidence:", err)
	}

	<-pool.EvidenceWaitChan()

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt change the size
	err = pool.AddEvidence(goodEvidence)
	assert.NoError(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}
