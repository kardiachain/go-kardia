package state

import (
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
	log.Error("BlockExecutor.ApplyBlock - Not yet implemented")
	return state, nil
	/*
		if err := blockExec.ValidateBlock(state, block); err != nil {
			return state, ErrInvalidBlock(err)
		}

		abciResponses, err := execBlockOnProxyApp(blockExec.logger, blockExec.proxyApp, block, state.LastValidators, blockExec.db)
		if err != nil {
			return state, ErrProxyAppConn(err)
		}

		fail.Fail() // XXX

		// save the results before we commit
		saveABCIResponses(blockExec.db, block.Height, abciResponses)

		fail.Fail() // XXX

		// update the state with the block and responses
		state, err = updateState(state, blockID, &block.Header, abciResponses)
		if err != nil {
			return state, fmt.Errorf("Commit failed for application: %v", err)
		}

		// lock mempool, commit app state, update mempoool
		appHash, err := blockExec.Commit(block)
		if err != nil {
			return state, fmt.Errorf("Commit failed for application: %v", err)
		}

		// Update evpool with the block and state.
		blockExec.evpool.Update(block, state)

		fail.Fail() // XXX

		// update the app hash and save the state
		state.AppHash = appHash
		SaveState(blockExec.db, state)

		fail.Fail() // XXX

		// events are fired after everything else
		// NOTE: if we crash between Commit and Save, events wont be fired during replay
		fireEvents(blockExec.logger, blockExec.eventBus, block, abciResponses)

		return state, nil
	*/
}
