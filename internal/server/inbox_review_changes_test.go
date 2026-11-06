package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestReviewReceivesNativeChangesDistinctFromHistoricalReport(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	id, commit := "attempt", strings.Repeat("a", 40)
	d := &task.TaskDetailResponse{Task: task.Task{ID: "task", CommitSHA: commit, Instructions: "Implement a PHP library only", Summary: "Worker claims all changes required", DiffSummary: "HISTORICAL_CHANGE"}, LatestAttempt: &task.TaskAttempt{ID: id, RunnerID: "runner"}}
	before := inboxReviewEvidenceDigest(d)
	record := task.RunnerVerificationRecord{Summary: "Checks", Changes: task.NewRunnerChangeRecord(commit, "web/src/features/landing/LandingPage.tsx | 435 +")}
	raw, _ := json.Marshal(record)
	d.Activities = []*task.TaskActivity{{ID: "verification:" + id, TaskID: d.ID, AttemptID: &id, Author: "runner", ActivityType: task.ActivityTypeRunnerVerification, Message: string(raw)}}
	var payload struct {
		Changes  task.ChangeEvidence `json:"change_evidence"`
		Original string              `json:"original_request"`
	}
	if err := json.Unmarshal([]byte(s.inboxReviewData(context.Background(), d)), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Changes.RunnerChangeRecord == nil || payload.Changes.Summary != record.Changes.Summary || payload.Changes.Provenance != "current_runner_branch_diff" || payload.Changes.Historical == nil || payload.Changes.Historical.Summary != "HISTORICAL_CHANGE" || payload.Original != d.Instructions {
		t.Fatal("review lost scope or native/historical distinction", payload)
	}
	if before == inboxReviewEvidenceDigest(d) {
		t.Fatal("new native change evidence did not invalidate old assessment")
	}
}
