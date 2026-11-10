package task

import (
	"context"
	"testing"
	"time"
)

func TestWorkflowTurnSurvivesHandoffButReleasesForOwnerAndTerminalState(t *testing.T) {
	now := time.Now().UTC()
	start, finish := now.Add(-time.Minute), now.Add(-time.Second)
	for _, mode := range []string{"held", "owner", "done", "cancelled", "scope changed", "expired", "foreign receipt", "older receipt cannot override owner"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store := NewMemoryStore()
			owner := &Task{ID: "owner-task", ProjectID: "project", SourceRoomID: "room", SourceMessageID: "approval", Instructions: "Cuan managed delegation v1.\nImplement the approved result", Status: TaskStatusReview, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
			other := &Task{ID: "other-task", ProjectID: "project", Status: TaskStatusQueued, CreatedAt: now, UpdatedAt: now}
			a := &TaskAttempt{ID: "developer", TaskID: owner.ID, RunnerID: "native", Status: AttemptStatusSucceeded, StartedAt: &start, FinishedAt: &finish, CreatedAt: start}
			for _, item := range []*Task{owner, other} {
				if err := store.CreateTask(ctx, item); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.CreateAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			handoff := NewWorkflowHandoff(owner, a, finish)
			if handoff == nil {
				t.Fatal("native completion omitted handoff")
			}
			if mode == "foreign receipt" {
				handoff.Author = "worker"
			}
			if mode == "expired" {
				handoff.CreatedAt = now.Add(-WorkflowHandoffGrace - time.Second)
			}
			if err := store.CreateActivity(ctx, handoff); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "owner", "older receipt cannot override owner":
				if mode == "older receipt cannot override owner" {
					old := *a
					old.ID = "prior"
					old.CreatedAt = start.Add(-time.Minute)
					oldEnd := finish.Add(-time.Second)
					old.FinishedAt = &oldEnd
					if err := store.CreateAttempt(ctx, &old); err != nil {
						t.Fatal(err)
					}
					if err := store.CreateActivity(ctx, NewWorkflowHandoff(owner, &old, oldEnd)); err != nil {
						t.Fatal(err)
					}
				}
				if err := store.CreateActivity(ctx, &TaskActivity{ID: "decision", TaskID: owner.ID, AttemptID: &a.ID, Author: "coordinator", ActivityType: "coordinator_review_assessment", Message: `{"assessment":{"manager_action":"needs_owner","requested_outcome":"needs_input"}}`, CreatedAt: now}); err != nil {
					t.Fatal(err)
				}
			case "done":
				owner.Status = TaskStatusDone
			case "cancelled":
				a.CancelRequested = true
				if err := store.UpdateAttempt(ctx, a); err != nil {
					t.Fatal(err)
				}
			case "scope changed":
				owner.Instructions += " changed mandate"
			}
			if err := store.UpdateTask(ctx, owner); err != nil {
				t.Fatal(err)
			}
			blocked, err := store.IsProjectLaneActive(ctx, "project", ExecutionCategoryCode, other.ID)
			if err != nil {
				t.Fatal(err)
			}
			if blocked != (mode == "held") {
				t.Fatalf("project turn=%v in %s", blocked, mode)
			}
			if blocked, err := store.IsProjectLaneActive(ctx, "project", ExecutionCategoryCode, owner.ID); err != nil || blocked {
				t.Fatal("own continuation was blocked", err)
			}
			if blocked, err := store.IsProjectLaneActive(ctx, "project", ExecutionCategoryReadOnly, other.ID); err != nil || blocked {
				t.Fatal("handoff blocked reader pool", err)
			}
			job, err := store.SaveLaunch(ctx, TaskLaunch{TaskID: other.ID, ProjectID: "project", RequestID: "start", OwnerID: "user", RoomID: "room", SourceRequestID: "approval", AgentID: "agent", InstructionsHash: "hash", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			claimed, err := store.ClaimLaunch(ctx, now, "dispatcher", job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (claimed == nil) != (mode == "held") {
				t.Fatal("launch admission ignored workflow turn", mode)
			}
		})
	}
}
