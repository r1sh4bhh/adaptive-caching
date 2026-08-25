package metrics

import (
	"math"
	"testing"
	"time"
)

func TestHistogramEmpty(t *testing.T) {
	h := NewHistogram()
	if h.Count() != 0 || h.MeanMillis() != 0 || h.P99() != 0 {
		t.Fatalf("empty histogram should report zeros, got count=%d mean=%g p99=%g",
			h.Count(), h.MeanMillis(), h.P99())
	}
}

func TestHistogramMeanIsExact(t *testing.T) {
	h := NewHistogram()
	h.Record(1 * time.Millisecond)
	h.Record(3 * time.Millisecond)
	if got, want := h.MeanMillis(), 2.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("MeanMillis = %g, want %g", got, want)
	}
	if h.Count() != 2 {
		t.Fatalf("Count = %d, want 2", h.Count())
	}
}

func TestHistogramQuantilesWithinBucketError(t *testing.T) {
	h := NewHistogram()
	// 1..1000 microseconds, uniformly.
	for i := 1; i <= 1000; i++ {
		h.Record(time.Duration(i) * time.Microsecond)
	}

	check := func(name string, got, wantMicros float64) {
		t.Helper()
		want := wantMicros / 1000 // milliseconds
		// The histogram keeps 4 significant bits, i.e. <=6.25% relative error.
		if rel := math.Abs(got-want) / want; rel > 0.07 {
			t.Errorf("%s = %g ms, want ~%g ms (relative error %.3f)", name, got, want, rel)
		}
	}
	check("p50", h.P50(), 500)
	check("p95", h.P95(), 950)
	check("p99", h.P99(), 990)
	check("mean", h.MeanMillis(), 500.5)

	// Max is stored exactly, not bucketed.
	if got, want := h.MaxMillis(), 1.0; math.Abs(got-want) > 1e-9 {
		t.Errorf("MaxMillis = %g, want %g", got, want)
	}
}

func TestHistogramSmallValuesAreExact(t *testing.T) {
	h := NewHistogram()
	for i := 0; i < 16; i++ {
		h.Record(time.Duration(i))
	}
	if got, want := h.QuantileMillis(0), 0.0; got != want {
		t.Errorf("min quantile = %g, want %g", got, want)
	}
	if got, want := h.QuantileMillis(1), 15.0/1e6; math.Abs(got-want) > 1e-12 {
		t.Errorf("max quantile = %g, want %g", got, want)
	}
}

func TestHistogramIgnoresNegatives(t *testing.T) {
	h := NewHistogram()
	h.Record(-time.Second)
	if h.Count() != 0 {
		t.Fatalf("negative duration should be ignored, count = %d", h.Count())
	}
}

func TestHistogramReset(t *testing.T) {
	h := NewHistogram()
	h.Record(time.Millisecond)
	h.Reset()
	if h.Count() != 0 || h.MaxMillis() != 0 || h.P50() != 0 {
		t.Fatal("Reset did not clear the histogram")
	}
}

func TestBucketIndexMonotonic(t *testing.T) {
	prev := -1
	for _, v := range []uint64{0, 1, 15, 16, 31, 32, 1000, 1 << 20, 1 << 40, 1 << 62} {
		idx := bucketIndex(v)
		if idx <= prev && v != 0 {
			t.Fatalf("bucketIndex(%d) = %d is not increasing (prev %d)", v, idx, prev)
		}
		if idx >= numBuckets {
			t.Fatalf("bucketIndex(%d) = %d exceeds numBuckets %d", v, idx, numBuckets)
		}
		prev = idx
	}
}
