package project

import (
	"context"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/git"
)

// CheckNativeReleaseTarget re-observes the frozen branch before execution. A
// prepared old target is not silently rebound to a newer remote revision.
func (s *Service) CheckNativeReleaseTarget(ctx context.Context, projectID, instructions, repository, branch, target string) error {
	if !git.IsCommitHash(target) {
		return errors.New("native release target identity unavailable")
	}
	p, err := s.TaskContext(ctx, projectID, instructions)
	if err != nil || p == nil || p.IsArchived {
		return errors.New("saved release project unavailable")
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	settings := SettingsFromMetadata(p.Metadata)
	expected := strings.TrimSpace(settings.DeployBranch)
	if expected == "" {
		expected = p.DefaultBranch
	}
	if err != nil || repository != owner+"/"+repo || branch != expected || !settings.IsAutoDeployEnabled() || ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" {
		return errors.New("native release target differs from frozen project")
	}
	current, err := s.store.GetProject(ctx, projectID)
	if err != nil || current == nil || current.IsArchived {
		return errors.New("saved release connection unavailable")
	}
	token, err := s.repositoryToken(ctx, current.EncryptedToken)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return ErrGitHubConnectionRequired
	}
	observed, err := s.gitClient.BranchCommit(ctx, owner, repo, branch, token)
	if err != nil {
		return err
	}
	if observed != target {
		return errors.New("remote release branch changed; prepared target was not rebound")
	}
	return nil
}
