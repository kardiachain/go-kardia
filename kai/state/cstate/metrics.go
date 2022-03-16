package cstate

import (
	"fmt"
	"github.com/kardiachain/go-kardia/lib/metrics"
)

var (
	MetricSaveState         = metricName("cstate", "state/save")
	MetricStateBytes        = metricName("cstate", "state/bytes")
	MetricValidatorsChanged = metricName("cstate", "state/validators_changed")
	MetricNextValidators    = metricName("cstate", "state/next_validators")
)

var (
	saveStateTimer                   = metrics.NewRegisteredTimer(MetricSaveState, metrics.DefaultRegistry)
	stateBytesLength                 = metrics.NewRegisteredGauge(MetricStateBytes, metrics.DefaultRegistry)
	lastHeightValidatorsChangedGauge = metrics.NewRegisteredGauge(MetricValidatorsChanged, metrics.DefaultRegistry)
	nextValidatorsGauge              = metrics.NewRegisteredGauge(MetricNextValidators, metrics.DefaultRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}
