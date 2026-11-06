package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// Only the authenticated native runner may request this independent observation.
// Callers choose an expected revision, never a command, target, credential or fact.
func (s *Server) handleRunnerVerifyPublication(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TaskID   string `json:"task_id"`
		RunnerID string `json:"runner_id"`
		Commit   string `json:"commit_sha"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || !releaseObjectID.MatchString(input.Commit) {
		respondJSONError(w, 400, "Invalid publication verification request")
		return
	}
	active := func() (*task.TaskDetailResponse, bool) {
		d, err := s.taskSvc.GetTaskDetail(r.Context(), input.TaskID)
		if err != nil || d == nil || d.LatestAttempt == nil {
			return nil, false
		}
		a := d.LatestAttempt
		return d, a.ID == chi.URLParam(r, "id") && a.RunnerID != "" && a.RunnerID == input.RunnerID && a.Status == task.AttemptStatusRunning && !a.CancelRequested && a.CanceledAt == nil && (a.LeaseExpiresAt == nil || a.LeaseExpiresAt.After(time.Now()))
	}
	d, ok := active()
	if !ok {
		respondJSONError(w, 409, "Attempt is no longer active for this runner")
		return
	}
	if s.projectSvc == nil || d.ProjectID == "" || (d.CommitSHA != "" && d.CommitSHA != input.Commit) {
		respondJSONError(w, 409, "Publication does not match the task project result")
		return
	}
	p, err := s.projectSvc.Get(r.Context(), d.ProjectID)
	if err != nil || p == nil {
		respondJSONError(w, 503, "Registered publication verifier is unavailable")
		return
	}
	c, err := publication.FromEnvironment(project.ParseEnvironment(project.SettingsFromMetadata(p.Metadata).Environment))
	if err != nil {
		respondJSONError(w, 503, "Registered publication verifier is unavailable")
		return
	}
	receipt := task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: input.Commit, ReleaseSHA: input.Commit, Environment: "production", Health: "healthy"}
	if task.VerifiedDeployment(d.Activities, d.LatestAttempt, input.Commit, time.Now().UTC()) {
		respondJSON(w, 200, map[string]any{"verified": true, "receipt": receipt})
		return
	}
	verify := s.publicationVerifier
	if verify == nil {
		verify = publication.Verify
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if verify(ctx, c, input.Commit) != nil {
		respondJSONError(w, 422, "Production revision and health could not be independently verified")
		return
	}
	fresh, ok := active()
	if !ok || fresh.ProjectID != d.ProjectID || fresh.CommitSHA != d.CommitSHA {
		respondJSONError(w, 409, "Attempt or result changed during verification")
		return
	}
	// Re-read settings so a revoked/replaced connection cannot publish late proof.
	current, err := s.projectSvc.Get(r.Context(), d.ProjectID)
	if err != nil || current == nil || string(current.Metadata) != string(p.Metadata) {
		respondJSONError(w, 409, "Project changed during verification")
		return
	}
	raw, _ := json.Marshal(receipt)
	event := &task.TaskActivity{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, input.Commit), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: time.Now().UTC()}
	if s.taskSvc.Store().CreateActivity(r.Context(), event) != nil {
		respondJSONError(w, 503, "Verified publication could not be recorded")
		return
	}
	respondJSON(w, 200, map[string]any{"verified": true, "receipt": receipt})
}
