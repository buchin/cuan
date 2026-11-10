package task

import (
	"strings"
	"testing"
	"time"
)

func TestInspectionProjectDeadlineRequiresNativeIncompleteChecks(t *testing.T) {
	for _, mode := range []string{"valid", "legacy abort", "legacy assertion", "command timeout", "command timeout with assertion", "command timeout cancelled", "unchanged", "cancelled", "running", "missing finish", "nonzero provider exit", "different failure", "missing report", "blocked report", "missing kind", "missing checks", "foreign runner", "wrong record", "assertion failure", "successful checks", "missing boundary", "changed boundary"} {
		t.Run(mode, func(t *testing.T) {
			start, end := time.Now().Add(-time.Minute), time.Now()
			exit := 0
			a := &TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "runner", Status: AttemptStatusFailed, ExitCode: &exit, StartedAt: &start, FinishedAt: &end, ErrorMessage: InspectionProjectVerificationDeadline}
			events := []*TaskActivity{
				{ID: "report", TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: "agent_report", NewStatus: TaskStatusReview},
				{ID: "kind", TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: "agent_result_kind", NewStatus: "implementation"},
				{ID: "verification:" + a.ID, TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: ActivityTypeRunnerVerification, Author: a.RunnerID, Message: `{"summary":"Project verification: test.sh --fast (exit -1)\nok config"}`},
				{ID: "repository-observation:" + a.ID, TaskID: a.TaskID, AttemptID: &a.ID, ActivityType: "repository_scope_observed", Author: a.RunnerID, NewStatus: AttemptStatusFailed, Message: `{"baseline":"` + strings.Repeat("b", 64) + `","final":""}`},
			}
			switch mode {
			case "legacy abort", "legacy assertion":
				a.ErrorMessage = "Live result inspection project checks did not pass"
				if mode == "legacy assertion" {
					events[2].Message = `{"summary":"Project verification: test.sh --fast (exit -1)\n--- FAIL: TestBehavior"}`
				}
			case "command timeout", "command timeout with assertion", "command timeout cancelled":
				a.ErrorMessage = InspectionProjectVerificationTimeout
				if mode == "command timeout with assertion" {
					events[2].Message = `{"summary":"Project verification: test.sh --fast (exit -1)\n--- FAIL: TestBehavior"}`
				}
				a.CancelRequested = mode == "command timeout cancelled"
			case "unchanged":
				events[3].Message = `{"baseline":"` + strings.Repeat("b", 64) + `","final":"` + strings.Repeat("b", 64) + `"}`
			case "cancelled":
				a.CancelRequested = true
			case "running":
				a.Status = AttemptStatusRunning
			case "missing finish":
				a.FinishedAt = nil
			case "nonzero provider exit":
				exit = 1
			case "different failure":
				a.ErrorMessage = "Project verification failed"
			case "missing report":
				events[0] = nil
			case "blocked report":
				events[0].NewStatus = TaskStatusBlocked
			case "missing kind":
				events[1] = nil
			case "missing checks":
				events[2] = nil
			case "foreign runner":
				events[2].Author = "other"
			case "wrong record":
				events[2].ID = "forged"
			case "assertion failure":
				events[2].Message = `{"summary":"Project verification: test.sh --fast (exit -1)\n--- FAIL: TestBehavior"}`
			case "successful checks":
				events[2].Message = `{"summary":"Project verification: test.sh --fast (exit 0)\nok config"}`
			case "missing boundary":
				events[3] = nil
			case "changed boundary":
				events[3].Message = `{"baseline":"` + strings.Repeat("b", 64) + `","final":"` + strings.Repeat("c", 64) + `"}`
			}
			if got, want := InspectionInfrastructureFailure(a, events), mode == "legacy abort" || mode == "valid" || mode == "command timeout" || mode == "unchanged"; got != want {
				t.Fatalf("deadline infrastructure=%v want=%v", got, want)
			}
		})
	}
}
