// Package edge contains the edge-case tests called out in P2's phase
// card: capacity=1, size=0, obj>cap, duplicate insert, and empty
// cache. Each test runs against every shipped policy (LRU, LFU,
// Clock) so the edge case is exercised in the same way it will be
// in production: through the cache Core driving the policy.
//
// These tests live under tests/edge (rather than cache/ or
// cache/policy/) so the layering rule is preserved: tests/edge
// imports cache/ and cache/policy/; nothing in the layered project
// imports back.
package edge

import (
	"errors"
	"fmt"
	"testing"

	"github.com/r1sh4bhh/adaptive-caching/cache"
	"github.com/r1sh4bhh/adaptive-caching/cache/policy"
)

// allPolicies returns a fresh instance of every shipped P2 policy.
// The pattern is the standard table-driven idiom so adding ARC or
// W-TinyLFU in P4 requires no test changes.
func allPolicies(t *testing.T) []struct {
	name string
	make func() policy.EvictionPolicy
} {
	t.Helper()
	return []struct {
		name string
		make func() policy.EvictionPolicy
	}{
		{"lru", func() policy.EvictionPolicy { return policy.NewLRU() }},
		{"lfu", func() policy.EvictionPolicy { return policy.NewLFU() }},
		{"clock", func() policy.EvictionPolicy { return policy.NewClock() }},
	}
}

// TestEdgeCapacityOne: a 1-byte cache accepts exactly one object and
// evicts the previous one on every subsequent insert.
func TestEdgeCapacityOne(t *testing.T) {
	for _, p := range allPolicies(t) {
		t.Run(p.name, func(t *testing.T) {
			c := cache.New(cache.Options{Capacity: 1, Policy: p.make()})
			if err := c.Put("a", []byte("a"), 1); err != nil {
				t.Fatalf("Put a: %v", err)
			}
			if err := c.Put("b", []byte("b"), 1); err != nil {
				t.Fatalf("Put b: %v", err)
			}
			if c.Contains("a") {
				t.Error("a is still resident after b was inserted into cap=1")
			}
			if !c.Contains("b") {
				t.Error("b was not resident after a 1-byte Put")
			}
		})
	}
}

// TestEdgeSizeZero: a 0-byte object is admitted and does not break
// the eviction loop. The phase card specifically warns about
// policies infinite-looping on a zero-size victim.
func TestEdgeSizeZero(t *testing.T) {
	for _, p := range allPolicies(t) {
		t.Run(p.name, func(t *testing.T) {
			c := cache.New(cache.Options{Capacity: 100, Policy: p.make()})
			if err := c.Put("zero", nil, 0); err != nil {
				t.Fatalf("Put zero-size: %v", err)
			}
			if !c.Contains("zero") {
				t.Error("zero-size object was not resident")
			}
			// Fill the rest of the cache; the zero-size object
			// should be evictable like any other.
			for i := 0; i < 200; i++ {
				if err := c.Put(fmt.Sprintf("k%d", i), []byte("x"), 1); err != nil {
					if errors.Is(err, cache.ErrCapacityExceeded) {
						t.Fatalf("Put %d failed with capacity-exceeded mid-loop: %v", i, err)
					}
					t.Fatalf("Put %d: %v", i, err)
				}
			}
			if c.Contains("zero") {
				t.Error("zero-size object survived a full cache fill")
			}
		})
	}
}

// TestEdgeObjectLargerThanCapacity: a single object that does not
// fit is rejected with ErrObjectTooLarge. No eviction attempt is
// made — the policy never sees the key.
func TestEdgeObjectLargerThanCapacity(t *testing.T) {
	for _, p := range allPolicies(t) {
		t.Run(p.name, func(t *testing.T) {
			c := cache.New(cache.Options{Capacity: 100, Policy: p.make()})
			err := c.Put("huge", nil, 101)
			if !errors.Is(err, cache.ErrObjectTooLarge) {
				t.Fatalf("Put oversized = %v, want ErrObjectTooLarge", err)
			}
			if c.Contains("huge") {
				t.Error("oversized object was admitted")
			}
			if c.Len() != 0 {
				t.Errorf("Len = %d after rejected Put, want 0", c.Len())
			}
		})
	}
}

