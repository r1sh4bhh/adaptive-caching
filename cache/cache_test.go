package cache

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/cache/policy"
	"github.com/r1sh4bhh/adaptive-caching/events"
	"github.com/r1sh4bhh/adaptive-caching/types"
)

// fifoPolicy is a minimal test double, NOT a shipped policy. P1 implements no
// eviction policies; this exists only to prove that Core drives the interface
// correctly and that the policy holds metadata while the store holds objects.
type fifoPolicy struct {
	mu       sync.Mutex
	order    []string
	admitAll bool
	inserts  int
	removes  int
	accesses int
	rebuilt  int
}

func newFIFO() *fifoPolicy { return &fifoPolicy{admitAll: true} }

func (p *fifoPolicy) Name() types.PolicyName { return types.PolicyName("fifo-test") }

func (p *fifoPolicy) OnAccess(string, *types.Entry) {
	p.mu.Lock()
	p.accesses++
	p.mu.Unlock()
}

func (p *fifoPolicy) OnInsert(key string, _ *types.Entry) {
	p.mu.Lock()
	p.order = append(p.order, key)
	p.inserts++
	p.mu.Unlock()
}

func (p *fifoPolicy) OnRemove(key string, _ *types.Entry) {
	p.mu.Lock()
	for i, k := range p.order {
		if k == key {
			p.order = append(p.order[:i], p.order[i+1:]...)
			break
		}
	}
	p.removes++
	p.mu.Unlock()
}

func (p *fifoPolicy) Victim() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.order) == 0 {
		return "", false
	}
	return p.order[0], true
}

func (p *fifoPolicy) Candidates(n int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n > len(p.order) {
		n = len(p.order)
	}
	return append([]string(nil), p.order[:n]...)
}

func (p *fifoPolicy) ShouldAdmit(string, int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.admitAll
}

func (p *fifoPolicy) Params() types.ParamSet         { return types.ParamSet{} }
func (p *fifoPolicy) SetParam(string, float64) error { return errors.New("no such parameter") }
func (p *fifoPolicy) MetadataBytes() int64           { return int64(len(p.order)) * 16 }
func (p *fifoPolicy) Rebuild(entries []*types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rebuilt++
	for _, e := range entries {
		p.order = append(p.order, e.Key)
	}
}

func (p *fifoPolicy) Reset() {
	p.mu.Lock()
	p.order = nil
	p.mu.Unlock()
}

var _ policy.EvictionPolicy = (*fifoPolicy)(nil)

func TestCoreImplementsCache(t *testing.T) {
	var _ Cache = New(Options{Capacity: 100})
}

func TestCapacityIsInBytes(t *testing.T) {
	c := New(Options{Capacity: 1000})
	if got, want := c.Capacity(), int64(1000); got != want {
		t.Fatalf("Capacity = %d, want %d bytes", got, want)
	}
	if err := c.Put("a", nil, 600); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// A second 600-byte object does not fit, even though only ONE object is
	// resident: capacity is bytes, not object count.
	if err := c.Put("b", nil, 600); !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("Put = %v, want ErrCapacityExceeded", err)
	}
	if got, want := c.Bytes(), int64(600); got != want {
		t.Errorf("Bytes = %d, want %d", got, want)
	}
}

func TestNilPolicyStillRecordsAndPublishes(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	sub := bus.Subscribe("test", 32)

	c := New(Options{Capacity: 100, Bus: bus})
	if got := c.PolicyName(); got != types.PolicyNone {
		t.Fatalf("PolicyName = %s, want none", got)
	}

	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected a miss")
	}
	if err := c.Put("a", []byte("payload"), 10); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if val, ok := c.Get("a"); !ok || string(val) != "payload" {
		t.Fatalf("Get = %q, %v; want payload, true", val, ok)
	}

	s := c.Stats()
	if s.TotalRequests != 2 || s.Hits != 1 || s.Misses != 1 {
		t.Fatalf("stats = %+v, want 2 requests / 1 hit / 1 miss", s)
	}
	if want := 0.5; s.HitRate != want {
		t.Errorf("HitRate = %g, want %g", s.HitRate, want)
	}
	// 10 bytes served from cache, 10 bytes fetched to fill the miss.
	if want := 0.5; s.ByteHitRate != want {
		t.Errorf("ByteHitRate = %g, want %g", s.ByteHitRate, want)
	}
	if s.ObjectCount != 1 || s.BytesUsed != 10 || s.Capacity != 100 {
		t.Errorf("stats occupancy = %+v", s)
	}
	if s.MetadataBytes <= 0 {
		t.Error("metadata bytes should be accounted for")
	}

	seen := map[events.Type]int{}
	for len(sub) > 0 {
		e := <-sub
		seen[e.Type]++
	}
	if seen[events.TypeMiss] != 1 || seen[events.TypeHit] != 1 {
		t.Errorf("published events = %v, want one hit and one miss", seen)
	}
}

func TestNilPolicyRejectsOversizedAndOverCapacity(t *testing.T) {
	c := New(Options{Capacity: 100})
	if err := c.Put("huge", nil, 101); !errors.Is(err, ErrObjectTooLarge) {
		t.Errorf("Put oversized = %v, want ErrObjectTooLarge", err)
	}
	if err := c.Put("a", nil, 90); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := c.Put("b", nil, 20); !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("Put over capacity = %v, want ErrCapacityExceeded", err)
	}
	if c.Contains("b") {
		t.Error("rejected object must not be resident")
	}
}

