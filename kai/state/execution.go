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

	fail "github.com/ebuchman/fail-test"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
type EvidencePool interface {
	PendingEvidence() []types.Evidence
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func ValidateBlock(state LastestBlockState, block *types.Block) error {
	return validateBlock(state, block)
}

// Validates the block against the state, and saves the new state.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func ApplyBlock(logger log.Logger, state LastestBlockState, blockID types.BlockID, block *types.Block) (LastestBlockState, error) {
	if err := ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	fail.Fail() // XXX

	// update the state with the block and responses
	var err error
	state, err = updateState(logger, state, blockID, block.Header())
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	logger.Warn("Update evidence pool.")
	fail.Fail() // XXX

	return state, nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(logger log.Logger, state LastestBlockState, blockID types.BlockID, header *types.Header) (LastestBlockState, error) {
	logger.Trace("updateState", "state", state, "blockID", blockID, "header", header)

	// copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators
	// TODO(namdoh@): Refactor to set next set of valiator here.
	nextValSet := state.Validators.Copy()

	// update the validator set with the latest abciResponses
	lastHeightValsChanged := state.LastHeightValidatorsChanged

	// Update validator accums and set state variables
	currentHeight := int64(header.Height)
	nextValSet.AdvanceProposer(currentHeight+1, currentHeight)

	var totalTx *cmn.BigInt
	if state.LastBlockTotalTx == nil {
		totalTx = nil
	} else {
		totalTx = state.LastBlockTotalTx.AddUint64(header.NumTxs)
	}

	return LastestBlockState{
		ChainID:                     state.ChainID,
		LastBlockHeight:             cmn.NewBigUint64(header.Height),
		LastBlockTotalTx:            totalTx,
		LastBlockID:                 blockID,
		LastBlockTime:               header.Time,
		Validators:                  nextValSet,
		LastValidators:              state.Validators.Copy(),
		LastHeightValidatorsChanged: lastHeightValsChanged,
	}, nil
}
