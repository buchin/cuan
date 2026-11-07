package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const inboxVerificationReviewPolicy = "Verification review policy: v1"

func verificationClassificationMissing(d *task.TaskDetailResponse) bool {
	if d == nil {
		return false
	}
	assessment, found := reviewAssessment(d, "")
	return found && assessment.RemainingWork == "" && d.LatestAttempt != nil && d.LatestAttempt.Status == task.AttemptStatusSucceeded && inboxReportEvidence(d).Provenance == "current_attempt_report"
}

// This is one supplemental read-only verification for an already tested commit,
// not a reset or extension of the implementation correction budget.
func (s *Server) startVerificationFollowup(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, assessment inboxReviewEvaluation) (string, bool) {
	if d == nil || d.LatestAttempt == nil || q.RoomID != d.SourceRoomID || d.Status == "cancelled" || d.Status == "canceled" || d.Status == task.TaskStatusDone || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || assessment.Decision != "revise" || assessment.RemainingWork != "verification" || d.ResultKind != "implementation" || len(task.AcceptanceGaps(&d.Task)) > 0 {
		return "", false
	}
	// Require the exact saved assessment, never text inferred from a chat promise.
	saved, ok := reviewAssessment(d, q.RequestID)
	if !ok || saved != assessment || d.CommitSHA == "" {
		return "", false
	}
	if live, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err != nil || live != nil {
		return "", false
	}
	previousIntent, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil {
		return "", false
	}
	var recovery *task.VerificationIntent
	if previousIntent != nil {
		recovery = s.verificationRuntimeRecoveryProof(ctx, q, d, previousIntent, assessment)
		if recovery == nil {
			return "", false
		}
	} else if !task.ProjectChecksPassed(d) {
		return "", false
	}
	repository, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return "", false
	}
	requests, err := repository.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return "", false
	}
	var origin *db.InboxRequest
	for i := range requests {
		candidate := &requests[i]
		if candidate.MessageID == d.SourceMessageID && candidate.OwnerID == q.OwnerID && s.reviewSnapshotRecoveryAuthorized(ctx, d, *candidate) {
			origin = candidate
			break
		}
	}
	if origin == nil {
		return "", false
	}
	proof := task.VerificationIntent{SourceAttemptID: d.LatestAttempt.ID, SourceCommit: d.CommitSHA, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: origin.RequestID, AssessmentRequestID: q.RequestID, SourceVerificationHash: task.VerificationSummaryHash(task.AttemptVerificationEvidence(d).Current)}
	if recovery != nil {
		proof = *recovery
	}
	visualEvidence := "Use CUAN_TASK_ARTIFACTS_DIR for verification output and attach session:PATH artifacts when required."
	gap := strings.ToLower(assessment.Revision)
	if s.taskRequiresVision(ctx, q.OwnerID, d, "", assessment.Revision) || strings.Contains(gap, "screenshot") || strings.Contains(gap, "visual inspection") || strings.Contains(gap, "tangkapan layar") {
		visualEvidence = "Use CUAN_TASK_ARTIFACTS_DIR for output and attach session:mobile.png, session:desktop.png and the actual inspection report. Use the installed native browser tool. Never use generated images, image-generation tools or HTML mockups as application screenshot evidence."
	}
	raw, _ := json.Marshal(proof)
	revision := "Scope: read-only repository. Do not push. Do not deploy.\n" + task.VerificationIntentMarker + string(raw) + "\nVerify only the remaining evidence for the exact implemented commit. Do not modify code, configuration, tests, documentation, git index, refs or commits. Preserve existing work; reconcile uncertain prior effects. " + visualEvidence + " Set up any required verification tools outside the repository. Report concrete checks and unchanged original criteria; never invent acceptance or publication.\nRemaining gap:\n" + truncateProjectText(assessment.Revision, 2500)
	requestID := task.VerificationRequestID(d.ID, d.CommitSHA)
	if proof.RecoveryAttemptID != "" {
		requestID = task.VerificationRecoveryRequestID(d.ID, d.CommitSHA)
	}
	candidate := &task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: requestID, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: origin.RequestID, InstructionsHash: proof.InstructionsHash, Revision: revision}
	if _, err := task.VerificationIntentForLaunch(&d.Task, candidate); err != nil {
		return "", false
	}
	// A source commit has one immutable intent, even if evidence/reviews change.
	previous, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return "", false
	}
	if previous != nil {
		if parsed, validationErr := task.VerificationIntentForLaunch(&d.Task, previous); validationErr == nil && parsed != nil && (previous.State == "queued" || previous.State == "claimed" || previous.State == "started") {
			return verificationFollowupNotice(d, assessment), true
		}
		return "", false
	}
	s.startAuthorizedLaunch(ctx, q, d, requestID, origin.RequestID, revision)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
	if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
		return "", false
	}
	// The launch text is operational copy; a short factual notice follows only a
	// durable intent, never the model's unsupported continuation promise.
	return verificationFollowupNotice(d, assessment), true
}

func verificationFollowupNotice(d *task.TaskDetailResponse, assessment inboxReviewEvaluation) string {
	return strings.TrimSpace(assessment.Finding) + "\n\nCuan menyiapkan verifikasi tambahan pada hasil yang sama.\n\n[View work](/tasks/" + d.ID + ")"
}

// Only backend receipts for the current verifier can bind old implementation
// checks to an unchanged source. Agent reports cannot create this context.
func (s *Server) verificationReviewContext(ctx context.Context, d *task.TaskDetailResponse) map[string]any {
	original, proof := s.verifiedSupplementalSource(ctx, d)
	if original == nil {
		return nil
	}
	return map[string]any{"source_attempt_id": proof.SourceAttemptID, "source_commit": proof.SourceCommit, "source_project_checks": task.AttemptVerificationEvidence(original).Current, "repository_unchanged": true, "finalized_outputs_from_attempt_id": proof.FinalizeAttemptID, "artifact_manifest_hash": proof.ArtifactManifestHash}
}

// Resolve implementation identity only through a successful supplemental
// verifier's native source/unchanged-state receipts. This is not acceptance or
// permission to replay implementation; callers retain the current review grant.
func (s *Server) verifiedSupplementalSource(ctx context.Context, d *task.TaskDetailResponse) (*task.TaskDetailResponse, *task.VerificationIntent) {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return nil, nil
	}
	a := d.LatestAttempt
	if a.CancelRequested || a.CanceledAt != nil || a.ExitCode == nil || *a.ExitCode != 0 || a.FinishedAt == nil || a.ErrorMessage != "" {
		return nil, nil
	}
	proof, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || proof == nil {
		return nil, nil
	}
	_, unchanged, err := s.inboxReviewRepositoryScope(ctx, d)
	if err != nil || !unchanged {
		return nil, nil
	}
	receipt := false
	for _, a := range d.Activities {
		if a != nil && a.TaskID == d.ID && a.AttemptID != nil && *a.AttemptID == d.LatestAttempt.ID && a.Author == d.LatestAttempt.RunnerID && a.ActivityType == "verification_source_verified" && a.Message == proof.SourceCommit {
			receipt = true
			break
		}
	}
	if !receipt {
		return nil, nil
	}
	source, err := s.taskSvc.Store().GetAttempt(ctx, proof.SourceAttemptID)
	if err != nil || source == nil {
		return nil, nil
	}
	original := *d
	original.LatestAttempt = source
	return &original, proof
}
