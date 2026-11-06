package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestInboxCompletionCannotApplyLegacyWorkerReplanOrRunProjectCloneChecks(t *testing.T) {
	for _, scenario := range []string{"legacy replan", "legacy pass", "no worker evaluation", "failed execution"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			svc := NewService(NewMemoryStore(store), store)
			svc.SetDoDEvaluator(&DoDEvaluator{CustomCommandRunner: func(context.Context, string, string, ...string) ([]byte, error) {
				t.Fatal("Inbox completion re-ran checks against the project clone")
				return nil, nil
			}})
			now := time.Now().UTC()
			item := &task.Task{ID: "inbox-work", Title: "Extract the approved Dojo package", Instructions: "Cuan managed delegation v1.\nExtract the approved package.", SourceRoomID: "room", SourceMessageID: "approved", Status: task.TaskStatusDoing, Summary: "Package prepared", CompleteNote: "Actual targeted test output retained", CreatedAt: now, UpdatedAt: now}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			attempt := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "runner", AttemptNumber: 2, Status: task.AttemptStatusRunning, CreatedAt: now, StartedAt: &now}
			if err := store.CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			req := CompleteAttemptRequest{RunnerID: "runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: targeted tests (exit 0)"}
			switch scenario {
			case "legacy replan":
				req.DoDValidation = &DoDValidationResult{Action: "auto_requeue", Fixable: true, NextAttemptNumber: 99, RequeueInstructions: "Create the unrelated Cuan Landing Page inside Dojo", Gaps: []string{"Landing Page Cuan missing"}}
			case "legacy pass":
				req.DoDValidation = &DoDValidationResult{Passed: true, Action: "mark_done", Details: "Old worker claims all criteria passed"}
			case "failed execution":
				req.Status = task.AttemptStatusFailed
				req.ExitCode = 1
				req.ErrorMessage = "Actual failing assertion"
				req.DoDValidation = &DoDValidationResult{Passed: true, Action: "mark_done"}
			}
			completed, err := svc.Complete(ctx, attempt.ID, req)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := store.GetTask(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			attempts, err := store.ListAttempts(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(attempts) != 1 || saved.Status == task.TaskStatusDone || saved.Status == task.TaskStatusQueued {
				t.Fatalf("worker created or accepted work: status=%s attempts=%d", saved.Status, len(attempts))
			}
			if scenario == "failed execution" {
				if completed.Status != task.AttemptStatusFailed || completed.ErrorMessage != "Actual failing assertion" {
					t.Fatal("real failure was waived")
				}
				return
			}
			if completed.Status != task.AttemptStatusSucceeded || saved.Status != task.TaskStatusReview {
				t.Fatal("successful execution did not reach independent review")
			}
			activities, err := store.ListActivities(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, activity := range activities {
				if activity.ActivityType == "worker_dod_assessment" {
					found = true
					if activity.AttemptID == nil || *activity.AttemptID != attempt.ID {
						t.Fatal("assessment provenance lost")
					}
					if scenario == "legacy replan" && !strings.Contains(activity.Message, "Landing Page Cuan missing") {
						t.Fatal("legacy diagnostic lost")
					}
				}
			}
			if found != (req.DoDValidation != nil) {
				t.Fatal("worker assessment diagnostic not retained")
			}
		})
	}
}
