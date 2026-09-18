# ADR-001: An append-only event log as the task backbone

Status: accepted
Author: Parag Sawant

## Context

FlowForge dispatches workflow tasks to a fleet of interchangeable workers. Workers
crash, deploy mid-task, and occasionally process the same message twice. The transport
between the engine and the workers decides what guarantees the whole system can offer,
so it's the first decision to pin down.

The realistic options:

- **A classic task queue (RabbitMQ / Redis lists / SQS).** Simple, great at fan-out
  and per-message ack. But once a message is acked it's gone — there's no log to replay
  a workflow from, and ordering across a partition isn't guaranteed.
- **An append-only partitioned log (Kafka / Redpanda).** Messages persist after
  consumption, consumers track their own offset, and order is preserved within a
  partition. You get replay and independent consumer groups for free.

## Decision

Use an **append-only partitioned log** (Kafka, or Redpanda as a drop-in for local dev)
as the task backbone, partitioned by `workflow_id`.

## Why

- **Replay.** Workflow history is the log. Rebuilding a run's state, or reprocessing
  after a bug fix, is re-reading from an offset — not possible once a queue has acked
  and dropped the message.
- **Ordering where it matters.** Partitioning by `workflow_id` puts all of one
  workflow's events on one partition, so its tasks stay ordered without a global lock.
- **Independent consumers.** The lease daemon, the metrics pipeline, and the audit
  sink are separate consumer groups reading the same log at their own pace.
- **At-least-once is honest.** The log gives at-least-once delivery; we make that safe
  with idempotency (ADR-002) rather than pretending to have exactly-once transport.

## Trade-offs

- **Operational weight.** A log is heavier to run than a queue — partitions, consumer
  groups, retention. Mitigated in dev by Redpanda (Kafka API, single binary, no
  ZooKeeper).
- **No native per-message retry/delay.** A queue gives you visibility timeouts for
  free; here, retry scheduling and dead-lettering are the engine's job (ADR-002). That's
  a deliberate trade — we take explicit, testable retry logic over transport magic.
- **Partition count caps worker parallelism** for a single workflow. Acceptable:
  parallelism *across* workflows is what scales, and that's bounded by total partitions,
  not per-workflow ones.

## Consequences

Idempotency becomes mandatory, not optional — at-least-once means duplicates will
happen. That's ADR-002. Replay and versioning (how a running workflow handles a changed
definition) build directly on the log being the source of truth.
