package state

import (
	"github.com/kardiachain/go-kardia/log"
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
