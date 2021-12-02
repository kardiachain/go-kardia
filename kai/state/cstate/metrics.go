package cstate

import (
	"fmt"
	"github.com/kardiachain/go-kardia/lib/metrics"
)

var (
	MetricSaveState 		= metricName("cstate", "state/save")
	MetricStateBytes 		= metricName("cstate", "state/bytes")
	MetricValidatorsChanged = metricName("cstate", "state/validators-changed")
	MetricNextValidators	= metricName("cstate", "state/next-validators")
)

var (
	saveStateTimer 		= metrics.NewRegisteredTimer(MetricSaveState, metrics.CStateRegistry)
	stateBytesLength 	= metrics.NewRegisteredGauge(MetricStateBytes, metrics.CStateRegistry)
	lastHeightValidatorsChangedGauge = metrics.NewRegisteredGauge(MetricValidatorsChanged, metrics.CStateRegistry)
	nextValidatorsGauge = metrics.NewRegisteredGauge(MetricNextValidators, metrics.CStateRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}
