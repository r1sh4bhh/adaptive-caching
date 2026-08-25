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