func TestPolicyDrivesEviction(t *testing.T) {
	p := newFIFO()
	bus := events.NewBus()
	defer bus.Close()
	sub := bus.Subscribe("evictions", 16, events.TypeEviction)

	c := New(Options{Capacity: 100, Policy: p, Bus: bus})
	for i := 0; i < 5; i++ {
		if err := c.Put(fmt.Sprintf("k%d", i), nil, 30); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}

	if got, want := c.Len(), 3; got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
	if c.Bytes() > c.Capacity() {
		t.Fatalf("Bytes %d exceeds Capacity %d", c.Bytes(), c.Capacity())
	}
	if c.Contains("k0") || c.Contains("k1") {
		t.Error("FIFO should have evicted the oldest keys")
	}
	if got := c.Stats().Evictions; got != 2 {
		t.Errorf("Evictions = %d, want 2", got)
	}
	if len(sub) != 2 {
		t.Errorf("published %d eviction events, want 2", len(sub))
	}
	if p.removes != 2 {
		t.Errorf("policy saw %d removals, want 2", p.removes)
	}
}

func TestPolicyCanRejectAdmission(t *testing.T) {
	p := newFIFO()
	p.admitAll = false
	c := New(Options{Capacity: 100, Policy: p})
	if err := c.Put("a", nil, 10); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if c.Contains("a") {
		t.Error("policy refused admission, object must not be resident")
	}
}

func TestPolicyReceivesAccessHooks(t *testing.T) {
	p := newFIFO()
	c := New(Options{Capacity: 100, Policy: p})
	if err := c.Put("a", nil, 10); err != nil {
		t.Fatalf("Put: %v", err)
	}
	c.Get("a")
	c.Get("nope")
	if p.inserts != 1 || p.accesses != 1 {
		t.Errorf("policy hooks: inserts=%d accesses=%d, want 1/1", p.inserts, p.accesses)
	}
	if !c.Remove("a") || p.removes != 1 {
		t.Errorf("Remove did not reach the policy: removes=%d", p.removes)
	}
	if c.Remove("a") {
		t.Error("removing a missing key should report false")
	}
}

func TestSetPolicyPreservesObjects(t *testing.T) {
	c := New(Options{Capacity: 100})
	if err := c.Put("a", []byte("v"), 10); err != nil {
		t.Fatalf("Put: %v", err)
	}

	p := newFIFO()
	c.SetPolicy(p)

	// The store is untouched; only the policy metadata is rebuilt.
	if val, ok := c.Get("a"); !ok || string(val) != "v" {
		t.Fatalf("object lost across policy install: %q %v", val, ok)
	}
	if p.rebuilt != 1 || len(p.order) != 1 {
		t.Errorf("policy not rebuilt from resident entries: rebuilt=%d order=%v", p.rebuilt, p.order)
	}
	if got := c.PolicyName(); got != types.PolicyName("fifo-test") {
		t.Errorf("PolicyName = %s", got)
	}

	c.SetPolicy(nil)
	if got := c.PolicyName(); got != types.PolicyNone {
		t.Errorf("PolicyName after clearing = %s, want none", got)
	}
	if !c.Contains("a") {
		t.Error("object lost when the policy was removed")
	}
}

func TestEventSampling(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	sub := bus.Subscribe("sampled", 64, events.TypeHit, events.TypeMiss)

	c := New(Options{Capacity: 1000, Bus: bus, RequestSampleRate: 10})
	for i := 0; i < 100; i++ {
		c.Get("missing")
	}
	if got, want := len(sub), 10; got != want {
		t.Fatalf("published %d events at 1-in-10 sampling, want %d", got, want)
	}
	if got := c.Stats().TotalRequests; got != 100 {
		t.Errorf("aggregate counters must be unsampled: TotalRequests = %d, want 100", got)
	}
}

func TestClearAndReplace(t *testing.T) {
	p := newFIFO()
	c := New(Options{Capacity: 100, Policy: p})
	if err := c.Put("a", nil, 10); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := c.Put("a", nil, 40); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got, want := c.Bytes(), int64(40); got != want {
		t.Errorf("Bytes after replace = %d, want %d", got, want)
	}
	if got, want := c.Len(), 1; got != want {
		t.Errorf("Len after replace = %d, want %d", got, want)
	}

	c.Clear()
	if c.Len() != 0 || c.Bytes() != 0 || len(p.order) != 0 {
		t.Error("Clear left state behind")
	}
}

func TestConcurrentUse(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()
	_ = bus.Subscribe("stalled", 1)

	c := New(Options{Capacity: 10_000, Policy: newFIFO(), Bus: bus, RequestSampleRate: 3})

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				key := fmt.Sprintf("k%d", (w*i)%50)
				if err := c.Put(key, nil, 100); err != nil && !errors.Is(err, ErrCapacityExceeded) {
					t.Errorf("Put: %v", err)
					return
				}
				c.Get(key)
				c.Contains(key)
				_ = c.Stats()
			}
		}(w)
	}
	wg.Wait()

	if c.Bytes() > c.Capacity() {
		t.Fatalf("capacity violated under concurrency: %d > %d", c.Bytes(), c.Capacity())
	}
}

func TestDeterministicClock(t *testing.T) {
	now := time.Unix(0, 0)
	c := New(Options{Capacity: 100, Now: func() time.Time { return now }})
	if err := c.Put("a", nil, 1); err != nil {
		t.Fatalf("Put: %v", err)
	}
	c.Get("a")
	if got := c.Stats().LatencyMean; got != 0 {
		t.Errorf("frozen clock should yield zero latency, got %g", got)
	}
	if got, want := c.Seq(), uint64(2); got != want {
		t.Errorf("Seq = %d, want %d", got, want)
	}
}
