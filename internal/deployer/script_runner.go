package deployer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	ErrEmptyProjectDir     = errors.New("project directory cannot be empty")
	ErrProjectDirNotFound  = errors.New("project directory does not exist")
	ErrBuildStepFailed     = errors.New("build step failed")
	ErrBuildTimeout        = errors.New("build pipeline timed out")
	ErrBuildCancelled      = errors.New("build pipeline cancelled")
	ErrCommandNotFound     = errors.New("command not found in PATH")
)

// CommandExecutor abstracts command execution for testability and remote execution.
type CommandExecutor interface {
	Execute(ctx context.Context, dir string, env []string, stdout io.Writer, stderr io.Writer, name string, args ...string) error
}

// LocalCommandExecutor executes commands locally on the host with process group isolation.
type LocalCommandExecutor struct{}

// NewLocalCommandExecutor constructs a LocalCommandExecutor.
func NewLocalCommandExecutor() *LocalCommandExecutor {
	return &LocalCommandExecutor{}
}

// Execute runs the command locally with context cancellation, dir, env, and streaming output.
func (e *LocalCommandExecutor) Execute(ctx context.Context, dir string, env []string, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}

	// Set process group to allow killing entire child process tree on cancel
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}

	// Watch for context cancellation to kill process group cleanly
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-done
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%w: %s %s", ErrBuildTimeout, name, strings.Join(args, " "))
		}
		return fmt.Errorf("%w: %s %s", ErrBuildCancelled, name, strings.Join(args, " "))
	case err := <-done:
		if err != nil {
			return err
		}
		return nil
	}
}

// BuildStep defines a single command step in the build pipeline.
type BuildStep struct {
	Name        string
	Binary      string
	Args        []string
	IgnoreError bool
	Timeout     time.Duration
}

