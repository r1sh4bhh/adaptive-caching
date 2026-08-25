package config

// FeatureFlags switch whole capabilities of the system on and off.
//
// Ablations are config permutations, never code branches. A study that asks
// "what does adaptation actually buy us?" is run by flipping Adaptive to false
// in a config file and re-running the same binary — not by maintaining a
// second, divergent code path. Eight ablations must not mean eight code paths.
type FeatureFlags struct {
	// Adaptive enables workload-driven policy selection and switching (P7).
	Adaptive bool `yaml:"adaptive"`
	// Tuning enables online parameter tuning (P10).
	Tuning bool `yaml:"tuning"`
	// SizeAware enables size-aware scoring and batch eviction (P9).
	SizeAware bool `yaml:"size_aware"`
	// Tiers enables the multi-tier hierarchy (P13).
	Tiers bool `yaml:"tiers"`
	// Shadow enables counterfactual shadow caches and the Bélády oracle (P4).
	Shadow bool `yaml:"shadow"`
}

// DefaultFeatureFlags returns the P1 defaults: everything off, because none of
// these subsystems exists yet.
func DefaultFeatureFlags() FeatureFlags {
	return FeatureFlags{}
}
