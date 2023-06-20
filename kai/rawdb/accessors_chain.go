package rawdb

import (
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/types"
)

// ReadHeadBlock returns the current canonical head block.
func ReadHeadBlock(db kaidb.Reader) *types.Block {
	headBlockHash := ReadHeadBlockHash(db)
	if headBlockHash == (common.Hash{}) {
		return nil
	}
	headBlockNumber := ReadHeaderHeight(db, headBlockHash)
	if headBlockNumber == nil {
		return nil
	}
	return ReadBlock(db, *headBlockNumber)
}
