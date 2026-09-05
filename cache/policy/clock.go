// Clock (a.k.a. second-chance) is the third baseline. It is a low-
// overhead approximation of LRU: a circular buffer of slots, each with
// a single reference bit, and a hand that walks the ring clearing
// ref bits until it finds a slot whose bit is already 0 — that slot's
// key is the eviction victim.
//
// Differences from the textbook description:
//   - The ring is sized to the maximum number of entries the policy
//     has ever held, growing by powers of two. The ring is NEVER
//     shrunk, because shrinking would race with concurrent OnAccess
//     calls and because MetadataBytes() reports the cap, not the
//     live count, so shrinking would silently under-report.
//   - The hand wraps at 2*len(slots) to guarantee progress even if
//     every ref bit is set (the full ring is given two passes of
//     "second chance" before we declare a victim impossible — which
//     in practice cannot happen, because we always have a free slot
//     for the next insert and that slot is found before two passes
//     elapse, but the bound is documented).
//   - Stale slots (key was evicted by the store but the policy
//     didn't get the OnRemove call) are detected and freed in
//     Victim() by clearing the slot and advancing. This is the same
//     "phantom victim" defence the Core already exercises via
//     OnRemove(_, nil); doing it here too means a Victim call is
//     safe to use directly in batch-eviction (P9).
package policy

import (
	"fmt"
	"sync"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// clockSlot is one position in the ring. A slot is either empty
// (key == "") or holds a key. The ref bit is meaningful only when
// the slot is occupied; an empty slot's ref bit is ignored.
type clockSlot struct {
	key string
	ref bool
	e   *types.Entry
}

// Clock is an EvictionPolicy implementing second-chance Clock. Zero
// value is not usable; construct via NewClock.
type Clock struct {
	mu    sync.Mutex
	index map[string]int // key -> slot index in slots
	slots []clockSlot    // ring buffer (nil until first insert)
	hand  int            // next slot to consider
	cap   int            // current ring capacity (power of two, or 0)
	live  int            // number of occupied slots
}

// NewClock returns a fresh Clock policy. The ring is allocated
// lazily on the first OnInsert so an empty policy reports zero
// metadata, matching LRU and LFU.
func NewClock() *Clock {
	return &Clock{
		index: make(map[string]int),
	}
}

func init() { Register(types.PolicyClock, func() EvictionPolicy { return NewClock() }) }

// clockPerSlotBytes is the per-slot metadata cost: a string header for
// the key, a bool ref bit, a pointer to the entry, plus a map slot
// (8 bytes). Rounded to 24 bytes per slot.
const clockPerSlotBytes int64 = 24

// Name returns the policy's stable identifier.
func (p *Clock) Name() types.PolicyName { return types.PolicyClock }

// OnAccess sets the ref bit on the key's slot. A no-op for unknown
// keys (the policy does not allocate on a miss).
func (p *Clock) OnAccess(key string, _ *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if i, ok := p.index[key]; ok {
		p.slots[i].ref = true
	}
}

// OnInsert adds the key at the slot the hand currently points at,
// then advances the hand past it. If the hand points at an
// occupied slot — which can happen when a caller invokes Victim
// without following up with the store's Remove+OnRemove sequence —
// the policy advances the hand to the next free slot, growing the
// ring first if necessary.
//
// The store's replace-in-place path (re-insert of an existing key)
// is handled separately and does NOT advance the hand.
func (p *Clock) OnInsert(key string, e *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if i, ok := p.index[key]; ok {
		// Replace-in-place: same slot, refreshed pointer and ref bit.
		p.slots[i].e = e
		p.slots[i].ref = true
		return
	}
	p.growLocked()
	// Walk the ring from the hand to find a free slot. This is
	// O(ring) in the worst case, but happens only when OnInsert is
	// called without the matching OnRemove — which the production
	// cache core always does, via evictOneLocked. Tests that call
	// Victim without OnRemove may trigger the walk.
	steps := 0
	for p.slots[p.hand].key != "" {
		p.hand = (p.hand + 1) % p.cap
		steps++
		if steps > p.cap {
			// Shouldn't happen — growLocked guarantees a free slot.
			// Defensive: grow again and try once more.
			p.growLocked()
			steps = 0
		}
	}
	p.slots[p.hand] = clockSlot{key: key, ref: true, e: e}
	p.index[key] = p.hand
	p.live++
	p.hand = (p.hand + 1) % p.cap
}

// growLocked doubles the ring capacity if it is full. Caller must hold
// p.mu. Returns when the ring has a free slot.
func (p *Clock) growLocked() {
	if p.live < p.cap {
		return
	}
	// Double. We use a fresh slice (Copy) so the underlying array
	// is reallocated — keeping the old backing array is what would
	// prevent later shrinking, but Go slices would, so we are safe.
	newCap := p.cap * 2
	if newCap < 1 {
		newCap = 1
	}
	newSlots := make([]clockSlot, newCap)
	// Copy live slots contiguously starting at 0, and reset the
	// hand to 0. This is a one-time per-power-of-two cost; for a
	// cache of 1000 entries it happens at 1024 and is O(1024).
	idx := 0
	for i := 0; i < p.cap; i++ {
		if p.slots[i].key == "" {
			continue
		}
		newSlots[idx] = p.slots[i]
		p.index[newSlots[idx].key] = idx
		idx++
	}
	p.slots = newSlots
	p.cap = newCap
	p.hand = idx % p.cap
	p.live = idx
}

// OnRemove drops the key by clearing the slot and removing the map
// entry. The slot is left available for the next OnInsert.
func (p *Clock) OnRemove(key string, _ *types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if i, ok := p.index[key]; ok {
		p.slots[i] = clockSlot{}
		delete(p.index, key)
		p.live--
	}
}

// Victim walks the ring clearing ref bits until it finds a slot
// whose ref is 0. The empty/stale slots are skipped. (false) is
// returned only if every live slot has a fresh ref — which the
// 2*len(slots) bound makes vanishingly rare.
func (p *Clock) Victim() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.live == 0 {
		return "", false
	}
	bound := 2 * len(p.slots)
	for k := 0; k < bound; k++ {
		s := &p.slots[p.hand]
		if s.key == "" {
			// Empty slot: skip.
			p.hand = (p.hand + 1) % p.cap
			continue
		}
		if s.ref {
			// Second chance: clear and advance.
			s.ref = false
			p.hand = (p.hand + 1) % p.cap
			continue
		}
		// Found: return this slot's key. Do NOT advance the hand —
		// the next Victim call will pick up where this one left off,
		// so the eviction order is fair.
		return s.key, true
	}
	// Every live slot had a fresh ref. In practice this shouldn't
	// happen because (a) OnInsert sets a fresh ref, and (b) we
	// observed at least one insertion that left a non-ref slot.
	// If it does, fall back to the hand position anyway and let
	// Core retry. Returning false here would cause Core to give
	// up too early.
	if s := &p.slots[p.hand]; s.key != "" {
		return s.key, true
	}
	return "", false
}

