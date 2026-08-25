# PROJECT STATE — Adaptive Cache
**Living document. Update at the end of EVERY phase. This is the handoff file.**

> **AI AGENT: START HERE.**
> 1. Read this file completely (5 min).
> 2. Read `context.md` PART 0, PART 3, PART 4 (the frozen interfaces).
> 3. Find the phase marked 🔵 IN PROGRESS in §3 below.
> 4. Open that phase's card in §4. It lists exactly what to create and touch.
> 5. Work ONLY within that phase. Do not skip ahead.
> 6. When done, run §6 exit checklist and update this file.

---

## §1 — SNAPSHOT

```
LAST UPDATED : 2026-08-26
UPDATED BY   : gptnmn
GIT TAG      : p01-complete
BUILD PHASE  : P2  🔵 IN PROGRESS
ACADEMIC     : Phase 3 (Methodology & Design)
NEXT REVIEW  : R1 — Design
BLOCKERS     : none
```

**Where we are in one sentence:**
> P1 is complete. The frozen interfaces (`Cache`, `EvictionPolicy`, `Event`,
> `Frame`, `Request`, `Entry`) and a non-blocking, bounded, droppable event bus
> are in place, along with config, metrics, the byte-capacity object store and a
> nil-safe cache core. `go build`, `go vet`, `go test` and
> `scripts/lint-arch.sh` pass; `go test -race` passes in CI on Linux.
> `./adaptive-cache --config configs/default.yaml --duration 5s` prints `Frame`
> JSON at 10 Hz.

**Next:** P2 — baseline policies (LRU, LFU, Clock) behind the frozen
`EvictionPolicy` interface, plus a policy registry.

---

## §2 — THE 60-SECOND BRIEFING

Building a cache that watches its own workload, classifies it, picks and tunes
the best-suited eviction policy for *right now*, accounts for object sizes, and
proves — with reproducible measurements, **including cases where it loses** —
whether adaptation is worth the overhead.

- **Language:** Go. **Config:** YAML. **Results:** CSV/JSON.
- **Not novel:** LRU/LFU/ARC/W-TinyLFU/Clock. **Our contribution:** the
  adaptive selection + tuning + size-awareness + honest evaluation *framework*.
- **Never say** "we invented a new eviction algorithm."
- **Never fabricate a number.** All targets in `context.md` PART 9 are
  hypotheses until a run ID backs them.

---

## §3 — PHASE LEDGER

Legend: ⬜ not started · 🔵 in progress · ✅ complete · ⏭️ skipped

| # | Phase | Status | Tag | New | Touch | Est | Review |
|---|---|---|---|---|---|---|---|
| P1 | Skeleton & Event Bus | ✅ | p01-complete | 14 | 0 | 1w | R2 |
| P2 | Baselines I (LRU/LFU/Clock) | ⬜ | — | 8 | 2 | 1w | R2 |
| P3 | Traces & Benchmark | ⬜ | — | 14 | 2 | 1w | R2 |
| P3.5 | Early TUI (policy race) | ⬜ | — | 6 | 2 | 2d | R2 |
| P4 | ARC, W-TinyLFU, Shadows | ⬜ | — | 11 | 2 | 2w | R2 |
| P5 | Feature Extraction | ⬜ | — | 8 | 1 | 1w | R3 |
| P6 | Workload Detector | ⬜ | — | 6 | 2 | 1w | R3 |
| P6.5 | TUI increment (detection) | ⬜ | — | 0 | 2 | 1d | R3 |
| P7 | **Adaptive Engine** ⭐ | ⬜ | — | 9 | 3 | 2w | R3 |
| P8 | Observability | ⬜ | — | 3 | 4 | 3d | R4 |
| P9 | Size-Aware Eviction | ⬜ | — | 7 | 3 | 1w | R4 |
| P10 | Online Tuning | ⬜ | — | 6 | 2 | 1w | R4 |
| P11 | **Evaluation** ⭐ | ⬜ | — | 9 | 1 | 2w | R4 |
| P12 | Concurrency & Profiling | ⬜ | — | 6 | 4 | 1.5w | R4 |
| P13 | Multi-Tier L1→L2 | ⬜ | — | 8 | 2 | 1.5w | R5 |
| P14 | Web UI & Docs | ⬜ | — | 10 | 2 | 2w | R5 |

