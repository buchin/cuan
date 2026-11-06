package task

import (
	"encoding/json"
	"unicode/utf8"
)

const reviewChangeLimit = 12000

// RunnerChangeRecord is a branch diff collected by the runner, not a worker
// claim. A branch diff may include inherited commits; it is not per-attempt blame.
type RunnerChangeRecord struct {
	ResultCommit string `json:"result_commit,omitempty"`
	Summary      string `json:"summary"`
	Truncated    bool   `json:"truncated,omitempty"`
}

func NewRunnerChangeRecord(commit, summary string) *RunnerChangeRecord {
	record := &RunnerChangeRecord{ResultCommit: commit, Summary: summary}
	if len(summary) > reviewChangeLimit {
		record.Truncated = true
		record.Summary = summary[:reviewChangeLimit]
		for !utf8.ValidString(record.Summary) {
			record.Summary = record.Summary[:len(record.Summary)-1]
		}
	}
	return record
}

type ChangeEvidence struct {
	*RunnerChangeRecord
	AttemptID  string              `json:"attempt_id,omitempty"`
	Provenance string              `json:"provenance"`
	Historical *RunnerChangeRecord `json:"historical,omitempty"`
}

func AttemptChangeEvidence(d *TaskDetailResponse) ChangeEvidence {
	out := ChangeEvidence{Provenance: "no_current_runner_change_record"}
	if d == nil {
		return out
	}
	if d.DiffSummary != "" {
		out.Historical = NewRunnerChangeRecord(d.CommitSHA, d.DiffSummary)
	}
	if d.LatestAttempt == nil {
		return out
	}
	a := d.LatestAttempt
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != a.ID || event.ID != "verification:"+a.ID || a.RunnerID == "" || event.Author != a.RunnerID || event.ActivityType != ActivityTypeRunnerVerification {
			continue
		}
		var record RunnerVerificationRecord
		if json.Unmarshal([]byte(event.Message), &record) != nil || record.Changes == nil {
			continue
		}
		changes := record.Changes
		if (changes.ResultCommit != "" && (!fullObjectID(changes.ResultCommit) || changes.ResultCommit != d.CommitSHA)) || (changes.Summary != "" && changes.ResultCommit == "") || len(changes.Summary) > reviewChangeLimit {
			out.Provenance = "invalid_current_runner_change_record"
			return out
		}
		out.RunnerChangeRecord, out.AttemptID, out.Provenance = changes, a.ID, "current_runner_branch_diff"
		if out.Historical != nil && out.Historical.ResultCommit == changes.ResultCommit && out.Historical.Summary == changes.Summary && out.Historical.Truncated == changes.Truncated {
			out.Historical = nil
		}
		return out
	}
	return out
}
