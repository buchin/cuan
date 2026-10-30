package auth

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu          sync.RWMutex
	owner       *Owner
	owners      map[string]*Owner
	sessions    map[string]*Session
	invitations map[string]*UserInvitation
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		owners:      make(map[string]*Owner),
		sessions:    make(map[string]*Session),
		invitations: make(map[string]*UserInvitation),
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
	if len(s.owners) > 0 {
		return len(s.owners), nil
	}
	if s.owner != nil {
		return 1, nil
	}
	return 0, nil
}

func (s *MemoryStore) CreateOwner(ctx context.Context, owner *Owner) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.owner != nil || len(s.owners) > 0 {
		return ErrSetupAlreadyCompleted
	}
	if owner.Role == "" {
		owner.Role = RoleOwner
	}
	owner.IsActive = true
	ownerCopy := *owner
	s.owner = &ownerCopy
	s.owners[owner.ID] = &ownerCopy
	return nil
}

func (s *MemoryStore) CreateUser(ctx context.Context, user *Owner) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.owners {
		if strings.EqualFold(o.Email, user.Email) {
			return ErrUserAlreadyExists
		}
		if user.Username != "" && strings.EqualFold(o.Username, user.Username) {
			return ErrUserAlreadyExists
		}
	}
	if user.Role == "" {
		user.Role = RoleMember
	}
	user.IsActive = true
	userCopy := *user
	s.owners[user.ID] = &userCopy
	if s.owner == nil {
		s.owner = &userCopy
	}
	return nil
}

func (s *MemoryStore) GetOwner(ctx context.Context) (*Owner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.owner != nil {
		ownerCopy := *s.owner
		return &ownerCopy, nil
	}
	// Fallback to first owner in owners map
	for _, o := range s.owners {
		if o.Role == RoleOwner {
			ownerCopy := *o
			return &ownerCopy, nil
		}
	}
	if len(s.owners) > 0 {
		for _, o := range s.owners {
			ownerCopy := *o
			return &ownerCopy, nil
		}
	}
	return nil, ErrOwnerNotFound
}

func (s *MemoryStore) GetOwnerByID(ctx context.Context, id string) (*Owner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, exists := s.owners[id]
	if !exists {
		if s.owner != nil && s.owner.ID == id {
			cp := *s.owner
			return &cp, nil
		}
		return nil, ErrOwnerNotFound
	}
	cp := *o
	return &cp, nil
}

func (s *MemoryStore) GetOwnerByEmailOrUsername(ctx context.Context, identifier string) (*Owner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	identifier = strings.TrimSpace(identifier)
	for _, o := range s.owners {
		if strings.EqualFold(o.Email, identifier) || (o.Username != "" && strings.EqualFold(o.Username, identifier)) {
			cp := *o
			return &cp, nil
		}
	}
	if s.owner != nil {
		if strings.EqualFold(s.owner.Email, identifier) || (s.owner.Username != "" && strings.EqualFold(s.owner.Username, identifier)) {
			cp := *s.owner
			return &cp, nil
		}
	}
	return nil, ErrOwnerNotFound
}

func (s *MemoryStore) ListOwners(ctx context.Context) ([]*Owner, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Owner, 0, len(s.owners))
	for _, o := range s.owners {
		cp := *o
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

func (s *MemoryStore) DeleteOwner(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.owners, id)
	if s.owner != nil && s.owner.ID == id {
		s.owner = nil
		for _, o := range s.owners {
			if o.Role == RoleOwner {
				s.owner = o
				break
			}
		}
	}
	for hash, sess := range s.sessions {
		if sess.OwnerID == id {
			delete(s.sessions, hash)
		}
	}
	return nil
}

func (s *MemoryStore) UpdateOwnerPassword(ctx context.Context, ownerID string, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, exists := s.owners[ownerID]
	if !exists {
		if s.owner == nil || s.owner.ID != ownerID {
			return ErrOwnerNotFound
		}
		o = s.owner
	}
	o.PasswordHash = passwordHash
	o.UpdatedAt = time.Now().UTC()
	if s.owner != nil && s.owner.ID == ownerID {
		s.owner.PasswordHash = passwordHash
		s.owner.UpdatedAt = o.UpdatedAt
	}
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

	owner, exists := s.owners[sess.OwnerID]
	if !exists {
		if s.owner != nil && s.owner.ID == sess.OwnerID {
			owner = s.owner
		} else {
			return nil, nil, ErrUnauthorized
		}
	}

	sessCopy := *sess
	ownerCopy := *owner
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

func (s *MemoryStore) CreateInvitation(ctx context.Context, inv *UserInvitation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *inv
	s.invitations[inv.ID] = &cp
	return nil
}

func (s *MemoryStore) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*UserInvitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inv := range s.invitations {
		if inv.TokenHash == tokenHash {
			cp := *inv
			return &cp, nil
		}
	}
	return nil, ErrInvitationNotFound
}

func (s *MemoryStore) GetInvitationByID(ctx context.Context, id string) (*UserInvitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, exists := s.invitations[id]
	if !exists {
		return nil, ErrInvitationNotFound
	}
	cp := *inv
	return &cp, nil
}

func (s *MemoryStore) ListInvitations(ctx context.Context) ([]*UserInvitation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*UserInvitation, 0, len(s.invitations))
	for _, inv := range s.invitations {
		cp := *inv
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *MemoryStore) AcceptInvitation(ctx context.Context, id string, acceptedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, exists := s.invitations[id]
	if !exists {
		return ErrInvitationNotFound
	}
	inv.AcceptedAt = &acceptedAt
	return nil
}

func (s *MemoryStore) DeleteInvitation(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.invitations, id)
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
		s.owners = map[string]*Owner{owner.ID: &cp}
	} else {
		s.owner = nil
		s.owners = make(map[string]*Owner)
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
	s.owners = make(map[string]*Owner)
	s.sessions = make(map[string]*Session)
	s.invitations = make(map[string]*UserInvitation)
}

func (s *MemoryStore) UpdateOwnerUsername(ctx context.Context, id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, exists := s.owners[id]
	if !exists {
		if s.owner == nil || s.owner.ID != id {
			return ErrOwnerNotFound
		}
		o = s.owner
	}
	o.Username = name
	o.UpdatedAt = time.Now().UTC()
	if s.owner != nil && s.owner.ID == id {
		s.owner.Username = name
		s.owner.UpdatedAt = o.UpdatedAt
	}
	return nil
}

func (s *MemoryStore) UpdateOwnerEmail(ctx context.Context, id, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, exists := s.owners[id]
	if !exists {
		if s.owner == nil || s.owner.ID != id {
			return ErrOwnerNotFound
		}
		o = s.owner
	}
	o.Email = email
	o.UpdatedAt = time.Now().UTC()
	if s.owner != nil && s.owner.ID == id {
		s.owner.Email = email
		s.owner.UpdatedAt = o.UpdatedAt
	}
	return nil
}
