package project

import (
	"context"
	"errors"
	"fmt"
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

// ReconcileNativeReleaseAncestry ensures that the target release commit provably
// contains the active running release before release activation. If diverged, it
// creates a clean in-memory merge of both parent lineages and publishes it to the target branch.
func (s *Service) ReconcileNativeReleaseAncestry(ctx context.Context, projectID, instructions, repository, targetBranch, runningCommit, targetCommit string) (string, error) {
	if s.gitClient == nil || projectID == "" || !git.IsCommitHash(runningCommit) || !git.IsCommitHash(targetCommit) {
		return "", errors.New("reconcile release ancestry identity unavailable")
	}
	if runningCommit == targetCommit {
		return targetCommit, nil
	}
	contained, err := s.PublishedCommitContained(ctx, projectID, runningCommit, targetCommit)
	if err == nil && contained {
		return targetCommit, nil
	}
	p, err := s.TaskContext(ctx, projectID, instructions)
	if err != nil || p == nil || p.IsArchived || p.LocalPath == "" {
		return "", errors.New("saved release project context unavailable")
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil || repository != owner+"/"+repo {
		return "", errors.New("reconcile release repository differs from task context")
	}
	registered, err := s.store.GetProject(ctx, projectID)
	if err != nil || registered == nil || registered.IsArchived {
		return "", errors.New("saved repository connection unavailable")
	}
	token, err := s.repositoryToken(ctx, registered.EncryptedToken)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", ErrGitHubConnectionRequired
	}

	lease, err := git.AcquireVerificationLease(ctx, p.LocalPath)
	if err != nil {
		return "", err
	}
	defer lease.Close()

	for _, c := range []string{targetCommit, runningCommit} {
		if err := s.gitClient.VerifySnapshotCommit(ctx, p.LocalPath, c, c); err != nil {
			_ = s.gitClient.FetchVerificationCommit(ctx, p.LocalPath, p.RepoURL, c, token)
		}
	}

	msg := fmt.Sprintf("Reconcile active release %s with verified release %s", runningCommit[:8], targetCommit[:8])
	reconciledSHA, err := s.gitClient.ReconcileCommits(ctx, p.LocalPath, targetCommit, runningCommit, msg)
	if err != nil {
		return "", err
	}
	if pushErr := s.gitClient.PushCommitToBranch(ctx, p.LocalPath, p.RepoURL, targetBranch, reconciledSHA, token); pushErr != nil {
		return "", pushErr
	}
	return reconciledSHA, nil
}
