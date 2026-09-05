package policy

import (
	"testing"
	"time"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// makeEntry constructs a minimal *types.Entry for tests. We never look
// at the value — the policy must not read it.
func makeEntry(key string) *types.Entry {
	return &types.Entry{
		Key:            key,
		Value:          nil,
		Size:           1,
		InsertionTime:  time.Unix(0, 0),
		LastAccessTime: time.Unix(0, 0),
		AccessCount:    1,
	}
}

// TestLRUClassicSequence is the textbook LRU scenario: insert three
// keys, touch the first, then insert a fourth that forces an eviction.
// The LEAST-recently-used of the three remaining is the victim.
func TestLRUClassicSequence(t *testing.T) {
	p := NewLRU()
	p.OnInsert("k1", makeEntry("k1"))
	p.OnInsert("k2", makeEntry("k2"))
	p.OnInsert("k3", makeEntry("k3"))
	p.OnAccess("k1", makeEntry("k1")) // k1 is now most-recently used
	p.OnInsert("k4", makeEntry("k4")) // forces eviction; k2 is LRU

	v, ok := p.Victim()
	if !ok {
		t.Fatal("Victim returned no candidate, want k2")
	}
	if v != "k2" {
		t.Errorf("Victim = %q, want %q", v, "k2")
	}
}

// TestLRUCandidatesOrder asserts the batch-eviction order is worst-first.
func TestLRUCandidatesOrder(t *testing.T) {
	p := NewLRU()
	for _, k := range []string{"k1", "k2", "k3", "k4"} {
		p.OnInsert(k, makeEntry(k))
	}
	p.OnAccess("k1", nil) // k1 becomes most-recent
	p.OnAccess("k2", nil) // k2 becomes most-recent; k3,k4 are older

	got := p.Candidates(2)
	if len(got) != 2 {
		t.Fatalf("Candidates(2) returned %d entries, want 2", len(got))
	}
	// List state: [k2, k1, k4, k3] (front to back). Back() = k3.
	// Walking backward from k3 yields k3, then k4 — those are the
	// LEAST-recently-used, which is the correct eviction order.
	if got[0] != "k3" || got[1] != "k4" {
		t.Errorf("Candidates(2) = %v, want [k3 k4]", got)
	}
}

// TestLRURemove cleans up the metadata.
func TestLRURemove(t *testing.T) {
	p := NewLRU()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	if p.n != 3 {
		t.Fatalf("n = %d, want 3", p.n)
	}
	p.OnRemove("b", nil)
	if p.n != 2 {
		t.Fatalf("n after Remove = %d, want 2", p.n)
	}
	if v, _ := p.Victim(); v == "b" {
		t.Errorf("Victim after Remove = %q, want any key but b", v)
	}
	// Double-remove is a no-op.
	p.OnRemove("b", nil)
	if p.n != 2 {
		t.Errorf("n after double-remove = %d, want 2", p.n)
	}
}

// TestLRUOnAccessUnknownKey is a no-op, not a panic.
func TestLRUOnAccessUnknownKey(t *testing.T) {
	p := NewLRU()
	p.OnInsert("a", makeEntry("a"))
	// Should not panic.
	p.OnAccess("not-there", nil)
	if p.n != 1 {
		t.Errorf("n after OnAccess(unknown) = %d, want 1", p.n)
	}
}

// TestLRUEmptyVictim is the edge case called out in the phase card.
func TestLRUEmptyVictim(t *testing.T) {
	p := NewLRU()
	if v, ok := p.Victim(); ok || v != "" {
		t.Errorf("Victim on empty LRU = (%q, %v), want (\"\", false)", v, ok)
	}
	if got := p.Candidates(5); got != nil {
		t.Errorf("Candidates on empty LRU = %v, want nil", got)
	}
}

// TestLRUMetadataScales proves MetadataBytes reports something
// proportional to the entry count.
func TestLRUMetadataScales(t *testing.T) {
	p := NewLRU()
	if got := p.MetadataBytes(); got != 0 {
		t.Errorf("MetadataBytes on empty LRU = %d, want 0", got)
	}
	const n = 100
	for i := 0; i < n; i++ {
		p.OnInsert(string(rune('a'+i%26))+string(rune('0'+i/26)), makeEntry(""))
	}
	mb := p.MetadataBytes()
	if mb <= 0 {
		t.Errorf("MetadataBytes after inserts = %d, want > 0", mb)
	}
	// Sanity: the per-entry estimate * n is the floor.
	if mb < int64(n) {
		t.Errorf("MetadataBytes = %d, want >= %d (n=%d)", mb, n, n)
	}
}

// TestLRURebuildAdoptsEntries is the round-trip test the phase card
// requires: a fresh policy adopts resident entries and is ready to
// serve Victims.
func TestLRURebuildAdoptsEntries(t *testing.T) {
	p := NewLRU()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	entries := []*types.Entry{
		makeEntry("x"),
		makeEntry("y"),
	}
	q := NewLRU()
	q.Rebuild(entries)
	if q.n != 2 {
		t.Errorf("Rebuild length = %d, want 2", q.n)
	}
	if v, _ := q.Victim(); v == "" {
		t.Error("Victim on rebuilt policy returned no candidate")
	}
}

// TestLRUResetZerosState verifies the policy can be reused after Reset.
func TestLRUResetZerosState(t *testing.T) {
	p := NewLRU()
	p.OnInsert("a", makeEntry("a"))
	p.OnInsert("b", makeEntry("b"))
	p.Reset()
	if p.n != 0 {
		t.Errorf("n after Reset = %d, want 0", p.n)
	}
	if v, ok := p.Victim(); ok {
		t.Errorf("Victim after Reset = (%q, true), want (\"\", false)", v)
	}
	if got := p.MetadataBytes(); got != 0 {
		t.Errorf("MetadataBytes after Reset = %d, want 0", got)
	}
}

// TestLRUParamsAndSetParam documents the deliberate absence of
// parameters on LRU.
func TestLRUParamsAndSetParam(t *testing.T) {
	p := NewLRU()
	if got := p.Params(); len(got) != 0 {
		t.Errorf("Params = %v, want empty", got)
	}
	if err := p.SetParam("decay_lambda", 0.5); err == nil {
		t.Error("SetParam succeeded, want error (LRU has no parameters)")
	}
}

// TestLRUShouldAdmitAlwaysTrue is the P2 baseline assertion: the
// policy does no admission filtering. P9 changes this.
func TestLRUShouldAdmitAlwaysTrue(t *testing.T) {
	p := NewLRU()
	if !p.ShouldAdmit("anything", 1<<20) {
		t.Error("ShouldAdmit returned false, want true (no admission filter)")
	}
}

// TestLRUReplaceInPlace exercises the cache's replace-in-place path:
// re-inserting an existing key must not double-count.
func TestLRUReplaceInPlace(t *testing.T) {
	p := NewLRU()
	p.OnInsert("a", makeEntry("a"))
	p.OnInsert("a", makeEntry("a")) // replace
	if p.n != 1 {
		t.Errorf("n after replace = %d, want 1", p.n)
	}
	if got := p.MetadataBytes(); got != int64(lruPerEntryBytes) {
		t.Errorf("MetadataBytes after replace = %d, want %d", got, lruPerEntryBytes)
	}
}

// TestLRURegistryWiring exercises the package's registry via Name().
func TestLRURegistryWiring(t *testing.T) {
	p, ok := New(types.PolicyLRU)
	if !ok {
		t.Fatal("registry did not know LRU")
	}
	if p.Name() != types.PolicyLRU {
		t.Errorf("Name = %s, want lru", p.Name())
	}
}
