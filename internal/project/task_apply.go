package project

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/buchin/cuan/internal/git"
)

// ApplyTaskPullRequest merges the task PR into the project default branch.
func (s *Service) ApplyTaskPullRequest(ctx context.Context, projectID string, prNumber int, title, message, sourceCommit string) (*git.MergePullRequestResult, error) {
	if s.gitClient == nil {
		return nil, errors.New("repository operations are unavailable")
	}
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
func (s *Service) ApplyTaskBranch(ctx context.Context, projectID, branchName, title, message, sourceCommit string) (*git.MergePullRequestResult, error) {
	if s.gitClient == nil {
		return nil, errors.New("repository operations are unavailable")
	}
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

	defaultBranch := p.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	mergeMsg := fmt.Sprintf("%s\n\n%s", title, message)
	if p.RepoURL != "" || p.RepoOwner != "" || p.RepoName != "" {
		owner, repo := p.RepoOwner, p.RepoName
		if p.RepoURL != "" {
			owner, repo, err = git.ParseGitHubURL(p.RepoURL)
			if err != nil {
				return nil, err
			}
		}
		if owner == "" || repo == "" {
			return nil, errors.New("registered repository identity is incomplete")
		}
		token, err := s.repositoryToken(ctx, p.EncryptedToken)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(token) != "" {
			res, err := s.gitClient.MergeBranch(ctx, git.MergeBranchRequest{Owner: owner, Repo: repo, BaseBranch: defaultBranch, SourceCommit: sourceCommit, Message: mergeMsg, Token: token})
			if err == nil && res != nil {
				if p.LocalPath != "" {
					_ = s.gitClient.CloneOrFetch(ctx, owner, repo, token, p.LocalPath)
				}
				return res, nil
			}
		}
		if p.LocalPath != "" {
			localRes, localErr := s.gitClient.MergeLocalBranch(ctx, p.LocalPath, defaultBranch, branchName, sourceCommit, mergeMsg)
			if localErr != nil {
				return nil, localErr
			}
			if p.RepoURL != "" && strings.TrimSpace(token) != "" {
				_ = s.gitClient.PushCommitToBranch(ctx, p.LocalPath, p.RepoURL, defaultBranch, localRes.SHA, token)
			}
			return localRes, nil
		}
		if strings.TrimSpace(token) == "" {
			return nil, ErrGitHubConnectionRequired
		}
		return nil, err
	}
	if p.LocalPath == "" {
		return nil, errors.New("project local repository path is not available")
	}
	return s.gitClient.MergeLocalBranch(ctx, p.LocalPath, defaultBranch, branchName, sourceCommit, mergeMsg)
}
