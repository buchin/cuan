package task

import (
	"context"
	"testing"
	"time"
)

func TestUnstartedWorkspaceRecoveryMemory(t *testing.T) {
	unstartedWorkspaceRecoveryContract(t, NewMemoryStore(), "launch-project")
}
func unstartedWorkspaceRecoveryContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	launches := store.(LaunchStore)
	recovery := store.(launchRecoveryStore)
	for _, scenario := range []string{"recover", "legacy-agent", "custom-agent-error", "no-revision", "scope-changed", "newer-intent", "already-reserved", "active-attempt", "cancelled", "done", "expired", "stale-observation", "wrong-blocker"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Microsecond)
			svc := NewService(store, nil)
			item, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{ProjectID: project, Title: scenario, Instructions: "Approved read-only audit", ExecutionCategory: ExecutionCategoryReadOnly})
			if err != nil {
				t.Fatal(err)
			}
			item.Status = TaskStatusQueued
			if err = store.UpdateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			input := TaskLaunch{TaskID: item.ID, ProjectID: project, RequestID: "recovery-" + item.ID, OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", SourceRequestID: "owner-approved", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(item.Instructions), Revision: "Inspect retained work and verify original criteria", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
			if scenario == "legacy-agent" {
				input.AgentID = "agent_sm"
			}
			if scenario == "no-revision" {
				input.Revision = ""
			}
			j, err := launches.SaveLaunch(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			j, err = launches.ClaimLaunch(ctx, now, "worker", j.ID)
			if err != nil || j == nil {
				t.Fatal(j, err)
			}
			if scenario == "already-reserved" || scenario == "active-attempt" {
				id := attemptRequestID(item.ID, input.RequestID)
				if scenario == "active-attempt" {
					id = "other-" + item.ID
				}
				a := &TaskAttempt{ID: id, TaskID: item.ID, RunnerID: "fixture", AgentName: "agent", Status: AttemptStatusQueued, CreatedAt: now}
				if err = store.CreateAttempt(ctx, a); err != nil {
					t.Fatal(err)
				}
				if scenario == "already-reserved" {
					a.Status = AttemptStatusFailed
					if err = store.UpdateAttempt(ctx, a); err != nil {
						t.Fatal(err)
					}
				}
			}
			reason := MissingWorkspaceLaunchError
			if scenario == "legacy-agent" || scenario == "custom-agent-error" {
				reason = "create attempt: Invalid task input"
			}
			if scenario == "wrong-blocker" {
				reason = "The saved task scope has changed"
			}
			if scenario == "cancelled" {
				if _, err = launches.CancelLaunches(ctx, item.ID, now); err != nil {
					t.Fatal(err)
				}
			} else if err = launches.FinishLaunch(ctx, j.ID, j.LeaseToken, "stopped", now, reason); err != nil {
				t.Fatal(err)
			}
			j, err = launches.GetLaunch(ctx, j.ID)
			if err != nil {
				t.Fatal(err)
			}
			current, _ := store.GetTask(ctx, item.ID)
			switch scenario {
			case "scope-changed":
				current.Instructions = "Different scope"
				store.UpdateTask(ctx, current)
			case "done":
				current.Status = TaskStatusDone
				store.UpdateTask(ctx, current)
			case "cancelled":
				current.Status = "cancelled"
				store.UpdateTask(ctx, current)
			case "newer-intent":
				next := input
				next.RequestID = "new-" + item.ID
				next.CreatedAt = now.Add(time.Second)
				next.AvailableAt = next.CreatedAt
				if _, err = launches.SaveLaunch(ctx, next); err != nil {
					t.Fatal(err)
				}
			}
			expected := j.UpdatedAt
			at := time.Now().UTC()
			if scenario == "expired" {
				at = now.Add(2 * time.Hour)
			}
			if scenario == "stale-observation" {
				expected = expected.Add(-time.Second)
			}
			recovered, err := recovery.RecoverUnstartedLaunch(ctx, j.ID, expected, at)
			if err != nil || recovered != (scenario == "recover" || scenario == "legacy-agent") {
				t.Fatalf("recovery %v: %v", recovered, err)
			}
			if !recovered {
				return
			}
			after, _ := store.GetTask(ctx, item.ID)
			if after.Status != TaskStatusQueued || after.Instructions != item.Instructions {
				t.Fatal("scope or queue state changed incorrectly")
			}
			activities, _ := store.ListActivities(ctx, item.ID)
			count := 0
			for _, a := range activities {
				if a.ActivityType == "execution_recovery" {
					count++
				}
			}
			if count != 1 {
				t.Fatal("missing recovery audit record", count)
			}
			owned, err := launches.ClaimLaunch(ctx, at, "after-repair", j.ID)
			if err != nil || owned == nil {
				t.Fatal(owned, err)
			}
			if err = launches.FinishLaunch(ctx, owned.ID, owned.LeaseToken, "stopped", at, reason); err != nil {
				t.Fatal(err)
			}
			failed, _ := launches.GetLaunch(ctx, j.ID)
			if again, e := recovery.RecoverUnstartedLaunch(ctx, j.ID, failed.UpdatedAt, at); e != nil || again {
				t.Fatal("replayed a consumed recovery receipt", again, e)
			}
		})
	}
}
