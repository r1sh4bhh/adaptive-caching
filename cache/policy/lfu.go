// LFU (least-frequently-used) is the second baseline. The textbook
// implementation (freq buckets + map) is augmented with an *ageing*
// mechanism (context.md §5.1) to mitigate the cold-start problem:
// freshly inserted items start at frequency 1, and without ageing they
// would be evicted before their first reuse.
//
// Ageing strategy: every `floor(1/decay_lambda)` accesses, all
// frequencies are halved (floored at 1). This is the GFS-LFU approach
// and is bounded-cost: per-request overhead is a single counter
// increment, plus an O(N) full pass once every K requests where
// K = floor(1/lambda). For the default lambda=0.05 and N=1000, the
// halving pass costs 1000 ops every 20 requests — 50 ops per request
// amortised.
//
// Why NOT a background goroutine: a policy owning a goroutine
// complicates Rebuild, Reset and SetPolicy, races with the cache
// core's mutex, and violates the "metadata is policy-owned, store
// holds objects" contract in spirit. The amortised cost is the right
// engineering choice.
package policy

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// maxLFUFreq is the ceiling for any single key's frequency counter.
// Mitigates the "unbounded frequency counters" trap called out in
// context.md §5.1: a pathological key accessed 2^32 times would
// otherwise overflow and wrap to zero, putting it at the bottom of
// the bucket chain.
const maxLFUFreq uint32 = 1 << 16

// lfuNode is the value carried in a bucket list element. The element
// itself is owned by container/list and lives in exactly one bucket
// at a time.
type lfuNode struct {
	key string
	// e is held by pointer; the policy never mutates Entry fields.
	e *types.Entry
}

// LFU is an EvictionPolicy implementing frequency-bucket LFU with
// amortised ageing. Zero value is not usable; construct via NewLFU.
type LFU struct {
	mu       sync.Mutex
	buckets  map[uint32]*list.List // freq -> FIFO of lfuNode
	items    map[string]*lfuItem   // key -> {node, freq, element}
	n        int
	ageCnt   uint64  // accesses since last halving
	ageMax   uint64  // K = floor(1/lambda) (computed from decay_lambda)
	decay    float64 // current decay_lambda
	metadata int64
}

type lfuItem struct {
	el   *list.Element
	freq uint32
}

// NewLFU returns a fresh LFU policy with the default decay lambda.
func NewLFU() *LFU {
	p := &LFU{
		buckets: make(map[uint32]*list.List),
		items:   make(map[string]*lfuItem),
		decay:   defaultDecayLambda,
	}
	p.recomputeAgeMax()
	return p
}

func init() { Register(types.PolicyLFU, func() EvictionPolicy { return NewLFU() }) }

// Default and bounds for decay_lambda. Decay is disabled at 0; raising
// it shortens the effective "memory" of the frequency signal.
const (
	defaultDecayLambda = 0.05
	minDecayLambda     = 0.0
	maxDecayLambda     = 1.0
	decayStep          = 0.01
)

// Name returns the policy's stable identifier.
func (p *LFU) Name() types.PolicyName { return types.PolicyLFU }

// Params returns the policy's tunable parameter. The single parameter,
// decay_lambda, governs the cold-start mitigation.
func (p *LFU) Params() types.ParamSet {
	return types.ParamSet{
		"decay_lambda": &types.Parameter{
			Name:    "decay_lambda",
			Min:     minDecayLambda,
			Max:     maxDecayLambda,
			Default: defaultDecayLambda,
			Current: p.decay,
			Step:    decayStep,
			Metric:  "hit_rate",
		},
	}
}

