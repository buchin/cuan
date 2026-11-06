package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestGoBuildLeaseProtectsActiveExecutionAndDeadline(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "go-build")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireGoBuildLease(context.Background(), cache)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := os.OpenFile(filepath.Join(filepath.Dir(cache), ".build-cache.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	if err = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("maintenance bypassed active lease", err)
	}
	second, err := acquireGoBuildLease(context.Background(), cache)
	if err != nil {
		t.Fatal("concurrent execution blocked", err)
	}
	lease.Close()
	second.Close()
	if err = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err = acquireGoBuildLease(ctx, cache); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("lease ignored execution deadline", err)
	}
	syscall.Flock(int(gate.Fd()), syscall.LOCK_UN)
	recovered, err := acquireGoBuildLease(context.Background(), cache)
	if err != nil {
		t.Fatal(err)
	}
	recovered.Close()
}
func TestGoBuildLeaseRejectsLinkedGate(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "go-build")
	os.Mkdir(cache, 0700)
	target := filepath.Join(root, "credentials")
	os.WriteFile(target, []byte("preserved"), 0600)
	os.Symlink(target, filepath.Join(root, ".build-cache.lock"))
	if lease, err := acquireGoBuildLease(context.Background(), cache); err == nil {
		lease.Close()
		t.Fatal("accepted linked gate")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "preserved" {
		t.Fatal("credentials changed")
	}
}
