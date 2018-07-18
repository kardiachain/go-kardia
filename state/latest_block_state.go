package states

import (
	"bytes"
	"time"
	
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
	LastBlockHeight  int64
	LastBlockTotalTx int64
	LastBlockID      types.BlockID
	LastBlockTime    time.Time

	// TODO(namdoh): Add consensus parameters used for validating blocks.
	// Changes returned by EndBlock and updated after Commit.
	LastHeightConsensusParamsChanged int64

	// Merkle root of the results from executing prev block
	LastResultsHash []byte

	// The latest AppHash we've received from calling abci.Commit()
	AppHash []byte
}
