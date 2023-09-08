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

package cstate_test

import (
	"testing"

	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	"github.com/kardiachain/go-kardia/kai/state/cstate"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
	"github.com/stretchr/testify/assert"
)

func TestSaveState(t *testing.T) {
	db := memorydb.New()
	stateStore := cstate.NewStore(db)
	val, _ := types.RandValidator(true, 10)
	val1, _ := types.RandValidator(true, 10)
	lvals := types.NewValidatorSet([]*types.Validator{val})
	vals := types.NewValidatorSet([]*types.Validator{val, val1})
	cparams := configs.DefaultConsensusParams()
	bz, _ := cparams.Marshal()
	cparamsHash := common.BytesToHash(bz)

	// 1) Consensus state at block 0
	state := cstate.LatestBlockState{
		LastBlockHeight:                  0,
		LastValidators:                   nil,
		Validators:                       lvals,
		NextValidators:                   lvals,
		LastHeightValidatorsChanged:      1,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(state)

	wstate := rawdb.ReadConsensusStateHeight(db, state.LastBlockHeight)
	assert.NotNil(t, wstate)
	assert.Equal(t, wstate.GetLastValidatorsInfoHash(), common.NewZeroHash().Bytes())
	assert.Equal(t, wstate.GetValidatorsInfoHash(), lvals.Hash().Bytes())
	assert.Equal(t, wstate.GetNextValidatorsInfoHash(), lvals.Hash().Bytes())
	assert.Equal(t, wstate.GetConsensusParamsInfoHash(), cparamsHash.Bytes())
	lValsInfo := rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(wstate.GetLastValidatorsInfoHash()))
	valsInfo := rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(wstate.GetValidatorsInfoHash()))
	nValsInfo := rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(wstate.GetNextValidatorsInfoHash()))
	cparamsInfo := rawdb.ReadConsensusParamsInfo(db, cparamsHash)
	assert.NotNil(t, lValsInfo)
	assert.NotNil(t, valsInfo)
	assert.NotNil(t, nValsInfo)
	assert.NotNil(t, cparamsInfo)
	assert.Equal(t, lValsInfo.LastHeightChanged, state.LastHeightValidatorsChanged)
	assert.Equal(t, valsInfo.LastHeightChanged, state.LastHeightValidatorsChanged)
	assert.Equal(t, nValsInfo.LastHeightChanged, state.LastHeightValidatorsChanged)
	assert.Equal(t, cparamsInfo.LastHeightChanged, state.LastHeightConsensusParamsChanged)

	// 2) Consensus state at block != 0
	anotherState := cstate.LatestBlockState{
		LastBlockHeight:                  1,
		LastValidators:                   lvals,
		Validators:                       vals,
		NextValidators:                   vals,
		LastHeightValidatorsChanged:      3,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(anotherState)

	wstate = rawdb.ReadConsensusStateHeight(db, anotherState.LastBlockHeight)
	assert.NotNil(t, wstate)
	assert.Equal(t, wstate.GetLastValidatorsInfoHash(), lvals.Hash().Bytes())
	assert.Equal(t, wstate.GetValidatorsInfoHash(), vals.Hash().Bytes())
	assert.Equal(t, wstate.GetNextValidatorsInfoHash(), vals.Hash().Bytes())
	assert.Equal(t, wstate.GetConsensusParamsInfoHash(), cparamsHash.Bytes())
	lValsInfo = rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(wstate.GetLastValidatorsInfoHash()))
	valsInfo = rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(wstate.GetValidatorsInfoHash()))
	nValsInfo = rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(wstate.GetNextValidatorsInfoHash()))
	cparamsInfo = rawdb.ReadConsensusParamsInfo(db, cparamsHash)
	assert.NotNil(t, lValsInfo)
	assert.NotNil(t, valsInfo)
	assert.NotNil(t, nValsInfo)
	assert.NotNil(t, cparamsInfo)
	assert.Equal(t, lValsInfo.LastHeightChanged, state.LastHeightValidatorsChanged)
	assert.Equal(t, valsInfo.LastHeightChanged, anotherState.LastHeightValidatorsChanged)
	assert.Equal(t, nValsInfo.LastHeightChanged, anotherState.LastHeightValidatorsChanged)
	assert.Equal(t, cparamsInfo.LastHeightChanged, anotherState.LastHeightConsensusParamsChanged)
}

