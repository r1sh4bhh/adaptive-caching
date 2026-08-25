# MASTER PROJECT CONTEXT
## Dynamic Adaptive Caching with Multi-Objective Optimization

> **READ THIS FIRST — instructions to any AI agent**
>
> This document is the single source of truth for this project. Before writing
> any code, read PART 0 (Orientation), PART 3 (Architecture), and PART 8
> (Phase Plan) in full. Then locate the **current phase** in PART 8 and work
> only within its scope.
>
> Hard rules, in priority order:
> 1. **Never fabricate results.** All numeric targets in this document are
>    hypotheses/success criteria, NOT measurements. See PART 9.
> 2. **Never break the layering.** `cache/` must never import `ui/`,
>    `benchmark/`, or `server/`. See PART 3.6.
> 3. **Build incrementally.** Do not jump from "build the adaptive cache" to
>    thousands of lines. Follow PART 10 (Workflow).
> 4. **Everything is seeded and reproducible.** See PART 6.4.
> 5. **Everything is feature-flagged.** Ablations must be config permutations,
>    never code branches. See PART 7.3.

---

# TABLE OF CONTENTS

- **PART 0 — ORIENTATION** — what this is, in 60 seconds
- **PART 1 — RESEARCH FRAMING** — problem, gap, positioning, research questions
- **PART 2 — SYSTEM BEHAVIOUR** — what the system must actually do
- **PART 3 — ARCHITECTURE** — components, layering, data flow, control flow
- **PART 4 — INTERFACES & DATA STRUCTURES** — the contracts
- **PART 5 — ALGORITHMS** — policies, detection, adaptation, tuning, eviction
- **PART 6 — WORKLOADS, TRACES & REPRODUCIBILITY**
- **PART 7 — EVALUATION** — metrics, ablations, statistics, visualisations
- **PART 8 — PHASE PLAN** — the incremental build roadmap (P1–P14)
- **PART 9 — ACADEMIC INTEGRITY & TARGETS**
- **PART 10 — WORKING AGREEMENT WITH AI AGENTS**
- **PART 11 — OBSERVABILITY, UI & DEMO**
- **PART 12 — FEATURE BACKLOG** — optional extensions ranked by value
- **PART 13 — SCOPE BOUNDARIES** — what we will NOT build
- **PART 14 — PROJECT PHASES (ACADEMIC) & CURRENT STATUS**

---
---

# PART 0 — ORIENTATION

## 0.1 One sentence

Build a cache that does not permanently rely on one eviction policy; instead it
continuously analyses the workload, determines what type of workload is
currently occurring, selects the most appropriate eviction policy, tunes that
policy when necessary, makes size-aware eviction decisions, and feeds observed
performance back into future decisions.

## 0.2 The core diagram — never lose sight of this

```
        TRADITIONAL CACHE                    OUR SYSTEM

        Workload                             Workload
           │                                    │
           ▼                                    ▼
      Fixed Policy                          OBSERVE
           │                                    │
           ▼                                    ▼
         Cache                             UNDERSTAND
                                                │
                                                ▼
                                             SELECT
                                                │
                                                ▼
                                              TUNE
                                                │
                                                ▼
                                              EVICT
                                                │
                                                ▼
                                             MEASURE
                                                │
                                                ▼
                                              ADAPT
                                                │
                                                └──────┐
                                                       │
                                    ┌──────────────────┘
                                    ▼
                            (feeds back to OBSERVE)
```

The system must continuously answer:

> **"Given what the workload looks like right now, what caching strategy
> should I use right now?"**

## 0.3 Project metadata

| Field | Value |
|---|---|
| Title | Dynamic Adaptive Caching with Multi-Objective Optimization |
| Type | Year-2, 2-credit Course-Integrated Design Project |
| Nature | Systems / research prototype — NOT a CRUD app, NOT a toy cache |
| Language | **Go** (systems programming, concurrency, benchmarking, single-binary deploy) |
| Frontend | Vanilla/Svelte SPA + uPlot, embedded via `go:embed` (Phase P14 only) |
| Config | YAML |
| Results format | CSV + JSON |
| Figures | Python + matplotlib (offline report generator) |

## 0.4 The three-layer mental model

```
┌─────────────────────────────────────────────────────────┐
│ LAYER 3 — PRESENTATION (optional, deletable)            │
│   TUI dashboard · Web dashboard · Report generator      │
│   READ-ONLY consumers of the event bus                  │
├─────────────────────────────────────────────────────────┤
│ LAYER 2 — EXPERIMENTATION (the science)                 │
│   Benchmark runner · Trace sources · Metrics · Stats    │
├─────────────────────────────────────────────────────────┤
│ LAYER 1 — THE CACHE (the contribution)                  │
│   Cache core · Policies · Detector · Adaptive engine    │
│   Tuner · Size-aware eviction · Tiers                   │
└─────────────────────────────────────────────────────────┘
```

**If you delete Layer 3, nothing breaks.** That is the design constraint that
makes the UI cheap. The UI is an *observability layer*, not a dependency.

---
---

# PART 1 — RESEARCH FRAMING

## 1.1 The fundamental problem

Existing cache eviction policies — LRU, LFU, ARC, W-TinyLFU, Clock — are strong
policies, but they are generally configured as **fixed** strategies.

Real workloads are not necessarily stationary. A workload can transition
between:

- temporal locality
- spatial locality
- working-set behaviour
- random access
- bursty traffic
- mixed patterns

A policy that works very well for one workload can perform poorly when the
workload changes.

- **LRU** works well when recent accesses are likely to be reused.
- **LFU** works well when a stable set of frequently accessed objects dominates.
- **ARC** balances recency and frequency, useful for mixed workloads.
- **W-TinyLFU** is a strong modern production-oriented baseline.
- **Clock** is a simple low-overhead baseline.

> **There is no single universally optimal eviction policy for every workload.**

This is the central motivation, identified explicitly in the Review 1 material.

## 1.2 Research gap

We are **not** claiming LRU, LFU, ARC, adaptive caching, or distributed caching
are individually new. Existing research already provides strong fixed policies,
workload-specific optimisation, distributed caching, evaluation frameworks, and
production implementations.

The gap is the **combination** of:

1. Dynamic workload detection
2. Automatic policy selection
3. Online policy parameter tuning
4. Heterogeneous object-size awareness
5. Evaluation under changing workloads
6. Optional multi-tier cache coordination

## 1.3 Positioning — say this, not that

❌ **DO NOT SAY:** "We invented a new cache eviction algorithm."

✅ **DO SAY:** "We are designing and evaluating a workload-aware adaptive cache
framework that dynamically selects and tunes existing eviction policies
according to observed workload characteristics."

This distinction is academically important. Enforce it in code comments,
documentation, README, and slides.

## 1.4 Research questions

These drive every experiment. Every phase should move at least one forward.

| ID | Question | Answered in phase |
|---|---|---|
| **RQ1** | Does workload-aware policy selection improve hit rate vs a fixed LRU baseline? | P7 |
| **RQ2** | Does the adaptive system outperform the *best fixed policy* on workloads that transition between patterns? | P7 (+ shadow caches, P4) |
| **RQ3** | How quickly can the system detect workload transitions? | P6 |
| **RQ4** | What is the overhead of workload detection? | P6, P12 |
| **RQ5** | What is the overhead of policy switching? | P7, P12 |
| **RQ6** | Does size-aware eviction improve byte hit rate or memory efficiency for heterogeneous objects? | P9 |
| **RQ7** | Does online parameter tuning improve performance beyond policy switching alone? | P10 |
| **RQ8** | Does multi-tier caching improve performance enough to justify its coordination overhead? | P13 |

## 1.5 The research narrative

- **Problem** — Fixed cache policies assume relatively stable workload behaviour.
- **Observation** — Real workloads change.
- **Hypothesis** — A cache that detects workload characteristics and adapts its
  policy can perform better than a fixed policy under changing workloads.
- **Solution** — A workload-aware adaptive cache framework.
- **Mechanism** — Observe → Extract features → Classify → Select → Tune →
  Size-aware evict → Measure → Adapt.
- **Evaluation** — vs LRU, LFU, ARC, W-TinyLFU, Clock under temporal, spatial,
  random, working-set, bursty, mixed workloads.
- **Research question** — Does adaptive policy selection provide measurable
  benefit, and at what overhead?

## 1.6 Success condition

The project succeeds if we demonstrate with **reproducible evidence**:

1. Correct implementation of established cache policies.
2. Identification of different workload patterns.
3. Detection of workload transitions.
4. Dynamic policy selection.
5. Parameter tuning without restarting.
6. Handling of variable object sizes.
7. Measurement of adaptation overhead.
8. Fair comparison against fixed baselines.
9. Demonstration of where adaptation **helps**.
10. Demonstration of where adaptation **does NOT help**.

> **Point 10 is not a consolation prize — it is a requirement.**
> A good research project does not need the proposed approach to win every
> experiment. If LRU wins on some workload, that is a valid and useful result.
> Report it prominently.

---
---

# PART 2 — SYSTEM BEHAVIOUR

## 2.1 The request

Minimum fields:

```
Request {
    Key       string    // object identifier
    Size      int64     // bytes
    Timestamp time.Time
}
```

Optional / extended fields:

```
    RequestID  uint64
    Op         OpType     // GET / PUT / DELETE
    ClientID   string
    Tier       TierID
    MissCost   time.Duration  // see PART 12 feature #8
    TTL        time.Duration  // see PART 12 feature #8
    Metadata   map[string]string
```

## 2.2 Request lifecycle (conceptual)

```
                        Client / TraceSource
                                │
                                ▼
                       ┌────────────────┐
                       │ Cache Frontend │
                       └────────┬───────┘
                                │
                                ▼
                       ┌────────────────┐
                       │  Cache Lookup  │
                       └────────┬───────┘
                                │
                  ┌─────────────┴─────────────┐
                 HIT                          MISS
                  │                             │
                  ▼                             ▼
          Return Object              ┌──────────────────┐
                  │                  │ Workload Monitor │
                  │                  └────────┬─────────┘
                  │                           ▼
                  │                  ┌──────────────────┐
                  │                  │Feature Extractor │
                  │                  └────────┬─────────┘
                  │                           ▼
                  │                  ┌──────────────────┐
                  │                  │Workload Classifier│
                  │                  └────────┬─────────┘
                  │                           ▼
                  │                  ┌──────────────────┐
                  │                  │ Adaptive Engine  │
                  │                  └────────┬─────────┘
                  │                           ▼
                  │                  ┌──────────────────┐
                  │                  │ Selected Policy  │
                  │                  └────────┬─────────┘
                  │                           ▼
                  │                  ┌──────────────────┐
                  │                  │ Size-Aware Evict │
                  │                  │  + Cache Insert  │
                  │                  └────────┬─────────┘
                  │                           │
                  └───────────┬───────────────┘
                              ▼
                     ┌──────────────────┐
                     │ Metrics Collector│
                     └────────┬─────────┘
                              ▼
                     ┌──────────────────┐
                     │    EVENT BUS     │
                     └────────┬─────────┘
                              │
                              ▼
                       Future Adaptation
                        (feedback loop)
```