// SetParam updates decay_lambda. The age counter is reset so the
// new rate takes effect from the next access.
func (p *LFU) SetParam(name string, v float64) error {
	if name != "decay_lambda" {
		return fmt.Errorf("lfu: no such parameter %q", name)
	}
	if v < minDecayLambda || v > maxDecayLambda {
		return fmt.Errorf("lfu: decay_lambda=%g out of [%g, %g]", v, minDecayLambda, maxDecayLambda)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.decay = v
	p.ageCnt = 0
	p.recomputeAgeMaxLocked()
	return nil
}

// recomputeAgeMax recomputes the period between halvings from the
// current decay. Caller must NOT hold the mutex.
func (p *LFU) recomputeAgeMax() {
	p.mu.Lock()
	p.recomputeAgeMaxLocked()
	p.mu.Unlock()
}

// recomputeAgeMaxLocked is recomputeAgeMax's internal helper for
// callers that already hold p.mu.
func (p *LFU) recomputeAgeMaxLocked() {
	if p.decay <= 0 {
		// Disabled: never halve.
		p.ageMax = 0
		return
	}
	k := uint64(1.0 / p.decay)
	if k < 1 {
		k = 1
	}
	p.ageMax = k
}

// OnAccess increments the key's frequency and may trigger an ageing
// pass. A no-op for unknown keys.
func (p *LFU) OnAccess(key string, _ *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	it, ok := p.items[key]
	if !ok {
		return
	}
	p.incrementLocked(key, it)
	if p.ageMax > 0 {
		p.ageCnt++
		if p.ageCnt >= p.ageMax {
			p.ageLocked()
			p.ageCnt = 0
		}
	}
}

// incrementLocked moves the key from its current bucket to the next
// frequency bucket, capping at maxLFUFreq to prevent counter
// overflow. Caller must hold p.mu.
func (p *LFU) incrementLocked(key string, it *lfuItem) {
	oldFreq := it.freq
	newFreq := oldFreq + 1
	if newFreq > maxLFUFreq {
		newFreq = maxLFUFreq
	}
	if oldFreq == newFreq {
		return // already at the ceiling
	}
	// Move the entry from oldFreq bucket to newFreq bucket. Remove
	// detaches the *list.Element from the old list; PushFront
	// allocates a new *list.Element in the new list and returns it.
	// The *lfuNode value rides along; we update it.el to the new
	// element pointer.
	oldBucket := p.buckets[oldFreq]
	oldBucket.Remove(it.el)
	if oldBucket.Len() == 0 {
		delete(p.buckets, oldFreq)
	}
	newBucket := p.buckets[newFreq]
	if newBucket == nil {
		newBucket = list.New()
		p.buckets[newFreq] = newBucket
	}
	it.el = newBucket.PushFront(it.el.Value)
	it.freq = newFreq
}

// OnInsert puts a brand-new key into frequency bucket 1. A re-insert
// of an existing key is treated as an access (the store has just
// updated the entry; we move to a new bucket as if the access
// happened) — this matches LRU's OnInsert semantics and keeps the
// invariant that OnInsert strictly precedes the next OnAccess.
func (p *LFU) OnInsert(key string, e *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if it, ok := p.items[key]; ok {
		// Replace-in-place: keep the current bucket, refresh pointer.
		it.el.Value.(*lfuNode).e = e
		p.incrementLocked(key, it)
		return
	}
	bucket := p.buckets[1]
	if bucket == nil {
		bucket = list.New()
		p.buckets[1] = bucket
	}
	el := bucket.PushFront(&lfuNode{key: key, e: e})
	p.items[key] = &lfuItem{el: el, freq: 1}
	p.n++
	p.metadata += lfuPerEntryBytes
}

// OnRemove drops the key from its bucket and the index.
func (p *LFU) OnRemove(key string, _ *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	it, ok := p.items[key]
	if !ok {
		return
	}
	bucket := p.buckets[it.freq]
	if bucket != nil {
		bucket.Remove(it.el)
		if bucket.Len() == 0 {
			delete(p.buckets, it.freq)
		}
	}
	delete(p.items, key)
	p.n--
	p.metadata -= lfuPerEntryBytes
}

// Victim returns the least-frequently-used key. Within a frequency
// bucket, the tail of the list is the OLDEST insertion at that
// frequency — that is the textbook LRU-within-LFU tie-break.
func (p *LFU) Victim() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	lowest := p.lowestFreqLocked()
	if lowest == 0 {
		return "", false
	}
	bucket := p.buckets[lowest]
	el := bucket.Back()
	if el == nil {
		return "", false
	}
	return el.Value.(*lfuNode).key, true
}

// lowestFreqLocked finds the smallest non-empty frequency bucket.
// Caller must hold p.mu.
func (p *LFU) lowestFreqLocked() uint32 {
	// Buckets are uint32 keys of a map; scanning all keys is O(#buckets),
	// which is O(N) in the worst case but typically tiny (most caches
	// have a small range of active frequencies). For pathological cases
	// this could be optimised with a min-heap of bucket sizes; P11
	// benchmarks will tell us if it matters.
	var best uint32
	seen := false
	for f := range p.buckets {
		if !seen || f < best {
			best = f
			seen = true
		}
	}
	if !seen {
		return 0
	}
	return best
}

