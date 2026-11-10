package task

import "encoding/json"

const ActivityTypeRunnerVerification = "runner_verification"

// RunnerVerificationRecord is saved atomically with execution completion, even
// when no verifier ran. Task-level TestSummary remains historical display data.
type RunnerVerificationRecord struct {
	Timing                      *ExecutionTiming    `json:"timing,omitempty"`
	Summary                     string              `json:"summary"`
	PublicationInstructionsHash string              `json:"publication_instructions_hash,omitempty"`
	PublicationRequired         *bool               `json:"publication_required,omitempty"`
	Changes                     *RunnerChangeRecord `json:"changes,omitempty"`
}

type VerificationEvidence struct {
	Current    string
	Historical string
	AttemptID  string
	Provenance string
	Author     string
}

func AttemptVerificationEvidence(d *TaskDetailResponse) VerificationEvidence {
	var out VerificationEvidence
	if d == nil || d.LatestAttempt == nil {
		return out
	}
	out.Provenance = "no_current_attempt_verification"
	out.Historical = d.TestSummary
	var latest *TaskActivity
	for _, a := range d.Activities {
		if a == nil || a.TaskID != d.ID || a.AttemptID == nil || *a.AttemptID != d.LatestAttempt.ID || a.ActivityType != ActivityTypeRunnerVerification {
			continue
		}
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID) {
			latest = a
		}
	}
	if latest == nil {
		return out
	}
	var record struct {
		Summary *string `json:"summary"`
	}
	if json.Unmarshal([]byte(latest.Message), &record) != nil || record.Summary == nil {
		out.Provenance = "invalid_current_attempt_verification"
		return out
	}
	out.Current = *record.Summary
	out.AttemptID = d.LatestAttempt.ID
	out.Author = latest.Author
	out.Provenance = "current_attempt_verification"
	if out.Historical == out.Current {
		out.Historical = ""
	}
	return out
}

// AttemptPublicationRequired reads the server's completion-time decision, never
// report prose or an older attempt's retained task commit.
func AttemptPublicationRequired(d *TaskDetailResponse) *bool {
	if d == nil || d.LatestAttempt == nil || d.Status == "cancelled" || d.Status == "canceled" {
		return nil
	}
	a := d.LatestAttempt
	if a.Status != AttemptStatusSucceeded || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || a.RunnerID == "" {
		return nil
	}
	for _, e := range d.Activities {
		if e == nil || e.ID != "verification:"+a.ID || e.TaskID != d.ID || e.AttemptID == nil || *e.AttemptID != a.ID || e.Author != a.RunnerID || e.ActivityType != ActivityTypeRunnerVerification || e.CreatedAt.Before(*a.FinishedAt) {
			continue
		}
		var record RunnerVerificationRecord
		if json.Unmarshal([]byte(e.Message), &record) == nil && record.PublicationInstructionsHash == LaunchInstructionsHash(d.Instructions) {
			return record.PublicationRequired
		}
	}
	return nil
}
