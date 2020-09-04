package cstate_test

import (
	"testing"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state/cstate"
	"github.com/kardiachain/go-kardiamain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreLoadValidators(t *testing.T) {
	stateDB := memorydb.New()
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val}, 0, 1)

	// 1) LoadValidators loads validators using a height where they were last changed
	cstate.SaveValidatorsInfo(stateDB, 1, 1, vals)
	cstate.SaveValidatorsInfo(stateDB, 2, 1, vals)
	loadedVals, err := cstate.LoadValidators(stateDB, 2)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())

	// 2) LoadValidators loads validators using a checkpoint height

	cstate.SaveValidatorsInfo(stateDB, cstate.ValSetCheckpointInterval, 1, vals)

	loadedVals, err = cstate.LoadValidators(stateDB, cstate.ValSetCheckpointInterval)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())
}
