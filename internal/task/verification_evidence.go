package task

import "encoding/json"

const ActivityTypeRunnerVerification = "runner_verification"

// RunnerVerificationRecord is saved atomically with execution completion, even
// when no verifier ran. Task-level TestSummary remains historical display data.
type RunnerVerificationRecord struct {
	Summary string `json:"summary"`
}

type VerificationEvidence struct {
	Current    string
	Historical string
	AttemptID  string
	Provenance string
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
	out.Provenance = "current_attempt_verification"
	if out.Historical == out.Current {
		out.Historical = ""
	}
	return out
}
