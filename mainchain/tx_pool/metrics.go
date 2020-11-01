// Package tx_pool
package tx_pool

import (
	"fmt"

	"github.com/kardiachain/go-kardiamain/lib/metrics"
)

var (
	metricsPrefix = "tx_pool"

	MetricPendingDiscard   = metricName("pending", "discard")
	MetricPendingReplace   = metricName("pending", "replace")
	MetricPendingRateLimit = metricName("pending", "rate_limit")
	MetricPendingNoFunds   = metricName("pending", "no_funds")

	MetricQueuedDiscard   = metricName("queued", "discard")
	MetricQueuedReplace   = metricName("queued", "replace")
	MetricQueuedRateLimit = metricName("queued", "rate_limit")
	MetricQueuedNoFunds   = metricName("queued", "no_funds")

	MetricKnown       = metricName("", "known")
	MetricValid       = metricName("", "valid")
	MetricInvalid     = metricName("", "invalid")
	MetricUnderPriced = metricName("", "under_priced")

	MetricPending = metricName("", "pending")
	MetricQueued  = metricName("", "queued")
	MetricLocal   = metricName("", "local")

	MetricTxsTime       = metricName("time", "txs")
	MetricLockedTxsTime = metricName("time", "locked_txs")
)

var (
	//
	// Metrics for the pending pool
	pendingDiscardMeter   = metrics.NewRegisteredMeter(MetricPendingDiscard, metrics.TxPoolRegistry)
	pendingReplaceMeter   = metrics.NewRegisteredMeter(MetricPendingReplace, metrics.TxPoolRegistry)
	pendingRateLimitMeter = metrics.NewRegisteredMeter(MetricPendingRateLimit, metrics.TxPoolRegistry) // Dropped due to rate limiting
	pendingNoFundsMeter   = metrics.NewRegisteredMeter(MetricPendingNoFunds, metrics.TxPoolRegistry)   // Dropped due to out-of-funds

	// Metrics for the queued pool
	queuedDiscardMeter   = metrics.NewRegisteredMeter(MetricQueuedDiscard, metrics.TxPoolRegistry)
	queuedReplaceMeter   = metrics.NewRegisteredMeter(MetricQueuedReplace, metrics.TxPoolRegistry)
	queuedRateLimitMeter = metrics.NewRegisteredMeter(MetricQueuedRateLimit, metrics.TxPoolRegistry) // Dropped due to rate limiting
	queuedNoFundsMeter   = metrics.NewRegisteredMeter(MetricQueuedNoFunds, metrics.TxPoolRegistry)   // Dropped due to out-of-funds

	// General tx metrics
	knownTxMeter       = metrics.NewRegisteredMeter(MetricKnown, metrics.TxPoolRegistry)
	validTxMeter       = metrics.NewRegisteredMeter(MetricValid, metrics.TxPoolRegistry)
	invalidTxMeter     = metrics.NewRegisteredMeter(MetricInvalid, metrics.TxPoolRegistry)
	underpricedTxMeter = metrics.NewRegisteredMeter(MetricUnderPriced, metrics.TxPoolRegistry)

	pendingGauge = metrics.NewRegisteredGauge(MetricPending, metrics.TxPoolRegistry)
	queuedGauge  = metrics.NewRegisteredGauge(MetricQueued, metrics.TxPoolRegistry)
	localGauge   = metrics.NewRegisteredGauge(MetricLocal, metrics.TxPoolRegistry)

	txsTimer       = metrics.NewRegisteredTimer(MetricTxsTime, metrics.TxPoolRegistry)
	lockedTxsTimer = metrics.NewRegisteredTimer(MetricLockedTxsTime, metrics.TxPoolRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s/%s", metricsPrefix, group, name)
	}
	return fmt.Sprintf("%s/%s", metricsPrefix, name)
}
