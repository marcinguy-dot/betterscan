# Speed Benchmarks

Benchmarks for Rust and Go runners using the Broken-Vulnerable-Code-Snippets
dataset (https://github.com/snoopysecurity/Broken-Vulnerable-Code-Snippets).
Note that this dataset contains broken snippets and is not a reliable benchmark.

## Environment
- Host OS: darwin 25.2.0
- CPU cores: 12
- OpenGrep: 1.15.1
- Rules: Aikido + Amplify (refreshed by runners)
- Dataset: `/Users/marcinkozlowski/cmcb/broken-vulnerable-code-snippets`

## Strategies
- `sequential`: run tools one-by-one with internal tool jobs
- `parallel`: run tools concurrently with a core-limited worker pool

## Results

Timing results for this dataset are not recorded yet.
