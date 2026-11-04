package task

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestStoppedLaunchBoardMemory(t *testing.T) {
	stoppedLaunchBoardContract(t, NewMemoryStore(), "launch-project")
}

func stoppedLaunchBoardContract(t *testing.T, store Store, projectID string) {
	t.Helper()
	ctx := context.Background()
	launches := store.(LaunchStore)
	for _, scenario := range []string{"stopped", "backoff", "newer", "running", "done", "cancelled", "stale-lease"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Microsecond)
			svc := NewService(store, nil)
			created, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: scenario, ProjectID: projectID, Instructions: "Approved scope"})
			if err != nil {
				t.Fatal(err)
			}
			status := TaskStatusQueued
			if scenario == "done" {
				status = TaskStatusDone
			}
			if scenario == "cancelled" {
				status = "cancelled"
			}
			item, _ := store.GetTask(ctx, created.ID)
			item.Status = status
			item.ExecutionCategory = ExecutionCategoryReadOnly
			if err = store.UpdateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			j, err := launches.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: projectID, RequestID: fmt.Sprintf("stop-%s", scenario), OwnerID: "owner", SourceRequestID: "approved", RoomID: "91000000-0000-4000-8000-000000000001", AgentID: "agent", InstructionsHash: "scope", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			// Claim first, then simulate state changes that must survive its outcome.
			if scenario == "done" || scenario == "cancelled" {
				item.Status = TaskStatusQueued
				store.UpdateTask(ctx, item)
			}
			claimed, err := launches.ClaimLaunch(ctx, now, "worker", j.ID)
			if err != nil || claimed == nil {
				t.Fatal(claimed, err)
			}
			if scenario == "done" || scenario == "cancelled" {
				item.Status = status
				store.UpdateTask(ctx, item)
			}
			if scenario == "newer" {
				other := *j
				other.ID = ""
				other.RequestID = "new-" + scenario
				other.CreatedAt = now.Add(time.Second)
				other.AvailableAt = other.CreatedAt
				if _, err = launches.SaveLaunch(ctx, other); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "running" {
				if err = store.CreateAttempt(ctx, &TaskAttempt{ID: "running-" + item.ID, TaskID: item.ID, RunnerID: "runner", AgentName: "agent", Status: AttemptStatusRunning, CreatedAt: now}); err != nil {
					t.Fatal(err)
				}
			}
			finishState := "stopped"
			if scenario == "backoff" {
				finishState = "queued"
			}
			token := claimed.LeaseToken
			if scenario == "stale-lease" {
				token = "wrong"
			}
			err = launches.FinishLaunch(ctx, j.ID, token, finishState, now.Add(time.Minute), "Workspace unavailable")
			if scenario == "stale-lease" {
				if err != ErrLaunchLease {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			result, err := store.GetTask(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			expected := status
			if scenario == "stopped" {
				expected = TaskStatusBlocked
			}
			if result.Status != expected {
				t.Fatalf("got %s, want %s", result.Status, expected)
			}
		})
	}
}
