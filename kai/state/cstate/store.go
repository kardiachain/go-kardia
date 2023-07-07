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

	"github.com/kardiachain/go-kardia/mainchain/genesis"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"

	"github.com/kardiachain/go-kardia/types"

	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/kai/rawdb"
	kstate "github.com/kardiachain/go-kardia/proto/kardiachain/state"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
)

const (
	// persist validators every valSetCheckpointInterval blocks to avoid
	// LoadValidators taking too much time.
	valSetCheckpointInterval          = 100000
	PRUNE_LATEST_BLOCK_STATE_INTERVAL = 10000
)

type Store interface {
	LoadStateFromDBOrGenesisDoc(genesisDoc *genesis.Genesis) (LatestBlockState, error)
	Load() LatestBlockState
	Save(LatestBlockState)
	LoadValidators(height uint64) (*types.ValidatorSet, error)
	LoadConsensusParams(height uint64) (kproto.ConsensusParams, error)
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
	saveState(s.db, state)

	// Starting from the 2nd interval, we try to prune
	// from beginning of previous interval
	// to the end of previous interval
	if state.LastBlockHeight%PRUNE_LATEST_BLOCK_STATE_INTERVAL == 0 && state.LastBlockHeight/PRUNE_LATEST_BLOCK_STATE_INTERVAL > 1 {
		to := state.LastBlockHeight - PRUNE_LATEST_BLOCK_STATE_INTERVAL
		from := to - PRUNE_LATEST_BLOCK_STATE_INTERVAL

		log.Info("Pruning consensus state", "from", from, "to", to)
		pruneState(s.db, from, to)
	}
}

func saveState(db kaidb.KeyValueStore, state LatestBlockState) {
	sp, err := state.ToProto()
	if err != nil {
		panic(fmt.Sprintf(`Failed to marshal state to proto: %v\n`, err))
	}

	batch := db.NewBatch()

	// write validators for block #0
	if state.LastBlockHeight == 0 {
		sp.LastValidatorsInfoHash = saveValidatorsInfo(batch, state.LastHeightValidatorsChanged, state.LastValidators).Bytes()
		sp.ValidatorsInfoHash = saveValidatorsInfo(batch, state.LastHeightValidatorsChanged, state.Validators).Bytes()
	}

	// write next validators
	sp.NextValidatorsInfoHash = saveValidatorsInfo(batch, state.LastHeightValidatorsChanged, state.NextValidators).Bytes()

	// write consensus params
	sp.ConsensusParamsInfoHash = saveConsensusParamsInfo(batch, state.LastHeightConsensusParamsChanged, state.ConsensusParams).Bytes()

	if metrics.EnabledExpensive {
		bz, _ := sp.Marshal()
		consensusStateWrittenBytesGauge.Inc(int64(len(bz)))
	}

	rawdb.WriteConsensusStateHeight(batch, state.LastBlockHeight, *sp)

	batch.Write()
}

func pruneState(db kaidb.KeyValueStore, from, to uint64) {
	valInfosCache := make(map[common.Hash]struct{}, 0) // map of val info hash -> last height validator changed
	for i := from; i < to; i++ {
		state := rawdb.ReadConsensusStateHeight(db, i)
		if state != nil {
			valInfoHash := common.BytesToHash(state.LastValidatorsInfoHash)
			valInfosCache[valInfoHash] = struct{}{}
		}
		if metrics.EnabledExpensive {
			bz, _ := state.Marshal()
			consensusStatePrunedBytesGauge.Inc(int64(len(bz)))
		}
		if err := rawdb.DeleteConsensusStateHeight(db, i); err != nil {
			log.Error("Failed to prune consensus state", "height", i)
		}
	}

	// discards validator infos which are used by next state
	nextState := rawdb.ReadConsensusStateHeight(db, to)
	if nextState != nil {
		delete(valInfosCache, common.BytesToHash(nextState.LastValidatorsInfoHash))
		delete(valInfosCache, common.BytesToHash(nextState.ValidatorsInfoHash))
		delete(valInfosCache, common.BytesToHash(nextState.NextValidatorsInfoHash))
	}

	// delete val infos
	for valInfoHash := range valInfosCache {
		if metrics.EnabledExpensive {
			valInfo := rawdb.ReadConsensusValidatorsInfo(db, valInfoHash)
			if valInfo != nil {
				bz, _ := valInfo.Marshal()
				consensusStatePrunedBytesGauge.Inc(int64(len(bz)))
			}
		}
		if err := rawdb.DeleteConsensusValidatorsInfo(db, valInfoHash); err != nil {
			log.Error("Failed to prune consensus validator info", "hash", valInfoHash)
		}
	}
}

// LoadState loads the State from the database.
func (s *dbStore) Load() LatestBlockState {
	head := rawdb.ReadHeadBlock(s.db)
	if state := loadStateAtHeight(s.db, head.Height()); state != nil {
		return *state
	}

	return LatestBlockState{}
}

