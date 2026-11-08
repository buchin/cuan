package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPublicationReconciliationRequiresNativeSourceChecksAndVerificationReview(t *testing.T) {
	for _, scenario := range []string{"valid", "missing commit", "wrong commit", "failed checks", "no verifier", "wrong author", "foreign record", "missing final report", "progress only", "implementation review", "wrong scope", "unreviewed", "cancelled", "failed process", "other failure", "no finish", "future finish", "owner prohibition"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC()
			created := now.Add(-3 * time.Minute)
			started := created.Add(time.Second)
			finished := now.Add(-time.Minute)
			exit := 0
			item := &Task{ID: "task", ProjectID: "project", SourceRoomID: "room", Instructions: "Cuan managed delegation v1.\nFix the original UI and verify its running result.", CommitSHA: strings.Repeat("a", 40), ExecutionCategory: ExecutionCategoryCode, Status: TaskStatusBlocked}
			source := &TaskAttempt{ID: "source", TaskID: item.ID, RunnerID: "runner", Status: AttemptStatusFailed, ExitCode: &exit, CreatedAt: created, StartedAt: &started, FinishedAt: &finished, ErrorMessage: PublicationVerificationUnavailableMessage}
			report := &TaskActivity{ID: "report", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: TaskStatusReview, CreatedAt: finished}
			kind := &TaskActivity{ID: "kind", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: finished}
			native := &TaskActivity{ID: "verification:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, ActivityType: ActivityTypeRunnerVerification, Author: source.RunnerID, CreatedAt: finished}
			record := RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 0)\nActual native check output", Changes: NewRunnerChangeRecord(item.CommitSHA, "src/session.tsx | 2 +-")}
			review := &TaskActivity{ID: "review", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "coordinator_review_assessment", Author: "coordinator", CreatedAt: now}
			decision := "verification"
			events := []*TaskActivity{report, kind, native, review}
			switch scenario {
			case "missing commit":
				item.CommitSHA = ""
			case "wrong commit":
				record.Changes.ResultCommit = strings.Repeat("b", 40)
			case "failed checks":
				record.Summary = "Project verification: test.sh --fast (exit 1)\nActual failure"
			case "no verifier":
				record.Summary = "No supported project verifier was available; agent report evidence retained."
			case "wrong author":
				native.Author = "agent"
			case "foreign record":
				native.TaskID = "other"
			case "missing final report":
				events = []*TaskActivity{kind, native, review}
			case "progress only":
				report.NewStatus = "progress"
			case "implementation review":
				decision = "implementation"
			case "wrong scope":
				item.Instructions += "\nChanged approved requirements."
			case "unreviewed":
				events = []*TaskActivity{report, kind, native}
			case "cancelled":
				source.CancelRequested = true
			case "failed process":
				exit = 1
			case "other failure":
				source.ErrorMessage = "Project verification failed"
			case "no finish":
				source.FinishedAt = nil
			case "future finish":
				future := now.Add(time.Minute)
				source.FinishedAt = &future
			case "owner prohibition":
				item.Instructions += "\nDo not deploy."
			}
			nativeRaw, _ := json.Marshal(record)
			native.Message = string(nativeRaw)
			instructions := item.Instructions
			if scenario == "wrong scope" {
				instructions = strings.TrimSuffix(instructions, "\nChanged approved requirements.")
			}
			reviewRaw, _ := json.Marshal(map[string]any{"request_id": "review-request", "instructions_hash": LaunchInstructionsHash(instructions), "evidence_digest": strings.Repeat("d", 64), "assessment": map[string]string{"decision": "revise", "remaining_work": decision}})
			review.Message = string(reviewRaw)
			detail := &TaskDetailResponse{Task: *item, LatestAttempt: source, Activities: events}
			if VerificationSourceChecksPassed(detail) != (scenario == "valid") {
				t.Fatalf("unpublished source verification admission failed for %s", scenario)
			}
			if ProjectChecksPassed(detail) {
				t.Fatal("failed publication became successful execution")
			}
			eligible := IsPublicationReconciliationOutcome(item, source, events)
			if eligible != (scenario == "valid") {
				t.Fatalf("incorrect eligibility for %s", scenario)
			}
			store := NewMemoryStore()
			ctx := context.Background()
			store.CreateTask(ctx, item)
			store.CreateAttempt(ctx, source)
			for _, event := range events {
				store.CreateActivity(ctx, event)
			}
			proof := &LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, TargetCommit: strings.Repeat("b", 40), InstructionsHash: LaunchInstructionsHash(item.Instructions), ObservedAt: now}
			if err := ValidateLiveInspectionSource(ctx, store, item, proof); (err == nil) != (scenario == "valid") {
				t.Fatalf("source proof bypassed eligibility: %s %v", scenario, err)
			}
			if source.Status != AttemptStatusFailed || source.ErrorMessage != (func() string {
				if scenario == "other failure" {
					return "Project verification failed"
				}
				return PublicationVerificationUnavailableMessage
			})() {
				t.Fatal("failure history changed")
			}
		})
	}
}
