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

package cstate

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/kardiachain/go-kardiamain/mainchain/genesis"

	"github.com/kardiachain/go-kardiamain/lib/common"

	"github.com/kardiachain/go-kardiamain/lib/rlp"
	"github.com/kardiachain/go-kardiamain/types"

	"github.com/kardiachain/go-kardiamain/kai/kaidb"
	"github.com/kardiachain/go-kardiamain/lib/math"
	tmstate "github.com/kardiachain/go-kardiamain/proto/kardiachain/state"
	kproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

const (
	// persist validators every valSetCheckpointInterval blocks to avoid
	// LoadValidators taking too much time.
	// https://github.com/tendermint/tendermint/pull/3438
	// 100000 results in ~ 100ms to get 100 validators (see BenchmarkLoadValidators)
	valSetCheckpointInterval = 100000
)

//------------------------------------------------------------------------

func calcValidatorsKey(height uint64) []byte {
	return []byte(fmt.Sprintf("validatorsKey:%v", height))
}

func calcConsensusParamsKey(height uint64) []byte {
	return []byte(fmt.Sprintf("consensusParamsKey:%v", height))
}

// LoadStateFromDBOrGenesisDoc loads the most recent state from the database,
// or creates a new one from the given genesisDoc and persists the result
// to the database.
func LoadStateFromDBOrGenesisDoc(stateDB kaidb.Database, genesisDoc *genesis.Genesis) (LastestBlockState, error) {
	state := LoadState(stateDB)

	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisState(genesisDoc)
		if err != nil {
			return state, err
		}
		SaveState(stateDB, state)
	}
	return state, nil
}

// SaveState persists the State, the ValidatorsInfo, and the ConsensusParamsInfo to the database.
// This flushes the writes (e.g. calls SetSync).
func SaveState(db kaidb.KeyValueStore, state LastestBlockState) {
	saveState(db, state, stateKey)
}

func saveState(db kaidb.KeyValueStore, state LastestBlockState, key []byte) {
	nextHeight := state.LastBlockHeight + 1
	// If first block, save validators for block 1.
	if nextHeight == 1 {
		// This extra logic due to Tendermint validator set changes being delayed 1 block.
		// It may get overwritten due to InitChain validator updates.
		lastHeightVoteChanged := uint64(1)
		saveValidatorsInfo(db, nextHeight, lastHeightVoteChanged, state.Validators)
	}
	// Save next validators.
	saveValidatorsInfo(db, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
	// Save next consensus params.
	saveConsensusParamsInfo(db, uint64(nextHeight), state.LastHeightConsensusParamsChanged, state.ConsensusParams)
	_ = db.Put(key, state.Bytes())
}

// LoadState loads the State from the database.
func LoadState(db kaidb.Database) LastestBlockState {
	return loadState(db, stateKey)
}

func loadState(db kaidb.Database, key []byte) (state LastestBlockState) {
	buf, _ := db.Get(key)

	if len(buf) == 0 {
		return state
	}
	sp := new(tmstate.State)
	err := proto.Unmarshal(buf, sp)

	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		panic(fmt.Sprintf(`LoadState: Data has been corrupted or its spec has changed:
		%v\n`, err))
	}

	sm, err := StateFromProto(sp)
	if err != nil {
		panic(err)
	}

	return *sm
}

//-----------------------------------------------------------------------------

// ValidatorsInfo represents the latest validator set, or the last height it changed
type ValidatorsInfo struct {
	ValidatorSet      *types.ValidatorSet `rlp:"nil"`
	LastHeightChanged uint64
}

// Bytes serializes the ValidatorsInfo
func (valInfo *ValidatorsInfo) Bytes() []byte {
	b, err := rlp.EncodeToBytes(valInfo)
	if err != nil {
		panic(err)
	}
	return b
}

// LoadValidators loads the ValidatorSet for a given height.
// Returns ErrNoValSetForHeight if the validator set can't be found for this height.
func LoadValidators(db kaidb.KeyValueStore, height uint64) (*types.ValidatorSet, error) {
	valInfo := loadValidatorsInfo(db, uint64(height))
	if valInfo == nil {
		return nil, ErrNoValSetForHeight{height}
	}
	if valInfo.ValidatorSet == nil {
		lastStoredHeight := lastStoredHeightFor(height, valInfo.LastHeightChanged)
		valInfo2 := loadValidatorsInfo(db, uint64(lastStoredHeight))
		if valInfo2 == nil || valInfo2.ValidatorSet == nil {
			panic(
				fmt.Sprintf("Couldn't find validators at height %d (height %d was originally requested)",
					lastStoredHeight,
					height,
				),
			)
		}
		vs, err := types.ValidatorSetFromProto(valInfo2.ValidatorSet)
		if err != nil {
			return nil, err
		}
		vs.IncrementProposerPriority(int64(height) - lastStoredHeight) // mutate
		vi2, err := vs.ToProto()
		if err != nil {
			return nil, err
		}

		valInfo2.ValidatorSet = vi2
		valInfo = valInfo2
	}
	vip, err := types.ValidatorSetFromProto(valInfo.ValidatorSet)
	if err != nil {
		return nil, err
	}
	return vip, nil
}

