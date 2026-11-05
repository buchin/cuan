package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func outcomeRevalidationFixture(t *testing.T) (*MemoryStore, *Task, *LiveInspectionIntent) {
	t.Helper()
	store := NewMemoryStore()
	now := time.Now().UTC()
	item := &Task{ID: "work", ProjectID: "project", SourceRoomID: "room", Instructions: "Original approved requirements", Status: TaskStatusBlocked}
	sourceEnd := now.Add(-20 * time.Minute)
	exit := 124
	source := &TaskAttempt{ID: "original-failure", TaskID: item.ID, Status: AttemptStatusFailed, ExitCode: &exit, ErrorMessage: "Execution timed out after adaptive timeout (5m)", FinishedAt: &sourceEnd}
	store.tasks[item.ID] = item
	store.attempts[source.ID] = source
	binding := LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: source.ID, TargetCommit: strings.Repeat("a", 40), Repository: "fixture/repo", OwnerID: "owner", RoomID: "room", SourceRequestID: "original-request", InstructionsHash: LaunchInstructionsHash(item.Instructions), ObservedAt: sourceEnd.Add(time.Minute)}
	request := binding.RequestID(item.ID)
	previousID := attemptRequestID(item.ID, request)
	end := now.Add(-10 * time.Minute)
	zero := 0
	previous := &TaskAttempt{ID: previousID, TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusSucceeded, ExitCode: &zero, FinishedAt: &end}
	store.attempts[previousID] = previous
	raw, _ := json.Marshal(binding)
	store.launches[LaunchID(item.ID, request)] = &TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, OwnerID: binding.OwnerID, RoomID: binding.RoomID, SourceRequestID: binding.SourceRequestID, RequestID: request, InstructionsHash: binding.InstructionsHash, CreatedAt: binding.ObservedAt, Revision: LiveInspectionMarker + string(raw)}
	checks := "Project verification: test.sh --fast (exit 0)"
	receipt, _ := json.Marshal(LiveInspectionReceipt{Purpose: CurrentOutcomeInspection, SourceAttemptID: source.ID, TargetCommit: binding.TargetCommit, Repository: binding.Repository, ProjectChecksHash: VerificationSummaryHash(checks)})
	review, _ := json.Marshal(map[string]any{"request_id": "rejected-review", "instructions_hash": binding.InstructionsHash, "assessment": map[string]string{"decision": "revise", "remaining_work": "implementation", "revision": "Fix internal/server/terminal_handlers.go and inspect internal/server/task_agent_test.go."}})
	for _, event := range []*TaskActivity{
		{ID: "verification:" + previousID, ActivityType: ActivityTypeRunnerVerification, Author: "runner", Message: `{"summary":"` + checks + `"}`},
		{ID: "scope", ActivityType: "repository_scope_verified", Author: "runner", NewStatus: "read_only", Message: repositoryScopeReceiptPrefix + strings.Repeat("c", 64)},
		{ID: "live-result:" + previousID, ActivityType: ActivityLiveInspection, Author: "runner", Message: string(receipt)},
		{ID: "review", ActivityType: "coordinator_review_assessment", Author: "coordinator", Message: string(review)},
	} {
		event.TaskID = item.ID
		event.AttemptID = &previousID
		event.CreatedAt = end.Add(time.Minute)
		store.activities[event.ID] = event
	}
	proof := binding
	proof.PreviousAttemptID = previousID
	proof.PreviousCommit = binding.TargetCommit
	proof.TargetCommit = strings.Repeat("b", 40)
	proof.ObservedAt = now
	proof.ReviewRequestID = "rejected-review"
	proof.ReviewPaths = []string{"internal/server/task_agent_test.go", "internal/server/terminal_handlers.go"}
	return store, item, &proof
}

func TestOutcomeRevalidationRequiresBoundRejectedInspection(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong owner", "wrong paths", "missing request", "canceled previous", "failed previous", "missing scope", "failed checks", "missing receipt", "newer approval", "chained inspection"} {
		t.Run(scenario, func(t *testing.T) {
			store, item, proof := outcomeRevalidationFixture(t)
			previous := store.attempts[proof.PreviousAttemptID]
			switch scenario {
			case "wrong owner":
				proof.OwnerID = "foreign"
			case "wrong paths":
				proof.ReviewPaths = []string{"internal/unrelated.go"}
			case "missing request":
				proof.ReviewRequestID = ""
			case "canceled previous":
				previous.CancelRequested = true
			case "failed previous":
				previous.Status = AttemptStatusFailed
			case "missing scope":
				delete(store.activities, "scope")
			case "failed checks":
				store.activities["verification:"+previous.ID].Message = `{"summary":"Project verification: test.sh --fast (exit 1)"}`
			case "missing receipt":
				delete(store.activities, "live-result:"+previous.ID)
			case "newer approval":
				store.activities["review"].Message = strings.Replace(store.activities["review"].Message, `"revise"`, `"pass"`, 1)
			case "chained inspection":
				for _, launch := range store.launches {
					launch.Revision = strings.Replace(launch.Revision, `"purpose":"current_outcome"`, `"purpose":"current_outcome","previous_attempt_id":"already-revalidated"`, 1)
				}
			}
			err := ValidateLiveInspectionSource(context.Background(), store, item, proof)
			if (err == nil) != (scenario == "valid") {
				t.Fatal("unexpected validation result", err)
			}
			if scenario == "valid" {
				key := proof.RequestID(item.ID)
				proof.TargetCommit = strings.Repeat("d", 40)
				if proof.RequestID(item.ID) != key || proof.SnapshotSourceCommit() != proof.PreviousCommit {
					t.Fatal("deploy renewed allowance or lost inspected baseline")
				}
			}
		})
	}
}

func TestOutcomeRevalidationReviewRejectsGenericOrUnsafePaths(t *testing.T) {
	for _, revision := range []string{"Investigate again", "Change internal/../secrets.go", "Change /home/member/private.go", "Change /tmp/internal/server/private.go"} {
		store, item, proof := outcomeRevalidationFixture(t)
		event := store.activities["review"]
		var value map[string]any
		json.Unmarshal([]byte(event.Message), &value)
		value["assessment"].(map[string]any)["revision"] = revision
		raw, _ := json.Marshal(value)
		event.Message = string(raw)
		if err := ValidateLiveInspectionSource(context.Background(), store, item, proof); err == nil {
			t.Fatal("generic or unsafe revision granted revalidation")
		}
	}
}
