package blockchain

import (
	"fmt"
	"github.com/kardiachain/go-kardia/lib/metrics"
)

var (
	MetricBlockInfoWrite 	= metricName("block", "write")
	MetricBlockHeight 		= metricName("block", "height")
	MetricBlockTransactions = metricName("block", "transactions")
	MetricBlockHash			= metricName("block", "hash")
	MetricBlockSave			= metricName("block", "save")
)

// Setup metrics
var (
	blockWriteTimer 		= metrics.NewRegisteredTimer(MetricBlockInfoWrite, metrics.BlockchainRegistry)
	blockHeightGauge 		= metrics.NewRegisteredGauge(MetricBlockHeight, metrics.BlockchainRegistry)
	blockTransactionsGauge 	= metrics.NewRegisteredGauge(MetricBlockTransactions, metrics.BlockchainRegistry)
	blockHashGauge			= metrics.NewRegisteredGauge(MetricBlockHash, metrics.BlockchainRegistry)
	blockSaveTimer			= metrics.NewRegisteredTimer(MetricBlockSave, metrics.BlockchainRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}