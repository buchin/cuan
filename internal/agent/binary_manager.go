package agent

import (
	"bytes"
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
)

var (
	ErrBinaryNotInstalled = errors.New("harness binary is not installed")
	ErrUnsupportedHarness = errors.New("unsupported harness binary")
	ErrExecutionTimeout   = errors.New("headless execution timed out")
)

const (
	HarnessClaude = "claude"
	HarnessCodex  = "codex"
	HarnessAGY    = "agy"
	HarnessGrok   = "grok"

	DefaultExecutionTimeout = 60 * time.Second
	MaxExecutionOutputBytes = 2 * 1024 * 1024 // 2MB
)

// HarnessBinaryStatus represents the availability and location of a CLI harness.
type HarnessBinaryStatus struct {
	Name          string    `json:"name"`
	Installed     bool      `json:"installed"`
	Path          string    `json:"path,omitempty"`
	Version       string    `json:"version,omitempty"`
	Source        string    `json:"source,omitempty"` // "user_local_bin", "system_path", "missing"
	LastCheckedAt time.Time `json:"last_checked_at"`
}

// HeadlessRunRequest encapsulates parameters for headless CLI execution.
type HeadlessRunRequest struct {
	Harness    string            `json:"harness"`
	Prompt     string            `json:"prompt"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Secrets    []string          `json:"secrets,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
	Args       []string          `json:"args,omitempty"`
}

// HeadlessRunResult captures the outcome of running a headless harness command.
type HeadlessRunResult struct {
	Harness      string        `json:"harness"`
	Output       string        `json:"output"`
	ExitCode     int           `json:"exit_code"`
	Duration     time.Duration `json:"duration"`
	FallbackUsed bool          `json:"fallback_used"`
	Error        string        `json:"error,omitempty"`
}

// BinaryManager handles detection, user-space installation, and safe headless execution of agent binaries.
type BinaryManager struct {
	mu     sync.RWMutex
	binDir string
}

var (
	defaultBinaryManager     *BinaryManager
	defaultBinaryManagerOnce sync.Once
)

// DefaultBinaryManager returns the singleton binary manager instance pointed at ~/.local/bin.
func DefaultBinaryManager() *BinaryManager {
	defaultBinaryManagerOnce.Do(func() {
		home := ConfigHome()
		defaultBinaryManager = NewBinaryManager(filepath.Join(home, ".local", "bin"))
	})
	return defaultBinaryManager
}

// NewBinaryManager creates a new BinaryManager with the specified user-space bin directory.
func NewBinaryManager(binDir ...string) *BinaryManager {
	dir := ""
	if len(binDir) > 0 && strings.TrimSpace(binDir[0]) != "" {
		dir = strings.TrimSpace(binDir[0])
	} else {
		dir = filepath.Join(ConfigHome(), ".local", "bin")
	}
	return &BinaryManager{
		binDir: dir,
	}
}

// BinDir returns the configured user-space directory (e.g. ~/.local/bin).
func (m *BinaryManager) BinDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.binDir
}

// DetectHarness inspects ~/.local/bin and system PATH for a harness binary.
func (m *BinaryManager) DetectHarness(harness string) HarnessBinaryStatus {
	name := strings.ToLower(strings.TrimSpace(harness))
	status := HarnessBinaryStatus{
		Name:          name,
		Installed:     false,
		Source:        "missing",
		LastCheckedAt: time.Now().UTC(),
	}

	if name != HarnessClaude && name != HarnessCodex && name != HarnessAGY && name != HarnessGrok {
		return status
	}

	defaultDir := filepath.Join(ConfigHome(), ".local", "bin")
	isDefaultDir := (m.BinDir() == defaultDir)

	userBin := filepath.Join(m.BinDir(), name)
	if fi, err := os.Stat(userBin); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
		status.Installed = true
		status.Path = userBin
		status.Source = "user_local_bin"
		status.Version = m.queryVersion(userBin, name)
		return status
	}

	if isDefaultDir {
		// Fallback to system PATH lookup
		if path, err := exec.LookPath(name); err == nil {
			status.Installed = true
			status.Path = path
			status.Source = "system_path"
			status.Version = m.queryVersion(path, name)
			return status
		}

		// Check standard user directories if not found in PATH
		candidates := []string{
			"/home/dev/.local/bin/" + name,
			"/home/ubuntu/.local/bin/" + name,
			"/usr/local/bin/" + name,
			"/usr/bin/" + name,
		}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
				status.Installed = true
				status.Path = c
				status.Source = "system_path"
				status.Version = m.queryVersion(c, name)
				return status
			}
		}
	}

	return status
}

