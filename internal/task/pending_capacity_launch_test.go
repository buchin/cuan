package task

import (
	"context"
	"testing"
	"time"
)

func TestPendingApprovedCapacityLaunchCanRecoverWithoutReplayingFailure(t *testing.T) {
	pendingCapacityLaunchContract(t, NewMemoryStore(), "project", "room")
}

func pendingCapacityLaunchContract(t *testing.T, store Store, projectID, roomID string) {
	for _, scenario := range []string{"recover", "manual-block", "scope-changed", "cancel-requested", "business-failure", "wrong-exit", "same-request", "expired", "cancelled-launch", "active-attempt", "accepted"} {
		t.Run("pending-"+scenario, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().UTC()
			finished := now.Add(-time.Minute)
			exit := 75
			item := &Task{ID: "pending-capacity-" + scenario, ProjectID: projectID, Title: "Approved work", Priority: "normal", AssignedAgent: "agent", ExecutionCategory: ExecutionCategoryCode, Status: TaskStatusBlocked, Instructions: "Preserve approved scope", CreatedAt: now.Add(-time.Hour), UpdatedAt: finished}
			if scenario == "accepted" {
				item.Status = TaskStatusDone
			}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			sourceRequest := "91000000-0000-4000-8000-000000000001"
			pendingRequest := "91000000-0000-4000-8000-000000000002"
			a := &TaskAttempt{ID: attemptRequestID(item.ID, sourceRequest), TaskID: item.ID, RunnerID: "fixture", AgentName: "agent", AttemptNumber: 1, Status: AttemptStatusFailed, ExitCode: &exit, ErrorMessage: "The AI provider reported a usage limit. The run was stopped; saved work will continue.", CreatedAt: now.Add(-2 * time.Minute), FinishedAt: &finished}
			if scenario == "cancel-requested" {
				a.CancelRequested = true
			}
			if scenario == "business-failure" {
				a.ErrorMessage = "Actual implementation failed"
			}
			if scenario == "wrong-exit" {
				code := 1
				a.ExitCode = &code
			}
			if scenario == "same-request" {
				pendingRequest = sourceRequest
			}
			if err := store.CreateAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, nil)
			j, err := svc.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: projectID, RequestID: pendingRequest, OwnerID: "owner", RoomID: roomID, SourceRequestID: sourceRequest, AgentID: "agent", Revision: "Continue already-approved work", InstructionsHash: LaunchInstructionsHash(item.Instructions), CreatedAt: now.Add(-90 * time.Second), AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "manual-block" {
				if err := store.CreateActivity(ctx, &TaskActivity{ID: "pause-" + item.ID, TaskID: item.ID, ActivityType: ActivityTypeStatusChange, Author: "owner", NewStatus: TaskStatusBlocked, CreatedAt: finished.Add(time.Second)}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "scope-changed" {
				item.Instructions = "Changed scope"
				if err := store.UpdateTask(ctx, item); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "cancelled-launch" {
				if _, err := svc.CancelLaunches(ctx, item.ID, now); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "active-attempt" {
				if err := store.CreateAttempt(ctx, &TaskAttempt{ID: "active-" + item.ID, TaskID: item.ID, RunnerID: "fixture", AgentName: "agent", AttemptNumber: 2, Status: AttemptStatusRunning, CreatedAt: now}); err != nil {
					t.Fatal(err)
				}
			}
			reconcileAt := now
			if scenario == "expired" {
				reconcileAt = now.Add(2 * time.Hour)
			}
			if err := svc.ReconcileOrphanLaunches(ctx, reconcileAt); err != nil {
				t.Fatal(err)
			}
			after, _ := svc.GetTaskDetail(ctx, item.ID)
			want := item.Status
			if scenario == "recover" {
				want = TaskStatusQueued
			}
			if after.Status != want {
				t.Fatalf("status=%s want=%s", after.Status, want)
			}
			count := 1
			if scenario == "active-attempt" {
				count = 2
			}
			if len(after.Attempts) != count {
				t.Fatal("replayed or reset execution")
			}
			original, _ := store.GetAttempt(ctx, a.ID)
			if original.Status != a.Status || original.ErrorMessage != a.ErrorMessage {
				t.Fatal("historical failure rewritten")
			}
			if scenario == "recover" {
				saved, _ := svc.GetLaunch(ctx, j.ID)
				if saved.State != "queued" || saved.RequestID != pendingRequest {
					t.Fatal("pending approval replaced")
				}
				if err := svc.ReconcileOrphanLaunches(ctx, reconcileAt.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				fresh, _ := svc.GetTaskDetail(ctx, item.ID)
				if len(fresh.Activities) != len(after.Activities) {
					t.Fatal("reconciliation not idempotent")
				}
			}
		})
	}
}
