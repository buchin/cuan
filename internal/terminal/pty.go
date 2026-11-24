package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

var (
	ErrPTYClosed      = errors.New("pty process is closed")
	ErrInvalidWorkDir = errors.New("invalid or insecure working directory")
	ErrShellNotFound  = errors.New("shell executable not found")
)

// PTYConfig defines parameters for spawning an isolated pseudo-terminal process.
type PTYConfig struct {
	TaskID  string
	WorkDir string
	Shell   string   // preferred shell, e.g. /bin/bash or /bin/sh
	Args    []string // optional arguments passed to shell
	Env     []string // optional environment variables (will be sanitized)
	// DelegatedEnv contains only explicitly authorized project values, never host environment.
	DelegatedEnv map[string]string
	InitialCols  uint16 // default 80
	InitialRows  uint16 // default 24
}

// PTYProcess wraps an active operating-system PTY slave and master descriptor.
type PTYProcess struct {
	cmd      *exec.Cmd
	ptyFile  *os.File
	mu       sync.Mutex
	closed   bool
	taskID   string
	workDir  string
	cols     uint16
	rows     uint16
	waitDone chan struct{}
	waitErr  error
}

// FindShell returns the first available shell binary on the host.
func FindShell(preferred string) string {
	if preferred != "" {
		if path, err := exec.LookPath(preferred); err == nil {
			return path
		}
	}

	candidates := []string{
		"/bin/bash",
		"/usr/bin/bash",
		"/bin/sh",
		"/usr/bin/sh",
	}

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
			return cand
		}
	}

	// Fallback to standard LookPath
	if path, err := exec.LookPath("bash"); err == nil {
		return path
	}
	if path, err := exec.LookPath("sh"); err == nil {
		return path
	}

	return "/bin/sh"
}

