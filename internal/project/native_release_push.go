package project

import (
	"context"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
)

// PushNativeReleaseSource publishes only a previously admitted clean checkpoint.
// Native coordination derives workspace/branch/source; no PR is created here.
func (s *Service) PushNativeReleaseSource(ctx context.Context, projectID, instructions, repository, workspace, branch, source string) (string, error) {
	limits := executionpolicy.FromInstructions(instructions)
	if limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository || executionpolicy.StandaloneArtifactWork(instructions) || !git.IsCommitHash(source) {
		return "", errors.New("native source publication is not authorized")
	}
	p, err := s.TaskContext(ctx, projectID, instructions)
	if err != nil || p == nil || p.IsArchived {
		return "", errors.New("saved native source project unavailable")
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil || repository != owner+"/"+repo {
		return "", errors.New("native source repository differs from task")
	}
	settings := SettingsFromMetadata(p.Metadata)
	target := strings.TrimSpace(settings.DeployBranch)
	if target == "" {
		target = p.DefaultBranch
	}
	if !settings.IsAutoDeployEnabled() || ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" || branch == target {
		return "", errors.New("native source publication configuration unavailable")
	}
	current, err := s.store.GetProject(ctx, projectID)
	if err != nil || current == nil || current.IsArchived {
		return "", errors.New("saved native source connection unavailable")
	}
	token, err := s.repositoryToken(ctx, current.EncryptedToken)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", ErrGitHubConnectionRequired
	}
	return s.gitClient.PushExecutionSource(ctx, workspace, p.RepoURL, branch, p.DefaultBranch, source, token)
}
