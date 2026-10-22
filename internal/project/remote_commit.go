package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

func (s *Service) ConfirmTaskRemoteHead(ctx context.Context, taskID string, p *ProjectResponse) (string, error) {
	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(taskID + "\x00" + source))
	branch := "cuan/task-" + hex.EncodeToString(digest[:])
	current, err := s.store.GetProject(ctx, p.ID)
	if err != nil {
		return "", err
	}
	token, err := s.repositoryToken(ctx, current.EncryptedToken)
	if err != nil {
		return "", err
	}
	return s.gitClient.RemoteBranchHead(ctx, p.RepoURL, branch, token)
}
