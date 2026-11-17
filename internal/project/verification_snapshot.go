package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
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
	destination, err := s.VerificationSnapshotPath(projectID, taskID, sourceCommit, targetCommit)
	if err != nil {
		return "", err
	}
	path, err := s.gitClient.PrepareVerificationSnapshot(ctx, p.LocalPath, destination, sourceCommit, targetCommit)
	if !errors.Is(err, git.ErrVerificationCommitUnavailable) {
		return path, err
	}
	if err := s.fetchVerificationTarget(ctx, projectID, targetCommit); err != nil {
		return "", err
	}
	path, err = s.gitClient.PrepareVerificationSnapshot(ctx, p.LocalPath, destination, sourceCommit, targetCommit)
	if errors.Is(err, git.ErrVerificationCommitUnavailable) {
		if errSource := s.fetchVerificationTarget(ctx, projectID, sourceCommit); errSource != nil {
			return "", errSource
		}
		path, err = s.gitClient.PrepareVerificationSnapshot(ctx, p.LocalPath, destination, sourceCommit, targetCommit)
	}
	return path, err
}

// Location is derived only from native project/task/revision identities. It
// neither creates a checkout nor needs access to the disposable source cache.
func (s *Service) VerificationSnapshotPath(projectID, taskID, source, target string) (string, error) {
	if projectID == "" || taskID == "" || source == "" || target == "" {
		return "", errors.New("snapshot identity unavailable")
	}
	digest := sha256.Sum256([]byte(projectID + "\x00" + taskID + "\x00" + source + "\x00" + target))
	return filepath.Abs(filepath.Join(s.baseDir, "verification-snapshots", hex.EncodeToString(digest[:])))
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
	fetchErr := s.gitClient.FetchVerificationCommit(fetch, registered.LocalPath, registered.RepoURL, targetCommit, token)
	if fetchErr == nil {
		return nil
	}

	// If remote fetch failed (e.g. unadvertised local commit on GitHub), search local workspace checkouts.
	searchDirs := []string{
		filepath.Join(s.baseDir, "task-workspaces"),
		filepath.Join(s.baseDir, "verification-snapshots"),
		filepath.Join(s.baseDir, "verification-checkouts"),
		filepath.Join(s.baseDir, "workspaces"),
		filepath.Join(s.baseDir, "workspaces", "verification"),
		filepath.Join(filepath.Dir(s.baseDir), "task-workspaces"),
		filepath.Join(filepath.Dir(s.baseDir), "verification-snapshots"),
		filepath.Join(filepath.Dir(s.baseDir), "verification-checkouts"),
		filepath.Join(filepath.Dir(s.baseDir), "workspaces"),
		filepath.Join(filepath.Dir(s.baseDir), "workspaces", "verification"),
	}
	for _, dir := range searchDirs {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(dir, entry.Name())
			if err := s.gitClient.FetchLocalCommit(ctx, registered.LocalPath, candidate, targetCommit); err == nil {
				return nil
			}
		}
	}
	return fetchErr
}

// PublishedCommitContained proves only immutable Git ancestry. The caller must
// independently observe target as the registered project's running release.
// No checkout, task workspace, repository refs or production state is changed.
func (s *Service) PublishedCommitContained(ctx context.Context, projectID, source, target string) (bool, error) {
	if s.gitClient == nil {
		return false, errors.New("publication repository is unavailable")
	}
	p, err := s.Get(ctx, projectID)
	if err != nil || p == nil || p.IsArchived || p.LocalPath == "" || p.RepoURL == "" {
		return false, errors.New("publication repository is unavailable")
	}
	contained, err := s.gitClient.CommitContained(ctx, p.LocalPath, source, target)
	if errors.Is(err, git.ErrVerificationCommitUnavailable) {
		// Fetch exact objects through this project's saved connection only.
		for _, commit := range []string{target, source} {
			if err = s.fetchVerificationTarget(ctx, projectID, commit); err != nil {
				return false, err
			}
			contained, err = s.gitClient.CommitContained(ctx, p.LocalPath, source, target)
			if !errors.Is(err, git.ErrVerificationCommitUnavailable) {
				break
			}
		}
	}
	fresh, readErr := s.Get(ctx, projectID)
	if readErr != nil || fresh == nil || fresh.IsArchived || fresh.LocalPath != p.LocalPath || fresh.RepoURL != p.RepoURL || string(fresh.Metadata) != string(p.Metadata) {
		return false, errors.New("publication project changed during verification")
	}
	return contained, err
}
