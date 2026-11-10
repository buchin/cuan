package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestManagedSourceTimeoutCannotInspectUnrelatedProduction(t *testing.T) {
	for _, mode := range []string{"managed", "ordinary", "foreign author", "foreign task", "changed instructions", "duplicate", "cancelled", "unknown field", "future record"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			store := NewMemoryStore()
			now, exit := time.Now().UTC(), 124
			start := now.Add(-time.Minute)
			item := &Task{ID: "work", ProjectID: "project", SourceRoomID: "room", ExecutionCategory: ExecutionCategoryCode, Instructions: "Implement and publish approved work", Status: TaskStatusBlocked}
			attempt := &TaskAttempt{ID: "developer", TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusFailed, StartedAt: &start, FinishedAt: &now, ExitCode: &exit, ErrorMessage: "Execution timed out after adaptive timeout (25m0s)"}
			claim := ManagedReleaseClaim{Version: 1, TaskID: item.ID, ProjectID: item.ProjectID, RoomID: item.SourceRoomID, AttemptID: attempt.ID, RunnerID: attempt.RunnerID, InstructionsHash: LaunchInstructionsHash(item.Instructions), Repository: "owner/repo", TargetBranch: "dev"}
			raw, _ := json.Marshal(claim)
			event := &TaskActivity{ID: "managed-release-claim:" + attempt.ID, TaskID: item.ID, AttemptID: &attempt.ID, Author: attempt.RunnerID, ActivityType: "managed_release_claim", Message: string(raw), CreatedAt: start.Add(time.Second)}
			switch mode {
			case "foreign author":
				event.Author = "model"
			case "foreign task":
				event.TaskID = "other"
			case "changed instructions":
				item.Instructions += " changed"
			case "cancelled":
				attempt.CancelRequested = true
			case "unknown field":
				event.Message = strings.TrimSuffix(event.Message, "}") + `,"extra":true}`
			case "future record":
				event.CreatedAt = now.Add(time.Hour)
			}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if err := store.CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			var events []*TaskActivity
			if mode != "ordinary" {
				events = append(events, event)
				if err := store.CreateActivity(ctx, event); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "duplicate" {
				events = append(events, event)
			}
			unfinished, err := IncompleteManagedSourceTimeout(item, attempt, events)
			if mode == "managed" && (err != nil || !unfinished) {
				t.Fatalf("known unfinished source was lost: %v", err)
			}
			if mode != "managed" && mode != "ordinary" && !attempt.CancelRequested && err == nil {
				t.Fatal("invalid native admission accepted")
			}
			if mode == "ordinary" && (err != nil || unfinished) {
				t.Fatal("ordinary timeout misclassified as managed")
			}
			// Revalidation is shared by already-saved launch admission: a legacy
			// current-outcome intent cannot bypass the source-stage boundary.
			proof := &LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: attempt.ID, InstructionsHash: LaunchInstructionsHash(item.Instructions), ObservedAt: now.Add(time.Second)}
			validation := ValidateLiveInspectionSource(ctx, store, item, proof)
			// A foreign task's record is absent from this task's saved activity
			// list, so it cannot establish managed ownership or disable ordinary
			// reconciliation of unknown effects.
			if (validation == nil) != (mode == "ordinary" || mode == "foreign task") {
				t.Fatalf("mode=%s source validation=%v", mode, validation)
			}
		})
	}
}
