package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

type taskResumePlan struct {
	Available bool             `json:"available"`
	AttemptID string           `json:"attempt_id,omitempty"`
	RequestID string           `json:"-"`
	Origin    *db.InboxRequest `json:"-"`
}

// Resume requests the existing PM handoff, preserving its phase, original approval,
// workspace and independent verification gates. It never restarts a dev run itself.
func (s *Server) taskResumePlan(ctx context.Context, owner string, d *task.TaskDetailResponse) (taskResumePlan, error) {
	plan := taskResumePlan{}
	if !reviewSnapshotRecoveryAllowed(d) || d.SourceRoomID == "" || currentTaskReviewAction(d) == "needs_owner" || currentTaskReviewAction(d) == "wait" {
		return plan, nil
	}
	plan.AttemptID = d.LatestAttempt.ID
	if d.Status != "blocked" && d.LatestAttempt.Status != "failed" && d.LatestAttempt.Status != "interrupted" {
		return plan, nil
	}
	repo, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return plan, errors.New("conversation supervision unavailable")
	}
	originalOwner, err := s.taskArtifactOwner(ctx, d)
	if err != nil || originalOwner != owner {
		return plan, nil
	}
	requests, err := repo.ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		return plan, err
	}
	key := ownerResumeReviewKey(d)
	plan.RequestID = delegationMessageID(key)
	for i := range requests {
		q := requests[i]
		if q.OwnerID != owner {
			continue
		}
		reviewed := s.inboxReviewedTask(ctx, q)
		if reviewed == nil || reviewed.ID != d.ID {
			continue
		}
		if q.RequestID == plan.RequestID {
			return plan, nil
		}
		if (q.State == "queued" || q.State == "running" || q.State == "cancelling") && q.Deadline.After(time.Now()) {
			return plan, nil
		}
		if q.State == "completed" || q.State == "failed" || q.State == "interrupted" {
			copied := q
			plan.Origin = &copied
		}
	}
	plan.Available = plan.Origin != nil
	return plan, nil
}

func (s *Server) handleTaskResume(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	if s.taskSvc == nil || s.projectStudioRepo == nil || s.terminalManager == nil {
		respondJSONError(w, 503, "Task continuation is unavailable")
		return
	}
	id := chi.URLParam(r, "id")
	if r.Method == http.MethodPost {
		if _, busy := s.taskLaunches.LoadOrStore(id, true); busy {
			respondJSONError(w, 409, "This work is already being continued")
			return
		}
		defer s.taskLaunches.Delete(id)
	}
	detail, err := s.taskSvc.GetTaskDetail(r.Context(), id)
	if err != nil {
		respondJSONError(w, 404, "Task not found")
		return
	}
	var expected string
	if r.Method == http.MethodPost {
		var body struct {
			AttemptID string `json:"attempt_id"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || body.AttemptID == "" {
			respondJSONError(w, 400, "Choose the paused attempt to continue")
			return
		}
		expected = body.AttemptID
		if detail.LatestAttempt == nil || expected != detail.LatestAttempt.ID {
			respondJSONError(w, 409, "The work has changed. Check its latest status.")
			return
		}
	}
	if r.Method == http.MethodPost {
		if !reviewSnapshotRecoveryAllowed(detail) {
			respondJSONError(w, 409, "This task is no longer paused")
			return
		}
		if repo, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			requests, err := repo.ListInboxRequests(r.Context(), detail.SourceRoomID)
			if err != nil {
				respondJSONError(w, 503, "Could not check the current continuation")
				return
			}
			for _, q := range requests {
				if q.OwnerID == owner && (q.State == "queued" || q.State == "running") && q.Deadline.After(time.Now()) {
					reviewed := s.inboxReviewedTask(r.Context(), q)
					if reviewed != nil && reviewed.ID == id {
						respondJSON(w, 202, map[string]any{"task_id": id, "attempt_id": expected, "request_id": q.RequestID, "status": q.State, "replayed": true})
						return
					}
				}
			}
		}
	}
	plan, err := s.taskResumePlan(r.Context(), owner, detail)
	if err != nil {
		respondJSONError(w, 503, "Could not check whether this work can continue")
		return
	}
	if r.Method == http.MethodGet {
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, 200, plan)
		return
	}
	if !plan.Available {
		respondJSONError(w, 409, "Check the latest task finding before continuing this work")
		return
	}
	if active, err := s.terminalManager.TaskExecutionActive(id); err != nil || active {
		respondJSONError(w, 409, "This work still has an active process")
		return
	}
	attempts, err := s.taskSvc.Store().ListAttempts(r.Context(), id)
	if err != nil {
		respondJSONError(w, 503, "Could not check the current work")
		return
	}
	for _, attempt := range attempts {
		if attempt.Status == "running" || attempt.Status == "queued" {
			respondJSONError(w, 409, "This work is already running or queued")
			return
		}
	}
	if !s.queueManagerAssessment(context.WithValue(r.Context(), taskOwnerResumeKey{}, true), *plan.Origin, detail) {
		respondJSONError(w, 409, "The continuation was not confirmed. Check the latest task status.")
		return
	}
	_, _ = s.taskSvc.AddComment(r.Context(), id, owner, "The owner requested continuation of the paused work through Cuan's existing supervision.")
	s.publishInbox(detail.SourceRoomID)
	respondJSON(w, 202, map[string]any{"task_id": id, "attempt_id": expected, "request_id": plan.RequestID, "status": "queued"})
}

// One owner-triggered reassessment per current evidence snapshot. This key does
// not renew any developer/verifier recovery allowance or change original scope.
func ownerResumeReviewKey(d *task.TaskDetailResponse) string {
	return "task-review:" + d.LatestAttempt.ID + ":owner-resume:" + inboxReviewEvidenceDigest(d)
}
