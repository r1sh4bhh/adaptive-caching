# Experiments

## Experiment matrix

The experiment matrix will cover policies × workloads × cache sizes × size distributions × seeds. Cache sizes represent byte capacity, not object counts.

## Reproducibility protocol

Every run records its seed, git commit, fully resolved configuration, Go version, and host.

## Ablation variants

Ablations are feature-flag configuration permutations, never code branches.

| Variant | Configuration |
|---|---|
| A | Fixed LRU (baseline) |
| B | Best fixed policy (hindsight) |
| C | Adaptive selection, no tuning |
| D | C + online tuning |
| E | C + size-aware eviction |
| F | Full system (C+D+E) |
| G | F + multi-tier |
| H | Bélády MIN oracle (upper bound) |
