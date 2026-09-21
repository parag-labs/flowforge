package core

import "errors"

// ErrInvalidTransition is returned when a caller asks for a move the state machine
// does not allow - the guard that keeps a crashed or duplicate worker from corrupting
// a task's lifecycle.
var ErrInvalidTransition = errors.New("invalid task transition")

// Task is the durable unit of work. The engine persists it; workers act on a copy and
// report the resulting transition back.
type Task struct {
	WorkflowID string
	TaskID     string
	State      TaskState
	Attempt    int // attempts started so far; the first claim makes this 1
	MaxRetries int
}

// NewTask creates a pending task with the given retry budget.
func NewTask(workflowID, taskID string, maxRetries int) *Task {
	return &Task{
		WorkflowID: workflowID,
		TaskID:     taskID,
		State:      Pending,
		Attempt:    0,
		MaxRetries: maxRetries,
	}
}

// Claim leases a pending task to a worker, starting a new attempt. Only a PENDING task
// may be claimed; anything else is a duplicate or out-of-order claim and is rejected.
func (t *Task) Claim() error {
	if t.State != Pending {
		return ErrInvalidTransition
	}
	t.State = Leased
	t.Attempt++
	return nil
}

// Complete marks a leased task done.
func (t *Task) Complete() error {
	if t.State != Leased {
		return ErrInvalidTransition
	}
	t.State = Completed
	return nil
}

// Fail handles a failed attempt: if retries remain the task goes back to PENDING for
// another worker (after BackoffDelayMs), otherwise it moves to the dead-letter queue.
// Returns the next state so the caller can schedule a retry or alert on the DLQ.
func (t *Task) Fail() (TaskState, error) {
	if t.State != Leased {
		return t.State, ErrInvalidTransition
	}
	if t.Attempt <= t.MaxRetries {
		t.State = Pending
	} else {
		t.State = DeadLetter
	}
	return t.State, nil
}

// ExpireLease returns a leased task to PENDING when its worker's lease runs out - the
// recovery path for a crashed worker. The attempt counter is preserved so the next
// claim increments it, giving the retried effect a fresh idempotency key.
func (t *Task) ExpireLease() error {
	if t.State != Leased {
		return ErrInvalidTransition
	}
	t.State = Pending
	return nil
}

// IdempotencyKey is the key for the task's current attempt.
func (t *Task) IdempotencyKey() string {
	return IdempotencyKey(t.WorkflowID, t.TaskID, t.Attempt)
}
