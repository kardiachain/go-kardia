package state

import (
	"fmt"

	fail "github.com/ebuchman/fail-test"
	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/types"
)

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	// TODO(namdoh): Save state, validators, consensus params in db.
	//db dbm.DB

	// events
	eventBus types.BlockEventPublisher

	// update these with block results after commit
	//namdoh@ mempool Mempool
	evpool EvidencePool

	logger log.Logger
}

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
type EvidencePool interface {
	PendingEvidence() []types.Evidence
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(state LastestBlockState, block *types.Block) error {
	return validateBlock(state, block)
}

// ApplyBlock validates the block against the state, executes it against the app,
// fires the relevant events, commits the app, and saves the new state and responses.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(state LastestBlockState, blockID types.BlockID, block *types.Block) (LastestBlockState, error) {
	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	//namdoh@ abciResponses, err := execBlockOnProxyApp(blockExec.logger, blockExec.proxyApp, block, state.LastValidators, blockExec.db)
	//namdoh@ if err != nil {
	//namdoh@ 	return state, ErrProxyAppConn(err)
	//namdoh@ }

	fail.Fail() // XXX

	// save the results before we commit
	//namdoh@ saveABCIResponses(blockExec.db, block.Height, abciResponses)

	fail.Fail() // XXX

	// update the state with the block and responses
	var err error
	state, err = updateState(state, blockID, block.Header())
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	// lock mempool, commit app state, update mempoool
	//namdoh@ appHash, err := blockExec.Commit(block)
	//namdoh@ if err != nil {
	//namdoh@ 	return state, fmt.Errorf("Commit failed for application: %v", err)
	//namdoh@ }

	// Update evpool with the block and state.
	//namdoh@ blockExec.evpool.Update(block, state)

	fail.Fail() // XXX

	// events are fired after everything else
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	//namdoh@ fireEvents(blockExec.logger, blockExec.eventBus, block, abciResponses)

	return state, nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(state LastestBlockState, blockID types.BlockID, header *types.Header) (LastestBlockState, error) {
	log.Trace("updateState", "state", state, "blockID", blockID, "header", header)

	// copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators
	nextValSet := state.Validators.Copy()

	// update the validator set with the latest abciResponses
	lastHeightValsChanged := state.LastHeightValidatorsChanged

	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)

	var totalTx *cmn.BigInt
	if state.LastBlockTotalTx == nil {
		totalTx = nil
	} else {
		totalTx = state.LastBlockTotalTx.Add(int64(header.NumTxs))
	}

	return LastestBlockState{
		ChainID:                     state.ChainID,
		LastBlockHeight:             cmn.NewBigInt(int64(header.Height)),
		LastBlockTotalTx:            totalTx,
		LastBlockID:                 blockID,
		LastBlockTime:               header.Time,
		Validators:                  nextValSet,
		LastValidators:              state.Validators.Copy(),
		LastHeightValidatorsChanged: lastHeightValsChanged,
	}, nil
}
