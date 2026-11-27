package task

import (
	"context"
	"testing"

	"github.com/buchin/cuan/internal/project"
)

type mockProjectFinder struct {
	names map[string]string
}

func (m *mockProjectFinder) Get(ctx context.Context, id string) (*project.ProjectResponse, error) {
	name, ok := m.names[id]
	if !ok {
		return nil, project.ErrProjectNotFound
	}
	return &project.ProjectResponse{
		ID:      id,
		Name:    name,
		RepoURL: "https://github.com/buchin/cuan",
	}, nil
}

func TestTaskService_CRUD_And_Activities(t *testing.T) {
	store := NewMemoryStore()
	finder := &mockProjectFinder{
		names: map[string]string{
			"proj_1": "Cuan Core",
		},
	}
	svc := NewService(store, finder)
	ctx := context.Background()
	author := "owner@cuan.dev"

	// 1. Create Task
	req := CreateTaskRequest{
		ProjectID:     "proj_1",
		Title:         "S01.4 Bangun task board dan histori",
		Instructions:  "Implementasikan task CRUD dan histori",
		Priority:      PriorityHigh,
		AssignedAgent: "agy",
	}

	task, err := svc.CreateTask(ctx, author, req)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	if task.ID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if task.Status != TaskStatusBacklog {
		t.Fatalf("expected initial status %q, got %q", TaskStatusBacklog, task.Status)
	}
	if task.Priority != PriorityHigh {
		t.Fatalf("expected priority %q, got %q", PriorityHigh, task.Priority)
	}

	// 2. Read Task Detail & Verify Initial Activity
	detail, err := svc.GetTaskDetail(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to get task detail: %v", err)
	}
	if detail.ProjectName != "Cuan Core" {
		t.Fatalf("expected project name 'Cuan Core', got %q", detail.ProjectName)
	}
	if len(detail.Activities) != 1 {
		t.Fatalf("expected 1 activity (task_created), got %d", len(detail.Activities))
	}
	if detail.Activities[0].ActivityType != ActivityTypeCreated {
		t.Fatalf("expected activity type %q, got %q", ActivityTypeCreated, detail.Activities[0].ActivityType)
	}

	// 3. Update Task Status (Transition from backlog to doing)
	doingStatus := TaskStatusDoing
	updated, err := svc.UpdateTask(ctx, task.ID, author, UpdateTaskRequest{
		Status: &doingStatus,
	})
	if err != nil {
		t.Fatalf("failed to update task status: %v", err)
	}
	if updated.Status != TaskStatusDoing {
		t.Fatalf("expected status %q, got %q", TaskStatusDoing, updated.Status)
	}

	// 4. Verify Transition Recorded in Activity History
	detail, err = svc.GetTaskDetail(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to get task detail: %v", err)
	}
	if len(detail.Activities) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(detail.Activities))
	}
	lastAct := detail.Activities[1]
	if lastAct.ActivityType != ActivityTypeStatusChange {
		t.Fatalf("expected status_change activity, got %q", lastAct.ActivityType)
	}
	if lastAct.OldStatus != TaskStatusBacklog || lastAct.NewStatus != TaskStatusDoing {
		t.Fatalf("expected transition backlog -> doing, got %s -> %s", lastAct.OldStatus, lastAct.NewStatus)
	}

	// 5. Add Comment
	commentAct, err := svc.AddComment(ctx, task.ID, author, "Mulai investigasi implementasi task board")
	if err != nil {
		t.Fatalf("failed to add comment: %v", err)
	}
	if commentAct.ActivityType != ActivityTypeComment {
		t.Fatalf("expected comment activity type, got %q", commentAct.ActivityType)
	}

	// 6. Delete Task
	err = svc.DeleteTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to delete task: %v", err)
	}

	_, err = svc.GetTaskDetail(ctx, task.ID)
	if err != ErrTaskNotFound {
		t.Fatalf("expected ErrTaskNotFound after delete, got %v", err)
	}
}

