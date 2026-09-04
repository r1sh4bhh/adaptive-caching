# Dynamic Adaptive Caching with Multi-Objective Optimization

A workload-aware adaptive cache framework that dynamically selects and tunes existing eviction policies according to observed workload characteristics.

> 🚧 **Build Phase P2 — Baselines I (LRU / LFU / Clock)** in progress. P1 (Skeleton & Event Bus) is complete, tag `p01-complete`. See [`PROJECT_STATE.md`](PROJECT_STATE.md) for the phase ledger and next actions.

## Concept

```text
   TRADITIONAL              OURS
   Workload                 Workload → OBSERVE → UNDERSTAND → SELECT
      ↓                       → TUNE → EVICT → MEASURE → ADAPT ─┐
   Fixed Policy               ↑                                  │
      ↓                       └──────────────────────────────────┘
   Cache
```

The framework will evaluate LRU, LFU, ARC, W-TinyLFU, and Clock across temporal, spatial, working-set, random, bursty, and mixed workloads. Capacity is measured in bytes so heterogeneous object sizes can be evaluated directly.

This is not a new eviction algorithm. It is a framework that selects and tunes existing, well-established policies based on what the workload is doing right now, and measures honestly whether that adaptation is worth its overhead.

## Research questions

| ID | Question |
|---|---|
| RQ1 | Does workload-aware policy selection improve hit rate vs a fixed LRU baseline? |
| RQ2 | Does the adaptive system outperform the best fixed policy on workloads that transition between patterns? |
| RQ3 | How quickly can the system detect workload transitions? |
| RQ4 | What is the overhead of workload detection? |
| RQ5 | What is the overhead of policy switching? |
| RQ6 | Does size-aware eviction improve byte hit rate or memory efficiency for heterogeneous objects? |
| RQ7 | Does online parameter tuning improve performance beyond policy switching alone? |
| RQ8 | Does multi-tier caching improve performance enough to justify its coordination overhead? |

## Repository layout

| Directory | Status | Purpose |
|---|---|---|
| `types/` | P1 | Shared leaf types: `Request`, `Entry`, `Value`, `WorkloadType`, `Features`, `PolicyName`, `ParamSet` |
| `events/` | P1 | Typed event bus (bounded, droppable, non-blocking `Publish`) and the `Frame` UI transport record |
| `config/`, `configs/` | P1 | YAML config loading, validation, feature flags; `configs/default.yaml` and future `configs/experiments/` ablation configs |
| `metrics/` | P1 | Atomic counters, lock-free latency histogram, payload-vs-metadata memory accounting, CSV/JSON output |
| `cache/` | P1 | Frozen `Cache` and `EvictionPolicy` interfaces, byte-capacity object store, nil-safe cache core |
| `cache/policy/` | P2+ | LRU, LFU, Clock (P2); ARC, W-TinyLFU and sketches (P4) |
| `cmd/adaptive-cache/` | P1 | Runs the cache against a config and prints `Frame` JSON at a fixed rate |
| `cmd/bench/`, `cmd/analyze/` | P3, P5 | Headless benchmark runner and trace-to-feature-vector tool |
| `trace/`, `scenarios/` | P3 | Seeded synthetic generators, CSV loader, scenario YAML replay with ground-truth transition marks |
| `benchmark/` | P3 | Experiment matrix, runner, manifests, regression checks |
| `shadow/` | P4 | Metadata-only counterfactual simulators and Bélády oracle |
| `workload/` | P5, P6 | Feature extraction and workload classification |
| `adaptive/` | P7 | Policy selection, anti-oscillation guards, state-preserving switching |
| `eviction/` | P9 | Size-aware scoring and batch eviction |
| `tuning/` | P10 | Online A/B parameter tuning |
| `tiers/` | P13 | L1/L2 coordination with simulated network latency |
| `tui/`, `server/` | P3.5+, P14 | Terminal dashboard and embedded web dashboard; read-only consumers of the event bus |
| `scripts/` | P1+ | `lint-arch.sh` (CI-enforced layering check) and future report generation |
| `tests/` | P2+ | Integration and edge-case tests |
| `docs/` | — | Architecture, decisions (ADR log), experiment protocol, limitations, API, results |
| `results/` | — | Benchmark manifests and permitted summary artifacts |

## Getting started

Requires **Go 1.22+**. The only external dependency is `gopkg.in/yaml.v3`.

```bash
make build        # builds ./adaptive-cache
make test         # go test ./...
make test-race    # go test -race ./...
make lint         # gofmt + go vet
make lint-arch    # fails if cache/ transitively imports server/, tui/, benchmark/, adaptive/ or ui/
```

Run the P1 acceptance demo, which drives the cache with a nil policy and prints `Frame` JSON at 10 Hz:

```bash
make run
# equivalent to:
./adaptive-cache --config configs/default.yaml --duration 5s
```

CI runs build, vet, test, test with the race detector, and the architecture lint on every push.

## Frozen contracts

`Cache`, `EvictionPolicy`, `Event`, `Frame`, `Request` and `Entry` are frozen as of P1. Changing any of them requires an ADR in [`docs/DECISIONS.md`](docs/DECISIONS.md) and updating all dependents in the same commit. The layering rule that `cache/` never imports the presentation or adaptive layers is enforced by `scripts/lint-arch.sh` in CI.

## Academic integrity

All performance targets in this repository are **hypotheses and success criteria, not measured results**. Never fabricate benchmark numbers. A result is reported only when it is backed by a reproducible run artifact in `results/`.

## Project documents

- [`context.md`](context.md) — design specification (read PART 0, 3, 4 and 8 first)
- [`PROJECT_STATE.md`](PROJECT_STATE.md) — current status, phase cards, and next actions
- [`CHANGELOG.md`](CHANGELOG.md) — what each phase added
- [`docs/`](docs/) — architecture, decisions, experiments, limitations, API, and results
