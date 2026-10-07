// Command load drives the FlowForge engine at concurrency and reports measured
// throughput and latency. It runs the real engine over HTTP in-process, seeds a batch
// of workflows, then hammers the claim/complete cycle from many worker goroutines.
//
// Numbers are whatever this machine produces — the point is a reproducible method, not
// a marketing figure. Run: go run ./load-tests/throughput
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/parag-labs/flowforge/services/engine-go/engine"
)

const (
	workflows   = 2000
	tasksPer    = 5
	workers     = 32
	leaseSecond = 30
)

func main() {
	store := engine.NewStore(leaseSecond)
	srv := httptest.NewServer(engine.NewAPI(store).Routes())
	defer srv.Close()
	client := &http.Client{}

	total := workflows * tasksPer
	fmt.Printf("FlowForge load test\n  %d workflows x %d tasks = %d tasks, %d workers\n\n",
		workflows, tasksPer, total, workers)

	// Seed workflows.
	seedStart := time.Now()
	for i := 0; i < workflows; i++ {
		tasks := make([]string, tasksPer)
		for t := range tasks {
			tasks[t] = fmt.Sprintf("task-%d", t)
		}
		body, _ := json.Marshal(map[string]any{
			"workflow_id": fmt.Sprintf("wf-%05d", i), "tasks": tasks, "max_retries": 3,
		})
		resp, _ := client.Post(srv.URL+"/workflows", "application/json", bytes.NewReader(body))
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	fmt.Printf("  seeded in %s\n", time.Since(seedStart).Round(time.Millisecond))

	// Drain: workers loop claim -> complete until the engine has no pending tasks.
	var completed atomic.Int64
	latencies := make([][]time.Duration, workers)
	var wg sync.WaitGroup
	start := time.Now()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("worker-%d", id)
			for {
				t0 := time.Now()
				task, ok := claim(client, srv.URL, name)
				if !ok {
					// No task claimed; if everything's done, stop, else keep polling.
					if completed.Load() >= int64(total) {
						return
					}
					continue
				}
				complete(client, srv.URL, task)
				latencies[id] = append(latencies[id], time.Since(t0))
				if completed.Add(1) >= int64(total) {
					return
				}
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// Aggregate latencies.
	var all []time.Duration
	for _, l := range latencies {
		all = append(all, l...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	fmt.Printf("\n  drained %d tasks in %s\n", completed.Load(), elapsed.Round(time.Millisecond))
	fmt.Printf("  throughput : %.0f tasks/sec\n", float64(completed.Load())/elapsed.Seconds())
	if len(all) > 0 {
		fmt.Printf("  latency p50: %s\n", all[len(all)*50/100].Round(time.Microsecond))
		fmt.Printf("  latency p95: %s\n", all[len(all)*95/100].Round(time.Microsecond))
		fmt.Printf("  latency p99: %s\n", all[len(all)*99/100].Round(time.Microsecond))
		fmt.Printf("  latency max: %s\n", all[len(all)-1].Round(time.Microsecond))
	}
}

type task struct {
	WorkflowID     string `json:"workflow_id"`
	TaskID         string `json:"task_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func claim(c *http.Client, base, worker string) (task, bool) {
	body, _ := json.Marshal(map[string]string{"worker": worker})
	resp, err := c.Post(base+"/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		return task{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return task{}, false
	}
	var t task
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return task{}, false
	}
	return t, true
}

func complete(c *http.Client, base string, t task) {
	body, _ := json.Marshal(map[string]any{
		"workflow_id": t.WorkflowID, "task_id": t.TaskID,
		"idempotency_key": t.IdempotencyKey, "result": "ok",
	})
	resp, err := c.Post(base+"/complete", "application/json", bytes.NewReader(body))
	if err == nil {
		_ = resp.Body.Close()
	}
}
