# Architecture Decision Records

This log is append-only. Add a new row when an architectural decision is made; do not rewrite prior decisions.

| Date | Phase | Decision | Rationale |
|---|---|---|---|
| 2026-08-25 | P0 | Event bus lands in P1, not P8 | Retrofitting is expensive; keeps UI decoupled and cheap |
| 2026-08-25 | P0 | Capacity measured in **bytes**, not object count | Heterogeneous objects are a core contribution |
| 2026-08-25 | P0 | Ablations via feature flags, not code branches | Avoids 8 divergent code paths |
| 2026-08-25 | P0 | Dual detection window (feature 1000 / detect 50) | Resolves “stable statistics” vs “detect within 10–20 requests” tension |
| 2026-08-25 | P0 | Shadow caches at P4 | Upgrades claim from “beats LRU” to “approaches hindsight-optimal” |
| 2026-08-25 | P0 | TUI split into P3.5 → P6.5 → P8 increments; web UI at P14 | Demo-ready 5 weeks earlier for ~1 extra day total |
| 2026-08-25 | P1 | `types.Value` is a concrete `[]byte` rather than an interface | Exact, allocation-free byte accounting; an interface would need reflection or a caller-supplied size that can disagree with reality, plus 16 bytes of header per entry against a <5% metadata-overhead target |
| 2026-08-25 | P1 | Latency histogram is log-linear with 4 significant bits (976 buckets, ~8 KB), mean kept exactly | Lock-free, allocation-free recording with ≤6.25% quantile error; an allocating or locking histogram would perturb the latencies it measures |
| 2026-08-25 | P1 | Event bus subscribers are named and carry per-subscriber drop counters | Observability on the observability — a stalled consumer shows up as a counter, never as backpressure on the cache |
| 2026-08-25 | P1 | The store never evicts; it returns `ErrCapacityExceeded` and the policy chooses victims | Enforces "policy holds metadata, store holds objects" at the type level, which is what makes P7 state-preserving switching possible |
| 2026-08-25 | P1 | With a nil policy the cache rejects over-capacity inserts instead of displacing objects | A cache with no eviction strategy must not invent one; keeps P1 free of policy logic |
| 2026-08-25 | P1 | Backend/fetch bytes are recorded on `Put`, not on a missing `Get` | The frozen `Cache.Get` signature carries no size; the object's size is only known at insertion |
| 2026-08-25 | P1 | `Entry` omits `Tier` and `ExpiresAt` for now | Unused before P13 and every field is paid for once per cached object on the overhead metric; they return with an ADR when tiers land |
| 2026-08-25 | P1 | `scripts/lint-arch.sh` uses `go list -deps` over `cache/...` rather than a third-party arch linter | Catches indirect violations, needs no extra CI dependency, and is trivially auditable |

