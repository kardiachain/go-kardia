package state

import (
	"time"

	cmn "github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

// It keeps all information necessary to validate new blocks,
// including the last validator set and the consensus params.
// All fields are exposed so the struct can be easily serialized,
// but none of them should be mutated directly.
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type LastestBlockState struct {
	// Immutable
	ChainID string

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	LastBlockHeight  *cmn.BigInt
	LastBlockTotalTx *cmn.BigInt
	LastBlockID      types.BlockID
	LastBlockTime    time.Time

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged *cmn.BigInt

	// TODO(namdoh): Add consensus parameters used for validating blocks.

	// Merkle root of the results from executing prev block
	//namdoh@ LastResultsHash []byte

	// The latest AppHash we've received from calling abci.Commit()
	//namdoh@ AppHash []byte
}

// Copy makes a copy of the State for mutating.
func (state LastestBlockState) Copy() LastestBlockState {
	return LastestBlockState{
		ChainID: state.ChainID,

		LastBlockHeight:  state.LastBlockHeight,
		LastBlockTotalTx: state.LastBlockTotalTx,
		LastBlockID:      state.LastBlockID,
		LastBlockTime:    state.LastBlockTime,

		Validators:                  state.Validators.Copy(),
		LastValidators:              state.LastValidators.Copy(),
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,

		//namdoh@ AppHash: state.AppHash,

		//namdoh@ LastResultsHash: state.LastResultsHash,
	}
}

// Creates a block from the latest state.
// MakeBlock builds a block with the given txs and commit from the current state.
func (state LastestBlockState) MakeBlock(height int64, txs []*types.Transaction, commit *types.Commit, accounts *types.AccountStates) *types.Block {
	// build base block
	// TODO(huny@): Fill receipt in making a new block.
	header := types.Header{
		// ChainID: state.ChainID, TODO(huny/namdoh): confims that ChainID is replaced by network id.
		Height:         uint64(height),
		Time:           time.Now(),
		NumTxs:         uint64(len(txs)),
		LastBlockID:    state.LastBlockID,
		ValidatorsHash: state.Validators.Hash(),
	}
	block := types.NewBlock(&header, txs, nil, commit, accounts)

	// TODO(namdoh): Fill the missing header info: AppHash, ConsensusHash,
	// LastResultHash.

	return block
}

// IsEmpty returns true if the State is equal to the empty State.
func (state LastestBlockState) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}
