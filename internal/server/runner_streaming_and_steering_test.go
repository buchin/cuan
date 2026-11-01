package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestRunnerStreamLogsAndSteerEndpoints(t *testing.T) {
	srv, token := setupTestChatServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	ctx := context.Background()
	taskID := "task-stream-steer-test"
	_ = srv.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:           taskID,
		Title:        "Streaming and Steering Test Task",
		Instructions: "Verify real-time stdout/stderr streaming and interactive steering",
		Status:       task.TaskStatusDoing,
	})

	attemptID := "att-stream-steer-01"
	attempt := &task.TaskAttempt{
		ID:            attemptID,
		TaskID:        taskID,
		RunnerID:      "cuan-runner",
		AgentName:     "agy",
		Status:        task.AttemptStatusRunning,
		AttemptNumber: 1,
		CreatedAt:     time.Now().UTC(),
	}
	if err := srv.taskSvc.Store().CreateAttempt(ctx, attempt); err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// 1. Initially, no logs streamed yet. GET /api/v1/runner/attempts/{id}/logs
	logsURL := ts.URL + "/api/v1/runner/attempts/" + attemptID + "/logs"
	req, _ := http.NewRequestWithContext(ctx, "GET", logsURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET logs failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from GET logs, got %d", resp.StatusCode)
	}

	var logData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&logData); err != nil {
		t.Fatalf("decode log response: %v", err)
	}
	if logData["is_final"] != false {
		t.Errorf("expected is_final false for running attempt, got %v", logData["is_final"])
	}
	if logData["status"] != string(task.AttemptStatusRunning) {
		t.Errorf("expected status running, got %v", logData["status"])
	}

	// 2. Append first chunk of streaming logs from runner
	streamURL := ts.URL + "/api/v1/runner/attempts/" + attemptID + "/logs"
	streamPayload := runner.StreamLogsRequest{
		RunnerID:  "cuan-runner",
		AttemptID: attemptID,
		Logs: []task.LogRecord{
			{Stream: "stdout", Line: "Agent initiated workspace scan"},
			{Stream: "stdout", Line: "Found 12 source files"},
		},
	}
	payloadBytes, _ := json.Marshal(streamPayload)
	streamReq, _ := http.NewRequestWithContext(ctx, "POST", streamURL, bytes.NewReader(payloadBytes))
	streamReq.Header.Set("Authorization", "Bearer "+token)
	streamReq.Header.Set("Content-Type", "application/json")

	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("POST stream logs failed: %v", err)
	}
	streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for stream append, got %d", streamResp.StatusCode)
	}

	// 3. Read first chunk via GET /api/v1/runner/attempts/{id}/logs?offset=0
	reqChunk1, _ := http.NewRequestWithContext(ctx, "GET", logsURL+"?offset=0", nil)
	reqChunk1.Header.Set("Authorization", "Bearer "+token)
	respChunk1, err := http.DefaultClient.Do(reqChunk1)
	if err != nil {
		t.Fatalf("GET chunk 1 failed: %v", err)
	}
	defer respChunk1.Body.Close()

	var dataChunk1 struct {
		AttemptID string           `json:"attempt_id"`
		IsFinal   bool             `json:"is_final"`
		Offset    int64            `json:"offset"`
		Total     int              `json:"total"`
		Logs      []task.LogRecord `json:"logs"`
	}
	if err := json.NewDecoder(respChunk1.Body).Decode(&dataChunk1); err != nil {
		t.Fatalf("decode chunk 1 failed: %v", err)
	}
	if dataChunk1.Total != 2 || len(dataChunk1.Logs) != 2 {
		t.Fatalf("expected 2 log records, got total=%d len=%d", dataChunk1.Total, len(dataChunk1.Logs))
	}
	if dataChunk1.IsFinal {
		t.Errorf("expected is_final false while running")
	}
	lastOffset := dataChunk1.Offset

	// 4. Append second chunk of streaming logs
	streamPayload2 := runner.StreamLogsRequest{
		RunnerID:  "cuan-runner",
		AttemptID: attemptID,
		Logs: []task.LogRecord{
			{Stream: "stdout", Line: "Compiled Go binary successfully"},
		},
	}
	payloadBytes2, _ := json.Marshal(streamPayload2)
	streamReq2, _ := http.NewRequestWithContext(ctx, "POST", streamURL, bytes.NewReader(payloadBytes2))
	streamReq2.Header.Set("Authorization", "Bearer "+token)
	streamReq2.Header.Set("Content-Type", "application/json")
	respStream2, _ := http.DefaultClient.Do(streamReq2)
	respStream2.Body.Close()

	// 5. Read incrementally using ?offset=lastOffset
	offsetURL := fmt.Sprintf("%s?offset=%d", logsURL, lastOffset)
	reqChunk2, _ := http.NewRequestWithContext(ctx, "GET", offsetURL, nil)
	reqChunk2.Header.Set("Authorization", "Bearer "+token)
	respChunk2, err := http.DefaultClient.Do(reqChunk2)
	if err != nil {
		t.Fatalf("GET chunk 2 failed: %v", err)
	}
	defer respChunk2.Body.Close()

	var dataChunk2 struct {
		IsFinal bool             `json:"is_final"`
		Total   int              `json:"total"`
		Logs    []task.LogRecord `json:"logs"`
	}
	if err := json.NewDecoder(respChunk2.Body).Decode(&dataChunk2); err != nil {
		t.Fatalf("decode chunk 2 failed: %v", err)
	}
	if dataChunk2.Total != 1 || len(dataChunk2.Logs) != 1 {
		t.Fatalf("expected exactly 1 delta log record, got %d", dataChunk2.Total)
	}
	if dataChunk2.Logs[0].Line != "Compiled Go binary successfully" {
		t.Errorf("unexpected log line content: %s", dataChunk2.Logs[0].Line)
	}

	// 6. Test Interactive Steering: POST /api/v1/runner/attempts/{id}/steer
	steerURL := ts.URL + "/api/v1/runner/attempts/" + attemptID + "/steer"
	var directSteerPrompt string
	srv.RunnerService().RegisterSteerHandler("cuan-runner", func(attID, prompt string) error {
		if attID == attemptID {
			directSteerPrompt = prompt
		}
		return nil
	})

	steerPayload := runner.SteerAttemptRequest{
		Prompt: "Please make sure to also update LandingPage.tsx",
		Author: "owner@example.com",
	}
	steerBytes, _ := json.Marshal(steerPayload)
	steerReq, _ := http.NewRequestWithContext(ctx, "POST", steerURL, bytes.NewReader(steerBytes))
	steerReq.Header.Set("Authorization", "Bearer "+token)
	steerReq.Header.Set("Content-Type", "application/json")

	steerResp, err := http.DefaultClient.Do(steerReq)
	if err != nil {
		t.Fatalf("POST steer failed: %v", err)
	}
	defer steerResp.Body.Close()

	if steerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from steer endpoint, got %d", steerResp.StatusCode)
	}

	var steerResult runner.SteerAttemptResponse
	if err := json.NewDecoder(steerResp.Body).Decode(&steerResult); err != nil {
		t.Fatalf("decode steer response failed: %v", err)
	}
	if steerResult.Status != "delivered" {
		t.Errorf("expected delivered status, got %s", steerResult.Status)
	}
	if directSteerPrompt != "Please make sure to also update LandingPage.tsx" {
		t.Errorf("direct steer handler did not receive expected prompt, got: %s", directSteerPrompt)
	}

	// 6b. Test Steering via Task ID: POST /api/v1/tasks/{taskId}/steer
	directSteerPrompt = ""
	taskSteerURL := ts.URL + "/api/v1/tasks/" + taskID + "/steer"
	taskSteerPayload := runner.SteerAttemptRequest{
		Prompt: "Task-scoped steering directive",
		Author: "owner@example.com",
	}
	taskSteerBytes, _ := json.Marshal(taskSteerPayload)
	taskSteerReq, _ := http.NewRequestWithContext(ctx, "POST", taskSteerURL, bytes.NewReader(taskSteerBytes))
	taskSteerReq.Header.Set("Authorization", "Bearer "+token)
	taskSteerReq.Header.Set("Content-Type", "application/json")

	taskSteerResp, err := http.DefaultClient.Do(taskSteerReq)
	if err != nil {
		t.Fatalf("POST /tasks/{id}/steer failed: %v", err)
	}
	defer taskSteerResp.Body.Close()

	if taskSteerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /tasks/{id}/steer, got %d", taskSteerResp.StatusCode)
	}
	if directSteerPrompt != "Task-scoped steering directive" {
		t.Errorf("expected directSteerPrompt 'Task-scoped steering directive', got: %s", directSteerPrompt)
	}

	// 7. Complete the attempt and verify is_final is true
	attempt.Status = task.AttemptStatusSucceeded
	_ = srv.taskSvc.Store().UpdateAttempt(ctx, attempt)

	reqFinal, _ := http.NewRequestWithContext(ctx, "GET", logsURL, nil)
	reqFinal.Header.Set("Authorization", "Bearer "+token)
	respFinal, err := http.DefaultClient.Do(reqFinal)
	if err != nil {
		t.Fatalf("GET final logs failed: %v", err)
	}
	defer respFinal.Body.Close()

	var finalData struct {
		IsFinal bool   `json:"is_final"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(respFinal.Body).Decode(&finalData); err != nil {
		t.Fatalf("decode final data: %v", err)
	}
	if !finalData.IsFinal {
		t.Errorf("expected is_final true after attempt completion")
	}
	if finalData.Status != string(task.AttemptStatusSucceeded) {
		t.Errorf("expected status succeeded, got %s", finalData.Status)
	}
}

func TestRunnerSteerGracefulFallbackWhenNotRunning(t *testing.T) {
	srv, token := setupTestChatServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	ctx := context.Background()
	taskID := "task-fallback-steer"
	_ = srv.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:     taskID,
		Title:  "Task in Review",
		Status: task.TaskStatusReview,
	})

	attemptID := "att-fallback-01"
	attempt := &task.TaskAttempt{
		ID:        attemptID,
		TaskID:    taskID,
		RunnerID:  "cuan-runner",
		Status:    task.AttemptStatusSucceeded,
		CreatedAt: time.Now().UTC(),
	}
	_ = srv.taskSvc.Store().CreateAttempt(ctx, attempt)

	steerURL := ts.URL + "/api/v1/runner/attempts/" + attemptID + "/steer"
	payload := runner.SteerAttemptRequest{
		Prompt: "Review note: looks good, deploy next",
		Author: "owner@example.com",
	}
	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", steerURL, bytes.NewReader(payloadBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST steer failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for graceful fallback steer, got %d", resp.StatusCode)
	}

	var res runner.SteerAttemptResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if res.Status != "queued" || !res.Fallback {
		t.Errorf("expected queued fallback, got status=%s fallback=%v", res.Status, res.Fallback)
	}
}
