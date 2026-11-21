package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreflightSync_FastForwardClean(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return strings.TrimSpace(string(out))
	}

	// 1. Setup bare remote with dev branch
	run("init", "--bare", remote)
	seed := filepath.Join(root, "seed")
	run("init", "-b", "dev", seed)
	run("-C", seed, "config", "user.name", "Tester")
	run("-C", seed, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("commit 1"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Commit 1")
	run("-C", seed, "push", remote, "dev")

	// 2. Clone local repo from remote
	run("clone", "-b", "dev", remote, local)
	run("-C", local, "config", "user.name", "Tester")
	run("-C", local, "config", "user.email", "tester@example.com")

	// 3. Push a new commit to remote from seed
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("commit 2"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Commit 2")
	run("-C", seed, "push", remote, "dev")
	remoteSHA := run("-C", seed, "rev-parse", "HEAD")

	// 4. Verify local is still on commit 1 before sync
	localSHA := run("-C", local, "rev-parse", "HEAD")
	if localSHA == remoteSHA {
		t.Fatalf("local should be behind remote before sync")
	}

	// 5. Run PreflightSync
	client := NewClient()
	if err := client.PreflightSync(context.Background(), local, "dev", ""); err != nil {
		t.Fatalf("PreflightSync failed: %v", err)
	}

	// 6. Assert local dev was fast-forwarded to remoteSHA
	newLocalSHA := run("-C", local, "rev-parse", "HEAD")
	if newLocalSHA != remoteSHA {
		t.Fatalf("expected local SHA %s, got %s", remoteSHA, newLocalSHA)
	}
	content, _ := os.ReadFile(filepath.Join(local, "file.txt"))
	if string(content) != "commit 2" {
		t.Fatalf("expected file content 'commit 2', got '%s'", string(content))
	}
}

func TestPreflightSync_DirtyWorkingTree(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return strings.TrimSpace(string(out))
	}

	run("init", "--bare", remote)
	seed := filepath.Join(root, "seed")
	run("init", "-b", "dev", seed)
	run("-C", seed, "config", "user.name", "Tester")
	run("-C", seed, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("v1"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Init")
	run("-C", seed, "push", remote, "dev")

	run("clone", "-b", "dev", remote, local)
	initialSHA := run("-C", local, "rev-parse", "HEAD")

	// Make working tree dirty in local
	_ = os.WriteFile(filepath.Join(local, "dirty.txt"), []byte("uncommitted"), 0644)

	// Advance remote
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("v2"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Remote v2")
	run("-C", seed, "push", remote, "dev")

	// PreflightSync must return ErrWorkingTreeDirty safely without altering uncommitted changes
	client := NewClient()
	err := client.PreflightSync(context.Background(), local, "dev", "")
	if !errors.Is(err, ErrWorkingTreeDirty) {
		t.Fatalf("expected ErrWorkingTreeDirty, got: %v", err)
	}

	// Verify local HEAD untouched
	currentSHA := run("-C", local, "rev-parse", "HEAD")
	if currentSHA != initialSHA {
		t.Fatalf("local HEAD changed despite dirty working tree")
	}
	dirtyContent, _ := os.ReadFile(filepath.Join(local, "dirty.txt"))
	if string(dirtyContent) != "uncommitted" {
		t.Fatalf("uncommitted file was corrupted")
	}
}

func TestPreflightSync_BranchesDiverged(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return strings.TrimSpace(string(out))
	}

	run("init", "--bare", remote)
	seed := filepath.Join(root, "seed")
	run("init", "-b", "dev", seed)
	run("-C", seed, "config", "user.name", "Tester")
	run("-C", seed, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("v1"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Init")
	run("-C", seed, "push", remote, "dev")

	run("clone", "-b", "dev", remote, local)
	run("-C", local, "config", "user.name", "Tester")
	run("-C", local, "config", "user.email", "tester@example.com")

	// Commit on remote
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("remote commit"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Remote change")
	run("-C", seed, "push", remote, "dev")

	// Divergent commit on local
	_ = os.WriteFile(filepath.Join(local, "file.txt"), []byte("local commit"), 0644)
	run("-C", local, "add", "file.txt")
	run("-C", local, "commit", "-m", "Local change")
	localSHA := run("-C", local, "rev-parse", "HEAD")

	client := NewClient()
	err := client.PreflightSync(context.Background(), local, "dev", "")
	if !errors.Is(err, ErrBranchesDiverged) {
		t.Fatalf("expected ErrBranchesDiverged, got: %v", err)
	}

	// Verify local commit was preserved
	afterSHA := run("-C", local, "rev-parse", "HEAD")
	if afterSHA != localSHA {
		t.Fatalf("local commit changed after diverged sync")
	}
}

func TestTryAcquireVerificationLease(t *testing.T) {
	root := t.TempDir()
	actualGit, _ := exec.LookPath("git")
	_ = exec.Command(actualGit, "init", root).Run()

	ctx := context.Background()
	lease1, err := TryAcquireVerificationLease(ctx, root)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer lease1.Close()

	// Second acquire should return ErrLeaseBusy immediately
	lease2, err := TryAcquireVerificationLease(ctx, root)
	if !errors.Is(err, ErrLeaseBusy) {
		t.Fatalf("expected ErrLeaseBusy, got %v (lease: %v)", err, lease2)
	}

	// Close lease1, now acquire should succeed
	lease1.Close()
	lease3, err := TryAcquireVerificationLease(ctx, root)
	if err != nil {
		t.Fatalf("acquire after close failed: %v", err)
	}
	lease3.Close()
}
