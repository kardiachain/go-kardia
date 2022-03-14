package blockchain

import (
	"fmt"

	"github.com/kardiachain/go-kardia/lib/metrics"
)

var (
	MetricBlockInfoWrite    = metricName("block", "write")
	MetricBlockHeight       = metricName("block", "height")
	MetricBlockTransactions = metricName("block", "transactions")
	MetricBlockHash         = metricName("block", "hash")
	MetricBlockSave         = metricName("block", "save")
	MetricBlockCommit       = metricName("block", "commit")
	MetricBlockSeenCommit   = metricName("block", "seen_commit")
	MetricBlockInfo         = metricName("block", "info")
)

// Setup metrics
var (
	blockWriteTimer        = metrics.NewRegisteredTimer(MetricBlockInfoWrite, metrics.DefaultRegistry)
	blockHeightGauge       = metrics.NewRegisteredGauge(MetricBlockHeight, metrics.DefaultRegistry)
	blockTransactionsGauge = metrics.NewRegisteredGauge(MetricBlockTransactions, metrics.DefaultRegistry)
	blockHashGauge         = metrics.NewRegisteredGauge(MetricBlockHash, metrics.DefaultRegistry)
	blockSaveTimer         = metrics.NewRegisteredTimer(MetricBlockSave, metrics.DefaultRegistry)
	blockCommitSave        = metrics.NewRegisteredGauge(MetricBlockCommit, metrics.DefaultRegistry)
	blockSeenCommitSave    = metrics.NewRegisteredGauge(MetricBlockSeenCommit, metrics.DefaultRegistry)
	blockInfoSave          = metrics.NewRegisteredGauge(MetricBlockInfo, metrics.DefaultRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}
