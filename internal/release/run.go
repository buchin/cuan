package release

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/agentrunner"
	"github.com/buchin/cuan/internal/git"
)

const outputLimit = 12000

type outputTail struct {
	mu   sync.Mutex
	data []byte
}

func (b *outputTail) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if n >= outputLimit {
		b.data = append(b.data[:0], p[n-outputLimit:]...)
	} else {
		if excess := len(b.data) + n - outputLimit; excess > 0 {
			b.data = append(b.data[:0], b.data[excess:]...)
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

type Result struct {
	Commit          string    `json:"commit"`
	Branch          string    `json:"branch"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	ExitCode        int       `json:"exit_code"`
	SourceUnchanged bool      `json:"source_unchanged"`
	Error           string    `json:"error,omitempty"`
	output          string
}

// Diagnostic requires an explicit redactor; raw process output is never JSON.
func (r Result) Diagnostic(redact func(string) string) string {
	if redact == nil {
		return ""
	}
	return redact(r.output)
}

// Run never retries, merges, accepts a task or claims that production is healthy.
// Native callers must supply their own owner-scoped environment explicitly.
func Run(parent context.Context, p *Plan, environment []string) Result {
	return RunWithProgress(parent, p, environment, nil)
}

// RunWithProgress reports allowlisted script milestones without forwarding raw
// output. Milestones never replace the returned execution facts or verification.
func RunWithProgress(parent context.Context, p *Plan, environment []string, observe func(string)) Result {
	result := Result{ExitCode: -1}
	if p == nil || environment == nil {
		result.Error = "release plan and explicit environment are required"
		return result
	}
	if err := CheckRuntime(p, environment); err != nil {
		result.Error = err.Error()
		return result
	}
	result.Commit, result.Branch = p.commit, p.branch
	ctx, cancel := context.WithTimeout(parent, time.Duration(p.spec.TimeoutSeconds)*time.Second)
	defer cancel()
	lease, err := git.AcquireVerificationLease(ctx, p.root)
	if err != nil {
		result.Error = "release workspace is unavailable or already in use"
		return result
	}
	defer lease.Close()
	if err := source(ctx, p.root, p.commit, p.branch); err != nil {
		result.Error = err.Error()
		return result
	}
	file, err := os.Open(filepath.Join(p.root, ManifestPath))
	if err != nil {
		result.Error = "release descriptor is unavailable"
		return result
	}
	raw, err := io.ReadAll(io.LimitReader(file, 16385))
	file.Close()
	if err != nil || len(raw) > 16384 || sha256.Sum256(raw) != p.manifestHash {
		result.Error = "release descriptor changed after planning"
		return result
	}
	if err = trackedFile(ctx, p.root, p.spec.Command[0], true); err != nil {
		result.Error = err.Error()
		return result
	}
	cmd := exec.CommandContext(ctx, filepath.Join(p.root, p.spec.Command[0]), p.spec.Command[1:]...)
	cmd.Dir = p.root
	cmd.Env = append([]string{}, environment...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return agentrunner.KillProcessTree(cmd.Process.Pid)
	}
	cmd.WaitDelay = 500 * time.Millisecond
	var output outputTail
	writer := io.MultiWriter(&output, &progressWriter{observe: observe})
	cmd.Stdout = writer
	cmd.Stderr = writer
	result.StartedAt = time.Now().UTC()
	err = cmd.Run()
	result.FinishedAt = time.Now().UTC()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	result.output = string(output.data)
	if err != nil {
		if result.ExitCode > 0 {
			result.Error = fmt.Sprintf("release process exited with code %d", result.ExitCode)
		} else {
			result.Error = "release process did not complete"
		}
		if ctx.Err() != nil {
			result.Error = "release process canceled or deadline reached"
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			result.Error = "release process left child output open"
		}
	}
	check, stop := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer stop()
	result.SourceUnchanged = source(check, p.root, p.commit, p.branch) == nil
	if !result.SourceUnchanged && result.Error == "" {
		result.Error = "release process changed its tracked source; production state must be reconciled"
	}
	return result
}
