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
	if d == nil || d.LatestAttempt == nil || (d.LatestAttempt.Status != task.AttemptStatusSucceeded && len(task.FailedCheckRevalidationPaths(d)) == 0) || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || d.LatestAttempt.FinishedAt == nil || s.projectSvc == nil || (d.ExecutionCategory != "" && d.ExecutionCategory != task.ExecutionCategoryCode) {
		return false
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.CurrentOutcomeInspection || previous.PreviousAttemptID != "" {
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
	if request == "" || (!evidence && len(paths) == 0) {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, previous.SourceAttemptID)
	if err != nil || limits.ReadOnlyRepository {
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
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	proof := *previous
	if evidence {
		proof.RevalidationKind = task.EvidenceRevalidation
	}
	proof.PreviousAttemptID = d.LatestAttempt.ID
	proof.PreviousCommit = previous.TargetCommit
	proof.TargetCommit = observed.Commit
	proof.ObservedAt = observed.ObservedAt
	proof.ReviewRequestID, proof.ReviewPaths = request, paths
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
	return s.startLiveResultInspection(ctx, d, origin, &proof, true)
}
