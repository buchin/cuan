package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/task"
)

// A continuation is a proposal tied to an independently saved assessment. It
// grants no authority until the owner selects the persisted message.
type inboxContinuationPlan struct {
	RemainingWork       string `json:"remaining_work,omitempty"`
	TaskID              string `json:"task_id"`
	AttemptID           string `json:"attempt_id"`
	AssessmentRequestID string `json:"assessment_request_id"`
	Revision            string `json:"revision"`
}

func continuationPlan(d *task.TaskDetailResponse, request string, decision inboxReviewEvaluation) string {
	if d == nil || d.LatestAttempt == nil || d.ProjectID == "" || (decision.Decision != "revise" && decision.Decision != "blocked") || strings.TrimSpace(decision.Revision) == "" || decision.RemainingWork == "external" {
		return ""
	}
	title, description := continuationPresentation(decision.RemainingWork)
	plan := inboxApprovalPlan{Title: title, Description: description, ProjectID: d.ProjectID,
		Continuation: &inboxContinuationPlan{TaskID: d.ID, AttemptID: d.LatestAttempt.ID, AssessmentRequestID: request, Revision: decision.Revision, RemainingWork: decision.RemainingWork}}
	raw, _ := json.Marshal(plan)
	return "\n\n```cuan-approval\n" + string(raw) + "\n```"
}

func (s *Server) validateInboxContinuation(ctx context.Context, room string, plan *inboxApprovalPlan) error {
	if plan.Continuation == nil {
		return nil
	}
	c := plan.Continuation
	invalid := errors.New("This recommendation is no longer current. Check the latest task result before approving.")
	if s.taskSvc == nil || c.AssessmentRequestID == "" {
		return invalid
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, c.TaskID)
	if err != nil || d == nil || d.SourceRoomID != room || d.ProjectID != plan.ProjectID || d.Status != task.TaskStatusBlocked || d.LatestAttempt == nil || d.LatestAttempt.ID != c.AttemptID || d.LatestAttempt.Status == task.AttemptStatusCanceled || d.LatestAttempt.Status == task.AttemptStatusRunning || d.LatestAttempt.Status == task.AttemptStatusQueued {
		return invalid
	}
	if launch, err := s.taskSvc.LatestLaunch(ctx, d.ID); err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return invalid
	} else if launch != nil && (launch.State == "queued" || launch.State == "claimed" || launch.State == "started") {
		return invalid
	}
	assessment, ok := reviewAssessment(d, c.AssessmentRequestID)
	if !ok || (assessment.Decision != "revise" && assessment.Decision != "blocked") || assessment.RemainingWork == "external" || strings.TrimSpace(c.Revision) == "" || !continuationDescriptionMatches(plan, assessment) {
		return invalid
	}
	if assessment.Revision != "" && c.Revision != assessment.Revision {
		return invalid
	}
	return nil
}
