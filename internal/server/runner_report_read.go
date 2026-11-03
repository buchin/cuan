package server

import (
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

// Called only behind runnerAuthMiddleware. Never returns task instructions,
// environment, capability tokens or a new reporting grant.
func (s *Server) handleRunnerReadAttemptReport(w http.ResponseWriter, r *http.Request) {
	detail, err := s.taskSvc.GetTaskDetail(r.Context(), r.URL.Query().Get("task_id"))
	if err != nil || detail == nil || detail.LatestAttempt == nil {
		respondJSONError(w, 404, "Active attempt not found")
		return
	}
	att := detail.LatestAttempt
	if att.ID != chi.URLParam(r, "id") || att.RunnerID == "" || att.RunnerID != r.URL.Query().Get("runner_id") || att.Status != task.AttemptStatusRunning || att.CancelRequested || (att.LeaseExpiresAt != nil && !att.LeaseExpiresAt.After(time.Now())) {
		respondJSONError(w, 409, "Attempt is not active for this runner")
		return
	}
	type reportActivity struct {
		AttemptID    *string `json:"attempt_id"`
		ActivityType string  `json:"activity_type"`
		NewStatus    string  `json:"new_status"`
	}
	activities := []reportActivity{}
	for _, a := range detail.Activities {
		if a != nil && a.AttemptID != nil && *a.AttemptID == att.ID && a.ActivityType == "agent_report" {
			activities = append(activities, reportActivity{a.AttemptID, a.ActivityType, a.NewStatus})
		}
	}
	respondJSON(w, 200, map[string]any{"summary": detail.Summary, "complete_note": detail.CompleteNote, "dod_checklist": detail.DoDChecklist, "activities": activities})
}
