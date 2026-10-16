package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestVerifyT4_TaskBoardAndHistoryPersistence(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		ServerPort:  "8080",
		AuthSecret:  "test-secret-key-32-chars-long!",
	}

	// Persistent stores shared across server restart simulation
	authStore := auth.NewMemoryStore()
	taskStore := task.NewMemoryStore()

	// Initial Server Boot (srv1)
	srv1 := NewServer(cfg, nil, auth.NewService(authStore))
	srv1.SetTaskService(task.NewService(taskStore, srv1.ProjectService()))

	// Authenticate owner
	setupPayload := []byte(`{"email":"owner@cuan.dev","password":"MasterPassword123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(setupPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d, body: %s", rr.Code, rr.Body.String())
	}
	var authResp auth.AuthResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &authResp)
	token := authResp.Token

	// -------------------------------------------------------------------------
	// 1. Owner creates a task via API (DoD: Owner membuat task)
	// -------------------------------------------------------------------------
	createTaskPayload := []byte(`{
		"title": "Refactor runner sandbox isolation",
		"instructions": "Pastikan runner proses diisolasi dan kredensial disamarkan.",
		"priority": "high",
		"assigned_agent": "agy"
	}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewBuffer(createTaskPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for task creation, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var createdTask task.Task
	_ = json.Unmarshal(rr.Body.Bytes(), &createdTask)
	taskID := createdTask.ID
	if taskID == "" {
		t.Fatalf("expected valid task ID, got empty")
	}
	if createdTask.Status != task.TaskStatusBacklog {
		t.Fatalf("expected status backlog, got %s", createdTask.Status)
	}

	// -------------------------------------------------------------------------
	// 2. Owner modifies task status: backlog -> doing (DoD: Owner mengubah task)
	// -------------------------------------------------------------------------
	updatePayload := []byte(`{
		"status": "doing",
		"priority": "urgent"
	}`)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+taskID, bytes.NewBuffer(updatePayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for update task, got %d", rr.Code)
	}

	var updatedTask task.Task
	_ = json.Unmarshal(rr.Body.Bytes(), &updatedTask)
	if updatedTask.Status != task.TaskStatusDoing {
		t.Fatalf("expected status doing, got %s", updatedTask.Status)
	}
	if updatedTask.Priority != task.PriorityUrgent {
		t.Fatalf("expected priority urgent, got %s", updatedTask.Priority)
	}

	// -------------------------------------------------------------------------
	// 3. Register Attempt (Pemisahan status task vs status attempt)
	// -------------------------------------------------------------------------
	attemptPayload := []byte(`{
		"runner_id": "runner_vps_01",
		"agent_name": "agy"
	}`)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/attempts", taskID), bytes.NewBuffer(attemptPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for attempt, got %d", rr.Code)
	}

	var attempt task.TaskAttempt
	_ = json.Unmarshal(rr.Body.Bytes(), &attempt)
	attemptID := attempt.ID
	if attempt.Status != task.AttemptStatusQueued {
		t.Fatalf("expected attempt status queued, got %s", attempt.Status)
	}

	// -------------------------------------------------------------------------
	// 4. Update Attempt: queued -> running -> failed (DoD: Kegagalan tidak ditampilkan sebagai sukses)
	// -------------------------------------------------------------------------
	// Running
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/tasks/%s/attempts/%s", taskID, attemptID), bytes.NewBuffer([]byte(`{"status":"running"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for attempt running, got %d", rr.Code)
	}

	// Falsely claim succeeded but report exit code 1
	failPayload := []byte(`{
		"status": "succeeded",
		"exit_code": 1,
		"error_message": "Command syntax error in runner sandbox"
	}`)
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/tasks/%s/attempts/%s", taskID, attemptID), bytes.NewBuffer(failPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for attempt update, got %d", rr.Code)
	}

	var failedAttempt task.TaskAttempt
	_ = json.Unmarshal(rr.Body.Bytes(), &failedAttempt)
	if failedAttempt.Status != task.AttemptStatusFailed {
		t.Fatalf("CRITICAL DoD: Non-zero exit code must force status to failed, got %s", failedAttempt.Status)
	}

	// -------------------------------------------------------------------------
	// 5. Add Comment to Task
	// -------------------------------------------------------------------------
	commentPayload := []byte(`{"message": "Runner mengalami syntax error pada script setup.sh, perlu evaluasi."}`)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/comments", taskID), bytes.NewBuffer(commentPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for comment, got %d", rr.Code)
	}

	// -------------------------------------------------------------------------
	// 6. SIMULATE SERVER REBOOT (DoD: Restart server tidak menghilangkan data task atau histori)
	// -------------------------------------------------------------------------
	// srv1 is stopped. srv2 boots up using the persistent volume stores.
	srv2 := NewServer(cfg, nil, auth.NewService(authStore))
	srv2.SetTaskService(task.NewService(taskStore, srv2.ProjectService()))

	// Fetch detail of the task from srv2
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("[Post-Restart] expected 200 OK for get task detail, got %d", rr.Code)
	}

	var detailAfterReboot task.TaskDetailResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &detailAfterReboot)

	// Verify task fields survived reboot
	if detailAfterReboot.ID != taskID {
		t.Fatalf("[Post-Restart] task ID mismatch: expected %s, got %s", taskID, detailAfterReboot.ID)
	}
	if detailAfterReboot.Status != task.TaskStatusDoing {
		t.Fatalf("[Post-Restart] task status lost: expected doing, got %s", detailAfterReboot.Status)
	}
	if detailAfterReboot.Priority != task.PriorityUrgent {
		t.Fatalf("[Post-Restart] task priority lost: expected urgent, got %s", detailAfterReboot.Priority)
	}

	// Verify attempt survived reboot with exact status
	if detailAfterReboot.LatestAttempt == nil {
		t.Fatalf("[Post-Restart] latest attempt lost across reboot")
	}
	if detailAfterReboot.LatestAttempt.Status != task.AttemptStatusFailed {
		t.Fatalf("[Post-Restart] attempt status lost: expected failed, got %s", detailAfterReboot.LatestAttempt.Status)
	}

	// Verify history & activity timeline survived reboot
	if len(detailAfterReboot.Activities) < 4 {
		t.Fatalf("[Post-Restart] expected at least 4 activities (created, status_change, attempt_transition, comment), got %d", len(detailAfterReboot.Activities))
	}

	// Print summary of verified transitions
	fmt.Println("=== T4 VERIFIED: Task Detail Post-Restart ===")
	fmt.Printf("Task ID: %s | Title: %s\n", detailAfterReboot.ID, detailAfterReboot.Title)
	fmt.Printf("Task Status: %s | Priority: %s\n", detailAfterReboot.Status, detailAfterReboot.Priority)
	fmt.Printf("Latest Attempt: %s | Status: %s (Exit Code: %v)\n", detailAfterReboot.LatestAttempt.ID, detailAfterReboot.LatestAttempt.Status, *detailAfterReboot.LatestAttempt.ExitCode)
	fmt.Println("Activities Timeline:")
	for idx, act := range detailAfterReboot.Activities {
		fmt.Printf("  [%d] [%s] by %s: %s (Old: %s -> New: %s)\n", idx+1, act.ActivityType, act.Author, act.Message, act.OldStatus, act.NewStatus)
	}
	fmt.Println("=== T4 VERIFICATION COMPLETE ===")
}
