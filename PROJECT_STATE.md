# PROJECT STATE

## 1) Snapshot

```
GIT TAG      : p01-complete
BUILD PHASE  : P2  🔵 IN PROGRESS
LAST UPDATED : 2026-08-25
```

**Where we are:** P1 is complete. The frozen interfaces (`Cache`,
`EvictionPolicy`, `Event`, `Frame`, `Request`, `Entry`) and a non-blocking,
bounded, droppable event bus are in place, along with config, metrics, the
byte-capacity object store and a nil-safe cache core. `go build`, `go vet`,
`go test`, `go test -race` and `scripts/lint-arch.sh` all pass, and
`./adaptive-cache --config configs/default.yaml --duration 5s` prints `Frame`
JSON at 10 Hz.

**Next:** P2 — the baseline policies (LRU, LFU, Clock) behind the frozen
`EvictionPolicy` interface, plus a policy registry.

## 3) Phase ledger

| Phase | Name | Status | Owner | New | Touch | Est. | Review |
|---|---|---|---|---:|---:|---|---|
| P1 | Skeleton | ✅ | — | 11 | 0 | 1w | R1 |
| P2 | Baselines I | ⬜ | — | 8 | 2 | 1w | R1 |
| P3 | Traces | ⬜ | — | 9 | 3 | 1w | R2 |
| P3.5 | Early TUI (policy race) | ⬜ | — | 6 | 2 | 2d | R2 |
| P4 | Baselines II + Shadows | ⬜ | — | 10 | 4 | 1.5w | R2 |
| P5 | Features | ⬜ | — | 6 | 3 | 1w | R3 |
| P6 | Detector | ⬜ | — | 7 | 3 | 1w | R3 |
| P6.5 | TUI increment (detection) | ⬜ | — | 0 | 2 | 1d | R3 |
| P7 | Adaptive engine | ⬜ | — | 9 | 6 | 2w | R4 |
| P8 | Observability | ⬜ | — | 3 | 4 | 3d | R4 |
| P9 | Size-aware | ⬜ | — | 5 | 4 | 1w | R5 |
| P10 | Tuning | ⬜ | — | 5 | 4 | 1w | R5 |
| P11 | Evaluation | ⬜ | — | 7 | 6 | 2w | R5 |
| P12 | Concurrency + profiling | ⬜ | — | 8 | 7 | 1.5w | R5 |
| P13 | Multi-tier | ⬜ | — | 7 | 6 | 1.5w | R5 |
| P14 | Web UI + docs | ⬜ | — | 10 | 7 | 2w | R5 |

**Totals:** 16 phases (P1–P14 + P3.5 + P6.5) · 111 new files · 63 touched files · ~18w + 1d planned. Net schedule change vs prior single-pass P8 TUI: **~+1 day**.

## 4) Phase cards

### ⬜ P6 — DETECTOR

**Goal:** Classify workload from stream features and quantify detection delay.

### ⬜ P6.5 — TUI INCREMENT (detection)

**Goal:** Make workload classification and detection delay visible. Extends the
P3.5 dashboard — creates no new files.

**CREATE:** none.

**TOUCH**
- `tui/panels.go` — add classified-workload line, confidence readout, feature
  bars (reuse-distance, entropy, burstiness, working-set, size-variance)
- `tui/chart.go` — render the **detected-transition marker** alongside the
  existing ground-truth marker, and label the Δ between them

**DONE WHEN**
- Dashboard shows current classified workload + confidence, updating live
- Feature bars visibly change signature as the workload changes
- Both markers appear on the chart with the measured Δ labelled

**TRAPS**
- ❌ Rewriting the P3.5 dashboard instead of extending it
- ❌ Showing the ground-truth workload where the *classified* workload belongs —
  the detector must not see ground truth; that data is only used to draw the
  comparison marker

**REVIEW TALKING POINT**
> "The system now sees the transition. The red marker is when the workload
> actually changed — ground truth from the generator. The amber marker is when
> our detector noticed. That gap is our measured detection delay."

### ⬜ P7 — ADAPTIVE ENGINE

**Goal:** Switch policies safely based on classifier output and guardrails.

### ⬜ P8 — OBSERVABILITY

**Goal:** Final observability increment and report output for review artifacts.

**Estimate:** 3 days.

**CREATE**
- `scripts/report/figures.py`
- `scripts/report/report.py`
- `scripts/report/requirements.txt`
- `docs/RESULTS.md (stub — already scaffolded; extend it)`

**TOUCH**
- `server/frames.go`
- `tui/dashboard.go`
- `tui/panels.go`
- `tui/chart.go`

**Note:** **The TUI already exists from P3.5/P6.5. This phase extends it with the policy timeline, decision card, oracle line, and shadow scoreboard. Do not rewrite it.**

## 8) Decision log

| Date | Phase | Decision | Why |
|---|---|---|---|
| 2026-08-25 | P0 | TUI split into P3.5 → P6.5 → P8 increments | Demo-ready 5 weeks earlier for +1 day total; P3.5 demonstrates the problem, which is a stronger R2 artifact than a static results table |
| 2026-08-25 | P1 | `types.Value` is a concrete `[]byte`, not an interface | Byte accounting is exact and allocation-free; an interface value would need reflection or a caller-supplied size that can silently disagree with reality, and would add 16 bytes of header per entry against a <5% metadata-overhead target |
| 2026-08-25 | P1 | Latency histogram is log-linear (4 significant bits, 976 buckets, ~8 KB) rather than exact or HDR-library-backed | Lock-free and allocation-free on the request path with ≤6.25% quantile error; an allocating or locking histogram would perturb the very latencies it measures. Mean is kept exactly from a running sum |
| 2026-08-25 | P1 | Bus subscribers are named, with per-subscriber drop counters; `Publish` drops on a full channel | Observability on the observability: a stalled consumer is visible as a counter rather than as silent backpressure that would corrupt every latency measurement |
| 2026-08-25 | P1 | The store reports `ErrCapacityExceeded` instead of evicting; only the policy chooses victims | Keeps "the policy holds metadata, the store holds objects" true at the type level, which is what makes P7's state-preserving switching possible |
| 2026-08-25 | P1 | Byte-hit-rate fetch bytes are recorded on `Put`, not on a missing `Get` | The frozen `Cache.Get` signature carries no size, and the cache cannot know how large a missing object is until it is inserted |
| 2026-08-25 | P1 | `Entry` omits the `Tier` and `ExpiresAt` fields sketched in context.md §5.5 | Nothing needs them before P13; every field costs metadata overhead per cached object. They will be added with an ADR when tiers land |
