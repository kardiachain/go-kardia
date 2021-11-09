/*
 *  Copyright 2020 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

// Package tx_pool
package tx_pool

import (
	"fmt"

	"github.com/kardiachain/go-kardia/lib/metrics"
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
	MetricSlot    = metricName("", "slot")

	MetricTxsTime       = metricName("time", "txs")
	MetricLockedTxsTime = metricName("time", "locked_txs")
)

// Setup metrics
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
	slotsGauge   = metrics.NewRegisteredGauge(MetricSlot, metrics.TxPoolRegistry)

	txsTimer       = metrics.NewRegisteredTimer(MetricTxsTime, metrics.TxPoolRegistry)
	lockedTxsTimer = metrics.NewRegisteredTimer(MetricLockedTxsTime, metrics.TxPoolRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s/%s", metricsPrefix, group, name)
	}
	return fmt.Sprintf("%s/%s", metricsPrefix, name)
}
