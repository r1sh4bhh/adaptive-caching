# Architecture

> **Layering rule:** `cache/` must never import `ui/`, `server/`, `tui/`, `benchmark/`, or `adaptive/`. This rule is CI-enforced by `scripts/lint-arch.sh`.

## Overview

A single Go binary. Three layers (context.md §0.4):

| Layer | Packages | Role |
|---|---|---|
| 3 — Presentation (optional, deletable) | `server/`, `tui/`, `scripts/report/` | Read-only consumers of the event bus |
| 2 — Experimentation (the science) | `benchmark/`, `trace/`, `metrics/`, `shadow/` | Runs and measures experiments |
| 1 — The cache (the contribution) | `cache/`, `cache/policy/`, `workload/`, `adaptive/`, `tuning/`, `eviction/`, `tiers/` | The system under study |

If Layer 3 is deleted, nothing breaks. That is the design constraint that keeps
the UI cheap.

As of P1 the following exist in code: `types/`, `events/`, `config/`,
`metrics/`, `cache/` (store + core + the `EvictionPolicy` interface) and
`cmd/adaptive-cache/`. Everything else is a scaffolded directory awaiting its
phase.

## Layering rules

```
ALLOWED IMPORT DIRECTION (top may import bottom, never the reverse)

    cmd/  ─────────────────────────────────────┐
    server/ · tui/ · benchmark/ ───────────┐   │
    shadow/ · tiers/ ──────────────────┐   │   │
    adaptive/ · tuning/ ───────────┐   │   │   │
    workload/ · eviction/ ─────┐   │   │   │   │
    cache/ · trace/ ───────┐   │   │   │   │   │
    events/ · metrics/ · config/ · types/  (leaves)
```

Within the leaf row the internal ordering is `types/` → `events/` → `metrics/`;
`types/` imports nothing internal at all.

**Forbidden, non-negotiable:**

- `cache/` importing `ui/`, `server/`, `tui/`, `benchmark/` or `adaptive/`
- Any policy knowing that a dashboard exists
- Any component reading global mutable state

**Enforcement.** `scripts/lint-arch.sh` walks the transitive dependency closure
of every package under `cache/...` with `go list -deps` and exits non-zero,
naming the offending edge, if any forbidden prefix appears. It runs in CI on
every push and pull request, and via `make lint-arch`. Because it uses the
*transitive* closure, an indirect violation (`cache → x → tui`) fails too. The
check was verified to fail by temporarily adding a forbidden import.

## Component map

See context.md §3.1 for the full diagram. The P1 subset:

```
        TraceSource (P3)
              │  Request stream
              ▼
    ┌────────────────────────────────────────┐
    │  CACHE CORE  (cache/core.go)           │
    │   ┌───────────────┐  ┌──────────────┐  │
    │   │ Object Store  │  │EvictionPolicy│  │
    │   │ key → *Entry  │  │  (nil in P1) │  │
    │   │ byte capacity │  │  metadata    │  │
    │   └───────────────┘  └──────────────┘  │
    └───────┬───────────────────┬────────────┘
            │                   │
            ▼                   ▼
     MetricsCollector       EVENT BUS  ──►  frame emitter (cmd/)
     (lossless, atomic)     (bounded, droppable, non-blocking)
```

**The store owns the objects; the policy owns only metadata.** A policy never
allocates, holds or frees a cached value — it maintains recency lists,
frequency counters, sketches and clock hands, and answers `Victim()` /
`Candidates(n)`. This is what makes P7's state-preserving policy switching
possible: installing a new policy calls `Rebuild(store.Entries())` and leaves
every cached object exactly where it is.

**Capacity is in bytes, never object count** (`Cache.Capacity() int64`), because
heterogeneous object sizes are a core research contribution (context.md §5.5).

## Event bus contract

`events.Bus` (implemented by `events.MemBus`):

```go
type Bus interface {
    Publish(Event)                                  // MUST be non-blocking
    Subscribe(name string, buf int, types ...Type) <-chan Event
    Unsubscribe(name string)
    DroppedCount(name string) uint64
}
```

Rules, all enforced by tests in `events/bus_test.go`:

1. **`Publish` never blocks.** Each subscriber owns a bounded channel. A send is
   attempted with a non-blocking `select`/`default`; if the channel is full the
   event is **dropped and counted**, never queued and never waited on.
   *Rationale:* a slow consumer — a browser dashboard, a paused debugger, a
   stalled log writer — must never exert backpressure on the cache. If it could,
   the cache would measure the consumer's latency instead of its own, and every
   latency number this project reports would be silently corrupt. Losing
   observability events is an acceptable, counted failure; distorting the
   measurement is not.
2. **Drops are observable.** `DroppedCount(name)` reports per-subscriber losses.
   Subscribers that must be lossless (the metrics collector) get a large buffer
   and a dedicated draining goroutine; a drop there is a bug worth failing a
   test over.
3. **Subscribers filter by type.** `Subscribe(name, buf)` with no types receives
   everything; with types it receives only those.
4. **Concurrency-safe.** Any number of publishers and subscribers; verified
   `-race` clean. Unsubscribe closes the subscriber's channel under the write
   lock, so it cannot race with an in-flight send.
5. **`TypeRequest`/`TypeHit`/`TypeMiss` are sampled**
   (`events.request_sample_rate`, default 1-in-1000). At high request rates an
   unsampled bus would dominate the CPU profile. Aggregate counters live in
   `metrics`, not on the bus.

The bus carries `Event{Seq, Timestamp, Type, Payload}`. Payloads are
`SwitchEvent`, `DetectionEvent`, `TuningEvent` and `ScenarioMarkEvent`.
`ScenarioMarkEvent` is ground truth emitted by the trace source, used to compute
detection delay honestly — the detector must never see it.

`Frame` is the aggregated UI transport record, emitted on a fixed tick (default
10 Hz, `ui.frame_rate_hz`) rather than per request, so UI bandwidth is bounded
regardless of request rate.

## Data flow

Requests and objects flow strictly downward:

```
TraceSource → Cache → ObjectStore
                  ↘ → Monitor → Features → Classifier
```

`cache.Core.Get` touches the store, notifies the policy (`OnAccess`), records
the request in the metrics collector and, if sampled, publishes a hit or miss
event. `Put` asks the policy whether to admit, inserts, and — only if the store
reports `ErrCapacityExceeded` — repeatedly evicts the policy's chosen victim
until the object fits. With a nil policy nothing can be evicted, so the insert
is rejected rather than displacing an object arbitrarily.

## Control flow

Decisions form a loop (P6–P10):

```
Classifier → AdaptiveEngine → Policy (mutates the Cache's behaviour)
     ▲                                        │
     └──────── Metrics ◄──── EventBus ◄───────┘
```

## Observation flow

Strictly outward and one-way:

```
Everything → EventBus → [Metrics | Logger | Frames | Shadows]
```

**Nothing on the right of the event bus may ever call back into the left.** That
one rule, plus the non-blocking `Publish`, is what makes the presentation layer
free: it can be slow, it can be absent, it can be deleted, and the measured
behaviour of the cache does not change.
