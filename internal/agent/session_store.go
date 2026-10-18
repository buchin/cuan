package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrSessionNotFound     = errors.New("agent session not found")
	ErrSessionExpired      = errors.New("agent session expired")
	ErrSessionRevoked      = errors.New("agent session revoked")
	ErrInvalidDeviceCode   = errors.New("invalid or expired device code")
	ErrEmptyMemberID       = errors.New("member id cannot be empty")
	ErrEmptySessionToken   = errors.New("session token cannot be empty")
)

const (
	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"
	SessionStatusExpired = "expired"

	DefaultSessionTTL = 30 * 24 * time.Hour // 30 days
	DeviceCodeTTL     = 15 * time.Minute
)

// AgentBinary represents a detected CLI agent or shell executable on the host system.
type AgentBinary struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
}

// DetectAgentBinaries inspects the server host environment for available agent CLIs and shells.
func DetectAgentBinaries() []AgentBinary {
	candidates := []struct {
		name        string
		binary      string
		versionArgs []string
	}{
		{name: "agy", binary: "agy", versionArgs: []string{"--version"}},
		{name: "claude", binary: "claude", versionArgs: []string{"--version"}},
		{name: "bash", binary: "bash", versionArgs: []string{"--version"}},
		{name: "sh", binary: "sh", versionArgs: []string{}},
	}

	results := make([]AgentBinary, 0, len(candidates))
	for _, c := range candidates {
		item := AgentBinary{Name: c.name}
		path, err := exec.LookPath(c.binary)
		if err == nil {
			item.Available = true
			item.Path = path
			if len(c.versionArgs) > 0 {
				cmd := exec.Command(path, c.versionArgs...)
				if out, errV := cmd.CombinedOutput(); errV == nil {
					firstLine := strings.Split(strings.TrimSpace(string(out)), "\n")[0]
					item.Version = strings.TrimSpace(firstLine)
				}
			}
		} else {
			item.Available = false
		}
		results = append(results, item)
	}

	return results
}

