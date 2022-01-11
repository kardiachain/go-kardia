package prometheus

import (
	"fmt"
	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"
	"net/http"
	"sort"
)

// Handler returns an HTTP handler which dump metrics in Prometheus format.
func Handler(reg metrics.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Gather and pre-sort the metrics to avoid random listings
		var names []string
		reg.Each(func(name string, i interface{}) {
			names = append(names, name)
		})
		sort.Strings(names)

		// Aggregate all the metris into a Prometheus collector
		c := newCollector()

		for _, name := range names {
			i := reg.Get(name)

			switch m := i.(type) {
			case metrics.Counter:
				c.addCounter(name, m.Snapshot())
			case metrics.Gauge:
				c.addGauge(name, m.Snapshot())
			case metrics.GaugeFloat64:
				c.addGaugeFloat64(name, m.Snapshot())
			case metrics.Histogram:
				c.addHistogram(name, m.Snapshot())
			case metrics.Meter:
				c.addMeter(name, m.Snapshot())
			case metrics.Timer:
				c.addTimer(name, m.Snapshot())
			case metrics.ResettingTimer:
				c.addResettingTimer(name, m.Snapshot())
			default:
				log.Warn("Unknown Prometheus metric type", "type", fmt.Sprintf("%T", i))
			}
		}
		w.Header().Add("Content-Type", "text/plain")
		w.Header().Add("Content-Length", fmt.Sprint(c.buff.Len()))
		w.Write(c.buff.Bytes())
	})
}