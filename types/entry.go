package types

import "time"

// Value is the payload stored in the cache.
//
// DECISION (P1): Value is a concrete []byte rather than an `any`/interface.
// Rationale:
//   - Byte accounting is the core of this project (capacity is bytes, not
//     object count). A []byte has an exactly measurable payload size; an
//     interface value would require reflection or a caller-supplied size that
//     could silently disagree with reality.
//   - It avoids the 16-byte interface header and the extra allocation an
//     interface boxing would add to every entry, which matters because
//     metadata overhead is itself an evaluation metric (target <5%).
//   - Traces are byte streams anyway; nothing in the roadmap needs typed
//     values.
//
// FROZEN after P1: see docs/DECISIONS.md before changing.
type Value []byte

// Entry is the cache's record for one key. The object store owns Entry values;
// an EvictionPolicy may only read them and stash its own bookkeeping in
// PolicyMeta.
//
// Keep this struct lean: metadata memory overhead is itself an evaluation
// metric (target <5%, acceptable <10%, see context.md §7.1). Every field added
// here is paid for on that chart, once per cached object.
//
// FROZEN after P1: see docs/DECISIONS.md before changing.
type Entry struct {
	Key            string
	Value          Value
	Size           int64
	InsertionTime  time.Time
	LastAccessTime time.Time
	AccessCount    uint32
	// PolicyMeta is policy-owned and opaque to the store. It is the only place
	// a policy may hang per-entry state that must survive inside the store.
	PolicyMeta any
}
