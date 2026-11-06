package task

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAttemptChangesRequireBoundRunnerAndCommit(t *testing.T) {
	for _, scenario := range []string{"valid", "legacy", "foreign runner", "foreign task", "foreign attempt", "wrong commit", "missing commit", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			commit, id := strings.Repeat("a", 40), "attempt"
			d := &TaskDetailResponse{Task: Task{ID: "task", CommitSHA: commit, DiffSummary: "HISTORICAL"}, LatestAttempt: &TaskAttempt{ID: id, RunnerID: "runner"}}
			record := RunnerVerificationRecord{Summary: "Checks", Changes: NewRunnerChangeRecord(commit, "web/src/features/landing/LandingPage.tsx | 435 +")}
			event := &TaskActivity{ID: "verification:" + id, TaskID: d.ID, AttemptID: &id, Author: "runner", ActivityType: ActivityTypeRunnerVerification}
			switch scenario {
			case "legacy":
				record.Changes = nil
			case "foreign runner":
				event.Author = "worker"
			case "foreign task":
				event.TaskID = "another-task"
			case "foreign attempt":
				old := "old"
				event.AttemptID = &old
			case "wrong commit":
				record.Changes.ResultCommit = strings.Repeat("b", 40)
			case "missing commit":
				record.Changes.ResultCommit = ""
			case "oversized":
				record.Changes.Summary = strings.Repeat("x", reviewChangeLimit+1)
			}
			raw, _ := json.Marshal(record)
			event.Message = string(raw)
			d.Activities = []*TaskActivity{event}
			got := AttemptChangeEvidence(d)
			if scenario == "valid" {
				if got.RunnerChangeRecord == nil || got.Summary != record.Changes.Summary || got.AttemptID != id || got.Provenance != "current_runner_branch_diff" {
					t.Fatal("native branch diff lost", got)
				}
			} else if got.RunnerChangeRecord != nil || got.AttemptID != "" {
				t.Fatal("unbound change evidence became current", got)
			}
			if got.Historical == nil || got.Historical.Summary != "HISTORICAL" {
				t.Fatal("historical context lost or promoted")
			}
		})
	}
}

func TestRunnerChangesBoundUnicodeAndFlagMissingRows(t *testing.T) {
	got := NewRunnerChangeRecord(strings.Repeat("a", 40), strings.Repeat("日", 6000))
	if !got.Truncated || !utf8.ValidString(got.Summary) || len(got.Summary) > reviewChangeLimit {
		t.Fatal("change truncation was silent or invalid")
	}
}
