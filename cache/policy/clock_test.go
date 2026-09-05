package policy

import (
	"testing"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// TestClockClassicSequence is the textbook scenario: insert three
// keys, touch all three (sets ref bits), then insert a fourth.
// Second-chance: walking the ring clears the ref bits, and the slot
// whose bit was already 0 (the freshly-inserted k4) is the victim.
func TestClockClassicSequence(t *testing.T) {
	p := NewClock()
	p.OnInsert("k1", makeEntry("k1"))
	p.OnInsert("k2", makeEntry("k2"))
	p.OnInsert("k3", makeEntry("k3"))
	p.OnAccess("k1", nil) // k1.ref=1
	p.OnAccess("k2", nil) // k2.ref=1
	p.OnAccess("k3", nil) // k3.ref=1
	// At this point the ring has [k1, k2, k3] in slots 0,1,2, hand=3
	// (next insert goes to slot 3). All ref bits are 1.
	p.OnInsert("k4", makeEntry("k4"))
	// Now ring is [k1, k2, k3, k4] with refs [1,1,1,1] and hand=0.

	// Victim: hand=0 (k1), ref=1 -> clear, advance. hand=1 (k2),
	// ref=1 -> clear, advance. hand=2 (k3), ref=1 -> clear, advance.
	// hand=3 (k4), ref=1 -> clear, advance. hand=0 (k1), ref=0 ->
	// return k1.
	v, ok := p.Victim()
	if !ok {
		t.Fatal("Victim returned no candidate")
	}
	if v != "k1" {
		t.Errorf("Victim = %q, want k1 (the slot the hand returns to after one full sweep)", v)
	}
}

// TestClockNoAccessFreshInsertIsVictim: when k1,k2,k3 are inserted
// (ref=1, advanced) and k4 inserted (ref=1) with no accesses,
// every slot has ref=1. After one full sweep clearing refs, the
// hand returns to k1 with ref=0 -> k1 is the victim.
func TestClockNoAccessFreshInsertIsVictim(t *testing.T) {
	p := NewClock()
	p.OnInsert("k1", makeEntry("k1"))
	p.OnInsert("k2", makeEntry("k2"))
	p.OnInsert("k3", makeEntry("k3"))
	p.OnInsert("k4", makeEntry("k4"))
	// All refs=1, hand=0. One Victim clears all and returns k1.
	v, _ := p.Victim()
	if v != "k1" {
		t.Errorf("Victim = %q, want k1", v)
	}
}

// TestClockRefBitProtects: ref=1 on a key forces the hand to take
// at least one extra lap before evicting it, demonstrating the
// "second chance" behaviour the policy is named for.
func TestClockRefBitProtects(t *testing.T) {
	p := NewClock()
	for _, k := range []string{"k1", "k2", "k3"} {
		p.OnInsert(k, makeEntry(k))
	}
	// Touch k2: k2.ref=1, the others ref=1 from insertion but
	// the hand will clear them on its first sweep.
	// Victim: hand=0 (k1, ref=1) -> clear, advance. hand=1
	// (k2, ref=1) -> clear, advance. hand=2 (k3, ref=1) -> clear,
	// advance. hand=0 (k1, ref=0) -> return k1.
	v1, _ := p.Victim()
	if v1 != "k1" {
		t.Fatalf("first victim = %q, want k1", v1)
	}
	// Now re-touch k1 — the ref bit protects it for one Victim.
	p.OnAccess("k1", nil)
	// hand=1 (k2, ref=0) -> return k2.
	v2, _ := p.Victim()
	if v2 != "k2" {
		t.Errorf("second victim = %q, want k2 (k1's ref=1 saves it)", v2)
	}
}

// TestClockRemoveClearsSlot: a key removed via OnRemove is gone
// from the index. The slot becomes empty and the hand skips it.
func TestClockRemove(t *testing.T) {
	p := NewClock()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	p.OnRemove("b", nil)
	// Slots: 0=a, 1=empty, 2=c. Hand is at 0. Victim returns a
	// (ref=0) and the policy keeps hand at 0. Simulate the store's
	// OnRemove call after a real eviction, then verify b stays gone.
	p.OnRemove("a", nil)
	// Now: 0=empty, 1=empty, 2=c. Hand=0.
	v, _ := p.Victim()
	if v != "c" {
		t.Errorf("Victim = %q, want c (the only live key)", v)
	}
}

// TestClockEmptyVictim.
func TestClockEmptyVictim(t *testing.T) {
	p := NewClock()
	if v, ok := p.Victim(); ok || v != "" {
		t.Errorf("Victim on empty Clock = (%q, %v), want (\"\", false)", v, ok)
	}
}

// TestClockReplaceInPlace.
func TestClockReplaceInPlace(t *testing.T) {
	p := NewClock()
	p.OnInsert("a", makeEntry("a"))
	p.OnInsert("a", makeEntry("a")) // replace
	if p.live != 1 {
		t.Errorf("live after replace = %d, want 1", p.live)
	}
}

// TestClockReset.
func TestClockReset(t *testing.T) {
	p := NewClock()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	p.Reset()
	if p.live != 0 {
		t.Errorf("live after Reset = %d, want 0", p.live)
	}
	if v, ok := p.Victim(); ok {
		t.Errorf("Victim after Reset = (%q, true), want (\"\", false)", v)
	}
}

// TestClockRebuild: a fresh policy adopts resident entries and is
// ready to serve Victims.
func TestClockRebuild(t *testing.T) {
	entries := []*types.Entry{
		makeEntry("a"),
		makeEntry("b"),
		makeEntry("c"),
	}
	p := NewClock()
	p.Rebuild(entries)
	if p.live != 3 {
		t.Errorf("live after Rebuild = %d, want 3", p.live)
	}
	v, ok := p.Victim()
	if !ok {
		t.Fatal("Victim after Rebuild returned no candidate")
	}
	if v == "" {
		t.Errorf("Victim after Rebuild = \"\", want one of a/b/c")
	}
}

// TestClockRingGrowthDoubles: insert up to 5 keys; the ring
// should grow from 1 to 8 (powers of two).
func TestClockRingGrowthDoubles(t *testing.T) {
	p := NewClock()
	for i := 0; i < 5; i++ {
		k := string(rune('a' + i))
		p.OnInsert(k, makeEntry(k))
	}
	if p.cap < 8 {
		t.Errorf("cap = %d, want >= 8 after 5 inserts", p.cap)
	}
	if p.cap > 8 {
		t.Errorf("cap = %d, want exactly 8 (powers of two)", p.cap)
	}
}

// TestClockShouldAdmitAlwaysTrue.
func TestClockShouldAdmitAlwaysTrue(t *testing.T) {
	p := NewClock()
	if !p.ShouldAdmit("anything", 1<<20) {
		t.Error("ShouldAdmit returned false, want true (no admission filter)")
	}
}

// TestClockParamsAndSetParam documents the deliberate absence of
// parameters on Clock.
func TestClockParamsAndSetParam(t *testing.T) {
	p := NewClock()
	if got := p.Params(); len(got) != 0 {
		t.Errorf("Params = %v, want empty", got)
	}
	if err := p.SetParam("ref_bits", 2); err == nil {
		t.Error("SetParam succeeded, want error (Clock has no parameters)")
	}
}

// TestClockOnAccessUnknownKey is a no-op, not a panic.
func TestClockOnAccessUnknownKey(t *testing.T) {
	p := NewClock()
	p.OnInsert("a", makeEntry("a"))
	// Should not panic.
	p.OnAccess("not-there", nil)
	if p.live != 1 {
		t.Errorf("live after OnAccess(unknown) = %d, want 1", p.live)
	}
}

// TestClockRegistryWiring.
func TestClockRegistryWiring(t *testing.T) {
	p, ok := New(types.PolicyClock)
	if !ok {
		t.Fatal("registry did not know Clock")
	}
	if p.Name() != types.PolicyClock {
		t.Errorf("Name = %s, want clock", p.Name())
	}
}

// TestClockCandidatesReturnsEvictionOrder: Candidates(n) should
// return keys in eviction order. For a freshly-inserted ring of 4
// (all ref=1), Candidates(2) returns the first two ref=0 slots the
// hand clears, which are k1 then k2.
func TestClockCandidatesReturnsEvictionOrder(t *testing.T) {
	p := NewClock()
	for _, k := range []string{"k1", "k2", "k3", "k4"} {
		p.OnInsert(k, makeEntry(k))
	}
	// All refs=1. Candidates(2): clears k1 (ref=1 -> 0), clears
	// k2 (ref=1 -> 0), returns [k1, k2].
	got := p.Candidates(2)
	if len(got) != 2 {
		t.Fatalf("Candidates(2) = %v, want 2 entries", got)
	}
	if got[0] != "k1" || got[1] != "k2" {
		t.Errorf("Candidates(2) = %v, want [k1 k2]", got)
	}
}
