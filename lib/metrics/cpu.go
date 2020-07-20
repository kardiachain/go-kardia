package metrics

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/shirou/gopsutil/cpu"
)

type CPUStats struct {
	GlobalTime int64 // Time spent by the CPU working on all processes
	GlobalWait int64 // Time spent by waiting on disk for all processes
	LocalTime  int64 // Time spent by the CPU working on this process
}

// ReadCPUStats retrieves the current CPU stats.
func ReadCPUStats(stats *CPUStats) {
	// passing false to request all cpu times
	timeStats, err := cpu.Times(false)
	if err != nil {
		log.Error("Could not read cpu stats", "err", err)
		return
	}
	// requesting all cpu times will always return an array with only one time stats entry
	timeStat := timeStats[0]
	stats.GlobalTime = int64((timeStat.User + timeStat.Nice + timeStat.System) * cpu.ClocksPerSec)
	stats.GlobalWait = int64((timeStat.Iowait) * cpu.ClocksPerSec)
	stats.LocalTime = getProcessCPUTime()
}
