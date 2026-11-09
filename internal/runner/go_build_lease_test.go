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
	lease, err := acquireBuildCacheLease(context.Background(), cache)
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
	second, err := acquireBuildCacheLease(context.Background(), cache)
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
	if _, err = acquireBuildCacheLease(ctx, cache); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("lease ignored execution deadline", err)
	}
	syscall.Flock(int(gate.Fd()), syscall.LOCK_UN)
	recovered, err := acquireBuildCacheLease(context.Background(), cache)
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
	if lease, err := acquireBuildCacheLease(context.Background(), cache); err == nil {
		lease.Close()
		t.Fatal("accepted linked gate")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "preserved" {
		t.Fatal("credentials changed")
	}
}

func TestReleaseBuildCacheLeaseWritesMetadata(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "go-build")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireBuildCacheLease(context.Background(), cache)
	if err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Unix(1731219116, 0)
	if err := releaseBuildCacheLeaseTime(lease, cache, fixedTime); err != nil {
		t.Fatal(err)
	}
	gatePath := filepath.Join(root, ".build-cache.lock")
	data, err := os.ReadFile(gatePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"last_used_at":1731219116,"version":1}` {
		t.Fatalf("unexpected metadata content: %s", string(data))
	}
}

func TestReleaseBuildCacheLeasePreservesLinkedAndForeignFilesWithoutMutation(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "go-build")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}

	// 1. Non-Go cache (e.g. npm-cache) must not write metadata
	npmCache := filepath.Join(root, "npm-cache")
	if err := os.Mkdir(npmCache, 0700); err != nil {
		t.Fatal(err)
	}
	npmLease, err := acquireBuildCacheLease(context.Background(), npmCache)
	if err != nil {
		t.Fatal(err)
	}
	npmGate := filepath.Join(root, ".build-cache.lock")
	if err := os.WriteFile(npmGate, []byte("original-npm-gate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := releaseBuildCacheLease(npmLease, npmCache); err != nil {
		t.Fatal(err)
	}
	npmContent, _ := os.ReadFile(npmGate)
	if string(npmContent) != "original-npm-gate" {
		t.Fatalf("npm gate mutated: %s", string(npmContent))
	}

	// 2. Hardlinked gate (st_nlink > 1) must not be mutated
	hardlinkRoot := t.TempDir()
	hardlinkCache := filepath.Join(hardlinkRoot, "go-build")
	os.Mkdir(hardlinkCache, 0700)
	hardlinkGate := filepath.Join(hardlinkRoot, ".build-cache.lock")
	os.WriteFile(hardlinkGate, []byte("hardlink-gate-data"), 0600)
	alias := filepath.Join(hardlinkRoot, "hardlink-alias")
	if err := os.Link(hardlinkGate, alias); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(hardlinkGate, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseBuildCacheLease(f, hardlinkCache); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(hardlinkGate)
	if string(data) != "hardlink-gate-data" {
		t.Fatalf("hardlinked gate was mutated: %s", string(data))
	}
}
