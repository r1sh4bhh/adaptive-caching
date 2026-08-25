# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [P1] - 2026-08-25

### Added
- **Build & CI:** `go.mod` (Go 1.22, `gopkg.in/yaml.v3` the only dependency),
  `Makefile` (`build`, `test`, `test-race`, `lint`, `lint-arch`, `run`,
  `clean`), `.github/workflows/ci.yml` and `scripts/lint-arch.sh`, which fails
  the build if `cache/` gains a transitive import of `server/`, `tui/`,
  `benchmark/`, `adaptive/` or `ui/`.
- **`types/`** — leaf package with `Request`/`OpType`, `Entry`/`Value`,
  `WorkloadType`/`Features`/`WorkloadPrediction`, and
  `PolicyName`/`Parameter`/`ParamSet`.
- **`events/`** — `Type` enum, `Event`, the `SwitchEvent`, `DetectionEvent`,
  `TuningEvent` and `ScenarioMarkEvent` payloads, the `Frame` UI transport
  record, and the `Bus` interface with its `MemBus` implementation:
  bounded per-subscriber channels, type filtering, per-subscriber dropped
  counters and a **`Publish` that never blocks** — a full subscriber has its
  event dropped and counted rather than being allowed to backpressure the
  cache. Tested, including a stalled subscriber that must cause drops rather
  than blocking, and a `-race`-clean concurrent publish test.
- **`config/`** — YAML load/parse over defaults, `ByteSize` parsing (`100MB`,
  `512KiB`, raw bytes), event-bus buffer sizes, 1-in-1000 request sampling,
  10 Hz frame rate, log level, `FeatureFlags` (ablations are config
  permutations, never code branches), range validation and
  `configs/default.yaml`.
- **`metrics/`** — `MetricsCollector` interface and atomic-counter `Collector`
  with CSV/JSON output, a lock-free log-linear latency histogram
  (mean/p50/p95/p99, ≤6.25% bucketing error) and memory accounting that
  separates object payload from entry, index and policy metadata via
  `runtime.ReadMemStats` plus `unsafe.Sizeof`.
- **`cache/`** — the frozen `Cache` interface (**capacity in bytes**), the
  frozen `EvictionPolicy` interface (metadata only — the store owns objects),
  a byte-accounting object `Store` that never decides what to evict, and a
  `Core` that wires store, policy, metrics and bus together with a **pluggable,
  nil-safe policy field**: with no policy the cache still serves, counts and
  publishes sampled hit/miss events, and rejects inserts that would exceed
  capacity.
- **`cmd/adaptive-cache`** — the P1 acceptance demo: loads a config, builds the
  cache with a nil policy and prints `Frame` JSON at the configured rate
  (`--config`, `--duration`).

### Notes
- No eviction policy, workload detection or adaptation logic exists yet; those
  are P2+.
- `Cache`, `EvictionPolicy`, `Event`, `Frame`, `Request` and `Entry` are now
  frozen. Changing them requires an ADR in `docs/DECISIONS.md` plus updating
  all dependents in the same commit.

## [P0] - 2026-08-25

- Repository scaffolded: directory tree, documentation stubs, project handoff files.
