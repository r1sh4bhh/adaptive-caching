package config

import (
	"errors"
	"fmt"
	"strings"
)

// validLogLevels are the accepted values of log.level.
var validLogLevels = []string{"debug", "info", "warn", "error"}

// maxFrameRateHz bounds the UI tick. Above this the aggregator costs more than
// the information it carries.
const maxFrameRateHz = 240

// Validate checks every configured value lies in a sane range and returns a
// combined error describing all problems found.
func (c *Config) Validate() error {
	var errs []error

	if c.Cache.Capacity <= 0 {
		errs = append(errs, fmt.Errorf("cache.capacity must be > 0 bytes, got %d", c.Cache.Capacity))
	}
	if c.Cache.Policy == "" {
		errs = append(errs, errors.New(`cache.policy must not be empty (use "none" until P2 adds policies)`))
	}

	if c.Events.BusBuffer <= 0 {
		errs = append(errs, fmt.Errorf("events.bus_buffer must be > 0, got %d", c.Events.BusBuffer))
	}
	if c.Events.MetricsBuffer <= 0 {
		errs = append(errs, fmt.Errorf("events.metrics_buffer must be > 0, got %d", c.Events.MetricsBuffer))
	}
	if c.Events.FrameBuffer <= 0 {
		errs = append(errs, fmt.Errorf("events.frame_buffer must be > 0, got %d", c.Events.FrameBuffer))
	}
	if c.Events.RequestSampleRate == 0 {
		errs = append(errs, errors.New("events.request_sample_rate must be >= 1 (1 means every request)"))
	}

	if c.UI.FrameRateHz <= 0 || c.UI.FrameRateHz > maxFrameRateHz {
		errs = append(errs, fmt.Errorf("ui.frame_rate_hz must be in (0, %d], got %g", maxFrameRateHz, c.UI.FrameRateHz))
	}

	if !validLogLevel(c.Log.Level) {
		errs = append(errs, fmt.Errorf("log.level must be one of %s, got %q",
			strings.Join(validLogLevels, ", "), c.Log.Level))
	}

	return errors.Join(errs...)
}

func validLogLevel(level string) bool {
	for _, l := range validLogLevels {
		if level == l {
			return true
		}
	}
	return false
}
