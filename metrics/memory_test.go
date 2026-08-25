package metrics

import (
	"math"
	"testing"
)

func TestAccountMemory(t *testing.T) {
	b := AccountMemory(10, 10_000, 100, 640)
	if b.EntryBytes != 10*EntryStructBytes() {
		t.Errorf("EntryBytes = %d, want %d", b.EntryBytes, 10*EntryStructBytes())
	}
	want := b.KeyBytes + b.EntryBytes + b.IndexBytes + b.PolicyBytes
	if b.MetadataBytes != want {
		t.Errorf("MetadataBytes = %d, want %d", b.MetadataBytes, want)
	}
	wantRatio := float64(b.MetadataBytes) / float64(b.PayloadBytes+b.MetadataBytes)
	if math.Abs(b.OverheadRatio-wantRatio) > 1e-12 {
		t.Errorf("OverheadRatio = %g, want %g", b.OverheadRatio, wantRatio)
	}
	if b.Runtime.SysBytes == 0 {
		t.Error("runtime memory should have been sampled")
	}
}

func TestAccountMemoryEmptyCache(t *testing.T) {
	b := AccountMemory(0, 0, 0, 0)
	if b.MetadataBytes != 0 || b.OverheadRatio != 0 {
		t.Fatalf("empty cache should report zero overhead, got %+v", b)
	}
}
