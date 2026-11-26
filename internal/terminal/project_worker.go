package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

// ProjectWorkerPayload contains all parameters needed to initialize a detached project terminal session.
type ProjectWorkerPayload struct {
	ProjectID    string   `json:"project_id"`
	WorkDir      string   `json:"work_dir"`
	Shell        string   `json:"shell,omitempty"`
	Cols         uint16   `json:"cols,omitempty"`
	Rows         uint16   `json:"rows,omitempty"`
	Env          []string `json:"env,omitempty"`
	OwnerID      string   `json:"owner_id,omitempty"`
	AccountID    string   `json:"account_id,omitempty"`
	AccountLabel string   `json:"account_label,omitempty"`
	ModelID      string   `json:"model_id,omitempty"`
	Provider     string   `json:"provider,omitempty"`
}

// ProjectWorkerMeta describes the active brain and session metadata for a detached project worker.
type ProjectWorkerMeta struct {
	ProjectID    string `json:"project_id"`
	SessionID    string `json:"session_id,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	AccountLabel string `json:"account_label,omitempty"`
	ModelID      string `json:"model_id,omitempty"`
	Provider     string `json:"provider,omitempty"`
}

var projectQuotaANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

type projectQuotaWatcher struct {
	mu       sync.Mutex
	ownerID  string
	acctID   string
	modelID  string
	tail     string
	recorded bool
}

func newProjectQuotaWatcher(ownerID, acctID, modelID string) *projectQuotaWatcher {
	if ownerID == "" || acctID == "" {
		return nil
	}
	return &projectQuotaWatcher{
		ownerID: ownerID,
		acctID:  acctID,
		modelID: modelID,
	}
}

func (w *projectQuotaWatcher) Observe(chunk []byte) {
	if w == nil || len(chunk) == 0 {
		return
	}
	w.mu.Lock()
	if w.recorded {
		w.mu.Unlock()
		return
	}
	w.tail += string(chunk)
	if len(w.tail) > 8192 {
		w.tail = w.tail[len(w.tail)-8192:]
	}
	plain := strings.Join(strings.Fields(projectQuotaANSI.ReplaceAllString(w.tail, "")), " ")
	hit := (strings.Contains(plain, "Individual quota reached") && strings.Contains(plain, "Please upgrade your subscription")) ||
		(strings.Contains(plain, "RESOURCE_EXHAUSTED") && strings.Contains(plain, "Individual quota reached"))
	if !hit {
		w.mu.Unlock()
		return
	}
	w.recorded = true
	w.mu.Unlock()

	duration, resetStr, _ := agent.ParseProviderQuotaResetDuration(plain)
	if duration > 24*time.Hour {
		duration = 24 * time.Hour
	}
	if w.modelID != "" {
		_ = agent.RecordProviderModelQuotaWithDuration(w.ownerID, w.acctID, w.modelID, duration)
	} else {
		_ = agent.RecordProviderAccountQuotaWithDuration(w.ownerID, w.acctID, duration, resetStr)
	}
}

func projectWorkerMetaPath(projectID string) string {
	safeID := safeSocketNameRegex.ReplaceAllString(strings.TrimPrefix(strings.TrimSpace(projectID), "project-term-"), "_")
	return filepath.Join(socketDir(), "cuan-workers", fmt.Sprintf("project-meta-%s.json", safeID))
}

// ReadProjectWorkerMeta reads the active brain metadata for a detached project terminal worker, if present.
func ReadProjectWorkerMeta(projectID string) (*ProjectWorkerMeta, bool) {
	if strings.TrimSpace(projectID) == "" {
		return nil, false
	}
	data, err := os.ReadFile(projectWorkerMetaPath(projectID))
	if err != nil {
		return nil, false
	}
	var meta ProjectWorkerMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, false
	}
	return &meta, true
}

// ProjectSocketPath returns the standard Unix domain socket path for a project terminal session.
func ProjectSocketPath(projectID string) string {
	return SocketPath(ProjectSessionKey(projectID))
}

// ProjectSocketExists checks whether an active Unix domain socket bridge exists for the project terminal.
func ProjectSocketExists(projectID string) bool {
	return SocketExists(ProjectSessionKey(projectID))
}

// RunProjectWorker executes the project shell inside a persistent PTY and serves it via SocketBridge.
func RunProjectWorker(ctx context.Context, payload ProjectWorkerPayload) error {
	projectID := strings.TrimSpace(payload.ProjectID)
	if projectID == "" {
		return errors.New("project id is required")
	}

	sessionKey := ProjectSessionKey(projectID)
	sockPath := ProjectSocketPath(projectID)

	// Clean up lingering socket file from previous run if any
	_ = os.Remove(sockPath)

	execPath, args, env := ResolveProjectTerminalExecutable(payload.WorkDir, payload.Shell, payload.Env)
	if execPath == "" {
		execPath = FindShell("")
	}

	cols := payload.Cols
	if cols == 0 {
		cols = 80
	}
	rows := payload.Rows
	if rows == 0 {
		rows = 24
	}

	ptyCfg := PTYConfig{
		TaskID:      sessionKey,
		WorkDir:     payload.WorkDir,
		Shell:       execPath,
		Args:        args,
		InitialCols: cols,
		InitialRows: rows,
		Env:         env,
	}

	spawnCtx := context.Background()
	if ctx != nil {
		spawnCtx = context.WithoutCancel(ctx)
	}

	proc, err := SpawnPTY(spawnCtx, ptyCfg)
	if err != nil {
		return fmt.Errorf("spawn pty for project %s: %w", projectID, err)
	}
	defer proc.Close()

	sessionID := fmt.Sprintf("proj-sess-%s-%d", projectID, time.Now().Unix())
	quotaWatcher := newProjectQuotaWatcher(payload.OwnerID, payload.AccountID, payload.ModelID)
	var onOutput func([]byte)
	if quotaWatcher != nil {
		onOutput = quotaWatcher.Observe
	}
	sess := NewSession(sessionID, sessionKey, proc, ModeInteractive, onOutput)
	sess.SetPersistent(true)
	defer sess.Close()

	bridge, err := NewSocketBridge(sess, sockPath)
	if err != nil {
		return fmt.Errorf("create socket bridge at %s: %w", sockPath, err)
	}
	defer bridge.Close()

	guardrailCtx, cancelGuardrail := context.WithCancel(spawnCtx)
	defer cancelGuardrail()
	if proc.cmd != nil && proc.cmd.Process != nil && proc.cmd.Process.Pid > 0 {
		go RunTerminalMemoryGuardrail(guardrailCtx, proc.cmd.Process.Pid, proc.ptyFile, GetTerminalMaxRSSMB(), 5*time.Second)
	}

	// Write worker metadata/PID
	statusDir := filepath.Join(socketDir(), "cuan-workers")
	_ = os.MkdirAll(statusDir, 0755)
	safeID := safeSocketNameRegex.ReplaceAllString(strings.TrimPrefix(projectID, "project-term-"), "_")
	pidFile := filepath.Join(statusDir, fmt.Sprintf("project_%s.pid", safeID))
	_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	defer os.Remove(pidFile)

	metaFile := projectWorkerMetaPath(projectID)
	if metaBytes, mErr := json.Marshal(ProjectWorkerMeta{
		ProjectID:    projectID,
		SessionID:    sessionID,
		AccountID:    payload.AccountID,
		AccountLabel: payload.AccountLabel,
		ModelID:      payload.ModelID,
		Provider:     payload.Provider,
	}); mErr == nil {
		_ = os.WriteFile(metaFile, metaBytes, 0600)
	}
	defer os.Remove(metaFile)

	// Detached project workers survive parent restarts and must ignore stray SIGHUP.
	signal.Ignore(syscall.SIGHUP)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigChan)

	select {
	case <-proc.waitDone:
		log.Printf("[ProjectWorker] Shell process exited for project %s", projectID)
	case sig := <-sigChan:
		log.Printf("[ProjectWorker] Received signal %v, terminating project %s", sig, projectID)
	case <-ctx.Done():
		log.Printf("[ProjectWorker] Context cancelled for project %s", projectID)
	}

	_ = os.Remove(sockPath)
	return nil
}

// RunProjectWorkerCLI parses command line arguments and runs the project worker session.
func RunProjectWorkerCLI(args []string) int {
	flags := flag.NewFlagSet("worker-project", flag.ExitOnError)
	payloadFile := flags.String("payload-file", "", "Path to JSON file containing ProjectWorkerPayload")
	projectID := flags.String("project-id", "", "Project ID")
	workDir := flags.String("work-dir", "", "Working directory")
	shell := flags.String("shell", "", "Shell path")
	cols := flags.Uint("cols", 80, "Initial columns")
	rows := flags.Uint("rows", 24, "Initial rows")
	_ = flags.Parse(args)

	var payload ProjectWorkerPayload
	if *payloadFile != "" {
		data, err := os.ReadFile(*payloadFile)
		if err != nil {
			log.Printf("[ProjectWorker] Failed to read payload file %s: %v", *payloadFile, err)
			return 1
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			log.Printf("[ProjectWorker] Failed to unmarshal payload JSON: %v", err)
			return 1
		}
	} else {
		payload = ProjectWorkerPayload{
			ProjectID: *projectID,
			WorkDir:   *workDir,
			Shell:     *shell,
			Cols:      uint16(*cols),
			Rows:      uint16(*rows),
		}
	}

	if payload.ProjectID == "" {
		log.Println("[ProjectWorker] Error: project-id is required")
		return 1
	}

	if err := RunProjectWorker(context.Background(), payload); err != nil {
		log.Printf("[ProjectWorker] Run failed: %v", err)
		return 1
	}
	return 0
}

// SpawnDetachedProjectWorker spawns a background, detached project worker process that serves
// the project terminal PTY over a Unix domain socket bridge.
func SpawnDetachedProjectWorker(payload ProjectWorkerPayload, customBin string) (*exec.Cmd, error) {
	if payload.ProjectID == "" {
		return nil, errors.New("project_id is required")
	}

	binPath := customBin
	if binPath == "" {
		if looked, err := exec.LookPath("cuan-runner"); err == nil {
			binPath = looked
		} else if lookedServer, err := exec.LookPath("cuan-server"); err == nil {
			binPath = lookedServer
		}
	}
	if binPath == "" {
		for _, cand := range []string{
			"/opt/cuan/current/bin/cuan-runner",
			"/opt/cuan/bin/cuan-runner",
			"bin/cuan-runner",
			"../../bin/cuan-runner",
			"../bin/cuan-runner",
			"/app/cuan-runner",
		} {
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				if abs, absErr := filepath.Abs(cand); absErr == nil {
					binPath = abs
				} else {
					binPath = cand
				}
				break
			}
		}
	}
	if binPath == "" {
		if execPath, err := os.Executable(); err == nil && !strings.HasSuffix(execPath, ".test") {
			cand := filepath.Join(filepath.Dir(execPath), "cuan-runner")
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				binPath = cand
			} else {
				binPath = execPath
			}
		}
	}
	if binPath == "" {
		return nil, errors.New("unable to resolve executable to spawn detached project worker")
	}

	workersDir := filepath.Join(socketDir(), "cuan-workers")
	_ = os.MkdirAll(workersDir, 0755)

	safeID := safeSocketNameRegex.ReplaceAllString(strings.TrimPrefix(strings.TrimSpace(payload.ProjectID), "project-term-"), "_")
	payloadFile := filepath.Join(workersDir, fmt.Sprintf("project-payload-%s.json", safeID))
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	if err := os.WriteFile(payloadFile, data, 0600); err != nil {
		return nil, fmt.Errorf("write payload file: %w", err)
	}

	logFile := filepath.Join(workersDir, fmt.Sprintf("project-%s.log", safeID))
	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		_ = os.Remove(payloadFile)
		return nil, fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(binPath, "worker-project", "--payload-file="+payloadFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Creates independent POSIX session detached from caller process tree
	}
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		_ = lf.Close()
		_ = os.Remove(payloadFile)
		return nil, fmt.Errorf("start detached project worker: %w", err)
	}
	_ = lf.Close()

	// Asynchronously reap worker process on exit
	go func() {
		_ = cmd.Wait()
		_ = os.Remove(payloadFile)
	}()

	// Wait for socket to become active
	sockPath := ProjectSocketPath(payload.ProjectID)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ProjectSocketExists(payload.ProjectID) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ProjectSocketExists(payload.ProjectID) {
		log.Printf("[ProjectWorker] Notice: socket %s not yet responsive after 3s", sockPath)
	}

	return cmd, nil
}

// TerminateProjectWorker terminates the detached project worker process if running and removes its socket.
func TerminateProjectWorker(projectID string) error {
	rawID := strings.TrimPrefix(strings.TrimSpace(projectID), "project-term-")
	sessionKey := ProjectSessionKey(rawID)
	sockPath := SocketPath(sessionKey)

	statusDir := filepath.Join(socketDir(), "cuan-workers")
	safeID := safeSocketNameRegex.ReplaceAllString(rawID, "_")
	pidFile := filepath.Join(statusDir, fmt.Sprintf("project_%s.pid", safeID))
	payloadFile := filepath.Join(statusDir, fmt.Sprintf("project-payload-%s.json", safeID))
	if pidData, err := os.ReadFile(pidFile); err == nil {
		if pid, pErr := strconv.Atoi(strings.TrimSpace(string(pidData))); pErr == nil && pid > 0 {
			caller := "unknown"
			if pc, file, line, ok := runtime.Caller(1); ok {
				fn := runtime.FuncForPC(pc)
				fnName := ""
				if fn != nil {
					fnName = fn.Name()
				}
				caller = fmt.Sprintf("%s:%d (%s)", filepath.Base(file), line, fnName)
			}
			logFile := filepath.Join(statusDir, fmt.Sprintf("project-%s.log", safeID))
			if lf, lErr := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); lErr == nil {
				_, _ = fmt.Fprintf(lf, "%s [ProjectWorker] TerminateProjectWorker(%s) pid=%d called from %s\n",
					time.Now().Format("2006/01/02 15:04:05"), rawID, pid, caller)
				_ = lf.Close()
			}
			if pid == os.Getpid() {
				_ = syscall.Kill(pid, syscall.SIGTERM)
			} else {
				// Signal entire process group first (Setsid creates a process group with PGID == PID)
				_ = syscall.Kill(-pid, syscall.SIGTERM)
				_ = syscall.Kill(pid, syscall.SIGTERM)
				time.Sleep(30 * time.Millisecond)
				_ = syscall.Kill(-pid, syscall.SIGKILL)
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		_ = os.Remove(pidFile)
	}

	_ = os.Remove(payloadFile)
	_ = os.Remove(projectWorkerMetaPath(rawID))
	_ = os.Remove(sockPath)
	return nil
}

// ProjectWorkerLogTail returns the last maxLines from the detached project worker log file.
func ProjectWorkerLogTail(projectID string, maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 25
	}
	safeID := safeSocketNameRegex.ReplaceAllString(strings.TrimPrefix(strings.TrimSpace(projectID), "project-term-"), "_")
	if safeID == "" {
		return nil
	}
	logFile := filepath.Join(socketDir(), "cuan-workers", fmt.Sprintf("project-%s.log", safeID))
	data, err := os.ReadFile(logFile)
	if err != nil || len(data) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}
