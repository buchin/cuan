package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
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
		outputFormat = "stream-json"
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

	// Set up pipes for live streaming and interactive steering
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open stderr pipe: %w", err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdin pipe: %w", err)
	}

	stdoutRing := NewRingBuffer(512 * 1024)
	stderrRing := NewRingBuffer(512 * 1024)

	var streamWg sync.WaitGroup
	streamWg.Add(2)

	// Stream stdout line-by-line in real-time
	go func() {
		defer streamWg.Done()
		reader := bufio.NewReaderSize(stdoutPipe, 32*1024)
		for {
			line, rErr := reader.ReadString('\n')
			if len(line) > 0 {
				_, _ = stdoutRing.Write([]byte(line))
				clean := strings.TrimRight(line, "\r\n")
				safe := MaskSecrets(clean, req.Secrets)
				if req.OnLog != nil {
					req.OnLog("stdout", safe)
				}
				if req.OnStdoutLine != nil {
					req.OnStdoutLine(safe)
				}
			}
			if rErr != nil {
				break
			}
		}
	}()

	// Stream stderr line-by-line in real-time
	go func() {
		defer streamWg.Done()
		reader := bufio.NewReaderSize(stderrPipe, 32*1024)
		for {
			line, rErr := reader.ReadString('\n')
			if len(line) > 0 {
				_, _ = stderrRing.Write([]byte(line))
				clean := strings.TrimRight(line, "\r\n")
				safe := MaskSecrets(clean, req.Secrets)
				if req.OnLog != nil {
					req.OnLog("stderr", safe)
				}
				if req.OnStderrLine != nil {
					req.OnStderrLine(safe)
				}
			}
			if rErr != nil {
				break
			}
		}
	}()

	// Provide interactive steering handle to stdin
	var stdinMu sync.Mutex
	var stdinClosed bool
	safeSteer := func(prompt string) error {
		stdinMu.Lock()
		defer stdinMu.Unlock()
		if stdinClosed || stdinPipe == nil {
			return errors.New("agent process stdin is not active or closed")
		}
		_, wErr := io.WriteString(stdinPipe, prompt+"\n")
		return wErr
	}

	if req.RegisterSteer != nil {
		req.RegisterSteer(safeSteer)
	}

	if req.SteerChan != nil {
		go func() {
			for {
				select {
				case <-cmdCtx.Done():
					return
				case prompt, ok := <-req.SteerChan:
					if !ok {
						return
					}
					_ = safeSteer(prompt)
				}
			}
		}()
	}

	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		stdinMu.Lock()
		stdinClosed = true
		_ = stdinPipe.Close()
		stdinMu.Unlock()
		return nil, fmt.Errorf("start agy process: %w", err)
	}

	pid := cmd.Process.Pid
	monitor := StartProcessMonitor(pid, 20*time.Millisecond)

	// Channel to signal process completion
	doneChan := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		streamWg.Wait() // ensure all streaming output has been read
		doneChan <- waitErr
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
		// Close stdin and terminate the entire process tree (PGID and all child subprocesses)
		stdinMu.Lock()
		stdinClosed = true
		if stdinPipe != nil {
			_ = stdinPipe.Close()
		}
		stdinMu.Unlock()

		if pid > 0 {
			_ = KillProcessTree(pid)
		}
		waitErr = <-doneChan

	case waitErr = <-doneChan:
		// Process completed naturally
		stdinMu.Lock()
		stdinClosed = true
		if stdinPipe != nil {
			_ = stdinPipe.Close()
		}
		stdinMu.Unlock()
	}

	duration := time.Since(startTime)
	peakRSS := monitor.Stop()

	// Redact secrets from bounded ring-buffers
	safeStdout := MaskSecrets(stdoutRing.String(), req.Secrets)
	safeStderr := MaskSecrets(stderrRing.String(), req.Secrets)

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

	// Parse structured output if JSON or stream-json format
	if (outputFormat == "json" || outputFormat == "stream-json") && len(safeStdout) > 0 {
		var foundResult bool
		// In stream-json mode, scan backwards for the final {"event":"result","result":{...}} chunk
		lines := strings.Split(safeStdout, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if strings.Contains(line, `"event":"result"`) || strings.Contains(line, `"result"`) {
				var streamEv struct {
					Event  string           `json:"event"`
					Result StructuredOutput `json:"result"`
				}
				if err := json.Unmarshal([]byte(line), &streamEv); err == nil && streamEv.Result.Status != "" {
					result.StructuredOutput = &streamEv.Result
					if streamEv.Result.Status != "" && result.Status == "SUCCESS" {
						result.Status = streamEv.Result.Status
					}
					foundResult = true
					break
				}
			}
		}

		if !foundResult {
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

