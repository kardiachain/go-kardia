package cstate

import "github.com/kardiachain/go-kardia/lib/metrics"

var (
	consensusStateWrittenBytesMeter = metrics.NewRegisteredMeter("cstate/written", nil)
)
