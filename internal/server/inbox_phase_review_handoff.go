package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Resume only the current saved phase decision whose earlier handoff created no
// launch. This never invokes a provider or replays a task execution. Normal launch
// persistence and phase authorization remain the only path to execution.
func (s *Server) recoverReviewedPhaseHandoff(ctx context.Context, d *task.TaskDetailResponse, requests []db.InboxRequest) bool {
	if d == nil || d.Status != task.TaskStatusReview || !reviewSnapshotRecoveryAllowed(d) || s.projectStudioRepo == nil {
		return false
	}
	source, err := s.projectStudioRepo.GetMessageByID(ctx, d.SourceMessageID)
	if err != nil || source == nil || source.AuthorType != "system" || source.AuthorID != "cuan_auto_delegation" {
		return false
	}
	assessment, found := reviewAssessment(d, "")
	if !found || assessment.Decision != "revise" || assessment.ManagerAction == "wait" || assessment.ManagerAction == "needs_owner" {
		return false
	}
	for _, q := range requests {
		if q.State != "completed" || q.RoomID != d.SourceRoomID || !s.sameRuntimeAgent(ctx, q.AgentID, d.AssignedAgent) {
			continue
		}
		saved, ok := reviewAssessment(d, q.RequestID)
		if !ok || saved != assessment {
			continue
		}
		if _, err := s.phaseApproval(ctx, d, q.OwnerID, q.RoomID); err != nil {
			continue
		}
		if bound := s.inboxReviewedTask(ctx, q); bound == nil || bound.ID != d.ID {
			continue
		}
		if taskRevisionAttemptCount(s.reviewBudgetDetail(ctx, d)) >= 3 && !s.managerContinuationAllowed(ctx, q, d, assessment.Revision) {
			continue
		}
		if launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); !errors.Is(err, task.ErrTaskNotFound) || launch != nil {
			continue
		}
		raw, err := json.Marshal(assessment)
		if err != nil {
			return false
		}
		outcome := s.applyInboxReviewEvaluation(ctx, q, string(raw))
		launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
		if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
			return false
		}
		if message, err := s.projectStudioRepo.GetMessageByID(ctx, q.ReplyID); err == nil && message != nil && message.RoomID == q.RoomID && message.AuthorType == "agent" {
			message.Content = outcome
			if err := s.projectStudioRepo.UpdateMessage(ctx, message); err != nil {
				log.Printf("phase review handoff reply update failed request=%s task=%s", q.RequestID, d.ID)
			}
		}
		s.publishInbox(q.RoomID)
		return true
	}
	return false
}
