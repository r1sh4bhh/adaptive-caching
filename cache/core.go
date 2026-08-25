package cache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/cache/policy"
	"github.com/r1sh4bhh/adaptive-caching/events"
	"github.com/r1sh4bhh/adaptive-caching/metrics"
	"github.com/r1sh4bhh/adaptive-caching/types"
)

// Options configures a Core. Every field except Capacity is optional.
type Options struct {
	// Capacity is the byte capacity of the cache.
	Capacity int64
	// Policy is the eviction policy. It may be nil: with no policy the cache
	// still serves, counts and publishes, and simply refuses inserts that
	// would exceed capacity.
	Policy policy.EvictionPolicy
	// Bus receives hit/miss/eviction events. May be nil.
	Bus events.Bus
	// Metrics is the aggregate collector. If nil, one is created.
	Metrics *metrics.Collector
	// RequestSampleRate emits one event per N requests. Zero means every
	// request.
	RequestSampleRate uint64
	// Now overrides the clock, for deterministic tests.
	Now func() time.Time
}

// Core wires the object store, the eviction policy, the metrics collector and
// the event bus together. It is safe for concurrent use.
//
// The policy field is pluggable and nil-safe by design: P1 ships no policies,
// and P7 swaps policies at runtime while the store — and therefore the cached
// objects — stay exactly where they are.
type Core struct {
	mu     sync.Mutex
	store  *Store
	policy policy.EvictionPolicy

	bus        events.Bus
	metrics    *metrics.Collector
	sampleRate uint64
	now        func() time.Time

	seq atomic.Uint64
}

// compile-time assertion that Core satisfies the frozen Cache interface.
var _ Cache = (*Core)(nil)

// New constructs a cache core.
func New(opts Options) *Core {
	c := &Core{
		store:      NewStore(opts.Capacity),
		policy:     opts.Policy,
		bus:        opts.Bus,
		metrics:    opts.Metrics,
		sampleRate: opts.RequestSampleRate,
		now:        opts.Now,
	}
	if c.metrics == nil {
		c.metrics = metrics.NewCollector()
	}
	if c.sampleRate == 0 {
		c.sampleRate = 1
	}
	if c.now == nil {
		c.now = time.Now
	}
	c.metrics.SetPolicy(c.PolicyName())
	return c
}

// Get looks a key up, recording the hit or miss.
func (c *Core) Get(key string) (types.Value, bool) {
	start := c.now()
	seq := c.seq.Add(1)

	c.mu.Lock()
	e, hit := c.store.Touch(key, start)
	var val types.Value
	var size int64
	if hit {
		val = e.Value
		size = e.Size
		if c.policy != nil {
			c.policy.OnAccess(key, e)
		}
	}
	c.mu.Unlock()

	latency := c.now().Sub(start)
	c.metrics.RecordRequest(types.Request{
		Key:       key,
		Size:      size,
		Timestamp: start,
		RequestID: seq,
		Op:        types.OpGet,
	}, hit, latency)

	if c.sampled(seq) {
		t := events.TypeMiss
		if hit {
			t = events.TypeHit
		}
		c.publish(events.Event{Seq: seq, Timestamp: start, Type: t})
	}
	return val, hit
}

// Put admits an object, evicting as needed via the policy.
//
// With a nil policy nothing can be evicted, so an object that does not fit is
// rejected with ErrCapacityExceeded rather than silently displacing something.
func (c *Core) Put(key string, val types.Value, size int64) error {
	now := c.now()
	seq := c.seq.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.policy != nil && !c.policy.ShouldAdmit(key, size) {
		return nil
	}

	for {
		e, err := c.store.Insert(key, val, size, now)
		if err == nil {
			if c.policy != nil {
				c.policy.OnInsert(key, e)
			}
			// An insert is how a miss gets filled, so this is where the
			// backend bytes are accounted for.
			c.metrics.RecordFetch(size)
			return nil
		}
		if err != ErrCapacityExceeded || c.policy == nil {
			return err
		}
		if !c.evictOneLocked(seq, now) {
			return ErrCapacityExceeded
		}
	}
}

