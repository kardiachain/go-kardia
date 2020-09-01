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

package state

import (
	"fmt"
	"math/big"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/rlp"

	"github.com/kardiachain/go-kardiamain/types"
)

// TODO(namdoh): Move to a common config file.
var (
	RefreshBackoffHeightStep = int64(200)
	RefreshHeightDelta       = int64(20)
	stateKey                 = []byte("stateKey")
)

// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type LastestBlockState struct {
	// Immutable
	ChainID string

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight  uint64
	LastBlockTotalTx uint64
	LastBlockID      types.BlockID
	LastBlockTime    *big.Int

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	NextValidators              *types.ValidatorSet `rlp:"nil"`
	Validators                  *types.ValidatorSet `rlp:"nil"`
	LastValidators              *types.ValidatorSet `rlp:"nil"`
	LastHeightValidatorsChanged uint64

	ConsensusParams                  types.ConsensusParams
	LastHeightConsensusParamsChanged uint64
	// TODO(namdoh): Add consensus parameters used for validating blocks.

	// Merkle root of the results from executing prev block
	//namdoh@ LastResultsHash []byte

	// The latest AppHash we've received from calling abci.Commit()
	//namdoh@ AppHash []byte
}

// Copy makes a copy of the State for mutating.
func (state LastestBlockState) Copy() LastestBlockState {
	return LastestBlockState{
		ChainID: state.ChainID,

		LastBlockHeight:  state.LastBlockHeight,
		LastBlockTotalTx: state.LastBlockTotalTx,
		LastBlockID:      state.LastBlockID,
		LastBlockTime:    state.LastBlockTime,

		NextValidators:              state.NextValidators.Copy(),
		Validators:                  state.Validators.Copy(),
		LastValidators:              state.LastValidators.Copy(),
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,

		//namdoh@ AppHash: state.AppHash,

		//namdoh@ LastResultsHash: state.LastResultsHash,
	}
}

// IsEmpty returns true if the State is equal to the empty State.
func (state LastestBlockState) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}

// Stringshort returns a short string representing State
func (state LastestBlockState) String() string {
	return fmt.Sprintf("{ChainID:%v LastBlockHeight:%v LastBlockTotalTx:%v LastBlockID:%v LastBlockTime:%v Validators:%v LastValidators:%v LastHeightValidatorsChanged:%v",
		state.ChainID, state.LastBlockHeight, state.LastBlockTotalTx, state.LastBlockID, time.Unix(state.LastBlockTime.Int64(), 0),
		state.Validators, state.LastValidators, state.LastHeightValidatorsChanged)
}

// Fetches the validator set at a given height.
// TODO(huny@): Please implement this function.
func (state *LastestBlockState) fetchValidatorSet(height int64) *types.ValidatorSet {
	// TODO(huny@): Update this.
	return state.Validators
}

// Bytes ...
func (state *LastestBlockState) Bytes() []byte {
	b, err := rlp.EncodeToBytes(state)
	if err != nil {
		panic(err)
	}
	return b
}
