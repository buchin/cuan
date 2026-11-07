package project

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/buchin/cuan/internal/git"
)

// ApplyTaskPullRequest merges the task PR into the project default branch.
func (s *Service) ApplyTaskPullRequest(ctx context.Context, projectID string, prNumber int, title, message, sourceCommit string) (*git.MergePullRequestResult, error) {
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	if prNumber <= 0 {
		return nil, errors.New("pull request number must be greater than 0")
	}

	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}
	if p.RepoOwner == "" || p.RepoName == "" {
		return nil, errors.New("project does not have a GitHub repository connected")
	}

	token, err := s.repositoryToken(ctx, p.EncryptedToken)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, ErrGitHubConnectionRequired
	}

	result, err := s.gitClient.MergePullRequest(ctx, git.MergePullRequestRequest{
		Owner:           p.RepoOwner,
		Repo:            p.RepoName,
		PullNumber:      prNumber,
		CommitTitle:     title,
		CommitMessage:   message,
		MergeMethod:     "squash",
		Token:           token,
		ExpectedHeadSHA: sourceCommit,
	})
	if err != nil {
		return nil, err
	}

	// If local repository path exists, fetch remote updates to keep workspace fresh
	if p.LocalPath != "" {
		_ = s.gitClient.CloneOrFetch(ctx, p.RepoOwner, p.RepoName, token, p.LocalPath)
	}

	return result, nil
}

// ApplyTaskBranch merges the task branch directly into the project's repository default branch.
func (s *Service) ApplyTaskBranch(ctx context.Context, projectID, branchName, title, message string) (*git.MergePullRequestResult, error) {
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	if branchName == "" {
		return nil, errors.New("branch name is required")
	}

	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	repoPath := p.LocalPath
	if repoPath == "" {
		return nil, errors.New("project local repository path is not available")
	}

	defaultBranch := p.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// Checkout default branch and merge the task branch
	_ = exec.CommandContext(ctx, "git", "-C", repoPath, "checkout", defaultBranch).Run()
	mergeMsg := fmt.Sprintf("%s\n\n%s", title, message)
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "merge", "--no-ff", "-m", mergeMsg, branchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fetching branch if not found locally
		_ = exec.CommandContext(ctx, "git", "-C", repoPath, "fetch", "origin", branchName).Run()
		cmdRetry := exec.CommandContext(ctx, "git", "-C", repoPath, "merge", "--no-ff", "-m", mergeMsg, branchName)
		outputRetry, errRetry := cmdRetry.CombinedOutput()
		if errRetry != nil {
			return nil, fmt.Errorf("git merge branch %s failed: %s (%w)", branchName, string(outputRetry), errRetry)
		}
	}
	_ = output

	revCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "HEAD")
	revOut, err := revCmd.Output()
	sha := ""
	if err == nil {
		sha = strings.TrimSpace(string(revOut))
	}

	return &git.MergePullRequestResult{
		Merged:  true,
		Message: "Task branch applied successfully",
		SHA:     sha,
	}, nil
}