// IsSensitiveKey returns true if an environment variable key is considered sensitive.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	// Git author identity, harness provider keys, and antigravity profile environment explicitly delegated to agent executions are permitted.
	switch upper {
	case "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "COMMAND_CODE_API_KEY",
		"ANTIGRAVITY_TOKEN_PATH", "ANTIGRAVITY_CONFIG_DIR", "GEMINI_CLI_HOME":
		return false
	}

	sensitivePatterns := []string{
		"SECRET", "PASSWORD", "PASSWD", "TOKEN", "KEY", "CREDENTIAL",
		"AUTH", "PRIVATE", "AWS_", "DOJO_", "DATABASE_URL", "RESEND",
		"TINYFISH", "NERD_", "S3_", "COOLIFY", "BEARER", "SESSION",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// SanitizeEnvironment filters out sensitive credentials and injects essential safe environment variables.
func SanitizeEnvironment(customEnv []string, taskID, workDir string) []string {
	safeVars := make(map[string]string)

	// Baseline environment
	safeVars["TERM"] = "xterm-256color"
	safeVars["LANG"] = "en_US.UTF-8"
	safeVars["LC_ALL"] = "en_US.UTF-8"
	safeVars["COLORTERM"] = "truecolor"
	if taskID != "" {
		safeVars["TASK_ID"] = taskID
	}
	if workDir != "" {
		safeVars["PWD"] = workDir
		safeVars["WORKSPACE_DIR"] = workDir
	}

	// Keep essential system variables from host if not sensitive
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := parts[0], parts[1]
		if IsSensitiveKey(k) {
			continue
		}

		switch k {
		case "PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR":
			safeVars[k] = v
		}
	}

	// Ensure fallback PATH if not set
	if safeVars["PATH"] == "" {
		safeVars["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	// Merge caller custom envs if not sensitive
	for _, env := range customEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := parts[0], parts[1]
		if !IsSensitiveKey(k) {
			safeVars[k] = v
		}
	}

	result := make([]string, 0, len(safeVars))
	for k, v := range safeVars {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// SpawnPTY spawns a shell process bound to an isolated working directory with creack/pty.
func SpawnPTY(ctx context.Context, cfg PTYConfig) (*PTYProcess, error) {
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("%w: workdir cannot be empty", ErrInvalidWorkDir)
	}

	// Ensure workdir exists
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("ensure working directory %q: %w", cfg.WorkDir, err)
	}

	// Clean and canonicalize workdir
	canonicalDir, err := filepath.Abs(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolve canonical workdir %q: %w", cfg.WorkDir, err)
	}

	shellPath := cfg.Shell
	cmdArgs := cfg.Args
	if shellPath == "" {
		shellPath = FindShell("")
		cmdArgs = []string{"-i"}
	} else if fi, err := os.Stat(shellPath); err != nil || fi.IsDir() || (fi.Mode()&0111 == 0) {
		// If custom executable is missing or not executable, look up in PATH or fall back to default shell without custom flags
		if looked, err := exec.LookPath(shellPath); err == nil {
			shellPath = looked
		} else {
			shellPath = FindShell("")
			cmdArgs = []string{"-i"}
		}
	}
	if shellPath == "" {
		return nil, ErrShellNotFound
	}

	cols := cfg.InitialCols
	if cols == 0 {
		cols = 80
	}
	rows := cfg.InitialRows
	if rows == 0 {
		rows = 24
	}

	env := SanitizeEnvironment(cfg.Env, cfg.TaskID, canonicalDir)
	for key, value := range cfg.DelegatedEnv {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("invalid delegated environment key")
		}
		// Ordinary values already passed through the normal environment path.
		// Only restore filtered project credentials; never overwrite the native
		// task capability or brain route with project configuration.
		upper := strings.ToUpper(key)
		if !IsSensitiveKey(key) || strings.HasPrefix(upper, "CUAN_TASK_") || strings.HasPrefix(upper, "CUAN_BRAIN_") {
			continue
		}
		prefix := key + "="
		filtered := env[:0]
		for _, item := range env {
			if !strings.HasPrefix(item, prefix) {
				filtered = append(filtered, item)
			}
		}
		env = append(filtered, prefix+value)
	}

	cmd := exec.CommandContext(ctx, shellPath, cmdArgs...)
	cmd.Dir = canonicalDir
	cmd.Env = env
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGINT)
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		_ = syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 3 * time.Second

	ws := &pty.Winsize{
		Cols: cols,
		Rows: rows,
	}

	ptyFile, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, fmt.Errorf("pty start shell %s: %w", shellPath, err)
	}

	proc := &PTYProcess{
		cmd:      cmd,
		ptyFile:  ptyFile,
		taskID:   cfg.TaskID,
		workDir:  canonicalDir,
		cols:     cols,
		rows:     rows,
		waitDone: make(chan struct{}),
	}

	// Asynchronously wait for the process to exit to avoid zombie processes
	go func() {
		proc.waitErr = cmd.Wait()
		proc.mu.Lock()
		proc.closed = true
		if proc.ptyFile != nil {
			_ = proc.ptyFile.Close()
		}
		proc.mu.Unlock()
		// The leader can exit while commands that ignored TERM/HUP remain in
		// its owned group. Clean that group before publishing execution completion.
		terminateOwnedPTYGroup(cmd.Process.Pid)
		close(proc.waitDone)
	}()

	return proc, nil
}

// PID returns the underlying process identifier.
func (p *PTYProcess) PID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// ExitCode returns the exit code of the process once exited, or -1 if still running.
func (p *PTYProcess) ExitCode() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.ProcessState != nil {
		return p.cmd.ProcessState.ExitCode()
	}
	return -1
}

// Read reads output from the master PTY descriptor.
func (p *PTYProcess) Read(buf []byte) (int, error) {
	p.mu.Lock()
	if p.closed || p.ptyFile == nil {
		p.mu.Unlock()
		return 0, ErrPTYClosed
	}
	f := p.ptyFile
	p.mu.Unlock()

	return f.Read(buf)
}

