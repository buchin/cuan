package server

import (
	"context"
	"github.com/buchin/cuan/internal/task"
)

var reviewFindingActivityKinds = []string{reviewAssessmentActivity, "agent_report", task.ActivityTypeRunnerVerification, task.ActivityScopeAcceptanceAudit, publicationConfigurationActivity, githubObservationActivity, "repository_scope_observed", task.ActivityRunnerFinalization, recoveredVerificationArtifactsActivity}

func currentTaskReviewFinding(d *task.TaskDetailResponse) string {
	if d == nil || d.LatestAttempt == nil || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || d.LatestAttempt.Status == task.AttemptStatusRunning || d.LatestAttempt.Status == task.AttemptStatusQueued || d.LatestAttempt.Status == task.AttemptStatusCanceled {
		return ""
	}
	assessment, ok := reviewAssessment(d, "")
	if !ok || (assessment.Decision != "revise" && assessment.Decision != "blocked") {
		return ""
	}
	return assessment.Finding
}

func (s *Server) projectTaskReviewFindings(ctx context.Context, items []*task.TaskItemResponse) error {
	ids := []string{}
	for _, item := range items {
		if item != nil {
			item.ReviewFinding = ""
			if item.LatestAttempt != nil && (item.Status == task.TaskStatusBlocked || item.Status == task.TaskStatusReview) {
				ids = append(ids, item.ID)
			}
		}
	}
	if len(ids) == 0 || s.taskSvc == nil {
		return nil
	}
	reader, ok := s.taskSvc.Store().(task.CurrentAttemptActivityStore)
	if !ok {
		return nil
	} // Legacy custom stores do not invent a finding or fall back to N+1 reads.
	rows, err := reader.CurrentAttemptActivities(ctx, ids, reviewFindingActivityKinds)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item != nil {
			item.ReviewFinding = currentTaskReviewFinding(&task.TaskDetailResponse{Task: item.Task, LatestAttempt: item.LatestAttempt, Activities: rows[item.ID]})
		}
	}
	return nil
}
