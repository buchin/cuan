package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconcileCommitsMergesIndependentLineages(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s", args, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	run("init", "--initial-branch=dev")
	run("config", "user.name", "Cuan Verifier")
	run("config", "user.email", "verifier@cuan.work")

	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	// 1. Base commit
	writeFile("base.txt", "initial\n")
	run("add", ".")
	run("commit", "-m", "Base commit")

	// 2. Lineage 1: Active running release (e.g. backend multi-brain support)
	run("checkout", "-b", "running-release")
	writeFile("brain.txt", "multi-brain support\n")
	run("add", ".")
	run("commit", "-m", "Add brain support")
	runningSHA := run("rev-parse", "HEAD")

	// 3. Lineage 2: Newly merged task release (e.g. textarea composer alignment)
	run("checkout", "dev")
	run("checkout", "-b", "target-release")
	writeFile("composer.txt", "aligned textarea composer\n")
	run("add", ".")
	run("commit", "-m", "Align composer")
	targetSHA := run("rev-parse", "HEAD")

	client := NewClient()

	// Identical commits return early
	same, err := client.ReconcileCommits(context.Background(), root, targetSHA, targetSHA, "No-op")
	if err != nil || same != targetSHA {
		t.Fatalf("expected identical commit returned, got %s, err %v", same, err)
	}

	// Reconcile divergent commits
	reconciledSHA, err := client.ReconcileCommits(context.Background(), root, targetSHA, runningSHA, "Reconcile running release with target release")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}

	if !IsCommitHash(reconciledSHA) {
		t.Fatalf("expected valid commit hash, got %s", reconciledSHA)
	}

	// Verify both parent commits are ancestors of the reconciled commit
	out1, err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", targetSHA, reconciledSHA).CombinedOutput()
	if err != nil {
		t.Fatalf("targetSHA not ancestor: %s", string(out1))
	}

	out2, err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", runningSHA, reconciledSHA).CombinedOutput()
	if err != nil {
		t.Fatalf("runningSHA not ancestor: %s", string(out2))
	}

	// Verify files from both lineages are present in the reconciled commit
	brainFile, err := exec.Command("git", "-C", root, "cat-file", "-p", reconciledSHA+":brain.txt").CombinedOutput()
	if err != nil || strings.TrimSpace(string(brainFile)) != "multi-brain support" {
		t.Fatalf("brain.txt content mismatch: %s", string(brainFile))
	}

	composerFile, err := exec.Command("git", "-C", root, "cat-file", "-p", reconciledSHA+":composer.txt").CombinedOutput()
	if err != nil || strings.TrimSpace(string(composerFile)) != "aligned textarea composer" {
		t.Fatalf("composer.txt content mismatch: %s", string(composerFile))
	}
}

func TestReconcileCommitsDetectsConflict(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s", args, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	run("init", "--initial-branch=dev")
	run("config", "user.name", "Cuan Verifier")
	run("config", "user.email", "verifier@cuan.work")

	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("conflict.txt", "base\n")
	run("add", ".")
	run("commit", "-m", "Base")

	run("checkout", "-b", "branch-a")
	writeFile("conflict.txt", "version A\n")
	run("add", ".")
	run("commit", "-m", "Change A")
	shaA := run("rev-parse", "HEAD")

	run("checkout", "dev")
	run("checkout", "-b", "branch-b")
	writeFile("conflict.txt", "version B\n")
	run("add", ".")
	run("commit", "-m", "Change B")
	shaB := run("rev-parse", "HEAD")

	client := NewClient()
	_, err := client.ReconcileCommits(context.Background(), root, shaA, shaB, "Conflict merge")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
}
