package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const independentAcceptanceGuidance = "For each original criterion, identify the actual observed check and what its assertions establish. Inspect whether existing tests cover the requested behavior before relying on them. For safety or rejection boundaries, independently exercise representative invalid inputs and verify their actual effects, using temporary fixtures or overlays outside the repository. Derive cases from the original requirement rather than copying the implementation's assumptions. A passing test count is not coverage proof. Mark an untested criterion incomplete; never claim full security or 100% verification from existing green tests alone. Retain reproducible commands, results and precise remaining gaps in the report."

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
	// Verification may itself be an unmet original criterion. Admission is a
	// read-only evidence run, not acceptance: the original checklist stays
	// incomplete until the verifier supplies current observed evidence.
	if d == nil || d.LatestAttempt == nil || q.RoomID != d.SourceRoomID || d.Status == "cancelled" || d.Status == "canceled" || d.Status == task.TaskStatusDone || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || assessment.Decision != "revise" || assessment.RemainingWork != "verification" {
		return "Source verification is not eligible for this attempt.", false
	}
	// Require the exact saved assessment, never text inferred from a chat promise.
	saved, ok := reviewAssessment(d, q.RequestID)
	if !ok || saved != assessment || d.CommitSHA == "" {
		return "The current source verification assessment is unavailable.", false
	}
	sourceDetail := d
	live, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil {
		return "The source inspection intent could not be validated.", false
	}
	if live != nil {
		if live.OwnerID != q.OwnerID {
			return "The source inspection owner does not match.", false
		}
		sourceDetail = s.testedSourceAfterFailedInspection(ctx, d, live)
		if sourceDetail == nil {
			return "This attempt already belongs to a live-result inspection.", false
		}
	}
	if sourceDetail == d {
		if priorVerification, _ := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); priorVerification != nil {
			if tested := s.testedSourceAfterFailedVerification(ctx, d, priorVerification); tested != nil {
				sourceDetail = tested
			}
		}
	}
	if sourceDetail.ResultKind != "implementation" {
		return "The source is not an implemented result.", false
	}
	previousIntent, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &sourceDetail.Task, sourceDetail.LatestAttempt.ID)
	if err != nil {
		return "The source verification intent could not be validated.", false
	}
	var recovery *task.VerificationIntent
	if previousIntent != nil {
		recovery = s.verificationRuntimeRecoveryProof(ctx, q, d, previousIntent, assessment)
		if recovery == nil {
			return "A separate verifier recovery is not authorized.", false
		}
	} else if !task.VerificationSourceChecksPassed(sourceDetail) {
		return "Required project checks have not passed for this source.", false
	}
	repository, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return "Source verification request storage is unavailable.", false
	}
	requests, err := repository.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return "The source verification authorization could not be read.", false
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
		return "The original source verification authorization is unavailable.", false
	}
	proof := task.VerificationIntent{IsolatedWorkspace: true, SourceAttemptID: sourceDetail.LatestAttempt.ID, SourceCommit: d.CommitSHA, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: origin.RequestID, AssessmentRequestID: q.RequestID, SourceVerificationHash: task.VerificationSummaryHash(task.AttemptVerificationEvidence(sourceDetail).Current)}
	if recovery != nil {
		proof = *recovery
	}
	visualEvidence := "Use CUAN_TASK_ARTIFACTS_DIR for verification output and attach session:PATH artifacts when required."
	gap := strings.ToLower(assessment.Revision)
	if s.taskRequiresVision(ctx, q.OwnerID, d, "", assessment.Revision) || strings.Contains(gap, "screenshot") || strings.Contains(gap, "visual inspection") || strings.Contains(gap, "tangkapan layar") {
		visualEvidence = "Use CUAN_TASK_ARTIFACTS_DIR for output and attach session:mobile.png, session:desktop.png and the actual inspection report. Use the installed native browser tool. Never use generated images, image-generation tools or HTML mockups as application screenshot evidence."
	}
	raw, _ := json.Marshal(proof)
	revision := "Scope: read-only repository. Do not push. Do not deploy.\n" + task.VerificationIntentMarker + string(raw) + "\nVerify only the remaining evidence for the exact implemented commit. This isolated verification snapshot contains only the exact implemented commit and its immediate parent; unrelated local or remote branch refs are intentionally excluded, and merge/push/deployment to main is handled separately after source verification. Do not modify code, configuration, tests, documentation, git index, refs or commits, and do not fail verification merely because branch refs, merge or push are unavailable in this read-only snapshot. Preserve existing work; reconcile uncertain prior effects. " + visualEvidence + " Set up any required verification tools outside the repository. Report concrete checks and unchanged original criteria; never invent acceptance or publication. " + independentAcceptanceGuidance + "\nRemaining gap:\n" + truncateProjectText(assessment.Revision, 2500)
	requestID := task.VerificationRequestID(d.ID, d.CommitSHA, proof.SourceAttemptID)
	if proof.RecoveryAttemptID != "" {
		requestID = task.VerificationRecoveryRequestID(d.ID, d.CommitSHA)
	}
	candidate := &task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: requestID, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: origin.RequestID, InstructionsHash: proof.InstructionsHash, Revision: revision}
	if _, err := task.VerificationIntentForLaunch(&d.Task, candidate); err != nil {
		return "The source verification scope could not be validated.", false
	}
	// One developer attempt and exact commit have one immutable intent. Another
	// developer handoff at the same SHA has different native evidence and cannot
	// reuse an older verifier's source binding.
	previous, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return "The saved source verification launch could not be read.", false
	}
	if previous != nil {
		if parsed, validationErr := task.VerificationIntentForLaunch(&d.Task, previous); validationErr == nil && parsed != nil && (previous.State == "queued" || previous.State == "claimed" || previous.State == "started") {
			return verificationFollowupNotice(d, assessment), true
		}
		return "The saved source verification launch is no longer active.", false
	}
	launchNotice := s.startAuthorizedLaunch(ctx, q, d, requestID, origin.RequestID, revision)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
	if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
		return launchNotice, false
	}
	// The launch text is operational copy; a short factual notice follows only a
	// durable intent, never the model's unsupported continuation promise.
	return verificationFollowupNotice(d, assessment), true
}

