package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeProjectCheckFailurePrecedesPublication(t *testing.T) {
	for _, scenario := range []string{"current", "foreign runner", "historical", "passing checks", "cancelled", "missing report", "unknown failure"} {
		t.Run(scenario, func(t *testing.T) {
			s, d := prerequisiteReviewFixture(t)
			d.ExecutionCategory = task.ExecutionCategoryCode
			d.LatestAttempt.ErrorMessage = "Project verification failed"
			for _, e := range d.Activities {
				if e.ID == "verification:"+d.LatestAttempt.ID {
					summary := "Project verification: test.sh --fast (exit 1)\nFAIL TestSupplementalVerifierUsesPrivateExactCommitWorkspace: different completion already retained"
					if scenario == "passing checks" {
						summary = "Project verification: test.sh --fast (exit 0)"
					}
					raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: summary})
					e.Message = string(raw)
					if scenario == "foreign runner" {
						e.Author = "foreign"
					}
					if scenario == "historical" {
						old := "old-attempt"
						e.AttemptID = &old
					}
				}
				if scenario == "missing report" && e.ID == "native-final-report" {
					e.ActivityType = "comment"
				}
			}
			if scenario == "cancelled" {
				d.LatestAttempt.CancelRequested = true
			}
			if scenario == "unknown failure" {
				d.LatestAttempt.ErrorMessage = "Unknown transport failure"
			}
			input := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Revision: "First reconcile the registered project's existing release and saved deployment effects."}
			got := s.requirePublicationReview(context.Background(), d, input)
			if scenario != "current" {
				if got.RemainingWork == "implementation" {
					t.Fatal("untrusted failure admitted source correction", got)
				}
				return
			}
			if got.RemainingWork != "implementation" || got.ManagerAction != "replan" || got.SourceReview != "not_checked" || !strings.Contains(got.Revision, "TestSupplementalVerifierUsesPrivateExactCommitWorkspace") {
				t.Fatal("concrete test failure became publication reconciliation", got)
			}
			if again := s.requirePublicationReview(context.Background(), d, got); again != got {
				t.Fatal("reapplying changed continuation identity")
			}
			input.Decision = "pass"
			if pass := s.requirePublicationReview(context.Background(), d, input); pass.Decision != "revise" {
				t.Fatal("model pass erased native test failure")
			}
		})
	}
}

func TestNativeCheckHandoffUsesExistingCorrectionLaunch(t *testing.T) {
	s, d := prerequisiteReviewFixture(t)
	ctx := context.Background()
	d.ExecutionCategory = task.ExecutionCategoryCode
	d.LatestAttempt.ErrorMessage = "Project verification failed"
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().UpdateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}
	for _, e := range d.Activities {
		if e.ID == "verification:"+d.LatestAttempt.ID {
			raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 1)\n" + strings.Repeat("successful setup chatter\n", 600) + "--- FAIL: exact-source completion fixture\nFAIL"})
			e.Message = string(raw)
			if err := s.taskSvc.Store().CreateActivity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	s.queueInboxTaskReview(ctx, d)
	requests := reviewRequests(t, s, d.SourceRoomID)
	q := requests[len(requests)-1]
	raw, ready := s.inboxNativeCheckHandoff(ctx, q)
	if !ready {
		t.Fatal("native failure still requires a model decision")
	}
	decision, err := parseInboxReviewEvaluation(raw)
	if err != nil || len(decision.Revision) > 6000 {
		t.Fatal("native handoff exceeds the real review/delegation contract", err)
	}
	foreign := q
	foreign.MessageID = db.GenerateUUID()
	if _, ready := s.inboxNativeCheckHandoff(ctx, foreign); ready {
		t.Fatal("foreign review admitted correction")
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy-check-correction", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		s.applyInboxReviewEvaluation(ctx, q, raw)
		current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
		launch, launchErr := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
		if err != nil || launchErr != nil || launch == nil || launch.State != "queued" || current.Status == task.TaskStatusDone || len(current.Attempts) != len(d.Attempts) {
			t.Fatalf("correction bypassed native queue or duplicated execution: %v %v %+v", err, launchErr, launch)
		}
		if !strings.Contains(launch.Revision, "exact-source completion fixture") {
			t.Fatal("correction lost the actual failing check", launch.Revision)
		}
	}
}