// Remove deletes a key, reporting whether it was present.
func (c *Core) Remove(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeLocked(key)
}

// Contains reports whether a key is resident, without counting an access.
func (c *Core) Contains(key string) bool { return c.store.Contains(key) }

// Len returns the object count.
func (c *Core) Len() int { return c.store.Len() }

// Bytes returns the resident payload bytes.
func (c *Core) Bytes() int64 { return c.store.Bytes() }

// Capacity returns the BYTE capacity.
func (c *Core) Capacity() int64 { return c.store.Capacity() }

// Stats returns a metrics snapshot including current cache occupancy.
func (c *Core) Stats() metrics.Stats {
	mem := c.Memory()
	c.metrics.ObserveCache(metrics.CacheState{
		Capacity:      c.store.Capacity(),
		BytesUsed:     c.store.Bytes(),
		MetadataBytes: mem.MetadataBytes,
		ObjectCount:   mem.ObjectCount,
	})
	return c.metrics.Snapshot()
}

// Memory returns the payload-versus-metadata breakdown.
func (c *Core) Memory() metrics.MemoryBreakdown {
	c.mu.Lock()
	var policyBytes int64
	if c.policy != nil {
		policyBytes = c.policy.MetadataBytes()
	}
	c.mu.Unlock()
	return metrics.AccountMemory(c.store.Len(), c.store.Bytes(), c.store.KeyBytes(), policyBytes)
}

// Clear empties the cache and the policy metadata. Metrics are untouched.
func (c *Core) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store.Clear()
	if c.policy != nil {
		c.policy.Reset()
	}
}

// PolicyName returns the installed policy's name, or PolicyNone.
func (c *Core) PolicyName() types.PolicyName {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.policy == nil {
		return types.PolicyNone
	}
	return c.policy.Name()
}

// SetPolicy installs a policy (or nil) and rebuilds its metadata from the
// objects already resident. The store — and therefore every cached object —
// is untouched, which is what makes switching cheap in P7.
func (c *Core) SetPolicy(p policy.EvictionPolicy) {
	c.mu.Lock()
	c.policy = p
	if p != nil {
		p.Reset()
		p.Rebuild(c.store.Entries())
	}
	name := types.PolicyNone
	if p != nil {
		name = p.Name()
	}
	c.mu.Unlock()
	c.metrics.SetPolicy(name)
}

// Metrics exposes the collector, so a caller can write CSV/JSON output.
func (c *Core) Metrics() *metrics.Collector { return c.metrics }

// Seq returns the number of operations served so far.
func (c *Core) Seq() uint64 { return c.seq.Load() }

// evictOneLocked evicts the policy's chosen victim. The caller must hold c.mu.
func (c *Core) evictOneLocked(seq uint64, now time.Time) bool {
	victim, ok := c.policy.Victim()
	if !ok {
		return false
	}
	e, removed := c.store.Remove(victim)
	if !removed {
		// The policy's metadata disagrees with the store; drop the stale
		// metadata so we cannot loop forever on the same phantom victim.
		c.policy.OnRemove(victim, nil)
		return false
	}
	c.policy.OnRemove(victim, e)
	c.metrics.RecordEviction(victim, e.Size)
	c.publish(events.Event{Seq: seq, Timestamp: now, Type: events.TypeEviction})
	return true
}

// removeLocked deletes a key. The caller must hold c.mu.
func (c *Core) removeLocked(key string) bool {
	e, ok := c.store.Remove(key)
	if !ok {
		return false
	}
	if c.policy != nil {
		c.policy.OnRemove(key, e)
	}
	return true
}

// sampled reports whether the event for this sequence number should be
// published. At high request rates an unsampled bus would dominate the CPU
// profile; aggregate counters live in the metrics collector instead.
func (c *Core) sampled(seq uint64) bool { return seq%c.sampleRate == 0 }

// publish sends an event if a bus is configured. Publish never blocks.
func (c *Core) publish(e events.Event) {
	if c.bus != nil {
		c.bus.Publish(e)
	}
}
