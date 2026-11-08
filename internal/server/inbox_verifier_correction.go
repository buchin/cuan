package server

import (
	"context"
	"fmt"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const inboxVerifierCorrectionPolicy = "Verified implementation correction: v2"

// A verifier never gains write permission. A saved PM assessment may instead
// admit a separate developer continuation within the original owner's scope.
func (s *Server) verifiedImplementationCorrectionAllowed(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, revision string) bool {
	if d == nil || d.LatestAttempt == nil || q.RoomID != d.SourceRoomID || s.inboxReviewedTask(ctx, q) == nil || !s.taskContinuationAfterCancel(ctx, q, d) {
		return false
	}
	saved, found := reviewAssessment(d, q.RequestID)
	if !found || saved.Decision != "revise" || saved.RemainingWork != "implementation" || saved.RequestedOutcome != "unfinished" || saved.Revision == "" || revision != saved.Revision || saved.ManagerReason == "" || (saved.ManagerAction != "continue" && saved.ManagerAction != "replan") {
		return false
	}
	scope, _ := s.inboxApprovedTaskScope(ctx, d)
	if scope == nil || scope.ReadOnlyRepository {
		return false
	}
	source, proof := s.verifiedSupplementalSource(ctx, d)
	return source != nil && proof != nil && proof.OwnerID == q.OwnerID && proof.RoomID == q.RoomID
}

// Historical native failures inform the PM; they do not authorize work, prove
// the current release failed or waive the original acceptance criteria.
func (s *Server) verifierCorrectionContext(ctx context.Context, d *task.TaskDetailResponse) map[string]any {
	source, proof := s.verifiedSupplementalSource(ctx, d)
	if source == nil || proof == nil {
		return nil
	}
	var previous *task.TaskAttempt
	var previousIntent *task.LiveInspectionIntent
	for _, a := range d.Attempts {
		if a == nil || a.TaskID != d.ID || a.ID == d.LatestAttempt.ID || a.Status != task.AttemptStatusFailed || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || a.FinishedAt.After(d.LatestAttempt.CreatedAt) || a.ErrorMessage != "Live result inspection project checks did not pass" {
			continue
		}
		intent, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, a.ID)
		if err != nil || intent == nil || intent.Purpose != task.CurrentOutcomeInspection || intent.SourceAttemptID != proof.SourceAttemptID || intent.SourceCommit != proof.SourceCommit || intent.OwnerID != proof.OwnerID || intent.RoomID != proof.RoomID || intent.SourceRequestID != proof.SourceRequestID {
			continue
		}
		if previous == nil || a.CreatedAt.After(previous.CreatedAt) {
			previous, previousIntent = a, intent
		}
	}
	if previous == nil {
		return nil
	}
	historical := *d
	historical.LatestAttempt = previous
	checks := task.AttemptVerificationEvidence(&historical)
	if checks.Author != previous.RunnerID || checks.Current == "" {
		return nil
	}
	return map[string]any{"source_attempt_id": proof.SourceAttemptID, "verified_source_commit": proof.SourceCommit, "source_verifier_attempt_id": d.LatestAttempt.ID, "historical_live_attempt_id": previous.ID, "historical_live_commit": previousIntent.TargetCommit, "historical_native_checks": truncateProjectText(checks.Current, 3000), "meaning": "These checks failed on the identified historical live revision; independent source verification later passed on the retained local source. Reconcile this concrete difference with the current registered release. Do not repeat an unchanged failed inspection, treat it as missing deployment evidence alone, or call incidental test corrections authentication fixes. If an actual in-scope source/integration correction remains, classify remaining_work implementation and give the developer a concrete scoped revision. A separate developer continuation may implement it within original owner limits; the verifier itself remains read-only. Preserve inherited work and all independent review and publication requirements."}
}

func (s *Server) verifiedCorrectionExecutionContext(ctx context.Context, d *task.TaskDetailResponse) string {
	_, proof := s.verifiedSupplementalSource(ctx, d)
	if proof == nil {
		return ""
	}
	return fmt.Sprintf("\n\nSeparate developer correction after independent source verification. Retain source attempt %s, commit %s, and verifier %s as immutable history. Inspect the existing private workspace and current project baseline before changing anything. Implement only the saved PM correction within the original approved task; preserve unrelated/inherited source and existing effects. The verifier's read-only boundary belongs to its completed session, not permission to mutate that snapshot. Work in the developer workspace, retain original owner restrictions, and hand the resulting exact source back to independent verification before acceptance. Do not publish the old branch merely to manufacture a receipt.", proof.SourceAttemptID, proof.SourceCommit, d.LatestAttempt.ID)
}
