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

// AGYProvider implements the Provider interface for the AGY / Antigravity CLI harness.
type AGYProvider struct {
	binaryPath string
}

// NewAGYProvider creates a new AGYProvider.
func NewAGYProvider() *AGYProvider {
	bin, _ := ResolveAGYBinaryPath()
	return &AGYProvider{binaryPath: bin}
}

func (p *AGYProvider) Name() string { return "Antigravity" }
func (p *AGYProvider) Type() string { return "agy" }

// VerifyConnection checks if the agy executable exists on the host.
func (p *AGYProvider) VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error) {
	bin := p.binaryPath
	if bin == "" {
		var err error
		bin, err = ResolveAGYBinaryPath()
		if err != nil {
			return &ConnectionProbeResult{
				Valid:        false,
				Provider:     "agy",
				ErrorMessage: "AGY binary not installed on this server",
				TestedAt:     time.Now().UTC(),
			}, nil
		}
	}
	return &ConnectionProbeResult{
		Valid:           true,
		Provider:        "agy",
		PlanHint:        "Antigravity CLI Harness",
		LatencyMs:       0,
		AvailableModels: DefaultAGYModels(),
		TestedAt:        time.Now().UTC(),
	}, nil
}

// ListModels probes available models from the native agy CLI or returns DefaultAGYModels.
func (p *AGYProvider) ListModels(ctx context.Context, credentials string) ([]ModelInfo, error) {
	bin := p.binaryPath
	if bin == "" {
		var err error
		bin, err = ResolveAGYBinaryPath()
		if err != nil {
			return DefaultAGYModels(), nil
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, bin, "models")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return DefaultAGYModels(), nil
	}

	models := ParseAGYModelsOutput(string(out))
	if len(models) == 0 {
		return DefaultAGYModels(), nil
	}
	return models, nil
}

// ParseAGYModelsOutput parses the stdout lines from `agy models`.
func ParseAGYModelsOutput(output string) []ModelInfo {
	var models []ModelInfo
	normalized := strings.ReplaceAll(output, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(trimmed, "Fetching available") || strings.Contains(trimmed, "Available models") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]
		// Skip spinner glyphs or invalid tokens
		if strings.HasPrefix(id, "⠋") || strings.HasPrefix(id, "⠙") || strings.HasPrefix(id, "⠹") || strings.HasPrefix(id, "•") {
			continue
		}

		displayName := id
		if len(fields) > 1 {
			displayName = strings.TrimSpace(trimmed[len(id):])
		}

		supportsVision := IsModelVisionCapable("agy", id)
		isDefault := id == "gemini-3.8-flash-high" || id == "gemini-1.5-pro"

		models = append(models, ModelInfo{
			ID:             id,
			Name:           id,
			DisplayName:    displayName,
			Description:    fmt.Sprintf("%s model via Antigravity harness", displayName),
			Provider:       "agy",
			IsDefault:      isDefault,
			SupportsVision: supportsVision,
			Capabilities: ModelCapabilities{
				Vision:         supportsVision,
				SupportsVision: supportsVision,
			},
		})
	}
	return models
}

// DefaultAGYModels returns standard models verified for the AGY harness.
func DefaultAGYModels() []ModelInfo {
	return []ModelInfo{
		{
			ID:             "gemini-3.8-flash-high",
			Name:           "gemini-3.8-flash-high",
			DisplayName:    "Gemini 3.8 Flash (High)",
			Description:    "Gemini 3.8 Flash with high thinking capacity",
			Provider:       "agy",
			IsDefault:      true,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "gemini-3.8-flash-medium",
			Name:           "gemini-3.8-flash-medium",
			DisplayName:    "Gemini 3.8 Flash (Medium)",
			Description:    "Gemini 3.8 Flash with medium thinking capacity",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "gemini-3.8-flash-low",
			Name:           "gemini-3.8-flash-low",
			DisplayName:    "Gemini 3.8 Flash (Low)",
			Description:    "Gemini 3.8 Flash with low thinking capacity",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "claude-sonnet-4-6",
			Name:           "claude-sonnet-4-6",
			DisplayName:    "Claude Sonnet 4.6 (Thinking)",
			Description:    "Frontier coding and vision reasoning with thinking",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "claude-opus-4-6-thinking",
			Name:           "claude-opus-4-6-thinking",
			DisplayName:    "Claude Opus 4.6 (Thinking)",
			Description:    "Deepest reasoning with vision and extended thinking",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "gpt-oss-120b-medium",
			Name:           "gpt-oss-120b-medium",
			DisplayName:    "GPT-OSS 120B (Medium)",
			Description:    "High-parameter open-weights coding model (Text-only)",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "gemini-2.0-flash",
			Name:           "gemini-2.0-flash",
			DisplayName:    "Gemini 2.0 Flash",
			Description:    "Next-generation ultra-fast multimodal model",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "gemini-1.5-pro",
			Name:           "gemini-1.5-pro",
			DisplayName:    "Gemini 1.5 Pro",
			Description:    "Multimodal reasoning with 2M token context",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "gemini-1.5-flash",
			Name:           "gemini-1.5-flash",
			DisplayName:    "Gemini 1.5 Flash",
			Description:    "Fast and versatile multimodal model",
			Provider:       "agy",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
	}
}

var _ Provider = (*AGYProvider)(nil)

