package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidTaskID   = errors.New("invalid task id: contains forbidden characters or path traversal elements")
	ErrSessionNotFound = errors.New("terminal session not found")
)

var validTaskIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// TaskRunHistory records the metadata and outcome of an agent execution on a task.
type TaskRunHistory struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"task_id"`
	AttemptID  string    `json:"attempt_id,omitempty"`
	SessionID  string    `json:"session_id"`
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	EndReason  string    `json:"end_reason,omitempty"`
	Status     string    `json:"status"` // "running", "completed", "failed"
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	DurationMs int64     `json:"duration_ms"`
}

// TaskAgentRunConfig specifies parameters for launching an agent process in a task workspace.
type TaskAgentRunConfig struct {
	AttachmentPaths []string
	AttemptID       string
	TaskID          string
	AgentID         string
	AgentName       string
	Command         string // "agy", "claude", "shell", or auto
	Args            []string
	Prompt          string
	WorkDir         string
	Env             []string
	Cols            uint16
	Rows            uint16
	Mode            Mode
	OnOutput        func([]byte)
	OnExit          func(history *TaskRunHistory)
}

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
	sessions  map[string]*Session          // taskID -> Session
	histories map[string][]*TaskRunHistory // taskID -> []History
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
		cfg:       cfg,
		sessions:  make(map[string]*Session),
		histories: make(map[string][]*TaskRunHistory),
		stopChan:  make(chan struct{}),
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
		runs := m.histories[taskID]
		activeAgent := len(runs) > 0 && runs[len(runs)-1].Status == "running"
		if s.IsClosed() || (!activeAgent && s.SubscriberCount() == 0 && s.LastActivity().Before(threshold)) {
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

// RunTaskAgent spawns an autonomous or interactive agent process bound to the task's workspace in a real PTY.
func (m *Manager) RunTaskAgent(ctx context.Context, cfg TaskAgentRunConfig) (*Session, error) {
	taskID := strings.TrimSpace(cfg.TaskID)
	if taskID == "" {
		return nil, ErrInvalidTaskID
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		resolved, err := m.ResolveTaskWorkspace(taskID)
		if err != nil {
			return nil, err
		}
		workDir = resolved
	}

	cols := cfg.Cols
	if cols == 0 {
		cols = m.cfg.DefaultCols
	}
	rows := cfg.Rows
	if rows == 0 {
		rows = m.cfg.DefaultRows
	}

	mode := cfg.Mode
	if mode == "" {
		mode = ModeInteractive
	}

	// Pass prompts directly as argv; never interpolate task input into a shell script.
	agentID := cfg.AgentID
	agentName := cfg.AgentName
	commandLabel := strings.ToLower(strings.TrimSpace(cfg.Command))
	if commandLabel == "" || commandLabel == "antigravity" {
		commandLabel = "agy"
	}
	var executable string
	var ptyArgs []string
	var err error
	switch commandLabel {
	case "agy", "claude":
		executable, err = exec.LookPath(commandLabel)
		ptyArgs = []string{cfg.Prompt}
		if commandLabel == "agy" {
			// Task execution is explicitly authorized by Start/Create. Keep the
			// interactive conversation alive between turns, as in Fural.
			// Inbox uses its own restricted launcher and does not inherit this flag.
			ptyArgs = []string{"--dangerously-skip-permissions", "--prompt-interactive", cfg.Prompt, "--add-dir", workDir}
			cfg.Env = append(cfg.Env, "SSH_CONNECTION=127.0.0.1 22 127.0.0.1 22", "SSH_CLIENT=127.0.0.1 22 22", "SSH_TTY=/dev/pts/0")
		}
	case "codex":
		executable, err = exec.LookPath("codex")
		ptyArgs = []string{"--full-auto", cfg.Prompt}
	case "shell":
		executable = FindShell(m.cfg.DefaultShell)
		ptyArgs = []string{"-i"}
	default:
		return nil, fmt.Errorf("unsupported task harness %q", commandLabel)
	}
	for _, file := range cfg.AttachmentPaths {
		if commandLabel == "agy" || commandLabel == "claude" {
			ptyArgs = append(ptyArgs, "--add-dir", filepath.Dir(file))
		}
		if commandLabel == "codex" {
			ext := strings.ToLower(filepath.Ext(file))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" {
				ptyArgs = append(ptyArgs, "--image", file)
			}
		}
	}
	if err != nil || executable == "" {
		return nil, fmt.Errorf("task harness %s is not installed", commandLabel)
	}

	m.mu.Lock()
	// A watch shell can be replaced, but never launch a duplicate active agent.
	if runs := m.histories[taskID]; len(runs) > 0 && runs[len(runs)-1].Status == "running" {
		m.mu.Unlock()
		return nil, fmt.Errorf("agent already running for this task")
	}
	// Replace any idle terminal session with the requested agent.
	if existing, exists := m.sessions[taskID]; exists && !existing.IsClosed() {
		_ = existing.Close()
		delete(m.sessions, taskID)
	}
	// Keep launch reservation until process and history are installed.

	sessionID := generateSessionID()
	ptyCfg := PTYConfig{
		TaskID:      taskID,
		WorkDir:     workDir,
		Shell:       executable,
		Args:        ptyArgs,
		Env:         cfg.Env,
		InitialCols: cols,
		InitialRows: rows,
	}

	// A task outlives the HTTP request that starts it, with a bounded lifetime.
	runContext, cancel := context.WithTimeout(context.Background(), 8*time.Hour)
	proc, err := SpawnPTY(runContext, ptyCfg)
	if err != nil {
		cancel()
		m.mu.Unlock()
		return nil, fmt.Errorf("spawn agent pty for task %s: %w", taskID, err)
	}

	sess := NewSession(sessionID, taskID, proc, mode, cfg.OnOutput)

	history := &TaskRunHistory{
		ID:        generateSessionID(),
		AttemptID: cfg.AttemptID,
		TaskID:    taskID,
		SessionID: sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		Command:   commandLabel,
		Status:    "running",
		StartedAt: time.Now().UTC(),
	}

	m.sessions[taskID] = sess
	m.histories[taskID] = append(m.histories[taskID], history)
	m.mu.Unlock()

	// Monitor completion in background to record exit code and duration
	go func() {
		defer cancel()
		_ = proc.Wait()
		select {
		case <-sess.readerDone:
		case <-time.After(3 * time.Second):
		}
		exitCode := proc.ExitCode()
		m.mu.Lock()
		history.FinishedAt = time.Now().UTC()
		history.DurationMs = history.FinishedAt.Sub(history.StartedAt).Milliseconds()
		history.ExitCode = exitCode
		if exitCode == 0 || history.EndReason == "agent_report" {
			history.Status = "completed"
		} else {
			history.Status = "failed"
		}

		snapshot := *history
		m.mu.Unlock()
		if cfg.OnExit != nil {
			cfg.OnExit(&snapshot)
		}
	}()

	return sess, nil
}

// FinishReportedSession closes only the exact active attempt that submitted a
// validated final report. The actual signal exit code remains in history.
func (m *Manager) FinishReportedSession(taskID, attemptID string, acceptedAt time.Time) bool {
	m.mu.Lock()
	runs := m.histories[taskID]
	sess := m.sessions[taskID]
	if len(runs) == 0 || sess == nil {
		m.mu.Unlock()
		return false
	}
	h := runs[len(runs)-1]
	if h.AttemptID != attemptID || h.SessionID != sess.ID() || h.Status != "running" || sess.IsClosed() || sess.LastInputAt().After(acceptedAt) {
		m.mu.Unlock()
		return false
	}
	h.EndReason = "agent_report"
	delete(m.sessions, taskID)
	m.mu.Unlock()
	_ = sess.Close()
	return true
}

// GetTaskHistory returns all execution history records for a task.
func (m *Manager) GetTaskHistory(taskID string) []*TaskRunHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list, exists := m.histories[taskID]
	if !exists {
		return []*TaskRunHistory{}
	}
	result := make([]*TaskRunHistory, len(list))
	for i, h := range list {
		clone := *h
		result[i] = &clone
	}
	return result
}
