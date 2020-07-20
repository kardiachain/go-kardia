package metrics

import (
	"syscall"

	"github.com/ethereum/go-ethereum/log"
)

// getProcessCPUTime retrieves the process' CPU time since program startup.
func getProcessCPUTime() int64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		log.Warn("Failed to retrieve CPU time", "err", err)
		return 0
	}
	return int64(usage.Utime.Sec+usage.Stime.Sec)*100 + int64(usage.Utime.Usec+usage.Stime.Usec)/10000 //nolint:unconvert
}
