package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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
