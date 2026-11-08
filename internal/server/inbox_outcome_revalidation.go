package server

import (
	"context"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// One read-only revalidation may inspect a newly changed rejected result. It is
// not another implementation attempt and is never granted for a deploy alone.
func (s *Server) recoverChangedOutcomeInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt == nil || (d.LatestAttempt.Status != task.AttemptStatusSucceeded && len(task.FailedCheckRevalidationPaths(d)) == 0) || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || d.LatestAttempt.FinishedAt == nil || s.projectSvc == nil || (d.ExecutionCategory != "" && d.ExecutionCategory != task.ExecutionCategoryCode) {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || (previous.Purpose != task.CurrentOutcomeInspection && previous.Purpose != task.NativeReleaseInspection) || previous.PreviousAttemptID != "" {
		return false
	}
	assessment, found := reviewAssessment(d, "")
	if !found || assessment.Decision != "revise" {
		return false
	}
	evidenceRequest, _ := task.OutcomeEvidenceReview(&d.Task, d.LatestAttempt, d.Activities, "")
	evidence := evidenceRequest != "" && assessment.RemainingWork == "verification"
	request, paths := task.OutcomeRevalidationReview(&d.Task, d.LatestAttempt, d.Activities, "")
	if evidence {
		request, paths = evidenceRequest, nil
	}
	if request == "" || (!evidence && (previous.Purpose == task.NativeReleaseInspection || len(paths) == 0)) {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, previous.SourceAttemptID)
	if err != nil || (previous.Purpose != task.NativeReleaseInspection && limits.ReadOnlyRepository) || !s.liveInspectionReadScopeAllowed(ctx, d, previous.SourceAttemptID) {
		return false
	}
	scope := project.TaskScopeInstructions(d.Instructions)
	if continuation, err := s.ownerContinuationLaunch(ctx, d); err != nil {
		return false
	} else if continuation != nil {
		scope += "\n" + continuation.Revision
	}
	if liveInspectionCheckProhibition.MatchString(scope) {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != previous.Repository || (!evidence && observed.Commit == previous.TargetCommit) {
		return false
	}
	if previous.Purpose == task.NativeReleaseInspection && observed.Commit != previous.TargetCommit {
		contained, err := s.projectSvc.PublishedCommitContained(ctx, d.ProjectID, previous.TargetCommit, observed.Commit)
		if err != nil || !contained {
			return false
		}
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	// Build the next stage from its source binding, not a copy of every
	// previous-stage flag. The completed parent retains its repair history.
	proof := task.LiveInspectionIntent{
		Purpose:                 previous.Purpose,
		SourceVerifierAttemptID: previous.SourceVerifierAttemptID,
		SourceAttemptID:         previous.SourceAttemptID, SourceCommit: previous.SourceCommit,
		Repository: previous.Repository, OwnerID: previous.OwnerID, RoomID: previous.RoomID,
		SourceRequestID: previous.SourceRequestID, InstructionsHash: previous.InstructionsHash,
		PreviousAttemptID: d.LatestAttempt.ID, PreviousCommit: previous.TargetCommit,
		TargetCommit: observed.Commit, ObservedAt: observed.ObservedAt,
		ReviewRequestID: request, ReviewPaths: paths,
	}
	if evidence {
		proof.RevalidationKind = task.EvidenceRevalidation
	}
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return false
	}
	// The request key is independent of the new commit: this allowance cannot
	// renew itself each time the application is deployed.
	saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, proof.RequestID(d.ID)))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return false
	}
	if saved != nil {
		return saved.State == "queued" || saved.State == "claimed" || saved.State == "started"
	}
	if !evidence {
		changed, err := s.projectSvc.VerificationPathsChanged(ctx, d.ProjectID, d.ID, proof.PreviousCommit, proof.TargetCommit, paths)
		if err != nil || !changed {
			return false
		}
	}
	return s.startLiveResultInspection(ctx, d, origin, &proof, proof.Purpose == task.CurrentOutcomeInspection)
}
