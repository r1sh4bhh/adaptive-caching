# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [P2] - 2026-09-05

### Added
- **`cache/policy/lru.go`** — the canonical LRU policy: doubly-linked
  list + `map[string]*list.Element`, O(1) access, O(1) eviction. No
  parameters exposed (none are genuinely tunable). Registered as
  `lru`.
- **`cache/policy/lfu.go`** — frequency-bucket LFU with amortised
  ageing to mitigate the cold-start problem (context.md §5.1). Each
  `OnAccess` increments a key's frequency, capped at `1<<16` to keep
  counters bounded; every `floor(1/decay_lambda)` accesses, all
  frequencies are halved. Exposes a single tunable: `decay_lambda`
  (range [0,1], default 0.05, metric `hit_rate`). Registered as
  `lfu`.
- **`cache/policy/clock.go`** — second-chance Clock: ring buffer of
  `clockSlot{key, ref, e}` plus a walking hand. The ring grows by
  powers of two on demand and is never shrunk (the metadata cap is
  what `MetadataBytes()` reports). No parameters (the Nth-chance
  variants exist but are not part of this baseline). Registered as
  `clock`.
- **`cache/policy/registry.go`** — name → constructor map. Policies
  self-register in `init()`; `policy.New(name)` resolves a name to a
  fresh instance and `policy.Names()` lists the registered set.
  Concurrent-read-safe via an `RWMutex`; writes only happen at
  package-init time.
- **`cache/policy/lru_test.go`, `lfu_test.go`, `clock_test.go`** —
  table-driven tests against hand-computed eviction sequences, plus
  rebuild round-trips, `Reset`/`MetadataBytes`/param round-trips,
  empty-policy edge cases, and registry wiring.
- **`tests/edge/edge_test.go`** — the five edge cases called out in
  the P2 phase card: `cap=1`, `size=0`, `obj>cap`, duplicate
  insert, empty policy. Each test runs against every shipped
  policy so the assertions are the same in production.
- **CLI `--policy` flag** on `cmd/adaptive-cache/main.go`. Overrides
  `cache.policy` from the YAML config when supplied; the help text
  enumerates the registered names at startup.
- **Config validation** — `config/validate.go` now rejects policy
  names that the registry does not know, with a clear error
  listing the available choices. Replaces the previous free-form
  string check.
- **Default config** — `configs/default.yaml` ships with
  `policy: lru` so the demo shows eviction actually happening.

### Notes
- The LRU and LFU `MetadataBytes()` reports a conservative
  per-entry estimate (64 bytes/entry) so the <5% metadata-overhead
  target is not gamed. Clock reports `24 * ring_capacity`, which
  includes the unused tail of the ring (honest accounting for the
  power-of-two growth strategy).
- P2 ships no workload detection, adaptive engine, or multi-tier
  logic. Those are P3+.
- The frozen `EvictionPolicy` interface from P1 is unchanged.
  Policies differ in their `Params()` output, not the contract.

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
