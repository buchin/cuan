package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestWorkerHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		AppVersion: "0.1.0",
		CommitSHA:  "workerhealth123",
	}
	s := NewServer(cfg, nil)

	// 1. Initial State: No runner registered yet -> status should be "attention"
	reqInitial := httptest.NewRequest(http.MethodGet, "/api/system/worker-health", nil)
	rrInitial := httptest.NewRecorder()
	s.Router().ServeHTTP(rrInitial, reqInitial)

	if rrInitial.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/system/worker-health, got %d: %s", rrInitial.Code, rrInitial.Body.String())
	}

	var healthResp WorkerHealthResponse
	if err := json.Unmarshal(rrInitial.Body.Bytes(), &healthResp); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if healthResp.Status != "attention" {
		t.Errorf("expected status 'attention' when no runner is active, got '%s'", healthResp.Status)
	}
	if healthResp.IsAlive {
		t.Errorf("expected is_alive false when no runner is active")
	}
	if healthResp.QueuedCount != 0 || healthResp.RunningCount != 0 || healthResp.QueueDepth != 0 {
		t.Errorf("expected empty queue, got queued=%d running=%d depth=%d",
			healthResp.QueuedCount, healthResp.RunningCount, healthResp.QueueDepth)
	}

	// 2. Register a runner heartbeat
	ctx := context.Background()
	hbReq := runner.HeartbeatRequest{
		RunnerID: "cuan-runner",
		Hostname: "vps-cuan-runner-01",
		CPUPercent: 12.5,
	}
	_, err := s.runnerSvc.Heartbeat(ctx, hbReq, 60*time.Second)
	if err != nil {
		t.Fatalf("failed to register runner heartbeat: %v", err)
	}

	// 3. Test GET /api/system/worker-health with active runner -> status "ready"
	reqActive := httptest.NewRequest(http.MethodGet, "/api/system/worker-health", nil)
	rrActive := httptest.NewRecorder()
	s.Router().ServeHTTP(rrActive, reqActive)

	if rrActive.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from active runner check, got %d: %s", rrActive.Code, rrActive.Body.String())
	}

	var activeResp WorkerHealthResponse
	if err := json.Unmarshal(rrActive.Body.Bytes(), &activeResp); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if activeResp.Status != "ready" {
		t.Errorf("expected status 'ready' for active idle runner, got '%s'", activeResp.Status)
	}
	if !activeResp.IsAlive {
		t.Errorf("expected is_alive true")
	}
	if activeResp.RunnerID != "cuan-runner" {
		t.Errorf("expected runner_id 'cuan-runner', got '%s'", activeResp.RunnerID)
	}
	if activeResp.LastSeenSecondsAgo < 0 || activeResp.LastSeenSecondsAgo > 5 {
		t.Errorf("expected last_seen_seconds_ago between 0 and 5, got %d", activeResp.LastSeenSecondsAgo)
	}

	// 4. Create a queued task and verify queue depth updates
	newTask := &task.Task{
		ID:        "tsk_test_worker_health",
		Title:     "Generate Rekap Penjualan",
		Status:    task.TaskStatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.taskSvc.Store().CreateTask(ctx, newTask); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	reqQueue := httptest.NewRequest(http.MethodGet, "/api/system/worker-health", nil)
	rrQueue := httptest.NewRecorder()
	s.Router().ServeHTTP(rrQueue, reqQueue)

	var queueResp WorkerHealthResponse
	_ = json.Unmarshal(rrQueue.Body.Bytes(), &queueResp)
	if queueResp.QueuedCount != 1 || queueResp.QueueDepth != 1 {
		t.Errorf("expected queued_count=1, queue_depth=1, got queued=%d depth=%d", queueResp.QueuedCount, queueResp.QueueDepth)
	}

	// 5. Change task to doing (running) -> status should become "busy"
	newTask.Status = task.TaskStatusDoing
	_ = s.taskSvc.Store().UpdateTask(ctx, newTask)

	reqBusy := httptest.NewRequest(http.MethodGet, "/api/system/worker-health", nil)
	rrBusy := httptest.NewRecorder()
	s.Router().ServeHTTP(rrBusy, reqBusy)

	var busyResp WorkerHealthResponse
	_ = json.Unmarshal(rrBusy.Body.Bytes(), &busyResp)
	if busyResp.RunningCount != 1 {
		t.Errorf("expected running_count=1, got %d", busyResp.RunningCount)
	}
	if busyResp.Status != "busy" {
		t.Errorf("expected status 'busy' when task is running, got '%s'", busyResp.Status)
	}

	// 6. Test /api/v1/system/worker-health alias
	reqV1 := httptest.NewRequest(http.MethodGet, "/api/v1/system/worker-health", nil)
	rrV1 := httptest.NewRecorder()
	s.Router().ServeHTTP(rrV1, reqV1)

	if rrV1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/system/worker-health alias, got %d: %s", rrV1.Code, rrV1.Body.String())
	}
	var v1Resp WorkerHealthResponse
	_ = json.Unmarshal(rrV1.Body.Bytes(), &v1Resp)
	if v1Resp.Status != "busy" || v1Resp.RunnerID != "cuan-runner" {
		t.Errorf("unexpected response from v1 alias: %+v", v1Resp)
	}
}