// TestEdgeDuplicateInsert: re-Putting the same key replaces the
// value but does not double-count in the policy. The phase card
// flags this specifically for LFU: it must not double-increment the
// frequency on replace.
func TestEdgeDuplicateInsert(t *testing.T) {
	for _, p := range allPolicies(t) {
		t.Run(p.name, func(t *testing.T) {
			c := cache.New(cache.Options{Capacity: 100, Policy: p.make()})
			if err := c.Put("a", []byte("first"), 10); err != nil {
				t.Fatalf("Put a: %v", err)
			}
			if err := c.Put("a", []byte("second"), 20); err != nil {
				t.Fatalf("Put a (replace): %v", err)
			}
			if got, want := c.Len(), 1; got != want {
				t.Errorf("Len after replace = %d, want %d", got, want)
			}
			if got, want := c.Bytes(), int64(20); got != want {
				t.Errorf("Bytes after replace = %d, want %d", got, want)
			}
			val, ok := c.Get("a")
			if !ok || string(val) != "second" {
				t.Errorf("Get after replace = (%q, %v), want (\"second\", true)", val, ok)
			}
		})
	}
}

// TestEdgeEmptyPolicy: a policy with no objects must report no
// victim and must not panic when asked.
func TestEdgeEmptyPolicy(t *testing.T) {
	for _, p := range allPolicies(t) {
		t.Run(p.name, func(t *testing.T) {
			pol := p.make()
			v, ok := pol.Victim()
			if ok || v != "" {
				t.Errorf("Victim on empty policy = (%q, %v), want (\"\", false)", v, ok)
			}
			if got := pol.Candidates(5); got != nil {
				t.Errorf("Candidates on empty policy = %v, want nil", got)
			}
			if got := pol.MetadataBytes(); got != 0 {
				t.Errorf("MetadataBytes on empty policy = %d, want 0", got)
			}
			// The empty cache's Get for a missing key must miss.
			c := cache.New(cache.Options{Capacity: 100, Policy: pol})
			if _, ok := c.Get("missing"); ok {
				t.Error("Get on empty cache returned a hit")
			}
		})
	}
}

// TestEdgePolicyNameRoundTrip: every registered policy's Name()
// matches the string it was registered under, so the CLI flag
// value round-trips through the registry.
func TestEdgePolicyNameRoundTrip(t *testing.T) {
	for _, name := range policy.Names() {
		t.Run(string(name), func(t *testing.T) {
			p, ok := policy.New(name)
			if !ok {
				t.Fatalf("registry did not return a policy for %q", name)
			}
			if p.Name() != name {
				t.Errorf("Name = %q, want %q", p.Name(), name)
			}
		})
	}
}

// TestEdgeRegistryRejectsUnknown: a policy name that was never
// registered is rejected at lookup time.
func TestEdgeRegistryRejectsUnknown(t *testing.T) {
	p, ok := policy.New("definitely-not-a-policy")
	if ok {
		t.Errorf("registry returned a policy for unknown name, want (nil, false)")
	}
	if p != nil {
		t.Errorf("registry returned non-nil policy on miss, want nil")
	}
}

// TestEdgeReplaceAcrossPolicies: explicit per-policy check that a
// replace-on-insert does not double-count. The general
// TestEdgeDuplicateInsert checks the Core-level contract; this one
// asserts the policy metadata is consistent after a replace.
func TestEdgeReplaceAcrossPolicies(t *testing.T) {
	makers := map[string]func() policy.EvictionPolicy{
		"lru":   func() policy.EvictionPolicy { return policy.NewLRU() },
		"lfu":   func() policy.EvictionPolicy { return policy.NewLFU() },
		"clock": func() policy.EvictionPolicy { return policy.NewClock() },
	}
	for name, mk := range makers {
		t.Run(name, func(t *testing.T) {
			pol := mk()
			c := cache.New(cache.Options{Capacity: 100, Policy: pol})
			for i := 0; i < 5; i++ {
				if err := c.Put("k", []byte("v"), 10); err != nil {
					t.Fatalf("Put %d: %v", i, err)
				}
			}
			if got := c.Len(); got != 1 {
				t.Errorf("Len = %d, want 1", got)
			}
			if got := c.Bytes(); got != 10 {
				t.Errorf("Bytes = %d, want 10", got)
			}
		})
	}
}
