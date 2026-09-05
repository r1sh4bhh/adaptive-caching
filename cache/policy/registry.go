// Package policy's registry maps a policy name to a constructor. Every
// concrete policy (LRU, LFU, Clock here; ARC, W-TinyLFU in P4) registers
// itself in an init() function so the rest of the system — the CLI, the
// benchmark runner, the adaptive engine's selector — can resolve a name
// without an import-time dependency on the specific policy package.
//
// The registry is read-only after init() returns. There is no mutex; a
// concurrent lookup is safe because we never write to the map after the
// package has been initialised.
package policy

import (
	"fmt"
	"sort"
	"sync"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// Constructor builds a fresh, ready-to-use EvictionPolicy. Each policy
// package exposes one and registers it via Register in its init().
type Constructor func() EvictionPolicy

var (
	registryMu sync.RWMutex
	registry   = map[types.PolicyName]Constructor{}
)

// Register associates name with ctor. It is intended to be called from a
// policy package's init(); calling it after init() is permitted but
// pointless (the adaptive engine reads Names() at config-validate time).
//
// Re-registering the same name replaces the prior constructor. This is
// what tests use to inject a fake policy.
func Register(name types.PolicyName, ctor Constructor) {
	if ctor == nil {
		panic("policy: Register called with nil constructor for " + string(name))
	}
	registryMu.Lock()
	registry[name] = ctor
	registryMu.Unlock()
}

// New looks up a policy by name and constructs a fresh instance. The
// second return value is false (and the policy is nil) when name has not
// been registered; callers should treat that as a configuration error.
func New(name types.PolicyName) (EvictionPolicy, bool) {
	registryMu.RLock()
	ctor, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, false
	}
	return ctor(), true
}

// Names returns every registered policy name, sorted lexicographically so
// config validation and error messages are deterministic.
func Names() []types.PolicyName {
	registryMu.RLock()
	out := make([]types.PolicyName, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	registryMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsKnown reports whether name has been registered. Convenience wrapper
// around New that does not allocate a policy.
func IsKnown(name types.PolicyName) bool {
	registryMu.RLock()
	_, ok := registry[name]
	registryMu.RUnlock()
	return ok
}

// UnregisterAll removes every registered policy. It exists exclusively
// for tests that need a clean slate between cases; production code must
// not call it.
func UnregisterAll() {
	registryMu.Lock()
	registry = map[types.PolicyName]Constructor{}
	registryMu.Unlock()
}

// ErrUnknownPolicy is returned by helper wrappers that want to surface a
// stable sentinel for "this name was never registered".
var ErrUnknownPolicy = fmt.Errorf("policy: unknown eviction policy")