// BuildStepResult records the outcome of a single step.
type BuildStepResult struct {
	Name     string        `json:"name"`
	Command  string        `json:"command"`
	Success  bool          `json:"success"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

// ScriptBuildRequest holds parameters for running a script build pipeline.
type ScriptBuildRequest struct {
	ProjectID       string            `json:"project_id"`
	ProjectDir      string            `json:"project_dir"`
	Stack           string            `json:"stack,omitempty"` // laravel, node, python, html, etc.
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	InstallDeps     bool              `json:"install_deps"`
	RunBuild        bool              `json:"run_build"`
	RunMigrate      bool              `json:"run_migrate"`
	PostInstallCmds []string          `json:"post_install_cmds,omitempty"`
	Timeout         time.Duration     `json:"timeout,omitempty"`
	Streamer        *LogStreamer      `json:"-"`
}

// ScriptBuildResult holds overall summary and step breakdown of a build run.
type ScriptBuildResult struct {
	ProjectID      string            `json:"project_id"`
	Stack          string            `json:"stack"`
	Success        bool              `json:"success"`
	ExecutedSteps  []BuildStepResult `json:"executed_steps"`
	TotalDuration  time.Duration     `json:"total_duration"`
	Error          string            `json:"error,omitempty"`
}

// ScriptRunner coordinates isolated dependency execution, post-install commands, and error handling.
type ScriptRunner struct {
	executor       CommandExecutor
	defaultTimeout time.Duration
	mu             sync.RWMutex
}

// NewScriptRunner constructs a new ScriptRunner.
func NewScriptRunner(executors ...CommandExecutor) *ScriptRunner {
	var exec CommandExecutor = NewLocalCommandExecutor()
	if len(executors) > 0 && executors[0] != nil {
		exec = executors[0]
	}
	return &ScriptRunner{
		executor:       exec,
		defaultTimeout: 10 * time.Minute,
	}
}

// SetExecutor sets a custom CommandExecutor (e.g. for mock testing or remote SSH).
func (r *ScriptRunner) SetExecutor(e CommandExecutor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executor = e
}

// DetectStack inspects the project directory and returns the detected primary stack.
func DetectStack(dir string) string {
	if dir == "" {
		return "unknown"
	}

	// 1. PHP / Laravel
	if fileExists(filepath.Join(dir, "artisan")) || fileExists(filepath.Join(dir, "composer.json")) {
		return "laravel"
	}
	if fileExists(filepath.Join(dir, "index.php")) || fileExists(filepath.Join(dir, "public", "index.php")) {
		return "php"
	}

	// 2. Node.js / Frameworks
	if fileExists(filepath.Join(dir, "package.json")) {
		// Inspect package.json for Next or React or general Node
		content, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err == nil {
			str := string(content)
			if strings.Contains(str, `"next"`) {
				return "next"
			}
			if strings.Contains(str, `"react"`) {
				return "react"
			}
			if strings.Contains(str, `"express"`) {
				return "express"
			}
		}
		return "node"
	}

	// 3. Python
	if fileExists(filepath.Join(dir, "requirements.txt")) || fileExists(filepath.Join(dir, "manage.py")) || fileExists(filepath.Join(dir, "Pipfile")) {
		if fileExists(filepath.Join(dir, "manage.py")) {
			return "django"
		}
		return "python"
	}

	// 4. HTML / Static
	if fileExists(filepath.Join(dir, "index.html")) || fileExists(filepath.Join(dir, "public", "index.html")) {
		return "html"
	}

	return "static"
}

// RunPipeline runs the dependency installation and post-install command sequence.
func (r *ScriptRunner) RunPipeline(ctx context.Context, req ScriptBuildRequest) (*ScriptBuildResult, error) {
	startTime := time.Now()

	result := &ScriptBuildResult{
		ProjectID:     req.ProjectID,
		ExecutedSteps: make([]BuildStepResult, 0),
	}

	if strings.TrimSpace(req.ProjectDir) == "" {
		return nil, ErrEmptyProjectDir
	}

	info, err := os.Stat(req.ProjectDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrProjectDirNotFound, req.ProjectDir)
	}

	// Set timeout
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = r.defaultTimeout
	}
	pipelineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Detect stack if not explicitly provided
	stack := strings.ToLower(strings.TrimSpace(req.Stack))
	if stack == "" {
		stack = DetectStack(req.ProjectDir)
	}
	result.Stack = stack

	r.log(req.Streamer, StreamSystem, fmt.Sprintf("[BuildRunner] Starting pipeline for project %s (Stack: %s, Dir: %s)", req.ProjectID, stack, req.ProjectDir))

	// Build environment variable slice
	var envSlice []string
	for k, v := range req.EnvVars {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	// Assemble pipeline steps
	steps := r.buildSteps(req.ProjectDir, stack, req)

	r.mu.RLock()
	executor := r.executor
	r.mu.RUnlock()

	for _, step := range steps {
		stepStart := time.Now()
		stepCmdStr := fmt.Sprintf("%s %s", step.Binary, strings.Join(step.Args, " "))
		r.log(req.Streamer, StreamSystem, fmt.Sprintf("[BuildRunner] Step: %s -> %s", step.Name, stepCmdStr))

		// Step-level timeout
		stepTimeout := step.Timeout
		if stepTimeout <= 0 {
			stepTimeout = 5 * time.Minute
		}
		stepCtx, stepCancel := context.WithTimeout(pipelineCtx, stepTimeout)

		// Create streaming writers
		stdoutPipe := newLogStreamWriter(req.Streamer, StreamStdout)
		stderrPipe := newLogStreamWriter(req.Streamer, StreamStderr)

		execErr := executor.Execute(stepCtx, req.ProjectDir, envSlice, stdoutPipe, stderrPipe, step.Binary, step.Args...)
		stepCancel()

		duration := time.Since(stepStart)
		stepResult := BuildStepResult{
			Name:     step.Name,
			Command:  stepCmdStr,
			Duration: duration,
			Success:  execErr == nil,
		}

		if execErr != nil {
			stepResult.Error = execErr.Error()
			stepResult.ExitCode = 1
			if exitErr, ok := execErr.(*exec.ExitError); ok {
				stepResult.ExitCode = exitErr.ExitCode()
			}

			r.log(req.Streamer, StreamStderr, fmt.Sprintf("[BuildRunner] Step '%s' FAILED: %v (took %s)", step.Name, execErr, duration.Round(time.Millisecond)))
			result.ExecutedSteps = append(result.ExecutedSteps, stepResult)

			if !step.IgnoreError {
				result.Success = false
				result.TotalDuration = time.Since(startTime)
				result.Error = fmt.Sprintf("step '%s' failed: %v", step.Name, execErr)
				return result, fmt.Errorf("%w: %s: %v", ErrBuildStepFailed, step.Name, execErr)
			}
		} else {
			stepResult.ExitCode = 0
			r.log(req.Streamer, StreamSystem, fmt.Sprintf("[BuildRunner] Step '%s' completed successfully (took %s)", step.Name, duration.Round(time.Millisecond)))
			result.ExecutedSteps = append(result.ExecutedSteps, stepResult)
		}
	}

	result.Success = true
	result.TotalDuration = time.Since(startTime)
	r.log(req.Streamer, StreamSystem, fmt.Sprintf("[BuildRunner] Build pipeline finished successfully in %s", result.TotalDuration.Round(time.Millisecond)))

	return result, nil
}

// buildSteps determines the necessary dependency and post-install command list.
func (r *ScriptRunner) buildSteps(dir, stack string, req ScriptBuildRequest) []BuildStep {
	var steps []BuildStep

	switch stack {
	case "laravel", "php":
		if req.InstallDeps && fileExists(filepath.Join(dir, "composer.json")) {
			steps = append(steps, BuildStep{
				Name:    "Composer Install",
				Binary:  "composer",
				Args:    []string{"install", "--no-dev", "--no-interaction", "--prefer-dist", "--optimize-autoloader"},
				Timeout: 5 * time.Minute,
			})
		}

		// Artisan post-install steps
		if fileExists(filepath.Join(dir, "artisan")) {
			// Check if key:generate is needed
			appKey := req.EnvVars["APP_KEY"]
			if strings.TrimSpace(appKey) == "" {
				steps = append(steps, BuildStep{
					Name:        "Artisan Key Generate",
					Binary:      "php",
					Args:        []string{"artisan", "key:generate", "--force"},
					IgnoreError: true, // Non-fatal if already configured in file
					Timeout:     30 * time.Second,
				})
			}

			// Migration
			if req.RunMigrate {
				steps = append(steps, BuildStep{
					Name:        "Artisan Migrate",
					Binary:      "php",
					Args:        []string{"artisan", "migrate", "--force"},
					IgnoreError: false,
					Timeout:     2 * time.Minute,
				})
			}

			// Storage link (if public folder exists and public/storage is missing)
			if fileExists(filepath.Join(dir, "public")) && !fileExists(filepath.Join(dir, "public", "storage")) {
				steps = append(steps, BuildStep{
					Name:        "Artisan Storage Link",
					Binary:      "php",
					Args:        []string{"artisan", "storage:link"},
					IgnoreError: true,
					Timeout:     30 * time.Second,
				})
			}
		}

	case "node", "next", "react", "express":
		if req.InstallDeps && fileExists(filepath.Join(dir, "package.json")) {
			steps = append(steps, BuildStep{
				Name:    "NPM Install",
				Binary:  "npm",
				Args:    []string{"install", "--production"},
				Timeout: 5 * time.Minute,
			})
		}

		if req.RunBuild && fileExists(filepath.Join(dir, "package.json")) {
			// Check if build script exists in package.json
			content, err := os.ReadFile(filepath.Join(dir, "package.json"))
			if err == nil && strings.Contains(string(content), `"build"`) {
				steps = append(steps, BuildStep{
					Name:    "NPM Build",
					Binary:  "npm",
					Args:    []string{"run", "build"},
					Timeout: 5 * time.Minute,
				})
			}
		}

	case "python", "django", "flask":
		if req.InstallDeps && fileExists(filepath.Join(dir, "requirements.txt")) {
			steps = append(steps, BuildStep{
				Name:    "Pip Install",
				Binary:  "pip",
				Args:    []string{"install", "-r", "requirements.txt", "--no-cache-dir"},
				Timeout: 5 * time.Minute,
			})
		}

		if stack == "django" && req.RunMigrate && fileExists(filepath.Join(dir, "manage.py")) {
			steps = append(steps, BuildStep{
				Name:    "Django Migrate",
				Binary:  "python",
				Args:    []string{"manage.py", "migrate", "--noinput"},
				Timeout: 2 * time.Minute,
			})
		}

	case "html", "static":
		// Zero dependencies for static sites
		r.log(req.Streamer, StreamSystem, "[BuildRunner] Static HTML site detected; skipping dependency installation.")
	}

	// Add any custom post-install commands
	for i, cmdStr := range req.PostInstallCmds {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			continue
		}
		binary := parts[0]
		args := parts[1:]
		steps = append(steps, BuildStep{
			Name:    fmt.Sprintf("Post-Install Command #%d", i+1),
			Binary:  binary,
			Args:    args,
			Timeout: 2 * time.Minute,
		})
	}

	return steps
}

func (r *ScriptRunner) log(streamer *LogStreamer, stream, msg string) {
	if streamer != nil {
		streamer.Append(context.Background(), LogEntry{
			Stream:    stream,
			Message:   msg,
			Timestamp: time.Now().UTC(),
		})
	}
}

// logStreamWriter writes lines to LogStreamer while preserving output.
type logStreamWriter struct {
	streamer *LogStreamer
	stream   string
	buf      bytes.Buffer
}

func newLogStreamWriter(streamer *LogStreamer, stream string) *logStreamWriter {
	return &logStreamWriter{
		streamer: streamer,
		stream:   stream,
	}
}

func (w *logStreamWriter) Write(p []byte) (n int, err error) {
	if w.streamer == nil {
		return len(p), nil
	}

	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Not a full line yet, keep remainder in buffer
			w.buf.WriteString(line)
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			w.streamer.Append(context.Background(), LogEntry{
				Stream:    w.stream,
				Message:   line,
				Timestamp: time.Now().UTC(),
			})
		}
	}
	return len(p), nil
}
