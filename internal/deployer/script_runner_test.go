package deployer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockCommandExecutor records commands executed and returns pre-configured outcomes.
type MockCommandExecutor struct {
	mu           sync.Mutex
	ExecutedCmds []string
	FailOnStep   string
	FailError    error
	Delay        time.Duration
	OutputLines  map[string][]string
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		ExecutedCmds: make([]string, 0),
		OutputLines:  make(map[string][]string),
	}
}

func (m *MockCommandExecutor) Execute(ctx context.Context, dir string, env []string, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	m.mu.Lock()
	cmdStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	m.ExecutedCmds = append(m.ExecutedCmds, cmdStr)
	failOn := m.FailOnStep
	failErr := m.FailError
	delay := m.Delay
	lines := m.OutputLines[name]
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if stdout != nil {
		for _, l := range lines {
			_, _ = stdout.Write([]byte(l + "\n"))
		}
		_, _ = stdout.Write([]byte(fmt.Sprintf("Mock execution of %s succeeded\n", name)))
	}

	if failOn != "" && strings.Contains(cmdStr, failOn) {
		if stderr != nil {
			_, _ = stderr.Write([]byte(fmt.Sprintf("Mock error executing %s\n", name)))
		}
		if failErr != nil {
			return failErr
		}
		return errors.New("command failed with mock error")
	}

	return nil
}

func TestDetectStack(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Static HTML
	htmlDir := filepath.Join(tmpDir, "html_proj")
	_ = os.MkdirAll(htmlDir, 0755)
	_ = os.WriteFile(filepath.Join(htmlDir, "index.html"), []byte("<h1>Hello</h1>"), 0644)
	if stack := DetectStack(htmlDir); stack != "html" {
		t.Fatalf("expected stack 'html', got '%s'", stack)
	}

	// 2. Laravel
	laravelDir := filepath.Join(tmpDir, "laravel_proj")
	_ = os.MkdirAll(laravelDir, 0755)
	_ = os.WriteFile(filepath.Join(laravelDir, "composer.json"), []byte(`{"name":"laravel/laravel"}`), 0644)
	_ = os.WriteFile(filepath.Join(laravelDir, "artisan"), []byte("#!/usr/bin/env php"), 0755)
	if stack := DetectStack(laravelDir); stack != "laravel" {
		t.Fatalf("expected stack 'laravel', got '%s'", stack)
	}

	// 3. Node.js / React
	nodeDir := filepath.Join(tmpDir, "node_proj")
	_ = os.MkdirAll(nodeDir, 0755)
	_ = os.WriteFile(filepath.Join(nodeDir, "package.json"), []byte(`{"dependencies":{"react":"^18.0.0"}}`), 0644)
	if stack := DetectStack(nodeDir); stack != "react" {
		t.Fatalf("expected stack 'react', got '%s'", stack)
	}

	// 4. Python
	pyDir := filepath.Join(tmpDir, "py_proj")
	_ = os.MkdirAll(pyDir, 0755)
	_ = os.WriteFile(filepath.Join(pyDir, "requirements.txt"), []byte("flask>=2.0\n"), 0644)
	if stack := DetectStack(pyDir); stack != "python" {
		t.Fatalf("expected stack 'python', got '%s'", stack)
	}
}

func TestScriptRunner_LaravelPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(`{"name":"laravel/app"}`), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "artisan"), []byte("#!/usr/bin/env php"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "public"), 0755)

	mockExec := NewMockCommandExecutor()
	runner := NewScriptRunner(mockExec)

	streamer := NewLogStreamer("test_laravel", nil, 100)

	req := ScriptBuildRequest{
		ProjectID:   "proj_laravel_1",
		ProjectDir:  tmpDir,
		Stack:       "laravel",
		InstallDeps: true,
		RunMigrate:  true,
		Streamer:    streamer,
	}

	result, err := runner.RunPipeline(context.Background(), req)
	if err != nil {
		t.Fatalf("expected pipeline to succeed, got: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected result.Success to be true")
	}

	if len(result.ExecutedSteps) < 3 {
		t.Fatalf("expected at least 3 steps, got %d", len(result.ExecutedSteps))
	}

	// Verify executed commands
	foundComposer := false
	foundKeyGen := false
	foundMigrate := false

	for _, cmd := range mockExec.ExecutedCmds {
		if strings.Contains(cmd, "composer install") {
			foundComposer = true
		}
		if strings.Contains(cmd, "artisan key:generate") {
			foundKeyGen = true
		}
		if strings.Contains(cmd, "artisan migrate") {
			foundMigrate = true
		}
	}

	if !foundComposer {
		t.Errorf("expected 'composer install' command to run")
	}
	if !foundKeyGen {
		t.Errorf("expected 'artisan key:generate' command to run")
	}
	if !foundMigrate {
		t.Errorf("expected 'artisan migrate' command to run")
	}

	// Check streamer received logs
	recent := streamer.GetRecent(50)
	if len(recent) == 0 {
		t.Errorf("expected streamer to capture log entries")
	}
}

