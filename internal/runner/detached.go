package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

// DetachedWorkerRecord holds persistent state of a detached worker executing a task attempt.
type DetachedWorkerRecord struct {
	AttemptID     string    `json:"attempt_id"`
	TaskID        string    `json:"task_id"`
	ProjectID     string    `json:"project_id,omitempty"`
	Lane          string    `json:"lane"`
	PID           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	ClaimFile     string    `json:"claim_file"`
	LogFile       string    `json:"log_file,omitempty"`
	WorkingDir    string    `json:"working_dir,omitempty"`
	ScopeUnit     string    `json:"scope_unit,omitempty"`
}

// WorkerRegistry manages persisted worker state on the local filesystem.
type WorkerRegistry struct {
	baseDir string
	mu      sync.Mutex
}

// NewWorkerRegistry initializes a registry in the given directory.
func NewWorkerRegistry(dir string) *WorkerRegistry {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "cuan-workers")
	}
	_ = os.MkdirAll(dir, 0755)
	return &WorkerRegistry{baseDir: dir}
}

// Dir returns the base directory of the registry.
func (r *WorkerRegistry) Dir() string {
	return r.baseDir
}

// RecordPath returns the file path for an attempt's worker record.
func (r *WorkerRegistry) RecordPath(attemptID string) string {
	safe := strings.ReplaceAll(attemptID, string(filepath.Separator), "_")
	return filepath.Join(r.baseDir, fmt.Sprintf("%s.json", safe))
}

// ClaimPath returns the file path for an attempt's claim payload.
func (r *WorkerRegistry) ClaimPath(attemptID string) string {
	safe := strings.ReplaceAll(attemptID, string(filepath.Separator), "_")
	return filepath.Join(r.baseDir, fmt.Sprintf("claim-%s.json", safe))
}

// LogPath returns the file path for an attempt's worker process log.
func (r *WorkerRegistry) LogPath(attemptID string) string {
	safe := strings.ReplaceAll(attemptID, string(filepath.Separator), "_")
	return filepath.Join(r.baseDir, fmt.Sprintf("worker-%s.log", safe))
}

// Save atomically writes a worker record to disk.
func (r *WorkerRegistry) Save(rec *DetachedWorkerRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal worker record: %w", err)
	}

	target := r.RecordPath(rec.AttemptID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp record: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit worker record: %w", err)
	}
	return nil
}

// Get retrieves a worker record from disk.
func (r *WorkerRegistry) Get(attemptID string) (*DetachedWorkerRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	target := r.RecordPath(attemptID)
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}

	var rec DetachedWorkerRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal worker record: %w", err)
	}
	return &rec, nil
}

// Delete removes a worker record, claim file, and optionally log file from disk.
func (r *WorkerRegistry) Delete(attemptID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_ = os.Remove(r.RecordPath(attemptID))
	_ = os.Remove(r.ClaimPath(attemptID))
	return nil
}

// List returns all active worker records stored in the registry directory.
func (r *WorkerRegistry) List() ([]*DetachedWorkerRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []*DetachedWorkerRecord
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, "claim-") || strings.HasSuffix(name, ".tmp") {
			continue
		}

		filePath := filepath.Join(r.baseDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var rec DetachedWorkerRecord
		if err := json.Unmarshal(data, &rec); err == nil && rec.AttemptID != "" {
			records = append(records, &rec)
		}
	}
	return records, nil
}

// UpdateHeartbeat touches the timestamp on an active record.
func (r *WorkerRegistry) UpdateHeartbeat(attemptID string) error {
	rec, err := r.Get(attemptID)
	if err != nil {
		return err
	}
	rec.LastHeartbeat = time.Now().UTC()
	return r.Save(rec)
}

// IsProcessAlive checks whether a process is running on the local host.
func (r *WorkerRegistry) IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Sending signal 0 performs error checking without actually delivering a signal
	err := syscall.Kill(pid, 0)
	if err == nil {
		// Process exists. On Linux, verify it is not in zombie state.
		statusPath := fmt.Sprintf("/proc/%d/status", pid)
		if data, sErr := os.ReadFile(statusPath); sErr == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "State:") {
					if strings.Contains(line, "Z (zombie)") || strings.Contains(line, "X (dead)") {
						return false
					}
					break
				}
			}
		}
		return true
	}

	if err == syscall.EPERM {
		// Process exists but owned by another user or privileged
		return true
	}

	return false
}

// DetachedLauncher handles launching detached worker processes with isolated scopes.
type DetachedLauncher struct {
	registry            *WorkerRegistry
	customCommandRunner func(cmd *exec.Cmd) error
}

// NewDetachedLauncher creates a launcher bound to a registry.
func NewDetachedLauncher(registry *WorkerRegistry) *DetachedLauncher {
	return &DetachedLauncher{registry: registry}
}

