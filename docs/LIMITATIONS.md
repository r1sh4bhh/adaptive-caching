# Limitations

This file must be updated whenever a simplification is made.

## Policy implementation fidelity

### Clock (P2)

The P2 Clock policy is the textbook **second-chance** variant — a
single ref bit per slot. The Nth-chance and CAR variants
(aging bits, multi-bit scores) are not implemented. The "low-overhead
comparison point" framing in context.md §5.1 is honest about this:
Clock is here as a baseline, not as a competitive policy.

Clock's ring is also **never shrunk**. The unused tail of the ring
counts toward `MetadataBytes()` so the metadata-overhead metric
reflects the real cost of the power-of-two growth strategy. P11
should compare a shrinking ring against the current implementation
to see which wins on the metric.

### LFU (P2)

LFU frequencies are **capped at `1<<16`** to prevent counter
overflow. A key accessed more than 65,536 times in the no-decay
limit is held at 65,536; with the default `decay_lambda=0.05`
(age every 20 accesses), the cap is unreachable in practice
because halving keeps frequencies far below it. The cap is
documented in `cache/policy/lfu.go` as `maxLFUFreq`.

LFU decay is **amortised ageing, not a background decay**. The
trade-off is documented in `docs/DECISIONS.md` (P2 row 1): a
background goroutine would race with `Reset`/`Rebuild` and add
thread-management overhead to a layer that should own no threads.

## ARC and W-TinyLFU fidelity

To be documented in P4 when those policies land. The P2 phase card
forbids a "fake ARC" (a recency+frequency score labelled as ARC) and
requires the same honesty for W-TinyLFU.

## Trace data provenance

Never present synthetic data as real-world data. Clearly label synthetic traces and any substitutes for unavailable datasets.

## Distributed cache scope

To be documented as the scope is defined.

## Statistical methods

To be documented with the experiment methodology.
