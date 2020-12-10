// Package kaidb
package kaidb

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/metrics"
)

const (
	degradationWarnInterval = time.Minute

	DefaultMetricGatherInterval = 3 * time.Second
)

var (
	MetricCompTime               = metricName("compact", "time")
	MetricCompReadTime           = metricName("compact", "read")
	MetricCompWriteTime          = metricName("compact", "write")
	MetricCompWriteDelayDuration = metricName("compact", "write_delay/duration")
	MetricCompWriteDelayCounter  = metricName("compact", "write_delay/counter")
	MetricCompMemory             = metricName("compact", "memory")
	MetricCompLevel0             = metricName("compact", "level0")
	MetricCompNonLevel0          = metricName("compact", "non_level0")
	MetricCompSeek               = metricName("compact", "seek")

	MetricDiskSize  = metricName("disk", "size")
	MetricDiskRead  = metricName("disk", "read")
	MetricDiskWrite = metricName("disk", "write")
)

// Setup metrics
var (
	compTimeMeter      = metrics.NewRegisteredMeter(MetricCompTime, metrics.DBRegistry)               // Meter for measuring the total time spent in database compaction
	compReadMeter      = metrics.NewRegisteredMeter(MetricCompReadTime, metrics.DBRegistry)           // Meter for measuring the data read during compaction
	compWriteMeter     = metrics.NewRegisteredMeter(MetricCompWriteTime, metrics.DBRegistry)          // Meter for measuring the data written during compaction
	writeDelayNMeter   = metrics.NewRegisteredMeter(MetricCompWriteDelayDuration, metrics.DBRegistry) // Meter for measuring the write delay number due to database compaction
	writeDelayMeter    = metrics.NewRegisteredMeter(MetricCompWriteDelayCounter, metrics.DBRegistry)  // Meter for measuring the write delay duration due to database compaction
	memCompGauge       = metrics.NewRegisteredGauge(MetricCompMemory, metrics.DBRegistry)             // Gauge for tracking the number of memory compaction
	level0CompGauge    = metrics.NewRegisteredGauge(MetricCompLevel0, metrics.DBRegistry)             // Gauge for tracking the number of table compaction in level0
	nonLevel0CompGauge = metrics.NewRegisteredGauge(MetricCompNonLevel0, metrics.DBRegistry)          // Gauge for tracking the number of table compaction in non0 level
	seekCompGauge      = metrics.NewRegisteredGauge(MetricCompSeek, metrics.DBRegistry)               // Gauge for tracking the number of table compaction caused by read opt

	diskSizeGauge  = metrics.NewRegisteredGauge(MetricDiskSize, metrics.DBRegistry)  // Gauge for tracking the size of all the levels in the database
	diskReadMeter  = metrics.NewRegisteredMeter(MetricDiskRead, metrics.DBRegistry)  // Meter for measuring the effective amount of data read
	diskWriteMeter = metrics.NewRegisteredMeter(MetricDiskWrite, metrics.DBRegistry) // Meter for measuring the effective amount of data written
)

func metricName(group, name string) string {
	if group != "" {
		return fmt.Sprintf("%s/%s", group, name)
	}
	return name
}

type GetProperty func(name string) (value string, err error)

