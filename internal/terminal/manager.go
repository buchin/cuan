package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidTaskID    = errors.New("invalid task id: contains forbidden characters or path traversal elements")
	ErrSessionNotFound  = errors.New("terminal session not found")
)

var validTaskIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// ManagerConfig defines configuration parameters for the terminal session manager.
type ManagerConfig struct {
	WorkspacesDir         string
	DefaultShell          string
	DefaultCols           uint16
	DefaultRows           uint16
	IdleTimeout           time.Duration
	DisconnectGracePeriod time.Duration
	WorkspaceResolver     func(taskID string) (string, error)
}

// Manager coordinates pseudo-terminal processes, workspaces, and lifecycle.
type Manager struct {
	cfg       ManagerConfig
	mu        sync.RWMutex
	sessions  map[string]*Session // taskID -> Session
	stopChan  chan struct{}
	stopOnce  sync.Once
}

// NewManager creates a new pseudo-terminal session manager.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.WorkspacesDir == "" {
		cfg.WorkspacesDir = filepath.Join("data", "workspaces")
	}
	if cfg.DefaultCols == 0 {
		cfg.DefaultCols = 80
	}
	if cfg.DefaultRows == 0 {
		cfg.DefaultRows = 24
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 15 * time.Minute
	}
	if cfg.DisconnectGracePeriod <= 0 {
		cfg.DisconnectGracePeriod = 30 * time.Second
	}

	m := &Manager{
		cfg:      cfg,
		sessions: make(map[string]*Session),
		stopChan: make(chan struct{}),
	}

	// Background sweeper for idle sessions
	go m.idleSweeper()

	return m
}

// ResolveTaskWorkspace returns the verified canonical workspace directory for a task.
// It ensures that path traversal attempts are rejected and locks the directory under WorkspacesDir.
func (m *Manager) ResolveTaskWorkspace(taskID string) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", ErrInvalidTaskID
	}

	// Security guardrail: strict task ID whitelist
	if !validTaskIDRegex.MatchString(taskID) {
		return "", fmt.Errorf("%w: %q", ErrInvalidTaskID, taskID)
	}

	if m.cfg.WorkspaceResolver != nil {
		customDir, err := m.cfg.WorkspaceResolver(taskID)
		if err == nil && customDir != "" {
			cleanCustom := filepath.Clean(customDir)
			if err := os.MkdirAll(cleanCustom, 0755); err != nil {
				return "", fmt.Errorf("create custom workspace directory: %w", err)
			}
			return cleanCustom, nil
		}
	}

	baseDir, err := filepath.Abs(m.cfg.WorkspacesDir)
	if err != nil {
		return "", fmt.Errorf("resolve base workspaces dir: %w", err)
	}

	targetDir := filepath.Join(baseDir, taskID)
	cleanTarget := filepath.Clean(targetDir)

	// Ensure target is strictly inside baseDir
	rel, err := filepath.Rel(baseDir, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return "", fmt.Errorf("%w: path escape detected %q", ErrInvalidWorkDir, targetDir)
	}

	if err := os.MkdirAll(cleanTarget, 0755); err != nil {
		return "", fmt.Errorf("create task workspace directory: %w", err)
	}

	return cleanTarget, nil
}

// GetOrCreateSession retrieves an active session for the task, or spawns a new one.
func (m *Manager) GetOrCreateSession(ctx context.Context, taskID string, mode Mode, cols, rows uint16) (*Session, error) {
	workDir, err := m.ResolveTaskWorkspace(taskID)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Check existing session
	if sess, exists := m.sessions[taskID]; exists && !sess.IsClosed() {
		return sess, nil
	}

	// 2. Prepare new session
	if cols == 0 {
		cols = m.cfg.DefaultCols
	}
	if rows == 0 {
		rows = m.cfg.DefaultRows
	}

	sessionID := generateSessionID()
	ptyCfg := PTYConfig{
		TaskID:      taskID,
		WorkDir:     workDir,
		Shell:       m.cfg.DefaultShell,
		InitialCols: cols,
		InitialRows: rows,
	}

	proc, err := SpawnPTY(ctx, ptyCfg)
	if err != nil {
		return nil, fmt.Errorf("spawn pty for task %s: %w", taskID, err)
	}

	sess := NewSession(sessionID, taskID, proc, mode)

	// Auto-cleanup hook when subscribers reach 0
	sess.SetOnEmpty(func(s *Session) {
		time.AfterFunc(m.cfg.DisconnectGracePeriod, func() {
			if s.SubscriberCount() == 0 && !s.IsClosed() {
				_ = m.CloseSession(s.TaskID())
			}
		})
	})

	m.sessions[taskID] = sess
	return sess, nil
}

// GetSession returns the active session for a task if present.
func (m *Manager) GetSession(taskID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, exists := m.sessions[taskID]
	if !exists || sess.IsClosed() {
		return nil, false
	}
	return sess, true
}

// CloseSession terminates and removes a task's terminal session.
func (m *Manager) CloseSession(taskID string) error {
	m.mu.Lock()
	sess, exists := m.sessions[taskID]
	if exists {
		delete(m.sessions, taskID)
	}
	m.mu.Unlock()

	if !exists {
		return nil
	}
	return sess.Close()
}

// ActiveSessions returns a snapshot list of running sessions.
func (m *Manager) ActiveSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if !s.IsClosed() {
			list = append(list, s)
		}
	}
	return list
}

// CloseAll terminates all active terminal sessions.
func (m *Manager) CloseAll() error {
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})

	m.mu.Lock()
	all := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	var lastErr error
	for _, s := range all {
		if err := s.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// CleanIdleSessions sweeps sessions that have been idle longer than the specified duration.
func (m *Manager) CleanIdleSessions(maxIdle time.Duration) int {
	if maxIdle <= 0 {
		maxIdle = m.cfg.IdleTimeout
	}

	threshold := time.Now().UTC().Add(-maxIdle)
	var toClose []string

	m.mu.RLock()
	for taskID, s := range m.sessions {
		if s.IsClosed() || (s.SubscriberCount() == 0 && s.LastActivity().Before(threshold)) {
			toClose = append(toClose, taskID)
		}
	}
	m.mu.RUnlock()

	cleaned := 0
	for _, id := range toClose {
		_ = m.CloseSession(id)
		cleaned++
	}
	return cleaned
}

// idleSweeper runs a periodic background ticker cleaning up idle or terminated sessions.
func (m *Manager) idleSweeper() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.CleanIdleSessions(m.cfg.IdleTimeout)
		}
	}
}

func generateSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "pty_" + hex.EncodeToString(b)
}
