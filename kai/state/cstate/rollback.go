package cstate

import (
	"errors"
	"fmt"

	"github.com/kardiachain/go-kardia/lib/common"
)

// Rollback overwrites the current Tendermint state (height n) with the most
// recent previous state (height n - 1).
// Note that this function does not affect application state.
func Rollback(bs BlockStore, ss Store) (uint64, common.Hash, error) {
	invalidState := ss.Load()
	if invalidState.IsEmpty() {
		return 0, common.Hash{}, errors.New("no state found")
	}

	height := bs.Height()

	currentBlock := bs.LoadBlockMeta(height)

	// NOTE: persistence of state and blocks don't happen atomically. Therefore it is possible that
	// when the user stopped the node the state wasn't updated but the blockstore was. In this situation
	// we don't need to rollback any state and can just return early
	if height == invalidState.LastBlockHeight+1 {
		return invalidState.LastBlockHeight, invalidState.AppHash, nil
	}

	// If the state store isn't one below nor equal to the blockstore height than this violates the
	// invariant
	if height != invalidState.LastBlockHeight {
		return 0, common.Hash{}, fmt.Errorf("statestore height (%d) is not one below or equal to blockstore height (%d)",
			invalidState.LastBlockHeight, height)
	}

	// state store height is equal to blockstore height. We're good to proceed with rolling back state
	rollbackHeight := invalidState.LastBlockHeight - 1
	rollbackBlock := bs.LoadBlockMeta(rollbackHeight)
	if rollbackBlock == nil {
		return 0, common.Hash{}, fmt.Errorf("block at height %d not found", rollbackHeight)
	}

	previousLastValidatorSet, err := ss.LoadValidators(rollbackHeight)
	if err != nil {
		return 0, common.Hash{}, err
	}

	valChangeHeight := invalidState.LastHeightValidatorsChanged
	// this can only happen if the validator set changed since the last block
	if valChangeHeight > rollbackHeight {
		valChangeHeight = rollbackHeight + 1
	}

	// build the new state from the old state and the prior block
	rolledBackState := LatestBlockState{
		// immutable fields
		ChainID:       invalidState.ChainID,
		InitialHeight: invalidState.InitialHeight,

		LastBlockHeight: rollbackBlock.Header.Height,
		LastBlockID:     rollbackBlock.BlockID,
		LastBlockTime:   rollbackBlock.Header.Time,

		NextValidators:              invalidState.Validators,
		Validators:                  invalidState.LastValidators,
		LastValidators:              previousLastValidatorSet,
		LastHeightValidatorsChanged: valChangeHeight,

		AppHash: currentBlock.Header.AppHash,
	}
	bs.WriteHeadBlockHash(rollbackBlock.BlockID.Hash)
	// persist the new state. This overrides the invalid one. NOTE: this will also
	// persist the validator set and consensus params over the existing structures,
	// but both should be the same
	ss.Save(rolledBackState)
	return rolledBackState.LastBlockHeight, rolledBackState.AppHash, nil
}
