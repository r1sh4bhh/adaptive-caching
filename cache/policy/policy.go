// Package policy defines the strategy interface every eviction policy
// implements. No policy is implemented in P1 — LRU, LFU and Clock arrive in
// P2, ARC and W-TinyLFU in P4.
package policy

import (
	"github.com/r1sh4bhh/adaptive-caching/types"
)

// EvictionPolicy is the interchangeable-strategy interface that lets the
// adaptive engine treat LRU, LFU, ARC, W-TinyLFU, Clock and anything added
// later as equivalent.
//
// KEY DESIGN POINT — THE POLICY HOLDS METADATA, THE STORE HOLDS OBJECTS.
// A policy never owns, allocates or frees the cached values. It maintains only
// the bookkeeping it needs to answer Victim/Candidates: recency lists,
// frequency counters, sketches, clock hands. This separation is what makes
// state-preserving policy switching possible in P7: when the engine switches
// LRU to ARC, the object store is untouched and only the metadata is rebuilt
// via Rebuild. If a policy owned the objects, every switch would mean
// evacuating and reinserting the whole cache, and switching would cost more
// than it could ever win.
//
// FROZEN after P1: changing this interface requires an ADR in
// docs/DECISIONS.md plus updating all dependents in the same commit.
type EvictionPolicy interface {
	// Name identifies the policy.
	Name() types.PolicyName

	// Lifecycle hooks. The policy maintains metadata only; e is read-only
	// apart from e.PolicyMeta, which the policy owns.
	OnAccess(key string, e *types.Entry)
	OnInsert(key string, e *types.Entry)
	OnRemove(key string, e *types.Entry)

	// Victim returns the single best eviction candidate.
	Victim() (key string, ok bool)
	// Candidates returns up to n eviction candidates, best first, for batch
	// eviction (P9).
	Candidates(n int) []string

	// ShouldAdmit decides whether a missing key is worth inserting at all.
	// TinyLFU-style admission and size-aware admission need this.
	ShouldAdmit(key string, size int64) bool

	// Params exposes the policy's tunable parameters, and SetParam applies a
	// tuner's decision (P10).
	Params() types.ParamSet
	SetParam(name string, v float64) error

	// Rebuild adopts the current cache contents, so a newly installed policy
	// starts from the objects already resident instead of from an empty
	// cache. See context.md §5.4.
	Rebuild(entries []*types.Entry)

	// MetadataBytes reports the policy's own memory footprint, for the
	// overhead metric (target <5%).
	MetadataBytes() int64

	// Reset discards all metadata.
	Reset()
}
