package rawdb

import (
	"github.com/kardiachain/go-kardia/kai/kaidb"
	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/rlp"
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

// ReadLastPivotNumber retrieves the number of the last pivot block. If the node
// full synced, the last pivot will always be nil.
func ReadLastPivotNumber(db kaidb.KeyValueReader) *uint64 {
	data, _ := db.Get(lastPivotKey)
	if len(data) == 0 {
		return nil
	}
	var pivot uint64
	if err := rlp.DecodeBytes(data, &pivot); err != nil {
		log.Error("Invalid pivot block number in database", "err", err)
		return nil
	}
	return &pivot
}

// WriteLastPivotNumber stores the number of the last pivot block.
func WriteLastPivotNumber(db kaidb.KeyValueWriter, pivot uint64) {
	enc, err := rlp.EncodeToBytes(pivot)
	if err != nil {
		log.Crit("Failed to encode pivot block number", "err", err)
	}
	if err := db.Put(lastPivotKey, enc); err != nil {
		log.Crit("Failed to store pivot block number", "err", err)
	}
}
