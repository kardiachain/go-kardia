package evidence

import "github.com/kardiachain/go-kardiamain/types"

type BlockStore interface {
	LoadBlockMeta(height uint64) *types.BlockMeta
	LoadBlockCommit(height uint64) *types.Commit
}
