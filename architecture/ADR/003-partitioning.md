# ADR-003: Partition the log by workflow_id

Status: accepted
Author: Parag Sawant

## Context

The event log is partitioned, and we have to choose the partition key. That key decides
two things at once: what stays ordered, and how load spreads across consumers. Getting
it wrong means either lost ordering guarantees or hot partitions.

## Decision

Partition by **`workflow_id`**.

## Why

- **Ordering where it matters, nowhere it doesn't.** A single workflow's tasks often
  have real ordering constraints (validate before charge, charge before ship). Putting
  all of one workflow's events on one partition keeps them ordered without any global
  lock. Across *different* workflows there's no ordering requirement, so they spread
  freely across partitions and consumers.
- **Natural parallelism unit.** Workflows are independent, so they're the right thing to
  shard on. Throughput scales by adding partitions and workers; each workflow is handled
  start-to-finish within its partition's ordering.
- **Even spread.** `workflow_id`s are high-cardinality and effectively random (ULIDs or
  UUIDs), so hashing them onto partitions balances load well — the same reasoning behind
  the consistent-hashing work in a sibling repo.

## Alternatives considered

- **Partition by task type.** Would let you scale hot task types independently, but it
  scatters a single workflow's tasks across partitions and destroys per-workflow
  ordering. Rejected.
- **Partition by tenant.** Good isolation, but a big tenant becomes a hot partition and a
  small one wastes a partition. Tenancy is better handled as a quota/rate-limit concern
  than a partitioning one.

## Trade-offs

- **A single workflow can't exceed one partition's throughput.** Acceptable: workflows
  are modest in size; it's the *number* of concurrent workflows that scales, and that's
  bounded by total partitions, not per-workflow ones.
- **A pathologically hot workflow** (one workflow with millions of tasks) would hot-spot
  its partition. If that ever became real, the fix is sub-partitioning by
  `workflow_id + task_group` — but designing for it now would be premature.

## Consequences

Partition count becomes the main horizontal-scaling lever: more partitions, more
parallel workflows, more workers. Rebalancing partitions across workers is handled by
the log's consumer-group protocol, not by FlowForge.
