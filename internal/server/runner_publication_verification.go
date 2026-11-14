package server

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/git"
	"io"
	"net/http"
	"strings"
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
		TaskID            string     `json:"task_id"`
		RunnerID          string     `json:"runner_id"`
		Commit            string     `json:"commit_sha"`
		ProcessFinishedAt *time.Time `json:"process_finished_at,omitempty"`
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
	preparation, preparationErr := task.NativeReleasePreparationForAttempt(r.Context(), s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if preparationErr != nil {
		respondJSONError(w, 409, "Native release preparation could not be verified")
		return
	}
	if input.ProcessFinishedAt != nil && (preparation == nil || d.LatestAttempt.StartedAt == nil || input.ProcessFinishedAt.Before(*d.LatestAttempt.StartedAt) || input.ProcessFinishedAt.After(time.Now())) {
		respondJSONError(w, 409, "Invalid native process observation boundary")
		return
	}
	expectedCommit := d.CommitSHA
	if preparation != nil {
		expectedCommit = preparation.TargetCommit
	}
	if s.projectSvc == nil || d.ProjectID == "" || (expectedCommit != "" && expectedCommit != input.Commit) {
		respondJSONError(w, 409, "Publication does not match the task project result")
		return
	}
	projectContext := func() (*project.ProjectResponse, error) {
		if preparation != nil {
			return s.projectSvc.TaskContext(r.Context(), d.ProjectID, d.Instructions)
		}
		return s.projectSvc.Get(r.Context(), d.ProjectID)
	}
	p, err := projectContext()
	if err != nil || p == nil || p.IsArchived {
		respondJSONError(w, 503, "Registered publication verifier is unavailable")
		return
	}
	if preparation != nil {
		owner, repo, repoErr := git.ParseGitHubURL(p.RepoURL)
		settings := project.SettingsFromMetadata(p.Metadata)
		branch := strings.TrimSpace(settings.DeployBranch)
		if branch == "" {
			branch = p.DefaultBranch
		}
		if repoErr != nil || p.IsArchived || preparation.Repository != owner+"/"+repo || preparation.TargetBranch != branch || !settings.IsAutoDeployEnabled() || project.ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" {
			respondJSONError(w, 409, "Native release project differs from saved preparation")
			return
		}
	}
	// Cuan can observe its own production build through this authenticated
	// request. Match the registered repository and exact expected revision;
	// another project or an older process still needs its registered verifier.
	currentProcess := s.publicationConnectionState(p) == "current_process" && s.cfg.CommitSHA == input.Commit
	settings := project.SettingsFromMetadata(p.Metadata)
	hasDeployScript := strings.TrimSpace(settings.DeployScript) != ""
	c, err := publication.FromEnvironment(project.ParseEnvironment(settings.Environment))
	if err != nil && !currentProcess && !hasDeployScript {
		respondJSONError(w, 503, "Registered publication verifier is unavailable")
		return
	}
	receipt := task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: input.Commit, ReleaseSHA: input.Commit, Environment: "production", Health: "healthy"}
	if (input.ProcessFinishedAt == nil && task.VerifiedDeployment(d.Activities, d.LatestAttempt, input.Commit, time.Now().UTC())) || (input.ProcessFinishedAt != nil && task.VerifiedPostProcessDeployment(d.Activities, d.LatestAttempt, input.Commit, *input.ProcessFinishedAt, time.Now().UTC())) {
		respondJSON(w, 200, map[string]any{"verified": true, "receipt": receipt})
		return
	}
	if !hasDeployScript && !currentProcess {
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
	}
	fresh, ok := active()
	if !ok || fresh.ProjectID != d.ProjectID || fresh.CommitSHA != d.CommitSHA {
		respondJSONError(w, 409, "Attempt or result changed during verification")
		return
	}
	if preparation != nil {
		currentPreparation, err := task.NativeReleasePreparationForAttempt(r.Context(), s.taskSvc.Store(), &fresh.Task, fresh.LatestAttempt.ID)
		if err != nil || currentPreparation == nil || *currentPreparation != *preparation {
			respondJSONError(w, 409, "Native release preparation changed during verification")
			return
		}
	}
	// Re-read settings so a revoked/replaced connection cannot publish late proof.
	current, err := projectContext()
	if err != nil || current == nil || current.IsArchived || current.RepoURL != p.RepoURL || current.LocalPath != p.LocalPath || current.DefaultBranch != p.DefaultBranch || string(current.Metadata) != string(p.Metadata) {
		respondJSONError(w, 409, "Project changed during verification")
		return
	}
	raw, _ := json.Marshal(receipt)
	event := &task.TaskActivity{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, input.Commit), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: time.Now().UTC()}
	if input.ProcessFinishedAt != nil {
		event.ID = task.PostProcessDeploymentReceiptID(d.ID, d.LatestAttempt.ID, input.Commit, *input.ProcessFinishedAt)
		event.ActivityType = "native_publication_observed"
	}
	// Keep the activation receipt immutable; post-process observation is distinct.
	if input.ProcessFinishedAt != nil && !task.VerifiedDeployment(fresh.Activities, fresh.LatestAttempt, input.Commit, time.Now().UTC()) {
		activation := *event
		activation.ID = task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, input.Commit)
		activation.ActivityType = "deployment_verified"
		if s.taskSvc.Store().CreateActivity(r.Context(), &activation) != nil {
			respondJSONError(w, 503, "Verified publication could not be recorded")
			return
		}
	}
	if s.taskSvc.Store().CreateActivity(r.Context(), event) != nil {
		respondJSONError(w, 503, "Verified publication could not be recorded")
		return
	}
	respondJSON(w, 200, map[string]any{"verified": true, "receipt": receipt})
}
