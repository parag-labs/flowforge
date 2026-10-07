# ADR-002: Idempotency by derived key, not exactly-once transport

Status: accepted
Author: Parag Sawant

## Context

The event log (ADR-001) gives at-least-once delivery: a task can be delivered more
than once, and a worker can crash after doing its side effect but before recording
that it did. Exactly-once *delivery* is famously not a thing you can buy from a message
transport. So the system has to make duplicate execution *safe* rather than pretend it
can't happen.

## Decision

Every task attempt carries a deterministic **idempotency key**, and the engine records
the key of every completed effect. A completion that presents an already-seen key is a
no-op that returns the cached result.

```
idempotency_key = fnv1a_64(workflow_id + "|" + task_id + "|" + attempt)  →  16-hex
```

The key is derived only from durable identifiers — never wall-clock time, a random
value, or the worker's id — so any worker in any language computes the same key for the
same attempt.

## Why these three inputs

- **workflow_id + task_id** identify *which* effect this is. Two different tasks, or the
  same task in two different workflow runs, get different keys — they're genuinely
  different side effects.
- **attempt** is the subtle one. A *duplicate delivery of the same attempt* must
  collapse to one effect, so the attempt number is fixed within an attempt. But a
  *legitimate retry after a failure* is a new, intended effect (the first try didn't
  take), so incrementing the attempt gives it a fresh key. Putting `attempt` in the key
  is what lets the system tell "the network delivered this twice" apart from "this
  genuinely needs to run again."

## Trade-offs

- **The effect must be de-dupable at the boundary.** For an internal state write the
  engine's key store is enough. For an external side effect (charging a card, calling a
  third-party API) the downstream must accept the idempotency key too — which every
  serious payments/API provider supports for exactly this reason. FlowForge passes the
  key through so a worker can forward it.
- **Key storage grows with completed tasks.** Bounded in practice by retaining keys only
  as long as a duplicate could plausibly arrive (the retention window of the log), then
  ageing them out.
- **Not a distributed transaction.** We get "the effect runs at least once and is
  recorded at most once," not a two-phase commit across the effect and the state store.
  That's the honest guarantee, and it's the one durable-execution systems actually make.

## Consequences

Combined with lease-based recovery (ADR-004), this is what makes a worker crash
survivable: the recovered attempt gets a new key, runs, and any late duplicate of
either attempt is absorbed. The chaos test in `chaos-tests/` demonstrates exactly this.
