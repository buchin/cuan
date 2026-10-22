package project

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/buchin/cuan/internal/git"
)

// ApplyTaskPullRequest merges the task PR into the project default branch.
func (s *Service) ApplyTaskPullRequest(ctx context.Context, projectID string, prNumber int, title, message string) (*git.MergePullRequestResult, error) {
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

	token, _ := s.repositoryToken(ctx, p.EncryptedToken)
	if token == "" {
		token = os.Getenv("GH_TOKEN")
		if token == "" {
			token = os.Getenv("GITHUB_TOKEN")
		}
	}

	result, err := s.gitClient.MergePullRequest(ctx, git.MergePullRequestRequest{
		Owner:         p.RepoOwner,
		Repo:          p.RepoName,
		PullNumber:    prNumber,
		CommitTitle:   title,
		CommitMessage: message,
		MergeMethod:   "squash",
		Token:         token,
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
