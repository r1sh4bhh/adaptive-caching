// Package config loads, defaults and validates the YAML configuration.
//
// Configuration is the only mechanism for enabling or disabling behaviour:
// ablation studies are config permutations, never code branches (see
// config/flags.go).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ByteSize is a byte count that unmarshals from human-readable strings such as
// "100MB", "512KiB", "2GB" or a bare number of bytes.
//
// Decimal suffixes (KB/MB/GB/TB) are powers of 1000; binary suffixes
// (KiB/MiB/GiB/TiB) are powers of 1024.
type ByteSize int64

var byteUnits = []struct {
	suffix string
	mult   int64
}{
	{"KIB", 1 << 10},
	{"MIB", 1 << 20},
	{"GIB", 1 << 30},
	{"TIB", 1 << 40},
	{"KB", 1000},
	{"MB", 1000 * 1000},
	{"GB", 1000 * 1000 * 1000},
	{"TB", 1000 * 1000 * 1000 * 1000},
	{"B", 1},
}

// ParseByteSize parses a human-readable byte size.
func ParseByteSize(s string) (ByteSize, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("empty byte size")
	}
	upper := strings.ToUpper(trimmed)
	for _, u := range byteUnits {
		if !strings.HasSuffix(upper, u.suffix) {
			continue
		}
		num := strings.TrimSpace(upper[:len(upper)-len(u.suffix)])
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
		}
		return ByteSize(v * float64(u.mult)), nil
	}
	v, err := strconv.ParseInt(upper, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q", s)
	}
	return ByteSize(v), nil
}

// UnmarshalYAML accepts either a number of bytes or a suffixed string.
func (b *ByteSize) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return err
	}
	v, err := ParseByteSize(raw)
	if err != nil {
		return err
	}
	*b = v
	return nil
}

// MarshalYAML emits the plain byte count so round-tripping is lossless.
func (b ByteSize) MarshalYAML() (any, error) { return int64(b), nil }

// Bytes returns the size as an int64 byte count.
func (b ByteSize) Bytes() int64 { return int64(b) }

// String renders the size using the largest exact binary unit.
func (b ByteSize) String() string {
	n := int64(b)
	switch {
	case n >= 1<<40:
		return fmt.Sprintf("%.2fTiB", float64(n)/(1<<40))
	case n >= 1<<30:
		return fmt.Sprintf("%.2fGiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.2fMiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.2fKiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// Config is the full runtime configuration.
type Config struct {
	Cache    CacheConfig  `yaml:"cache"`
	Events   EventsConfig `yaml:"events"`
	UI       UIConfig     `yaml:"ui"`
	Log      LogConfig    `yaml:"log"`
	Features FeatureFlags `yaml:"features"`
}

// CacheConfig configures the cache core. Capacity is in BYTES, never in object
// count — heterogeneous object sizes are a core contribution of the project.
type CacheConfig struct {
	Capacity ByteSize `yaml:"capacity"`
	// Policy names the eviction policy to install. P2 ships LRU, LFU
	// and Clock; "none" runs with a nil policy. Unknown names are
	// rejected at config load (see config/validate.go).
	Policy string `yaml:"policy"`
}

// EventsConfig configures the event bus and its subscribers.
type EventsConfig struct {
	// BusBuffer is the default per-subscriber channel capacity.
	BusBuffer int `yaml:"bus_buffer"`
	// MetricsBuffer is the (larger) buffer for the lossless metrics consumer.
	MetricsBuffer int `yaml:"metrics_buffer"`
	// FrameBuffer is the buffer for the frame aggregator.
	FrameBuffer int `yaml:"frame_buffer"`
	// RequestSampleRate emits one TypeRequest event per N requests. At high
	// request rates an unsampled bus would dominate the CPU profile.
	RequestSampleRate uint64 `yaml:"request_sample_rate"`
}

// UIConfig configures the observation layer's transport rate.
type UIConfig struct {
	// FrameRateHz is how often a Frame is aggregated and emitted.
	FrameRateHz float64 `yaml:"frame_rate_hz"`
}

// LogConfig configures logging.
type LogConfig struct {
	Level string `yaml:"level"`
}

// Default returns the built-in defaults. configs/default.yaml documents the
// same values.
func Default() Config {
	return Config{
		Cache: CacheConfig{
			Capacity: 100 * 1000 * 1000,
			Policy:   "lru",
		},
		Events: EventsConfig{
			BusBuffer:         1024,
			MetricsBuffer:     8192,
			FrameBuffer:       256,
			RequestSampleRate: 1000,
		},
		UI:       UIConfig{FrameRateHz: 10},
		Log:      LogConfig{Level: "info"},
		Features: DefaultFeatureFlags(),
	}
}

// Parse decodes YAML over the defaults and validates the result. Fields absent
// from the document keep their default value.
func Parse(data []byte) (*Config, error) {
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}
	return cfg, nil
}

// FrameInterval returns the wall-clock period between frames.
func (c *Config) FrameInterval() time.Duration {
	if c.UI.FrameRateHz <= 0 {
		return 100 * time.Millisecond
	}
	return time.Duration(float64(time.Second) / c.UI.FrameRateHz)
}
