package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The engine's core must reproduce the committed contract vectors exactly, the same
// discipline every language worker follows.
type contract struct {
	Idempotency []struct {
		WorkflowID string `json:"workflow_id"`
		TaskID     string `json:"task_id"`
		Attempt    int    `json:"attempt"`
		Key        string `json:"key"`
	} `json:"idempotency"`
	BackoffMs map[string]int `json:"backoff_ms"`
}

func loadContract(t *testing.T) contract {
	t.Helper()
	// contracts/ lives at the repo root, four levels up from this package.
	path := filepath.Join("..", "..", "..", "contracts", "vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var c contract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	return c
}

func TestConformanceIdempotency(t *testing.T) {
	for _, v := range loadContract(t).Idempotency {
		if got := IdempotencyKey(v.WorkflowID, v.TaskID, v.Attempt); got != v.Key {
			t.Errorf("key(%s,%s,%d)=%s want %s", v.WorkflowID, v.TaskID, v.Attempt, got, v.Key)
		}
	}
}

func TestConformanceBackoff(t *testing.T) {
	for attemptStr, want := range loadContract(t).BackoffMs {
		attempt, _ := strconv.Atoi(attemptStr)
		if got := BackoffDelayMs(attempt); got != want {
			t.Errorf("backoff(%d)=%d want %d", attempt, got, want)
		}
	}
}
