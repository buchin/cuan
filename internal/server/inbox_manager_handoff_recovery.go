package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Complete a persisted manager decision whose native admission was never saved.
// Never replay a model/owner message, an existing launch, or a typed inspection
// through ordinary execution. Normal approval/scope/repeat checks still apply.
func (s *Server) recoverManagerVerificationHandoff(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest, requests []db.InboxRequest) bool {
	if !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt == nil {
		return false
	}
	live, liveErr := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if liveErr != nil {
		return false
	}
	inspectedSource := false
	localCorrection := false
	if live != nil {
		inspectedSource = live.OwnerID == origin.OwnerID && s.testedSourceAfterFailedInspection(ctx, d, live) != nil
		if !inspectedSource {
			for _, reviewed := range requests {
				if reviewed.State == "completed" && reviewed.OwnerID == origin.OwnerID {
					if decision, found := reviewAssessment(d, reviewed.RequestID); found && s.localSourceCorrectionAllowed(ctx, reviewed, d, decision.Revision) {
						localCorrection = true
						break
					}
				}
			}
			if !localCorrection {
				return false
			}
		}
	}
	proof, proofErr := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if proofErr != nil {
		return false
	}
	sourceCorrection := false
	if proof != nil {
		source, verified := s.verifiedSupplementalSource(ctx, d)
		sourceCorrection = source != nil && verified != nil && verified.OwnerID == origin.OwnerID
		if !sourceCorrection {
			return false
		}
	}
	decision, found := reviewAssessment(d, "")
	if !found || decision.Decision != "revise" || (decision.RemainingWork != "verification" && !((sourceCorrection || localCorrection) && decision.RemainingWork == "implementation")) {
		return false
	}
	for _, q := range requests {
		if q.State != "completed" || q.OwnerID != origin.OwnerID || q.RoomID != d.SourceRoomID {
			continue
		}
		saved, found := reviewAssessment(d, q.RequestID)
		if !found || saved != decision {
			continue
		}
		if inspectedSource {
			message, messageErr := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
			if messageErr != nil || message == nil || !strings.Contains(message.Content, inboxOwnershipPolicy) || s.inboxReviewedTask(ctx, q) == nil {
				continue
			}
			if saved.ManagerReason == "" || (saved.ManagerAction != "continue" && saved.ManagerAction != "replan") {
				continue
			}
			if _, scheduled := s.startVerificationFollowup(ctx, q, d, saved); scheduled {
				s.publishInbox(d.SourceRoomID)
				return true
			}
			return false
		}
		if sourceCorrection && saved.RemainingWork == "verification" {
			reconciled := s.requirePublicationReview(ctx, d, saved)
			if reconciled.Decision == "revise" && reconciled.RemainingWork == "implementation" {
				// A changed native finding deserves a fresh saved PM decision,
				// not mutation of the old assessment or replay of its worker.
				return s.queueManagerAssessment(ctx, q, d)
			}
		}
		// A native receipt-only failure has already retained a tested checkpoint
		// and a saved verification decision. Its independent, read-only handoff
		// does not need optional metadata authorizing another implementation run.
		if saved.RemainingWork == "verification" && saved.RequestedOutcome != "needs_input" && saved.ManagerAction != "wait" && saved.ManagerAction != "needs_owner" &&
			task.IsPublicationReconciliationOutcome(&d.Task, d.LatestAttempt, d.Activities) && s.independentCodeReviewMissing(ctx, d) {
			message, messageErr := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
			if messageErr != nil || message == nil || !strings.Contains(message.Content, inboxOwnershipPolicy) || s.inboxReviewedTask(ctx, q) == nil {
				continue
			}
			if _, scheduled := s.startVerificationFollowup(ctx, q, d, saved); scheduled {
				s.publishInbox(d.SourceRoomID)
				return true
			}
			return false
		}
		if !s.managerContinuationAllowed(ctx, q, d, saved.Revision) {
			continue
		}
		message, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
		if err != nil || message == nil || !strings.Contains(message.Content, inboxOwnershipPolicy) {
			continue
		}
		launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
		if (err != nil && !errors.Is(err, task.ErrTaskNotFound)) || launch != nil {
			continue
		}
		raw, err := json.Marshal(saved)
		if err != nil {
			return false
		}
		s.applyInboxReviewEvaluation(ctx, q, string(raw))
		sourceRequest := task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID)
		if sourceLaunch, sourceErr := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, sourceRequest)); sourceErr == nil && sourceLaunch != nil && (sourceLaunch.State == "queued" || sourceLaunch.State == "claimed" || sourceLaunch.State == "started") {
			proof, proofErr := task.VerificationIntentForLaunch(&d.Task, sourceLaunch)
			if proofErr == nil && proof != nil && proof.SourceAttemptID == d.LatestAttempt.ID && proof.SourceCommit == d.CommitSHA && proof.OwnerID == origin.OwnerID {
				s.publishInbox(d.SourceRoomID)
				return true
			}
		}
		launch, err = s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
		if err == nil && launch != nil && (launch.State == "queued" || launch.State == "claimed" || launch.State == "started") {
			s.publishInbox(d.SourceRoomID)
			return true
		}
		return false
	}
	return false
}
