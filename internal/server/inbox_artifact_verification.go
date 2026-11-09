package server

import (
	"context"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

// A harness subagent is still part of the developer attempt. File work uses
// the existing manager verification continuation, with no source checkout.
func (s *Server) artifactIndependentlyVerified(ctx context.Context, d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || !inboxReviewExecutionSucceeded(d) || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return false
	}
	store, ok := s.taskSvc.Store().(task.AttemptLaunchStore)
	if !ok {
		return false
	}
	launch, err := store.LaunchForAttempt(ctx, d.LatestAttempt.ID)
	if err != nil || launch == nil || launch.TaskID != d.ID || launch.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || !managerVerificationObservationGrant(d, launch) {
		return false
	}
	origin, authorized, err := s.authorizeSavedTaskLaunch(ctx, *launch, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	policy, unchanged, err := s.inboxReviewRepositoryScope(ctx, d)
	return err == nil && policy.ReadOnlyRepository && unchanged
}

func (s *Server) requireIndependentArtifactReview(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, assessment inboxReviewEvaluation) inboxReviewEvaluation {
	if d == nil || d.Status == task.TaskStatusDone || d.ExecutionCategory != task.ExecutionCategoryOps || !executionpolicy.StandaloneArtifactWork(d.Instructions) || assessment.Decision != "pass" || !inboxReviewExecutionSucceeded(d) || s.artifactIndependentlyVerified(ctx, d) {
		return assessment
	}
	inputs, err := s.deliveredFileInputs(ctx, q.OwnerID, d, "Delivered result awaiting verification", "Files awaiting independent verification")
	if err != nil || inputs == "" {
		return inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "implementation", Finding: "Berkas hasil belum tersedia untuk diperiksa.", Explanation: "Berkas hasil belum tersedia untuk diperiksa.", Revision: "Preserve existing work and restore the current attempt's original downloadable artifact delivery. Do not regenerate the result or change scope merely to obtain evidence; report any precise missing-file gap.", ManagerAction: "replan", ManagerReason: "Current owner-scoped file delivery is unavailable."}
	}
	return inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", Finding: "Berkas sedang diperiksa dalam sesi terpisah.", Explanation: "Berkas sedang diperiksa dalam sesi terpisah.", Revision: "Independently inspect the existing delivered files against every original requirement in this separate native session. Do not delegate this check back to the developer or to a harness subagent. Read the actual supplied files and verify their hashes and contents; worker claims are not observations. Preserve originals and the private artifact workspace. Do not open or clone the application repository, run application tests, or repeat external effects. Report concrete checks with task report and attach the verified files as downloadable session artifacts. If changes are needed, report the gap without implementing it.\n" + inputs, ManagerAction: "continue", ManagerReason: "Standalone artifacts require a separate native verification attempt; a developer subagent cannot establish this."}
}
