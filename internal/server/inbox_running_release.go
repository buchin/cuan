package server

import (
	"context"
	"crypto/sha256"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
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
	if s.cfg == nil || s.cfg.Environment != "production" || s.projectSvc == nil || d == nil || d.ProjectID == "" {
		return nil
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if err != nil || p == nil || (p.IsArchived && p.ID != "default") {
		return nil
	}
	owner, name, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil {
		return nil
	}
	repository := strings.ToLower(owner + "/" + name)
	if repository == s.cfg.SourceRepository && releaseObjectID.MatchString(s.cfg.CommitSHA) && s.cfg.SourceRepositoryURL() != "" {
		return &runningProjectRelease{Repository: repository, Commit: s.cfg.CommitSHA, Provenance: "current_process_build", ObservedAt: time.Now().UTC()}
	}
	c, err := publication.FromEnvironment(project.ParseEnvironment(project.SettingsFromMetadata(p.Metadata).Environment))
	if err != nil || (p.IsArchived && p.ID != "default") {
		return nil
	}
	// One observation per project at a time. Short-lived caching also bounds
	// repeated review reads after an unavailable SSH target. Configuration
	// changes invalidate both success and failure without retaining credentials.
	entry, _ := s.releaseObservations.LoadOrStore(p.ID, &releaseObservationCache{})
	cache := entry.(*releaseObservationCache)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	fingerprint := sha256.Sum256(append([]byte(p.RepoURL+"\n"), p.Metadata...))
	if cache.fingerprint == fingerprint && time.Now().Before(cache.expires) {
		return cache.result
	}
	observe := s.publicationObserver
	if observe == nil {
		observe = publication.Observe
	}
	commit, err := observe(ctx, c)
	cache.fingerprint, cache.expires, cache.result = fingerprint, time.Now().Add(30*time.Second), nil
	if err != nil || !releaseObjectID.MatchString(commit) {
		return nil
	}
	// Revocation or retargeting while an SSH read is in flight invalidates it.
	fresh, err := s.projectSvc.Get(ctx, p.ID)
	if err != nil || fresh == nil || (fresh.IsArchived && fresh.ID != "default") || fresh.RepoURL != p.RepoURL || string(fresh.Metadata) != string(p.Metadata) {
		return nil
	}
	cache.result = &runningProjectRelease{Repository: repository, Commit: commit, Provenance: "registered_ssh_release", ObservedAt: time.Now().UTC()}
	return cache.result
}

type releaseObservationCache struct {
	mu          sync.Mutex
	fingerprint [32]byte
	expires     time.Time
	result      *runningProjectRelease
}
