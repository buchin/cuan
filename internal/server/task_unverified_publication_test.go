package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestAutoApplyCannotPublishUnverifiedAttempt(t *testing.T) {
	for _, status := range []string{"", task.AttemptStatusRunning, task.AttemptStatusFailed, task.AttemptStatusCanceled} {
		t.Run("attempt-"+status, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			svc := task.NewService(store, nil)
			item := &task.Task{ID: "task", ProjectID: "project", Title: "Implement feature", Status: task.TaskStatusDoing, BranchName: "task/feature", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if status != "" {
				if err := store.CreateAttempt(ctx, &task.TaskAttempt{ID: "attempt", TaskID: item.ID, Status: status, CreatedAt: time.Now()}); err != nil {
					t.Fatal(err)
				}
			}
			// Publishing would cross into absent project/deployer dependencies.
			s := &Server{taskSvc: svc}
			s.autoApplyAndDeployTask(ctx, item.ID, item.ProjectID)
			after, err := store.GetTask(ctx, item.ID)
			if err != nil || after.Status != task.TaskStatusDoing {
				t.Fatal("unverified task accepted", err)
			}
		})
	}
}
