// Package events is the observation layer's transport: a typed, bounded,
// droppable publish/subscribe bus plus the payload structs carried on it.
//
// Everything flows outward through this bus (context.md §3.3). Nothing on the
// consuming side of the bus may ever call back into the cache.
package events

import (
	"time"

	"github.com/r1sh4bhh/adaptive-caching/types"
)

// Type enumerates the kinds of event carried on the bus.
type Type uint8

const (
	// TypeRequest is a sampled individual request (see config SampleRate);
	// aggregate counters live in the metrics collector, not on the bus.
	TypeRequest Type = iota
	// TypeHit is a cache hit.
	TypeHit
	// TypeMiss is a cache miss.
	TypeMiss
	// TypeEviction is an object removed to reclaim bytes.
	TypeEviction
	// TypeDetection is a classifier prediction.
	TypeDetection
	// TypeSwitch is a policy change.
	TypeSwitch
	// TypeTuning is a parameter change.
	TypeTuning
	// TypeTierPromote is an object moved to a faster tier.
	TypeTierPromote
	// TypeTierDemote is an object moved to a slower tier.
	TypeTierDemote
	// TypeScenarioMark is a ground-truth workload transition boundary emitted
	// by the trace source. The detector must never see it.
	TypeScenarioMark
	// TypeRunStart marks the beginning of a run.
	TypeRunStart
	// TypeRunEnd marks the end of a run.
	TypeRunEnd
)

// maxType is the exclusive upper bound of the Type enum, used to size the
// per-subscriber filter.
const maxType = int(TypeRunEnd) + 1

// String implements fmt.Stringer.
func (t Type) String() string {
	switch t {
	case TypeRequest:
		return "request"
	case TypeHit:
		return "hit"
	case TypeMiss:
		return "miss"
	case TypeEviction:
		return "eviction"
	case TypeDetection:
		return "detection"
	case TypeSwitch:
		return "switch"
	case TypeTuning:
		return "tuning"
	case TypeTierPromote:
		return "tier_promote"
	case TypeTierDemote:
		return "tier_demote"
	case TypeScenarioMark:
		return "scenario_mark"
	case TypeRunStart:
		return "run_start"
	case TypeRunEnd:
		return "run_end"
	default:
		return "unknown"
	}
}

// Event is the unit carried on the bus.
//
// FROZEN after P1: see docs/DECISIONS.md before changing.
type Event struct {
	// Seq is the monotonic request index at the time of emission.
	Seq       uint64    `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Type      Type      `json:"type"`
	// Payload is one of the *Event structs in this file, or nil.
	Payload any `json:"payload,omitempty"`
}

// SwitchEvent is the payload of TypeSwitch.
type SwitchEvent struct {
	From          types.PolicyName   `json:"from"`
	To            types.PolicyName   `json:"to"`
	Workload      types.WorkloadType `json:"workload"`
	Confidence    float64            `json:"confidence"`
	HitRateBefore float64            `json:"hit_rate_before"`
	Reason        string             `json:"reason"`
	OverheadMs    float64            `json:"overhead_ms"`
	EntriesKept   int                `json:"entries_kept"`
}

// DetectionEvent is the payload of TypeDetection.
type DetectionEvent struct {
	Workload    types.WorkloadType `json:"workload"`
	Previous    types.WorkloadType `json:"previous"`
	Confidence  float64            `json:"confidence"`
	Features    types.Features     `json:"features"`
	WindowStart uint64             `json:"window_start"`
	WindowEnd   uint64             `json:"window_end"`
}

// TuningEvent is the payload of TypeTuning.
type TuningEvent struct {
	Policy       types.PolicyName `json:"policy"`
	Parameter    string           `json:"parameter"`
	OldValue     float64          `json:"old_value"`
	NewValue     float64          `json:"new_value"`
	MetricBefore float64          `json:"metric_before"`
	MetricAfter  float64          `json:"metric_after"`
	Accepted     bool             `json:"accepted"`
}

// ScenarioMarkEvent is the payload of TypeScenarioMark: ground truth emitted by
// the trace source, used to compute detection delay honestly.
type ScenarioMarkEvent struct {
	Seq          uint64             `json:"seq"`
	FromWorkload types.WorkloadType `json:"from_workload"`
	ToWorkload   types.WorkloadType `json:"to_workload"`
	SegmentName  string             `json:"segment_name"`
}

// Frame is the aggregated UI transport record (context.md §3.5). Frames are
// emitted on a fixed tick (default 10 Hz) rather than per request, so the UI
// costs a bounded, tiny amount of bandwidth regardless of request rate.
//
// FROZEN after P1: see docs/DECISIONS.md before changing.
type Frame struct {
	Seq       uint64    `json:"seq"`
	WallClock time.Time `json:"wall_clock"`
	// Progress is 0..1 through the scenario.
	Progress float64 `json:"progress"`

	HitRate     float64 `json:"hit_rate"`
	ByteHitRate float64 `json:"byte_hit_rate"`
	// Throughput is requests per second.
	Throughput float64 `json:"throughput"`
	// P50, P95 and P99 are latencies in milliseconds.
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`

	Policy string `json:"policy"`
	// PolicyResidency is requests since the last switch.
	PolicyResidency uint64  `json:"policy_residency"`
	Workload        string  `json:"workload"`
	Confidence      float64 `json:"confidence"`

	BytesUsed     int64 `json:"bytes_used"`
	Capacity      int64 `json:"capacity"`
	ObjectCount   int   `json:"object_count"`
	MetadataBytes int64 `json:"metadata_bytes"`

	Features types.Features `json:"features"`

	// Shadow holds counterfactual hit rates, e.g. "lru":0.71 "oracle":0.83.
	Shadow map[string]float64 `json:"shadow,omitempty"`
	// Params holds current tunable values.
	Params map[string]float64 `json:"params,omitempty"`
}
