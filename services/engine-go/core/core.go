// Package core holds FlowForge's language-neutral rules: the task state machine, the
// idempotency-key derivation, and the retry backoff. Everything here is deterministic
// and matches contracts/vectors.json so every language worker agrees with the engine.
package core

import "fmt"

// TaskState is where a task sits in its lifecycle.
type TaskState string

const (
	Pending    TaskState = "PENDING"
	Leased     TaskState = "LEASED"
	Completed  TaskState = "COMPLETED"
	DeadLetter TaskState = "DEAD_LETTER"
)

// Backoff parameters. Defaults match the contract; the engine can override them per
// workflow, but the contract vectors pin these values.
const (
	BaseMs      = 200
	MaxMs       = 30000
	MaxAttempts = 5
)

const (
	fnv64Offset uint64 = 0xCBF29CE484222325
	fnv64Prime  uint64 = 0x100000001B3
)

// Fnv1a64 is FNV-1a over the UTF-8 bytes of s. Shared with every worker so the
// idempotency keys line up across languages.
func Fnv1a64(s string) uint64 {
	h := fnv64Offset
	for _, b := range []byte(s) {
		h ^= uint64(b)
		h *= fnv64Prime
	}
	return h
}

// IdempotencyKey is derived only from durable identifiers - never wall-clock, random,
// or worker id - so a duplicate delivery of the same attempt collapses to a no-op.
func IdempotencyKey(workflowID, taskID string, attempt int) string {
	return fmt.Sprintf("%016x", Fnv1a64(fmt.Sprintf("%s|%s|%d", workflowID, taskID, attempt)))
}

// BackoffDelayMs is deterministic exponential backoff, capped at MaxMs.
func BackoffDelayMs(attempt int) int {
	d := BaseMs
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= MaxMs {
			return MaxMs
		}
	}
	if d > MaxMs {
		return MaxMs
	}
	return d
}