func lastStoredHeightFor(height, lastHeightChanged uint64) int64 {
	checkpointHeight := height - height%valSetCheckpointInterval
	return math.MaxInt64(int64(checkpointHeight), int64(lastHeightChanged))
}

// CONTRACT: Returned ValidatorsInfo can be mutated.
func loadValidatorsInfo(db kaidb.Database, height uint64) *tmstate.ValidatorsInfo {
	buf, err := db.Get(calcValidatorsKey(height))
	if err != nil {
		panic(err)
	}
	if len(buf) == 0 {
		return nil
	}

	v := new(tmstate.ValidatorsInfo)
	err = v.Unmarshal(buf)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		panic(fmt.Sprintf(`LoadValidators: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}

	return v
}

// saveValidatorsInfo persists the validator set.
//
// `height` is the effective height for which the validator is responsible for
// signing. It should be called from s.Save(), right before the state itself is
// persisted.
func saveValidatorsInfo(db kaidb.Database, height, lastHeightChanged uint64, valSet *types.ValidatorSet) {
	if lastHeightChanged > height {
		panic("LastHeightChanged cannot be greater than ValidatorsInfo height")
	}
	valInfo := &tmstate.ValidatorsInfo{
		LastHeightChanged: lastHeightChanged,
	}

	pv, err := valSet.ToProto()
	if err != nil {
		panic(err)
	}
	valInfo.ValidatorSet = pv

	bz, err := valInfo.Marshal()
	if err != nil {
		panic(err)
	}

	err = db.Put(calcValidatorsKey(height), bz)
	if err != nil {
		panic(err)
	}
}

//-----------------------------------------------------------------------------

// LoadConsensusParams loads the ConsensusParams for a given height.
func LoadConsensusParams(db kaidb.Database, height uint64) (kproto.ConsensusParams, error) {
	empty := kproto.ConsensusParams{}

	paramsInfo, err := loadConsensusParamsInfo(db, height)
	if err != nil {
		return empty, fmt.Errorf("could not find consensus params for height #%d: %w", height, err)
	}

	if paramsInfo.ConsensusParams.Equal(&empty) {
		paramsInfo2, err := loadConsensusParamsInfo(db, paramsInfo.LastHeightChanged)
		if err != nil {
			return empty, fmt.Errorf(
				"couldn't find consensus params at height %d as last changed from height %d: %w",
				paramsInfo.LastHeightChanged,
				height,
				err,
			)
		}

		paramsInfo = paramsInfo2
	}

	return paramsInfo.ConsensusParams, nil
}

func loadConsensusParamsInfo(db kaidb.Database, height uint64) (*tmstate.ConsensusParamsInfo, error) {
	buf, err := db.Get(calcConsensusParamsKey(uint64(height)))
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, nil
	}

	paramsInfo := new(tmstate.ConsensusParamsInfo)
	if err = paramsInfo.Unmarshal(buf); err != nil {
		return nil, err
	}
	// TODO: ensure that buf is completely read.

	return paramsInfo, nil
}

// saveConsensusParamsInfo persists the consensus params for the next block to disk.
// It should be called from s.Save(), right before the state itself is persisted.
// If the consensus params did not change after processing the latest block,
// only the last height for which they changed is persisted.
func saveConsensusParamsInfo(db kaidb.Database, nextHeight, changeHeight uint64, params kproto.ConsensusParams) {
	paramsInfo := &tmstate.ConsensusParamsInfo{
		LastHeightChanged: changeHeight,
	}

	if changeHeight == nextHeight {
		paramsInfo.ConsensusParams = params
	}

	bz, err := paramsInfo.Marshal()
	if err != nil {
		panic(err)
	}
	err = db.Put(calcConsensusParamsKey(nextHeight), bz)
	if err != nil {
		panic(err)
	}
}

// MakeGenesisState creates state from types.GenesisDoc.
func MakeGenesisState(genDoc *genesis.Genesis) (LastestBlockState, error) {
	var validatorSet, nextValidatorSet *types.ValidatorSet
	if genDoc.Validators == nil {
		validatorSet = nil
		nextValidatorSet = nil
	} else {
		validators := make([]*types.Validator, len(genDoc.Validators))
		for i, val := range genDoc.Validators {
			validators[i] = types.NewValidator(common.HexToAddress(val.Address), val.Power)
		}
		validatorSet = types.NewValidatorSet(validators)
		nextValidatorSet = types.NewValidatorSet(validators).CopyIncrementProposerPriority(1)
	}
	return LastestBlockState{
		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastBlockTime:   genDoc.Timestamp,

		NextValidators:              nextValidatorSet,
		Validators:                  validatorSet,
		LastValidators:              nil,
		LastHeightValidatorsChanged: 0,

		ConsensusParams:                  *genDoc.ConsensusParams,
		LastHeightConsensusParamsChanged: 1,
	}, nil
}