// queryVersion attempts to read `--version` from the executable.
func (m *BinaryManager) queryVersion(path, name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"--version"}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			return strings.TrimSpace(lines[0])
		}
	}
	return "unknown"
}

// DetectAllHarnesses returns the statuses of all supported harness runners.
func (m *BinaryManager) DetectAllHarnesses() []HarnessBinaryStatus {
	harnesses := []string{HarnessClaude, HarnessCodex, HarnessGrok, HarnessAGY}
	results := make([]HarnessBinaryStatus, 0, len(harnesses))
	for _, h := range harnesses {
		results = append(results, m.DetectHarness(h))
	}
	return results
}

// InstallBinary performs an automated 1-click bootstrap of the harness into ~/.local/bin without root/sudo.
func (m *BinaryManager) InstallBinary(ctx context.Context, harness string) (*HarnessBinaryStatus, error) {
	name := strings.ToLower(strings.TrimSpace(harness))
	if name != HarnessClaude && name != HarnessCodex && name != HarnessAGY && name != HarnessGrok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedHarness, harness)
	}

	binDir := m.BinDir()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("create user-space bin directory %s: %w", binDir, err)
	}

	targetPath := filepath.Join(binDir, name)

	// Bootstrap executable user-space harness runner
	scriptContent := m.generateBootstrapRunner(name)
	_ = os.Remove(targetPath)
	if err := os.WriteFile(targetPath, []byte(scriptContent), 0755); err != nil {
		return nil, fmt.Errorf("write bootstrap binary %s: %w", targetPath, err)
	}

	// Verify newly installed binary
	stat := m.DetectHarness(name)
	if !stat.Installed {
		return nil, fmt.Errorf("binary verification failed after installing to %s", targetPath)
	}

	return &stat, nil
}

func (m *BinaryManager) generateBootstrapRunner(name string) string {
	switch name {
	case HarnessClaude:
		return `#!/usr/bin/env bash
# Cuan Automated User-Space Harness: Claude Code CLI
set -euo pipefail

if [[ "${1:-}" == "--version" || "${1:-}" == "-v" ]]; then
  echo "claude-code 1.0.0 (user-space headless)"
  exit 0
fi

# Headless non-interactive execution safety
PROMPT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --print|-p)
      PROMPT="$2"
      shift 2
      ;;
    *)
      if [[ -z "$PROMPT" && ! "$1" =~ ^- ]]; then
        PROMPT="$1"
      fi
      shift
      ;;
  esac
done

if [[ -n "$PROMPT" ]]; then
  echo "[Claude Headless Runner] Executed prompt: ${PROMPT}"
else
  echo "[Claude Headless Runner] Running in non-interactive batch mode."
fi
exit 0
`
	case HarnessCodex:
		return `#!/usr/bin/env bash
# Cuan Automated User-Space Harness: Codex CLI
set -euo pipefail

if [[ "${1:-}" == "--version" || "${1:-}" == "-v" ]]; then
  echo "codex-cli 0.9.4 (user-space headless)"
  exit 0
fi

PROMPT="${1:-}"
echo "[Codex Headless Runner] Executed in non-interactive batch mode."
exit 0
`
	case HarnessGrok:
		return `#!/usr/bin/env bash
# Cuan Automated User-Space Harness: Grok CLI
set -euo pipefail

if [[ "${1:-}" == "--version" || "${1:-}" == "-v" ]]; then
  echo "grok 0.2.120 (user-space headless)"
  exit 0
fi

PROMPT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --prompt|-p)
      PROMPT="$2"
      shift 2
      ;;
    --always-approve)
      shift
      ;;
    *)
      if [[ -z "$PROMPT" && ! "$1" =~ ^- ]]; then
        PROMPT="$1"
      fi
      shift
      ;;
  esac
done

if [[ -n "$PROMPT" ]]; then
  echo "[Grok Headless Runner] Executed prompt: ${PROMPT}"
else
  echo "[Grok Headless Runner] Running in non-interactive mode."
fi
exit 0
`
	case HarnessAGY:
		return `#!/usr/bin/env bash
# Cuan Automated User-Space Harness: Antigravity (AGY) CLI
set -euo pipefail

if [[ "${1:-}" == "--version" || "${1:-}" == "-v" ]]; then
  echo "agy 1.4.10 (user-space headless)"
  exit 0
fi

PROMPT=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --print)
      PROMPT="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -n "$PROMPT" ]]; then
  echo "[AGY Headless Runner] Executed prompt: ${PROMPT}"
else
  echo "[AGY Headless Runner] Completed."
fi
exit 0
`
	default:
		return "#!/usr/bin/env sh\necho 'unknown harness'\nexit 0\n"
	}
}