func TestTaskService_AttemptSeparation_And_FailureRules(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, nil)
	ctx := context.Background()
	author := "owner@cuan.dev"

	task, err := svc.CreateTask(ctx, author, CreateTaskRequest{
		Title:        "Task Uji Attempt",
		Instructions: "Uji coba aturan pemisahan attempt dan kegagalan",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 1. Create Attempt -> Status Queued
	att, err := svc.CreateAttempt(ctx, task.ID, author, CreateAttemptRequest{
		RunnerID:  "runner_01",
		AgentName: "agy",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}
	if att.Status != AttemptStatusQueued {
		t.Fatalf("expected attempt status queued, got %q", att.Status)
	}

	// Verify Task status is STILL backlog (Attempt queued does NOT mutate task status automatically)
	detail, _ := svc.GetTaskDetail(ctx, task.ID)
	if detail.Status != TaskStatusBacklog {
		t.Fatalf("task status must remain backlog, got %q", detail.Status)
	}
	if detail.LatestAttempt == nil || detail.LatestAttempt.ID != att.ID {
		t.Fatalf("expected latest attempt to match %s", att.ID)
	}

	// 2. Move Attempt to Running
	attRunning, err := svc.UpdateAttemptStatus(ctx, att.ID, author, UpdateAttemptRequest{
		Status: AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("failed to update attempt to running: %v", err)
	}
	if attRunning.Status != AttemptStatusRunning || attRunning.StartedAt == nil {
		t.Fatalf("expected attempt running with started_at timestamp")
	}

	// 3. CRITICAL DoD RULE: Non-zero exit code MUST be recorded as Failed
	// Even if caller requested succeeded, exit_code=1 MUST force status to failed!
	exitCodeFail := 1
	attFailed, err := svc.UpdateAttemptStatus(ctx, att.ID, author, UpdateAttemptRequest{
		Status:       AttemptStatusSucceeded, // Caller falsely claims succeeded
		ExitCode:     &exitCodeFail,          // Non-zero exit code!
		ErrorMessage: "Automated test suite failed with exit 1",
	})
	if err != nil {
		t.Fatalf("failed to update attempt: %v", err)
	}
	if attFailed.Status != AttemptStatusFailed {
		t.Fatalf("CRITICAL: Non-zero exit code must force status to failed, got %q", attFailed.Status)
	}

	// 4. CRITICAL DoD RULE: Exit code 0 does not automatically accept if test failed
	exitCodeZero := 0
	attVerificationFailed, err := svc.UpdateAttemptStatus(ctx, att.ID, author, UpdateAttemptRequest{
		Status:       AttemptStatusFailed, // Verification failed
		ExitCode:     &exitCodeZero,       // Process exited 0 but tests failed
		ErrorMessage: "Exit code was 0 but verification assertions rejected output",
	})
	if err != nil {
		t.Fatalf("failed to update attempt: %v", err)
	}
	if attVerificationFailed.Status != AttemptStatusFailed {
		t.Fatalf("CRITICAL: Attempt marked as failed must remain failed even with exit code 0, got %q", attVerificationFailed.Status)
	}

	// 5. Verify Activity Log recorded accurate transition history
	detail, err = svc.GetTaskDetail(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to get task detail: %v", err)
	}
	foundFailureLog := false
	for _, act := range detail.Activities {
		if act.ActivityType == ActivityTypeAttempt && act.NewStatus == AttemptStatusFailed {
			foundFailureLog = true
			break
		}
	}
	if !foundFailureLog {
		t.Fatalf("expected failure transition log in activity timeline")
	}
}

func TestTaskService_Filtering(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, nil)
	ctx := context.Background()

	_, _ = svc.CreateTask(ctx, "owner", CreateTaskRequest{
		ProjectID: "p1",
		Title:     "Frontend UI Tasks",
		Priority:  PriorityHigh,
	})
	_, _ = svc.CreateTask(ctx, "owner", CreateTaskRequest{
		ProjectID: "p1",
		Title:     "Backend API Chi",
		Priority:  PriorityNormal,
	})
	_, _ = svc.CreateTask(ctx, "owner", CreateTaskRequest{
		ProjectID: "p2",
		Title:     "Database Migration",
		Priority:  PriorityLow,
	})

	// Filter by project p1
	p1Tasks, err := svc.ListTasks(ctx, TaskFilter{ProjectID: "p1"})
	if err != nil || len(p1Tasks) != 2 {
		t.Fatalf("expected 2 tasks for p1, got %d (err: %v)", len(p1Tasks), err)
	}

	// Filter by search query
	searchTasks, err := svc.ListTasks(ctx, TaskFilter{Search: "Frontend"})
	if err != nil || len(searchTasks) != 1 {
		t.Fatalf("expected 1 task for 'Frontend', got %d (err: %v)", len(searchTasks), err)
	}
}

func TestUpdateTask_NoOpDoesNotRecordUpdatedActivity(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil)
	created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		ProjectID: "default",
		Title:     "No-op Update Guard",
		Status:    TaskStatusBacklog,
	})
	if err != nil {
		t.Fatal(err)
	}
	blocked := TaskStatusBlocked
	if _, err := svc.UpdateTask(ctx, created.ID, "coordinator", UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal(err)
	}
	beforeActs, err := svc.Store().ListActivities(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	sameStatus := TaskStatusBlocked
	if _, err := svc.UpdateTask(ctx, created.ID, "coordinator", UpdateTaskRequest{Status: &sameStatus}); err != nil {
		t.Fatal(err)
	}
	afterActs, err := svc.Store().ListActivities(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterActs) != len(beforeActs) {
		t.Fatalf("expected no new activity on no-op UpdateTask, got %d -> %d", len(beforeActs), len(afterActs))
	}
}
