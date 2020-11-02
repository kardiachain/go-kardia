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
	"math/big"

	fail "github.com/ebuchman/fail-test"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/staking"
	"github.com/kardiachain/go-kardiamain/types"
)

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
type EvidencePool interface {
	Update(LastestBlockState)
	VMEvidence(height uint64, evidence []types.Evidence) []staking.Evidence
	CheckEvidence(evList types.EvidenceList) error
}

// BlockStore ...
type BlockStore interface {
	CommitAndValidateBlockTxs(*types.Block, staking.LastCommitInfo, []staking.Evidence) ([]*types.Validator, common.Hash, error)
}

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	evpool   EvidencePool
	bc       BlockStore
	store    Store
	eventBus *types.EventBus
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(stateStore Store, evpool EvidencePool, bc BlockStore) *BlockExecutor {
	return &BlockExecutor{
		evpool: evpool,
		bc:     bc,
		store:  stateStore,
	}
}

// SetEventBus sets event bus.
func (blockExec *BlockExecutor) SetEventBus(b *types.EventBus) {
	blockExec.eventBus = b
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(state LastestBlockState, block *types.Block) error {
	return validateBlock(blockExec.evpool, blockExec.store, state, block)
}

// ApplyBlock Validates the block against the state, and saves the new state.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(logger log.Logger, state LastestBlockState, blockID types.BlockID, block *types.Block) (LastestBlockState, error) {
	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}
	fail.Fail() // XXX

	byzVals := blockExec.evpool.VMEvidence(block.Height(), block.Evidence().Evidence)
	commitInfo := getBeginBlockValidatorInfo(block, blockExec.store)

	valUpdates, appHash, err := blockExec.bc.CommitAndValidateBlockTxs(block, commitInfo, byzVals)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %v", err)
	}

	valUpdates = calculateValidatorSetUpdates(state.Validators.Validators, valUpdates)
	// update the state with the block and responses
	state, err = updateState(logger, state, blockID, block.Header(), valUpdates)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	state.AppHash = appHash
	blockExec.store.Save(state)

	// Update evpool with the block and state.
	blockExec.evpool.Update(state)
	fail.Fail() // XXX
	// Events are fired after everything else.
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	fireEvents(logger, blockExec.eventBus, block, valUpdates)
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
		lastHeightValsChanged = header.Height + 2
		err := nValSet.UpdateWithChangeSet(validatorUpdates)
		if err != nil {
			return state, fmt.Errorf("error changing validator set: %v", err)
		}

	}
	nValSet.IncrementProposerPriority(1)
	return LastestBlockState{
		ChainID:                     state.ChainID,
		LastBlockHeight:             header.Height,
		LastBlockID:                 blockID,
		LastBlockTime:               header.Time,
		NextValidators:              nValSet,
		Validators:                  state.NextValidators.Copy(),
		LastValidators:              state.Validators.Copy(),
		LastHeightValidatorsChanged: lastHeightValsChanged,
		ConsensusParams:             state.ConsensusParams,
	}, nil
}

func getBeginBlockValidatorInfo(b *types.Block, store Store) staking.LastCommitInfo {
	lastCommit := b.LastCommit()
	voteInfos := make([]staking.VoteInfo, lastCommit.Size())
	// block.Height=1 -> LastCommitInfo.Votes are empty.
	// Remember that the first LastCommit is intentionally empty, so it makes
	// sense for LastCommitInfo.Votes to also be empty.
	if b.Height() > 1 {
		lastValSet, err := store.LoadValidators(b.Height() - 1)
		if err != nil {
			panic(err)
		}

		// Sanity check that commit size matches validator set size - only applies
		// after first block.
		var (
			commitSize = lastCommit.Size()
			valSetLen  = len(lastValSet.Validators)
		)
		if commitSize != valSetLen {
			panic(fmt.Sprintf("commit size (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				commitSize, valSetLen, b.Height(), lastCommit.Signatures, lastValSet.Validators))
		}

		for i, val := range lastValSet.Validators {
			commitSig := lastCommit.Signatures[i]
			voteInfos[i] = staking.VoteInfo{
				Address:         val.Address,
				VotingPower:     big.NewInt(int64(val.VotingPower)),
				SignedLastBlock: commitSig.Signature != nil,
			}
		}
	}

	return staking.LastCommitInfo{
		Votes: voteInfos,
	}
}

func calculateValidatorSetUpdates(lastVals []*types.Validator, vals []*types.Validator) (updates []*types.Validator) {
	if len(vals) == 0 {
		return
	}
	last := make(map[common.Address]uint64)
	for _, validator := range lastVals {
		last[validator.Address] = validator.VotingPower
	}

	for _, val := range vals {
		oldPower, found := last[val.Address]
		if !found || oldPower != val.VotingPower {
			updates = append(updates, val)
		}
		delete(last, val.Address)
	}

	for valAddr := range last {
		updates = append(updates, &types.Validator{
			Address:     valAddr,
			VotingPower: 0,
		})
	}
	return updates
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(
	logger log.Logger,
	eventBus types.BlockEventPublisher,
	block *types.Block,
	validatorUpdates []*types.Validator,
) {
	if err := eventBus.PublishEventNewBlock(types.EventDataNewBlock{
		Block: block,
	}); err != nil {
		logger.Error("Error publishing new block", "err", err)
	}

	if err := eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header: block.Header(),
	}); err != nil {
		logger.Error("Error publishing new block header", "err", err)
	}
}
