package server

import (
	"context"
	"testing"

	"github.com/buchin/cuan/internal/scheduler"
	"github.com/buchin/cuan/internal/task"
)

func TestServer_ExecuteScheduledTask(t *testing.T) {
	s, _ := setupTestChatServer(t)

	p, err := s.projectSvc.EnsureDefault(context.Background())
	if err != nil {
		t.Fatalf("EnsureDefault failed: %v", err)
	}

	schedTask := &scheduler.ScheduledTask{
		ID:        "sch_test_1",
		Title:     "🛠️ Routine Health Check",
		Prompt:    "Run system check and verify code health",
		ProjectID: p.ID,
	}

	ctx := context.Background()
	err = s.executeScheduledTask(ctx, schedTask)
	if err != nil {
		t.Fatalf("executeScheduledTask failed: %v", err)
	}

	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	found := false
	for _, tItem := range tasks {
		if tItem.Title == schedTask.Title && strings.HasPrefix(tItem.Instructions, schedTask.Prompt) {
			found = true
			if tItem.Status != task.TaskStatusQueued {
				t.Errorf("expected task status %q, got %q", task.TaskStatusQueued, tItem.Status)
			}
			if tItem.ProjectID != p.ID {
				t.Errorf("expected task project %q, got %q", p.ID, tItem.ProjectID)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected created task for routine not found in task service")
	}
}
