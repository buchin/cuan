package project

import (
	"context"
	"errors"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
)

// ApplyNativeReleaseSource applies an exact admitted source to the task's frozen
// release branch. The caller must validate the task's original authorization and
// canonical source handoff before invoking this publication operation.
func (s *Service) ApplyNativeReleaseSource(ctx context.Context, projectID, instructions, repository, targetBranch, sourceCommit string) (*git.MergePullRequestResult, error) {
	limits := executionpolicy.FromInstructions(instructions)
	if s.gitClient == nil || projectID == "" || !git.IsCommitHash(sourceCommit) || limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository || executionpolicy.StandaloneArtifactWork(instructions) {
		return nil, errors.New("native release publication scope is unavailable")
	}
	p, err := s.TaskContext(ctx, projectID, instructions)
	if err != nil || p == nil || p.IsArchived {
		return nil, errors.New("saved release project context is unavailable")
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil || repository != owner+"/"+repo {
		return nil, errors.New("native release repository differs from saved task context")
	}
	settings := SettingsFromMetadata(p.Metadata)
	branch := strings.TrimSpace(settings.DeployBranch)
	if branch == "" {
		branch = p.DefaultBranch
	}
	if !settings.IsAutoDeployEnabled() || ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" || branch == "" || targetBranch != branch {
		return nil, errors.New("native release target differs from saved task context")
	}
	registered, err := s.store.GetProject(ctx, projectID)
	if err != nil || registered == nil || registered.IsArchived {
		return nil, errors.New("saved repository connection unavailable")
	}
	token, err := s.repositoryToken(ctx, registered.EncryptedToken)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, ErrGitHubConnectionRequired
	}
	return s.gitClient.MergeBranch(ctx, git.MergeBranchRequest{Owner: owner, Repo: repo, BaseBranch: branch, SourceCommit: sourceCommit, Message: "Apply verified task source " + sourceCommit, Token: token})
}
