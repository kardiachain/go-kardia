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
	"math/big"

	"github.com/kardiachain/go-kardia/configs"

	"github.com/gogo/protobuf/proto"
	"github.com/kardiachain/go-kardia/mainchain/genesis"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"

	"github.com/kardiachain/go-kardia/lib/rlp"
	"github.com/kardiachain/go-kardia/types"

	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	kmath "github.com/kardiachain/go-kardia/lib/math"
	kstate "github.com/kardiachain/go-kardia/proto/kardiachain/state"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
)

const (
	// persist validators every valSetCheckpointInterval blocks to avoid
	// LoadValidators taking too much time.
	valSetCheckpointInterval = 100000
)

type Store interface {
	LoadStateFromDBOrGenesisDoc(genesisDoc *genesis.Genesis) (LatestBlockState, error)
	Load() LatestBlockState
	Save(LatestBlockState)
	LoadValidators(height uint64) (*types.ValidatorSet, error)
	LoadConsensusParams(height uint64) (kproto.ConsensusParams, error)
	LoadStateAtHeight(height uint64) LatestBlockState
}

//------------------------------------------------------------------------

func calcValidatorsKey(height uint64) []byte {
	return []byte(fmt.Sprintf("validatorsKey:%v", height))
}

func calcConsensusParamsKey(height uint64) []byte {
	return []byte(fmt.Sprintf("consensusParamsKey:%v", height))
}

type dbStore struct {
	db kaidb.Database
}

func NewStore(db kaidb.Database) Store {
	return &dbStore{db: db}
}

// LoadStateFromDBOrGenesisDoc loads the most recent state from the database,
// or creates a new one from the given genesisDoc and persists the result
// to the database.
func (s *dbStore) LoadStateFromDBOrGenesisDoc(genesisDoc *genesis.Genesis) (LatestBlockState, error) {
	state := s.Load()

	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisState(genesisDoc)
		if err != nil {
			return state, err
		}
		s.Save(state)
	}
	return state, nil
}

// SaveState persists the State, the ValidatorsInfo, and the ConsensusParamsInfo to the database.
// This flushes the writes (e.g. calls SetSync).
func (s *dbStore) Save(state LatestBlockState) {
	saveState(s.db, state, stateKey)
}

func saveState(db kaidb.KeyValueStore, state LatestBlockState, key []byte) {
	nextHeight := state.LastBlockHeight + 1
	// If first block, save validators for block 1.
	if nextHeight == 1 {
		nextHeight = state.InitialHeight
		// This extra logic due to validator set changes being delayed 1 block.
		// It may get overwritten due to InitChain validator updates.
		saveValidatorsInfo(db, nextHeight, nextHeight, state.Validators)
	}
	// Save next validators.
	saveValidatorsInfo(db, nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators)
	// Save next consensus params.
	saveConsensusParamsInfo(db, uint64(nextHeight), state.LastHeightConsensusParamsChanged, state.ConsensusParams)
	_ = db.Put(key, state.Bytes())
}

// LoadState loads the State from the database.
func (s *dbStore) Load() LatestBlockState {
	latestState := loadState(s.db, stateKey)

	head := rawdb.ReadHeadBlock(s.db)
	if head != nil && head.Height() < latestState.LastBlockHeight {
		log.Warn("Rewound cstate.LatestBlockState", "from", latestState.LastBlockHeight, "to", head.Height)
		return s.LoadStateAtHeight(head.Height())
	}

	return latestState
}

func loadState(db kaidb.Database, key []byte) (state LatestBlockState) {
	buf, _ := db.Get(key)

	if len(buf) == 0 {
		return state
	}
	sp := new(kstate.State)
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
	if sm.InitialHeight == 0 {
		sm.InitialHeight = 1
	}
	return *sm
}