**Totals:** 16 phases (P1–P14 + P3.5 + P6.5), ~124 new files (incl. tests),
~19 weeks. Net schedule change vs prior single-pass P8 TUI: **~+1 day**.
**Critical path:** P1 → P2 → P3 → P4 → P5 → P6 → **P7** → **P11**.
**Cut order if short on time:** P13 → P14-web → P10 → learned selector.
**Never cut:** P11.

---

## §4 — PHASE CARDS

Each card is a self-contained work order. Give the card + `context.md` to any AI.

---

### ✅ P1 — SKELETON & EVENT BUS

**Goal:** Frozen interfaces + working event bus. No caching logic yet.
**Why first:** Every later phase implements these interfaces. Retrofitting the
event bus later is painful; adding it now is ~100 lines.

**CREATE**
```
go.mod  Makefile  .github/workflows/ci.yml  .gitignore
types/request.go        Request, OpType
types/entry.go          Entry (+ Value)
types/workload.go       WorkloadType enum, Features, WorkloadPrediction
types/policy.go         PolicyName, ParamSet, Parameter
events/bus.go           Bus iface + impl (BOUNDED, DROPPABLE, NON-BLOCKING)
events/types.go         Event, SwitchEvent, DetectionEvent, TuningEvent,
                        ScenarioMarkEvent, Frame
events/bus_test.go      MUST test: slow subscriber drops, never blocks
config/config.go        YAML load
config/flags.go         FeatureFlags{Adaptive,Tuning,SizeAware,Tiers,Shadow}
config/validate.go
configs/default.yaml
metrics/collector.go    MetricsCollector iface + impl
metrics/latency.go      histogram, p50/p95/p99
metrics/memory.go       ReadMemStats + Sizeof breakdown
cache/cache.go          Cache interface        ← FREEZE (context.md §4.1)
cache/store.go          byte-capacity object store
cache/policy/policy.go  EvictionPolicy iface   ← FREEZE (context.md §4.2)
cache/core.go           orchestration; policy = pluggable, nil-safe
cmd/adaptive-cache/main.go   dumps Frames to stdout
scripts/lint-arch.sh    forbid cache/ importing ui|server|benchmark|adaptive
```

**TOUCH:** none.

**DONE WHEN**
- `go test ./...` green; `make lint-arch` green
- `./adaptive-cache --config configs/default.yaml` prints Frames at 10 Hz
- Bus test proves a stalled subscriber causes **drops, not blocking**

**FREEZE AFTER THIS PHASE:** `Cache`, `EvictionPolicy`, `Event`, `Frame`,
`Request`, `Entry`. Changing these later requires an ADR in `docs/DECISIONS.md`.

**TRAPS:** blocking `Publish()` (fatal — corrupts latency measurements);
capacity in objects instead of **bytes**; policy owning objects instead of
metadata only.

---

### ⬜ P2 — BASELINES I

**Goal:** LRU, LFU, Clock behind the frozen interface.

**CREATE**
```
cache/policy/lru.go        list + map
cache/policy/lfu.go        freq buckets O(1) + decay_lambda param
cache/policy/clock.go      circular buffer + ref bits
cache/policy/registry.go   name → constructor  ← every later policy registers here
cache/policy/lru_test.go   table-driven vs known eviction sequences
cache/policy/lfu_test.go   incl. cold-start + decay
cache/policy/clock_test.go
tests/edge/edge_test.go    cap=1, size=0, obj>cap, dup insert, empty
```
**TOUCH:** `cache/core.go` (wire registry), `cmd/.../main.go` (`--policy` flag)

**DONE WHEN:** `./bench --policy=lru --trace=x.csv` prints a hit rate;
eviction order matches hand-computed sequences in tests.

**TRAPS:** LFU cold-start (new items freq=1, evicted instantly — mitigate with
decay/aging); unbounded frequency counters; `Rebuild()` left unimplemented
(P7 needs it — stub it now, correctly).