func loadStateAtHeight(db kaidb.Database, height uint64) *LatestBlockState {
	sp := rawdb.ReadConsensusStateHeight(db, height)
	if sp == nil {
		return nil
	}

	state, err := StateFromProto(sp)
	if err != nil {
		panic(err)
	}

	if state.InitialHeight == 0 {
		state.InitialHeight = 1
	}

	blockMeta := rawdb.ReadBlockMeta(db, height)
	if blockMeta == nil {
		panic(fmt.Errorf(`block meta not found at height %v`, height))
	}
	state.LastBlockHeight = blockMeta.Header.Height
	state.LastBlockID = blockMeta.BlockID
	state.LastBlockTime = blockMeta.Header.Time
	state.LastBlockTotalTx = blockMeta.Header.NumTxs

	appHash := rawdb.ReadAppHash(db, height)
	state.AppHash = appHash

	lValsInfo := rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(sp.LastValidatorsInfoHash))
	if state.LastBlockHeight > 0 {
		state.LastValidators, err = types.ValidatorSetFromProto(lValsInfo.ValidatorSet)
		if err != nil {
			panic(err)
		}
	}

	valsInfo := rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(sp.ValidatorsInfoHash))
	state.Validators, err = types.ValidatorSetFromProto(valsInfo.ValidatorSet)
	if err != nil {
		panic(err)
	}

	nValsInfo := rawdb.ReadConsensusValidatorsInfo(db, common.BytesToHash(sp.NextValidatorsInfoHash))
	state.NextValidators, err = types.ValidatorSetFromProto(nValsInfo.ValidatorSet)
	if err != nil {
		panic(err)
	}
	state.LastHeightValidatorsChanged = nValsInfo.LastHeightChanged

	cparams := rawdb.ReadConsensusParamsInfo(db, common.BytesToHash(sp.ConsensusParamsInfoHash))
	if cparams == nil {
		panic(fmt.Errorf(`failed to load consensus params at height %v`, height))
	}
	state.ConsensusParams = cparams.ConsensusParams
	state.LastHeightConsensusParamsChanged = cparams.LastHeightChanged

	return state
}

// LoadValidators loads the ValidatorSet for a given height.
// Returns ErrNoValSetForHeight if the validator set can't be found for this height.
func (s *dbStore) LoadValidators(height uint64) (*types.ValidatorSet, error) {
	cstate := rawdb.ReadConsensusStateHeight(s.db, height)

	valInfo := rawdb.ReadConsensusValidatorsInfo(s.db, common.BytesToHash(cstate.LastValidatorsInfoHash))
	if valInfo == nil {
		return nil, ErrNoValSetForHeight{height}
	}

	vip, err := types.ValidatorSetFromProto(valInfo.ValidatorSet)
	if err != nil {
		return nil, err
	}
	return vip, nil
}

func saveValidatorsInfo(db kaidb.KeyValueWriter, lastHeightChanged uint64, valSet *types.ValidatorSet) common.Hash {
	valInfo := kstate.ValidatorsInfo{
		LastHeightChanged: lastHeightChanged,
	}
	hash := common.NewZeroHash()
	if valSet != nil {
		hash = valSet.Hash()
		pv, err := valSet.ToProto()
		if err != nil {
			panic(err)
		}
		valInfo.ValidatorSet = pv
	}

	if metrics.EnabledExpensive {
		bz, _ := valInfo.Marshal()
		consensusStateWrittenBytesGauge.Inc(int64(len(bz)))
	}

	rawdb.WriteConsensusValidatorsInfo(db, hash, valInfo)
	return hash
}

// LoadConsensusParams loads the ConsensusParams for a given height.
func (s *dbStore) LoadConsensusParams(height uint64) (kproto.ConsensusParams, error) {
	cstate := rawdb.ReadConsensusStateHeight(s.db, height)

	params := rawdb.ReadConsensusParamsInfo(s.db, common.BytesToHash(cstate.ConsensusParamsInfoHash))
	if params == nil {
		return kproto.ConsensusParams{}, fmt.Errorf("could not find consensus params for height #%d", height)
	} else {
		return params.ConsensusParams, nil
	}
}

func saveConsensusParamsInfo(db kaidb.KeyValueWriter, lastHeightChanged uint64, params kproto.ConsensusParams) common.Hash {
	paramsInfo := kstate.ConsensusParamsInfo{
		LastHeightChanged: lastHeightChanged,
		ConsensusParams:   params,
	}

	bz, err := paramsInfo.Marshal()
	if err != nil {
		panic(err)
	}

	if metrics.EnabledExpensive {
		consensusStateWrittenBytesGauge.Inc(int64(len(bz)))
	}

	hash := common.BytesToHash(bz)
	rawdb.WriteConsensusParamsInfo(db, hash, paramsInfo)
	return hash
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
