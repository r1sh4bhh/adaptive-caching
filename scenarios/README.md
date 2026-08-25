# Scenarios

Scenario YAML files are consumed by **both** the benchmark harness and the live demo: one artifact, two uses. The UI never generates data — it selects a `(scenario, seed)` and the backend produces the stream.

The following files are planned for P3; these examples document their intended shape but do not create the files yet.

## `showcase.yaml`

```yaml
name: "adaptation showcase"
seed: 42
capacity: 100MB
sizeDistribution: heterogeneous
segments:
  - {workload: temporal,    requests: 20000, uniqueKeys: 5000}
  - {workload: random,      requests: 20000, uniqueKeys: 50000}
  - {workload: bursty,      requests: 15000, hotKeys: 20, spikeFactor: 12}
  - {workload: working_set, requests: 20000, setSize: 500}
  - {workload: temporal,    requests: 20000, uniqueKeys: 5000}
```

## `adversarial_oscillation.yaml`

This scenario transitions every 500 requests, below `min_residency=2000`.

```yaml
name: "adversarial oscillation"
transitionEvery: 500
minResidency: 2000
```

**The expected outcome is that adaptive UNDERPERFORMS plain LRU due to switching cost. This is a RESULT, not a bug.**