func (s *dbStore) LoadStateAtHeight(height uint64) LatestBlockState {
	latestState := loadState(s.db, stateKey)
	if height <= 2 || height >= latestState.LastBlockHeight {
		panic(fmt.Sprintf(`Latest state is at height=%v requested height=%v`, latestState.LastBlockHeight, height))
	}

	state := latestState.Copy()

	block := rawdb.ReadBlock(s.db, height)
	blockMeta := rawdb.ReadBlockMeta(s.db, height)
	state.LastBlockHeight = height
	state.LastBlockTotalTx = block.NumTxs()
	state.LastBlockID = blockMeta.BlockID
	state.LastBlockTime = block.Time()

	lastvals, _ := s.LoadValidators(height - 1)
	vals, _ := s.LoadValidators(height)
	nextvals, _ := s.LoadValidators(height + 1)

	state.LastValidators = lastvals
	state.Validators = vals
	state.NextValidators = nextvals

	valInfo := loadValidatorsInfo(s.db, height)
	state.LastHeightValidatorsChanged = valInfo.LastHeightChanged

	appHash := rawdb.ReadAppHash(s.db, height)
	state.AppHash = appHash

	return state
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
func (s *dbStore) LoadValidators(height uint64) (*types.ValidatorSet, error) {
	valInfo := loadValidatorsInfo(s.db, uint64(height))
	if valInfo == nil {
		return nil, ErrNoValSetForHeight{height}
	}
	if valInfo.ValidatorSet == nil {
		lastStoredHeight := lastStoredHeightFor(height, valInfo.LastHeightChanged)
		valInfo2 := loadValidatorsInfo(s.db, uint64(lastStoredHeight))
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
	return kmath.MaxInt64(int64(checkpointHeight), int64(lastHeightChanged))
}

// CONTRACT: Returned ValidatorsInfo can be mutated.
func loadValidatorsInfo(db kaidb.Database, height uint64) *kstate.ValidatorsInfo {
	buf, err := db.Get(calcValidatorsKey(height))
	if err != nil {
		panic(err)
	}
	if len(buf) == 0 {
		return nil
	}

	v := new(kstate.ValidatorsInfo)
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
	valInfo := &kstate.ValidatorsInfo{
		LastHeightChanged: lastHeightChanged,
	}

	if height == lastHeightChanged || height%valSetCheckpointInterval == 0 {
		pv, err := valSet.ToProto()
		if err != nil {
			panic(err)
		}
		valInfo.ValidatorSet = pv
	}
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
func (s *dbStore) LoadConsensusParams(height uint64) (kproto.ConsensusParams, error) {
	empty := kproto.ConsensusParams{}

	paramsInfo, err := loadConsensusParamsInfo(s.db, height)
	if err != nil {
		return empty, fmt.Errorf("could not find consensus params for height #%d: %w", height, err)
	}

	if paramsInfo.ConsensusParams.Equal(&empty) {
		paramsInfo2, err := loadConsensusParamsInfo(s.db, paramsInfo.LastHeightChanged)
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

func loadConsensusParamsInfo(db kaidb.Database, height uint64) (*kstate.ConsensusParamsInfo, error) {
	buf, err := db.Get(calcConsensusParamsKey(uint64(height)))
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, nil
	}

	paramsInfo := new(kstate.ConsensusParamsInfo)
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
	paramsInfo := &kstate.ConsensusParamsInfo{
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
func MakeGenesisState(genDoc *genesis.Genesis) (LatestBlockState, error) {
	if genDoc.InitialHeight == 0 {
		genDoc.InitialHeight = 1
	}

	var validatorSet, nextValidatorSet *types.ValidatorSet
	if genDoc.Validators == nil {
		validatorSet = nil
		nextValidatorSet = nil
	} else {
		var validators []*types.Validator
		for _, val := range genDoc.Validators {
			// in genesis state, only start those validators whose StartWithGenesis flag is true
			if val.StartWithGenesis {
				tokens, _ := big.NewInt(0).SetString(val.SelfDelegate, 10)
				power := tokens.Div(tokens, configs.PowerReduction)
				validators = append(validators, types.NewValidator(common.HexToAddress(val.Address), power.Int64()))
			}
		}
		validatorSet = types.NewValidatorSet(validators)
		nextValidatorSet = types.NewValidatorSet(validators).CopyIncrementProposerPriority(1)
	}
	return LatestBlockState{
		InitialHeight:   genDoc.InitialHeight,
		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastBlockTime:   genDoc.Timestamp,

		NextValidators:              nextValidatorSet,
		Validators:                  validatorSet,
		LastValidators:              nil,
		LastHeightValidatorsChanged: genDoc.InitialHeight,

		ConsensusParams:                  *genDoc.ConsensusParams,
		LastHeightConsensusParamsChanged: genDoc.InitialHeight,
	}, nil
}
