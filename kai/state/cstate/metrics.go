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
	//saveStateTimer		= promauto.NewHistogram(prometheus.HistogramOpts{
	//	Name: "save_state_timer",
	//	Help: ""
	//})

	//StateBytesLength = promauto.NewGauge(prometheus.GaugeOpts{
	//	Name: "state_bytes_length_total",
	//	Help: "state bytes length",
	//})

	saveStateTimer 		= metrics.NewRegisteredTimer(MetricSaveState, metrics.DefaultRegistry)
	StateBytesLength 	= metrics.NewRegisteredGauge(MetricStateBytes, metrics.DefaultRegistry)
	lastHeightValidatorsChangedGauge = metrics.NewRegisteredGauge(MetricValidatorsChanged, metrics.DefaultRegistry)
	nextValidatorsGauge = metrics.NewRegisteredGauge(MetricNextValidators, metrics.DefaultRegistry)
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}
