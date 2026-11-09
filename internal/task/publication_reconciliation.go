package task

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
)

const PublicationVerificationUnavailableMessage = "Kepatuhan auto_deploy: production publication could not be independently verified through the registered project channel. Reconcile the existing live revision; do not repeat deployment solely because a receipt is missing. Report text is not deployment evidence."

const PublicationReceiptRequiredMessage = "Kepatuhan auto_deploy: verified production deployment receipt is required for the exact task, attempt and commit; commit/push and report claims alone do not satisfy acceptance"

// Both native completion paths retain a failed attempt when publication proof
// is missing. Recognizing that stage never establishes execution or source proof;
// the caller must still validate the runner, checks, checkpoint and saved review.
func publicationEvidenceMissing(message string) bool {
	return message == PublicationVerificationUnavailableMessage || message == PublicationReceiptRequiredMessage
}

// IsPublicationReconciliationOutcome permits one existing current-outcome
// inspection, not implementation or deployment. Preserve the failed source:
// independent tests and a clean result commit do not prove publication.
func IsPublicationReconciliationOutcome(t *Task, a *TaskAttempt, activities []*TaskActivity) bool {
	if t == nil || a == nil || a.TaskID != t.ID || !fullObjectID(t.CommitSHA) || !strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") || (t.ExecutionCategory != "" && t.ExecutionCategory != ExecutionCategoryCode) || a.Status != AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || !publicationEvidenceMissing(a.ErrorMessage) || a.RunnerID == "" || a.CancelRequested || a.CanceledAt != nil || a.CreatedAt.IsZero() || a.StartedAt == nil || a.FinishedAt == nil || a.StartedAt.Before(a.CreatedAt) || a.FinishedAt.Before(*a.StartedAt) || a.FinishedAt.After(time.Now()) {
		return false
	}
	limits := executionpolicy.FromInstructions(t.Instructions)
	if limits.DenyDeploy || limits.ReadOnlyRepository || !liveImplementationFinalReport(activities, t.ID, a.ID) || VerifiedDeployment(activities, a, t.CommitSHA, time.Now()) {
		return false
	}
	detail := &TaskDetailResponse{Task: *t, LatestAttempt: a, Activities: activities}
	changes := AttemptChangeEvidence(detail)
	if changes.Provenance != "current_runner_branch_diff" || changes.RunnerChangeRecord == nil || changes.ResultCommit != t.CommitSHA || strings.TrimSpace(changes.Summary) == "" {
		return false
	}
	checksPassed := false
	reviewed := false
	for _, event := range activities {
		if event == nil || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != a.ID {
			continue
		}
		if event.ID == "verification:"+a.ID && event.ActivityType == ActivityTypeRunnerVerification && event.Author == a.RunnerID {
			var record RunnerVerificationRecord
			if json.Unmarshal([]byte(event.Message), &record) == nil && PassingProjectCheckSummary(record.Summary) {
				checksPassed = true
			}
		}
		if event.ActivityType == "coordinator_review_assessment" && event.Author == "coordinator" && !event.CreatedAt.Before(*a.FinishedAt) && !event.CreatedAt.After(time.Now()) {
			var record struct {
				RequestID        string `json:"request_id"`
				InstructionsHash string `json:"instructions_hash"`
				EvidenceDigest   string `json:"evidence_digest"`
				Assessment       struct {
					Decision      string `json:"decision"`
					RemainingWork string `json:"remaining_work"`
				} `json:"assessment"`
			}
			if json.Unmarshal([]byte(event.Message), &record) == nil && record.RequestID != "" && record.InstructionsHash == LaunchInstructionsHash(t.Instructions) && len(record.EvidenceDigest) == 64 && record.Assessment.Decision == "revise" && record.Assessment.RemainingWork == "verification" {
				reviewed = true
			}
		}
	}
	return checksPassed && reviewed
}
