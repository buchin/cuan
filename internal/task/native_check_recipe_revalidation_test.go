package task

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNativeCheckRecipeReviewRequiresAbortedNativeScriptAndSavedReplan(t *testing.T) {
	for _, mode := range []string{"valid", "assertion", "foreign runner", "missing report", "changed repository", "unsupported command", "continue", "needs owner", "old instructions", "missing reason", "cancelled", "foreign review"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().UTC()
			start, finish := now.Add(-time.Minute), now.Add(-time.Second)
			zero := 0
			item := &Task{ID: "work", Instructions: "Original approved DoD"}
			a := &TaskAttempt{ID: "check", TaskID: item.ID, RunnerID: "native", Status: AttemptStatusFailed, ExitCode: &zero, StartedAt: &start, FinishedAt: &finish, ErrorMessage: InspectionProjectVerificationTimeout}
			summary := "Project verification: test.sh --fast (exit -1)\nNative verifier result: Project verification timed out after 5 minutes"
			author := a.RunnerID
			baseline, final := strings.Repeat("a", 64), strings.Repeat("a", 64)
			record := outcomeRevalidationReviewRecord{RequestID: "saved-review", InstructionsHash: LaunchInstructionsHash(item.Instructions), EvidenceDigest: strings.Repeat("b", 64)}
			record.Assessment.Decision = "revise"
			record.Assessment.RequestedOutcome = "unfinished"
			record.Assessment.ManagerAction = "replan"
			record.Assessment.ManagerReason = "Native checks aborted; correct recipe before revalidation"
			record.Assessment.RemainingWork = "publication"
			record.Assessment.Revision = "Verify original outcome with corrected recipe"
			switch mode {
			case "assertion":
				summary += "\n--- FAIL: TestActualFailure"
			case "foreign runner":
				author = "other"
			case "changed repository":
				final = strings.Repeat("c", 64)
			case "unsupported command":
				summary = strings.Replace(summary, "test.sh --fast", "npm test", 1)
			case "continue":
				record.Assessment.ManagerAction = "continue"
			case "needs owner":
				record.Assessment.ManagerAction = "needs_owner"
			case "old instructions":
				record.InstructionsHash = "old"
			case "missing reason":
				record.Assessment.ManagerReason = ""
			case "cancelled":
				a.CancelRequested = true
			}
			raw, _ := json.Marshal(RunnerVerificationRecord{Summary: summary})
			scope, _ := json.Marshal(map[string]string{"baseline": baseline, "final": final})
			review, _ := json.Marshal(record)
			events := []*TaskActivity{{ID: "verification:" + a.ID, TaskID: item.ID, AttemptID: &a.ID, Author: author, ActivityType: ActivityTypeRunnerVerification, Message: string(raw)}, {ID: "repository-observation:" + a.ID, TaskID: item.ID, AttemptID: &a.ID, Author: a.RunnerID, ActivityType: "repository_scope_observed", NewStatus: AttemptStatusFailed, Message: string(scope)}, {ID: "review", TaskID: item.ID, AttemptID: &a.ID, Author: "coordinator", ActivityType: "coordinator_review_assessment", CreatedAt: now, Message: string(review)}}
			if mode != "missing report" {
				events = append(events, &TaskActivity{ID: "report", TaskID: item.ID, AttemptID: &a.ID, ActivityType: "agent_report", NewStatus: TaskStatusReview}, &TaskActivity{ID: "kind", TaskID: item.ID, AttemptID: &a.ID, ActivityType: "agent_result_kind", NewStatus: "implementation"})
			}
			request := "saved-review"
			if mode == "foreign review" {
				request = "other"
			}
			got, paths := NativeCheckRecipeReview(item, a, events, request)
			if mode == "valid" {
				if got != request || len(paths) != 1 || paths[0] != "scripts/test.sh" {
					t.Fatal("missing exact native recipe", got, paths)
				}
			} else if got != "" || paths != nil {
				t.Fatal("unsupported recovery admitted", got, paths)
			}
		})
	}
}
