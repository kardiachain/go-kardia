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

package cstate

import (
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/kardiachain/go-kardia/lib/common"

	kstate "github.com/kardiachain/go-kardia/proto/kardiachain/state"
	kproto "github.com/kardiachain/go-kardia/proto/kardiachain/types"
	"github.com/kardiachain/go-kardia/types"
	ktime "github.com/kardiachain/go-kardia/types/time"
)

// TODO(namdoh): Move to a common config file.
var (
	RefreshBackoffHeightStep = int64(200)
	RefreshHeightDelta       = int64(20)
	stateKey                 = []byte("stateKey")
)

// LatestBlockState It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type LatestBlockState struct {
	// Immutable
	ChainID       string
	InitialHeight uint64 // should be 1, not 0, when starting from height 1

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight  uint64
	LastBlockTotalTx uint64
	LastBlockID      types.BlockID
	LastBlockTime    time.Time

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	NextValidators              *types.ValidatorSet `rlp:"nil"`
	Validators                  *types.ValidatorSet `rlp:"nil"`
	LastValidators              *types.ValidatorSet `rlp:"nil"`
	LastHeightValidatorsChanged uint64

	LastHeightConsensusParamsChanged uint64
	AppHash                          common.Hash
	ConsensusParams                  kproto.ConsensusParams
}

// Copy makes a copy of the State for mutating.
func (state LatestBlockState) Copy() LatestBlockState {
	return LatestBlockState{
		ChainID:       state.ChainID,
		InitialHeight: state.InitialHeight,

		LastBlockHeight:  state.LastBlockHeight,
		LastBlockTotalTx: state.LastBlockTotalTx,
		LastBlockID:      state.LastBlockID,
		LastBlockTime:    state.LastBlockTime,

		NextValidators:              state.NextValidators.Copy(),
		Validators:                  state.Validators.Copy(),
		LastValidators:              state.LastValidators.Copy(),
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,
		AppHash:                     state.AppHash,
		ConsensusParams:             state.ConsensusParams,
	}
}

// IsEmpty returns true if the State is equal to the empty State.
func (state LatestBlockState) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}

// Stringshort returns a short string representing State
func (state LatestBlockState) String() string {
	return fmt.Sprintf("{ChainID:%v LastBlockHeight:%v LastBlockTotalTx:%v LastBlockID:%v LastBlockTime:%v Validators:%v LastValidators:%v LastHeightValidatorsChanged:%v",
		state.ChainID, state.LastBlockHeight, state.LastBlockTotalTx, state.LastBlockID, state.LastBlockTime,
		state.Validators, state.LastValidators, state.LastHeightValidatorsChanged)
}

// Bytes ...
func (state *LatestBlockState) Bytes() []byte {
	sm, err := state.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(sm)
	if err != nil {
		panic(err)
	}
	return bz
}

// ToProto takes the local state type and returns the equivalent proto type
func (state *LatestBlockState) ToProto() (*kstate.State, error) {
	if state == nil {
		return nil, ErrNilState
	}

	sm := new(kstate.State)

	//sm.Version = state.Version
	sm.ChainID = state.ChainID
	sm.InitialHeight = state.InitialHeight
	sm.LastBlockHeight = state.LastBlockHeight

	sm.LastBlockID = state.LastBlockID.ToProto()
	sm.LastBlockTime = state.LastBlockTime
	vals, err := state.Validators.ToProto()
	if err != nil {
		return nil, err
	}
	sm.Validators = vals

	nVals, err := state.NextValidators.ToProto()
	if err != nil {
		return nil, err
	}
	sm.NextValidators = nVals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := state.LastValidators.ToProto()
		if err != nil {
			return nil, err
		}
		sm.LastValidators = lVals
	}

	sm.LastHeightValidatorsChanged = state.LastHeightValidatorsChanged
	sm.ConsensusParams = state.ConsensusParams
	sm.LastHeightConsensusParamsChanged = state.LastHeightConsensusParamsChanged
	sm.AppHash = state.AppHash.Bytes()

	return sm, nil
}

// StateFromProto takes a state proto message & returns the local state type
func StateFromProto(pb *kstate.State) (*LatestBlockState, error) { //nolint:golint
	if pb == nil {
		return nil, ErrNilState
	}

	state := new(LatestBlockState)

	state.ChainID = pb.ChainID
	state.InitialHeight = pb.InitialHeight

	bi, err := types.BlockIDFromProto(&pb.LastBlockID)
	if err != nil {
		return nil, err
	}
	state.LastBlockID = *bi
	state.LastBlockHeight = pb.LastBlockHeight
	state.LastBlockTime = pb.LastBlockTime

	vals, err := types.ValidatorSetFromProto(pb.Validators)
	if err != nil {
		return nil, err
	}
	state.Validators = vals

	nVals, err := types.ValidatorSetFromProto(pb.NextValidators)
	if err != nil {
		return nil, err
	}
	state.NextValidators = nVals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := types.ValidatorSetFromProto(pb.LastValidators)
		if err != nil {
			return nil, err
		}
		state.LastValidators = lVals
	} else {
		state.LastValidators = types.NewValidatorSet(nil)
	}

	state.LastHeightValidatorsChanged = pb.LastHeightValidatorsChanged
	state.ConsensusParams = pb.ConsensusParams
	state.LastHeightConsensusParamsChanged = pb.LastHeightConsensusParamsChanged
	state.AppHash = common.BytesToHash(pb.AppHash)

	return state, nil
}

// MedianTime computes a median time for a given Commit (based on Timestamp field of votes messages) and the
// corresponding validator set. The computed time is always between timestamps of
// the votes sent by honest processes, i.e., a faulty processes can not arbitrarily increase or decrease the
// computed value.
func MedianTime(commit *types.Commit, validators *types.ValidatorSet) time.Time {
	weightedTimes := make([]*ktime.WeightedTime, len(commit.Signatures))
	totalVotingPower := int64(0)

	for i, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue
		}
		_, validator := validators.GetByAddress(commitSig.ValidatorAddress)
		// If there's no condition, TestValidateBlockCommit panics; not needed normally.
		if validator != nil {
			votingPower := int64(validator.VotingPower)
			totalVotingPower += votingPower
			weightedTimes[i] = ktime.NewWeightedTime(commitSig.Timestamp, votingPower)
		}
	}

	return ktime.WeightedMedian(weightedTimes, totalVotingPower)
}
