package auth

import (
	"context"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	owner    *Owner
	sessions map[string]*Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
	}
}

func (s *MemoryStore) IsSetup(ctx context.Context) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owner != nil && s.owner.IsSetup, nil
}

func (s *MemoryStore) CountOwners(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.owner != nil {
		return 1, nil
	}
	return 0, nil
}

func (s *MemoryStore) CreateOwner(ctx context.Context, owner *Owner) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != nil {
		return ErrSetupAlreadyCompleted
	}
	ownerCopy := *owner
	s.owner = &ownerCopy
	return nil
}

func (s *MemoryStore) GetOwner(ctx context.Context) (*Owner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.owner == nil {
		return nil, ErrOwnerNotFound
	}
	ownerCopy := *s.owner
	return &ownerCopy, nil
}

func (s *MemoryStore) UpdateOwnerPassword(ctx context.Context, ownerID string, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner == nil || s.owner.ID != ownerID {
		return ErrOwnerNotFound
	}
	s.owner.PasswordHash = passwordHash
	s.owner.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryStore) CreateSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessCopy := *session
	s.sessions[session.TokenHash] = &sessCopy
	return nil
}

func (s *MemoryStore) GetSession(ctx context.Context, tokenHash string) (*Session, *Owner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessions[tokenHash]
	if !exists {
		return nil, nil, ErrUnauthorized
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, nil, ErrUnauthorized
	}

	if s.owner == nil || s.owner.ID != sess.OwnerID {
		return nil, nil, ErrUnauthorized
	}

	sessCopy := *sess
	ownerCopy := *s.owner
	return &sessCopy, &ownerCopy, nil
}

func (s *MemoryStore) DeleteSession(ctx context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenHash)
	return nil
}

func (s *MemoryStore) DeleteExpiredSessions(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for hash, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, hash)
		}
	}
	return nil
}

func (s *MemoryStore) Snapshot() (*Owner, map[string]*Session) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ownerCp *Owner
	if s.owner != nil {
		cp := *s.owner
		ownerCp = &cp
	}
	sessionsCp := make(map[string]*Session, len(s.sessions))
	for k, v := range s.sessions {
		cp := *v
		sessionsCp[k] = &cp
	}
	return ownerCp, sessionsCp
}

func (s *MemoryStore) Restore(owner *Owner, sessions map[string]*Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner != nil {
		cp := *owner
		s.owner = &cp
	} else {
		s.owner = nil
	}
	s.sessions = make(map[string]*Session, len(sessions))
	for k, v := range sessions {
		cp := *v
		s.sessions[k] = &cp
	}
}

func (s *MemoryStore) Wipe() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owner = nil
	s.sessions = make(map[string]*Session)
}

func (s *MemoryStore) UpdateOwnerUsername(ctx context.Context, id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner == nil || s.owner.ID != id {
		return ErrOwnerNotFound
	}
	s.owner.Username = name
	s.owner.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *MemoryStore) UpdateOwnerEmail(ctx context.Context, id, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner == nil || s.owner.ID != id {
		return ErrOwnerNotFound
	}
	s.owner.Email = email
	s.owner.UpdatedAt = time.Now().UTC()
	return nil
}