---

### ⬜ P3 — TRACES & BENCHMARK

**Goal:** Reproducible request streams + the harness. **The demo's entire data
layer exists after this phase, before any UI.**

**CREATE**
```
trace/source.go             TraceSource iface, SourceInfo, Segment
trace/csv.go
trace/synthetic/temporal.go · spatial.go · random.go
trace/synthetic/bursty.go · workingset.go · mixed.go
trace/synthetic/sizes.go    homogeneous | heterogeneous | heavy-tailed
trace/scenario.go           YAML replay → emits ScenarioMarkEvent  ← CRITICAL
trace/journal.go            record/replay for determinism
scenarios/showcase.yaml
scenarios/adversarial_oscillation.yaml
benchmark/runner.go · experiment.go · matrix.go · results.go
cmd/bench/main.go
benchmark/regression_test.go   fail CI if hit rate drops >2%
```
**TOUCH:** `cmd/.../main.go`, `configs/default.yaml`

**DONE WHEN:** same seed twice → **byte-identical** results; one trace replayed
across LRU/LFU/Clock → CSV table; every run writes a manifest (git commit, seed,
config, Go version).

**TRAPS:** forgetting `ScenarioMarkEvent` (P6 cannot measure detection delay
without it); non-deterministic map iteration leaking into generators; the UI
generating data (**it must not** — see `context.md` §6.2).

---

### ⬜ P4 — ARC, W-TINYLFU, SHADOWS

**Goal:** Complete the baselines + the comparison machinery.

**CREATE**
```
cache/policy/arc.go            REAL ARC: T1,T2,B1,B2 + adaptive p
cache/policy/arc_test.go       vs published sequences
cache/policy/tinylfu.go        window LRU + SLRU + admission
cache/policy/sketch/countmin.go · doorkeeper.go · sketch_test.go
cache/policy/tinylfu_test.go
shadow/shadow.go               metadata-only simulators
shadow/oracle.go               Bélády MIN (offline)
shadow/regret.go               cumulative regret vs hindsight-best
shadow/shadow_test.go
```
**TOUCH:** `cache/policy/registry.go`, `events/types.go` (`Frame.Shadow` map)

**DONE WHEN:** 5 policies × 6 workloads matrix + oracle upper bound;
shadows overhead measured and <1% of runtime.

**TRAPS:** ❌ **fake ARC** (a recency+frequency score is NOT ARC — if
simplified, document precisely what in `docs/LIMITATIONS.md`); claiming
production fidelity for W-TinyLFU; shadows storing payloads (metadata only).

---

### ⬜ P5 — FEATURE EXTRACTION

**CREATE**
```
workload/monitor.go              ring buffer, O(1)
workload/features/features.go    Extract() + stable Names()
workload/features/locality.go    reuse distance, contiguous ratio
workload/features/frequency.go   entropy, Zipf α, top-K concentration
workload/features/burstiness.go  CV, spike ratio
workload/features/workingset.go
workload/features/size.go
workload/features/*_test.go      each feature tested INDEPENDENTLY
cmd/analyze/main.go              trace → feature vector
```
**TOUCH:** `cache/core.go` (feed monitor on every request — hits AND misses)

