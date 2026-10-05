# Chaos tests

The failure scenarios FlowForge is built to survive, run as real programs that produce
evidence — not sentences in a README.

## crash_recovery — worker dies mid-task

The signature scenario. A worker claims a money-moving task, crashes before completing
it, and the system recovers without charging the card twice.

```
go run ./chaos-tests/crash_recovery
```

Captured run:

```
FlowForge chaos test — crash mid-task, recover via lease expiry

  30.720 submitted wf-chaos with task charge-card
  30.721 worker-A claimed charge-card (attempt 1, key c8decad26aa04363)
  30.721 worker-A CRASHES before completing (no /complete sent)
  30.722 worker-B tries to claim -> nothing available (task still leased to A)
  30.722 waiting 2s for worker-A's lease to expire...
  33.723 worker-B reclaimed charge-card (attempt 2, key c8decbd26aa04516)
  33.723 recovered attempt has a distinct idempotency key -> a retry, not a duplicate
  33.723 worker-B completed charge-card -> charged:100.00 (duplicate=false)
  33.723 duplicate completion arrives late -> engine returns duplicate=true, card NOT charged twice

RESULT: task survived a worker crash and completed exactly once. PASS
```

## What it proves

| Failure | Expected result | Shown |
|---------|-----------------|-------|
| Worker crashes mid-task | Task is recovered, not lost | lease expiry returns it to PENDING |
| Task ownership recovery | Another worker picks it up | worker-B claims on attempt 2 |
| Duplicate delivery | No duplicate business action | late `/complete` returns `duplicate=true` |
| Retried effect | Distinct from the original | attempt 2 has a different idempotency key |

The lease is set to 2 seconds here so the test runs fast; in production it's tens of
seconds. The mechanism is identical — see [ADR-002](../architecture/ADR/002-idempotency.md).
