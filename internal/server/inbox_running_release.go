package server

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

var releaseObjectID = regexp.MustCompile(`^[a-f0-9]{40}$`)

type runningProjectRelease struct {
	Repository string    `json:"repository"`
	Commit     string    `json:"commit"`
	Provenance string    `json:"provenance"`
	ObservedAt time.Time `json:"observed_at"`
}

// Identifies this process's build only when its source matches this task's
// project. It does not prove ancestry, feature behavior, tests or deployment of
// the task's result. Old builds without source identity fail closed.
func (s *Server) runningProjectRelease(ctx context.Context, d *task.TaskDetailResponse) *runningProjectRelease {
	if s.cfg == nil || s.cfg.Environment != "production" || !releaseObjectID.MatchString(s.cfg.CommitSHA) || s.cfg.SourceRepositoryURL() == "" || s.projectSvc == nil || d == nil || d.ProjectID == "" {
		return nil
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil {
		return nil
	}
	owner, name, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil || strings.ToLower(owner+"/"+name) != s.cfg.SourceRepository {
		return nil
	}
	return &runningProjectRelease{Repository: s.cfg.SourceRepository, Commit: s.cfg.CommitSHA, Provenance: "current_process_build", ObservedAt: time.Now().UTC()}
}
