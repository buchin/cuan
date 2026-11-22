package project

import (
	"context"
	"errors"
	"path/filepath"
	"time"
)

// RefreshExecutionBase updates only the approved remote-tracking ref using the
// Cuan user's repository connection. It never checks out or resets source/task
// files, and does not borrow developer credentials from the server environment.
func (s *Service) RefreshExecutionBase(ctx context.Context, p *ProjectResponse) error {
	if p == nil || p.RepoURL == "" {
		return nil
	}
	if s.gitClient == nil || p.LocalPath == "" {
		return errors.New("Project repository access is unavailable")
	}
	registered, err := s.store.GetProject(ctx, p.ID)
	if err != nil {
		return errors.New("Project repository connection is unavailable")
	}
	token, err := s.repositoryToken(ctx, registered.EncryptedToken)
	if err != nil {
		return err
	}
	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return errors.New("Project repository is unavailable")
	}
	refresh, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.gitClient.FetchBoundBranch(refresh, source, p.RepoURL, p.DefaultBranch, token)
}

// PreflightSyncProject executes a pre-flight fetch and fast-forward of the project's
// default branch (default "dev") in its local path if clean.
func (s *Service) PreflightSyncProject(ctx context.Context, p *ProjectResponse) error {
	if p == nil || p.RepoURL == "" || p.LocalPath == "" {
		return nil
	}
	registered, err := s.store.GetProject(ctx, p.ID)
	if err != nil || registered == nil {
		return errors.New("Project repository connection is unavailable")
	}
	return s.PreflightSyncStoredProject(ctx, registered)
}

// PreflightSyncStoredProject executes a pre-flight fetch and fast-forward of the project's
// default branch (default "dev") in its local path using stored project data directly,
// avoiding extra round-trip database queries and response mapping.
func (s *Service) PreflightSyncStoredProject(ctx context.Context, p *Project) error {
	if p == nil || p.RepoURL == "" || p.LocalPath == "" || p.IsArchived {
		return nil
	}
	if s.gitClient == nil {
		return errors.New("Project repository access is unavailable")
	}
	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return errors.New("Project repository is unavailable")
	}
	token, _ := s.repositoryToken(ctx, p.EncryptedToken)
	branch := p.DefaultBranch
	if branch == "" {
		branch = "dev"
	}
	return s.gitClient.PreflightSync(ctx, source, branch, token)
}
