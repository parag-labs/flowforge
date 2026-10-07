# Benchmarks

Measured on this machine, reproducible: `go run ./load-tests/throughput`. These are
the reference engine's numbers, not a marketing figure — the method matters more than
the absolute value, which will differ on your hardware.

## Method

- The real engine runs over HTTP in-process (`httptest` server).
- 2,000 workflows x 5 tasks = **10,000 tasks** are seeded.
- **32 worker goroutines** drain them via the real `claim` -> `complete` cycle, each a
  full JSON HTTP round trip.
- Throughput is tasks fully completed per second; latency is per claim+complete cycle.

## Results (reference in-memory engine)

```
2000 workflows x 5 tasks = 10000 tasks, 32 workers

seeded in 1.193s
drained 10000 tasks in 3.337s
throughput : 2997 tasks/sec
latency p50: 9.244ms
latency p95: 21.96ms
latency p99: 33.62ms
latency max: 59.776ms
```

| Metric | Value |
|--------|------:|
| Throughput | ~3,000 tasks/sec |
| Latency p50 | ~9 ms |
| Latency p99 | ~34 ms |

Each completed task is two HTTP round trips (claim + complete), so the engine is
serving roughly **6,000 requests/sec** end to end here.

## Honest bottleneck

The reference store keeps tasks in a map guarded by a single mutex, and `claim` does a
linear scan for the first pending task. At 10,000 in-flight tasks that scan under a
global lock is the throughput ceiling — not the state machine or the hashing, which are
trivial. That's a deliberate trade for a readable reference implementation.

The production path removes it two ways, both already in the design:

- **Partition by `workflow_id`** ([ADR-003](architecture/ADR/003-partitioning.md)) so no
  single lock or scan covers all tasks — throughput scales with partition count.
- **Postgres claim** ([ADR-004](architecture/ADR/004-state-management.md)) turns "find a
  pending task" into an indexed `SELECT ... FOR UPDATE SKIP LOCKED`, which is O(log n)
  and contention-free across workers, instead of an O(n) scan under one mutex.

So the reference number is a floor set by the simplest possible store, and the
architecture's scaling levers are the ones that matter — which is the honest thing to
report. Fabricated six-figure throughput would be easy and worthless; this is what the
code actually does today.
