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

	"github.com/kardiachain/go-kardiamain/lib/common"

	fail "github.com/ebuchman/fail-test"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/types"
)

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
type EvidencePool interface {
	PendingEvidence(int64) []types.Evidence
	Update(*types.Block, LastestBlockState)
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func ValidateBlock(state LastestBlockState, block *types.Block) error {
	return validateBlock(state, block)
}

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	evpool EvidencePool
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(evpool EvidencePool) *BlockExecutor {
	return &BlockExecutor{
		evpool: evpool,
	}
}

// ApplyBlock Validates the block against the state, and saves the new state.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(logger log.Logger, state LastestBlockState, blockID types.BlockID, block *types.Block) (LastestBlockState, error) {
	if err := ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	fail.Fail() // XXX

	// update the state with the block and responses
	var err error
	state, err = updateState(logger, state, blockID, block.Header(), nil)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	logger.Warn("Update evidence pool.")
	// Update evpool with the block and state.
	blockExec.evpool.Update(block, state)
	fail.Fail() // XXX

	return state, nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(logger log.Logger, state LastestBlockState, blockID types.BlockID, header *types.Header, validatorUpdates []*types.Validator) (LastestBlockState, error) {
	logger.Trace("updateState", "state", state, "blockID", blockID, "header", header)
	// Copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators.
	nValSet := state.NextValidators.Copy()

	// Update the validator set with the latest abciResponses
	lastHeightValsChanged := state.LastHeightValidatorsChanged

	if len(validatorUpdates) > 0 {
		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = common.NewBigUint64(header.Height + 2)
	}
	nValSet.AdvanceProposer(1)
	return LastestBlockState{
		ChainID:                     state.ChainID,
		LastBlockHeight:             common.NewBigUint64(header.Height),
		LastBlockID:                 blockID,
		LastBlockTime:               header.Time.Uint64(),
		NextValidators:              nValSet,
		Validators:                  state.NextValidators.Copy(),
		LastValidators:              state.Validators.Copy(),
		LastHeightValidatorsChanged: lastHeightValsChanged,
	}, nil
}
