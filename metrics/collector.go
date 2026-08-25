// Package metrics collects the numbers this project is ultimately judged on:
// hit rates, latency distributions, memory overhead and adaptation behaviour.
//
// Counters are atomics so that recording is cheap and lock-free in the request
// path; anything that must stop the world (memory sampling) happens on the
// frame tick instead.
package metrics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/events"
	"github.com/r1sh4bhh/adaptive-caching/types"
)

// Stats is an immutable snapshot of everything the collector knows.
// Latencies are in milliseconds.
type Stats struct {
	TotalRequests uint64  `json:"total_requests"`
	Hits          uint64  `json:"hits"`
	Misses        uint64  `json:"misses"`
	HitRate       float64 `json:"hit_rate"`
	ByteHitRate   float64 `json:"byte_hit_rate"`
	BytesServed   int64   `json:"bytes_served"`
	BytesFetched  int64   `json:"bytes_fetched"`

	LatencyMean float64 `json:"latency_mean_ms"`
	LatencyP50  float64 `json:"latency_p50_ms"`
	LatencyP95  float64 `json:"latency_p95_ms"`
	LatencyP99  float64 `json:"latency_p99_ms"`

	Capacity      int64 `json:"capacity"`
	BytesUsed     int64 `json:"bytes_used"`
	MetadataBytes int64 `json:"metadata_bytes"`
	ObjectCount   int   `json:"object_count"`

	CurrentPolicy         types.PolicyName `json:"current_policy"`
	PolicySwitches        uint64           `json:"policy_switches"`
	SwitchOverheadTotalMs float64          `json:"switch_overhead_total_ms"`
	DetectionDelayMean    float64          `json:"detection_delay_mean"`
	DetectionAccuracy     float64          `json:"detection_accuracy"`

	Evictions       uint64 `json:"evictions"`
	BackendRequests uint64 `json:"backend_requests"`
}

// MetricsCollector is the aggregate-metrics sink. It is deliberately separate
// from the event bus: aggregate counting must be lossless and cheap, whereas
// the bus is sampled and droppable.
type MetricsCollector interface {
	RecordRequest(r types.Request, hit bool, latency time.Duration)
	RecordEviction(key string, size int64)
	RecordSwitch(events.SwitchEvent)
	RecordTuning(events.TuningEvent)

	Snapshot() Stats
	Reset()
	WriteCSV(path string) error
	WriteJSON(path string) error
}

// CacheState is the cache-side information the collector cannot derive on its
// own. The cache pushes it on the frame tick via ObserveCache.
type CacheState struct {
	Capacity      int64
	BytesUsed     int64
	MetadataBytes int64
	ObjectCount   int
}

// Collector is the default MetricsCollector implementation. It is safe for
// concurrent use.
type Collector struct {
	total        atomic.Uint64
	hits         atomic.Uint64
	misses       atomic.Uint64
	bytesServed  atomic.Int64
	bytesFetched atomic.Int64
	evictions    atomic.Uint64
	evictedBytes atomic.Int64
	backendReqs  atomic.Uint64

	switches       atomic.Uint64
	switchOverhead atomic.Uint64 // microseconds, kept integral for atomicity
	tuningAttempts atomic.Uint64
	tuningAccepted atomic.Uint64

	latency *Histogram

	mu     sync.RWMutex
	policy types.PolicyName
	state  CacheState
}

// NewCollector returns an empty collector.
func NewCollector() *Collector {
	return &Collector{
		latency: NewHistogram(),
		policy:  types.PolicyNone,
	}
}

// RecordRequest records one completed request.
func (c *Collector) RecordRequest(r types.Request, hit bool, latency time.Duration) {
	c.total.Add(1)
	if hit {
		c.hits.Add(1)
		c.bytesServed.Add(r.Size)
	} else {
		c.misses.Add(1)
		c.bytesFetched.Add(r.Size)
		c.backendReqs.Add(1)
	}
	c.latency.Record(latency)
}

// RecordFetch records bytes fetched from the backend to fill a miss. Get
// cannot do this itself: the cache does not know how large a missing object is
// until the caller inserts it, so byte-hit-rate accounting needs this hook.
func (c *Collector) RecordFetch(size int64) {
	c.bytesFetched.Add(size)
}

// RecordEviction records one evicted object.
func (c *Collector) RecordEviction(_ string, size int64) {
	c.evictions.Add(1)
	c.evictedBytes.Add(size)
}

// RecordSwitch records a completed policy switch and its overhead.
func (c *Collector) RecordSwitch(e events.SwitchEvent) {
	c.switches.Add(1)
	if e.OverheadMs > 0 {
		c.switchOverhead.Add(uint64(e.OverheadMs * 1000))
	}
	c.mu.Lock()
	c.policy = e.To
	c.mu.Unlock()
}

// RecordTuning records a parameter-tuning attempt.
func (c *Collector) RecordTuning(e events.TuningEvent) {
	c.tuningAttempts.Add(1)
	if e.Accepted {
		c.tuningAccepted.Add(1)
	}
}

