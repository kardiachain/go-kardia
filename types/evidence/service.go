package evidence

import (
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/types"
)

// BlockStore ...
type BlockStore interface {
	LoadBlockMeta(height int64) *types.BlockMeta
}

// StateStore ...
type StateStore interface {
	LoadValidators(height int64) (*types.ValidatorSet, error)
	LoadState() state.LastestBlockState
}
