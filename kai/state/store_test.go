package state_test

import (
	"testing"

	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/types"
)

func TestStoreLoadValidators(t *testing.T) {
	stateDB := memorydb.New()
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})

	// 1) LoadValidators loads validators using a height where they were last changed
	state.SaveValidatorsInfo(stateDB, 1, 1, vals)
	state.SaveValidatorsInfo(stateDB, 2, 1, vals)
	loadedVals, err := state.LoadValidators(stateDB, 2)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())

	// 2) LoadValidators loads validators using a checkpoint height

	state.SaveValidatorsInfo(stateDB, state.ValSetCheckpointInterval, 1, vals)

	loadedVals, err = state.LoadValidators(stateDB, state.ValSetCheckpointInterval)
	require.NoError(t, err)
	assert.NotZero(t, loadedVals.Size())
}
