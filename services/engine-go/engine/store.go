package engine

import (
	"errors"
	"sync"
	"time"

	"github.com/parag-labs/flowforge/services/engine-go/core"
)

// Store is the durable state the engine owns: workflows, their tasks, the leases held
// by workers, and the set of completed idempotency keys. This in-memory implementation
// is the reference; the Postgres-backed store implements the same interface so the
// engine logic never changes. Every method is safe for concurrent workers.
type Store struct {
	mu           sync.Mutex
	tasks        map[string]*core.Task // key: workflowID + "/" + taskID
	leases       map[string]lease      // key -> current lease
	completedKey map[string]string     // idempotency key -> cached result
	leaseSeconds int
	now          func() time.Time
}

type lease struct {
	worker  string
	expires time.Time
}

// ErrNoTask is returned when there is no claimable task.
var ErrNoTask = errors.New("no claimable task")

// NewStore builds an empty in-memory store with the given lease duration.
func NewStore(leaseSeconds int) *Store {
	return &Store{
		tasks:        make(map[string]*core.Task),
		leases:       make(map[string]lease),
		completedKey: make(map[string]string),
		leaseSeconds: leaseSeconds,
		now:          time.Now,
	}
}

func taskKey(workflowID, taskID string) string { return workflowID + "/" + taskID }

// AddTask registers a pending task with the engine.
func (s *Store) AddTask(t *core.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[taskKey(t.WorkflowID, t.TaskID)] = t
}

// Claim leases the first pending task to a worker and returns a copy plus the
// idempotency key for this attempt. Expired leases are reclaimed first so a crashed
// worker's task becomes available again.
func (s *Store) Claim(worker string) (*core.Task, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reclaimExpiredLocked()

	for key, t := range s.tasks {
		if t.State != core.Pending {
			continue
		}
		if err := t.Claim(); err != nil {
			continue
		}
		s.leases[key] = lease{worker: worker, expires: s.now().Add(time.Duration(s.leaseSeconds) * time.Second)}
		cp := *t
		return &cp, t.IdempotencyKey(), nil
	}
	return nil, "", ErrNoTask
}

// Complete records a task as done. If the idempotency key was already completed the
// call is a no-op that returns the cached result - this is what makes at-least-once
// delivery safe: a duplicate completion never double-applies the effect.
func (s *Store) Complete(workflowID, taskID, idempotencyKey, result string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cached, ok := s.completedKey[idempotencyKey]; ok {
		return cached, true, nil // duplicate - already applied
	}
	t, ok := s.tasks[taskKey(workflowID, taskID)]
	if !ok {
		return "", false, ErrNoTask
	}
	if err := t.Complete(); err != nil {
		return "", false, err
	}
	s.completedKey[idempotencyKey] = result
	delete(s.leases, taskKey(workflowID, taskID))
	return result, false, nil
}

// Fail reports a failed attempt; the task either goes back to pending for retry or to
// the dead-letter queue. Returns the resulting state.
func (s *Store) Fail(workflowID, taskID string) (core.TaskState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[taskKey(workflowID, taskID)]
	if !ok {
		return "", ErrNoTask
	}
	state, err := t.Fail()
	if err != nil {
		return state, err
	}
	delete(s.leases, taskKey(workflowID, taskID))
	return state, nil
}

// reclaimExpiredLocked returns any leased task whose lease has passed back to pending.
// Caller must hold the lock.
func (s *Store) reclaimExpiredLocked() {
	now := s.now()
	for key, l := range s.leases {
		if now.After(l.expires) {
			if t, ok := s.tasks[key]; ok && t.State == core.Leased {
				_ = t.ExpireLease()
			}
			delete(s.leases, key)
		}
	}
}

// Snapshot returns a read-only view of task states for the API / operator console.
func (s *Store) Snapshot() []core.Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]core.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, *t)
	}
	return out
}
