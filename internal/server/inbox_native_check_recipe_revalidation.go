package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Continue missing independent checks only after their registered recipe actually
// changed. Reuses the existing read-only snapshot/revalidation lane and queue.
func (s *Server) recoverNativeCheckRecipe(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	proof, origin, _ := s.planNativeCheckRecipe(ctx, d, requests)
	return proof != nil && origin != nil && s.startLiveResultInspection(ctx, d, *origin, proof, false)
}

func (s *Server) planNativeCheckRecipe(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) (*task.LiveInspectionIntent, *db.InboxRequest, string) {
	if s.projectSvc == nil || !reviewSnapshotRecoveryAllowed(d) {
		return nil, nil, "native_check_recovery_not_available"
	}
	request, paths := task.NativeCheckRecipeReview(&d.Task, d.LatestAttempt, d.Activities, "")
	if request == "" {
		return nil, nil, "native_check_replan_required"
	}
	decision, found := reviewAssessment(d, request)
	if !found || decision.ManagerAction != "replan" {
		return nil, nil, "native_check_replan_stale"
	}
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || previous == nil || previous.Purpose != task.NativeReleaseInspection || task.NativeCheckRecipeRevalidation(previous) {
		return nil, nil, "native_check_parent_invalid"
	}
	observed := s.runningProjectRelease(ctx, d)
	if observed == nil || observed.Repository != previous.Repository || observed.Commit == previous.TargetCommit {
		return nil, nil, "native_check_new_target_missing"
	}
	contained, err := s.projectSvc.PublishedCommitContained(ctx, d.ProjectID, previous.TargetCommit, observed.Commit)
	if err != nil || !contained {
		return nil, nil, "native_check_target_not_containing"
	}
	changed, err := s.projectSvc.VerificationPathsChanged(ctx, d.ProjectID, d.ID, previous.TargetCommit, observed.Commit, paths)
	if err != nil || !changed {
		return nil, nil, "native_check_recipe_unchanged"
	}
	proof := *previous
	proof.PreviousAttemptID, proof.PreviousCommit = d.LatestAttempt.ID, previous.TargetCommit
	proof.ReviewRequestID, proof.ReviewPaths, proof.RevalidationKind = request, paths, ""
	proof.ReportingRecoveryAttemptID, proof.VerifierRecoveryAttemptID, proof.CapacityRecoveryAttemptID = "", "", ""
	proof.TargetCommit, proof.ObservedAt = observed.Commit, observed.ObservedAt
	if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, &proof) != nil {
		return nil, nil, "native_check_lineage_invalid"
	}
	for _, origin := range requests {
		if origin.RequestID == proof.SourceRequestID && origin.OwnerID == proof.OwnerID && origin.RoomID == proof.RoomID && s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) && s.taskContinuationAfterCancel(ctx, origin, d) && s.liveInspectionReadScopeAllowed(ctx, d, proof.SourceAttemptID) {
			return &proof, &origin, "native_check_recipe_ready"
		}
	}
	return nil, nil, "native_check_original_authority_unavailable"
}