**DONE WHEN:** `./analyze trace.csv` prints a vector; each generated workload
produces a *visibly distinct* signature (record these in `docs/EXPERIMENTS.md`
— P6's rules come from them).

**TRAPS:** O(n²) reuse-distance (use a stack-distance approximation);
feeding only misses to the monitor; feature extraction on the hot path
(sample/window it).

---

### ⬜ P6 — WORKLOAD DETECTOR

**CREATE**
```
workload/classify/classifier.go   iface
workload/classify/rules.go        V1 rule-based (thresholds from P5)
workload/classify/tree.go         V2 decision tree
workload/classify/detector.go     dual window: feature(1000) + detection(50)
workload/classify/delay.go        DetectionEvent vs ScenarioMarkEvent
workload/classify/*_test.go       generated temporal ⇒ classified temporal
```
**TOUCH:** `cache/core.go`, `configs/default.yaml` (windows, threshold 0.80)

**DONE WHEN:** ≥5/6 workloads classified correctly; **measured** detection delay
(mean/median/p95 + failures-to-detect) → **RQ3 answered**.

**TRAPS:** hard-coding 1000 (both windows configurable, both swept); starting
with deep learning (❌ — rules first); **fabricating detection accuracy**
(report the real distribution, including misses).

---

### ⬜ P6.5 — TUI INCREMENT (detection)

**Goal:** Make workload classification and detection delay visible. Extends the
P3.5 dashboard — creates no new files.

**CREATE:** none.

**TOUCH**
```
tui/panels.go   add classified-workload line, confidence readout, feature bars
                (reuse-distance, entropy, burstiness, working-set, size-variance)
tui/chart.go    render the detected-transition marker alongside the existing
                ground-truth marker, and label the Δ between them
```

**DONE WHEN**
- Dashboard shows current classified workload + confidence, updating live
- Feature bars visibly change signature as the workload changes
- Both markers appear on the chart with the measured Δ labelled

**TRAPS:** rewriting the P3.5 dashboard instead of extending it; showing the
ground-truth workload where the *classified* workload belongs — the detector
must not see ground truth; that data is only used to draw the comparison marker.

**REVIEW TALKING POINT**
> "The system now sees the transition. The red marker is when the workload
> actually changed — ground truth from the generator. The amber marker is when
> our detector noticed. That gap is our measured detection delay."

---

### ⬜ P7 — ADAPTIVE ENGINE ⭐ THE CONTRIBUTION

**CREATE**
```
adaptive/engine.go       Observe/Decide/Apply
adaptive/selector.go     workload→policy from CONFIG, not code
adaptive/guards.go       confidence · min-residency · hysteresis ·
                         cooldown · expected-gain · switch-cost
adaptive/switcher.go     STATE-PRESERVING switch (shared store + Rebuild)
adaptive/decision.go     AdaptiveDecision incl. GuardsFailed
adaptive/explain.go      last-N decisions + feature vectors
adaptive/*_test.go
tests/integration/adaptation_test.go
tests/integration/oscillation_test.go   guards prevent thrashing
```
**TOUCH:** `cache/core.go`, `configs/default.yaml`, `cmd/.../main.go`

**DONE WHEN:** adaptive vs fixed on `showcase.yaml` → **RQ1, RQ2 answered**;
switch overhead measured in ms; **zero objects evicted during a switch**;
`adversarial_oscillation.yaml` shows guards firing (log `GuardsFailed`).

**TRAPS:** ❌ **destroying/recreating the cache on switch** (cold cache =
invalid results — objects live in the store, only metadata rebuilds);
hard-coding the workload→policy map in Go (config only); not logging *rejected*
switches (they explain periods of stability — you'll be asked).

---

### ⬜ P8 — OBSERVABILITY

**Goal:** Final observability increment and report output for review artifacts.
**The report generator matters more than the TUI.**

**CREATE**
```
scripts/report/figures.py     all 12 figures (context.md §7.5)
scripts/report/report.py      JSON → Markdown/PDF
scripts/report/requirements.txt
docs/RESULTS.md               stub — already scaffolded; extend it
```

**TOUCH**
```
server/frames.go
tui/dashboard.go
tui/panels.go
tui/chart.go
```

**Note:** **The TUI already exists from P3.5/P6.5. This phase extends it with
the policy timeline, decision card, oracle line, and shadow scoreboard. Do not
rewrite it.**

**DONE WHEN:** `./adaptive-cache --tui --scenario=showcase.yaml` shows live
workload/policy/hit-rate/timeline/decision; `make report` emits all figures.

**TRAPS:** `tui/` imported by `cache/` (lint must fail); TUI blocking on a slow
render (drop frames); rewriting the existing TUI instead of extending it;
building the *web* UI here (that's P14 — TUI is your demo-day insurance).

---

### ⬜ P9 — SIZE-AWARE EVICTION

**CREATE**
```
eviction/scorer.go        SizeAwareScorer iface
eviction/strategies.go    none | hitprob_per_byte | sqrt_normalised | gdsf |
                          cost_aware
eviction/batch.go         EvictUntil(requiredBytes)   ← not EvictOne
eviction/admission.go     ShouldAdmit(key,size,features)
eviction/*_test.go
tests/edge/hetero_test.go 10MB object into a near-full 100MB cache
```
**TOUCH:** `cache/core.go`, `cache/store.go`, `metrics/collector.go` (byte hit rate)

**DONE WHEN:** ablation E (size-aware vs naive) on heterogeneous sizes →
**RQ6 answered**; byte hit rate visibly diverges from object hit rate.

**TRAPS:** evicting one object when 5 MB is needed; assuming a scoring formula
is "correct" (it's an experimental choice — *compare* them).

---

### ⬜ P10 — ONLINE TUNING

**CREATE**
```
tuning/tuner.go · parameter.go (registry: range/default/current/step/metric)
tuning/optimizer.go        A/B sweep, keep winner
tuning/convergence.go
tuning/*_test.go
configs/experiments/tuning_sweep.yaml
```
**TOUCH:** `adaptive/engine.go`, `configs/default.yaml` (interval 5000)

**DONE WHEN:** ablation D (adaptive+tuning vs adaptive) → **RQ7 answered**;
sensitivity sweep at intervals 1000/5000/10000.

**TRAPS:** ❌ reinforcement learning (simple A/B only); inventing parameters
that don't actually affect the policy; tuning during a workload transition
(gate it).

---

### ⬜ P11 — EVALUATION ⭐ THE GRADE

**Not a coding phase — a *running and interpreting* phase.**

**CREATE**
```
benchmark/ablation.go      variants A–H via FEATURE FLAGS, not code branches
benchmark/multirun.go      ≥10 seeds, mean/CI/p95/p99
metrics/statistics.go      ANOVA, t-test + Bonferroni, Cohen's d
configs/experiments/ablation_{a..h}.yaml
scripts/run_all.sh
docs/RESULTS.md            ← REAL NUMBERS, run IDs cited
docs/LIMITATIONS.md
```
**TOUCH:** `scripts/report/figures.py`

**DONE WHEN:** ablations A–H × 6 workloads × ≥10 seeds complete; every ⬜ in
`context.md` §9.1 flipped to ✅ **with a run ID**, or explicitly marked
NOT MET; **the failure case is documented prominently** (Success Condition #10);
regret vs hindsight-optimal reported.

**TRAPS:** ❌ **fabricating any number** — if adaptive loses, report it;
ablations as `if` branches instead of config flags (you'll drown);
running statistical tests whose assumptions don't hold.

---

### ⬜ P12 — CONCURRENCY & PROFILING

**CREATE**
```
cache/sharded.go           sharded store
cache/concurrent_test.go   -race
benchmark/threads.go       1/4/16/32/64/100+
scripts/profile.sh         pprof
tests/edge/concurrent_test.go
docs/PERFORMANCE.md
```
**TOUCH:** `cache/core.go`, `cache/store.go`, `metrics/collector.go` (atomics),
`adaptive/engine.go`

**DONE WHEN:** `-race` clean; throughput-vs-threads chart; detection + switch
overhead quantified → **RQ4, RQ5 answered**; coverage ≥80%.

**TRAPS:** optimising before profiling; claiming "lock-free" when it isn't.

---

### ⬜ P13 — MULTI-TIER L1→L2

**CREATE**
```
tiers/tier.go · l1.go · l2.go · promote.go · netsim.go
tiers/*_test.go
tests/integration/tier_test.go     L2 hit → L1 promotion
configs/experiments/tiers.yaml
```
**TOUCH:** `cache/core.go`, `configs/default.yaml`

**DONE WHEN:** ablation G → **RQ8 answered**; promotion benefit measured
(**do not assume promotion helps — it may not**).

**TRAPS:** L3 before L2 is stable; production consensus (❌ — simple quorum is
the ceiling); letting this jeopardise the core contribution (**cut it if
behind**).

---

### ⬜ P14 — WEB UI & DOCS

**Gate: do NOT start until `docs/RESULTS.md` has real numbers.**

**CREATE**
```
server/server.go · ws.go · control.go · api.go
server/web/index.html · app.js · style.css      go:embed
Dockerfile
docs/ARCHITECTURE.md · API.md · EXPERIMENTS.md
README.md
```
**TOUCH:** `cmd/adaptive-cache/main.go`, `Makefile`

**DONE WHEN:** single binary serves the dashboard; scenario/seed/replay/inject
controls work; side-by-side comparison panel live; demo recorded.

**TRAPS:** `cache/` importing `server/`; UI generating data; a frontend
framework (one WebSocket, one reducer — vanilla is enough).

---

## §5 — FROZEN CONTRACTS

Changing any of these requires an ADR in `docs/DECISIONS.md` + updating all
dependents in the same commit.

| Contract | Since | File |
|---|---|---|
| `Cache` | P1 | `cache/cache.go` |
| `EvictionPolicy` | P1 | `cache/policy/policy.go` |
| `Event`, `Frame` | P1 | `events/types.go` |
| `Request`, `Entry` | P1 | `types/` |
| `TraceSource` | P3 | `trace/source.go` |
| `Classifier` | P6 | `workload/classify/classifier.go` |
| `SizeAwareScorer` | P9 | `eviction/scorer.go` |

**Architecture rule (CI-enforced):** `cache/` must never import `ui/`,
`server/`, `tui/`, `benchmark/`, or `adaptive/`.

---

## §6 — EXIT CHECKLIST (run at the end of every phase)

```
[ ] go test ./... green
[ ] go test -race ./... green (P12+)
[ ] make lint-arch green
[ ] regression benchmark within 2% (P3+)
[ ] CHANGELOG.md: what this phase added
[ ] docs/DECISIONS.md: any non-obvious choice + why
[ ] docs/LIMITATIONS.md: any simplification made
[ ] §1 SNAPSHOT updated
[ ] §3 LEDGER: mark ✅, record git tag
[ ] §7 RESULTS: append any measured numbers (with run IDs)
[ ] §8 DECISIONS: append anything that changes future phases
[ ] git tag pNN-complete && git push --tags
```

---

## §7 — MEASURED RESULTS

**Only real numbers. Cite a run ID. Empty is correct until P11.**

| Metric | Target | Acceptable | Measured | Run ID | Status |
|---|---|---|---|---|---|
| Hit rate vs LRU | +12% | +8% | — | — | ⬜ NOT MEASURED |
| p99 latency vs LRU | −25% | −15% | — | — | ⬜ NOT MEASURED |
| Policy switch time | <100ms | <200ms | — | — | ⬜ NOT MEASURED |
| Memory overhead | <5% | <10% | — | — | ⬜ NOT MEASURED |
| Detection delay | 1–5 req | 1–10 req | — | — | ⬜ NOT MEASURED |
| Distribution cost | <10% | <15% | — | — | ⬜ NOT MEASURED |

**Research questions**

| RQ | Question | Phase | Answer |
|---|---|---|---|
| RQ1 | Adaptive vs fixed LRU hit rate? | P7 | ⬜ |
| RQ2 | Adaptive vs best fixed on transitions? | P7 | ⬜ |
| RQ3 | Transition detection speed? | P6 | ⬜ |
| RQ4 | Detection overhead? | P12 | ⬜ |
| RQ5 | Switching overhead? | P12 | ⬜ |
| RQ6 | Size-aware → byte hit rate? | P9 | ⬜ |
| RQ7 | Tuning beyond selection? | P10 | ⬜ |
| RQ8 | Multi-tier worth the overhead? | P13 | ⬜ |

---

## §8 — DECISION LOG

Append-only. Anything that changes how future phases must be built.

| Date | Phase | Decision | Rationale |
|---|---|---|---|
| 2026-08-25 | P0 | Event bus lands in P1, not P8 | Retrofitting is expensive; UI stays decoupled and cheap |
| 2026-08-25 | P0 | Capacity in **bytes**, not object count | Heterogeneous objects are a core contribution |
| 2026-08-25 | P0 | Ablations via feature flags, not code branches | Avoids 8 divergent code paths |
| 2026-08-25 | P0 | Dual detection window (feature 1000 / detect 50) | Resolves "stable stats" vs "detect in 10–20 req" tension |
| 2026-08-25 | P0 | Shadow caches at P4 | Upgrades claim from "beats LRU" to "approaches hindsight-optimal" |
| 2026-08-25 | P0 | TUI at P8, web at P14 | TUI = 90% of demo value at 20% cost; insurance for demo day |
| 2026-08-25 | P0 | TUI split into P3.5 → P6.5 → P8 increments | Demo-ready 5 weeks earlier for +1 day total; P3.5 demonstrates the problem, which is a stronger R2 artifact than a static results table |
| 2026-08-25 | P1 | `types.Value` is a concrete `[]byte`, not an interface | Byte accounting is exact and allocation-free; an interface value would need reflection or a caller-supplied size that can silently disagree with reality, and would add 16 bytes of header per entry against a <5% metadata-overhead target |
| 2026-08-25 | P1 | Latency histogram is log-linear (4 significant bits, 976 buckets, ~8 KB) rather than exact or HDR-library-backed | Lock-free and allocation-free on the request path with ≤6.25% quantile error; an allocating or locking histogram would perturb the very latencies it measures. Mean is kept exactly from a running sum |
| 2026-08-25 | P1 | Bus subscribers are named, with per-subscriber drop counters; `Publish` drops on a full channel | Observability on the observability: a stalled consumer is visible as a counter rather than as silent backpressure that would corrupt every latency measurement |
| 2026-08-25 | P1 | The store reports `ErrCapacityExceeded` instead of evicting; only the policy chooses victims | Keeps "the policy holds metadata, the store holds objects" true at the type level, which is what makes P7's state-preserving switching possible |
| 2026-08-25 | P1 | Byte-hit-rate fetch bytes are recorded on `Put`, not on a missing `Get` | The frozen `Cache.Get` signature carries no size, and the cache cannot know how large a missing object is until it is inserted |
| 2026-08-25 | P1 | `Entry` omits the `Tier` and `ExpiresAt` fields sketched in context.md §5.5 | Nothing needs them before P13; every field costs metadata overhead per cached object. They will be added with an ADR when tiers land |
| 2026-08-26 | P1 | `PROJECT_STATE.md` was truncated by an agent rewrite and subsequently restored from the author's local copy | This file is edited in place, never regenerated |

---

## §9 — OPEN QUESTIONS

| # | Question | Blocks | Owner |
|---|---|---|---|
| 1 | Are Wikipedia/MSR/CloudPhysics traces actually obtainable? | P11 | verify BEFORE P11 — if not, document a labelled substitute |
| 2 | Is real ARC feasible in the P4 budget, or simplify? | P4 | if simplified, state exactly what in LIMITATIONS.md |
| 3 | Which size-aware formula wins? | P9 | empirical — do not assume |
| 4 | Does promotion in L1→L2 actually help? | P13 | benchmark it; may be a negative result |
| 5 | Why do latency percentiles (`p50`, `p95`, `p99`) read zero in `Frame` output at ~9.7M ops/sec with a nil policy? This is plausibly sub-bucket flooring, but RQ targets include a −25% p99 latency claim. | P11 | verify real percentile values during P2, when policy work makes operations measurably slower, and before P11 |
| 6 | Can metadata overhead meet the <5% target? The nil-policy floor is ~15.6% (318,890 bytes metadata against 2,048,000 bytes payload, 1 KB synthetic objects), before policies add their own metadata. | P11 | revisit the `Entry` layout or the target itself via an ADR if needed |

---

## §10 — IF YOU'RE BEHIND SCHEDULE

Cut in this order:
1. P13 (multi-tier) — RQ8 becomes "future work"
2. P14 web UI — TUI is sufficient for the demo
3. P10 (tuning) — RQ7 becomes future work
4. Learned selector, cost-aware eviction, admission control

**Never cut P11.** A project with 5 policies + working adaptation + rigorous
evaluation beats one with tiers, a web UI, and thin results.

**Minimum defensible project:** P1–P8 + P11.
```