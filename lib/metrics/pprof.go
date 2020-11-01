// Package metrics
package metrics

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func PoolMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, strings.Join(os.Args, "\x00"))

	// If nothing then return all

	// Show metrics for specific pool
	// Pending pool

	// Queue pool

	// Tx metrics

}
