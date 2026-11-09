package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func documentationFixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspaces", "repo")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v %s", err, out)
		}
	}
	git("init", "-b", "dev")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	os.MkdirAll(filepath.Join(root, "scripts"), 0700)
	os.WriteFile(filepath.Join(root, "scripts/test.sh"), []byte("#!/bin/sh\necho 'project verifier reached'\nexit 7\n"), 0700)
	git("add", ".")
	git("commit", "-m", "baseline")
	git("checkout", "-b", "task")
	return root
}
func TestDocumentationVerificationIncludesCommittedAndUntrackedFiles(t *testing.T) {
	root := documentationFixture(t)
	os.WriteFile(filepath.Join(root, "guide.md"), []byte("# Guide\n\nA useful guide.\n"), 0600)
	cmd := exec.Command("git", "add", "guide.md")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "commit", "-m", "write guide")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "notes.md"), []byte("Additional notes."), 0600)
	result := runProjectVerification(context.Background(), root, "dev")
	if !result.Passed || !strings.Contains(result.Summary, "2 readable") {
		t.Fatalf("committed/untracked docs not verified: %+v", result)
	}
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0600)
	result = runProjectVerification(context.Background(), root, "dev")
	if result.Passed || !strings.Contains(result.Summary, "project verifier reached") {
		t.Fatalf("mixed code bypassed project verifier: %+v", result)
	}
}

func TestDocumentationVerificationUsesBoundBaseline(t *testing.T) {
	root := documentationFixture(t)
	cmd := exec.Command("git", "update-ref", "refs/remotes/cuan-base/dev", "HEAD")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "guide.md"), []byte("Approved guide."), 0600)
	result := verifyDocumentationChanges(context.Background(), root, "refs/remotes/cuan-base/dev")
	if result == nil || !result.Passed {
		t.Fatalf("bound baseline not used: %+v", result)
	}
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0600)
	if result := verifyDocumentationChanges(context.Background(), root, "refs/remotes/cuan-base/dev"); result != nil {
		t.Fatalf("code change incorrectly treated as docs: %+v", result)
	}
}
func TestDocumentationVerificationRejectsEmptyOrSymlinkFiles(t *testing.T) {
	for _, kind := range []string{"empty", "symlink", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			root := documentationFixture(t)
			p := filepath.Join(root, "guide.md")
			switch kind {
			case "empty":
				os.WriteFile(p, []byte(" \n"), 0600)
			case "symlink":
				target := filepath.Join(t.TempDir(), "private.md")
				os.WriteFile(target, []byte("private"), 0600)
				os.Symlink(target, p)
			case "invalid":
				os.WriteFile(p, []byte{0xff, 0, 1}, 0600)
			}
			if result := runProjectVerification(context.Background(), root, "dev"); result.Passed {
				t.Fatalf("invalid doc passed: %+v", result)
			}
		})
	}
}
func TestUnknownDocumentationBaseFallsBackToProjectVerifier(t *testing.T) {
	root := documentationFixture(t)
	os.WriteFile(filepath.Join(root, "guide.md"), []byte("guide"), 0600)
	result := runProjectVerification(context.Background(), root, "missing")
	if result.Passed || !strings.Contains(result.Summary, "project verifier reached") {
		t.Fatal(result)
	}
}
