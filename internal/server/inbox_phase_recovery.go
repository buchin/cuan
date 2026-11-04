package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type inboxHandoffPersistenceKey struct{}

const inboxHandoffPersistenceWaiting = "Saving the verified handoff. I’ll retry automatically."

type phaseHandoffIntent struct {
	Version          int                   `json:"version"`
	Owner            string                `json:"owner"`
	Review           db.InboxRequest       `json:"review"`
	InstructionsHash string                `json:"instructions_hash"`
	BeforeDigest     string                `json:"before_digest"`
	AcceptedDigest   string                `json:"accepted_digest"`
	Decision         inboxReviewEvaluation `json:"decision"`
}

func (s *Server) savePhaseHandoffIntent(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, accepted *task.TaskDetailResponse, decision inboxReviewEvaluation) error {
	if d.LatestAttempt == nil {
		return phaseDenied("phase attempt unavailable")
	}
	id := "act_handoff_" + delegationMessageID(d.ID+":"+d.LatestAttempt.ID+":"+q.RequestID+":"+inboxReviewEvidenceDigest(accepted))
	// Replaying the same verified decision must not rewrite the original evidence.
	for _, a := range d.Activities {
		if a != nil && a.ID == id {
			if a.Author != "cuan-plan" || a.ActivityType != task.PhaseHandoffIntent {
				return phaseDenied("handoff intent identity changed")
			}
			return nil
		}
	}
	intent := phaseHandoffIntent{Version: 1, Owner: q.OwnerID, Review: q, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), BeforeDigest: inboxReviewEvidenceDigest(d), AcceptedDigest: inboxReviewEvidenceDigest(accepted), Decision: decision}
	// Runtime progress is not part of the durable review identity.
	intent.Review.Steps = nil
	raw, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	err = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: id, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.PhaseHandoffIntent, Author: "cuan-plan", Message: string(raw), CreatedAt: time.Now().UTC()})
	if err != nil {
		fresh, e := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if e == nil {
			for _, a := range fresh.Activities {
				if a != nil && a.ID == id && a.Author == "cuan-plan" && a.ActivityType == task.PhaseHandoffIntent && a.Message == string(raw) {
					return nil
				}
			}
		}
	}
	return err
}

// Recovery uses only saved coordinator decisions. It never calls a provider,
// creates new consent, or starts a replacement attempt for the accepted parent.
func (s *Server) recoverPhaseHandoffs(ctx context.Context) {
	store, ok := s.taskSvc.Store().(task.PendingCoordinatorStore)
	if !ok {
		return
	}
	now := time.Now().UTC()
	intents, err := store.PendingPhaseHandoffs(ctx, now, 20)
	if err != nil {
		return
	}
	for _, event := range intents {
		if ctx.Err() != nil {
			return
		}
		s.recoverPhaseHandoff(ctx, event, now)
	}
}
func (s *Server) recoverPhaseHandoff(ctx context.Context, event *task.TaskActivity, now time.Time) {
	key := "phase-recovery:" + event.ID
	if _, busy := s.inboxWorkers.LoadOrStore(key, true); busy {
		return
	}
	defer s.inboxWorkers.Delete(key)
	finish := func(reason string) {
		_ = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: event.ID + ":done", TaskID: event.TaskID, AttemptID: event.AttemptID, ActivityType: "phase_handoff_finished", Author: "cuan-plan", Message: reason, CreatedAt: now})
	}
	retry := func() {
		_ = s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: fmt.Sprintf("%s:retry:%d", event.ID, now.Unix()/300), TaskID: event.TaskID, AttemptID: event.AttemptID, ActivityType: "phase_handoff_retry", Author: "cuan-plan", Message: event.ID, CreatedAt: now})
	}
	var intent phaseHandoffIntent
	if event.Author != "cuan-plan" || event.ActivityType != task.PhaseHandoffIntent || json.Unmarshal([]byte(event.Message), &intent) != nil || intent.Version != 1 || intent.Owner == "" || intent.Decision.Decision != "pass" || event.AttemptID == nil {
		finish("Invalid handoff intent; no phase started.")
		return
	}
	if now.Sub(event.CreatedAt) > 24*time.Hour {
		finish("Handoff recovery expired; no phase started.")
		return
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, event.TaskID)
	if err != nil {
		retry()
		return
	}
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.ID != *event.AttemptID || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil || task.LaunchInstructionsHash(d.Instructions) != intent.InstructionsHash {
		finish("Parent attempt or approved scope changed; no phase started.")
		return
	}
	q := intent.Review
	q.OwnerID = intent.Owner
	if d.Status != task.TaskStatusDone {
		if inboxReviewEvidenceDigest(d) != intent.BeforeDigest {
			finish("Review evidence changed; no phase started.")
			return
		}
		_, scopeVerified, scopeErr := s.inboxReviewRepositoryScope(ctx, d)
		if scopeErr != nil {
			retry()
			return
		}
		if !scopeVerified || !inboxReviewExecutionSucceeded(d) || len(task.AcceptanceGaps(&d.Task)) > 0 {
			finish("Original acceptance gates no longer pass; no phase started.")
			return
		}
		// The original review binding and all acceptance gates are checked again.
		raw, _ := json.Marshal(intent.Decision)
		s.applyInboxReviewEvaluation(ctx, q, string(raw))
		d, err = s.taskSvc.GetTaskDetail(ctx, event.TaskID)
		if err != nil || d == nil || d.Status != task.TaskStatusDone {
			retry()
			return
		}
	}
	if inboxReviewEvidenceDigest(d) != intent.AcceptedDigest {
		finish("Accepted evidence changed; no phase started.")
		return
	}
	_, err = s.attemptInboxAutoDelegation(ctx, q, d, intent.Decision.Explanation)
	if err != nil {
		var denied *phaseAuthorizationError
		if errors.As(err, &denied) {
			finish("Approval no longer valid; no phase started.")
		} else {
			retry()
		}
		return
	}
	finish("Handoff recorded; execution belongs to the durable launch worker.")
}