func verificationFollowupNotice(d *task.TaskDetailResponse, assessment inboxReviewEvaluation) string {
	return deduplicateWorkLinks(strings.TrimSpace(assessment.Finding) + "\n\nCuan menyiapkan verifikasi tambahan pada hasil yang sama.\n\n[View work](/tasks/" + d.ID + ")")
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
	if d == nil || d.LatestAttempt == nil {
		return nil, nil
	}
	var candidates []*task.TaskAttempt
	if d.LatestAttempt.Status == task.AttemptStatusSucceeded {
		candidates = append(candidates, d.LatestAttempt)
	} else if intent, err := task.NativeReleaseIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err == nil && intent != nil && intent.SourceReviewAttemptID != "" {
		if verifier, err := s.taskSvc.Store().GetAttempt(ctx, intent.SourceReviewAttemptID); err == nil && verifier != nil && verifier.TaskID == d.ID {
			candidates = append(candidates, verifier)
		}
	}
	for i := len(d.Attempts) - 1; i >= 0; i-- {
		att := d.Attempts[i]
		if att == nil || att.ID == d.LatestAttempt.ID {
			continue
		}
		found := false
		for _, c := range candidates {
			if c.ID == att.ID {
				found = true
				break
			}
		}
		if !found && att.Status == task.AttemptStatusSucceeded {
			candidates = append(candidates, att)
		}
	}

	for _, a := range candidates {
		if a == nil || a.CancelRequested || a.CanceledAt != nil || a.ExitCode == nil || *a.ExitCode != 0 || a.FinishedAt == nil || a.ErrorMessage != "" {
			continue
		}
		proof, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &d.Task, a.ID)
		if err != nil || proof == nil {
			continue
		}
		_, unchanged, err := s.inboxReviewRepositoryScope(ctx, d)
		if err != nil || !unchanged {
			return nil, nil
		}
		receipt := false
		for _, act := range d.Activities {
			if act != nil && act.TaskID == d.ID && act.AttemptID != nil && *act.AttemptID == a.ID && act.Author == a.RunnerID && act.ActivityType == "verification_source_verified" && act.Message == proof.SourceCommit {
				receipt = true
				break
			}
		}
		if !receipt {
			continue
		}
		source, err := s.taskSvc.Store().GetAttempt(ctx, proof.SourceAttemptID)
		if err != nil || source == nil {
			continue
		}
		original := *d
		original.LatestAttempt = source
		return &original, proof
	}
	return nil, nil
}

// A failed production inspection does not erase the tested unpublished source
// explicitly bound into its native intent. Reuse that exact source for one
// isolated verifier; never replay the inspection or turn it into implementation.
func (s *Server) testedSourceAfterFailedInspection(ctx context.Context, d *task.TaskDetailResponse, live *task.LiveInspectionIntent) *task.TaskDetailResponse {
	if d == nil || d.LatestAttempt == nil || live == nil || live.Purpose != task.CurrentOutcomeInspection || live.SourceCommit == "" || live.SourceCommit != d.CommitSHA || live.TargetCommit == live.SourceCommit || live.PreviousAttemptID != "" || live.SourceVerifierAttemptID != "" {
		return nil
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || a.FinishedAt == nil || a.CancelRequested || a.CanceledAt != nil || a.ErrorMessage != "Live result inspection project checks did not pass" {
		return nil
	}
	if err := task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, live); err != nil {
		return nil
	}
	source, err := s.taskSvc.Store().GetAttempt(ctx, live.SourceAttemptID)
	if err != nil || source == nil || source.TaskID != d.ID || !task.IsPublicationReconciliationOutcome(&d.Task, source, d.Activities) {
		return nil
	}
	original := *d
	original.LatestAttempt, original.ResultKind = source, "implementation"
	return &original
}

func (s *Server) testedSourceAfterFailedVerification(ctx context.Context, d *task.TaskDetailResponse, prior *task.VerificationIntent) *task.TaskDetailResponse {
	if d == nil || d.LatestAttempt == nil || prior == nil || prior.SourceAttemptID == "" || !releaseObjectID.MatchString(prior.SourceCommit) {
		return nil
	}
	a := d.LatestAttempt
	if (a.Status != task.AttemptStatusFailed && a.Status != task.AttemptStatusCanceled) || a.FinishedAt == nil {
		return nil
	}
	source, err := s.taskSvc.Store().GetAttempt(ctx, prior.SourceAttemptID)
	if err != nil || source == nil || source.TaskID != d.ID || source.Status != task.AttemptStatusSucceeded {
		return nil
	}
	original := *d
	original.LatestAttempt, original.ResultKind = source, "implementation"
	return &original
}
