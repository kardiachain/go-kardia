// Go port of Coda Hale's Metrics library
//
// <https://github.com/rcrowley/go-metrics>
//
// Coda Hale's original work: <https://github.com/codahale/metrics>
package metrics

import (
	"runtime"
	"time"
)

// Enabled is checked by the constructor functions for all of the
// standard metrics. If it is true, the metric returned is a stub.
//
// This global kill-switch helps quantify the observer effect and makes
// for less cluttered pprof profiles.
var Enabled = true

// CollectProcessMetrics periodically collects various metrics about the running
// process.
func CollectProcessMetrics(refresh time.Duration) {
	// Short circuit if the metrics system is disabled
	if !Enabled {
		return
	}
	refreshFreq := int64(refresh / time.Second)

	// Create the various data collectors
	cpuStats := make([]*CPUStats, 2)
	memstats := make([]*runtime.MemStats, 2)
	diskstats := make([]*DiskStats, 2)
	for i := 0; i < len(memstats); i++ {
		cpuStats[i] = new(CPUStats)
		memstats[i] = new(runtime.MemStats)
		diskstats[i] = new(DiskStats)
	}
	// Define the various metrics to collect
	var (
		cpuSysLoad    = NewRegisteredGauge("cpu/sysload", SystemRegistry)
		cpuSysWait    = NewRegisteredGauge("cpu/syswait", SystemRegistry)
		cpuProcLoad   = NewRegisteredGauge("cpu/procload", SystemRegistry)
		cpuThreads    = NewRegisteredGauge("cpu/threads", SystemRegistry)
		cpuGoroutines = NewRegisteredGauge("cpu/goroutines", SystemRegistry)

		memPauses = NewRegisteredMeter("memory/pauses", SystemRegistry)
		memAllocs = NewRegisteredMeter("memory/allocs", SystemRegistry)
		memFrees  = NewRegisteredMeter("memory/frees", SystemRegistry)
		memHeld   = NewRegisteredGauge("memory/held", SystemRegistry)
		memUsed   = NewRegisteredGauge("memory/used", SystemRegistry)

		diskReads             = NewRegisteredMeter("disk/readcount", SystemRegistry)
		diskReadBytes         = NewRegisteredMeter("disk/readdata", SystemRegistry)
		diskReadBytesCounter  = NewRegisteredCounter("disk/readbytes", SystemRegistry)
		diskWrites            = NewRegisteredMeter("disk/writecount", SystemRegistry)
		diskWriteBytes        = NewRegisteredMeter("disk/writedata", SystemRegistry)
		diskWriteBytesCounter = NewRegisteredCounter("disk/writebytes", SystemRegistry)
	)
	// Iterate loading the different stats and updating the meters
	for i := 1; ; i++ {
		location1 := i % 2
		location2 := (i - 1) % 2

		ReadCPUStats(cpuStats[location1])
		cpuSysLoad.Update((cpuStats[location1].GlobalTime - cpuStats[location2].GlobalTime) / refreshFreq)
		cpuSysWait.Update((cpuStats[location1].GlobalWait - cpuStats[location2].GlobalWait) / refreshFreq)
		cpuProcLoad.Update((cpuStats[location1].LocalTime - cpuStats[location2].LocalTime) / refreshFreq)
		cpuThreads.Update(int64(threadCreateProfile.Count()))
		cpuGoroutines.Update(int64(runtime.NumGoroutine()))

		runtime.ReadMemStats(memstats[location1])
		memPauses.Mark(int64(memstats[location1].PauseTotalNs - memstats[location2].PauseTotalNs))
		memAllocs.Mark(int64(memstats[location1].Mallocs - memstats[location2].Mallocs))
		memFrees.Mark(int64(memstats[location1].Frees - memstats[location2].Frees))
		memHeld.Update(int64(memstats[location1].HeapSys - memstats[location1].HeapReleased))
		memUsed.Update(int64(memstats[location1].Alloc))

		if ReadDiskStats(diskstats[location1]) == nil {
			diskReads.Mark(diskstats[location1].ReadCount - diskstats[location2].ReadCount)
			diskReadBytes.Mark(diskstats[location1].ReadBytes - diskstats[location2].ReadBytes)
			diskWrites.Mark(diskstats[location1].WriteCount - diskstats[location2].WriteCount)
			diskWriteBytes.Mark(diskstats[location1].WriteBytes - diskstats[location2].WriteBytes)

			diskReadBytesCounter.Inc(diskstats[location1].ReadBytes - diskstats[location2].ReadBytes)
			diskWriteBytesCounter.Inc(diskstats[location1].WriteBytes - diskstats[location2].WriteBytes)
		}
		time.Sleep(refresh)
	}
}
