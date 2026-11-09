// Package browserfetch runs an application-selected browser for one anonymous
// page inspection. It does not manage user logins or replace UI verification.
package browserfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const BodyLimit = 64 * 1024
const Timeout = 15 * time.Second

var ErrBusy = errors.New("browser inspection is busy; use HTTP retrieval or retry later")

// Runtime is selected by trusted Cuan configuration, never tool arguments.
// LockFile is shared across MCP processes to limit concurrent browser execution.
type Runtime struct {
	Binary   string `json:"binary"`
	LockFile string `json:"lock_file"`
	Sandbox  string `json:"sandbox"`
	// Optional application-managed glibc compatibility runtime for musl hosts.
	Loader     string `json:"loader"`
	LibraryDir string `json:"library_dir"`
}
type Result struct {
	URL       string `json:"url"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

type boundedOutput struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.buffer.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

// Fetch starts no persistent daemon, uses a fresh HOME/cache/storage directory,
// and releases the shared slot and temporary files on every return path.
func (r Runtime) Fetch(ctx context.Context, rawURL string) (*Result, error) {
	return r.fetch(ctx, rawURL, Timeout)
}
func (r Runtime) fetch(ctx context.Context, rawURL string, timeout time.Duration) (*Result, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("a public HTTP(S) URL without embedded credentials is required")
	}
	if !filepath.IsAbs(r.Binary) || !filepath.IsAbs(r.LockFile) {
		return nil, errors.New("browser runtime is not configured")
	}
	info, err := os.Stat(r.Binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, errors.New("browser executable is unavailable")
	}
	fd, err := syscall.Open(r.LockFile, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, errors.New("browser concurrency control is unavailable")
	}
	lock := os.NewFile(uintptr(fd), r.LockFile)
	defer lock.Close()
	info, err = lock.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("browser slot must be a regular file")
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, errors.New("browser concurrency control failed")
	}
	defer syscall.Flock(fd, syscall.LOCK_UN)
	home, err := os.MkdirTemp("", "cuan-browser-inspection-")
	if err != nil {
		return nil, errors.New("browser temporary workspace unavailable")
	}
	defer os.RemoveAll(home)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	binary, args, err := r.sandboxCommand(home, []string{"--v8-flags", "--max-old-space-size=128 --max-semi-space-size=4 --optimize-for-size", "fetch", rawURL, "--dump", "text", "--timeout", "10", "--storage-dir", "/home/browser/storage"})
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = home
	cmd.Env = []string{"HOME=" + home, "XDG_CACHE_HOME=" + filepath.Join(home, "cache"), "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "TMPDIR=" + home, "PATH=/usr/bin:/bin", "LANG=C.UTF-8", "OBSCURA_ALLOW_PRIVATE_NETWORK=0"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	killGroup := func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.Cancel = killGroup
	cmd.WaitDelay = 200 * time.Millisecond
	output := &boundedOutput{limit: BodyLimit}
	stderr := &boundedOutput{limit: 4096}
	cmd.Stdout = output
	cmd.Stderr = stderr
	err = cmd.Run()
	// Reap remaining same-group descendants even if the leader exited first.
	if cmd.Process != nil {
		_ = killGroup()
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("browser inspection stopped: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("browser inspection failed (%s); no rendered content was verified", browserFailureCode(stderr.buffer.String()))
	}
	return &Result{URL: rawURL, Content: output.buffer.String(), Truncated: output.truncated}, nil
}

// Preserve useful native failure classification without returning arbitrary
// stderr, page text, request URLs or environment data to the coordinator.
func browserFailureCode(stderr string) string {
	if strings.Contains(stderr, "bwrap:") {
		if (strings.Contains(stderr, "setting up uid map") || strings.Contains(stderr, "setting up gid map")) && strings.Contains(stderr, "Read-only file system") {
			return "sandbox_userns_maps_readonly"
		}
		if strings.Contains(stderr, "Creating new namespace failed") || strings.Contains(stderr, "No permissions to create new namespace") {
			return "sandbox_namespace_unavailable"
		}
		return "sandbox_setup_failed"
	}
	return "engine_failed"
}
