package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		return fmt.Errorf("%w: taskSvc nil or AssessmentRequestID empty", invalid)
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, c.TaskID)
	if err != nil || d == nil {
		return fmt.Errorf("%w: get task detail err=%v, d=%v", invalid, err, d)
	}
	if d.SourceRoomID != room {
		return fmt.Errorf("%w: room mismatch: %s != %s", invalid, d.SourceRoomID, room)
	}
	if d.ProjectID != plan.ProjectID {
		return fmt.Errorf("%w: project mismatch: %s != %s", invalid, d.ProjectID, plan.ProjectID)
	}
	if d.Status != task.TaskStatusBlocked {
		return fmt.Errorf("%w: status mismatch: %s != blocked", invalid, d.Status)
	}
	if d.LatestAttempt == nil || d.LatestAttempt.ID != c.AttemptID {
		return fmt.Errorf("%w: attempt mismatch", invalid)
	}
	if d.LatestAttempt.Status == task.AttemptStatusCanceled || d.LatestAttempt.Status == task.AttemptStatusRunning || d.LatestAttempt.Status == task.AttemptStatusQueued {
		return fmt.Errorf("%w: attempt status: %s", invalid, d.LatestAttempt.Status)
	}
	if launch, err := s.taskSvc.LatestLaunch(ctx, d.ID); err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return fmt.Errorf("%w: launch err=%v", invalid, err)
	} else if launch != nil && (launch.State == "queued" || launch.State == "claimed" || launch.State == "started") {
		return fmt.Errorf("%w: active launch %s", invalid, launch.State)
	}
	assessment, ok := reviewAssessment(d, c.AssessmentRequestID)
	if !ok {
		return fmt.Errorf("%w: reviewAssessment not found", invalid)
	}
	if assessment.Decision != "revise" && assessment.Decision != "blocked" {
		return fmt.Errorf("%w: decision neither revise nor blocked: %s", invalid, assessment.Decision)
	}
	if strings.TrimSpace(c.Revision) == "" {
		if strings.TrimSpace(assessment.Revision) != "" {
			c.Revision = strings.TrimSpace(assessment.Revision)
		} else if strings.TrimSpace(assessment.Explanation) != "" {
			c.Revision = strings.TrimSpace(assessment.Explanation)
		} else if strings.TrimSpace(assessment.Finding) != "" {
			c.Revision = strings.TrimSpace(assessment.Finding)
		} else {
			return fmt.Errorf("%w: empty c.Revision", invalid)
		}
	}
	if !continuationDescriptionMatches(plan, assessment) {
		return fmt.Errorf("%w: continuationDescriptionMatches false", invalid)
	}
	if assessment.Revision != "" && c.Revision != assessment.Revision {
		return fmt.Errorf("%w: assessment.Revision %s != %s", invalid, assessment.Revision, c.Revision)
	}
	return nil
}
