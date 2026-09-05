// LRU (least-recently-used) is the canonical baseline eviction policy. It
// keeps a doubly-linked list ordered by recency — head is most recently
// used, tail is least recently used — backed by a map from key to list
// element for O(1) lookup. On every access the touched node is moved to
// the head; on every insert a fresh node is added at the head; the
// eviction victim is always the tail.
//
// The policy holds METADATA ONLY. The store holds the objects; this
// policy never reads or copies an Entry's Value, only its Key and (for
// rebuild seeding) AccessCount.
package policy

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// lruNode is the value type stored in container/list. The container/list
// API uses an interface value for Element.Value, so we wrap in a struct
// rather than a bare string.
type lruNode struct {
	key string
	// e is the live *types.Entry from the store. It is held by pointer
	// only; the policy never mutates it and never owns it. Kept here so
	// the policy could enrich metadata later (e.g. a second-chance
	// variant) without an interface change.
	e *types.Entry
}

// LRU is an EvictionPolicy implementing the textbook LRU. Zero-value is
// NOT usable; construct with NewLRU. The zero-value guard exists for
// future code that may declare an LRU by value.
type LRU struct {
	mu       sync.Mutex
	ll       *list.List
	items    map[string]*list.Element
	n        int
	metadata int64
}

// NewLRU returns a fresh, empty LRU policy. The returned value is
// registered automatically with the package registry, so callers usually
// just call policy.New(types.PolicyLRU).
func NewLRU() *LRU {
	return &LRU{
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}
}

func init() { Register(types.PolicyLRU, func() EvictionPolicy { return NewLRU() }) }

// Name returns the policy's stable identifier.
func (p *LRU) Name() types.PolicyName { return types.PolicyLRU }

// OnAccess moves the entry to the head of the recency list, marking it
// most-recently-used. A no-op if the policy has no metadata for key
// (which would be a store/policy disagreement).
func (p *LRU) OnAccess(key string, _ *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if el, ok := p.items[key]; ok {
		p.ll.MoveToFront(el)
	}
}

// OnInsert adds the key at the head, replacing any existing node. The
// store may legitimately call OnInsert with a key that is already in the
// cache (size changes, value replacement), and the policy must NOT
// double-count in that case.
func (p *LRU) OnInsert(key string, e *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if el, ok := p.items[key]; ok {
		// Replace-in-place: keep the recency position. The store already
		// updated the entry; we just refresh the pointer.
		el.Value.(*lruNode).e = e
		return
	}
	el := p.ll.PushFront(&lruNode{key: key, e: e})
	p.items[key] = el
	p.n++
	p.metadata += lruPerEntryBytes
}

// OnRemove drops the key from the metadata. The entry is provided for
// symmetry with the interface and for any future use (e.g. logging);
// LRU does not need it. The e==nil case (the store disagreed with us
// about the victim being present) is treated identically.
func (p *LRU) OnRemove(key string, _ *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if el, ok := p.items[key]; ok {
		p.ll.Remove(el)
		delete(p.items, key)
		p.n--
		p.metadata -= lruPerEntryBytes
	}
}

// Victim returns the least-recently-used key, i.e. the tail of the list.
// (false) on an empty policy.
func (p *LRU) Victim() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	el := p.ll.Back()
	if el == nil {
		return "", false
	}
	return el.Value.(*lruNode).key, true
}

// Candidates returns up to n eviction candidates, worst-first (least
// recently used first). Used by batch eviction in P9; LRU's batch order
// is its single-victim order repeated.
func (p *LRU) Candidates(n int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n <= 0 || p.n == 0 {
		return nil
	}
	if n > p.n {
		n = p.n
	}
	out := make([]string, 0, n)
	for e, i := p.ll.Back(), 0; e != nil && i < n; e, i = e.Prev(), i+1 {
		out = append(out, e.Value.(*lruNode).key)
	}
	return out
}

// ShouldAdmit is always true for LRU. There is no admission filter at
// the policy level; size-aware admission arrives in P9 as a separate
// scoring layer.
func (p *LRU) ShouldAdmit(_ string, _ int64) bool { return true }

// Params returns an empty set. LRU has no genuine tunable parameters —
// the recency-weight α in context.md §5.1 is only meaningful in hybrid
// scoring modes that LRU does not implement, so exposing it would
// violate the "no parameters that do not meaningfully affect the
// implementation" rule.
func (p *LRU) Params() types.ParamSet { return types.ParamSet{} }

// SetParam always returns an error. LRU exposes no parameters; the
// error message is the auditor's friend.
func (p *LRU) SetParam(name string, _ float64) error {
	return fmt.Errorf("lru: no such parameter %q", name)
}

// Rebuild adopts the resident entries. The current call order in
// cache.Core is OnInsert(...) per entry, so the recency order of the
// rebuilt list is determined by Go's map iteration — UNDEFINED, but
// correct: any recency ordering over the same set of keys is
// internally consistent for the eviction contract, and P7's switcher
// only requires that the policy be ready to serve Victims, not that
// it preserve the previous policy's notion of recency.
//
// AccessCount is ignored: LRU's only recency signal is the
// (re)ordering done by OnAccess, and Rebuild's caller is the cache
// core, which immediately starts receiving real access events.
func (p *LRU) Rebuild(entries []*types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ll = list.New()
	p.items = make(map[string]*list.Element, len(entries))
	p.n = 0
	p.metadata = 0
	for _, e := range entries {
		el := p.ll.PushFront(&lruNode{key: e.Key, e: e})
		p.items[e.Key] = el
		p.n++
		p.metadata += lruPerEntryBytes
	}
}

// MetadataBytes reports the policy's memory footprint, conservatively
// estimated. The constant deliberately errs large so the
// <5% metadata-overhead target is not gamed by an unrealistically
// tight estimate.
func (p *LRU) MetadataBytes() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.metadata == 0 {
		// Re-derive if the invariant was ever broken (e.g. a test
		// that called Reset+Rebuild out of order).
		p.metadata = int64(p.n) * lruPerEntryBytes
	}
	return p.metadata
}

// Reset discards all metadata. Used by Clear() and by SetPolicy(nil).
func (p *LRU) Reset() {
	p.mu.Lock()
	p.ll = list.New()
	p.items = make(map[string]*list.Element)
	p.n = 0
	p.metadata = 0
	p.mu.Unlock()
}

// lruPerEntryBytes is the conservative per-entry metadata cost: two
// list.Element pointers (next/prev), the key string header, a pointer
// to the list element, and the map slot overhead. Rounded up to 64
// bytes per entry.
const lruPerEntryBytes int64 = 64

// Compile-time assertion that *LRU satisfies the frozen interface.
var _ EvictionPolicy = (*LRU)(nil)