// ObserveCache updates the cache-side fields of the snapshot.
func (c *Collector) ObserveCache(s CacheState) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

// SetPolicy records which policy is currently installed.
func (c *Collector) SetPolicy(name types.PolicyName) {
	c.mu.Lock()
	c.policy = name
	c.mu.Unlock()
}

// TuningAttempts returns the number of tuning proposals observed, and how many
// were accepted.
func (c *Collector) TuningAttempts() (attempts, accepted uint64) {
	return c.tuningAttempts.Load(), c.tuningAccepted.Load()
}

// Snapshot returns the current statistics.
func (c *Collector) Snapshot() Stats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	served := c.bytesServed.Load()
	fetched := c.bytesFetched.Load()

	s := Stats{
		TotalRequests:         c.total.Load(),
		Hits:                  hits,
		Misses:                misses,
		BytesServed:           served,
		BytesFetched:          fetched,
		LatencyMean:           c.latency.MeanMillis(),
		LatencyP50:            c.latency.P50(),
		LatencyP95:            c.latency.P95(),
		LatencyP99:            c.latency.P99(),
		PolicySwitches:        c.switches.Load(),
		SwitchOverheadTotalMs: float64(c.switchOverhead.Load()) / 1000,
		Evictions:             c.evictions.Load(),
		BackendRequests:       c.backendReqs.Load(),
	}
	if n := hits + misses; n > 0 {
		s.HitRate = float64(hits) / float64(n)
	}
	if b := served + fetched; b > 0 {
		s.ByteHitRate = float64(served) / float64(b)
	}

	c.mu.RLock()
	s.CurrentPolicy = c.policy
	s.Capacity = c.state.Capacity
	s.BytesUsed = c.state.BytesUsed
	s.MetadataBytes = c.state.MetadataBytes
	s.ObjectCount = c.state.ObjectCount
	c.mu.RUnlock()

	return s
}

// Reset clears every counter. Used to separate warm-up from steady state.
func (c *Collector) Reset() {
	c.total.Store(0)
	c.hits.Store(0)
	c.misses.Store(0)
	c.bytesServed.Store(0)
	c.bytesFetched.Store(0)
	c.evictions.Store(0)
	c.evictedBytes.Store(0)
	c.backendReqs.Store(0)
	c.switches.Store(0)
	c.switchOverhead.Store(0)
	c.tuningAttempts.Store(0)
	c.tuningAccepted.Store(0)
	c.latency.Reset()
}

// csvHeader is the stable column ordering for WriteCSV.
var csvHeader = []string{
	"total_requests", "hits", "misses", "hit_rate", "byte_hit_rate",
	"bytes_served", "bytes_fetched",
	"latency_mean_ms", "latency_p50_ms", "latency_p95_ms", "latency_p99_ms",
	"capacity", "bytes_used", "metadata_bytes", "object_count",
	"current_policy", "policy_switches", "switch_overhead_total_ms",
	"detection_delay_mean", "detection_accuracy",
	"evictions", "backend_requests",
}

// Row renders the snapshot in the stable CSV column order.
func (s Stats) Row() []string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	return []string{
		strconv.FormatUint(s.TotalRequests, 10),
		strconv.FormatUint(s.Hits, 10),
		strconv.FormatUint(s.Misses, 10),
		f(s.HitRate),
		f(s.ByteHitRate),
		strconv.FormatInt(s.BytesServed, 10),
		strconv.FormatInt(s.BytesFetched, 10),
		f(s.LatencyMean),
		f(s.LatencyP50),
		f(s.LatencyP95),
		f(s.LatencyP99),
		strconv.FormatInt(s.Capacity, 10),
		strconv.FormatInt(s.BytesUsed, 10),
		strconv.FormatInt(s.MetadataBytes, 10),
		strconv.Itoa(s.ObjectCount),
		string(s.CurrentPolicy),
		strconv.FormatUint(s.PolicySwitches, 10),
		f(s.SwitchOverheadTotalMs),
		f(s.DetectionDelayMean),
		f(s.DetectionAccuracy),
		strconv.FormatUint(s.Evictions, 10),
		strconv.FormatUint(s.BackendRequests, 10),
	}
}

// CSVHeader returns a copy of the stable CSV column names.
func CSVHeader() []string { return append([]string(nil), csvHeader...) }

// WriteCSV writes the current snapshot as a one-row CSV file with a header.
func (c *Collector) WriteCSV(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write csv %q: %w", path, err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	if err := w.Write(csvHeader); err != nil {
		return fmt.Errorf("write csv %q: %w", path, err)
	}
	if err := w.Write(c.Snapshot().Row()); err != nil {
		return fmt.Errorf("write csv %q: %w", path, err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("write csv %q: %w", path, err)
	}
	return file.Close()
}

// WriteJSON writes the current snapshot as indented JSON.
func (c *Collector) WriteJSON(path string) error {
	data, err := json.MarshalIndent(c.Snapshot(), "", "  ")
	if err != nil {
		return fmt.Errorf("write json %q: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write json %q: %w", path, err)
	}
	return nil
}
