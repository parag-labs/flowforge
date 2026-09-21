package core

import "testing"

func TestClaimOnlyFromPending(t *testing.T) {
	task := NewTask("wf-1", "validate", 3)
	if err := task.Claim(); err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}
	if task.State != Leased || task.Attempt != 1 {
		t.Fatalf("after claim want LEASED attempt 1, got %s attempt %d", task.State, task.Attempt)
	}
	// A second claim on an already-leased task is a duplicate and must be rejected.
	if err := task.Claim(); err != ErrInvalidTransition {
		t.Fatalf("double claim should be rejected, got %v", err)
	}
}

func TestCompleteHappyPath(t *testing.T) {
	task := NewTask("wf-1", "validate", 3)
	_ = task.Claim()
	if err := task.Complete(); err != nil {
		t.Fatalf("complete should succeed: %v", err)
	}
	if task.State != Completed {
		t.Fatalf("want COMPLETED, got %s", task.State)
	}
}

func TestFailRetriesThenDeadLetters(t *testing.T) {
	task := NewTask("wf-1", "charge", 2) // 1 initial + 2 retries = 3 attempts allowed
	// attempt 1 -> fail -> back to pending
	_ = task.Claim()
	if s, _ := task.Fail(); s != Pending {
		t.Fatalf("attempt 1 fail want PENDING, got %s", s)
	}
	// attempt 2 -> fail -> back to pending
	_ = task.Claim()
	if s, _ := task.Fail(); s != Pending {
		t.Fatalf("attempt 2 fail want PENDING, got %s", s)
	}
	// attempt 3 -> fail -> dead letter (retries exhausted)
	_ = task.Claim()
	if task.Attempt != 3 {
		t.Fatalf("want attempt 3, got %d", task.Attempt)
	}
	if s, _ := task.Fail(); s != DeadLetter {
		t.Fatalf("attempt 3 fail want DEAD_LETTER, got %s", s)
	}
}

func TestLeaseExpiryRecoversTask(t *testing.T) {
	task := NewTask("wf-1", "process", 5)
	_ = task.Claim() // attempt 1, leased to a worker that then "crashes"
	if err := task.ExpireLease(); err != nil {
		t.Fatalf("lease expiry should recover the task: %v", err)
	}
	if task.State != Pending {
		t.Fatalf("after expiry want PENDING, got %s", task.State)
	}
	// A new worker claims it: attempt increments, giving a fresh idempotency key.
	prevKey := IdempotencyKey("wf-1", "process", 1)
	_ = task.Claim()
	if task.Attempt != 2 {
		t.Fatalf("recovered claim want attempt 2, got %d", task.Attempt)
	}
	if task.IdempotencyKey() == prevKey {
		t.Fatal("retried attempt must have a different idempotency key")
	}
}

func TestInvalidTransitionsAreGuarded(t *testing.T) {
	task := NewTask("wf-1", "x", 1)
	if err := task.Complete(); err != ErrInvalidTransition {
		t.Fatalf("complete on pending should fail, got %v", err)
	}
	if _, err := task.Fail(); err != ErrInvalidTransition {
		t.Fatalf("fail on pending should fail, got %v", err)
	}
	if err := task.ExpireLease(); err != ErrInvalidTransition {
		t.Fatalf("expire on pending should fail, got %v", err)
	}
}
