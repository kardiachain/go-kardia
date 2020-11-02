// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	cstate "github.com/kardiachain/go-kardiamain/kai/state/cstate"
	genesis "github.com/kardiachain/go-kardiamain/mainchain/genesis"

	go_kardiamaintypes "github.com/kardiachain/go-kardiamain/types"

	mock "github.com/stretchr/testify/mock"

	types "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// Load provides a mock function with given fields:
func (_m *Store) Load() cstate.LastestBlockState {
	ret := _m.Called()

	var r0 cstate.LastestBlockState
	if rf, ok := ret.Get(0).(func() cstate.LastestBlockState); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(cstate.LastestBlockState)
	}

	return r0
}

// LoadConsensusParams provides a mock function with given fields: height
func (_m *Store) LoadConsensusParams(height uint64) (types.ConsensusParams, error) {
	ret := _m.Called(height)

	var r0 types.ConsensusParams
	if rf, ok := ret.Get(0).(func(uint64) types.ConsensusParams); ok {
		r0 = rf(height)
	} else {
		r0 = ret.Get(0).(types.ConsensusParams)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(uint64) error); ok {
		r1 = rf(height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LoadStateFromDBOrGenesisDoc provides a mock function with given fields: genesisDoc
func (_m *Store) LoadStateFromDBOrGenesisDoc(genesisDoc *genesis.Genesis) (cstate.LastestBlockState, error) {
	ret := _m.Called(genesisDoc)

	var r0 cstate.LastestBlockState
	if rf, ok := ret.Get(0).(func(*genesis.Genesis) cstate.LastestBlockState); ok {
		r0 = rf(genesisDoc)
	} else {
		r0 = ret.Get(0).(cstate.LastestBlockState)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*genesis.Genesis) error); ok {
		r1 = rf(genesisDoc)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LoadValidators provides a mock function with given fields: height
func (_m *Store) LoadValidators(height uint64) (*go_kardiamaintypes.ValidatorSet, error) {
	ret := _m.Called(height)

	var r0 *go_kardiamaintypes.ValidatorSet
	if rf, ok := ret.Get(0).(func(uint64) *go_kardiamaintypes.ValidatorSet); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*go_kardiamaintypes.ValidatorSet)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(uint64) error); ok {
		r1 = rf(height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Save provides a mock function with given fields: _a0
func (_m *Store) Save(_a0 cstate.LastestBlockState) {
	_m.Called(_a0)
}
