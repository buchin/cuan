package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestInboxReviewSeparatesHistoricalRunnerCheckFromLatestFailure(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	id := "exit-two"
	d := &task.TaskDetailResponse{Task: task.Task{ID: "task", TestSummary: "OLD_SETUP_FAILURE"}, LatestAttempt: &task.TaskAttempt{ID: id, Status: task.AttemptStatusFailed, ErrorMessage: "exit code 2"}}
	for _, bound := range []bool{false, true} {
		if bound {
			d.Activities = append(d.Activities, &task.TaskActivity{TaskID: d.ID, AttemptID: &id, ActivityType: task.ActivityTypeRunnerVerification, Message: `{"summary":"CURRENT_VERIFIER"}`})
		}
		var p map[string]any
		if err := json.Unmarshal([]byte(s.inboxReviewData(context.Background(), d)), &p); err != nil {
			t.Fatal(err)
		}
		if p["historical_runner_verification"] != "OLD_SETUP_FAILURE" {
			t.Fatal("lost historical verifier")
		}
		if bound {
			if p["runner_verification"] != "CURRENT_VERIFIER" || p["runner_verification_attempt_id"] != id {
				t.Fatal("current verifier not bound")
			}
		} else if p["runner_verification"] != "" || p["runner_verification_attempt_id"] != "" {
			t.Fatal("historical verification became current failure cause")
		}
	}
}
