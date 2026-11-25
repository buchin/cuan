package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
)

var ErrGitHubConnectionRequired = errors.New("Connect GitHub in Settings to save this task branch")

// PushNativeTaskBranch uses the server-derived execution root and saved branch.
// It never prepares a second worktree or uses ambient developer credentials.
func (s *Service) PushNativeTaskBranch(ctx context.Context, taskID, projectID, instructions, workspace, branch string, publish bool, title string) (string, *git.PullRequestResult, error) {
	if executionpolicy.FromInstructions(instructions).DenyPush {
		return "", nil, errors.New("This task scope forbids pushing. Local work is preserved")
	}
	p, err := s.TaskContext(ctx, projectID, instructions)
	if err != nil {
		return "", nil, err
	}
	current, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return "", nil, err
	}
	token, err := s.repositoryToken(ctx, current.EncryptedToken)
	if err != nil {
		return "", nil, err
	}
	if token == "" {
		return "", nil, ErrGitHubConnectionRequired
	}
	commit, err := s.gitClient.PushExecutionBranch(ctx, workspace, p.RepoURL, branch, p.DefaultBranch, token)
	if errors.Is(err, git.ErrRemoteBranchDiverged) {
		// Baseline reconciliation may rebase a retained checkpoint. Save its
		// exact source with the existing immutable-ref protocol; never overwrite
		// the old task branch or create a PR referring to that older source.
		source, sourceErr := s.gitClient.GetLatestCommitSHA(ctx, workspace)
		if sourceErr != nil {
			return "", nil, sourceErr
		}
		commit, sourceErr = s.gitClient.PushExecutionSource(ctx, workspace, p.RepoURL, branch, p.DefaultBranch, source, token)
		return commit, nil, sourceErr
	}
	if err != nil || !publish {
		return commit, nil, err
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	if err != nil {
		return commit, nil, err
	}
	// A PR is optional publication metadata; failure must not misreport a
	// confirmed push. Strip frozen project/environment snapshots from its body.
	pr, _ := s.gitClient.CreateOrUpdatePullRequest(ctx, git.PullRequestRequest{
		Owner: owner, Repo: repo, HeadBranch: branch, BaseBranch: p.DefaultBranch,
		Title: fmt.Sprintf("Task %s: %s", taskID, title), Description: TaskScopeInstructions(instructions), Token: token,
	})
	return commit, pr, nil
}

func (s *Service) PushTaskBranch(ctx context.Context, taskID, projectID, instructions string) (string, error) {
	if executionpolicy.FromInstructions(instructions).DenyPush {
		return "", errors.New("This task scope forbids pushing. Local work is preserved")
	}
	p, err := s.TaskContext(ctx, projectID, instructions)
	if err != nil {
		return "", err
	}
	if p.RepoURL == "" {
		return "", errors.New("This task has no connected repository")
	}
	workspace, err := s.TaskExecutionWorkspace(ctx, taskID, instructions, p)
	if err != nil {
		return "", err
	}
	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(taskID + "\x00" + source))
	branch := "cuan/task-" + hex.EncodeToString(digest[:])
	current, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	token, err := s.repositoryToken(ctx, current.EncryptedToken)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", ErrGitHubConnectionRequired
	}
	return s.gitClient.PushBoundBranch(ctx, workspace, p.RepoURL, branch, token)
}

// PushProjectBranch pushes a non-default branch from the project's local workspace
// using the server-stored GitHub connection token.
func (s *Service) PushProjectBranch(ctx context.Context, projectID, branch string) (string, string, error) {
	current, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return "", "", err
	}
	if current == nil {
		return "", "", ErrProjectNotFound
	}
	if strings.TrimSpace(current.RepoURL) == "" {
		return "", "", errors.New("This project has no connected repository")
	}
	workDir := strings.TrimSpace(current.LocalPath)
	if workDir == "" {
		if wd, wdErr := s.ProjectTerminalWorkspace(ctx, projectID, current.ToResponse()); wdErr == nil {
			workDir = strings.TrimSpace(wd)
		}
	}
	if workDir == "" {
		return "", "", errors.New("This project has no local workspace")
	}
	token, err := s.repositoryToken(ctx, current.EncryptedToken)
	if err != nil {
		return "", "", err
	}
	if token == "" {
		return "", "", ErrGitHubConnectionRequired
	}
	targetBranch := strings.TrimSpace(branch)
	if targetBranch == "" {
		out, symErr := exec.CommandContext(ctx, "git", "-C", workDir, "symbolic-ref", "--short", "HEAD").Output()
		if symErr != nil {
			return "", "", errors.New("Unable to determine current branch in project workspace")
		}
		targetBranch = strings.TrimSpace(string(out))
	}
	commit, err := s.gitClient.PushExecutionBranch(ctx, workDir, current.RepoURL, targetBranch, current.DefaultBranch, token)
	if err != nil {
		return targetBranch, "", err
	}
	return targetBranch, commit, nil
}
