// Command crash_recovery is FlowForge's signature chaos test. It reproduces the
// scenario the whole design exists to survive:
//
//	worker crashes mid-task -> lease expires -> another worker claims -> idempotency
//	check -> task safely resumes, with no duplicate side effect.
//
// It runs the real engine over HTTP in-process (no external infra), drives two
// workers, and prints a timeline of what happened. Run: go run ./chaos-tests/crash_recovery
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/parag-labs/flowforge/services/engine-go/engine"
)

func main() {
	// Short lease so a "crashed" worker's task recovers in seconds, not minutes.
	const leaseSeconds = 2
	store := engine.NewStore(leaseSeconds)
	srv := httptest.NewServer(engine.NewAPI(store).Routes())
	defer srv.Close()

	step := func(msg string) { fmt.Printf("  %-6s %s\n", time.Now().Format("05.000"), msg) }
	fmt.Print("FlowForge chaos test — crash mid-task, recover via lease expiry\n\n")

	// A workflow with a single money-moving task, so a double-execution would be a
	// real bug (a double charge).
	post(srv.URL, "/workflows", map[string]any{
		"workflow_id": "wf-chaos", "tasks": []string{"charge-card"}, "max_retries": 3,
	})
	step("submitted wf-chaos with task charge-card")

	// Worker A claims the task, then "crashes" — it never reports completion.
	claimA := claim(srv.URL, "worker-A")
	step(fmt.Sprintf("worker-A claimed charge-card (attempt %v, key %s)", claimA["attempt"], claimA["idempotency_key"]))
	step("worker-A CRASHES before completing (no /complete sent)")

	// While the lease is held, no other worker can take the task.
	if claim(srv.URL, "worker-B") == nil {
		step("worker-B tries to claim -> nothing available (task still leased to A)")
	} else {
		fail("worker-B should not have been able to claim a leased task")
	}

	// Wait for the lease to expire; the engine returns the task to PENDING.
	step(fmt.Sprintf("waiting %ds for worker-A's lease to expire...", leaseSeconds))
	time.Sleep((leaseSeconds + 1) * time.Second)

	// Worker B now claims the recovered task — a fresh attempt with a new key.
	claimB := claim(srv.URL, "worker-B")
	if claimB == nil {
		fail("worker-B should have reclaimed the expired task")
	}
	step(fmt.Sprintf("worker-B reclaimed charge-card (attempt %v, key %s)", claimB["attempt"], claimB["idempotency_key"]))

	if claimA["idempotency_key"] == claimB["idempotency_key"] {
		fail("recovered attempt must have a DIFFERENT idempotency key")
	}
	step("recovered attempt has a distinct idempotency key -> a retry, not a duplicate")

	// Worker B completes it. The card is charged exactly once.
	res := post(srv.URL, "/complete", map[string]any{
		"workflow_id": "wf-chaos", "task_id": "charge-card",
		"idempotency_key": claimB["idempotency_key"], "result": "charged:100.00",
	})
	step(fmt.Sprintf("worker-B completed charge-card -> %v (duplicate=%v)", res["result"], res["duplicate"]))

	// A late duplicate delivery of the SAME completion must be a no-op.
	dup := post(srv.URL, "/complete", map[string]any{
		"workflow_id": "wf-chaos", "task_id": "charge-card",
		"idempotency_key": claimB["idempotency_key"], "result": "charged:100.00",
	})
	if dup["duplicate"] != true {
		fail("a duplicate completion must be reported as a no-op")
	}
	step("duplicate completion arrives late -> engine returns duplicate=true, card NOT charged twice")

	fmt.Print("\nRESULT: task survived a worker crash and completed exactly once. PASS\n")
}

func claim(base, worker string) map[string]any {
	resp := rawPost(base, "/claim", map[string]any{"worker": worker})
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	_ = resp.Body.Close()
	return out
}

func post(base, path string, body map[string]any) map[string]any {
	resp := rawPost(base, path, body)
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	_ = resp.Body.Close()
	return out
}

func rawPost(base, path string, body map[string]any) *http.Response {
	b, _ := json.Marshal(body)
	resp, err := http.Post(base+path, "application/json", bytes.NewReader(b))
	if err != nil {
		fail(err.Error())
	}
	return resp
}

func fail(msg string) {
	fmt.Println("\nFAIL:", msg)
	os.Exit(1)
}
