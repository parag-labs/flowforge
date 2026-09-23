// Command worker is the FlowForge reference worker in Go. It polls the engine for a
// task, runs the task's side effect (here a pluggable stub), and reports the result.
// Because completion is keyed by the engine's idempotency key, a duplicate delivery or
// a retried attempt never applies the effect twice.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type claimResp struct {
	WorkflowID     string `json:"workflow_id"`
	TaskID         string `json:"task_id"`
	Attempt        int    `json:"attempt"`
	IdempotencyKey string `json:"idempotency_key"`
}

func main() {
	engine := envOr("FLOWFORGE_ENGINE", "http://localhost:8080")
	worker := envOr("FLOWFORGE_WORKER", "worker-go")
	poll := 500 * time.Millisecond

	log.Printf("%s polling %s", worker, engine)
	for {
		task, ok := claim(engine, worker)
		if !ok {
			time.Sleep(poll)
			continue
		}
		result, err := execute(task)
		if err != nil {
			report(engine, "/fail", map[string]any{
				"workflow_id": task.WorkflowID, "task_id": task.TaskID,
			})
			log.Printf("task %s/%s attempt %d failed: %v", task.WorkflowID, task.TaskID, task.Attempt, err)
			continue
		}
		report(engine, "/complete", map[string]any{
			"workflow_id":     task.WorkflowID,
			"task_id":         task.TaskID,
			"idempotency_key": task.IdempotencyKey,
			"result":          result,
		})
		log.Printf("task %s/%s attempt %d -> %s", task.WorkflowID, task.TaskID, task.Attempt, result)
	}
}

func claim(engine, worker string) (claimResp, bool) {
	body, _ := json.Marshal(map[string]string{"worker": worker})
	resp, err := http.Post(engine+"/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		return claimResp{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return claimResp{}, false
	}
	var task claimResp
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return claimResp{}, false
	}
	return task, true
}

// execute runs the task's effect. In this reference worker it succeeds deterministically;
// real workers dispatch on task_id. The result string is what the engine caches under
// the idempotency key.
func execute(task claimResp) (string, error) {
	return "done:" + task.TaskID, nil
}

func report(engine, path string, payload map[string]any) {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(engine+path, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("report %s failed: %v", path, err)
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
