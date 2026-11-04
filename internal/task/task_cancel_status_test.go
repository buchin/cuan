package task

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Retiring obsolete scope is distinct from interrupting an active execution.
// Historical attempt evidence must survive and queued/running workers must not
// be hidden by an administrative status update.
func TestRetireObsoleteTaskPreservesAttemptsAndStopsLaunch(t *testing.T) {
	for _, attemptStatus := range []string{"", AttemptStatusFailed, AttemptStatusSucceeded, AttemptStatusCanceled, AttemptStatusQueued, AttemptStatusRunning} {
		for _, spelling := range []string{"canceled", "cancelled"} {
			t.Run(attemptStatus+"/"+spelling, func(t *testing.T) {
				ctx := context.Background()
				store := NewMemoryStore()
				svc := NewService(store, nil)
				item, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: "project", Title: "Old scope", Instructions: "Approved original work"})
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now().UTC()
				job, err := store.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: "request", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", SourceRequestID: "approval", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(item.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
				if err != nil {
					t.Fatal(err)
				}
				if attemptStatus != "" {
					if err = store.CreateAttempt(ctx, &TaskAttempt{ID: "attempt", TaskID: item.ID, Status: attemptStatus, CreatedAt: now}); err != nil {
						t.Fatal(err)
					}
				}
				reason := "Superseded by the current approved design"
				_, err = svc.UpdateTask(ctx, item.ID, "owner", UpdateTaskRequest{Status: &spelling, Summary: &reason})
				active := attemptStatus == AttemptStatusQueued || attemptStatus == AttemptStatusRunning
				if active && !errors.Is(err, ErrTaskActive) {
					t.Fatal("active worker bypassed", err)
				}
				if !active && err != nil {
					t.Fatal(err)
				}
				saved, _ := store.GetTask(ctx, item.ID)
				launch, _ := store.GetLaunch(ctx, job.ID)
				if active {
					if saved.Status == "cancelled" || launch.State == "cancelled" {
						t.Fatal("active work was hidden")
					}
				} else {
					if saved.Status != "cancelled" || saved.Summary != reason || launch.State != "cancelled" {
						t.Fatal(saved.Status, launch.State)
					}
					claimed, err := store.ClaimLaunch(ctx, now, "worker", job.ID)
					if err != nil || claimed != nil {
						t.Fatal("retired work relaunched", claimed, err)
					}
				}
				if attemptStatus != "" {
					attempt, _ := store.GetLatestAttempt(ctx, item.ID)
					if attempt.Status != attemptStatus {
						t.Fatal("historical result overwritten")
					}
				}
			})
		}
	}
}
