package metrics

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/events"
	"github.com/r1sh4bhh/adaptive-caching/types"
)

func TestCollectorImplementsInterface(t *testing.T) {
	var _ MetricsCollector = NewCollector()
}

func TestRecordRequestRates(t *testing.T) {
	c := NewCollector()
	c.RecordRequest(types.Request{Key: "a", Size: 100}, true, time.Millisecond)
	c.RecordRequest(types.Request{Key: "b", Size: 300}, false, 2*time.Millisecond)
	c.RecordRequest(types.Request{Key: "c", Size: 100}, true, time.Millisecond)

	s := c.Snapshot()
	if s.TotalRequests != 3 || s.Hits != 2 || s.Misses != 1 {
		t.Fatalf("got total=%d hits=%d misses=%d, want 3/2/1", s.TotalRequests, s.Hits, s.Misses)
	}
	if want := 2.0 / 3.0; s.HitRate != want {
		t.Errorf("HitRate = %g, want %g", s.HitRate, want)
	}
	if want := 200.0 / 500.0; s.ByteHitRate != want {
		t.Errorf("ByteHitRate = %g, want %g", s.ByteHitRate, want)
	}
	if s.BackendRequests != 1 {
		t.Errorf("BackendRequests = %d, want 1", s.BackendRequests)
	}
	if s.LatencyMean <= 0 {
		t.Errorf("LatencyMean = %g, want > 0", s.LatencyMean)
	}
}

func TestRecordEvictionSwitchTuningAndReset(t *testing.T) {
	c := NewCollector()
	c.RecordEviction("a", 512)
	c.RecordSwitch(events.SwitchEvent{From: types.PolicyLRU, To: types.PolicyARC, OverheadMs: 2.5})
	c.RecordTuning(events.TuningEvent{Accepted: true})
	c.RecordTuning(events.TuningEvent{Accepted: false})
	c.ObserveCache(CacheState{Capacity: 1000, BytesUsed: 512, MetadataBytes: 64, ObjectCount: 2})

	s := c.Snapshot()
	if s.Evictions != 1 {
		t.Errorf("Evictions = %d, want 1", s.Evictions)
	}
	if s.PolicySwitches != 1 || s.CurrentPolicy != types.PolicyARC {
		t.Errorf("got switches=%d policy=%s, want 1/arc", s.PolicySwitches, s.CurrentPolicy)
	}
	if s.SwitchOverheadTotalMs != 2.5 {
		t.Errorf("SwitchOverheadTotalMs = %g, want 2.5", s.SwitchOverheadTotalMs)
	}
	if attempts, accepted := c.TuningAttempts(); attempts != 2 || accepted != 1 {
		t.Errorf("tuning attempts=%d accepted=%d, want 2/1", attempts, accepted)
	}
	if s.Capacity != 1000 || s.BytesUsed != 512 || s.ObjectCount != 2 || s.MetadataBytes != 64 {
		t.Errorf("cache state not reflected: %+v", s)
	}

	c.Reset()
	s = c.Snapshot()
	if s.Evictions != 0 || s.PolicySwitches != 0 || s.TotalRequests != 0 || s.LatencyMean != 0 {
		t.Errorf("Reset left counters behind: %+v", s)
	}
}

func TestConcurrentRecording(t *testing.T) {
	c := NewCollector()
	var wg sync.WaitGroup
	const workers, each = 8, 500
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				c.RecordRequest(types.Request{Key: "k", Size: 10}, i%2 == 0, time.Microsecond)
			}
		}(w)
	}
	wg.Wait()
	if got, want := c.Snapshot().TotalRequests, uint64(workers*each); got != want {
		t.Fatalf("TotalRequests = %d, want %d", got, want)
	}
}

func TestWriteCSVAndJSON(t *testing.T) {
	c := NewCollector()
	c.RecordRequest(types.Request{Key: "a", Size: 100}, true, time.Millisecond)

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "stats.csv")
	jsonPath := filepath.Join(dir, "stats.json")

	if err := c.WriteCSV(csvPath); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	if err := c.WriteJSON(jsonPath); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("csv has %d rows, want 2", len(rows))
	}
	if len(rows[0]) != len(CSVHeader()) || len(rows[1]) != len(CSVHeader()) {
		t.Fatalf("csv column count mismatch: header=%d row=%d want=%d",
			len(rows[0]), len(rows[1]), len(CSVHeader()))
	}
	if rows[1][0] != "1" {
		t.Errorf("csv total_requests = %q, want 1", rows[1][0])
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var s Stats
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if s.TotalRequests != 1 || s.Hits != 1 {
		t.Errorf("json snapshot = %+v, want 1 request / 1 hit", s)
	}
}
