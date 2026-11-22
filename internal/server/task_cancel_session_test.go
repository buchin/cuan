package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestCancelSession_EndpointAndLockRelease(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()

	created := workspaceTask(t, call)
	projID := created.ProjectID
	if projID == "" {
		projID = "fixture-project"
	}

	att, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatalf("CreateAttempt failed: %v", err)
	}

	doing := task.TaskStatusDoing
	_, err = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	_, err = s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("UpdateAttemptStatus failed: %v", err)
	}

	// 1. Verify project lane lock is active while attempt is running
	if active := s.isProjectLaneActive(ctx, projID, task.ExecutionCategoryCode, ""); !active {
		t.Fatalf("expected project lane to be locked while task is running")
	}

	// 2. Trigger cancellation via the official endpoint (same as modal Cancel session button)
	res := call("POST", "/api/v1/tasks/"+created.ID+"/cancel", "")
	if res.Code != 200 {
		t.Fatalf("expected HTTP 200 on cancel, got %d: %s", res.Code, res.Body.String())
	}

	// 3. Verify task and attempt status are updated to cancelled in the database
	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusCancelled {
		t.Errorf("expected task status %q, got %q", task.TaskStatusCancelled, detail.Status)
	}
	if detail.LatestAttempt == nil || detail.LatestAttempt.Status != task.AttemptStatusCanceled {
		t.Errorf("expected attempt status %q, got %v", task.AttemptStatusCanceled, detail.LatestAttempt)
	}

	// 4. Verify project lane lock is immediately released
	if active := s.isProjectLaneActive(ctx, projID, task.ExecutionCategoryCode, ""); active {
		t.Errorf("expected project lane lock to be released immediately after cancellation")
	}

	// 5. Subsequent task can be started without conflict
	created2 := workspaceTask(t, call)
	att2, err := s.taskSvc.CreateAttempt(ctx, created2.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatalf("expected subsequent task attempt to be created without conflict, got: %v", err)
	}
	if att2 == nil {
		t.Fatal("expected non-nil attempt for subsequent task")
	}
}

func TestCancelSession_FailedAttemptIdempotent(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()

	created := workspaceTask(t, call)
	projID := created.ProjectID
	if projID == "" {
		projID = "fixture-project"
	}

	att, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatalf("CreateAttempt failed: %v", err)
	}

	doing := task.TaskStatusDoing
	_, err = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	failed := task.AttemptStatusFailed
	exitFail := 1
	errMsg := "AI provider reported usage limit"
	_, err = s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status:       failed,
		ExitCode:     &exitFail,
		ErrorMessage: errMsg,
	})
	if err != nil {
		t.Fatalf("UpdateAttemptStatus failed: %v", err)
	}

	// Trigger cancellation via endpoint when attempt is already in failed status
	res := call("POST", "/api/v1/tasks/"+created.ID+"/cancel", "")
	if res.Code != 200 {
		t.Fatalf("expected HTTP 200 on cancel for failed attempt, got %d: %s", res.Code, res.Body.String())
	}

	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusCancelled {
		t.Errorf("expected task status %q, got %q", task.TaskStatusCancelled, detail.Status)
	}
	if detail.LatestAttempt == nil || !detail.LatestAttempt.CancelRequested {
		t.Errorf("expected attempt CancelRequested to be true, got %v", detail.LatestAttempt)
	}

	// Concurrency lock must be released
	if active := s.isProjectLaneActive(ctx, projID, task.ExecutionCategoryCode, ""); active {
		t.Errorf("expected project lane lock to be released")
	}
}

func TestCancelSession_AlreadyCancelledIdempotent(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()

	created := workspaceTask(t, call)
	projID := created.ProjectID
	if projID == "" {
		projID = "fixture-project"
	}

	att, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatalf("CreateAttempt failed: %v", err)
	}

	_, err = s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("UpdateAttemptStatus failed: %v", err)
	}

	// First cancellation
	res1 := call("POST", "/api/v1/tasks/"+created.ID+"/cancel", "")
	if res1.Code != 200 {
		t.Fatalf("expected HTTP 200 on first cancel, got %d: %s", res1.Code, res1.Body.String())
	}

	// Second cancellation (idempotent repeat)
	res2 := call("POST", "/api/v1/tasks/"+created.ID+"/cancel", "")
	if res2.Code != 200 {
		t.Fatalf("expected HTTP 200 on repeated cancel, got %d: %s", res2.Code, res2.Body.String())
	}

	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusCancelled {
		t.Errorf("expected task status %q, got %q", task.TaskStatusCancelled, detail.Status)
	}
	if detail.LatestAttempt == nil || detail.LatestAttempt.Status != task.AttemptStatusCanceled {
		t.Errorf("expected attempt status %q, got %v", task.AttemptStatusCanceled, detail.LatestAttempt)
	}

	// Lock remains released
	if active := s.isProjectLaneActive(ctx, projID, task.ExecutionCategoryCode, ""); active {
		t.Errorf("expected project lane lock to remain released")
	}
}

func TestCancelDedicatedProjectTerminalSession(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()

	w := call("POST", "/api/v1/tasks", `{"request_id":"20000000-0000-4000-8000-000000000099","title":"Terminal · default","instructions":"Interactive terminal session"}`)
	if w.Code != 201 {
		t.Fatalf("CreateTask failed: %d %s", w.Code, w.Body.String())
	}
	var created task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	att, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatalf("CreateAttempt failed: %v", err)
	}

	doing := task.TaskStatusDoing
	_, err = s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	_, err = s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("UpdateAttemptStatus failed: %v", err)
	}

	// 1. Cancel the dedicated terminal session
	res := call("POST", "/api/v1/tasks/"+created.ID+"/cancel", "")
	if res.Code != 200 {
		t.Fatalf("expected HTTP 200 on cancel, got %d: %s", res.Code, res.Body.String())
	}

	// 2. Verify task status is cancelled and attempt is canceled
	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusCancelled {
		t.Fatalf("expected task status %q, got %q", task.TaskStatusCancelled, detail.Status)
	}
	if detail.LatestAttempt == nil || detail.LatestAttempt.Status != task.AttemptStatusCanceled {
		t.Fatalf("expected attempt status %q, got %v", task.AttemptStatusCanceled, detail.LatestAttempt)
	}

	// 3. Attempting to connect to cancelled terminal via GET WebSocket must NOT auto-resurrect
	termRes := call("GET", "/api/v1/tasks/"+created.ID+"/terminal", "")
	if termRes.Code != 410 && termRes.Code != 409 {
		t.Fatalf("expected HTTP 410 or 409 on cancelled terminal connect, got %d", termRes.Code)
	}

	// 4. Verify task is STILL cancelled and no new attempt was created
	detailAfter, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detailAfter.Status != task.TaskStatusCancelled {
		t.Fatalf("expected task status to remain cancelled, got %q", detailAfter.Status)
	}
	if len(detailAfter.Attempts) != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", len(detailAfter.Attempts))
	}
}

