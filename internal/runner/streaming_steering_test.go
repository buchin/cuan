package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

type mockStreamingSteeringAdapter struct {
	mu           sync.Mutex
	steeredText  string
	steerFunc    func(string) error
	executedReq  agent.ExecutionRequest
}

func (m *mockStreamingSteeringAdapter) Name() string { return "mock-stream-steer" }
func (m *mockStreamingSteeringAdapter) Version() (string, error) { return "1.0.0", nil }
func (m *mockStreamingSteeringAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	m.mu.Lock()
	m.executedReq = req
	m.mu.Unlock()

	// Register steering handle
	if req.RegisterSteer != nil {
		req.RegisterSteer(func(prompt string) error {
			m.mu.Lock()
			m.steeredText = prompt
			m.mu.Unlock()
			if req.OnLog != nil {
				req.OnLog("stdout", "Executed steering: "+prompt)
			}
			return nil
		})
	}

	// Stream initial lines
	if req.OnLog != nil {
		req.OnLog("stdout", "Starting streaming attempt")
		req.OnLog("stderr", "Compiling assets")
	}

	// Listen on SteerChan if provided
	if req.SteerChan != nil {
		select {
		case prompt := <-req.SteerChan:
			m.mu.Lock()
			m.steeredText = prompt
			m.mu.Unlock()
			if req.OnLog != nil {
				req.OnLog("stdout", "Received steer from chan: "+prompt)
			}
		case <-time.After(100 * time.Millisecond):
		}
	}

	return &agent.ExecutionResult{
		Status:   "SUCCESS",
		ExitCode: 0,
		Stdout:   "All tasks completed",
		Duration: 50 * time.Millisecond,
	}, nil
}

func TestRunner_LiveStreamingAndInteractiveSteering(t *testing.T) {
	var mu sync.Mutex
	var receivedStreamLogs []task.LogRecord
	attemptID := "att-runner-stream-test"

	// Mock Cuan API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/runner/heartbeat":
			// Heartbeat returns a steering directive for the active attempt
			resp := HeartbeatResponse{
				Status:           "ok",
				LeaseDurationSec: 30,
				SteeringDirectives: []SteeringDirective{
					{
						AttemptID: attemptID,
						Prompt:    "heartbeat steering instruction",
						Author:    "scrum-master",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case "/api/v1/runner/attempts/" + attemptID + "/logs":
			var req StreamLogsRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				mu.Lock()
				receivedStreamLogs = append(receivedStreamLogs, req.Logs...)
				mu.Unlock()
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "appended": len(req.Logs)})

		case "/api/v1/runner/attempts/" + attemptID + "/complete":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		RunnerID:        "test-stream-runner",
		RunnerServerURL: server.URL,
		RunnerInterval:  10 * time.Millisecond,
	}

	adapter := &mockStreamingSteeringAdapter{}
	r := NewRunner(cfg, adapter, server.Client())

	ctx := context.Background()

	// Simulate claiming and executing an attempt
	claim := ClaimTaskResponse{
		Claimed: true,
		Attempt: &task.TaskAttempt{
			ID:            attemptID,
			TaskID:        "task-123",
			AttemptNumber: 1,
			Status:        task.AttemptStatusRunning,
		},
		Task: &task.Task{
			ID:                "task-123",
			Title:             "Test Task",
			Instructions:      "Run live streaming test",
			ExecutionCategory: task.ExecutionCategoryOps,
		},
	}

	// Register in activeAttempts
	steerChan := make(chan string, 16)
	r.mu.Lock()
	r.activeAttempts[attemptID] = &activeAttempt{
		attempt:   claim.Attempt,
		lane:      task.ExecutionCategoryOps,
		steerChan: steerChan,
	}
	r.mu.Unlock()

	// 1. Test SteerAttempt direct injection before execution starts
	// When runAttempt starts, it will wire RegisterSteer to act.steerFunc
	go func() {
		// Wait until attempt is running
		for i := 0; i < 50; i++ {
			time.Sleep(10 * time.Millisecond)
			if err := r.SteerAttempt(attemptID, "direct steering prompt"); err == nil {
				return
			}
		}
	}()

	r.runAttempt(ctx, claim)

	// Verify that logs were streamed live
	mu.Lock()
	defer mu.Unlock()

	if len(receivedStreamLogs) == 0 {
		t.Fatalf("expected streamed logs to be received by server, got 0")
	}

	var foundInitial, foundSteering bool
	for _, l := range receivedStreamLogs {
		if l.Line == "Starting streaming attempt" {
			foundInitial = true
		}
		if l.Line == "Executed steering: direct steering prompt" || l.Line == "Received steer from chan: direct steering prompt" {
			foundSteering = true
		}
	}

	if !foundInitial {
		t.Errorf("expected initial stream log line 'Starting streaming attempt'")
	}
	if !foundSteering {
		t.Errorf("expected steered log line in stream logs")
	}

	adapter.mu.Lock()
	steered := adapter.steeredText
	adapter.mu.Unlock()

	if steered != "direct steering prompt" {
		t.Errorf("expected adapter to receive steering text, got: %s", steered)
	}
}

func TestRunner_HeartbeatSteeringDirectiveDelivery(t *testing.T) {
	attemptID := "att-hb-steer-test"
	var receivedPrompt string
	var mu sync.Mutex

	// Mock server returning SteeringDirectives in HeartbeatResponse
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runner/heartbeat" {
			resp := HeartbeatResponse{
				Status:           "ok",
				LeaseDurationSec: 30,
				SteeringDirectives: []SteeringDirective{
					{
						AttemptID: attemptID,
						Prompt:    "fix typo in LandingPage.tsx",
						Author:    "buchin",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		RunnerID:        "test-hb-runner",
		RunnerServerURL: server.URL,
	}

	adapter := &mockStreamingSteeringAdapter{}
	r := NewRunner(cfg, adapter, server.Client())

	r.mu.Lock()
	r.activeAttempts[attemptID] = &activeAttempt{
		attempt: &task.TaskAttempt{ID: attemptID, Status: task.AttemptStatusRunning},
		steerFunc: func(prompt string) error {
			mu.Lock()
			receivedPrompt = prompt
			mu.Unlock()
			return nil
		},
	}
	r.mu.Unlock()

	// Trigger heartbeat
	r.sendHeartbeat(context.Background(), "test-host")

	mu.Lock()
	prompt := receivedPrompt
	mu.Unlock()

	if prompt != "fix typo in LandingPage.tsx" {
		t.Fatalf("expected heartbeat steering prompt 'fix typo in LandingPage.tsx', got '%s'", prompt)
	}
}
