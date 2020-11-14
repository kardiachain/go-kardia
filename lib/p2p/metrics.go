package p2p

import (
	"fmt"
	"time"

	"github.com/kardiachain/go-kardiamain/lib/metrics"
)

const metricsTickerDuration = 10 * time.Second

const (
	MetricPeers                 = "peers"
	MetricPeerReceiveBytesTotal = "total_received"
	MetricPeerSendBytesTotal    = "total_sent"
	MetricPeerPendingSendBytes  = "pending_send"
	MetricNumTxs                = "num_txs"
)

// Metrics contains metrics exposed by this package.
type Metrics struct {
	// Number of peers.
	Peers metrics.Gauge
	// Number of bytes received from a given peer.
	PeerReceiveBytesTotal metrics.Counter
	// Number of bytes sent to a given peer.
	PeerSendBytesTotal metrics.Counter
	// Pending bytes to be sent to a given peer.
	PeerPendingSendBytes metrics.Gauge
	// Number of transactions submitted by each peer.
	NumTxs metrics.Gauge
}

func InitMetrics() *Metrics {
	return &Metrics{
		Peers:                 metrics.NewRegisteredGauge(metricName("", MetricPeers), metrics.P2PRegistry),
		PeerReceiveBytesTotal: metrics.NewRegisteredCounter(metricName("", MetricPeerReceiveBytesTotal), metrics.P2PRegistry),
		PeerSendBytesTotal:    metrics.NewRegisteredCounter(metricName("", MetricPeerSendBytesTotal), metrics.P2PRegistry),
		PeerPendingSendBytes:  metrics.NewRegisteredGauge(metricName("", MetricPeerPendingSendBytes), metrics.P2PRegistry),
		NumTxs:                metrics.NewRegisteredGauge(metricName("", MetricNumTxs), metrics.P2PRegistry),
	}
}

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}
