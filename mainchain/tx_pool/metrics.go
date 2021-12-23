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
	MetricEvictionMeter   = metricName("queued", "eviction")

	MetricKnown            = metricName("", "known")
	MetricValid            = metricName("", "valid")
	MetricInvalid          = metricName("", "invalid")
	MetricUnderPriced      = metricName("", "under_priced")
	MetricOveflowedTx      = metricName("", "overflowed")
	MetricThrottleTx       = metricName("", "throttle")
	MetricDropBetweenReorg = metricName("", "dropbetweenreorg")

	MetricPending = metricName("", "pending")
	MetricQueued  = metricName("", "queued")
	MetricLocal   = metricName("", "local")
	MetricSlot    = metricName("", "slot")

	MetricTxsTime       = metricName("time", "txs")
	MetricReorgTime     = metricName("", "reorgtime")
	MetricReheapTime    = metricName("", "reheap")
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
	queuedEvictionMeter  = metrics.NewRegisteredMeter(MetricEvictionMeter, metrics.TxPoolRegistry)   // Dropped due to lifetime

	// General tx metrics
	knownTxMeter       = metrics.NewRegisteredMeter(MetricKnown, metrics.TxPoolRegistry)
	validTxMeter       = metrics.NewRegisteredMeter(MetricValid, metrics.TxPoolRegistry)
	invalidTxMeter     = metrics.NewRegisteredMeter(MetricInvalid, metrics.TxPoolRegistry)
	underpricedTxMeter = metrics.NewRegisteredMeter(MetricUnderPriced, metrics.TxPoolRegistry)
	overflowedTxMeter  = metrics.NewRegisteredMeter(MetricOveflowedTx, metrics.TxPoolRegistry)
	// throttleTxMeter counts how many transactions are rejected due to too-many-changes between
	// txpool reorgs.
	throttleTxMeter = metrics.NewRegisteredMeter(MetricThrottleTx, metrics.TxPoolRegistry)
	// dropBetweenReorgHistogram counts how many drops we experience between two reorg runs. It is expected
	// that this number is pretty low, since txpool reorgs happen very frequently.
	dropBetweenReorgHistogram = metrics.NewRegisteredHistogram(MetricDropBetweenReorg, nil, metrics.NewExpDecaySample(1028, 0.015))

	pendingGauge = metrics.NewRegisteredGauge(MetricPending, metrics.TxPoolRegistry)
	queuedGauge  = metrics.NewRegisteredGauge(MetricQueued, metrics.TxPoolRegistry)
	localGauge   = metrics.NewRegisteredGauge(MetricLocal, metrics.TxPoolRegistry)
	slotsGauge   = metrics.NewRegisteredGauge(MetricSlot, metrics.TxPoolRegistry)

	txsTimer           = metrics.NewRegisteredTimer(MetricTxsTime, metrics.TxPoolRegistry)
	reorgDurationTimer = metrics.NewRegisteredTimer(MetricReheapTime, metrics.TxPoolRegistry)
	lockedTxsTimer     = metrics.NewRegisteredTimer(MetricLockedTxsTime, metrics.TxPoolRegistry)
	reheapTimer        = metrics.NewRegisteredTimer("txpool/reheap", nil)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s/%s", metricsPrefix, group, name)
	}
	return fmt.Sprintf("%s/%s", metricsPrefix, name)
}
