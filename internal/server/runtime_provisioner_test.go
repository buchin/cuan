package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeProvisionerIsolatedEnv(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("PATH", temp)
	if !isIsolatedTestEnv() {
		t.Fatalf("expected isolated test environment when PATH=%s", temp)
	}

	t.Setenv("PATH", "/usr/bin:/bin")
	if isIsolatedTestEnv() {
		t.Fatal("expected non-isolated environment when PATH contains /usr/bin")
	}
}

func TestEnsureCandidateInPath(t *testing.T) {
	orig := os.Getenv("PATH")
	defer os.Setenv("PATH", orig)

	testDir := "/test/custom/bin"
	os.Setenv("PATH", "/usr/bin:/bin")
	ensureCandidateInPath(testDir)

	if !strings.HasPrefix(os.Getenv("PATH"), testDir) {
		t.Fatalf("expected PATH to start with %s, got: %s", testDir, os.Getenv("PATH"))
	}

	// Calling again should not duplicate
	ensureCandidateInPath(testDir)
	count := strings.Count(os.Getenv("PATH"), testDir)
	if count != 1 {
		t.Fatalf("expected %s to appear once in PATH, got count=%d", testDir, count)
	}
}

func TestFindCandidateSandboxWithoutExecution(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	userBin := filepath.Join(tempHome, ".local", "bin")
	if err := os.MkdirAll(userBin, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(userBin, "executed")
	bwrapPath := filepath.Join(userBin, "bwrap")
	if err := os.WriteFile(bwrapPath, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0755); err != nil {
		t.Fatal(err)
	}

	found := findCandidateSandbox()
	if found != bwrapPath {
		t.Fatalf("expected to find candidate %s, got %s", bwrapPath, found)
	}

	// Verify no execution occurred
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("findCandidateSandbox executed the candidate binary")
	}
}

func TestEnsureRuntimePrerequisitesInIsolatedTestNoOp(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("PATH", temp)
	// Should return immediately without attempting any host modifications or network
	EnsureRuntimePrerequisites(context.Background())
}
