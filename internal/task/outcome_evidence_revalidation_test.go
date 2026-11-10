package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvidenceRevalidationUsesExistingAllowanceAndNativeParent(t *testing.T) {
	for _, scenario := range []string{"same target", "new target", "missing receipt", "forged receipt", "missing scope", "failed checks", "cancelled parent", "failed parent", "no manager basis", "owner decision", "changed review", "paths mixed", "unknown kind", "chained parent"} {
		t.Run(scenario, func(t *testing.T) {
			store, item, proof := outcomeRevalidationFixture(t)
			proof.RevalidationKind, proof.ReviewPaths = EvidenceRevalidation, nil
			codeKey := proof.RequestID(item.ID)
			previous := store.attempts[proof.PreviousAttemptID]
			review := map[string]any{"request_id": proof.ReviewRequestID, "instructions_hash": proof.InstructionsHash, "evidence_digest": strings.Repeat("d", 64), "assessment": map[string]string{"decision": "revise", "remaining_work": "verification", "revision": "Capture the actual RoomCard at390px and desktop, not the unrelated task card.", "manager_action": "replan", "manager_reason": "The original criterion lacks targeted evidence."}}
			switch scenario {
			case "same target":
				proof.TargetCommit = proof.PreviousCommit
			case "missing receipt":
				delete(store.activities, "live-result:"+previous.ID)
			case "forged receipt":
				store.activities["live-result:"+previous.ID].Author = "agent"
			case "missing scope":
				delete(store.activities, "scope")
			case "failed checks":
				store.activities["verification:"+previous.ID].Message = `{"summary":"Project verification: test.sh --fast (exit 1)"}`
			case "cancelled parent":
				previous.CancelRequested = true
			case "failed parent":
				previous.Status = AttemptStatusFailed
			case "no manager basis":
				review["assessment"].(map[string]string)["manager_reason"] = ""
			case "owner decision":
				review["assessment"].(map[string]string)["manager_action"] = "needs_owner"
			case "changed review":
				proof.ReviewRequestID = "another-review"
			case "paths mixed":
				proof.ReviewPaths = []string{"web/src/Other.tsx"}
			case "unknown kind":
				proof.RevalidationKind = "unbounded"
			case "chained parent":
				for _, launch := range store.launches {
					launch.Revision = strings.Replace(launch.Revision, `"purpose":"current_outcome"`, `"purpose":"current_outcome","previous_attempt_id":"already-revalidated"`, 1)
				}
			}
			raw, _ := json.Marshal(review)
			store.activities["review"].Message = string(raw)
			raw, _ = json.Marshal(proof)
			launch := &TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, RequestID: proof.RequestID(item.ID), InstructionsHash: proof.InstructionsHash, CreatedAt: proof.ObservedAt, Revision: LiveInspectionMarker + string(raw)}
			_, parseErr := LiveInspectionForLaunch(item, launch)
			err := ValidateLiveInspectionSource(context.Background(), store, item, proof)
			valid := scenario == "same target" || scenario == "new target"
			if (parseErr == nil && err == nil) != valid {
				t.Fatalf("invalid evidence revalidation: parse=%v source=%v", parseErr, err)
			}
			if valid && (proof.RequestID(item.ID) != codeKey || proof.SnapshotSourceCommit() != proof.PreviousCommit) {
				t.Fatal("evidence mode renewed the source allowance")
			}
		})
	}
}

func TestReportingEvidenceReviewRequiresExplicitAuthenticatedReplan(t *testing.T) {
	for _, mode := range []string{"verification", "publication", "continue", "needs input", "foreign record", "checks ran", "changed source", "cancelled", "old assessment"} {
		t.Run(mode, func(t *testing.T) {
			start, finish := time.Now().Add(-time.Minute), time.Now().Add(-time.Second)
			exit := 124
			item := &Task{ID: "task", Instructions: "Original scope"}
			previous := &TaskAttempt{ID: "inspection", TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusFailed, ExitCode: &exit, StartedAt: &start, FinishedAt: &finish, ErrorMessage: InspectionReportingTimeout}
			events := []*TaskActivity{
				{ID: "repository-observation:" + previous.ID, TaskID: item.ID, AttemptID: &previous.ID, Author: previous.RunnerID, ActivityType: "repository_scope_observed", NewStatus: AttemptStatusFailed, Message: `{"baseline":"` + strings.Repeat("b", 64) + `","final":"` + strings.Repeat("b", 64) + `"}`},
				{ID: "verification:" + previous.ID, TaskID: item.ID, AttemptID: &previous.ID, Author: previous.RunnerID, ActivityType: ActivityTypeRunnerVerification, Message: `{"summary":""}`},
				{ID: "timeout", TaskID: item.ID, AttemptID: &previous.ID, Author: previous.RunnerID, ActivityType: ActivityTypeAttempt, OldStatus: TaskStatusDoing, NewStatus: TaskStatusBlocked, CreatedAt: finish, Message: "Attempt selesai dengan status 'failed' (exit code: 124, peak RSS: 0 bytes)."},
			}
			assessment := map[string]string{"decision": "revise", "requested_outcome": "unfinished", "remaining_work": "verification", "revision": "Inspect remaining original outcome and save its evidence.", "manager_action": "replan", "manager_reason": "Native reporting stopped before a final report."}
			switch mode {
			case "publication":
				assessment["remaining_work"] = "publication"
			case "continue":
				assessment["manager_action"] = "continue"
			case "needs input":
				assessment["requested_outcome"] = "needs_input"
			case "foreign record":
				events[1].Author = "foreign"
			case "checks ran":
				events[1].Message = `{"summary":"Project verification: test.sh --fast (exit 1)"}`
			case "changed source":
				events[0].Message = `{"baseline":"` + strings.Repeat("b", 64) + `","final":"` + strings.Repeat("c", 64) + `"}`
			case "cancelled":
				previous.CancelRequested = true
			}
			raw, _ := json.Marshal(map[string]any{"request_id": "review", "instructions_hash": LaunchInstructionsHash(item.Instructions), "evidence_digest": strings.Repeat("d", 64), "assessment": assessment})
			review := &TaskActivity{ID: "review", TaskID: item.ID, AttemptID: &previous.ID, Author: "coordinator", ActivityType: "coordinator_review_assessment", CreatedAt: time.Now(), Message: string(raw)}
			if mode == "old assessment" {
				review.CreatedAt = start
			}
			events = append(events, review)
			request, _ := OutcomeEvidenceReview(item, previous, events, "")
			if (request != "") != (mode == "verification" || mode == "publication") {
				t.Fatal("incorrect reporting replan admission", mode, request)
			}
		})
	}
}
