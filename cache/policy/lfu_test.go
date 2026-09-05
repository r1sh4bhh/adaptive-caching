package policy

import (
	"testing"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// TestLFUVictimIsLowestFrequency: with no accesses, every key is at
// freq=1. Victim() must return some key at freq=1; the exact key is
// FIFO within the bucket so it depends on map iteration order — but
// it must be a freq-1 key, never an unseen one.
func TestLFUVictimIsLowestFrequency(t *testing.T) {
	p := NewLFU()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	v, ok := p.Victim()
	if !ok {
		t.Fatal("Victim on non-empty LFU returned no candidate")
	}
	if v != "a" && v != "b" && v != "c" {
		t.Errorf("Victim = %q, want one of a/b/c (all at freq=1)", v)
	}
}

// TestLFUAccessMovesFreq: a single access on a key at freq=1 moves
// it to freq=2, and a victim must be a key still at freq=1.
func TestLFUAccessMovesFreq(t *testing.T) {
	p := NewLFU()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	p.OnAccess("a", nil) // a -> freq=2
	p.OnAccess("a", nil) // a -> freq=3
	v, ok := p.Victim()
	if !ok {
		t.Fatal("Victim returned no candidate")
	}
	if v == "a" {
		t.Errorf("Victim = a (freq=3), want a freq-1 key (b or c)")
	}
}

// TestLFUColdStartMitigation: without decay, a brand-new key at
// freq=1 always loses to a freq>=2 key. With decay, after enough
// accesses the freq-2 key drops to freq=1 and the new key survives
// to be touched.
func TestLFUColdStartMitigation(t *testing.T) {
	// Set decay=0.5 so a halving happens every 2 accesses.
	p := NewLFU()
	if err := p.SetParam("decay_lambda", 0.5); err != nil {
		t.Fatalf("SetParam: %v", err)
	}
	p.OnInsert("hot", makeEntry("hot"))
	p.OnInsert("cold", makeEntry("cold"))
	for i := 0; i < 10; i++ {
		p.OnAccess("hot", nil) // freq keeps rising but halving eats it
	}
	// After many accesses, hot should have been halved several times
	// (default ageMax for decay=0.5 is 2 — every 2 accesses halve).
	// Insert a new key; if decay works, "hot" will eventually be
	// evicted before "new" because the new key gets touched enough.
	p.OnInsert("new", makeEntry("new"))
	for i := 0; i < 6; i++ {
		p.OnAccess("new", nil)
	}
	if v, _ := p.Victim(); v == "new" {
		t.Errorf("Victim = new, want hot (decay should have weakened hot's frequency)")
	}
}

// TestLFUDecayLambdaParamRoundTrip exercises Params/SetParam.
func TestLFUDecayLambdaParamRoundTrip(t *testing.T) {
	p := NewLFU()
	ps := p.Params()
	lam, ok := ps["decay_lambda"]
	if !ok {
		t.Fatal("Params() missing decay_lambda")
	}
	if lam.Current != defaultDecayLambda {
		t.Errorf("decay_lambda.Current = %g, want %g", lam.Current, defaultDecayLambda)
	}
	if lam.Min != 0 || lam.Max != 1 {
		t.Errorf("decay_lambda bounds = [%g, %g], want [0, 1]", lam.Min, lam.Max)
	}

	if err := p.SetParam("decay_lambda", 0.1); err != nil {
		t.Fatalf("SetParam: %v", err)
	}
	if got := p.Params()["decay_lambda"].Current; got != 0.1 {
		t.Errorf("Current after SetParam = %g, want 0.1", got)
	}
	// Out-of-range rejection.
	if err := p.SetParam("decay_lambda", -0.1); err == nil {
		t.Error("SetParam(negative) succeeded, want error")
	}
	if err := p.SetParam("decay_lambda", 1.5); err == nil {
		t.Error("SetParam(>1) succeeded, want error")
	}
	// Unknown parameter rejection.
	if err := p.SetParam("not_a_param", 0); err == nil {
		t.Error("SetParam(unknown) succeeded, want error")
	}
}

// TestLFURebuildSeedsFrequencies: after Rebuild with a slice of
// entries, frequencies are seeded from AccessCount and the victim
// is the lowest-frequency key. Decay is disabled to isolate the
// rebuild seeding from the amortised ageing pass.
func TestLFURebuildSeedsFrequencies(t *testing.T) {
	entries := []*types.Entry{
		{Key: "warm", AccessCount: 5},
		{Key: "cool", AccessCount: 3},
		{Key: "cold", AccessCount: 1},
	}
	p := NewLFU()
	if err := p.SetParam("decay_lambda", 0); err != nil {
		t.Fatalf("SetParam(0): %v", err)
	}
	p.Rebuild(entries)

	// cold has freq=1, cool has freq=3, warm has freq=5. Victim
	// is cold.
	v, _ := p.Victim()
	if v != "cold" {
		t.Errorf("Victim = %q, want cold (lowest freq)", v)
	}

	// Promote cold past warm and cool; warm should become the victim.
	for i := 0; i < 10; i++ {
		p.OnAccess("cold", nil) // cold climbs 1->2->...->11
	}
	v2, _ := p.Victim()
	if v2 != "cool" {
		t.Errorf("Victim after cold promotion = %q, want cool (next lowest)", v2)
	}
	for i := 0; i < 10; i++ {
		p.OnAccess("cool", nil) // cool climbs 3->4->...->13
	}
	v3, _ := p.Victim()
	if v3 != "warm" {
		t.Errorf("Victim after cool promotion = %q, want warm (now lowest)", v3)
	}
}

// TestLFURemoveCleansUp removes a key and confirms it is no longer
// a candidate.
func TestLFURemoveCleansUp(t *testing.T) {
	p := NewLFU()
	for _, k := range []string{"a", "b", "c"} {
		p.OnInsert(k, makeEntry(k))
	}
	p.OnAccess("a", nil) // a at freq=2
	p.OnAccess("b", nil) // b at freq=2
	p.OnRemove("c", nil)
	v, _ := p.Victim()
	if v == "c" {
		t.Errorf("Victim = c, want a or b (c was removed)")
	}
}

// TestLFUEmptyVictim is the empty-policy edge case.
func TestLFUEmptyVictim(t *testing.T) {
	p := NewLFU()
	if v, ok := p.Victim(); ok || v != "" {
		t.Errorf("Victim on empty LFU = (%q, %v), want (\"\", false)", v, ok)
	}
}

// TestLFUMaxFreqCeiling: a single key accessed many times must NOT
// overflow its frequency counter. Decay is disabled to isolate the
// ceiling behaviour from the ageing pass.
func TestLFUMaxFreqCeiling(t *testing.T) {
	p := NewLFU()
	if err := p.SetParam("decay_lambda", 0); err != nil {
		t.Fatalf("SetParam(0): %v", err)
	}
	p.OnInsert("hot", makeEntry("hot"))
	// Far more accesses than maxLFUFreq.
	for i := 0; i < int(maxLFUFreq)*2; i++ {
		p.OnAccess("hot", nil)
	}
	// Inspect: hot's frequency is capped at maxLFUFreq.
	p.mu.Lock()
	freq := p.items["hot"].freq
	p.mu.Unlock()
	if freq != maxLFUFreq {
		t.Errorf("hot.freq = %d, want %d (capped)", freq, maxLFUFreq)
	}
}

// TestLFUReplaceInPlace: re-inserting a key must increment its
// frequency (matching LRU's "treat as access" semantics) and must
// not create a duplicate.
func TestLFUReplaceInPlace(t *testing.T) {
	p := NewLFU()
	p.OnInsert("a", makeEntry("a"))
	p.OnInsert("a", makeEntry("a")) // replace -> freq=2
	if p.n != 1 {
		t.Errorf("n after replace = %d, want 1", p.n)
	}
	p.mu.Lock()
	freq := p.items["a"].freq
	p.mu.Unlock()
	if freq != 2 {
		t.Errorf("a.freq after replace = %d, want 2", freq)
	}
}

// TestLFUResetZerosState.
func TestLFUResetZerosState(t *testing.T) {
	p := NewLFU()
	p.OnInsert("a", makeEntry("a"))
	p.OnAccess("a", nil)
	p.Reset()
	if p.n != 0 {
		t.Errorf("n after Reset = %d, want 0", p.n)
	}
	if v, ok := p.Victim(); ok {
		t.Errorf("Victim after Reset = (%q, true), want (\"\", false)", v)
	}
}

// TestLFUShouldAdmitAlwaysTrue: P2 baseline; admission filtering
// is P9.
func TestLFUShouldAdmitAlwaysTrue(t *testing.T) {
	p := NewLFU()
	if !p.ShouldAdmit("anything", 1<<20) {
		t.Error("ShouldAdmit returned false, want true (no admission filter)")
	}
}

// TestLFURegistryWiring.
func TestLFURegistryWiring(t *testing.T) {
	p, ok := New(types.PolicyLFU)
	if !ok {
		t.Fatal("registry did not know LFU")
	}
	if p.Name() != types.PolicyLFU {
		t.Errorf("Name = %s, want lfu", p.Name())
	}
}

// TestLFUMetadataScales: MetadataBytes is proportional to n.
func TestLFUMetadataScales(t *testing.T) {
	p := NewLFU()
	if got := p.MetadataBytes(); got != 0 {
		t.Errorf("MetadataBytes on empty = %d, want 0", got)
	}
	const n = 100
	for i := 0; i < n; i++ {
		p.OnInsert(string(rune('a'+i%26))+string(rune('0'+i/26)), makeEntry(""))
	}
	mb := p.MetadataBytes()
	if mb < int64(n) {
		t.Errorf("MetadataBytes = %d, want >= %d", mb, n)
	}
}

// TestLFUDecayDisabled exercises the no-decay path.
func TestLFUDecayDisabled(t *testing.T) {
	p := NewLFU()
	if err := p.SetParam("decay_lambda", 0); err != nil {
		t.Fatalf("SetParam(0): %v", err)
	}
	// After SetParam(0), ageMax is 0 -> halving is disabled.
	p.mu.Lock()
	if p.ageMax != 0 {
		t.Errorf("ageMax = %d, want 0 (decay disabled)", p.ageMax)
	}
	p.mu.Unlock()
}
