# FlowForge event & idempotency contract

Every worker — Go, Python, Rust, C#, Java, TypeScript — speaks this one protocol. The
engine owns durable state; workers claim tasks, execute them, and report results. The
contract is what makes a Python worker and a Go worker interchangeable against the same
workflow, the same way an SDK works in Temporal or Cadence.

## Task lifecycle

```
PENDING ──claim──▶ LEASED ──complete──▶ COMPLETED
   ▲                  │
   │                  ├──fail (retries left)──▶ PENDING (backoff)
   └──lease expires───┘
                      └──fail (no retries)────▶ DEAD_LETTER
```

A task is `LEASED` to exactly one worker for `lease_seconds`. If the worker crashes,
the lease expires and the engine returns the task to `PENDING` so another worker can
claim it. Because execution is idempotent (below), re-running a partially-done task is
safe.

## Idempotency key

The heart of at-least-once delivery. Every side effect a worker performs is tagged
with a deterministic key so a duplicate execution is a no-op. The key is derived only
from durable identifiers — never from wall-clock time, random values, or worker id —
so any worker, in any language, computes the **same** key for the same task attempt:

```
idempotency_key = fnv1a_64( workflow_id + "|" + task_id + "|" + attempt )  →  16-hex
```

- `workflow_id` — the run this task belongs to (ULID/uuid string)
- `task_id` — the task node within the workflow definition
- `attempt` — the retry attempt number, starting at 1

The engine stores each completed key; a worker that presents an already-seen key gets
the cached result instead of running the effect twice. `attempt` is included so a
legitimate retry after a *failure* is a new effect, while a duplicate delivery of the
*same* attempt collapses.

## Retry & backoff

Deterministic exponential backoff, so every language schedules the same next-attempt
delay:

```
delay_ms(attempt) = min( base_ms * 2^(attempt-1), max_ms )
```

Defaults: `base_ms = 200`, `max_ms = 30000`. After `max_attempts` (default 5) the task
moves to the dead-letter queue.

## Conformance

`contracts/vectors.json` pins the idempotency keys and backoff delays for a fixed set
of inputs. Every worker ships a test that reproduces them exactly — if any language
disagrees, the workers are not interchangeable and CI fails. Same discipline as the
consistent-hash golden vectors.
