package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func failedCheckRevalidationFixture(t *testing.T) (*MemoryStore, *Task, *LiveInspectionIntent) {
	t.Helper()
	store, item, proof := outcomeRevalidationFixture(t)
	previous := store.attempts[proof.PreviousAttemptID]
	previous.Status = AttemptStatusFailed
	previous.ErrorMessage = "Live result inspection project checks did not pass"
	summary := "Project verification: test.sh --fast (exit 1)\n--- FAIL: TestOriginalCriterion (0.00s)\n    criterion_test.go:19: actual assertion failed\nFAIL\tgithub.com/fixture/repo/internal/runner\t1.2s\nFAIL\n"
	raw, _ := json.Marshal(RunnerVerificationRecord{Summary: summary})
	store.activities["verification:"+previous.ID].Message = string(raw)
	delete(store.activities, "scope")
	delete(store.activities, "live-result:"+previous.ID)
	observation, _ := json.Marshal(map[string]string{"baseline": strings.Repeat("c", 64), "final": strings.Repeat("c", 64)})
	stamp := previous.FinishedAt.Add(-time.Second)
	for _, event := range []*TaskActivity{
		{ID: "repository-observation:" + previous.ID, ActivityType: "repository_scope_observed", Author: previous.RunnerID, NewStatus: AttemptStatusFailed, Message: string(observation)},
		{ID: "current-final", ActivityType: "agent_report", Author: "agent", NewStatus: TaskStatusReview, Message: "Actual scoped final report"},
		{ID: "current-kind", ActivityType: "agent_result_kind", Author: "agent", NewStatus: "implementation"},
	} {
		event.TaskID = item.ID
		event.AttemptID = &previous.ID
		event.CreatedAt = stamp
		store.activities[event.ID] = event
	}
	var review map[string]any
	json.Unmarshal([]byte(store.activities["review"].Message), &review)
	review["assessment"].(map[string]any)["remaining_work"] = "verification"
	review["assessment"].(map[string]any)["revision"] = "Investigate the actual failed registered check without changing the read-only snapshot."
	raw, _ = json.Marshal(review)
	store.activities["review"].Message = string(raw)
	proof.ReviewPaths = []string{"internal/runner/criterion_test.go"}
	return store, item, proof
}

func TestFailedCheckRevalidationPreservesNativeFailureAndSingleAllowance(t *testing.T) {
	for _, scenario := range []string{"valid", "foreign owner", "wrong review", "wrong paths", "cancelled", "missing observation", "changed source", "wrong observation author", "wrong check author", "wrong check identity", "no final report", "progress only", "no kind", "compiler failure", "multiple packages", "no file", "wrong exit", "timeout", "chained"} {
		t.Run(scenario, func(t *testing.T) {
			store, item, proof := failedCheckRevalidationFixture(t)
			previous := store.attempts[proof.PreviousAttemptID]
			check := store.activities["verification:"+previous.ID]
			observation := store.activities["repository-observation:"+previous.ID]
			replaceSummary := func(old, new string) { check.Message = strings.ReplaceAll(check.Message, old, new) }
			switch scenario {
			case "foreign owner":
				proof.OwnerID = "foreign"
			case "wrong review":
				proof.ReviewRequestID = "other"
			case "wrong paths":
				proof.ReviewPaths = []string{"internal/unrelated_test.go"}
			case "cancelled":
				previous.CancelRequested = true
			case "missing observation":
				delete(store.activities, observation.ID)
			case "changed source":
				observation.Message = strings.Replace(observation.Message, `"final":"`+strings.Repeat("c", 64), `"final":"`+strings.Repeat("d", 64), 1)
			case "wrong observation author":
				observation.Author = "agent"
			case "wrong check author":
				check.Author = "agent"
			case "wrong check identity":
				check.ID = "unbound"
			case "no final report":
				delete(store.activities, "current-final")
			case "progress only":
				store.activities["current-final"].NewStatus = "progress"
			case "no kind":
				delete(store.activities, "current-kind")
			case "compiler failure":
				replaceSummary("--- FAIL: TestOriginalCriterion", "compiler unavailable")
			case "multiple packages":
				replaceSummary(`FAIL\n`, `FAIL\tgithub.com/other/repo/pkg/other\t1.0s\nFAIL\n`)
			case "no file":
				replaceSummary("criterion_test.go:19:", "no source file")
			case "wrong exit":
				replaceSummary("(exit 1)", "(exit 124)")
			case "timeout":
				previous.ErrorMessage = "Execution timed out after adaptive timeout (10m0s)"
			case "chained":
				for _, launch := range store.launches {
					launch.Revision = strings.Replace(launch.Revision, `"purpose":"current_outcome"`, `"purpose":"current_outcome","previous_attempt_id":"earlier"`, 1)
				}
			}
			err := ValidateLiveInspectionSource(context.Background(), store, item, proof)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("incorrect failed-check eligibility %s: %v", scenario, err)
			}
			if previous.Status != AttemptStatusFailed {
				t.Fatal("old failure relabelled")
			}
			if scenario == "valid" {
				id := proof.RequestID(item.ID)
				proof.TargetCommit = strings.Repeat("f", 40)
				if id != proof.RequestID(item.ID) {
					t.Fatal("new deploy reset revalidation allowance")
				}
			}
		})
	}
}