**Important:** the Workload Monitor observes **every** request (hit and miss),
not only misses. The diagram above shows the miss path because that is where
eviction happens; feature extraction is fed by all requests.

## 2.3 Worked example — one request end-to-end

```
Request{Key:"obj_77", Size:4096, Timestamp:1024ms}

 1. Cache.Get("obj_77")                       → MISS
 2. Monitor.Record(req)                       // O(1) ring buffer append
 3. every N requests:
      FeatureExtractor.Extract(window)
        → reuseDistanceMean = 812
          keyEntropy        = 9.4
          burstinessCV      = 0.31
          workingSetEst     = 4820
          sizeP95           = 65536
 4. Classifier.Classify(features)
        → WorkloadPrediction{Type: RANDOM, Confidence: 0.91}
 5. AdaptiveEngine.Decide(prediction, cacheStats, policyHistory)
      guards:
        confidence   0.91  > threshold      0.80   ✓
        residency    4102  > minResidency   2000   ✓
        cooldown     elapsed                       ✓
        expectedGain 0.06  > switchCost 0.01
                              + minGain   0.02     ✓
        → AdaptiveDecision{
             CurrentPolicy:     LFU
             RecommendedPolicy: ARC
             Confidence:        0.91
             Reason:            "low reuse + high key entropy"
             Parameters:        {beta: 0.5}
             ShouldSwitch:      true
          }
 6. PolicySwitcher.Switch(LFU → ARC)
      shared object store preserved; ARC metadata rebuilt
      measured overhead: 12.4 ms
 7. EventBus.Publish(SwitchEvent{...})
      ├─→ MetricsCollector  → appended to results/run_047/switches.csv
      ├─→ Logger            → "[20279] LFU→ARC conf=0.91 reason=..."
      ├─→ WSBroadcaster     → UI redraws timeline, flashes decision card
      └─→ ShadowCaches      → (unaffected; they continue independently)
 8. SizeAwareEviction.EvictUntil(4096)
      evicts obj_12 (1024B), obj_31 (2048B), obj_9 (2048B) → 5120B freed
 9. Cache.Put("obj_77", value, 4096)
10. MetricsCollector.RecordMiss(latency, bytes)
```

Note that step 7's `SwitchEvent` is *simultaneously* a log line, a CSV row, and
a UI notification. **One struct, three consumers, zero coupling.**

---
---

# PART 3 — ARCHITECTURE

## 3.1 Component map

```
┌───────────────────────────────────────────────────────────────────────┐
│                          SINGLE GO BINARY                             │
│                                                                       │
│  ┌──────────────┐                                                     │
│  │ TraceSource  │  synthetic generators │ CSV loader │ live injector  │
│  └──────┬───────┘                                                     │
│         │ Request stream                                              │
│         ▼                                                             │
│  ┌─────────────────────────────────────────────────┐                  │
│  │              CACHE CORE                         │                  │
│  │  ┌───────────────┐    ┌──────────────────────┐  │                  │
│  │  │ Object Store  │    │  EvictionPolicy      │  │                  │
│  │  │ key → Entry   │◄──►│  (LRU/LFU/ARC/       │  │                  │
│  │  │ (shared,      │    │   W-TinyLFU/Clock)   │  │                  │
│  │  │  survives     │    └──────────────────────┘  │                  │
│  │  │  switches)    │    ┌──────────────────────┐  │                  │
│  │  └───────────────┘    │  SizeAwareScorer     │  │                  │
│  │                       │  + BatchEvictor      │  │                  │
│  │                       └──────────────────────┘  │                  │
│  └───────┬─────────────────────────────┬───────────┘                  │
│          │                             │                              │
│          ▼                             ▼                              │
│  ┌───────────────┐            ┌─────────────────┐                     │
│  │WorkloadMonitor│            │  AdaptiveEngine │                     │
│  │  ring buffer  │            │  ┌───────────┐  │                     │
│  └───────┬───────┘            │  │ Selector  │  │                     │
│          ▼                    │  ├───────────┤  │                     │
│  ┌───────────────┐            │  │  Guards   │  │                     │
│  │FeatureExtractor│──────────►│  ├───────────┤  │                     │
│  └───────┬───────┘            │  │  Tuner    │  │                     │
│          ▼                    │  ├───────────┤  │                     │
│  ┌───────────────┐            │  │ Switcher  │  │                     │
│  │  Classifier   │───────────►│  └───────────┘  │                     │
│  └───────────────┘            └────────┬────────┘                     │
│                                        │                              │
│  ┌─────────────────────────────────────▼───────────────────────────┐  │
│  │                        EVENT BUS                                │  │
│  │   bounded, droppable, fire-and-forget, non-blocking             │  │
│  └──┬──────────┬──────────────┬──────────────┬────────────────────┘  │
│     │          │              │              │                        │
│     ▼          ▼              ▼              ▼                        │
│ ┌────────┐┌────────┐┌──────────────┐┌────────────────┐                │
│ │Metrics ││ Logger ││FrameAggregator││ ShadowCaches  │                │
│ │CSV/JSON││        ││   (10 Hz)     ││ LRU/LFU/ARC/  │                │
│ └────────┘└────────┘└──────┬───────┘│ Oracle (Bélády)│                │
│                            │        └────────────────┘                │
│                            ▼                                          │
│                    ┌───────────────┐                                  │
│                    │ WSBroadcaster │                                  │
│                    └───────┬───────┘                                  │
│                            │                                          │
│  HTTP SERVER               │                                          │
│    GET  /          → embedded SPA (go:embed)                          │
│    WS   /ws        ◄───────┘  Frames (10 Hz) + Events (immediate)     │
│    POST /control   → start|pause|resume|seed|speed|inject|scenario    │
│    GET  /api/runs  → historical results                               │
│    GET  /api/explain → last N AdaptiveDecisions + feature vectors     │
│    GET  /metrics   → Prometheus (optional)                            │
│                                                                       │
│  ALTERNATE FRONTEND (fallback, no browser required)                   │
│    TUI dashboard (bubbletea) — subscribes to the same FrameAggregator │
└───────────────────────────────────────────────────────────────────────┘
```

## 3.2 Module list

| # | Module | Package | Responsibility |
|---|---|---|---|
| 1 | Cache Core | `cache/` | Object store, capacity accounting, Get/Put/Remove |
| 2 | Eviction Policies | `cache/policy/` | LRU, LFU, ARC, W-TinyLFU, Clock |
| 3 | Workload Monitor | `workload/` | Sliding-window request recording |
| 4 | Feature Extractor | `workload/features/` | Locality, frequency, burstiness, size stats |
| 5 | Workload Classifier | `workload/classify/` | Features → WorkloadPrediction |
| 6 | Adaptive Engine | `adaptive/` | Selection, guards, switching orchestration |
| 7 | Parameter Tuner | `tuning/` | Online A/B parameter optimisation |
| 8 | Heterogeneous Objects | `eviction/` | Size-aware scoring, batch eviction |
| 9 | Metrics Collector | `metrics/` | Counters, latency histograms, memory accounting |
| 10 | Event Bus | `events/` | Typed pub/sub, bounded + droppable |
| 11 | Trace Sources | `trace/` | Synthetic generators, CSV loader, injector |
| 12 | Benchmark Engine | `benchmark/` | Experiment matrix, runner, results |
| 13 | Multi-Tier | `tiers/` | L1/L2 (+L3), promotion/demotion |
| 14 | Config | `config/` | YAML load, validation, feature flags |
| 15 | Server | `server/` | HTTP + WebSocket + embedded SPA |
| 16 | TUI | `tui/` | Terminal dashboard |
| 17 | Shadow Caches | `shadow/` | Counterfactual simulators + Bélády oracle |

## 3.3 Data flow vs control flow

**Data flow** (requests and objects) is strictly downward:

```
TraceSource → Cache → ObjectStore
                  ↘ → Monitor → Features → Classifier
```

**Control flow** (decisions) is a loop:

```
Classifier → AdaptiveEngine → Policy (mutates behaviour of Cache)
     ▲                                        │
     └──────── Metrics ◄──── EventBus ◄───────┘
```

**Observation flow** is strictly outward and one-way:

```
Everything → EventBus → [Metrics | Logger | Frames | Shadows]
```

Nothing on the right of the EventBus may ever call back into the left.

## 3.4 The Event Bus — design contract

This is the keystone. Get it right in **P1**; retrofitting it later is painful.

```go
package events

type Type uint8

const (
    TypeRequest Type = iota  // sampled, not every request at high rates
    TypeHit
    TypeMiss
    TypeEviction
    TypeDetection      // classifier produced a prediction
    TypeSwitch         // policy changed
    TypeTuning         // parameter changed
    TypeTierPromote
    TypeTierDemote
    TypeScenarioMark   // ground-truth workload transition boundary
    TypeRunStart
    TypeRunEnd
)

type Event struct {
    Seq       uint64      // monotonic request index at time of emission
    Timestamp time.Time
    Type      Type
    Payload   any         // one of the *Event structs below
}

type Bus interface {
    Publish(Event)                    // MUST be non-blocking
    Subscribe(name string, buf int, types ...Type) <-chan Event
    Unsubscribe(name string)
    DroppedCount(name string) uint64  // observability on the observability
}
```

**Rules:**

- `Publish` **must never block.** Each subscriber has a bounded channel; if it
  is full, **drop the event and increment a counter.** A slow browser must never
  backpressure the cache and corrupt latency measurements.
- Subscribers that must be lossless (MetricsCollector) get a large buffer and
  are drained by a dedicated goroutine; if they still drop, that is a bug worth
  failing a test over.
- `TypeRequest` events are **sampled** (configurable, default 1-in-1000) because
  at 500k req/s the bus would dominate the profile. Aggregate counters live in
  MetricsCollector, not on the bus.

