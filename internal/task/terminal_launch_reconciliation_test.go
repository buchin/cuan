package task

import (
	"context"
	"testing"
	"time"
)

func TestTerminalReservedAttemptCannotLeaveGhostQueuedLaunch(t *testing.T) {
	terminalReservedLaunchContract(t, NewMemoryStore(), "project", "room")
}

func terminalReservedLaunchContract(t *testing.T, store Store, projectID, roomID string) {
	for _, state := range []string{AttemptStatusSucceeded, AttemptStatusFailed, AttemptStatusCanceled, AttemptStatusInterrupted, AttemptStatusRunning, AttemptStatusQueued} {
		t.Run(state, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().UTC()
			svc := NewService(store, nil)
			item := &Task{ID: "terminal-launch-task-" + state, ProjectID: projectID, Title: "Approved work", Priority: "normal", AssignedAgent: "agent", ExecutionCategory: ExecutionCategoryCode, Status: TaskStatusBlocked, Instructions: "Approved work", CreatedAt: now, UpdatedAt: now}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			attempt := &TaskAttempt{ID: attemptRequestID(item.ID, "reserved-request"), TaskID: item.ID, RunnerID: "fixture", AgentName: "agent", AttemptNumber: 1, Status: state, ErrorMessage: "original evidence", CreatedAt: now}
			if err := store.CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			launch, err := svc.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: "reserved-request", OwnerID: "owner", RoomID: roomID, SourceRequestID: "approved", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(item.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			newer, err := svc.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: "newer-request", OwnerID: "owner", RoomID: roomID, SourceRequestID: "approved", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(item.Instructions), CreatedAt: now.Add(time.Second), AvailableAt: now.Add(time.Second), ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if err := svc.ReconcileOrphanLaunches(ctx, now.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			old, _ := svc.GetLaunch(ctx, launch.ID)
			fresh, _ := svc.GetLaunch(ctx, newer.ID)
			terminal := state != AttemptStatusRunning && state != AttemptStatusQueued
			if (old.State == "stopped") != terminal || fresh.State != "queued" {
				t.Fatal("terminal intent not closed or newer approval affected")
			}
			d, _ := svc.GetTaskDetail(ctx, item.ID)
			if d.Status != TaskStatusBlocked || len(d.Attempts) != 1 || d.LatestAttempt.Status != state || d.LatestAttempt.ErrorMessage != "original evidence" {
				t.Fatal("execution history or task status rewritten")
			}
		})
	}
}
