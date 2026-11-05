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
	if err := s.fetchVerificationTarget(ctx, projectID, targetCommit); err != nil {
		return "", err
	}
	return s.gitClient.PrepareVerificationSnapshot(ctx, p.LocalPath, destination, sourceCommit, targetCommit)
}

func (s *Service) VerificationPathsChanged(ctx context.Context, projectID, taskID, source, target string, paths []string) (bool, error) {
	if projectID == "" || taskID == "" || s.gitClient == nil {
		return false, errors.New("verification project identity is unavailable")
	}
	p, err := s.Get(ctx, projectID)
	if err != nil || p == nil || p.LocalPath == "" || p.RepoURL == "" {
		return false, errors.New("verification project repository is unavailable")
	}
	changed, err := s.gitClient.CommittedPathsChanged(ctx, p.LocalPath, source, target, paths)
	if !errors.Is(err, git.ErrVerificationCommitUnavailable) {
		return changed, err
	}
	if err := s.fetchVerificationTarget(ctx, projectID, target); err != nil {
		return false, err
	}
	return s.gitClient.CommittedPathsChanged(ctx, p.LocalPath, source, target, paths)
}

func (s *Service) fetchVerificationTarget(ctx context.Context, projectID, targetCommit string) error {
	registered, err := s.store.GetProject(ctx, projectID)
	if err != nil || registered == nil || registered.LocalPath == "" || registered.RepoURL == "" {
		return errors.New("project repository connection is unavailable")
	}
	token, err := s.repositoryToken(ctx, registered.EncryptedToken)
	if err != nil {
		return err
	}
	fetch, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.gitClient.FetchVerificationCommit(fetch, registered.LocalPath, registered.RepoURL, targetCommit, token)
}
