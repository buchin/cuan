package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// A task that declares its stages decides publication itself. Declaring
// implement+verify without publish must not be overridden by observed code
// changes, which is what blocked finished work in production.
func TestDeclaredStagesWithoutPublishSkipReceipt(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)

	finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{
		"proj_declared": {
			ID:       "proj_declared",
			Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`),
		},
	}}
	svc := NewService(runnerStore, taskStore, finder)

	tk := &task.Task{
		ID:             "task_declared",
		ProjectID:      "proj_declared",
		Title:          "Adjust hero copy",
		Status:         task.TaskStatusDoing,
		RequiredStages: []string{task.StageImplement, task.StageVerify},
	}
	if err := taskStore.CreateTask(ctx, tk); err != nil {
		t.Fatal(err)
	}
	att := &task.TaskAttempt{ID: "att_declared", RunnerID: "runner_1", TaskID: tk.ID, Status: task.AttemptStatusRunning, CreatedAt: time.Now()}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatal(err)
	}

	changed := true
	completed, err := svc.Complete(ctx, att.ID, CompleteAttemptRequest{
		RunnerID:             "runner_1",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
		AgentReportRequired:  true,
		CommitSHA:            strings.Repeat("c", 40),
		BranchName:           "cuan/task-declared",
		HasCodeChanges:       &changed,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if completed.Status == task.AttemptStatusFailed {
		t.Fatalf("declaration without publish was ignored: %s", completed.ErrorMessage)
	}
}

// A declaration that includes publish still requires a verified receipt, so the
// exemption above cannot be used to skip real publication.
func TestDeclaredPublishStillRequiresReceipt(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)

	finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{
		"proj_publish": {
			ID:       "proj_publish",
			Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`),
		},
	}}
	svc := NewService(runnerStore, taskStore, finder)

	tk := &task.Task{
		ID:             "task_publish",
		ProjectID:      "proj_publish",
		Title:          "Ship the change",
		Status:         task.TaskStatusDoing,
		RequiredStages: []string{task.StageImplement, task.StagePublish},
	}
	if err := taskStore.CreateTask(ctx, tk); err != nil {
		t.Fatal(err)
	}
	att := &task.TaskAttempt{ID: "att_publish", RunnerID: "runner_1", TaskID: tk.ID, Status: task.AttemptStatusRunning, CreatedAt: time.Now()}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatal(err)
	}

	changed := true
	completed, err := svc.Complete(ctx, att.ID, CompleteAttemptRequest{
		RunnerID:             "runner_1",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
		AgentReportRequired:  true,
		CommitSHA:            strings.Repeat("d", 40),
		BranchName:           "cuan/task-publish",
		HasCodeChanges:       &changed,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if completed.Status != task.AttemptStatusFailed {
		t.Fatal("declared publish was accepted without a verified receipt")
	}
}

// Undeclared tasks keep the previous behaviour, including the auto-deploy gate,
// so older work is not silently exempted.
func TestUndeclaredTaskKeepsLegacyPublicationGate(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)

	finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{
		"proj_legacy": {
			ID:       "proj_legacy",
			Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`),
		},
	}}
	svc := NewService(runnerStore, taskStore, finder)

	tk := &task.Task{ID: "task_legacy", ProjectID: "proj_legacy", Title: "Legacy work", Status: task.TaskStatusDoing}
	if err := taskStore.CreateTask(ctx, tk); err != nil {
		t.Fatal(err)
	}
	att := &task.TaskAttempt{ID: "att_legacy", RunnerID: "runner_1", TaskID: tk.ID, Status: task.AttemptStatusRunning, CreatedAt: time.Now()}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatal(err)
	}

	changed := true
	completed, err := svc.Complete(ctx, att.ID, CompleteAttemptRequest{
		RunnerID:             "runner_1",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
		AgentReportRequired:  true,
		CommitSHA:            strings.Repeat("e", 40),
		BranchName:           "cuan/task-legacy",
		HasCodeChanges:       &changed,
	})
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if completed.Status != task.AttemptStatusFailed {
		t.Fatal("undeclared task lost its legacy publication gate")
	}
}