func TestScriptRunner_NodePipeline(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0644)

	mockExec := NewMockCommandExecutor()
	runner := NewScriptRunner(mockExec)

	streamer := NewLogStreamer("test_node", nil, 100)

	req := ScriptBuildRequest{
		ProjectID:   "proj_node_1",
		ProjectDir:  tmpDir,
		Stack:       "node",
		InstallDeps: true,
		RunBuild:    true,
		Streamer:    streamer,
	}

	result, err := runner.RunPipeline(context.Background(), req)
	if err != nil {
		t.Fatalf("expected node pipeline to succeed, got: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected result.Success to be true")
	}

	foundInstall := false
	foundBuild := false
	for _, cmd := range mockExec.ExecutedCmds {
		if strings.Contains(cmd, "npm install") {
			foundInstall = true
		}
		if strings.Contains(cmd, "npm run build") {
			foundBuild = true
		}
	}

	if !foundInstall {
		t.Errorf("expected 'npm install' to run")
	}
	if !foundBuild {
		t.Errorf("expected 'npm run build' to run")
	}
}

func TestScriptRunner_PythonPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("flask\n"), 0644)

	mockExec := NewMockCommandExecutor()
	runner := NewScriptRunner(mockExec)

	req := ScriptBuildRequest{
		ProjectID:   "proj_py_1",
		ProjectDir:  tmpDir,
		Stack:       "python",
		InstallDeps: true,
	}

	result, err := runner.RunPipeline(context.Background(), req)
	if err != nil {
		t.Fatalf("expected python pipeline to succeed, got: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected result.Success to be true")
	}

	foundPip := false
	for _, cmd := range mockExec.ExecutedCmds {
		if strings.Contains(cmd, "pip install") {
			foundPip = true
		}
	}
	if !foundPip {
		t.Errorf("expected 'pip install' to run")
	}
}

func TestScriptRunner_StaticSite(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<h1>Welcome</h1>"), 0644)

	mockExec := NewMockCommandExecutor()
	runner := NewScriptRunner(mockExec)

	req := ScriptBuildRequest{
		ProjectID:   "proj_static_1",
		ProjectDir:  tmpDir,
		Stack:       "html",
		InstallDeps: true,
	}

	result, err := runner.RunPipeline(context.Background(), req)
	if err != nil {
		t.Fatalf("expected static site to succeed, got: %v", err)
	}

	if !result.Success {
		t.Fatalf("expected success for static site")
	}

	if len(mockExec.ExecutedCmds) != 0 {
		t.Errorf("expected 0 commands executed for static site, got %d", len(mockExec.ExecutedCmds))
	}
}

func TestScriptRunner_TimeoutHandling(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(`{}`), 0644)

	mockExec := NewMockCommandExecutor()
	mockExec.Delay = 500 * time.Millisecond // simulate long running install

	runner := NewScriptRunner(mockExec)

	req := ScriptBuildRequest{
		ProjectID:   "proj_timeout_1",
		ProjectDir:  tmpDir,
		Stack:       "laravel",
		InstallDeps: true,
		Timeout:     50 * time.Millisecond, // very short timeout
	}

	result, err := runner.RunPipeline(context.Background(), req)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	if result == nil || result.Success {
		t.Fatalf("expected result.Success to be false on timeout")
	}
}

func TestScriptRunner_StepFailure(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(`{}`), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "artisan"), []byte(`test`), 0755)

	mockExec := NewMockCommandExecutor()
	mockExec.FailOnStep = "composer install"
	mockExec.FailError = errors.New("network error: package repository unreachable")

	runner := NewScriptRunner(mockExec)

	req := ScriptBuildRequest{
		ProjectID:   "proj_fail_1",
		ProjectDir:  tmpDir,
		Stack:       "laravel",
		InstallDeps: true,
		RunMigrate:  true,
	}

	result, err := runner.RunPipeline(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error on failed step, got nil")
	}

	if !errors.Is(err, ErrBuildStepFailed) {
		t.Errorf("expected ErrBuildStepFailed, got %v", err)
	}

	if result.Success {
		t.Errorf("expected result.Success to be false")
	}

	// Verify that migrate was not run after composer failed
	for _, cmd := range mockExec.ExecutedCmds {
		if strings.Contains(cmd, "artisan migrate") {
			t.Errorf("expected artisan migrate NOT to run after composer failure")
		}
	}
}
