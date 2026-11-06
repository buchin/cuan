package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"

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
