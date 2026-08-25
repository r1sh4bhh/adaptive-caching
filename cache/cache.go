// Package cache is the cache core: the object store, capacity accounting and
// the orchestration that wires store, policy, metrics and the event bus
// together.
//
// Layering: cache/ may import types/, events/, metrics/, config/ and
// cache/policy/ only. It must never import server/, tui/, benchmark/,
// adaptive/ or a ui/ package — enforced by scripts/lint-arch.sh in CI.
package cache

import (
	"errors"

	"github.com/r1sh4bhh/adaptive-caching/metrics"
	"github.com/r1sh4bhh/adaptive-caching/types"
)

// Errors returned by the cache.
var (
	// ErrObjectTooLarge is returned when a single object cannot fit in the
	// cache even when the cache is empty.
	ErrObjectTooLarge = errors.New("cache: object larger than capacity")
	// ErrCapacityExceeded is returned when an object cannot be admitted
	// because no policy is installed to free space for it.
	ErrCapacityExceeded = errors.New("cache: insufficient capacity")
	// ErrInvalidSize is returned for a negative object size.
	ErrInvalidSize = errors.New("cache: negative object size")
)

// Cache is the interface every cache implementation in this project satisfies,
// including the shadow caches (P4) and the tiered cache (P13).
//
// Capacity is expressed in BYTES, not object count. This is deliberate:
// heterogeneous object sizes are a core research contribution (context.md
// §5.5), and an object-count capacity would make size-aware eviction
// meaningless.
//
// FROZEN after P1: changing this interface requires an ADR in
// docs/DECISIONS.md plus updating all dependents in the same commit.
type Cache interface {
	Get(key string) (types.Value, bool)
	Put(key string, val types.Value, size int64) error
	Remove(key string) bool
	Contains(key string) bool

	// Len is the object count.
	Len() int
	// Bytes is the payload bytes currently resident.
	Bytes() int64
	// Capacity is the byte capacity.
	Capacity() int64
	Stats() metrics.Stats

	Clear()
}