### Payload structs

```go
type SwitchEvent struct {
    From, To      PolicyName
    Workload      WorkloadType
    Confidence    float64
    HitRateBefore float64
    Reason        string
    OverheadMs    float64
    EntriesKept   int
}

type DetectionEvent struct {
    Workload    WorkloadType
    Previous    WorkloadType
    Confidence  float64
    Features    Features
    WindowStart uint64
    WindowEnd   uint64
}

type TuningEvent struct {
    Policy    PolicyName
    Parameter string
    OldValue  float64
    NewValue  float64
    MetricBefore, MetricAfter float64
    Accepted  bool
}

type ScenarioMarkEvent struct {   // emitted by TraceSource, ground truth
    Seq          uint64
    FromWorkload WorkloadType
    ToWorkload   WorkloadType
    SegmentName  string
}
```

`ScenarioMarkEvent` is what lets us compute **detection delay** honestly and
draw the "actual vs detected" markers in the UI. The generator knows the truth;
the detector must not.

## 3.5 Frame protocol (UI transport)

Do **not** stream every request to the browser. Aggregate server-side at a fixed
tick (default 10 Hz).

```go
type Frame struct {
    Seq          uint64
    WallClock    time.Time
    Progress     float64            // 0..1 through the scenario

    HitRate      float64            // windowed
    ByteHitRate  float64
    Throughput   float64            // req/s
    P50, P95, P99 float64           // ms

    Policy       string
    PolicyResidency uint64          // requests since last switch
    Workload     string
    Confidence   float64

    BytesUsed    int64
    Capacity     int64
    ObjectCount  int
    MetadataBytes int64

    Features     Features           // for the radar chart

    Shadow       map[string]float64 // "lru":0.71 "lfu":0.68 "arc":0.74 "oracle":0.83

    Params       map[string]float64 // current tunable values
}
```

Two WebSocket channels multiplexed on one connection:

1. **Frames** — 10 Hz heartbeat, ~5 KB/s. Drives all charts.
2. **Events** — pushed immediately when they occur (switch/detection/tuning/
   scenario mark). Rare. Drives the decision card and timeline annotations.

## 3.6 Layering rules — enforce with a CI lint

```
ALLOWED IMPORT DIRECTION (top may import bottom, never reverse)

    cmd/  ─────────────────────────────────────┐
    server/ · tui/ · benchmark/ ───────────┐   │
    shadow/ · tiers/ ──────────────────┐   │   │
    adaptive/ · tuning/ ───────────┐   │   │   │
    workload/ · eviction/ ─────┐   │   │   │   │
    cache/ · trace/ ───────┐   │   │   │   │   │
    events/ · metrics/ · config/ · types/  (leaves — import nothing internal)
```

**Forbidden, non-negotiable:**

- `cache/` importing `ui/`, `server/`, `tui/`, `benchmark/`, `adaptive/`
- Any policy knowing that a dashboard exists
- Any component reading global mutable state

Add a `make lint-arch` target (e.g. `go-arch-lint` or a simple script over
`go list -deps`) in P1 and wire it into CI. This single check is what keeps the
UI cheap for the whole project.

## 3.7 Repository structure

```
adaptive-cache/
├── cmd/
│   ├── adaptive-cache/main.go      # server + UI
│   ├── bench/main.go               # headless benchmark runner
│   └── analyze/main.go             # trace → feature vector
│
├── types/                          # shared leaf types, zero deps
│   ├── request.go
│   ├── entry.go
│   ├── workload.go
│   └── policy.go
│
├── events/
│   ├── bus.go
│   ├── types.go
│   └── bus_test.go
│
├── config/
│   ├── config.go
│   ├── flags.go                    # feature flags
│   └── validate.go
│
├── metrics/
│   ├── collector.go
│   ├── latency.go                  # HDR histogram / t-digest
│   ├── memory.go                   # runtime.ReadMemStats + Sizeof breakdown
│   └── statistics.go               # mean, CI, p95, ANOVA, Cohen's d
│
├── cache/
│   ├── cache.go                    # Cache interface
│   ├── store.go                    # shared object store
│   ├── core.go                     # orchestration
│   └── policy/
│       ├── policy.go               # EvictionPolicy interface
│       ├── lru.go
│       ├── lfu.go
│       ├── arc.go
│       ├── tinylfu.go
│       ├── clock.go
│       ├── sketch/                 # count-min, doorkeeper
│       └── *_test.go
│
├── eviction/
│   ├── scorer.go                   # SizeAwareScorer interface
│   ├── strategies.go               # hitprob/size, score/sqrt(size), GDSF...
│   └── batch.go                    # EvictUntil
│
├── workload/
│   ├── monitor.go                  # ring buffer
│   ├── features/
│   │   ├── features.go
│   │   ├── locality.go             # reuse distance, stack distance
│   │   ├── frequency.go            # entropy, Zipf skew, top-K
│   │   ├── burstiness.go           # CV, spike ratio
│   │   ├── workingset.go
│   │   └── size.go
│   └── classify/
│       ├── classifier.go           # interface
│       ├── rules.go                # V1 rule-based
│       ├── tree.go                 # V2 decision tree
│       └── learned.go              # V3 trained on shadow outcomes
│
├── adaptive/
│   ├── engine.go
│   ├── selector.go                 # workload → policy (configurable)
│   ├── guards.go                   # hysteresis, residency, cooldown
│   ├── switcher.go                 # state-preserving switch
│   └── decision.go
│
├── tuning/
│   ├── tuner.go
│   ├── parameter.go                # registry: range/default/current
│   └── optimizer.go                # A/B sweep
│
├── shadow/
│   ├── shadow.go                   # metadata-only simulators
│   ├── oracle.go                   # Bélády MIN (offline)
│   └── regret.go                   # cumulative regret vs hindsight-best
│
├── tiers/
│   ├── tier.go
│   ├── l1.go · l2.go · l3.go
│   ├── promote.go
│   └── netsim.go                   # simulated latency
│
├── trace/
│   ├── source.go                   # TraceSource interface
│   ├── csv.go
│   ├── synthetic/
│   │   ├── temporal.go · spatial.go · random.go
│   │   ├── bursty.go · workingset.go · mixed.go
│   │   └── sizes.go                # size distributions
│   ├── scenario.go                 # YAML scenario replay
│   └── injector.go                 # live injection (UI-driven)
│
├── benchmark/
│   ├── runner.go
│   ├── experiment.go
│   ├── matrix.go                   # policy × workload × size × seed
│   └── results.go                  # CSV/JSON writers
│
├── server/
│   ├── server.go
│   ├── ws.go                       # broadcaster
│   ├── frames.go                   # FrameAggregator
│   ├── control.go                  # POST /control
│   ├── api.go                      # /api/runs, /api/explain
│   └── web/                        # go:embed target
│       ├── index.html
│       ├── app.js
│       └── style.css
│
├── tui/
│   └── dashboard.go                # bubbletea
│
├── scenarios/
│   ├── showcase.yaml
│   ├── adversarial_oscillation.yaml
│   ├── heterogeneous.yaml
│   └── ...
│
├── configs/
│   ├── default.yaml
│   └── experiments/
│       ├── ablation_a_lru.yaml
│       ├── ablation_c_no_tuning.yaml
│       └── ...
│
├── scripts/
│   ├── benchmark.sh
│   ├── generate_workloads.sh
│   └── report/                     # python + matplotlib
│       ├── figures.py
│       └── report.py
│
├── tests/
│   ├── integration/
│   └── edge/
│
├── docs/
│   ├── ARCHITECTURE.md
│   ├── API.md
│   ├── EXPERIMENTS.md
│   ├── RESULTS.md
│   ├── DECISIONS.md                # ADR log — why we chose things
│   └── LIMITATIONS.md
│
├── results/                        # gitignored except summaries
├── Dockerfile
├── Makefile
├── CHANGELOG.md
├── go.mod
└── README.md
```

> This tree is a strong default, not scripture. If a better Go design is
> justified, change it — **and record why in `docs/DECISIONS.md`.**

---
---

# PART 4 — INTERFACES & DATA STRUCTURES

## 4.1 Cache

```go
type Cache interface {
    Get(key string) (Value, bool)
    Put(key string, val Value, size int64) error
    Remove(key string) bool
    Contains(key string) bool

    Len() int          // object count
    Bytes() int64      // bytes used
    Capacity() int64   // byte capacity
    Stats() Stats

    Clear()
}
```

Capacity is expressed in **bytes**, not object count. This is a deliberate
choice driven by PART 5.5 (heterogeneous objects).

## 4.2 EvictionPolicy

Every policy implements this, so the adaptive engine can treat LRU, LFU, ARC,
W-TinyLFU, Clock and future policies as **interchangeable strategies**.

```go
type EvictionPolicy interface {
    Name() PolicyName

    // Lifecycle hooks — the policy maintains metadata only.
    // It does NOT own the objects.
    OnAccess(key string, e *Entry)
    OnInsert(key string, e *Entry)
    OnRemove(key string, e *Entry)

    // Eviction
    Victim() (key string, ok bool)          // single best victim
    Candidates(n int) []string              // for batch eviction

    // Admission (TinyLFU and adaptive admission need this)
    ShouldAdmit(key string, size int64) bool

    // Parameter tuning
    Params() ParamSet
    SetParam(name string, v float64) error

    // Policy switching — see PART 5.4
    Rebuild(entries []*Entry)               // adopt existing cache contents
    MetadataBytes() int64                   // for memory-overhead accounting

    Reset()
}
```

**Key design point:** the policy holds *metadata*, the store holds *objects*.
This separation is what makes state-preserving policy switching possible.

## 4.3 Workload pipeline

```go
type WorkloadMonitor interface {
    Record(r Request)
    Window() []Request      // most recent N
    WindowSize() int
    SetWindowSize(int)
}

type FeatureExtractor interface {
    Extract(window []Request) Features
    Names() []string        // stable ordering for CSV/ML
}

type Classifier interface {
    Classify(f Features) WorkloadPrediction
    Name() string
}
```