// ResolveRunnerBinary finds the appropriate cuan-runner executable.
func ResolveRunnerBinary(cfg *config.Config) string {
	if cfg != nil && cfg.RunnerBinPath != "" {
		if fi, err := os.Stat(cfg.RunnerBinPath); err == nil && !fi.IsDir() {
			return cfg.RunnerBinPath
		}
	}

	for _, cand := range []string{
		"/opt/cuan/current/bin/cuan-runner",
		"/opt/cuan/bin/cuan-runner",
		"bin/cuan-runner",
		"/data/home/cuan/bin/cuan-runner",
		"/app/cuan-runner",
	} {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}

	if execPath, err := os.Executable(); err == nil {
		if filepath.Base(execPath) == "cuan-runner" {
			return execPath
		}
		cand := filepath.Join(filepath.Dir(execPath), "cuan-runner")
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}

	if looked, err := exec.LookPath("cuan-runner"); err == nil {
		return looked
	}

	return "cuan-runner"
}

// SpawnDetachedWorker spawns a decoupled worker process isolated from supervisor cgroup.
func (l *DetachedLauncher) Spawn(ctx context.Context, cfg *config.Config, claim ClaimTaskResponse) (*DetachedWorkerRecord, error) {
	if claim.Attempt == nil || claim.Task == nil {
		return nil, fmt.Errorf("invalid claim payload: attempt and task required")
	}

	attemptID := claim.Attempt.ID
	taskID := claim.Task.ID
	lane := task.NormalizeExecutionCategory(claim.Task.ExecutionCategory)
	if lane == "" {
		lane = task.ExecutionCategoryCode
	}

	// 1. Write claim payload to disk for the detached worker
	claimPath := l.registry.ClaimPath(attemptID)
	claimBytes, err := json.Marshal(claim)
	if err != nil {
		return nil, fmt.Errorf("marshal claim payload: %w", err)
	}
	if err := os.WriteFile(claimPath, claimBytes, 0644); err != nil {
		return nil, fmt.Errorf("write claim file %s: %w", claimPath, err)
	}

	binPath := ResolveRunnerBinary(cfg)
	logPath := l.registry.LogPath(attemptID)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		_ = os.Remove(claimPath)
		return nil, fmt.Errorf("open worker log file: %w", err)
	}

	workerArgs := []string{
		"worker-task",
		"--claim-file=" + claimPath,
		"--registry-dir=" + l.registry.Dir(),
		"--runner-id=" + cfg.RunnerID,
		"--server-url=" + cfg.RunnerServerURL,
	}
	if cfg.RunnerToken != "" {
		workerArgs = append(workerArgs, "--runner-token="+cfg.RunnerToken)
	}

	var cmd *exec.Cmd
	scopeUnit := ""

	// Try systemd-run --scope if privileged/available
	if os.Geteuid() == 0 {
		if _, lookErr := exec.LookPath("systemd-run"); lookErr == nil {
			unit := fmt.Sprintf("cuan-worker-%s.scope", strings.ReplaceAll(attemptID, "_", "-"))
			sysArgs := []string{
				"--scope",
				"--unit=" + unit,
				"--description=Cuan Detached Worker " + attemptID,
				"--quiet",
				binPath,
			}
			sysArgs = append(sysArgs, workerArgs...)
			sysCmd := exec.Command("systemd-run", sysArgs...)
			sysCmd.Stdout = logFile
			sysCmd.Stderr = logFile
			sysCmd.Env = os.Environ()
			if sErr := sysCmd.Start(); sErr == nil {
				cmd = sysCmd
				scopeUnit = unit
			}
		}
	}

	// Fallback to POSIX session detachment (setsid)
	if cmd == nil {
		cmd = exec.Command(binPath, workerArgs...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true, // Creates a new session, detaching from parent process tree and terminal
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.Env = os.Environ()

		if l.customCommandRunner != nil {
			if err := l.customCommandRunner(cmd); err != nil {
				_ = logFile.Close()
				_ = os.Remove(claimPath)
				return nil, fmt.Errorf("custom spawn worker: %w", err)
			}
		} else {
			if err := cmd.Start(); err != nil {
				_ = logFile.Close()
				_ = os.Remove(claimPath)
				return nil, fmt.Errorf("start detached worker: %w", err)
			}
		}
	}

	_ = logFile.Close()

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
		// Detachment must survive supervisor shutdown, but while the parent
		// remains alive it still owns wait(2). Reap asynchronously so completed
		// workers do not accumulate as zombies. Exit reporting belongs to the
		// worker protocol; waiting here neither accepts nor retries the task.
		go func() { _ = cmd.Wait() }()
	}

	rec := &DetachedWorkerRecord{
		AttemptID:     attemptID,
		TaskID:        taskID,
		ProjectID:     claim.Task.ProjectID,
		Lane:          lane,
		PID:           pid,
		StartedAt:     time.Now().UTC(),
		LastHeartbeat: time.Now().UTC(),
		ClaimFile:     claimPath,
		LogFile:       logPath,
		WorkingDir:    claim.WorkingDir,
		ScopeUnit:     scopeUnit,
	}

	if err := l.registry.Save(rec); err != nil {
		log.Printf("[DetachedLauncher] Warning: failed to save worker record for PID %d: %v", pid, err)
	}

	log.Printf("[DetachedLauncher] Spawned detached worker PID %d for attempt %s (lane: %s, scope: '%s')",
		pid, attemptID, lane, scopeUnit)

	return rec, nil
}