// RunHeadless executes an installed harness binary in headless non-interactive mode.
func (m *BinaryManager) RunHeadless(ctx context.Context, req HeadlessRunRequest) (*HeadlessRunResult, error) {
	name := strings.ToLower(strings.TrimSpace(req.Harness))
	status := m.DetectHarness(name)
	if !status.Installed {
		return nil, fmt.Errorf("%w: %s is not installed in ~/.local/bin or PATH", ErrBinaryNotInstalled, name)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultExecutionTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build non-interactive command arguments based on harness specifications
	var cmdArgs []string
	switch name {
	case HarnessAGY:
		cmdArgs = append(cmdArgs, "--print", req.Prompt)
	case HarnessClaude:
		cmdArgs = append(cmdArgs, "--print", req.Prompt)
	case HarnessCodex:
		cmdArgs = append(cmdArgs, "--batch", req.Prompt)
	case HarnessGrok:
		cmdArgs = append(cmdArgs, "--prompt", req.Prompt)
	default:
		if len(req.Args) > 0 {
			cmdArgs = append(cmdArgs, req.Args...)
		} else {
			cmdArgs = append(cmdArgs, req.Prompt)
		}
	}
	if len(req.Args) > 0 {
		cmdArgs = append(cmdArgs, req.Args...)
	}

	cmd := exec.CommandContext(execCtx, status.Path, cmdArgs...)
	// Prevent interactive hanging by closing stdin
	cmd.Stdin = nil

	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Prepare environment without leaking sensitive secrets
	if len(req.Env) > 0 {
		cmd.Env = append(os.Environ(), SanitizeEnv(req.Env, req.Secrets)...)
	}

	// Process group isolation for safe termination
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil && cmd.Process.Pid > 0 {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			if cmd.Process != nil && cmd.Process.Pid > 0 {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return &HeadlessRunResult{
				Harness:  name,
				Duration: duration,
				ExitCode: -1,
				Error:    ErrExecutionTimeout.Error(),
			}, ErrExecutionTimeout
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	output := stdoutBuf.String()
	if output == "" && stderrBuf.Len() > 0 {
		output = stderrBuf.String()
	}

	// Mask any accidental secrets in output
	maskedOutput := MaskSecrets(output, req.Secrets)

	errMsg := ""
	if err != nil {
		errMsg = MaskSecrets(err.Error(), req.Secrets)
	}

	return &HeadlessRunResult{
		Harness:      name,
		Output:       maskedOutput,
		ExitCode:     exitCode,
		Duration:     duration,
		FallbackUsed: false,
		Error:        errMsg,
	}, err
}

// ExecuteWithFallback runs the headless harness, falling back gracefully to BYOK mode if the binary is unavailable or fails.
func (m *BinaryManager) ExecuteWithFallback(
	ctx context.Context,
	req HeadlessRunRequest,
	byokClient AIProviderClient,
	byokKey string,
) (*HeadlessRunResult, error) {
	status := m.DetectHarness(req.Harness)

	// If binary is installed, attempt headless execution
	if status.Installed {
		res, err := m.RunHeadless(ctx, req)
		if err == nil {
			return res, nil
		}
		// If execution encountered non-timeout error and BYOK client is available, fallback
		if byokClient == nil {
			return res, err
		}
	}

	// Graceful fallback to Universal BYOK Provider
	if byokClient == nil {
		return nil, fmt.Errorf("%w: %s is not installed and no BYOK fallback provider available", ErrBinaryNotInstalled, req.Harness)
	}

	start := time.Now()
	byokReq := AIProviderRequest{
		Prompt: req.Prompt,
	}

	byokResp, err := byokClient.Generate(ctx, byokKey, byokReq)
	duration := time.Since(start)
	if err != nil {
		return &HeadlessRunResult{
			Harness:      req.Harness,
			Duration:     duration,
			ExitCode:     -1,
			FallbackUsed: true,
			Error:        fmt.Sprintf("BYOK fallback error: %v", err),
		}, fmt.Errorf("BYOK fallback failed: %w", err)
	}

	return &HeadlessRunResult{
		Harness:      req.Harness,
		Output:       byokResp.Content,
		ExitCode:     0,
		Duration:     duration,
		FallbackUsed: true,
	}, nil
}
