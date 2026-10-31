package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyBinaryIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Missing file
	missing := filepath.Join(tmpDir, "missing-bin")
	if err := VerifyBinaryIntegrity(missing); err == nil {
		t.Fatal("expected error for missing binary")
	}

	// 2. Empty file
	empty := filepath.Join(tmpDir, "empty-bin")
	_ = os.WriteFile(empty, []byte{}, 0755)
	if err := VerifyBinaryIntegrity(empty); err == nil {
		t.Fatal("expected error for 0-byte binary")
	}

	// 3. Non-executable file
	nonExec := filepath.Join(tmpDir, "non-exec-bin")
	_ = os.WriteFile(nonExec, []byte("echo hello"), 0644)
	if err := VerifyBinaryIntegrity(nonExec); err == nil {
		t.Fatal("expected error for non-executable file")
	}

	// 4. Valid executable script
	valid := filepath.Join(tmpDir, "valid-bin")
	_ = os.WriteFile(valid, []byte("#!/bin/sh\nexit 0\n"), 0755)
	if err := VerifyBinaryIntegrity(valid); err != nil {
		t.Fatalf("expected valid binary to pass integrity check: %v", err)
	}
}

func TestUpdateManager_CheckForUpdateAndPerform(t *testing.T) {
	tmpDir := t.TempDir()

	currentBin := filepath.Join(tmpDir, "current-runner")
	targetBin := filepath.Join(tmpDir, "target-runner")

	// Create current runner script
	_ = os.WriteFile(currentBin, []byte("#!/bin/sh\necho v1\n"), 0755)

	um := NewUpdateManager(targetBin)
	um.currentBinPath = currentBin

	// Target does not exist yet
	hasUpdate, sha, err := um.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasUpdate {
		t.Fatal("expected no update before target exists")
	}

	// Target created with same content
	_ = os.WriteFile(targetBin, []byte("#!/bin/sh\necho v1\n"), 0755)
	hasUpdate, _, err = um.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasUpdate {
		t.Fatal("expected no update when target has identical checksum")
	}

	// Target updated to new release v2
	_ = os.WriteFile(targetBin, []byte("#!/bin/sh\necho v2\n"), 0755)
	hasUpdate, sha, err = um.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasUpdate || sha == "" {
		t.Fatalf("expected update detected with SHA, got hasUpdate=%v, sha=%s", hasUpdate, sha)
	}

	// PerformSelfUpdate with custom restartFunc
	restarted := false
	var restartedPath string
	um.SetRestartFunc(func(p string, args []string, env []string) error {
		restarted = true
		restartedPath = p
		return nil
	})

	if err := um.PerformSelfUpdate([]string{"cuan-runner"}, []string{"FOO=BAR"}); err != nil {
		t.Fatalf("PerformSelfUpdate failed: %v", err)
	}
	if !restarted {
		t.Fatal("expected restartFunc to be called")
	}
	if restartedPath != targetBin {
		t.Fatalf("expected restarted path %s, got %s", targetBin, restartedPath)
	}
}
