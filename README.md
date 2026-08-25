# Dynamic Adaptive Caching with Multi-Objective Optimization

A workload-aware adaptive cache framework that dynamically selects and tunes existing eviction policies according to observed workload characteristics.

> 🚧 Build Phase P1 — Skeleton &amp; Event Bus (not started). See `PROJECT_STATE.md`.

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

| Directory | Purpose |
|---|---|
| `cmd/` | Future adaptive-cache, benchmark, and analysis command entry points |
| `types/`, `events/`, `config/`, `configs/` | Shared contracts, event definitions, configuration code, and experiment configurations |
| `cache/`, `eviction/` | Future cache core, policy implementations, sketches, and size-aware eviction |
| `workload/` | Feature extraction and workload classification |
| `adaptive/`, `tuning/`, `shadow/` | Policy selection, online tuning, and shadow-cache evaluation |
| `tiers/`, `trace/` | Multi-tier coordination and trace generation/ingestion |
| `metrics/`, `benchmark/` | Measurements and reproducible benchmark harness |
| `server/`, `tui/` | Future server/web and terminal interfaces |
| `scenarios/`, `configs/experiments/` | Shared scenarios and feature-flag ablation configurations |
| `scripts/` | Reporting and repository automation |
| `tests/` | Integration and edge-case tests |
| `docs/` | Architecture, decisions, experiment protocol, limitations, API, and results |
| `results/` | Benchmark manifests and permitted summary artifacts |

## Getting started

Go is required. Build and test commands will arrive with Build Phase P1; this phase contains documentation and repository structure only.

## Academic integrity

All performance targets in this repository are **hypotheses and success criteria, not measured results**. Never fabricate benchmark numbers. A result is reported only when it is backed by a reproducible run artifact in `results/`.

## Project documents

- [`context.md`](context.md) — design specification
- [`PROJECT_STATE.md`](PROJECT_STATE.md) — current status and next actions
- [`docs/`](docs/) — architecture, decisions, experiments, limitations, API, and results
