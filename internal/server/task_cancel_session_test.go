package server

import (
	"context"
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
