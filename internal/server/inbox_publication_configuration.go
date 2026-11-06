package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/task"
)

const publicationConfigurationActivity = "publication_configuration_observed"

type publicationConfigurationRecord struct {
	ProjectID string    `json:"project_id"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"project_updated_at"`
}

func (s *Server) publicationConnectionState(p *project.ProjectResponse) string {
	if p == nil {
		return "unavailable"
	}
	if s.cfg != nil && s.cfg.Environment == "production" && releaseObjectID.MatchString(s.cfg.CommitSHA) {
		owner, name, err := git.ParseGitHubURL(p.RepoURL)
		if err == nil && strings.ToLower(owner+"/"+name) == s.cfg.SourceRepository {
			return "current_process"
		}
	}
	return publication.ConfigurationState(project.ParseEnvironment(project.SettingsFromMetadata(p.Metadata).Environment))
}

func (s *Server) publicationConfigurationForTask(ctx context.Context, d *task.TaskDetailResponse) string {
	if d == nil || s.projectSvc == nil || d.ProjectID == "" {
		return "unavailable"
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil {
		return "unavailable"
	}
	return s.publicationConnectionState(p)
}

func publicationConfigurationObservation(d *task.TaskDetailResponse) *task.TaskActivity {
	if d == nil || d.LatestAttempt == nil {
		return nil
	}
	var latest *task.TaskActivity
	for _, e := range d.Activities {
		if e == nil || e.TaskID != d.ID || e.AttemptID == nil || *e.AttemptID != d.LatestAttempt.ID || e.ActivityType != publicationConfigurationActivity || e.Author != "coordinator" {
			continue
		}
		if latest == nil || e.CreatedAt.After(latest.CreatedAt) || (e.CreatedAt.Equal(latest.CreatedAt) && e.ID > latest.ID) {
			latest = e
		}
	}
	return latest
}

// A changed registered connection is new review evidence, not a new execution
// approval or deployment receipt. Wake the normal snapshot review once, without
// copying credentials, executing a task or rewriting the previous diagnosis.
func (s *Server) observeInboxPublicationConfiguration(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) {
	if d == nil || s.projectSvc == nil || d.ProjectID == "" || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) || (d.ExecutionCategory != "" && d.ExecutionCategory != task.ExecutionCategoryCode) {
		return
	}
	base := executionpolicy.FromInstructions(d.Instructions)
	if base.DenyDeploy || base.ReadOnlyRepository {
		return
	}
	owner, ownerErr := s.taskArtifactOwner(ctx, d)
	if ownerErr != nil || owner != origin.OwnerID {
		return
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil || p.IsArchived || !project.SettingsFromMetadata(p.Metadata).IsAutoDeployEnabled() {
		return
	}
	record := publicationConfigurationRecord{ProjectID: p.ID, State: s.publicationConnectionState(p), UpdatedAt: p.UpdatedAt}
	raw, _ := json.Marshal(record)
	if current := publicationConfigurationObservation(d); current != nil && current.Message == string(raw) {
		return
	}
	event := &task.TaskActivity{ID: delegationMessageID("publication-configuration:" + d.ID + ":" + d.LatestAttempt.ID + ":" + string(raw)), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: publicationConfigurationActivity, Author: "coordinator", Message: string(raw), CreatedAt: time.Now().UTC()}
	if s.taskSvc.Store().CreateActivity(ctx, event) != nil {
		return
	}
	d.Activities = append(d.Activities, event)
}

func missingPublicationConnection(state string) bool {
	return state == "not_configured" || state == "unsupported" || state == "incomplete"
}

const missingPublicationConnectionGuidance = "The registered project has no usable production verification connection. This is a setup prerequisite, not evidence that implementation or deployment failed. Reconcile the approved production target and the owner's existing saved project connections before requesting more implementation. Reuse already authorized access where available; if access is genuinely absent, identify only the indispensable connection or consent. Never borrow ambient developer credentials, bypass host trust, alter the approved target, modify application code or repeat deployment solely to obtain a receipt. Inspect the actual saved outcome against every original criterion and preserve other verified gaps."
