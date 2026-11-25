package project

import (
	"context"
	"errors"
	"github.com/buchin/cuan/internal/crypto"
)

// SetSharedRepositoryAccess installs the single owner's connection resolver.
// Credentials are read on each operation so rotation/disconnection takes effect.
func (s *Service) SetSharedRepositoryAccess(resolve func(context.Context) (string, error)) {
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	s.sharedAccess = resolve
}
func (s *Service) repositoryToken(ctx context.Context, legacy string) (string, error) {
	if legacy != "" {
		token, err := crypto.Decrypt(legacy, s.authSecret)
		if err != nil {
			return "", errors.New("Saved repository access could not be read.")
		}
		return token, nil
	}
	s.accessMu.RLock()
	resolve := s.sharedAccess
	s.accessMu.RUnlock()
	if resolve != nil {
		return resolve(ctx)
	}
	return "", nil
}

// RepositoryToken resolves the active GitHub token for a project, checking
// per-project encrypted credentials first and falling back to shared owner access.
func (s *Service) RepositoryToken(ctx context.Context, projectID string) (string, error) {
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
	return token, nil
}
