package server

import (
	"context"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// Resume the preparation of an already authorized inspection, not a new model
// grant. A later model opinion cannot turn this read-only admission into deploy
// permission or acceptance. Cancellation and all current source checks apply.
func (s *Server) recoverUnstartedLiveInspection(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest, sourceID string) bool {
	if d.Launch == nil || d.Launch.State != "stopped" || (d.Launch.SafeError != task.InspectionPublicationPreparationError && d.Launch.SafeError != task.InspectionScopePreparationError) {
		return false
	}
	parent, err := s.taskSvc.LatestLaunch(ctx, d.ID)
	if err != nil || !task.IsInspectionPreparationFailure(parent) {
		return false
	}
	old, err := task.LiveInspectionForLaunch(&d.Task, parent)
	if err != nil || old == nil || old.SourceAttemptID != sourceID || old.SourceCommit != d.CommitSHA || old.OwnerID != origin.OwnerID || old.RoomID != origin.RoomID || old.SourceRequestID != origin.RequestID || old.PreparationLaunchID != "" {
		return false
	}
	limits, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, sourceID)
	if err != nil || limits.ReadOnlyRepository {
		return false
	}
	if !s.liveInspectionReadScopeAllowed(ctx, d, sourceID) {
		return false
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != old.Repository {
		return false
	}
	proof := *old
	proof.PreparationLaunchID, proof.TargetCommit, proof.ObservedAt = parent.ID, observed.Commit, observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return false
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	return s.startLiveResultInspection(ctx, d, origin, &proof, proof.Purpose == task.CurrentOutcomeInspection)
}

func (s *Server) liveInspectionReadScopeAllowed(ctx context.Context, d *task.TaskDetailResponse, sourceID string) bool {
	checkScope := project.TaskScopeInstructions(d.Instructions)
	if launches, ok := s.taskSvc.Store().(task.AttemptLaunchStore); ok {
		launch, err := launches.LaunchForAttempt(ctx, sourceID)
		if err != nil {
			return false
		}
		if launch != nil {
			checkScope += "\n" + launch.Revision
		}
	}
	continuation, err := s.ownerContinuationLaunch(ctx, d)
	if err != nil {
		return false
	}
	if continuation != nil {
		checkScope += "\n" + continuation.Revision
	}
	return !liveInspectionCheckProhibition.MatchString(checkScope)
}
