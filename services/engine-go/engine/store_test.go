package engine

import (
	"testing"
	"time"

	"github.com/parag-labs/flowforge/services/engine-go/core"
)

func TestClaimCompleteFlow(t *testing.T) {
	s := NewStore(30)
	s.AddTask(core.NewTask("wf-1", "validate", 3))

	task, key, err := s.Claim("worker-a")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if task.State != core.Leased {
		t.Fatalf("claimed task should be leased, got %s", task.State)
	}
	if _, dup, err := s.Complete("wf-1", "validate", key, "ok"); err != nil || dup {
		t.Fatalf("first complete: err=%v dup=%v", err, dup)
	}
}

func TestDuplicateCompletionIsNoOp(t *testing.T) {
	s := NewStore(30)
	s.AddTask(core.NewTask("wf-1", "charge", 3))
	_, key, _ := s.Claim("worker-a")

	if _, dup, _ := s.Complete("wf-1", "charge", key, "charged:42"); dup {
		t.Fatal("first completion must not be a duplicate")
	}
	// A duplicate delivery of the SAME attempt's key: the effect must not re-apply.
	result, dup, err := s.Complete("wf-1", "charge", key, "charged:42")
	if err != nil {
		t.Fatalf("duplicate complete errored: %v", err)
	}
	if !dup {
		t.Fatal("second completion with same key must report duplicate")
	}
	if result != "charged:42" {
		t.Fatalf("duplicate should return cached result, got %q", result)
	}
}

func TestCrashedWorkerLeaseIsReclaimed(t *testing.T) {
	s := NewStore(30)
	s.AddTask(core.NewTask("wf-1", "process", 5))

	// Worker A claims, then "crashes" - it never completes.
	clock := time.Now()
	s.now = func() time.Time { return clock }
	taskA, keyA, _ := s.Claim("worker-a")
	if taskA.Attempt != 1 {
		t.Fatalf("first attempt should be 1, got %d", taskA.Attempt)
	}

	// Before the lease expires, the task is not claimable by anyone else.
	if _, _, err := s.Claim("worker-b"); err != ErrNoTask {
		t.Fatalf("task should not be claimable while leased, got %v", err)
	}

	// Lease expires; worker B can now claim the same task on a fresh attempt.
	clock = clock.Add(31 * time.Second)
	taskB, keyB, err := s.Claim("worker-b")
	if err != nil {
		t.Fatalf("worker B should reclaim the expired task: %v", err)
	}
	if taskB.Attempt != 2 {
		t.Fatalf("reclaim should be attempt 2, got %d", taskB.Attempt)
	}
	if keyA == keyB {
		t.Fatal("reclaimed attempt must have a distinct idempotency key")
	}
}

func TestRetryThenDeadLetter(t *testing.T) {
	s := NewStore(30)
	s.AddTask(core.NewTask("wf-1", "flaky", 1)) // 1 initial + 1 retry

	_, _, _ = s.Claim("w")
	if st, _ := s.Fail("wf-1", "flaky"); st != core.Pending {
		t.Fatalf("first fail want PENDING, got %s", st)
	}
	_, _, _ = s.Claim("w")
	if st, _ := s.Fail("wf-1", "flaky"); st != core.DeadLetter {
		t.Fatalf("second fail want DEAD_LETTER, got %s", st)
	}
}
