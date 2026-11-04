package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func snapshotFixture(t *testing.T) (source, oldTask, sourceCommit, targetCommit string, run func(string, ...string) string) {
	t.Helper()
	root := t.TempDir()
	source = filepath.Join(root, "project-cache")
	createTestBaseRepo(t, source)
	run = func(directory string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", directory}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	sourceCommit = run(source, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(source, "feature.txt"), []byte("LIVE_RESULT"), 0600); err != nil {
		t.Fatal(err)
	}
	run(source, "add", ".")
	run(source, "commit", "-m", "Live result")
	targetCommit = run(source, "rev-parse", "HEAD")
	oldTask = filepath.Join(root, "old-task")
	run(source, "clone", "--shared", source, oldTask)
	run(oldTask, "checkout", "--detach", sourceCommit)
	for _, directory := range []string{source, oldTask} {
		if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte("PRESERVE_PENDING"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "pending.txt"), []byte("UNTRACKED_WORK"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run(oldTask, "add", "README.md")
	return
}

func TestVerificationSnapshotPreservesCacheAndOriginalTask(t *testing.T) {
	source, oldTask, original, live, run := snapshotFixture(t)
	cacheState := run(source, "status", "--porcelain=v1")
	taskState := run(oldTask, "status", "--porcelain=v1")
	refs := run(source, "for-each-ref", "--format=%(refname):%(objectname)")
	cacheIndex, err := os.ReadFile(filepath.Join(source, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	taskIndex, err := os.ReadFile(filepath.Join(oldTask, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "audit")
	client := NewClient()
	path, err := client.PrepareVerificationSnapshot(context.Background(), source, destination, original, live)
	if err != nil {
		t.Fatal(err)
	}
	if actual, err := os.ReadFile(filepath.Join(source, ".git", "index")); err != nil || !bytes.Equal(actual, cacheIndex) {
		t.Fatal("snapshot changed the source index", err)
	}
	if actual, err := os.ReadFile(filepath.Join(oldTask, ".git", "index")); err != nil || !bytes.Equal(actual, taskIndex) {
		t.Fatal("snapshot changed the saved task index", err)
	}
	if run(path, "rev-parse", "HEAD") != live || run(path, "remote") != "" {
		t.Fatal("snapshot followed the wrong revision or retained a publication remote")
	}
	if _, err := os.Stat(filepath.Join(path, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Fatal("snapshot still depends on the cache object's lifetime")
	}
	if err := filepath.WalkDir(filepath.Join(path, ".git", "objects"), func(object string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(filepath.Join(path, ".git", "objects"), object)
		if err != nil {
			return err
		}
		original, originalErr := os.Stat(filepath.Join(source, ".git", "objects", rel))
		copy, copyErr := os.Stat(object)
		if originalErr == nil && copyErr == nil && os.SameFile(original, copy) {
			t.Fatal("snapshot shares mutable cache object files")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(path, "feature.txt")); err != nil || string(data) != "LIVE_RESULT" {
		t.Fatal("snapshot did not contain the committed result", err)
	}
	if run(source, "status", "--porcelain=v1") != cacheState || run(oldTask, "status", "--porcelain=v1") != taskState || run(source, "for-each-ref", "--format=%(refname):%(objectname)") != refs {
		t.Fatal("verification changed existing source/task files, index or refs")
	}
	for _, directory := range []string{source, oldTask} {
		if data, err := os.ReadFile(filepath.Join(directory, "README.md")); err != nil || string(data) != "PRESERVE_PENDING" {
			t.Fatal("verification replaced saved work", err)
		}
	}
	if repeated, err := client.PrepareVerificationSnapshot(context.Background(), source, destination, original, live); err != nil || repeated != path {
		t.Fatal("restart did not reuse the exact owned snapshot", err)
	}
	if _, err := client.PrepareVerificationSnapshot(context.Background(), source, destination, original, original); err == nil || run(path, "rev-parse", "HEAD") != live {
		t.Fatal("a changed target rewound the existing audit")
	}
	if err := os.WriteFile(filepath.Join(path, "pending.txt"), []byte("KEEP_NEW_AUDIT_WORK"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PrepareVerificationSnapshot(context.Background(), source, destination, original, live); err == nil {
		t.Fatal("dirty audit was silently reset")
	}
	if data, err := os.ReadFile(filepath.Join(path, "pending.txt")); err != nil || string(data) != "KEEP_NEW_AUDIT_WORK" {
		t.Fatal("dirty audit work was lost", err)
	}
}

func TestVerificationSnapshotRejectsUnrelatedAndUnownedTargets(t *testing.T) {
	source, _, original, live, _ := snapshotFixture(t)
	client := NewClient()
	for _, scenario := range []string{"wrong ancestry", "short revision", "unowned directory", "source directory", "parent aliases source"} {
		t.Run(scenario, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "audit")
			start, target := original, live
			switch scenario {
			case "wrong ancestry":
				start, target = live, original
			case "short revision":
				target = live[:8]
			case "unowned directory":
				if err := os.Mkdir(destination, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(destination, "saved.txt"), []byte("KEEP"), 0600); err != nil {
					t.Fatal(err)
				}
			case "source directory":
				destination = filepath.Join(source, "audit")
			case "parent aliases source":
				alias := filepath.Join(t.TempDir(), "source-alias")
				if err := os.Symlink(source, alias); err != nil {
					t.Fatal(err)
				}
				destination = filepath.Join(alias, "new-directory", "deeper", "audit")
			}
			if _, err := client.PrepareVerificationSnapshot(context.Background(), source, destination, start, target); err == nil {
				t.Fatal("invalid snapshot was accepted")
			}
			if scenario == "unowned directory" {
				if data, err := os.ReadFile(filepath.Join(destination, "saved.txt")); err != nil || string(data) != "KEEP" {
					t.Fatal("unowned directory was replaced", err)
				}
			}
			if scenario == "parent aliases source" {
				if _, err := os.Stat(filepath.Join(source, "new-directory")); !os.IsNotExist(err) {
					t.Fatal("snapshot modified source through an aliased parent")
				}
			}
		})
	}
}

func TestVerificationSnapshotDoesNotBorrowAmbientProviderIdentity(t *testing.T) {
	source, _, original, live, _ := snapshotFixture(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "git")
	script := "#!/bin/sh\ntest -z \"${GH_TOKEN+x}${GITHUB_TOKEN+x}${SSH_AUTH_SOCK+x}${GIT_DIR+x}${GIT_CONFIG_COUNT+x}\" || exit 99\nexec '" + strings.ReplaceAll(realGit, "'", "'\\''") + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "SSH_AUTH_SOCK", "GIT_DIR", "GIT_CONFIG_COUNT"} {
		t.Setenv(key, "synthetic-unrelated-identity")
	}
	client := NewClient()
	client.gitBinary = wrapper
	if _, err := client.PrepareVerificationSnapshot(context.Background(), source, filepath.Join(t.TempDir(), "audit"), original, live); err != nil {
		t.Fatal("local snapshot inherited ambient credentials/configuration", err)
	}
}

func TestVerificationSnapshotWaitRespectsCancellationWithoutCreatingFiles(t *testing.T) {
	source, _, original, live, _ := snapshotFixture(t)
	verificationSnapshotGate <- struct{}{}
	defer func() { <-verificationSnapshotGate }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	destination := filepath.Join(t.TempDir(), "not-created", "audit")
	started := time.Now()
	if _, err := NewClient().PrepareVerificationSnapshot(ctx, source, destination, original, live); err == nil || time.Since(started) > 2*time.Second {
		t.Fatal("snapshot admission ignored cancellation", err)
	}
	if _, err := os.Stat(filepath.Dir(destination)); !os.IsNotExist(err) {
		t.Fatal("canceled snapshot admission created directories")
	}
}

func TestVerificationSnapshotExcludesUnrelatedCacheObjects(t *testing.T) {
	source, _, original, live, run := snapshotFixture(t)
	noise := filepath.Join(t.TempDir(), "old-build.bin")
	if err := os.WriteFile(noise, []byte(strings.Repeat("UNRELATED_OLD_BUILD", 4096)), 0600); err != nil {
		t.Fatal(err)
	}
	unrelated := run(source, "hash-object", "-w", noise)
	path, err := NewClient().PrepareVerificationSnapshot(context.Background(), source, filepath.Join(t.TempDir(), "audit"), original, live)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", path, "cat-file", "-e", unrelated).Run(); err == nil {
		t.Fatal("snapshot copied unrelated cached build objects")
	}
}

func TestVerificationSnapshotDoesNotTrustReplacedHistory(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	createTestBaseRepo(t, source)
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	original := run("rev-parse", "HEAD")
	run("checkout", "--orphan", "unrelated")
	run("rm", "-rf", ".")
	if err := os.WriteFile(filepath.Join(source, "other.txt"), []byte("UNRELATED"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "Unrelated root")
	target := run("rev-parse", "HEAD")
	run("replace", "--graft", target, original)
	run("merge-base", "--is-ancestor", original, target)
	if _, err := NewClient().PrepareVerificationSnapshot(context.Background(), source, filepath.Join(t.TempDir(), "audit"), original, target); err == nil {
		t.Fatal("replacement history fabricated source ancestry")
	}
}
