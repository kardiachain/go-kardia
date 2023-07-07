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
)

func TestStoreLoadValidators(t *testing.T) {
	// stateDB := memorydb.New()
	// stateStore := cstate.NewStore(stateDB)
	// val, _ := types.RandValidator(true, 10)
	// vals := types.NewValidatorSet([]*types.Validator{val})

	// // 1) LoadValidators loads validators using a height where they were last changed
	// cstate.SaveValidatorsInfo(stateDB, 1, vals)
	// loadedVals, err := stateStore.LoadValidators(2)
	// require.NoError(t, err)
	// assert.NotZero(t, loadedVals.Size())

	// // 2) LoadValidators loads validators using a checkpoint height

	// cstate.SaveValidatorsInfo(stateDB, cstate.ValSetCheckpointInterval, 1, vals)

	// loadedVals, err = stateStore.LoadValidators(cstate.ValSetCheckpointInterval)
	// require.NoError(t, err)
	// assert.NotZero(t, loadedVals.Size())
}
