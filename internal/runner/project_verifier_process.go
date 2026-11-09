package runner

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

	"github.com/buchin/agentrunner"
)

const projectVerifierOutputLimit = 12000

type projectVerifierBuildEnvironmentKey struct{}

// Native checks must reuse the build caches already leased for this execution,
// rather than falling back to the supervisor's unrelated HOME. Carry only build
// settings: task/provider credentials and project secrets are not inherited here.
func withProjectVerifierBuildEnvironment(ctx context.Context, settings map[string]string) context.Context {
	build := make(map[string]string)
	for _, key := range []string{"GOPATH", "GOMODCACHE", "GOCACHE", "GOENV", "GOTOOLCHAIN", "NPM_CONFIG_CACHE", "COMPOSER_CACHE_DIR"} {
		if value := settings[key]; value != "" {
			build[key] = value
		}
	}
	return context.WithValue(ctx, projectVerifierBuildEnvironmentKey{}, build)
}

func projectVerifierBuildEnvironment(ctx context.Context) []string {
	build, _ := ctx.Value(projectVerifierBuildEnvironmentKey{}).(map[string]string)
	if len(build) == 0 {
		return nil
	}
	environment := make([]string, 0, len(os.Environ())+len(build))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := build[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range build {
		environment = append(environment, key+"="+value)
	}
	return environment
}

// Keep the diagnostic tail bounded while a verifier is running, including
// simultaneous stdout/stderr. A noisy child cannot grow the runner's heap.
type verifierOutputTail struct {
	mu   sync.Mutex
	data []byte
}

func (b *verifierOutputTail) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if b.data == nil {
		b.data = make([]byte, 0, projectVerifierOutputLimit)
	}
	if len(p) >= projectVerifierOutputLimit {
		b.data = append(b.data[:0], p[len(p)-projectVerifierOutputLimit:]...)
	} else {
		if excess := len(b.data) + len(p) - projectVerifierOutputLimit; excess > 0 {
			copy(b.data, b.data[excess:])
			b.data = b.data[:len(b.data)-excess]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func (b *verifierOutputTail) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }

func runProjectVerifierCommand(parent context.Context, root, command string, args ...string) verificationResult {
	return runProjectVerifierCommandEnvironment(parent, root, projectVerifierBuildEnvironment(parent), command, args...)
}

func runProjectVerifierCommandEnvironment(parent context.Context, root string, environment []string, command string, args ...string) verificationResult {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = root
	cmd.Env = environment
	// A repository script normally spawns test/compiler children. Killing only
	// the shell leaves those children alive and their inherited pipes open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return agentrunner.KillProcessTree(cmd.Process.Pid)
	}
	cmd.WaitDelay = 500 * time.Millisecond
	var output verifierOutputTail
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	// Also clean up descendants left behind by an exited verifier script.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	summary := fmt.Sprintf("Project verification: %s %s (exit %d)\n%s", filepath.Base(command), strings.Join(args, " "), exitCode, strings.TrimSpace(output.String()))
	if err != nil {
		if exitCode == 0 {
			summary = strings.Replace(summary, "Project verification:", "Project verification incomplete:", 1)
		}
		reason := "Project verification failed"
		switch {
		case errors.Is(parent.Err(), context.DeadlineExceeded):
			reason = "Project verification stopped at the task deadline"
		case errors.Is(parent.Err(), context.Canceled):
			reason = "Project verification canceled"
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			reason = "Project verification timed out after 5 minutes"
		case errors.Is(err, exec.ErrWaitDelay):
			reason = "Project verifier exited without closing child output"
		case cmd.ProcessState == nil:
			reason = "Project verifier could not start"
		}
		return verificationResult{Summary: summary, Error: reason}
	}
	return verificationResult{Passed: true, Summary: summary}
}
