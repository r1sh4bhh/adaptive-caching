package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseByteSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"100MB", 100 * 1000 * 1000},
		{"1GB", 1000 * 1000 * 1000},
		{"512KiB", 512 * 1024},
		{"2MiB", 2 * 1024 * 1024},
		{"1024", 1024},
		{"  64kb ", 64 * 1000},
		{"1.5MB", 1_500_000},
		{"4096B", 4096},
	}
	for _, c := range cases {
		got, err := ParseByteSize(c.in)
		if err != nil {
			t.Fatalf("ParseByteSize(%q) error: %v", c.in, err)
		}
		if got.Bytes() != c.want {
			t.Errorf("ParseByteSize(%q) = %d, want %d", c.in, got.Bytes(), c.want)
		}
	}

	for _, bad := range []string{"", "MB", "abc", "12XB"} {
		if _, err := ParseByteSize(bad); err == nil {
			t.Errorf("ParseByteSize(%q) expected an error", bad)
		}
	}
}

func TestDefaultsAreValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}
	if cfg.Events.RequestSampleRate != 1000 {
		t.Errorf("default request_sample_rate = %d, want 1000", cfg.Events.RequestSampleRate)
	}
	if cfg.UI.FrameRateHz != 10 {
		t.Errorf("default frame_rate_hz = %g, want 10", cfg.UI.FrameRateHz)
	}
	if got, want := cfg.FrameInterval(), 100*time.Millisecond; got != want {
		t.Errorf("FrameInterval() = %v, want %v", got, want)
	}
	if cfg.Features != (FeatureFlags{}) {
		t.Errorf("all feature flags should default to false, got %+v", cfg.Features)
	}
}

func TestParseOverridesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`
cache:
  capacity: 8MiB
events:
  request_sample_rate: 1
features:
  shadow: true
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := cfg.Cache.Capacity.Bytes(), int64(8*1024*1024); got != want {
		t.Errorf("capacity = %d, want %d", got, want)
	}
	if cfg.Events.RequestSampleRate != 1 {
		t.Errorf("request_sample_rate = %d, want 1", cfg.Events.RequestSampleRate)
	}
	if !cfg.Features.Shadow {
		t.Error("features.shadow should be true")
	}
	// Untouched fields keep their defaults.
	if cfg.Events.BusBuffer != Default().Events.BusBuffer {
		t.Errorf("bus_buffer = %d, want default %d", cfg.Events.BusBuffer, Default().Events.BusBuffer)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("log.level = %q, want default info", cfg.Log.Level)
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	cfg := Default()
	cfg.Cache.Capacity = 0
	cfg.Events.RequestSampleRate = 0
	cfg.UI.FrameRateHz = 0
	cfg.Log.Level = "loud"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, want := range []string{"cache.capacity", "request_sample_rate", "frame_rate_hz", "log.level"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestLoadDefaultYAML(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "configs", "default.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Default()
	if cfg.Cache.Capacity != want.Cache.Capacity {
		t.Errorf("capacity = %v, want %v", cfg.Cache.Capacity, want.Cache.Capacity)
	}
	if *cfg != want {
		t.Errorf("configs/default.yaml = %+v, want it to match Default() %+v", *cfg, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected an error for a missing config file")
	}
}
