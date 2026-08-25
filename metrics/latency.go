package metrics

import (
	"math/bits"
	"sync/atomic"
	"time"
)

const (
	// subBits is the number of significant bits kept below the leading bit of
	// a recorded value. 4 bits -> 16 linear sub-buckets per power of two.
	subBits  = 4
	subCount = 1 << subBits
	// numBuckets covers the whole uint64 range: 64 exponents x 16 sub-buckets.
	numBuckets = (64 - subBits + 1) * subCount
)

// Histogram is a lock-free, bucketed latency histogram over nanosecond values.
//
// ACCURACY TRADE-OFF: values are bucketed log-linearly — 16 linear buckets per
// power of two — so a recorded value is stored to within 1/16 = 6.25% relative
// error (values below 16ns are exact). Reported quantiles are therefore
// accurate to ~6% of the true value, which is far finer than the differences
// this project reports on, and the bounded 976-bucket array costs ~8KB with
// no allocation and no lock on the recording path. That matters more than exact
// quantiles: the histogram sits in the request path, and an allocating or
// locking histogram would perturb the very latencies it measures.
//
// Mean is computed from an exact running sum, not from the buckets, so it is
// not subject to bucketing error.
type Histogram struct {
	buckets [numBuckets]atomic.Uint64
	count   atomic.Uint64
	sum     atomic.Uint64 // nanoseconds
	max     atomic.Uint64 // nanoseconds
}

// NewHistogram returns an empty histogram.
func NewHistogram() *Histogram { return &Histogram{} }

// bucketIndex maps a nanosecond value to its bucket.
func bucketIndex(v uint64) int {
	if v < subCount {
		return int(v)
	}
	exp := bits.Len64(v) - 1
	shift := uint(exp - subBits)
	sub := (v >> shift) - subCount
	return int((uint64(shift)+1)*subCount + sub)
}

// bucketMidpoint is the representative value reported for a bucket.
func bucketMidpoint(i int) float64 {
	if i < subCount {
		return float64(i)
	}
	shift := uint(i/subCount) - 1
	sub := uint64(i % subCount)
	lo := (uint64(subCount) + sub) << shift
	width := uint64(1) << shift
	return float64(lo) + float64(width)/2
}

// Record adds one observation. Negative durations are ignored.
func (h *Histogram) Record(d time.Duration) {
	if d < 0 {
		return
	}
	ns := uint64(d)
	h.buckets[bucketIndex(ns)].Add(1)
	h.count.Add(1)
	h.sum.Add(ns)
	for {
		cur := h.max.Load()
		if ns <= cur || h.max.CompareAndSwap(cur, ns) {
			break
		}
	}
}

// Count returns the number of recorded observations.
func (h *Histogram) Count() uint64 { return h.count.Load() }

// MeanMillis returns the exact arithmetic mean in milliseconds.
func (h *Histogram) MeanMillis() float64 {
	n := h.count.Load()
	if n == 0 {
		return 0
	}
	return float64(h.sum.Load()) / float64(n) / 1e6
}

// MaxMillis returns the largest observation in milliseconds.
func (h *Histogram) MaxMillis() float64 { return float64(h.max.Load()) / 1e6 }

// QuantileMillis returns the q-quantile (0..1) in milliseconds, accurate to
// within one bucket width (<=6.25% relative error).
func (h *Histogram) QuantileMillis(q float64) float64 {
	if q < 0 {
		q = 0
	}
	if q > 1 {
		q = 1
	}
	total := h.count.Load()
	if total == 0 {
		return 0
	}
	target := uint64(q * float64(total))
	if target >= total {
		target = total - 1
	}
	var seen uint64
	for i := range h.buckets {
		c := h.buckets[i].Load()
		if c == 0 {
			continue
		}
		seen += c
		if seen > target {
			return bucketMidpoint(i) / 1e6
		}
	}
	return h.MaxMillis()
}

// P50 returns the median in milliseconds.
func (h *Histogram) P50() float64 { return h.QuantileMillis(0.50) }

// P95 returns the 95th percentile in milliseconds.
func (h *Histogram) P95() float64 { return h.QuantileMillis(0.95) }

// P99 returns the 99th percentile in milliseconds.
func (h *Histogram) P99() float64 { return h.QuantileMillis(0.99) }

// Reset clears every observation.
func (h *Histogram) Reset() {
	for i := range h.buckets {
		h.buckets[i].Store(0)
	}
	h.count.Store(0)
	h.sum.Store(0)
	h.max.Store(0)
}
