package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/buchin/cuan/internal/executionpolicy"
	"path/filepath"
)

var ErrGitHubConnectionRequired = errors.New("Connect GitHub in Settings to save this task branch")

func (s *Service) PushTaskBranch(ctx context.Context, taskID, projectID, instructions string) (string, error) {
	if executionpolicy.FromInstructions(instructions).DenyPush {
		return "", errors.New("This task scope forbids pushing. Local work is preserved")
	}
	p, err := s.TaskWorkspace(ctx, projectID, instructions)
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
