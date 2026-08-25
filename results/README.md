# Benchmark results

Benchmark output lands here and is gitignored except for `.gitkeep` files and `summary.json` summaries. Do not commit raw benchmark output.

Every run directory must contain a manifest recording:

- `run_id`
- timestamp
- `git_commit`
- seed
- scenario
- byte capacity
- policy
- full resolved configuration
- feature flags
- Go version
- host information

Reported measurements must refer to a run ID and its reproducibility metadata. Never fabricate benchmark numbers.
