package server

import (
	"encoding/json"
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
	activities := []any{}
	for _, a := range detail.Activities {
		if a != nil && a.AttemptID != nil && *a.AttemptID == att.ID && a.ActivityType == "agent_report" {
			activities = append(activities, reportActivity{a.AttemptID, a.ActivityType, a.NewStatus})
		}
		if a != nil && a.AttemptID != nil && *a.AttemptID == att.ID && a.ActivityType == "deployment_verified" && a.Author == "cuan-deploy" {
			var receipt task.DeploymentReceipt
			if json.Unmarshal([]byte(a.Message), &receipt) == nil && task.VerifiedDeployment([]*task.TaskActivity{a}, att, receipt.CommitSHA, time.Now().UTC()) {
				// Return only the typed deployment fact, never arbitrary activity
				// text or metadata through this narrow runner recovery endpoint.
				payload, _ := json.Marshal(receipt)
				activities = append(activities, &task.TaskActivity{ID: a.ID, TaskID: a.TaskID, AttemptID: a.AttemptID, ActivityType: a.ActivityType, Author: a.Author, Message: string(payload), CreatedAt: a.CreatedAt})
			}
		}
	}
	respondJSON(w, 200, map[string]any{"summary": detail.Summary, "complete_note": detail.CompleteNote, "dod_checklist": detail.DoDChecklist, "activities": activities})
}
