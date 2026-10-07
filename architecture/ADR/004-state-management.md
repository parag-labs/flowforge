# ADR-004: Task ownership by lease, state in Postgres

Status: accepted
Author: Parag Sawant

## Context

Two questions decide how the engine survives failure: how does exactly one worker own a
task at a time, and where does durable task state live so nothing is lost when a worker
— or the engine — restarts?

## Decision

- **Ownership by time-bounded lease.** Claiming a task leases it to one worker for
  `lease_seconds`. If the worker doesn't complete or renew within that window, the lease
  expires and the engine returns the task to `PENDING` for another worker.
- **State in Postgres** (the reference build keeps an in-memory store with the same
  interface, so the engine logic is identical and the stack runs with zero setup).

## Why leases, not locks

A distributed lock (e.g. in Redis) would also give single ownership, but a crashed
holder leaves a lock that needs a separate expiry mechanism anyway — so you've
reinvented a lease with more moving parts. A lease *is* a lock with a built-in
expiry, which is exactly the failure mode we care about: a worker that dies holding
work. The attempt counter (ADR-002) advances on each re-claim, so the recovered run
gets a fresh idempotency key and the retry is safe.

## Why Postgres for state

- **Transactional claim.** Claiming a task — check it's pending, set it leased, bump the
  attempt — is a single row update guarded by optimistic concurrency (a version/state
  check in the `WHERE` clause). Two workers racing to claim the same task: exactly one
  wins, the other's update affects zero rows and it moves on. No external lock needed.
- **Durability and queryability.** Task state, leases, and completed idempotency keys
  are relational and small; Postgres gives crash-durable storage and lets the operator
  console query live state directly.
- **Boring on purpose.** State-store choices are where clever costs you at 3am. Postgres
  is well understood, easy to run, and more than fast enough at this scale.

## Trade-offs

- **Lease tuning.** Too short and a slow-but-alive worker loses its task to a
  double-run (safe, thanks to idempotency, but wasteful); too long and recovery from a
  real crash is slow. It's a knob, defaulted to tens of seconds and set per workflow.
- **Postgres as a scaling ceiling.** A single primary caps write throughput. Mitigated
  by partitioning work (ADR-003) and, if ever needed, sharding the state store by
  `workflow_id` — the same key the log already partitions on.
- **Clock dependence.** Lease expiry compares timestamps. The engine owns the clock (it
  stamps and checks expiry), so worker clock skew is irrelevant; only the engine's clock
  matters.

## Consequences

Lease + transactional claim + Postgres durability is the recovery backbone the chaos
test exercises: kill a worker, its lease lapses, another worker claims the still-durable
task and finishes it. Nothing is lost, nothing runs twice.
