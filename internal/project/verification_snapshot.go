package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/buchin/cuan/internal/git"
)

// VerificationSnapshot uses the registered project's cache and connection.
// It never resumes or reconstructs the implementation task workspace.
func (s *Service) VerificationSnapshot(ctx context.Context, projectID, taskID, sourceCommit, targetCommit string) (string, error) {
	if projectID == "" || taskID == "" || s.gitClient == nil {
		return "", errors.New("verification project identity is unavailable")
	}
	p, err := s.Get(ctx, projectID)
	if err != nil || p == nil || p.RepoURL == "" || p.LocalPath == "" {
		return "", errors.New("verification project repository is unavailable")
	}
	digest := sha256.Sum256([]byte(projectID + "\x00" + taskID + "\x00" + sourceCommit + "\x00" + targetCommit))
	destination := filepath.Join(s.baseDir, "verification-snapshots", hex.EncodeToString(digest[:]))
	path, err := s.gitClient.PrepareVerificationSnapshot(ctx, p.LocalPath, destination, sourceCommit, targetCommit)
	if !errors.Is(err, git.ErrVerificationCommitUnavailable) {
		return path, err
	}
	registered, err := s.store.GetProject(ctx, projectID)
	if err != nil || registered == nil {
		return "", errors.New("project repository connection is unavailable")
	}
	token, err := s.repositoryToken(ctx, registered.EncryptedToken)
	if err != nil {
		return "", err
	}
	fetch, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.gitClient.FetchVerificationCommit(fetch, p.LocalPath, p.RepoURL, targetCommit, token); err != nil {
		return "", err
	}
	return s.gitClient.PrepareVerificationSnapshot(ctx, p.LocalPath, destination, sourceCommit, targetCommit)
}