// Candidates returns up to n victims, worst-first. Worst-first here
// means: lowest frequency first, oldest-within-bucket first.
func (p *LFU) Candidates(n int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n <= 0 || p.n == 0 {
		return nil
	}
	out := make([]string, 0, n)
	for len(out) < n {
		lowest := p.lowestFreqLocked()
		if lowest == 0 {
			break
		}
		bucket := p.buckets[lowest]
		for e := bucket.Back(); e != nil && len(out) < n; e = e.Prev() {
			out = append(out, e.Value.(*lfuNode).key)
		}
	}
	return out
}

// ShouldAdmit is always true. Size-aware admission is P9.
func (p *LFU) ShouldAdmit(_ string, _ int64) bool { return true }

// ageLocked divides every frequency in half, floored at 1. This is
// the ageing pass that mitigates cold-start (context.md §5.1) and
// stale frequencies (a once-hot key that no longer is). Caller must
// hold p.mu.
func (p *LFU) ageLocked() {
	// Take a snapshot of current frequencies so we can rebuild the
	// bucket map without mutating it during iteration.
	freqs := make([]uint32, 0, len(p.buckets))
	for f := range p.buckets {
		freqs = append(freqs, f)
	}
	for _, oldF := range freqs {
		newF := oldF / 2
		if newF < 1 {
			newF = 1
		}
		if newF == oldF {
			// Frequency 1 stays at 1; nothing to move.
			continue
		}
		bucket := p.buckets[oldF]
		newBucket := p.buckets[newF]
		if newBucket == nil {
			newBucket = list.New()
			p.buckets[newF] = newBucket
		}
		// Move every element from the old bucket into the new bucket.
		// PushFront returns a NEW *list.Element, so we must update
		// each item's pointer to the new element.
		for {
			el := bucket.Front()
			if el == nil {
				break
			}
			bucket.Remove(el)
			node := el.Value.(*lfuNode)
			newEl := newBucket.PushFront(node)
			p.items[node.key].freq = newF
			p.items[node.key].el = newEl
		}
		delete(p.buckets, oldF)
	}
}

// Rebuild adopts the resident entries. Each key's initial frequency
// is min(Entry.AccessCount, maxLFUFreq) — the count survives the
// rebuild, but a key with AccessCount=0 (inserted but never hit)
// gets freq=1, same as a brand-new insert.
func (p *LFU) Rebuild(entries []*types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buckets = make(map[uint32]*list.List)
	p.items = make(map[string]*lfuItem, len(entries))
	p.n = 0
	p.metadata = 0
	p.ageCnt = 0
	for _, e := range entries {
		freq := uint32(e.AccessCount)
		if freq < 1 {
			freq = 1
		}
		if freq > maxLFUFreq {
			freq = maxLFUFreq
		}
		bucket := p.buckets[freq]
		if bucket == nil {
			bucket = list.New()
			p.buckets[freq] = bucket
		}
		node := &lfuNode{key: e.Key, e: e}
		el := bucket.PushFront(node)
		p.items[e.Key] = &lfuItem{el: el, freq: freq}
		p.n++
		p.metadata += lfuPerEntryBytes
	}
}

// MetadataBytes reports the policy's memory footprint, conservatively
// estimated as the per-entry cost times the resident count plus the
// bucket map overhead. The per-entry cost includes the lfuItem struct
// (16 bytes) plus a list element (24 bytes) plus a map slot (8).
const lfuPerEntryBytes int64 = 64

func (p *LFU) MetadataBytes() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.metadata == 0 {
		p.metadata = int64(p.n) * lfuPerEntryBytes
	}
	return p.metadata
}

// Reset clears all metadata.
func (p *LFU) Reset() {
	p.mu.Lock()
	p.buckets = make(map[uint32]*list.List)
	p.items = make(map[string]*lfuItem)
	p.n = 0
	p.metadata = 0
	p.ageCnt = 0
	p.mu.Unlock()
}

// compile-time interface check.
var _ EvictionPolicy = (*LFU)(nil)
