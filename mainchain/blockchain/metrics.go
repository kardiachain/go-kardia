package blockchain

import (
	"fmt"
	"github.com/kardiachain/go-kardia/lib/metrics"
)
var (
	MetricBlockWrite = metricName("block", "write")
	MetricBlockHeight = metricName("block", "height")
	MetricBlockTransactions = metricName("block", "transactions")
)

// Setup metrics
var (
	blockWriteTimer = metrics.NewRegisteredTimer(MetricBlockWrite, metrics.BlockchainRegistry)
	blockHeightGauge = metrics.NewRegisteredGauge(MetricBlockHeight, metrics.BlockchainRegistry)
	blockTransactionsGauge = metrics.NewRegisteredGauge(MetricBlockTransactions, metrics.BlockchainRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}