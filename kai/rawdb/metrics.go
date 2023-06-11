package rawdb

import "github.com/kardiachain/go-kardia/lib/metrics"

var (
	BlockInfoWrittenBytes = metrics.NewRegisteredMeter("kvstore/blockinfo", nil)
	ABIWrittenBytes       = metrics.NewRegisteredMeter("kvstore/abi", nil)
	EventWrittenBytes     = metrics.NewRegisteredMeter("kvstore/event", nil)
	TxLookupWrittenBytes  = metrics.NewRegisteredMeter("kvstore/txlookup", nil)
	BloombitsWrittenBytes = metrics.NewRegisteredMeter("kvstore/bloombits", nil)

	BlockMetaWrittenBytes       = metrics.NewRegisteredMeter("kvstore/blockmeta", nil)
	BlockPartWrittenBytes       = metrics.NewRegisteredMeter("kvstore/blockpart", nil)
	BlockCommitWrittenBytes     = metrics.NewRegisteredMeter("kvstore/blockcommit", nil)
	BlockSeenCommitWrittenBytes = metrics.NewRegisteredMeter("kvstore/blockseencommit", nil)
)