// Write writes input bytes into the master PTY descriptor (piped to shell stdin).
func (p *PTYProcess) Write(buf []byte) (int, error) {
	p.mu.Lock()
	if p.closed || p.ptyFile == nil {
		p.mu.Unlock()
		return 0, ErrPTYClosed
	}
	f := p.ptyFile
	p.mu.Unlock()

	return f.Write(buf)
}

// SetSize updates the pseudo-terminal window dimensions and emits SIGWINCH.
func (p *PTYProcess) SetSize(cols, rows uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || p.ptyFile == nil {
		return ErrPTYClosed
	}

	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}

	ws := &pty.Winsize{
		Cols: cols,
		Rows: rows,
	}

	if err := pty.Setsize(p.ptyFile, ws); err != nil {
		return fmt.Errorf("pty setsize: %w", err)
	}

	p.cols = cols
	p.rows = rows
	return nil
}

// GetSize returns current terminal dimensions (cols, rows).
func (p *PTYProcess) GetSize() (uint16, uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cols, p.rows
}

// TaskID returns the bound task identifier.
func (p *PTYProcess) TaskID() string {
	return p.taskID
}

// WorkDir returns the canonical working directory.
func (p *PTYProcess) WorkDir() string {
	return p.workDir
}

// IsClosed checks if the process has been terminated.
func (p *PTYProcess) IsClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

// Wait blocks until the shell process exits.
func (p *PTYProcess) Wait() error {
	<-p.waitDone
	return p.waitErr
}

// Done returns a channel that is closed when the process has exited.
func (p *PTYProcess) Done() <-chan struct{} {
	return p.waitDone
}

// Kill terminates the process group gracefully with SIGINT/SIGTERM, followed by SIGKILL if required.
func (p *PTYProcess) Kill() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		<-p.waitDone
		return nil
	}
	p.closed = true

	cmd := p.cmd
	ptyFile := p.ptyFile
	p.mu.Unlock()

	// 1. Signal process group with SIGINT and SIGTERM
	if cmd != nil && cmd.Process != nil {
		pid := cmd.Process.Pid
		_ = syscall.Kill(-pid, syscall.SIGINT)
		_ = cmd.Process.Signal(syscall.SIGINT)
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = syscall.Kill(-pid, syscall.SIGHUP)
		_ = cmd.Process.Signal(syscall.SIGHUP)
	}

	// 2. Close master PTY descriptor to trigger hangup on slave
	if ptyFile != nil {
		_ = ptyFile.Close()
	}

	// 3. Graceful check, escalate to SIGKILL if not exited within 200ms
	select {
	case <-p.waitDone:
		return nil
	case <-time.After(200 * time.Millisecond):
		if cmd != nil && cmd.Process != nil {
			pid := cmd.Process.Pid
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
	}

	<-p.waitDone
	return nil
}

// TerminateProcessGroup terminates an entire process group gracefully with SIGINT and SIGTERM,
// escalating to SIGKILL if processes remain alive.
func TerminateProcessGroup(pid int) {
	if pid <= 0 {
		return
	}
	if syscall.Kill(-pid, 0) == syscall.ESRCH && syscall.Kill(pid, 0) == syscall.ESRCH {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGINT)
	_ = syscall.Kill(pid, syscall.SIGINT)
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	_ = syscall.Kill(pid, syscall.SIGTERM)
	if waitPTYGroupExit(pid, 200*time.Millisecond) {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
	waitPTYGroupExit(pid, 200*time.Millisecond)
}

// Only the fresh session/process group created by SpawnPTY is signalled. This
// does not discover daemonized children that explicitly leave that group.
func terminateOwnedPTYGroup(pid int) {
	TerminateProcessGroup(pid)
}

func waitPTYGroupExit(pid int, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		if syscall.Kill(-pid, 0) == syscall.ESRCH && syscall.Kill(pid, 0) == syscall.ESRCH {
			return true
		}
		select {
		case <-poll.C:
		case <-deadline.C:
			return false
		}
	}
}

// Close is an alias for Kill.
func (p *PTYProcess) Close() error {
	return p.Kill()
}
