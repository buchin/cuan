package task

import (
	"encoding/json"
	"strings"
)

const InspectionDependencyFailurePrefix = "locked frontend dependency preparation failed; saved source is preserved"
const InspectionProjectVerificationTimeout = "Live result inspection independent project verification timed out; checks are incomplete"
const InspectionProjectVerificationDeadline = "Live result inspection deadline expired during independent project verification; checks are incomplete"

// A saved report followed by a native command abort may use the existing
// single infrastructure retry. Legacy generic failures still require an
// authenticated exit -1 check record without assertions; no timeout is inferred.
// It grants no credit for incomplete checks or the
// unknown final repository state; the retry prepares and checks a fresh snapshot.
func inspectionProjectVerificationDeadlineFailure(a *TaskAttempt, events []*TaskActivity) bool {
	if a == nil || a.Status != AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || (a.ErrorMessage != InspectionProjectVerificationDeadline && a.ErrorMessage != InspectionProjectVerificationTimeout && a.ErrorMessage != "Live result inspection project checks did not pass") || a.RunnerID == "" || a.StartedAt == nil || a.FinishedAt == nil || a.FinishedAt.Before(*a.StartedAt) || a.CancelRequested || a.CanceledAt != nil || !liveImplementationFinalReport(events, a.TaskID, a.ID) {
		return false
	}
	checks, boundary := false, false
	for _, event := range events {
		if event == nil || event.TaskID != a.TaskID || event.AttemptID == nil || *event.AttemptID != a.ID {
			continue
		}
		if event.ID == "verification:"+a.ID {
			var record RunnerVerificationRecord
			if event.Author != a.RunnerID || event.ActivityType != ActivityTypeRunnerVerification || json.Unmarshal([]byte(event.Message), &record) != nil || !strings.HasPrefix(record.Summary, "Project verification: ") || !strings.Contains(record.Summary, " (exit -1)\n") || record.Changes != nil {
				return false
			}
			for _, failure := range []string{"--- FAIL:", "\nFAIL", "AssertionError", "FAILURES!", "ERRORS!"} {
				if strings.Contains(record.Summary, failure) {
					return false
				}
			}
			checks = true
		}
		if event.ID == "repository-observation:"+a.ID {
			var observed struct {
				Baseline string `json:"baseline"`
				Final    string `json:"final"`
			}
			if event.Author != a.RunnerID || event.ActivityType != "repository_scope_observed" || event.NewStatus != AttemptStatusFailed || json.Unmarshal([]byte(event.Message), &observed) != nil || !validVerificationHash(observed.Baseline) || (observed.Final != "" && observed.Final != observed.Baseline) {
				return false
			}
			boundary = true
		}
	}
	return checks && boundary
}

// InspectionInfrastructureFailure recognizes saved application/runner failures,
// not a model's explanation. It permits a new bounded read-only inspection, not
// replay of effects or acceptance of the failed attempt. In the legacy launch
// race the provider may have started; its unknown result stays unknown.
func InspectionInfrastructureFailure(a *TaskAttempt, events []*TaskActivity) bool {
	if inspectionProjectVerificationDeadlineFailure(a, events) {
		return true
	}
	if a == nil || a.Status != AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 1 || a.RunnerID == "" || a.StartedAt == nil || a.FinishedAt == nil || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt.Before(*a.StartedAt) {
		return false
	}
	legacyLaunch := a.ErrorMessage == "Could not save task status before starting."
	setup := strings.HasPrefix(a.ErrorMessage, InspectionDependencyFailurePrefix+": ")
	if !legacyLaunch && !setup {
		return false
	}
	transition, emptyChecks := false, false
	for _, event := range events {
		if event == nil || event.TaskID != a.TaskID || event.AttemptID == nil || *event.AttemptID != a.ID {
			continue
		}
		// An actual check/result failure requires its own review. Setup recovery
		// cannot discard those findings or a final blocked report.
		if event.ActivityType == "repository_scope_observed" || (event.ActivityType == "agent_report" && (event.NewStatus == TaskStatusReview || event.NewStatus == TaskStatusDone || event.NewStatus == TaskStatusBlocked)) {
			return false
		}
		if event.ActivityType == ActivityTypeRunnerVerification {
			var checks RunnerVerificationRecord
			if !setup || event.ID != "verification:"+a.ID || event.Author != a.RunnerID || json.Unmarshal([]byte(event.Message), &checks) != nil || checks.Summary != "" || checks.Changes != nil {
				return false
			}
			emptyChecks = true
		}
		if event.ActivityType != ActivityTypeAttempt || event.CreatedAt.Before(*a.StartedAt) || event.CreatedAt.After(*a.FinishedAt) {
			continue
		}
		if legacyLaunch && event.OldStatus == AttemptStatusRunning && event.NewStatus == AttemptStatusFailed && event.Author == "runner" && strings.Contains(event.Message, "[Message: "+a.ErrorMessage+"]") {
			transition = true
		}
		if setup && event.OldStatus == TaskStatusDoing && event.NewStatus == TaskStatusBlocked && event.Author == a.RunnerID && strings.Contains(event.Message, "selesai dengan status 'failed' (exit code: 1,") {
			transition = true
		}
	}
	return transition && (!setup || emptyChecks)
}
