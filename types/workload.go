package types

import "time"

// WorkloadType is the coarse class of access pattern the detector believes is
// currently occurring. See context.md §4.3 and §6.1.
type WorkloadType uint8

const (
	// WorkloadUnknown is the zero value: no prediction yet, or confidence too
	// low to commit to one.
	WorkloadUnknown WorkloadType = iota
	// WorkloadTemporal is recency-dominated reuse.
	WorkloadTemporal
	// WorkloadSpatial is neighbouring-key / scan-like access.
	WorkloadSpatial
	// WorkloadWorkingSet is a stable hot subset of keys.
	WorkloadWorkingSet
	// WorkloadRandom is uniform access with little exploitable structure.
	WorkloadRandom
	// WorkloadBursty is highly variable arrival rate with spikes.
	WorkloadBursty
	// WorkloadMixed is a blend with no single dominant signature.
	WorkloadMixed
)

// String implements fmt.Stringer.
func (w WorkloadType) String() string {
	switch w {
	case WorkloadUnknown:
		return "unknown"
	case WorkloadTemporal:
		return "temporal"
	case WorkloadSpatial:
		return "spatial"
	case WorkloadWorkingSet:
		return "working_set"
	case WorkloadRandom:
		return "random"
	case WorkloadBursty:
		return "bursty"
	case WorkloadMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

// Features is the feature vector extracted from a request window. Field order
// is grouped by the property being measured; the extractor (P5) is responsible
// for a stable ordering when writing CSV.
//
// Nothing populates this in P1 — the struct is declared now so that events,
// frames and the classifier all agree on its shape.
type Features struct {
	// Temporal locality.
	ReuseDistanceMean float64 `json:"reuse_distance_mean"`
	ReuseDistanceP50  float64 `json:"reuse_distance_p50"`
	ReuseDistanceP95  float64 `json:"reuse_distance_p95"`
	RepeatedKeyRatio  float64 `json:"repeated_key_ratio"`
	InterArrivalMean  float64 `json:"inter_arrival_mean"`

	// Spatial locality.
	KeyDistanceMean float64 `json:"key_distance_mean"`
	ContiguousRatio float64 `json:"contiguous_ratio"`
	RangeDensity    float64 `json:"range_density"`

	// Frequency distribution.
	UniqueKeys        int     `json:"unique_keys"`
	TopKConcentration float64 `json:"topk_concentration"`
	KeyEntropy        float64 `json:"key_entropy"`
	ZipfAlphaEstimate float64 `json:"zipf_alpha_estimate"`

	// Burstiness.
	RequestRateMean   float64 `json:"request_rate_mean"`
	RequestRateStdDev float64 `json:"request_rate_stddev"`
	BurstinessCV      float64 `json:"burstiness_cv"`
	SpikeRatio        float64 `json:"spike_ratio"`

	// Working set.
	WorkingSetEstimate int     `json:"working_set_estimate"`
	ActiveSetStability float64 `json:"active_set_stability"`

	// Object size.
	SizeMean     float64 `json:"size_mean"`
	SizeMedian   float64 `json:"size_median"`
	SizeP95      float64 `json:"size_p95"`
	SizeVariance float64 `json:"size_variance"`
	SmallRatio   float64 `json:"small_ratio"`
	MediumRatio  float64 `json:"medium_ratio"`
	LargeRatio   float64 `json:"large_ratio"`

	BytesRequested int64 `json:"bytes_requested"`
}

// WorkloadPrediction is one classifier output.
type WorkloadPrediction struct {
	Type        WorkloadType
	Confidence  float64
	Features    Features
	WindowStart uint64
	WindowEnd   uint64
	DetectedAt  time.Time
}
