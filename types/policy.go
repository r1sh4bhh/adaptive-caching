package types

import "fmt"

// PolicyName identifies an eviction policy. Policies themselves arrive in
// P2/P4; the names are declared here so that events, metrics and config can
// refer to them from P1 onwards.
type PolicyName string

const (
	// PolicyNone means no policy is installed (P1's nil-policy cache).
	PolicyNone PolicyName = "none"
	// PolicyLRU is least-recently-used.
	PolicyLRU PolicyName = "lru"
	// PolicyLFU is least-frequently-used.
	PolicyLFU PolicyName = "lfu"
	// PolicyARC is adaptive replacement cache.
	PolicyARC PolicyName = "arc"
	// PolicyWTinyLFU is window TinyLFU.
	PolicyWTinyLFU PolicyName = "wtinylfu"
	// PolicyClock is the clock / second-chance approximation of LRU.
	PolicyClock PolicyName = "clock"
	// PolicyAdaptive is the adaptive engine selecting among the above.
	PolicyAdaptive PolicyName = "adaptive"
)

// String implements fmt.Stringer.
func (p PolicyName) String() string { return string(p) }

// Parameter is one tunable knob exposed by a policy. Min/Max bound the search
// space for the tuner (P10), Step is the proposal granularity and Metric names
// the objective the parameter is tuned against.
type Parameter struct {
	Name    string
	Min     float64
	Max     float64
	Default float64
	Current float64
	Step    float64
	Metric  string
}

// Clamp returns v constrained to [Min, Max].
func (p *Parameter) Clamp(v float64) float64 {
	if v < p.Min {
		return p.Min
	}
	if v > p.Max {
		return p.Max
	}
	return v
}

// Set assigns Current after validating the value lies within [Min, Max].
func (p *Parameter) Set(v float64) error {
	if v < p.Min || v > p.Max {
		return fmt.Errorf("parameter %q: value %g out of range [%g, %g]", p.Name, v, p.Min, p.Max)
	}
	p.Current = v
	return nil
}

// ParamSet is a policy's tunable parameters, keyed by Parameter.Name.
type ParamSet map[string]*Parameter

// Clone returns a deep copy so a tuner can propose a candidate set without
// mutating the live one.
func (ps ParamSet) Clone() ParamSet {
	out := make(ParamSet, len(ps))
	for k, v := range ps {
		if v == nil {
			continue
		}
		cp := *v
		out[k] = &cp
	}
	return out
}

// Values returns the current values keyed by parameter name, for Frames and
// metrics output.
func (ps ParamSet) Values() map[string]float64 {
	out := make(map[string]float64, len(ps))
	for k, v := range ps {
		if v == nil {
			continue
		}
		out[k] = v.Current
	}
	return out
}
