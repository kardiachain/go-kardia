package cstate

import "github.com/kardiachain/go-kardia/lib/metrics"

var (
	consensusStateWrittenBytesGauge = metrics.NewRegisteredGauge("cstate/written", nil)
	consensusStatePrunedBytesGauge  = metrics.NewRegisteredGauge("cstate/pruned", nil)
)