```go
type Features struct {
    // Temporal locality
    ReuseDistanceMean   float64
    ReuseDistanceP50    float64
    ReuseDistanceP95    float64
    RepeatedKeyRatio    float64
    InterArrivalMean    float64

    // Spatial locality
    KeyDistanceMean     float64
    ContiguousRatio     float64
    RangeDensity        float64

    // Frequency distribution
    UniqueKeys          int
    TopKConcentration   float64   // share of requests in top-K keys
    KeyEntropy          float64
    ZipfAlphaEstimate   float64

    // Burstiness
    RequestRateMean     float64
    RequestRateStdDev   float64
    BurstinessCV        float64   // stddev/mean
    SpikeRatio          float64

    // Working set
    WorkingSetEstimate  int
    ActiveSetStability  float64

    // Object size
    SizeMean, SizeMedian, SizeP95, SizeVariance float64
    SmallRatio, MediumRatio, LargeRatio         float64
    BytesRequested                              int64
}

type WorkloadPrediction struct {
    Type        WorkloadType
    Confidence  float64
    Features    Features
    WindowStart uint64
    WindowEnd   uint64
    DetectedAt  time.Time
}

type WorkloadType uint8
const (
    WorkloadUnknown WorkloadType = iota
    WorkloadTemporal
    WorkloadSpatial
    WorkloadWorkingSet
    WorkloadRandom
    WorkloadBursty
    WorkloadMixed
)
```

## 4.4 Adaptive engine

```go
type PolicySelector interface {
    Select(p WorkloadPrediction, s Stats, h PolicyHistory) (PolicyName, string)
}

type AdaptiveEngine interface {
    Observe(r Request, hit bool)
    Decide() (AdaptiveDecision, bool)
    Apply(AdaptiveDecision) error
    History() []AdaptiveDecision
}

type AdaptiveDecision struct {
    Seq               uint64
    CurrentPolicy     PolicyName
    RecommendedPolicy PolicyName
    ShouldSwitch      bool
    Reason            string            // human-readable — REQUIRED
    Confidence        float64
    Workload          WorkloadType
    Parameters        map[string]float64
    ExpectedGain      float64
    EstimatedCost     float64
    GuardsPassed      []string
    GuardsFailed      []string          // why we did NOT switch — also valuable
}
```

Recording `GuardsFailed` is as important as recording switches — it explains
periods of *stability*, and it is what you show when a reviewer asks "why didn't
it switch there?"

## 4.5 Tuning

```go
type Parameter struct {
    Name     string
    Min, Max float64
    Default  float64
    Current  float64
    Step     float64
    Metric   string        // which metric this parameter is tuned against
}

type ParamSet map[string]*Parameter

type Tuner interface {
    Propose(current ParamSet, s Stats) (ParamSet, bool)
    Observe(candidate ParamSet, s Stats)
    Interval() uint64      // tune every N requests
}
```

## 4.6 Eviction scoring

```go
type SizeAwareScorer interface {
    Name() string
    Score(e *Entry, now time.Time) float64  // LOWER = evict first
}

type BatchEvictor interface {
    EvictUntil(requiredBytes int64) (evicted []string, freed int64)
}
```

## 4.7 Metrics

```go
type MetricsCollector interface {
    RecordRequest(r Request, hit bool, latency time.Duration)
    RecordEviction(key string, size int64)
    RecordSwitch(SwitchEvent)
    RecordTuning(TuningEvent)

    Snapshot() Stats
    Reset()
    WriteCSV(path string) error
    WriteJSON(path string) error
}

type Stats struct {
    TotalRequests uint64
    Hits, Misses  uint64
    HitRate       float64
    ByteHitRate   float64
    BytesServed, BytesFetched int64

    LatencyMean, LatencyP50, LatencyP95, LatencyP99 float64

    Capacity, BytesUsed, MetadataBytes int64
    ObjectCount int

    CurrentPolicy   PolicyName
    PolicySwitches  uint64
    SwitchOverheadTotalMs float64
    DetectionDelayMean    float64
    DetectionAccuracy     float64

    Evictions uint64
    BackendRequests uint64
}
```

## 4.8 Trace source

```go
type TraceSource interface {
    Next() (Request, bool)
    Describe() SourceInfo
    Close() error
}

type SourceInfo struct {
    Name        string
    Kind        string   // "synthetic" | "csv" | "scenario" | "injected"
    Seed        int64
    Length      uint64
    Workload    WorkloadType
    Params      map[string]any
    Segments    []Segment      // for scenarios — GROUND TRUTH
}

type Segment struct {
    Name      string
    Workload  WorkloadType
    StartSeq  uint64
    EndSeq    uint64
    Params    map[string]any
}
```

## 4.9 Tier

```go
type Tier interface {
    ID() TierID
    Get(key string) (Value, bool)
    Put(key string, v Value, size int64) error
    Evict() (key string, v Value, ok bool)
    Latency() time.Duration     // simulated
    Stats() Stats
}
```

---
---

# PART 5 — ALGORITHMS

## 5.1 Baseline policies

### LRU
Doubly-linked list + map. Track recency, insertion, access, eviction.
Characteristics: simple, predictable, low overhead, **vulnerable to sequential
scans**. Tunable: recency weight α (only meaningful in hybrid scoring modes —
if it does not meaningfully affect the implementation, do not expose it; see
PART 5.6).

### LFU
Frequency buckets (O(1) increment) + map. Must address:
- **cold-start problem** — new items have frequency 1 and are evicted immediately
- **stale frequencies** — old hot items stay hot forever
- **unbounded counters**

Mitigate with **frequency decay λ** (periodic halving or exponential decay).
This is a genuine tunable parameter.

### ARC
Implement the **real** ARC: T1, T2 (resident) + B1, B2 (ghost lists) with the
adaptive target size `p`.

> ❌ **Do NOT** implement a "recency + frequency score" and call it ARC.
> If you implement a simplified variant for the prototype, **explicitly document
> what is simplified** in `docs/LIMITATIONS.md`.

### W-TinyLFU
Window (LRU, ~1%) + main (SLRU: probation + protected) + count-min sketch
frequency estimator + doorkeeper bloom filter + periodic sketch reset.
Admission: a candidate is admitted only if its estimated frequency exceeds the
victim's.

> If a fully faithful implementation is too large for the MVP, structure the
> code so it can be completed cleanly, and **clearly document the implementation
> boundary. Do not falsely claim production fidelity.**

### Clock
Circular buffer + reference bits. Second-chance approximation of LRU.
Low-overhead comparison point.

## 5.2 Workload detection

### Windowing

```
requests:  ... ─────────────────────────────────────────►
                     ┌──────── feature window (N) ───────┐
                     │  stable statistical features      │
                     └───────────────────────────────────┘
                                              ┌── detection
                                              │   window (M)
                                              └───  fast transitions
```

The original design specifies a feature-analysis window of ~1000 requests, while
the objective is to identify transitions within ~10–20 requests. These are in
tension. Resolve with **two windows**:

- **Feature window** (large, default 1000) — stable statistics
- **Detection window** (small, default 50) — change-point signal

> **Do NOT hard-code 1000 anywhere.** Both must be configurable and swept.

### Classifier progression

| Version | Approach | Phase |
|---|---|---|
| V1 | Rule-based thresholds | P6 |
| V2 | Decision tree | P6/P11 |
| V3 | Lightweight learned model trained on **shadow-cache outcomes** | P11+ (optional) |

> ❌ Do **not** start with deep learning. This is a systems project.
> Prefer: interpretable, fast, deterministic, low-overhead, benchmarkable.

Confidence threshold default **0.80** — configurable.

### Detection delay — measure it, never assume it

```go
detectionDelay := detectionPoint - transitionPoint
```

`transitionPoint` comes from `ScenarioMarkEvent` (ground truth from the
generator). `detectionPoint` comes from the first `DetectionEvent` whose `Type`
matches the new segment with confidence ≥ threshold.

> **Never fabricate detection accuracy or transition speed.** Report the measured
> distribution: mean, median, p95, and failures-to-detect.

## 5.3 Policy selection

Initial mapping — these are **hypotheses and decision rules, not guaranteed
truths**. They live in config, not in code:

```yaml
selector:
  mapping:
    TEMPORAL:     LRU
    SPATIAL:      LRU
    WORKING_SET:  LFU
    RANDOM:       ARC
    BURSTY:       W_TINYLFU    # re-evaluate against live hit rate
    MIXED:        ARC
    UNKNOWN:      keep_current
```