// AgentSession records an authenticated CLI or worker agent session for a team member.
type AgentSession struct {
	ID         string    `json:"id"`
	MemberID   string    `json:"member_id"`
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	Provider   string    `json:"provider"`
	BinaryType string    `json:"binary_type"`
	BinaryPath string    `json:"binary_path,omitempty"`
	DeviceCode string    `json:"device_code,omitempty"`
	Token      string    `json:"token,omitempty"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

// AuthorizePairingRequest contains parameters to link an agent CLI with a team member.
type AuthorizePairingRequest struct {
	DeviceCode string `json:"device_code"`
	MemberID   string `json:"member_id"`
	AgentID    string `json:"agent_id"`
	AgentName  string `json:"agent_name"`
	Provider   string `json:"provider,omitempty"`
	BinaryType string `json:"binary_type,omitempty"`
}

// SessionStore manages persistence and validation for self-hosted agent sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, s *AgentSession) error
	GetSession(ctx context.Context, id string) (*AgentSession, error)
	GetSessionByToken(ctx context.Context, token string) (*AgentSession, error)
	GetSessionByDeviceCode(ctx context.Context, code string) (*AgentSession, error)
	ListSessions(ctx context.Context, memberID string) ([]*AgentSession, error)
	ValidateSession(ctx context.Context, token string) (*AgentSession, error)
	RevokeSession(ctx context.Context, id string) error
	AuthorizeDevicePairing(ctx context.Context, req AuthorizePairingRequest) (*AgentSession, error)
}

// MemorySessionStore provides a thread-safe in-memory session store.
type MemorySessionStore struct {
	mu           sync.RWMutex
	sessionsByID map[string]*AgentSession
	tokenIndex   map[string]string // token -> sessionID
	codeIndex    map[string]string // deviceCode -> sessionID
}

// NewMemorySessionStore creates a new in-memory agent session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessionsByID: make(map[string]*AgentSession),
		tokenIndex:   make(map[string]string),
		codeIndex:    make(map[string]string),
	}
}

func (s *MemorySessionStore) CreateSession(ctx context.Context, sess *AgentSession) error {
	if sess == nil {
		return errors.New("session cannot be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess.ID == "" {
		sess.ID = generateHexID("asess")
	}
	if sess.Token == "" {
		sess.Token = generateToken("agt")
	}
	if sess.Status == "" {
		sess.Status = SessionStatusActive
	}
	now := time.Now().UTC()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	if sess.LastUsedAt.IsZero() {
		sess.LastUsedAt = now
	}
	if sess.ExpiresAt.IsZero() {
		sess.ExpiresAt = now.Add(DefaultSessionTTL)
	}

	clone := *sess
	s.sessionsByID[sess.ID] = &clone
	s.tokenIndex[sess.Token] = sess.ID
	if sess.DeviceCode != "" {
		s.codeIndex[sess.DeviceCode] = sess.ID
	}

	return nil
}

func (s *MemorySessionStore) GetSession(ctx context.Context, id string) (*AgentSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, exists := s.sessionsByID[id]
	if !exists {
		return nil, ErrSessionNotFound
	}
	clone := *sess
	return &clone, nil
}

func (s *MemorySessionStore) GetSessionByToken(ctx context.Context, token string) (*AgentSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.tokenIndex[token]
	if !exists {
		return nil, ErrSessionNotFound
	}
	sess, exists := s.sessionsByID[id]
	if !exists {
		return nil, ErrSessionNotFound
	}
	clone := *sess
	return &clone, nil
}

func (s *MemorySessionStore) GetSessionByDeviceCode(ctx context.Context, code string) (*AgentSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.codeIndex[code]
	if !exists {
		return nil, ErrInvalidDeviceCode
	}
	sess, exists := s.sessionsByID[id]
	if !exists {
		return nil, ErrInvalidDeviceCode
	}
	clone := *sess
	return &clone, nil
}

func (s *MemorySessionStore) ListSessions(ctx context.Context, memberID string) ([]*AgentSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*AgentSession
	for _, sess := range s.sessionsByID {
		if memberID == "" || sess.MemberID == memberID {
			clone := *sess
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (s *MemorySessionStore) ValidateSession(ctx context.Context, token string) (*AgentSession, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrEmptySessionToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, exists := s.tokenIndex[token]
	if !exists {
		return nil, ErrSessionNotFound
	}

	sess, exists := s.sessionsByID[id]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if sess.Status == SessionStatusRevoked {
		return nil, ErrSessionRevoked
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		sess.Status = SessionStatusExpired
		return nil, ErrSessionExpired
	}

	sess.LastUsedAt = time.Now().UTC()
	clone := *sess
	return &clone, nil
}

func (s *MemorySessionStore) RevokeSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, exists := s.sessionsByID[id]
	if !exists {
		return ErrSessionNotFound
	}

	sess.Status = SessionStatusRevoked
	return nil
}

func (s *MemorySessionStore) AuthorizeDevicePairing(ctx context.Context, req AuthorizePairingRequest) (*AgentSession, error) {
	code := strings.TrimSpace(req.DeviceCode)
	if code == "" {
		return nil, ErrInvalidDeviceCode
	}
	if strings.TrimSpace(req.MemberID) == "" {
		return nil, ErrEmptyMemberID
	}

	binType := strings.ToLower(strings.TrimSpace(req.BinaryType))
	if binType == "" {
		binType = "agy"
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "antigravity"
	}

	binPath := ""
	if p, err := exec.LookPath(binType); err == nil {
		binPath = p
	}

	agentName := req.AgentName
	if agentName == "" {
		agentName = fmt.Sprintf("Agent (%s)", binType)
	}

	now := time.Now().UTC()
	sess := &AgentSession{
		ID:         generateHexID("asess"),
		MemberID:   req.MemberID,
		AgentID:    req.AgentID,
		AgentName:  agentName,
		Provider:   provider,
		BinaryType: binType,
		BinaryPath: binPath,
		DeviceCode: code,
		Token:      generateToken("agt"),
		Status:     SessionStatusActive,
		CreatedAt:  now,
		ExpiresAt:  now.Add(DefaultSessionTTL),
		LastUsedAt: now,
	}

	if err := s.CreateSession(ctx, sess); err != nil {
		return nil, err
	}

	return sess, nil
}

func generateHexID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func generateToken(prefix string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
