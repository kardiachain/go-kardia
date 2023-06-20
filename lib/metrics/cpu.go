// Copyright 2018 The go-kardia Authors
// This file is part of the go-kardia library.
//
// The go-kardia library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-kardia library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.

package metrics

import (
	"github.com/kardiachain/go-kardia/lib/log"
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
