package engine

import (
	"encoding/json"
	"net/http"

	"github.com/parag-labs/flowforge/services/engine-go/core"
)

// API exposes the engine over HTTP: submit a workflow, claim/complete/fail tasks (the
// worker protocol), and read the current task states for the operator console.
type API struct {
	store *Store
}

// NewAPI wires an HTTP handler onto a store.
func NewAPI(store *Store) *API { return &API{store: store} }

// Routes returns the engine's HTTP mux.
func (a *API) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /workflows", a.submitWorkflow)
	mux.HandleFunc("POST /claim", a.claim)
	mux.HandleFunc("POST /complete", a.complete)
	mux.HandleFunc("POST /fail", a.fail)
	mux.HandleFunc("GET /tasks", a.tasks)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

type submitReq struct {
	WorkflowID string   `json:"workflow_id"`
	Tasks      []string `json:"tasks"`
	MaxRetries int      `json:"max_retries"`
}

func (a *API) submitWorkflow(w http.ResponseWriter, r *http.Request) {
	var req submitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.WorkflowID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	retries := req.MaxRetries
	if retries == 0 {
		retries = core.MaxAttempts - 1
	}
	for _, taskID := range req.Tasks {
		a.store.AddTask(core.NewTask(req.WorkflowID, taskID, retries))
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"workflow_id": req.WorkflowID, "tasks": len(req.Tasks)})
}

type claimReq struct {
	Worker string `json:"worker"`
}

func (a *API) claim(w http.ResponseWriter, r *http.Request) {
	var req claimReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Worker == "" {
		req.Worker = "anonymous"
	}
	task, key, err := a.store.Claim(req.Worker)
	if err != nil {
		writeJSON(w, http.StatusNoContent, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workflow_id":     task.WorkflowID,
		"task_id":         task.TaskID,
		"attempt":         task.Attempt,
		"idempotency_key": key,
	})
}

type completeReq struct {
	WorkflowID     string `json:"workflow_id"`
	TaskID         string `json:"task_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Result         string `json:"result"`
}

func (a *API) complete(w http.ResponseWriter, r *http.Request) {
	var req completeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	result, dup, err := a.store.Complete(req.WorkflowID, req.TaskID, req.IdempotencyKey, req.Result)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "duplicate": dup})
}

type failReq struct {
	WorkflowID string `json:"workflow_id"`
	TaskID     string `json:"task_id"`
}

func (a *API) fail(w http.ResponseWriter, r *http.Request) {
	var req failReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	state, err := a.store.Fail(req.WorkflowID, req.TaskID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": state})
}

func (a *API) tasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.store.Snapshot())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