func TestPruneState(t *testing.T) {
	db := memorydb.New()
	stateStore := cstate.NewStore(db)
	val, _ := types.RandValidator(true, 10)
	val1, _ := types.RandValidator(true, 10)
	val2, _ := types.RandValidator(true, 10)
	lvals := types.NewValidatorSet([]*types.Validator{val})
	vals := types.NewValidatorSet([]*types.Validator{val, val1})
	nvals := types.NewValidatorSet([]*types.Validator{val, val1, val2})
	cparams := configs.DefaultConsensusParams()

	// Consensus state at block #0, which will not be pruned
	state := cstate.LatestBlockState{
		LastBlockHeight:                  0,
		LastValidators:                   nil,
		Validators:                       lvals,
		NextValidators:                   lvals,
		LastHeightValidatorsChanged:      1,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(state)

	// Consensus state at block #1, which will be pruned
	state = cstate.LatestBlockState{
		LastBlockHeight:                  1,
		LastValidators:                   lvals,
		Validators:                       vals,
		NextValidators:                   vals,
		LastHeightValidatorsChanged:      1,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(state)

	// Consensus state at block #2, which will be pruned
	state = cstate.LatestBlockState{
		LastBlockHeight:                  2,
		LastValidators:                   vals,
		Validators:                       vals,
		NextValidators:                   vals,
		LastHeightValidatorsChanged:      1,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(state)

	// Consensus state at blocks #3 and #4 are not presented

	// Consensus state at block #5, which will not be pruned
	state = cstate.LatestBlockState{
		LastBlockHeight:                  5,
		LastValidators:                   nvals,
		Validators:                       nvals,
		NextValidators:                   nvals,
		LastHeightValidatorsChanged:      4,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(state)

	// perform pruningƒ
	prunedStates, prunedValInfos, _ := stateStore.PruneState(0, 5)
	assert.Equal(t, uint64(2), prunedStates)
	assert.Equal(t, uint64(1), prunedValInfos)

	genesisState := rawdb.ReadConsensusStateHeight(db, 0)
	assert.NotNil(t, genesisState)
	prunedState := rawdb.ReadConsensusStateHeight(db, 1)
	assert.Nil(t, prunedState)
	notPrunedState := rawdb.ReadConsensusStateHeight(db, 5)
	assert.NotNil(t, notPrunedState)
	lValsInfo := rawdb.ReadConsensusValidatorsInfo(db, lvals.Hash())
	assert.NotNil(t, lValsInfo)
	valsInfo := rawdb.ReadConsensusValidatorsInfo(db, vals.Hash())
	assert.Nil(t, valsInfo)
	nValsInfo := rawdb.ReadConsensusValidatorsInfo(db, nvals.Hash())
	assert.NotNil(t, nValsInfo)
}

func TestLoadValidators(t *testing.T) {
	db := memorydb.New()
	stateStore := cstate.NewStore(db)
	val, _ := types.RandValidator(true, 10)
	val1, _ := types.RandValidator(true, 10)
	lvals := types.NewValidatorSet([]*types.Validator{val})
	vals := types.NewValidatorSet([]*types.Validator{val, val1})
	cparams := configs.DefaultConsensusParams()

	// State
	// Consensus state at block 0
	state := cstate.LatestBlockState{
		LastBlockHeight:                  0,
		LastValidators:                   nil,
		Validators:                       lvals,
		NextValidators:                   lvals,
		LastHeightValidatorsChanged:      1,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(state)

	// Consensus state at block 1
	anotherState := cstate.LatestBlockState{
		LastBlockHeight:                  1,
		LastValidators:                   lvals,
		Validators:                       vals,
		NextValidators:                   vals,
		LastHeightValidatorsChanged:      3,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(anotherState)

	// Consensus state at block 2
	ananotherState := cstate.LatestBlockState{
		LastBlockHeight:                  2,
		LastValidators:                   vals,
		Validators:                       vals,
		NextValidators:                   vals,
		LastHeightValidatorsChanged:      3,
		ConsensusParams:                  *cparams,
		LastHeightConsensusParamsChanged: 0,
	}
	stateStore.Save(ananotherState)

	vs1, _ := stateStore.LoadValidators(state.LastBlockHeight)
	assert.Nil(t, vs1)
	vs2, _ := stateStore.LoadValidators(anotherState.LastBlockHeight)
	assert.NotNil(t, vs2)
	assert.Equal(t, vs2.Hash(), lvals.Hash())
	vs3, _ := stateStore.LoadValidators(ananotherState.LastBlockHeight)
	assert.NotNil(t, vs3)
	assert.Equal(t, vs3.Hash(), vals.Hash())
}
