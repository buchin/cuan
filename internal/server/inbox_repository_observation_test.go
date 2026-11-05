package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestFailedRepositoryObservationRequiresExactRunnerProvenance(t *testing.T) {
	for _, scenario := range []string{"unchanged", "changed", "unavailable", "foreign_runner", "stale_attempt", "foreign_task", "wrong_id", "wrong_status", "malformed", "canceled", "successful", "not_readonly"} {
		t.Run(scenario, func(t *testing.T) {
			attempt := "attempt"
			evidence := runner.RepositoryVerification{Baseline: strings.Repeat("a", 64), Final: strings.Repeat("a", 64)}
			if scenario == "changed" {
				evidence.Final = strings.Repeat("b", 64)
			}
			if scenario == "unavailable" {
				evidence.Final = ""
			}
			if scenario == "malformed" {
				evidence.Final = "invalid"
			}
			raw, _ := json.Marshal(evidence)
			event := &task.TaskActivity{ID: "repository-observation:attempt", TaskID: "task", AttemptID: &attempt, ActivityType: "repository_scope_observed", Author: "runner", NewStatus: task.AttemptStatusFailed, Message: string(raw)}
			latest := &task.TaskAttempt{ID: "attempt", TaskID: "task", RunnerID: "runner", Status: task.AttemptStatusFailed}
			switch scenario {
			case "foreign_runner":
				event.Author = "agent"
			case "stale_attempt":
				attempt = "previous"
			case "foreign_task":
				event.TaskID = "foreign"
			case "wrong_id":
				event.ID = "fake"
			case "wrong_status":
				event.NewStatus = task.AttemptStatusSucceeded
			case "canceled":
				latest.CancelRequested = true
			case "successful":
				latest.Status = task.AttemptStatusSucceeded
			}
			d := &task.TaskDetailResponse{Task: task.Task{ID: "task"}, LatestAttempt: latest, Activities: []*task.TaskActivity{event}}
			got := failedRepositoryObservation(d, scenario != "not_readonly")
			valid := scenario == "unchanged" || scenario == "changed" || scenario == "unavailable"
			if (got != nil) != valid {
				t.Fatal("incorrect evidence admission", got)
			}
			if valid && (got["repository_state_at_observation"] != scenario || got["execution_status"] != task.AttemptStatusFailed) {
				t.Fatal("observation became a different claim", got)
			}
		})
	}
}