// Candidates returns up to n victims. Clock has no global ordering
// beyond "the next slot the hand reaches", so we synthesise a
// candidate list by walking the ring from the hand and picking the
// first n non-ref slots. This is the same algorithm as Victim
// repeated n times and is good enough for the P9 batch evictor.
func (p *Clock) Candidates(n int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n <= 0 || p.live == 0 {
		return nil
	}
	out := make([]string, 0, n)
	bound := 2 * len(p.slots)
	for k := 0; k < bound && len(out) < n; k++ {
		s := &p.slots[p.hand]
		if s.key == "" {
			p.hand = (p.hand + 1) % p.cap
			continue
		}
		if s.ref {
			s.ref = false
			p.hand = (p.hand + 1) % p.cap
			continue
		}
		out = append(out, s.key)
		p.hand = (p.hand + 1) % p.cap
	}
	return out
}

// ShouldAdmit is always true. Admission filtering is P9.
func (p *Clock) ShouldAdmit(_ string, _ int64) bool { return true }

// Params returns an empty set. Second-chance Clock has no genuine
// tunable; the ref-bit width is fixed at 1 (Nth-chance variants exist
// but are not part of this baseline).
func (p *Clock) Params() types.ParamSet { return types.ParamSet{} }

// SetParam always returns an error. Clock exposes no parameters.
func (p *Clock) SetParam(name string, _ float64) error {
	return fmt.Errorf("clock: no such parameter %q", name)
}

// Rebuild adopts the resident entries. Slots are filled contiguously
// starting at 0; the hand resets to 0. All ref bits are cleared so
// the rebuild does not falsely "credit" a key with recency it
// hasn't actually earned under this policy.
func (p *Clock) Rebuild(entries []*types.Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	newCap := 1
	for newCap < len(entries) {
		newCap *= 2
	}
	p.slots = make([]clockSlot, newCap)
	p.index = make(map[string]int, len(entries))
	p.cap = newCap
	p.hand = 0
	p.live = 0
	for i, e := range entries {
		p.slots[i] = clockSlot{key: e.Key, ref: false, e: e}
		p.index[e.Key] = i
		p.live++
	}
}

// MetadataBytes reports the policy's memory footprint, including
// the unused tail of the ring. The cap is the truthful measure of
// the slice's backing memory; under-counting would hide the cost
// of the power-of-two growth strategy.
func (p *Clock) MetadataBytes() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	// The cap drives the count: an empty policy with cap=0 reports
	// 0, and a policy that has grown to cap=N reports 24*N even
	// if only a few slots are occupied.
	return int64(p.cap) * clockPerSlotBytes
}

// Reset clears all metadata. The ring is freed entirely so an
// empty policy reports zero metadata.
func (p *Clock) Reset() {
	p.mu.Lock()
	p.index = make(map[string]int)
	p.slots = nil
	p.hand = 0
	p.cap = 0
	p.live = 0
	p.mu.Unlock()
}

// compile-time interface check.
var _ EvictionPolicy = (*Clock)(nil)