// meter periodically retrieves internal leveldb counters and reports them to
// the metrics subsystem.
//
// This is how a LevelDB stats table looks like (currently):
//   Compactions
//    Level |   Tables   |    Size(MB)   |    Time(sec)  |    Read(MB)   |   Write(MB)
//   -------+------------+---------------+---------------+---------------+---------------
//      0   |          0 |       0.00000 |       1.27969 |       0.00000 |      12.31098
//      1   |         85 |     109.27913 |      28.09293 |     213.92493 |     214.26294
//      2   |        523 |    1000.37159 |       7.26059 |      66.86342 |      66.77884
//      3   |        570 |    1113.18458 |       0.00000 |       0.00000 |       0.00000
//
// This is how the write delay look like (currently):
// DelayN:5 Delay:406.604657ms Paused: false
//
// This is how the iostats look like (currently):
// Read(MB):3895.04860 Write(MB):3654.64712
func UpdateDBMeter(refresh time.Duration, propertyPrefix string, getProperty GetProperty, quitChan chan chan error) {
	logger := log.New()
	// Create the counters to store current and previous compaction values
	compactions := make([][]float64, 2)
	for i := 0; i < 2; i++ {
		compactions[i] = make([]float64, 4)
	}
	// Create storage for iostats.
	var iostats [2]float64

	// Create storage and warning log tracer for write delay.
	var (
		delayStats      [2]int64
		lastWritePaused time.Time
	)

	var (
		errCh chan error
		mErr  error
	)

	// Iterate ad infinitum and collect the stats
	for i := 1; errCh == nil && mErr == nil; i++ {
		// Retrieve the database stats
		stats, err := getProperty(propertyName(propertyPrefix, "stats"))
		if err != nil {
			logger.Error("Failed to read database stats", "err", err)
			mErr = err
			continue
		}
		// Find the compaction table, skip the header
		lines := strings.Split(stats, "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[0]) != "Compactions" {
			lines = lines[1:]
		}
		if len(lines) <= 3 {
			logger.Error("Compaction leveldbTable not found")
			mErr = errors.New("compaction leveldbTable not found")
			continue
		}
		lines = lines[3:]

		// Iterate over all the leveldbTable rows, and accumulate the entries
		for j := 0; j < len(compactions[i%2]); j++ {
			compactions[i%2][j] = 0
		}
		for _, line := range lines {
			parts := strings.Split(line, "|")
			if len(parts) != 6 {
				break
			}
			for idx, counter := range parts[2:] {
				value, err := strconv.ParseFloat(strings.TrimSpace(counter), 64)
				if err != nil {
					logger.Error("Compaction entry parsing failed", "err", err)
					mErr = err
					continue
				}
				compactions[i%2][idx] += value
			}
		}
		// Update all the requested meters
		if diskSizeGauge != nil {
			diskSizeGauge.Update(int64(compactions[i%2][0] * 1024 * 1024))
		}
		if compTimeMeter != nil {
			compTimeMeter.Mark(int64((compactions[i%2][1] - compactions[(i-1)%2][1]) * 1000 * 1000 * 1000))
		}
		if compReadMeter != nil {
			compReadMeter.Mark(int64((compactions[i%2][2] - compactions[(i-1)%2][2]) * 1024 * 1024))
		}
		if compWriteMeter != nil {
			compWriteMeter.Mark(int64((compactions[i%2][3] - compactions[(i-1)%2][3]) * 1024 * 1024))
		}
		// Retrieve the write delay statistic
		writeDelay, err := getProperty(propertyName(propertyPrefix, "writedelay"))
		if err != nil {
			logger.Error("Failed to read database write delay statistic", "err", err)
			mErr = err
			continue
		}
		var (
			delayN        int64
			delayDuration string
			duration      time.Duration
			paused        bool
		)
		if n, err := fmt.Sscanf(writeDelay, "DelayN:%d Delay:%s Paused:%t", &delayN, &delayDuration, &paused); n != 3 || err != nil {
			logger.Error("Write delay statistic not found")
			mErr = err
			continue
		}
		duration, err = time.ParseDuration(delayDuration)
		if err != nil {
			logger.Error("Failed to parse delay duration", "err", err)
			mErr = err
			continue
		}
		if writeDelayNMeter != nil {
			writeDelayNMeter.Mark(delayN - delayStats[0])
		}
		if writeDelayMeter != nil {
			writeDelayMeter.Mark(duration.Nanoseconds() - delayStats[1])
		}
		// If a warning that db is performing compaction has been displayed, any subsequent
		// warnings will be withheld for one minute not to overwhelm the user.
		if paused && delayN-delayStats[0] == 0 && duration.Nanoseconds()-delayStats[1] == 0 &&
			time.Now().After(lastWritePaused.Add(degradationWarnInterval)) /* time.Minute should move to */ {
			logger.Warn("Database compacting, degraded performance")
			lastWritePaused = time.Now()
		}
		delayStats[0], delayStats[1] = delayN, duration.Nanoseconds()

		// Retrieve the database iostats.
		ioStats, err := getProperty(propertyName(propertyPrefix, "iostats"))
		if err != nil {
			logger.Error("Failed to read database iostats", "err", err)
			mErr = err
			continue
		}
		var nRead, nWrite float64
		parts := strings.Split(ioStats, " ")
		if len(parts) < 2 {
			logger.Error("Bad syntax of ioStats", "ioStats", ioStats)
			mErr = fmt.Errorf("bad syntax of ioStats %s", ioStats)
			continue
		}
		if n, err := fmt.Sscanf(parts[0], "Read(MB):%f", &nRead); n != 1 || err != nil {
			logger.Error("Bad syntax of read entry", "entry", parts[0])
			mErr = err
			continue
		}
		if n, err := fmt.Sscanf(parts[1], "Write(MB):%f", &nWrite); n != 1 || err != nil {
			logger.Error("Bad syntax of write entry", "entry", parts[1])
			mErr = err
			continue
		}
		if diskReadMeter != nil {
			diskReadMeter.Mark(int64((nRead - iostats[0]) * 1024 * 1024))
		}
		if diskWriteMeter != nil {
			diskWriteMeter.Mark(int64((nWrite - iostats[1]) * 1024 * 1024))
		}
		iostats[0], iostats[1] = nRead, nWrite

		compCount, err := getProperty(propertyName(propertyPrefix, "compcount"))
		if err != nil {
			logger.Error("Failed to read database iostats", "err", err)
			mErr = err
			continue
		}

		var (
			memComp       uint32
			level0Comp    uint32
			nonLevel0Comp uint32
			seekComp      uint32
		)
		if n, err := fmt.Sscanf(compCount, "MemComp:%d Level0Comp:%d NonLevel0Comp:%d SeekComp:%d", &memComp, &level0Comp, &nonLevel0Comp, &seekComp); n != 4 || err != nil {
			logger.Error("Compaction count statistic not found")
			mErr = err
			continue
		}
		memCompGauge.Update(int64(memComp))
		level0CompGauge.Update(int64(level0Comp))
		nonLevel0CompGauge.Update(int64(nonLevel0Comp))
		seekCompGauge.Update(int64(seekComp))

		// Sleep a bit, then repeat the stats collection
		select {
		case errCh = <-quitChan:
			// Quit requesting, stop hammering the database
		case <-time.After(refresh):
			// Timeout, gather a new set of stats
		}
	}

	if errCh == nil {
		errCh = <-quitChan
	}
	errCh <- mErr
}

func propertyName(prefix, p string) string {
	return fmt.Sprintf("%s.%s", prefix, p)
}
