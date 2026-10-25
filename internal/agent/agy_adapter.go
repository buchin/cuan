package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// AGYAdapter implements the Adapter interface using the official agy CLI.
type AGYAdapter struct {
	binaryPath string
}

// ResolveAGYBinaryPath locates the agy executable in PATH or standard candidate directories.
func ResolveAGYBinaryPath() (string, error) {
	if bin, err := exec.LookPath("agy"); err == nil {
		return bin, nil
	}
	for _, candidate := range []string{
		"/home/dev/.local/bin/agy",
		"/home/ubuntu/.local/bin/agy",
		"/usr/local/bin/agy",
		"/usr/bin/agy",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("agy binary not found in PATH or standard candidate directories")
}

// NewAGYAdapter creates a new AGYAdapter, verifying the binary location.
func NewAGYAdapter(customBinaryPath string) (*AGYAdapter, error) {
	bin := customBinaryPath
	if bin == "" {
		var err error
		bin, err = ResolveAGYBinaryPath()
		if err != nil {
			return nil, fmt.Errorf("agy binary not found: %w", err)
		}
	}

	return &AGYAdapter{binaryPath: bin}, nil
}

func (a *AGYAdapter) Name() string {
	return "agy"
}

func (a *AGYAdapter) Version() (string, error) {
	cmd := exec.Command(a.binaryPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("execute agy --version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *AGYAdapter) Execute(ctx context.Context, req ExecutionRequest) (*ExecutionResult, error) {
	if req.Prompt == "" {
		return nil, errors.New("prompt is required")
	}

	outputFormat := req.OutputFormat
	if outputFormat == "" {
		outputFormat = "json"
	}

	args := []string{
		"-p", req.Prompt,
		"--output-format", outputFormat,
		"--dangerously-skip-permissions",
	}

	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}

	cmdCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, a.binaryPath, args...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Prepare environment
	env := os.Environ()
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Setup process group for clean cancellation
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start agy process: %w", err)
	}

	pid := cmd.Process.Pid
	monitor := StartProcessMonitor(pid, 20*time.Millisecond)

	// Channel to signal process completion
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- cmd.Wait()
	}()

	var waitErr error
	var canceled bool
	var timedOut bool

	select {
	case <-ctx.Done():
		canceled = true
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			timedOut = true
		}
		// Terminate the entire process tree (PGID and all child subprocesses)
		if pid > 0 {
			_ = KillProcessTree(pid)
		}
		waitErr = <-doneChan

	case waitErr = <-doneChan:
		// Process completed naturally
	}

	duration := time.Since(startTime)
	peakRSS := monitor.Stop()

	// Redact secrets
	safeStdout := MaskSecrets(stdoutBuf.String(), req.Secrets)
	safeStderr := MaskSecrets(stderrBuf.String(), req.Secrets)

	result := &ExecutionResult{
		Duration:     duration,
		PeakRSSBytes: peakRSS,
		Stdout:       safeStdout,
		Stderr:       safeStderr,
	}

	if canceled {
		if timedOut {
			result.Status = "TIMEOUT"
			result.Error = "execution timed out"
		} else {
			result.Status = "CANCELED"
			result.Error = "execution canceled by context"
		}
		result.ExitCode = -1
		return result, nil
	}

	if waitErr != nil {
		result.Status = "FAILED"
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = waitErr.Error()
	} else {
		result.Status = "SUCCESS"
		result.ExitCode = 0
	}

	// Parse structured output if JSON format
	if outputFormat == "json" && len(safeStdout) > 0 {
		var structured StructuredOutput
		// In case output contains extra preamble or trailing lines, try to find JSON block
		trimmed := strings.TrimSpace(safeStdout)
		if idx := strings.Index(trimmed, "{"); idx != -1 {
			trimmed = trimmed[idx:]
		}
		if err := json.Unmarshal([]byte(trimmed), &structured); err == nil {
			result.StructuredOutput = &structured
			if structured.Status != "" && result.Status == "SUCCESS" {
				result.Status = structured.Status
			}
		}
	}

	return result, nil
}
