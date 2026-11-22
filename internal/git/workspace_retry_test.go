package git

import (
	"context"
	"errors"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsGitIndexLockContention(t *testing.T) {
	cases := []struct {
		msg      string
		expected bool
	}{
		{"fatal: Unable to create '/repo/.git/index.lock': File exists.", true},
		{"Another git process seems to be running in this repository: index.lock locked", true},
		{"error: could not lock config file .git/config: File exists", false},
		{"fatal: destination path 'workspace' already exists and is not an empty directory.", false},
		{"error: Your local changes to the following files would be overwritten by checkout", false},
	}

	for _, c := range cases {
		got := isGitIndexLockContention(c.msg)
		if got != c.expected {
			t.Errorf("isGitIndexLockContention(%q) = %v; want %v", c.msg, got, c.expected)
		}
	}
}

func TestExecuteGitWithRetry_RecoversFromIndexLock(t *testing.T) {
	var attempts atomic.Int32

	cmdFn := func() *exec.Cmd {
		curr := attempts.Add(1)
		if curr <= 2 {
			// Simulate transient index.lock failure
			return exec.Command("bash", "-c", "echo 'fatal: Unable to create .git/index.lock: File exists' >&2; exit 1")
		}
		// Succeeded on 3rd attempt
		return exec.Command("echo", "success")
	}

	out, err := executeGitWithRetry(context.Background(), cmdFn, 3, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("expected command to succeed after retries, got err: %v, out: %s", err, string(out))
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", attempts.Load())
	}
}

func TestExecuteGitWithRetry_NonLockErrorDoesNotRetry(t *testing.T) {
	var attempts atomic.Int32

	cmdFn := func() *exec.Cmd {
		attempts.Add(1)
		// Non-lock error (regular git error)
		return exec.Command("bash", "-c", "echo 'fatal: branch not found' >&2; exit 1")
	}

	_, err := executeGitWithRetry(context.Background(), cmdFn, 3, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected command to fail")
	}
	// Should fail immediately without retrying
	if attempts.Load() != 1 {
		t.Fatalf("expected exactly 1 attempt on non-lock error, got %d", attempts.Load())
	}
}

func TestExecuteGitWithRetry_HonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cmdFn := func() *exec.Cmd {
		cancel() // Cancel during first attempt
		return exec.Command("bash", "-c", "echo 'fatal: Unable to create .git/index.lock: File exists' >&2; exit 1")
	}

	_, err := executeGitWithRetry(ctx, cmdFn, 3, 500*time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}
