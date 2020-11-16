package trust

import "time"

// MetricConfig - Configures the weight functions and time intervals for the metric
type MetricConfig struct {
	// Determines the percentage given to current behavior
	ProportionalWeight float64

	// Determines the percentage given to prior behavior
	IntegralWeight float64

	// The window of time that the trust metric will track events across.
	// This can be set to cover many days without issue
	TrackingWindow time.Duration

	// Each interval should be short for adapability.
	// Less than 30 seconds is too sensitive,
	// and greater than 5 minutes will make the metric numb
	IntervalLength time.Duration
}

// DefaultConfig returns a config with values that have been tested and produce desirable results
func DefaultConfig() MetricConfig {
	return MetricConfig{
		ProportionalWeight: 0.4,
		IntegralWeight:     0.6,
		TrackingWindow:     (time.Minute * 60 * 24) * 14, // 14 days.
		IntervalLength:     1 * time.Minute,
	}
}

// Ensures that all configuration elements have valid values
func customConfig(mc MetricConfig) MetricConfig {
	config := DefaultConfig()

	// Check the config for set values, and setup appropriately
	if mc.ProportionalWeight > 0 {
		config.ProportionalWeight = mc.ProportionalWeight
	}

	if mc.IntegralWeight > 0 {
		config.IntegralWeight = mc.IntegralWeight
	}

	if mc.IntervalLength > time.Duration(0) {
		config.IntervalLength = mc.IntervalLength
	}

	if mc.TrackingWindow > time.Duration(0) &&
		mc.TrackingWindow >= config.IntervalLength {
		config.TrackingWindow = mc.TrackingWindow
	}
	return config
}