> ❌ Do **not** hard-code the claim that these mappings are universally optimal.
> The architecture must allow modification based on experimental results.
> Ideally, **replace this table with one derived from shadow-cache outcomes**
> (PART 12 feature #6).

## 5.4 Policy switching

### What NOT to do

❌ Destroy the cache and recreate it. That causes cold cache, huge performance
disturbance, unfair evaluation, and artificial misses.

### Strategies

| | Strategy | Description |
|---|---|---|
| A | **Shared cache state** ⭐ | Keep objects in a common store; rebuild only policy metadata |
| B | Dual policy transition | Run new policy alongside old temporarily |
| C | Incremental migration | Gradually migrate metadata |
| D | Policy abstraction | Common object store + separate policy metadata |

**Choose A/D for the MVP.** Prioritise correctness and *measurable* switching
overhead over complex zero-downtime schemes. Keep the architecture open to B/C.

```
BEFORE SWITCH                          AFTER SWITCH

┌─────────────────┐                    ┌─────────────────┐
│  Object Store   │  ◄── UNCHANGED ──► │  Object Store   │
│  k1,k2,...,kN   │                    │  k1,k2,...,kN   │
└─────────────────┘                    └─────────────────┘
        ▲                                      ▲
┌───────┴─────────┐                    ┌───────┴─────────┐
│  LFU metadata   │  ── DISCARDED ──►  │  ARC metadata   │  ◄── Rebuild()
│  freq buckets   │                    │  T1,T2,B1,B2,p  │
└─────────────────┘                    └─────────────────┘

Zero objects evicted. Overhead = O(N) metadata rebuild. MEASURE IT.
```

### Switching safety — anti-oscillation

Without guards you get:

```
LRU → LFU → LRU → LFU → LRU ...   (catastrophic)
```

Implement **all** of:

- **Minimum policy residency** — no switch within N requests of the last one
- **Confidence threshold** — prediction confidence ≥ 0.80
- **Hysteresis** — require the new workload to persist for K windows
- **Performance improvement threshold** — minimum expected gain
- **Cooldown period** — after a failed switch, back off
- **Switch cost awareness**

```
switch only if:
    expectedGain > switchingCost + minGainThreshold
```

Every rejected switch is logged with `GuardsFailed`. This is data, not noise.

## 5.5 Heterogeneous objects & size-aware eviction

Object sizes range **1 byte → 1 MB**, with larger stress cases in evaluation.
A cache with fixed byte capacity must reason about the *value* of retaining each
object, not just its recency.

```
Object A: size 1 KB,   hit probability 0.1
Object B: size 100 KB, hit probability 0.9

A naive object-count cache makes poor decisions here.
```

### Scoring — modular, swappable, ablatable

```go
// LOWER score = evict first
type SizeAwareScorer interface { Score(e *Entry, now time.Time) float64 }
```

Implement several strategies so ablation is possible:

| Strategy | Score |
|---|---|
| `none` | policy order only (baseline) |
| `hitprob_per_byte` | `hitProbability / size` |
| `sqrt_normalised` | `policyScore(age, freq) / sqrt(size)` |
| `gdsf` | GreedyDual-Size Frequency |
| `cost_aware` | `(missCost × hitProb) / size` (see PART 12 #8) |

> The exact formula is an **experimental design choice**, not a claim. Report a
> comparison.

### Batch eviction

If inserting a 10 MB object requires freeing 5 MB, do not blindly evict one
object.

```go
EvictUntil(requiredBytes int64) (evicted []string, freed int64)
// keep evicting while freed < required
```

Not just `EvictOne()`. This is essential for variable-sized objects.

### Entry metadata

```go
type Entry struct {
    Key            string
    Value          Value
    Size           int64
    InsertionTime  time.Time
    LastAccessTime time.Time
    AccessCount    uint32
    Tier           TierID
    ExpiresAt      time.Time    // optional
    PolicyMeta     any          // policy-owned, opaque to the store
}
```

> Design this carefully — **metadata memory overhead is itself an evaluation
> metric** (target <5%). Every field you add costs you on that chart.

## 5.6 Online parameter tuning

The system must not only choose policies — it must tune them at runtime.

```
    evaluate configuration A  (N requests)
              ↓
    evaluate configuration B  (N requests)
              ↓
    compare hit rate / byte hit rate
              ↓
    retain better configuration
              ↓
    periodically repeat
```

Start simple and robust. **Avoid reinforcement learning.** The goal is online
optimisation, not AI complexity.

### Tunable parameters

| Policy | Parameter | Meaning |
|---|---|---|
| LFU | `decay_lambda` | frequency decay rate |
| ARC | `ghost_ratio` | ghost list size relative to c |
| W-TinyLFU | `window_ratio` | window LRU share (default ~0.01) |
| W-TinyLFU | `sketch_reset_interval` | count-min reset period |
| Adaptive | `confidence_threshold` | switch gate |
| Adaptive | `min_residency` | anti-oscillation |
| Eviction | `size_exponent` | in `score / size^k` |

> ❌ **Do not invent parameters that don't meaningfully affect the
> implementation.** Every tunable must have: allowed range, default, current
> value, update mechanism, measurement metric, and tuning interval.

Tuning interval default **5000** requests — configurable. Sweep 1000 / 5000 /
10000 for sensitivity analysis.

## 5.7 Multi-tier

```
        Request
           │
           ▼
        ┌──────┐  hit
        │  L1  │──────► return  (small, fast, node-local)
        └──┬───┘
           │ miss
           ▼
        ┌──────┐  hit
        │  L2  │──────► return + PROMOTE to L1  (larger, shared, adaptive)
        └──┬───┘
           │ miss
           ▼
        ┌──────┐  hit
        │  L3  │──────► return + promote        (large, remote — EXTENSION)
        └──┬───┘
           │ miss
           ▼
        Backend

  L1 eviction → DEMOTE to L2
  L2 eviction → DEMOTE to L3 / final deletion
```

**Scope discipline:**
- **MVP (P13):** L1 → L2 only.
- **Extension:** L1 → L2 → L3, only after the two-tier system is stable.

> ❌ Do NOT let distributed complexity prevent the core adaptive caching system
> from working. This is explicitly the project mitigation strategy.

> ⚠️ **Do not assume promotion always improves performance. Benchmark it.**

Distributed prototype needs: remote communication, serialisation, routing, tier
sync, failure handling, basic consistency, **simulated network latency**. A
production-grade consensus system is **not** required — a simple quorum is the
documented ceiling. Keep this component intentionally bounded.

---
---

# PART 6 — WORKLOADS, TRACES & REPRODUCIBILITY

## 6.1 Workload types

| Type | Pattern | Example | Hypothesised policy |
|---|---|---|---|
| **Temporal** | recent accesses reused | `A B C D A B C D A B` | LRU |
| **Spatial** | nearby keys accessed together | `100 101 102 103 104` | LRU |
| **Working-set** | stable active set repeated | `A B C D E / A B C D E` | LFU |
| **Random** | weak locality, ~uniform | `Q X B M A T P Z` | ARC |
| **Bursty** | sudden spike / hot subset | normal → spike → normal | evaluate live |
| **Mixed** | transitions between the above | — | **the whole point** |

The **mixed** workload is the most important — the entire premise of the
adaptive system is handling transitions.

## 6.2 The critical rule about demo data

> **The UI never generates data. The UI selects a `(source, seed)` and the
> backend produces the stream.**

If the UI generated the stream, a reviewer would immediately ask: *"How do we
know the workload isn't rigged to make your adaptive policy win?"*

With this rule:
- Same seed → same trace → same result, whether run from dashboard, CLI, or
  benchmark harness.
- The dashboard is not a special demo path — it is a **viewport onto the exact
  code that produced the paper's numbers.**
- You can claim: *"What you're watching live is byte-identical to run #47 in
  our results table."*

**Put the seed on screen. Add a replay button. Let the reviewer choose the
seed.**

## 6.3 Scenario files — one artifact, two uses

A declarative timeline the backend replays. Consumed by **both** the benchmark
harness and the live demo.

```yaml
# scenarios/showcase.yaml
name: "adaptation showcase"
seed: 42
capacity: 100MB
sizeDistribution: heterogeneous   # 1B .. 1MB, lognormal
segments:
  - {workload: temporal,    requests: 20000, uniqueKeys: 5000}
  - {workload: random,      requests: 20000, uniqueKeys: 50000}   # forces LRU→ARC
  - {workload: bursty,      requests: 15000, hotKeys: 20, spikeFactor: 12}
  - {workload: working_set, requests: 20000, setSize: 500}
  - {workload: temporal,    requests: 20000, uniqueKeys: 5000}    # does it switch back?
```

```yaml
# scenarios/adversarial_oscillation.yaml
# PURPOSE: demonstrate where adaptation FAILS (Success Condition #10)
name: "adversarial rapid oscillation"
seed: 7
capacity: 100MB
segments:
  - {workload: temporal, requests: 500}
  - {workload: random,   requests: 500}
  - {workload: temporal, requests: 500}
  - {workload: random,   requests: 500}
  # ... repeated: transitions faster than min_residency.
  # EXPECTED: adaptive UNDERPERFORMS plain LRU due to switching cost.
  # This is a RESULT, not a bug. Report it.
```

Each segment boundary emits a `ScenarioMarkEvent` — the **ground truth** used
for detection-delay measurement and the UI's "actual vs detected" markers.

## 6.4 Reproducibility — non-negotiable

Every synthetic benchmark supports a `seed`. `seed = 42` must produce an
identical workload every time.

Every experiment records a manifest:

```json
{
  "run_id": "run_047",
  "timestamp": "2026-08-25T14:03:11Z",
  "git_commit": "a3f9c21",
  "seed": 42,
  "scenario": "showcase.yaml",
  "capacity_bytes": 104857600,
  "policy": "adaptive",
  "config": { "...full resolved config..." },
  "feature_flags": { "adaptive": true, "tuning": false, "sizeAware": true },
  "go_version": "go1.22.3",
  "host": { "os": "linux", "arch": "amd64", "cpus": 8 }
}
```

Add **deterministic replay**: journal every request + decision; replaying the
journal must reproduce the run exactly. This makes bugs reproducible and results
defensible.

## 6.5 Real-world traces

Candidates: Facebook/MSR, Wikipedia/UMass, CloudPhysics.

> ❌ **DO NOT fabricate trace data.**
>
> If a dataset is unavailable:
> 1. Clearly document it.
> 2. Use an available substitute if academically appropriate.
> 3. **Label it as a substitute.**
> 4. Never pretend synthetic data is real-world data.

Verify availability and preprocessing format *before* implementation. Normalise
all traces to:

```go
TraceRecord { Key string; Timestamp time.Time; Size int64 }
```

so the same benchmark engine feeds LRU, LFU, ARC, W-TinyLFU, Clock and Adaptive
**the identical stream**. This is essential for fair comparison.

---
---

# PART 7 — EVALUATION

## 7.1 Metrics

### Cache
`totalRequests`, `hits`, `misses`, `hitRate = hits/total`, `missRate`,
`byteHitRate = bytesServed / bytesRequested`

### Latency
`mean`, `p50`, `p95`, **`p99`**. Tail latency matters because a miss can trigger
expensive downstream work.

### Memory — measure, don't estimate
Differentiate:
```
object payload memory
    vs metadata memory
    vs policy data-structure memory
    vs detector state
    vs metrics state
```
Use `runtime.ReadMemStats` + `unsafe.Sizeof` breakdowns. Document exactly how
overhead is computed. Target <5%, acceptable <10% — **must be measured.**

### Adaptation
`currentPolicy`, `policySwitches`, `switchTimestamps`, `switchOverhead`,
`detectionDelay`, `detectionAccuracy`, `policyResidenceTime`, `guardsFailed`

### Tuning
`parameterValues`, `tuningAttempts`, `configurationsEvaluated`,
`performancePerConfiguration`, `convergenceTime`

### Backend
`backendRequests`, `backendLoad`, `requestsAvoided`, `bytesFetched`

### Warmup separation
Report steady-state metrics separately from cold-start, so warmup doesn't
contaminate policy comparisons.

## 7.2 Evaluation matrix

```
              POLICIES  ×  WORKLOADS  ×  CACHE SIZES  ×  SIZE DISTS  ×  SEEDS

   LRU          temporal        0.1% of WSS      homogeneous       1..10
   LFU          spatial         1%               heterogeneous
   ARC          random          5%               heavy-tailed
   W-TinyLFU    working_set     10%
   Clock        bursty          25%
   ADAPTIVE     mixed
   ORACLE(MIN)  adversarial
```

## 7.3 Ablation studies — via config flags, not code branches

| Variant | Configuration | Isolates |
|---|---|---|
| **A** | fixed LRU | baseline |
| **B** | best fixed policy (hindsight) | strong baseline |
| **C** | adaptive selection, no tuning | value of selection |
| **D** | C + online tuning | value of tuning |
| **E** | C + size-aware eviction | value of size awareness |
| **F** | full system (C+D+E) | combined |
| **G** | F + multi-tier | value of tiering |
| **H** | Bélády MIN oracle | upper bound |

```yaml
features:
  adaptive:  true
  tuning:    false
  sizeAware: true
  tiers:     false
  shadow:    true
```

> **This is the single highest-leverage design choice in the project.** With
> feature flags, the ablation table falls out for free. Without them, you will
> be maintaining eight divergent code paths.

Without ablations you cannot tell whether improvement came from detection,
switching, tuning, size awareness, or tiering.

## 7.4 Statistics

Multiple runs (≥10 seeds). Report mean, median, stddev, p95, p99, confidence
intervals.

Optional, where assumptions hold: ANOVA, pairwise t-tests with Bonferroni
correction, Cohen's d.

> ❌ Do not blindly run statistical tests just to make the project look
> scientific. Justify each test's assumptions.

## 7.5 Required figures

| # | Figure | Purpose |
|---|---|---|
| 1 | Hit rate bar chart: policies × workloads | overall comparison |
| 2 | **Hit rate over time with transition markers** | ⭐ the money shot |
| 3 | Latency distribution box plots | tail behaviour |
| 4 | Policy selection timeline | what the engine did |
| 5 | Workload classification timeline | what it saw |
| 6 | Heatmap: workload × policy | where each policy wins |
| 7 | Detection delay: actual vs detected | RQ3 |
| 8 | Memory efficiency: hit rate / memory used | RQ4 |
| 9 | Byte hit rate vs object hit rate | RQ6 |
| 10 | Throughput vs thread count | concurrency |
| 11 | **Cumulative regret vs hindsight-optimal** | RQ2, strongest evidence |
| 12 | Ablation bar chart A–H | component contribution |

Figure 2 in detail:

```
 Hit Rate
   100% ┤
        │                          ╌╌╌╌╌╌╌╌╌ ORACLE (Bélády, upper bound)
    90% ┤                    ╭─────────────
        │                ╭───╯               ADAPTIVE
    80% ┤    ╭───────────╯
        │    │        ╭╮
    70% ┤────╯     ───╯╰────────────         LRU (shadow)
        │
    60% ┤              ╲╱
        │  TEMPORAL  │  RANDOM  │  BURSTY  │  WORKING_SET
        └────────────┴──────────┴──────────┴──────────────► requests
                     ▲          ▲
          ground-truth transition│
                      └─ detected (Δ = 7 requests)
```

## 7.6 Explainability

Every decision must be human-readable:

```
Previous policy : LRU
New policy      : LFU
Workload        : WORKING_SET
Confidence      : 0.91
Hit rate        : 72%
Reason          : stable active set + high frequency concentration
Guards passed   : confidence, residency, cooldown, expected_gain
Switch overhead : 8.2 ms
```

Exposed at `GET /api/explain`. Used for debugging, research analysis, the demo,
and the final presentation.

---
---

# PART 8 — PHASE PLAN

> This is the implementation roadmap. **Find the current phase, work only within
> it.** Each phase is shippable, testable, and resumable by a different AI agent.

## 8.1 Phase table

| Phase | Build | Ends with (demo-able) | Est. |
|---|---|---|---|
| **P1 — Skeleton** | `types/`, `Cache` + `EvictionPolicy` interfaces, byte-capacity object store, **event bus**, **frame aggregator**, metrics collector, YAML config + feature flags, Makefile, CI, arch-lint | `go test ./...` green; no-op cache counting hits/misses; frames dumped to stdout | 1 wk |
| **P2 — Baselines I** | LRU, LFU (freq buckets + decay), Clock. Table-driven tests vs known eviction sequences | `bench --policy=lru --trace=x.csv` prints hit rate | 1 wk |
| **P3 — Traces** | `TraceSource` interface, all 6 synthetic generators (seeded), CSV loader, **scenario YAML replay + ScenarioMarkEvent**, benchmark runner, CSV/JSON output | Same trace replayed across 3 policies → results table. **The demo's data layer is complete before any UI exists.** | 1 wk |
| **P4 — Baselines II + Shadows** | ARC (real T1/T2/B1/B2), W-TinyLFU (count-min + doorkeeper + window), **shadow caches + Bélády oracle** | 5 baselines × 6 workloads matrix + oracle upper bound | 1.5 wk |
| **P5 — Features** | Feature extractor: reuse distance, entropy, CV, working-set estimate, size stats. Each independently unit-tested | `analyze trace.csv` prints a feature vector | 1 wk |
| **P6 — Detector** | Rule-based classifier, confidence, dual windowing. Validate: generated temporal ⇒ classified temporal. **Detection-delay measurement vs ScenarioMark** | Classification timeline + measured detection delay → **RQ3** | 1 wk |
| **P7 — Adaptive engine** ⭐ | Selector (config-driven map), state-preserving switcher, all guards (hysteresis/residency/cooldown/gain), decision logging incl. `GuardsFailed` | **First real result: adaptive vs fixed on mixed workload → RQ1, RQ2** | 2 wk |
| **P8 — Observability** | Event-bus subscribers → **TUI dashboard** (bubbletea) + **report generator** (matplotlib) | The TUI screen; all PART 7.5 figures generated from JSON | 1 wk |
| **P9 — Size-aware** | `SizeAwareScorer` strategies, `EvictUntil`, byte hit rate, heterogeneous size distributions | Ablation E: size-aware vs naive → **RQ6** | 1 wk |
| **P10 — Tuning** | Parameter registry, A/B sweep tuner, convergence tracking | Ablation D: adaptive+tuning vs adaptive → **RQ7** | 1 wk |
| **P11 — Evaluation** ⭐ | Multi-run harness, mean/CI/p95/p99, ablations A–H, real traces (or documented substitutes), regret analysis, optional learned selector | **`docs/RESULTS.md` with real numbers** | 2 wk |
| **P12 — Concurrency + profiling** | Locking strategy, sharding, pprof, thread-scaling benchmarks, full edge-case suite, 80% coverage | Throughput-vs-threads chart → **RQ4, RQ5** | 1.5 wk |
| **P13 — Multi-tier** | L1→L2, promotion/demotion, simulated network latency, tier metrics | **RQ8** answered | 1.5 wk |
| **P14 — Web UI + docs** | WebSocket dashboard, `go:embed` SPA, `/control`, inject buttons, Docker, README, diagrams, report, slides | **Final demo** | 2 wk |

## 8.2 Dependency graph

```
P1 ──┬─► P2 ──┬─► P3 ──┬─► P4 ──┬─► P5 ──► P6 ──► P7 ──┬─► P8
     │        │        │        │                      │
     │        │        │        └── shadows ───────────┤
     │        │        │                               │
     │        │        └── scenarios ──────────────────┤
     │        │                                        │
     │        └────────────────────────────────────────┤
     │                                                 │
     └── event bus (everything depends on this) ───────┤
                                                       │
                              ┌────────────────────────┤
                              ▼                        ▼
                         P9 (size)              P10 (tuning)
                              │                        │
                              └────────┬───────────────┘
                                       ▼
                                  P11 (evaluation)  ⭐ THE GRADE
                                       │
                              ┌────────┼────────┐
                              ▼        ▼        ▼
                            P12      P13      P14
                        (concur)   (tiers)  (web UI)
```

**Critical path to a defensible project: P1 → P7 → P11.** Everything else is
enhancement. If time runs out, P12/P13/P14 are the ones to cut — never P11.

## 8.3 Rules that keep this AI-resumable

1. **One package per phase**, behind an interface defined in P1. New phases
   *add implementations*, never rewrite earlier ones.
2. **Every phase ends with:** tests green, `CHANGELOG.md` updated,
   `docs/DECISIONS.md` records *why*, and a runnable command demonstrating it.
3. **Feature flags** in config (`adaptive.enabled`, `sizeAware.enabled`,
   `tuning.enabled`, `tiers.enabled`, `shadow.enabled`). Ablations = config
   permutations, never code branches.
4. **Never modify a completed phase's public interface** without recording an
   ADR in `docs/DECISIONS.md` and updating all dependents in the same commit.
5. **The event bus lands in P1.** Retrofitting it in P8 is painful; adding it in
   P1 is ~100 lines.

---
---

# PART 9 — ACADEMIC INTEGRITY & TARGETS

## 9.1 The targets are HYPOTHESES, not results

| Metric | Target | Acceptable | Status |
|---|---|---|---|
| Hit rate vs LRU | +12% | +8% | ⬜ **NOT MEASURED** |
| p99 latency vs LRU | −25% | −15% | ⬜ **NOT MEASURED** |
| Policy switch time | <100 ms | <200 ms | ⬜ **NOT MEASURED** |
| Memory overhead | <5% | <10% | ⬜ **NOT MEASURED** |
| Detection delay | 1–5 req | 1–10 req | ⬜ **NOT MEASURED** |
| Distribution cost | <10% | <15% | ⬜ **NOT MEASURED** |

After each phase, an AI agent may **only** change ⬜ to ✅ with a citation to a
specific run ID in `results/`.

## 9.2 Prohibited

> ❌ At no point may implementation or documentation claim
> *"Our system improves hit rate by 12%"* unless an actual experiment produced
> that result.

Never manufacture:
- benchmark numbers
- detection accuracy
- latency figures
- memory usage
- statistical significance
- percentage improvements
- dataset contents
- citations to papers not actually read

## 9.3 Required honesty

- Do not claim an algorithm is novel when it isn't.
- Do not claim production readiness without evidence.
- Do not claim "lock-free" unless the implementation actually is lock-free.
- Distinguish implementation from research hypothesis.
- Distinguish expected results from measured results.
- If something cannot be implemented faithfully, **say so**.
- If a simplification is made, **document it in `docs/LIMITATIONS.md`**.

## 9.4 Design priority under conflict

```
1. Correctness
2. Reproducibility
3. Benchmarkability
4. Clear architecture
5. Core adaptive mechanism
6. Performance
7. Distributed extension
8. Extra features
```

The central research contribution outranks building a large distributed system.

---
---

# PART 10 — WORKING AGREEMENT WITH AI AGENTS

## 10.1 Role

You are not merely a code generator. Act as a combination of:
systems architect · Go engineer · cache systems researcher · benchmarking
engineer · testing engineer · academic project advisor.

## 10.2 Per-feature protocol

When asked to implement something, respond in this order:

1. Where it fits in the architecture (reference PART 3)
2. The relevant interfaces (reference PART 4)
3. Dependencies (what must exist first)
4. Implementation
5. Tests
6. How to run the tests
7. How to benchmark it
8. Assumptions made
9. Limitations
10. What should be implemented next

## 10.3 Per-feature workflow

```
REQUIREMENTS → ARCHITECTURE → INTERFACE DESIGN → IMPLEMENTATION
    → UNIT TESTS → INTEGRATION TEST → BENCHMARK → VALIDATION
    → DOCUMENTATION → NEXT FEATURE
```

Do not jump from *"build adaptive cache"* to thousands of lines of code.

## 10.4 Coding principles

**Use:** idiomatic Go · clear package boundaries · interfaces where useful ·
focused structs · dependency injection · deterministic tests · configurable
parameters · proper error handling · concurrency safety where needed ·
benchmarkable components.

**Avoid:** giant files · giant structs · global mutable state · hard-coded
benchmark values · duplicated policy logic · hidden dependencies · premature
optimisation · unnecessary frameworks.

## 10.5 Testing strategy

### Unit tests (required for every component)
cache · LRU · LFU · ARC · W-TinyLFU · Clock · detector · feature extractor ·
policy selector · guards · switcher · tuner · size-aware eviction · batch
evictor · metrics · event bus · trace generators · scenario parser

### Integration scenarios
1. Temporal workload → LRU selected
2. Working-set workload → LFU selected
3. Transition temporal → random → policy eventually changes
4. Large object insertion → batch eviction correct
5. L2 hit → L1 promotion
6. Rapid oscillation → guards prevent thrashing
7. Scenario replay → detection delay computed correctly
8. Same seed twice → byte-identical results

### Edge cases (explicit tests)
empty cache · capacity = 1 · cache smaller than object · object size 0 ·
duplicate insertion · repeated access · unknown workload · low classifier
confidence · rapid workload oscillation · extreme burst · all keys unique ·
one extremely hot key · very large object · very small object · concurrent
requests · policy switching under high load · tuning during transition · tier
unavailable · network delay · serialisation failure · event-bus subscriber
stalls (must drop, not block)

### Regression benchmarks in CI
Fail the build if hit rate on a fixed trace+seed drops more than 2%. Catches
silent policy bugs.

Coverage target: **80%+** by P12.

## 10.6 Concurrency discipline

Establish correctness **single-threaded first**. Then add mutexes, sharded maps,
atomic counters, and background goroutines for tuning/detection **only where
justified**.

Benchmark at 1 / 4 / 16 / 32 / 64 / 100+ threads. Measure throughput, latency,
contention, correctness.

## 10.7 Optimisation discipline

```
correctness → testing → benchmarking → profiling → optimisation
```

Use pprof and Go benchmarks to find **actual** hotspots. Do not optimise
prematurely.

---
---

# PART 11 — OBSERVABILITY, UI & DEMO

## 11.1 Position

The research contribution is **100% backend**. The UI is optional but
strategically valuable, because the demo requirement ("the feedback loop should
be obvious") is exactly what a UI does best, and PART 7.5 already requires
hit-rate-over-time charts, policy timelines, and classification timelines.

> **Backend = the project. UI = an observability layer over the backend's
> existing metrics/event stream.**

The mitigation for "don't spend significant time on a GUI" is:
**never build a UI the cache depends on.** Build one that only *reads* what the
cache already emits. That makes it ~4–5 days instead of 3 weeks — *because*
P1/P3/P8 already built everything underneath.

## 11.2 Three frontends, one backend

| Option | Tech | Phase | Purpose |
|---|---|---|---|
| **A — TUI** ⭐ | bubbletea / termui | P8 | Single binary, no JS, no internet. **Insurance policy for demo day.** |
| **B — Web** | `go:embed` SPA + uPlot | P14 | Interactive, impressive, injectable |
| **C — Report** | Python + matplotlib | P8 | **Do this regardless.** It's what goes in the report. |

## 11.3 TUI layout (P8)

```
┌─ ADAPTIVE CACHE ────────────────────────── req 24,381 ─┐
│ Workload:  BURSTY          confidence 0.91             │
│ Policy:    ARC             residency 4,102 req         │
│ Hit rate:  78.3%           byte hit 71.2%              │
│ Capacity:  81.4 MB / 100 MB        objects 3,204       │
├─ HIT RATE (last 20k) ──────────────────────────────────┤
│ 90│              ╭──╮        ╭───── adaptive           │
│ 80│     ╭────────╯  ╰────────╯                         │
│ 70│─────╯    ╭╮         ╭──────────── LRU shadow       │
│ 60│          ╰╯─────────╯                              │
├─ POLICY TIMELINE ──────────────────────────────────────┤
│ LRU ████████ LFU ██████ ARC ███████████                │
│         ↑transition        ↑detected (delay: 7)        │
├─ LAST DECISION ────────────────────────────────────────┤
│ LFU → ARC  @ 20,279                                    │
│ workload TEMPORAL→RANDOM, conf 0.91                    │
│ reason: low reuse + high key entropy                   │
│ switch overhead: 12.4 ms                               │
└────────────────────────────────────────────────────────┘
```

That single screen sells the entire thesis in 10 seconds.

## 11.4 Web dashboard layout (P14)

```
┌──────────────────────────────────────────────────────────────────┐
│ ▶ Pause  ⟲ Replay   seed 42   speed 10×   scenario: showcase ▾   │
│ 100MB │ conf-threshold 0.80 │ min-residency 2000                 │
├──────────────────────────────────────────────────────────────────┤
│  WORKLOAD      POLICY       HIT RATE     BYTE HR      p99        │
│  BURSTY        ARC          78.3% ▲      71.2%        1.9ms      │
│  conf 0.91     4,102 req    (LRU 71.0%)               (LRU 3.1ms)│
├──────────────────────────────────────────────────────────────────┤
│ HIT RATE OVER TIME                                               │
│ 90│                    ╭─────── adaptive                         │
│ 80│      ╭─────╮   ╭───╯   ╌╌╌╌ oracle (Bélády)                 │
│ 70│──────╯     ╰───╯       ──── LRU shadow                       │
│ 60│  ┊TEMPORAL ┊ RANDOM ┊ BURSTY ┊ WORKING_SET ┊                │
│   └──┊─────────┊────────┊────────┊─────────────┊──────► requests │
│      ▲ ground-truth transition   ▲ detected (Δ=7)                │
├──────────────────────────────────────────────────────────────────┤
│ POLICY TIMELINE                                                  │
│ ████LRU████│██LFU██│███████ARC███████│████LRU████                │
├────────────────────────────┬─────────────────────────────────────┤
│ FEATURE RADAR              │ ⚡ LFU → ARC   @ req 20,279         │
│   reuse-dist  ▁▃▅          │   workload TEMPORAL → RANDOM        │
│   entropy     ▇▇▇          │   confidence 0.91                   │
│   burstiness  ▂            │   hit rate 71% → 78%                │
│   ws-size     ▅▅           │   reason: low reuse + high entropy  │
│   size-var    ▃            │   switch overhead 12.4 ms           │
├────────────────────────────┴─────────────────────────────────────┤
│ INJECT:  [burst] [hot key] [scan] [→random] [huge object 10MB]   │
└──────────────────────────────────────────────────────────────────┘
```

### Why each element earns its place

- **Shadow lines are the whole argument.** "Adaptive: 78%" is a number.
  "Adaptive *above LRU* and *approaching oracle*" is a **result**. This is the
  single most important thing on screen.
- **Ground-truth transition markers.** The backend generated the workload, so it
  *knows* when the transition happened. Drawing "actual" vs "detected" makes RQ3
  visually self-evident — you are *showing* detection delay, not asserting it.
- **The decision card** is explainability rendered. It makes the system legible
  rather than magic.
- **Inject buttons** are the credibility move. Hand the reviewer the mouse.
- **Seed + replay** kills the "your demo is rigged" objection before it's raised.

## 11.5 Control API

```
POST /control
  {"action":"start",    "scenario":"showcase.yaml", "seed":42}
  {"action":"pause"} | {"action":"resume"}
  {"action":"speed",    "value":10}        // 1 = realtime, 0 = max
  {"action":"replay"}
  {"action":"inject",   "kind":"burst",      "requests":5000}
  {"action":"inject",   "kind":"hot_key",    "key":"k_hot"}
  {"action":"inject",   "kind":"scan",       "range":10000}
  {"action":"inject",   "kind":"huge_object","size":10485760}
  {"action":"setflag",  "flag":"tuning", "value":true}
```

Speed control matters — the demo must fit the time slot.

## 11.6 Demo script (~4 minutes)

| # | Time | Action | Point being made |
|---|---|---|---|
| 1 | 0:30 | Start on temporal. "LRU, 87% hit rate. Nothing interesting yet." | establish baseline |
| 2 | 0:45 | Transition to random. LRU shadow **collapses**, detector fires, ARC takes over, adaptive line separates. Point at the Δ=7 marker. | ⭐ **the money shot** — RQ1, RQ2, RQ3 in one visual |
| 3 | 0:30 | Bursty segment. Different policy, and the *reason string changes*. | explainability |
| 4 | 1:00 | **Hand over the mouse.** "Click any inject button." | credibility — unrehearsed |
| 5 | 0:30 | Inject the 10 MB object. Batch eviction; byte-hit-rate diverges from object-hit-rate. | RQ6 |
| 6 | 0:30 | Load `adversarial_oscillation.yaml`. Adaptation **loses** to plain LRU because switching cost dominates. Say so out loud. | Success Condition #10 — lands harder than any win |
| 7 | 0:20 | Replay with the same seed → identical curve. "Every number in our report came from this pipeline." | reproducibility |

> **Record the demo. Venue demos fail.**

## 11.7 Two cautions

**1. Do not let the UI leak into the cache.**
If `cache/` ever imports `ui/`, or a policy ever knows a dashboard exists, you
have broken the thing that keeps this cheap. The event bus must be
fire-and-forget with a bounded, **droppable** channel — a slow browser must never
backpressure the cache and corrupt latency measurements. **Drop frames, never
block.**

**2. Do not let the UI eat Phase P11.**
Evaluation (real numbers, ablations, statistics) is what the project is graded
on. A beautiful dashboard over thin results is a *worse* project than an ugly
CLI over rigorous ones. Keep the TUI as insurance; build the web UI only once
`docs/RESULTS.md` has real content.

---
---

# PART 12 — FEATURE BACKLOG

Ranked by (research value ÷ cost).

## Tier 1 — high value, low cost. Do these.

**1. Shadow / counterfactual caches** ⭐⭐⭐ *(P4)*
Run LRU, LFU, ARC as **metadata-only simulators** (no payloads, near-zero cost)
alongside the live cache. Enables: *"adaptive achieved 78%; the best fixed
policy in hindsight achieved 76%."* This is an **oracle bound** and the
strongest possible evidence for RQ2. It also lets the selector use *observed*
rather than *assumed* policy fitness.

**2. Bélády MIN oracle** ⭐⭐ *(P4)*
Offline optimal on a known trace. Gives an upper bound on every chart.
Reviewers love it. Cheap on offline traces.

**3. Deterministic replay / event journal** ⭐⭐ *(P3)*
Record every request + decision; replay reproduces a run exactly. Makes bugs
reproducible and results defensible.

**4. Explainability API** ⭐⭐ *(P7)*
`GET /explain` → last N decisions with feature vectors. Serves PART 7.6 and
feeds the UI decision card directly.

**5. Regression benchmarks in CI** ⭐⭐ *(P3)*
Fail the build if hit rate on a fixed trace drops >2%. Catches silent policy
bugs that would otherwise poison every downstream result.

## Tier 2 — genuinely strengthens the research claim

**6. Learned selector trained on shadow-cache outcomes** ⭐⭐⭐ *(P11)*
The rule-based classifier maps workload→policy *by hypothesis*. Instead: log
`(features, actual_best_policy_from_shadows)`, fit a small decision tree
offline, deploy it. Now the mapping is **empirically derived, not asserted.**
Directly addresses PART 5.3's caveat and is a legitimate contribution.

**7. Regret analysis vs a bandit baseline** ⭐⭐⭐ *(P11)*
Report cumulative regret vs the hindsight-optimal policy. Framed this way, the
adaptive engine is an **online policy-selection / expert-advice problem** — and
you can compare against ε-greedy or EXP3 over policies. Gives theoretical
grounding and a strong related-work position.

**8. Cost-aware / TTL-aware eviction** ⭐⭐ *(P9)*
Extend `size` to `(size, missCost, ttl)`. A 1 KB object costing a 200 ms backend
fetch is worth more than a 1 KB object costing 2 ms. Generalises the
benefit/cost metric naturally and is realistic.

**9. Adaptive admission control** ⭐⭐ *(P9)*
Not just eviction — `ShouldAdmit(key, size, features)` as a first-class adaptive
decision. TinyLFU already does admission; making it adaptive is a distinct axis
most student projects miss.

**10. Adversarial workloads** ⭐⭐⭐ *(P6/P11)*
Deliberately construct traces that defeat the detector: oscillation faster than
min-residency, workloads sitting exactly on classifier decision boundaries.
Reporting *where adaptation fails* is Success Condition #10 — **this is what
separates a good project from a great one.**

## Tier 3 — nice to have

11. **Warmup / steady-state separation** in metrics (avoid cold-start
    contaminating comparisons)
12. **Prometheus `/metrics` endpoint** — ~20 lines, and the dashboard story
    writes itself
13. **`cache-cli` REPL** — `get k`, `put k 4096`, `inject bursty 5000`, `stats`,
    `explain`
14. **Memory accounting** via `runtime.ReadMemStats` + `unsafe.Sizeof`
    per-component breakdown (PART 7.1 demands this be *measured*)
15. **Sharded cache** for concurrency scaling (P12)
16. **Trace compression / sampling** for very large real traces

---
---

# PART 13 — SCOPE BOUNDARIES

## 13.1 Explicitly NOT building

- ❌ Production GUI / heavyweight dashboard framework
- ❌ Kubernetes integration
- ❌ GPU acceleration
- ❌ Persistence / durability / WAL
- ❌ Complex networking infrastructure
- ❌ Advanced deep-learning models
- ❌ Production-grade distributed consensus (Raft/Paxos)
- ❌ Unnecessary frontend work (auth, routing, state libraries, SSR)

The project is primarily about **adaptive cache replacement and workload-aware
decision making.**

## 13.2 Deliberately bounded

- Distributed cache → simple quorum at most; simulated network latency
- L3 tier → extension only, after L1→L2 is stable
- Web UI → one WebSocket, one reducer, no framework required
- Statistical tests → only where assumptions hold

## 13.3 The MVP definition

### Primary MVP (proves the central research idea)
```
single-node cache + LRU + LFU + ARC
  + workload generators + feature extraction + workload detector
  + adaptive policy selection + metrics + benchmark framework
```

### Secondary MVP (strong comparative system)
```
+ online tuning + size-aware eviction + W-TinyLFU + Clock
```

### Advanced extension
```
+ L1→L2 distributed cache   (then L1→L2→L3 if time allows)
```

> ❌ Do not allow distributed caching to jeopardise the core research
> contribution.

---
---

# PART 14 — PROJECT PHASES (ACADEMIC) & CURRENT STATUS

## 14.1 Academic phase timeline

| Phase | Weeks | Focus | Deliverable | Status |
|---|---|---|---|---|
| **0 — Foundation** | 1–2 | Literature review, existing policies/systems, research gap | Literature Review | ✅ **COMPLETE** |
| **1 — Problem Definition** | 3–4 | Problem statement, justification, industry relevance, feasibility | Problem Justification Document | ✅ **COMPLETE** |
| **2 — Objectives & Scope** | 5–6 | Objectives, scope, metrics, success criteria | Objectives & Scope Document | ✅ **COMPLETE** |
| **3 — Methodology & Design** | 7–10 | Architecture, interfaces, algorithms, data structures, detector, adaptive engine, heterogeneous objects, multi-tier, evaluation methodology, repo structure | **Technical Design Document** | 🔵 **CURRENT** |
| **4 — Implementation** | 11–16 | Sprint 1: core + LRU/LFU/ARC + traces + metrics. Sprint 2: detector + selector + tuning + switching. Sprint 3: 2-tier + heterogeneous + promotion/demotion | Working code repository | ⬜ Upcoming |
| **5 — Evaluation** | 17–19 | Baselines, synthetic + real traces, metrics, statistics, ablations | CSV/JSON results, graphs, tables, analysis | ⬜ Upcoming |
| **6 — Optimisation** | 20–21 | Profiling, concurrency, memory, edge cases, 80%+ coverage | Optimised system + test suite | ⬜ Upcoming |
| **7 — Documentation** | 22–24 | Final report, API docs, architecture diagrams, benchmark docs, README, presentation, demo | Documentation set | ⬜ Upcoming |
| **8 — Final Submission** | 24–26 | Code (repo, binary, Dockerfile, tests, scripts), documentation, presentation (30+ slides, demo video, poster) | Final deliverables | ⬜ Upcoming |

## 14.2 Mapping: academic phases → build phases

```
Academic Phase 3  →  design docs (this document + docs/ARCHITECTURE.md)
Academic Phase 4  →  Build P1 – P10
Academic Phase 5  →  Build P11
Academic Phase 6  →  Build P12
Academic Phase 7  →  Build P13, P14
Academic Phase 8  →  packaging, slides, poster, recorded demo
```

## 14.3 Current status

```
Academic:   Phase 0 ✅   Phase 1 ✅   Phase 2 ✅   Phase 3 🔵 CURRENT
Build:      P1 ⬜ (not started)
```

## 14.4 Immediate next objective

Complete the **Phase 3 Technical Design Document** covering:

1. Final architecture (component diagram, module responsibilities, data flow,
   control flow) — *see PART 3*
2. Interfaces: Cache, EvictionPolicy, WorkloadDetector, FeatureExtractor,
   PolicySelector, ParameterTuner, MetricsCollector, TraceSource, Tier,
   **EventBus** — *see PART 4*
3. Data structures: entry, request, features, prediction, decision, metrics,
   benchmark result, **Frame** — *see PART 4*
4. How LRU/LFU/ARC/W-TinyLFU/Clock fit the common abstraction — *PART 5.1*
5. Adaptive engine: detector → selector → tuner → cache — *PART 5.3*
6. Switching strategy: how cache state survives policy changes — *PART 5.4*
7. Workload classifier: features, windowing, classification logic — *PART 5.2*
8. Size-aware eviction: scoring abstraction — *PART 5.5*
9. Benchmark architecture: identical traces replayed against every policy —
   *PART 6, PART 7.2*
10. Testing architecture: unit, integration, benchmark, edge — *PART 10.5*
11. Final Go project tree — *PART 3.7*
12. Dependency-aware implementation roadmap — *PART 8*
13. **Observability boundary: event bus → metrics/log/frames/UI** — *PART 3.4, PART 11*

Then begin **Build P1**.

---
---

# APPENDIX A — QUICK REFERENCE FOR AI AGENTS

## A.1 "I've been handed this project. What do I do?"

1. Read PART 0, PART 3, PART 8.
2. Check PART 14.3 for the current status.
3. Read `CHANGELOG.md` and `docs/DECISIONS.md` in the repo for what actually
   happened (this document describes intent; those describe reality).
4. Locate the current build phase in PART 8.1.
5. Work only within that phase's scope.
6. Follow the per-feature protocol in PART 10.2.

## A.2 Things that will get the work rejected

- Fabricating any number (PART 9)
- `cache/` importing `ui/`, `server/`, or `benchmark/` (PART 3.6)
- A blocking event bus (PART 3.4)
- Hard-coding 1000 as a window size (PART 5.2)
- Calling a recency+frequency score "ARC" (PART 5.1)
- Ablations as `if` branches instead of config flags (PART 7.3)
- Building the web UI before `docs/RESULTS.md` has real content (PART 11.7)
- Destroying and recreating the cache on policy switch (PART 5.4)
- Starting with a deep-learning classifier (PART 5.2)
- Claiming "lock-free" without a lock-free implementation (PART 9.3)

## A.3 The one-line summary to keep in your head

> A cache that watches its own workload, figures out what kind of workload it is,
> picks and tunes the best-suited eviction policy for *right now*, accounts for
> object sizes, and proves — with reproducible measurements, including the cases
> where it loses — whether that adaptation was worth the overhead.

# END OF PROJECT CONTEXT