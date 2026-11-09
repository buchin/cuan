package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyReuseLockfileWarmAndInvalidation(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()

	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0700); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}

	write(filepath.Join(root, "package.json"), `{"name":"fixture","version":"1.0.0"}`)
	initialLock := `{"name":"fixture","version":"1.0.0","lockfileVersion":3,"packages":{}}`
	write(filepath.Join(root, "package-lock.json"), initialLock)
	write(filepath.Join(root, ".gitignore"), "node_modules/\n")

	// Mock npm installer that records how many times it was invoked
	installerCounter := filepath.Join(bin, "installer-calls")
	npmScript := `#!/bin/sh
count=0
if [ -f "` + installerCounter + `" ]; then
  count=$(cat "` + installerCounter + `")
fi
count=$((count + 1))
echo "$count" > "` + installerCounter + `"
mkdir -p node_modules/fixture-pkg
echo 'installed' > node_modules/fixture-pkg/content.txt
`
	write(filepath.Join(bin, "npm"), npmScript)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	readCalls := func() string {
		data, err := os.ReadFile(installerCounter)
		if err != nil {
			return "0"
		}
		return strings.TrimSpace(string(data))
	}

	// 1. Initial cold run: npm ci should be executed
	if err := prepareInspectionDependencies(context.Background(), root); err != nil {
		t.Fatalf("initial cold prepare failed: %v", err)
	}
	if readCalls() != "1" {
		t.Fatalf("expected 1 installer call, got %s", readCalls())
	}
	fingerprintPath := filepath.Join(root, "node_modules", dependencyLockFingerprintFilename)
	fingerprint, err := os.ReadFile(fingerprintPath)
	if err != nil || string(fingerprint) == "" {
		t.Fatalf("expected fingerprint to be recorded, err: %v", err)
	}

	// 2. Warm run with identical lockfile: npm ci should NOT be executed
	if err := prepareInspectionDependencies(context.Background(), root); err != nil {
		t.Fatalf("warm prepare failed: %v", err)
	}
	if readCalls() != "1" {
		t.Fatalf("expected installer NOT to run on warm reuse, calls: %s", readCalls())
	}

	// 3. Changed lockfile: invalidation triggers fresh npm ci
	changedLock := `{"name":"fixture","version":"1.0.1","lockfileVersion":3,"packages":{"node_modules/example":{"resolved":"https://registry.npmjs.org/example/-/example-1.0.0.tgz"}}}`
	write(filepath.Join(root, "package-lock.json"), changedLock)

	if err := prepareInspectionDependencies(context.Background(), root); err != nil {
		t.Fatalf("invalidation prepare failed: %v", err)
	}
	if readCalls() != "2" {
		t.Fatalf("expected installer to run after lockfile change, calls: %s", readCalls())
	}
	newFingerprint, err := os.ReadFile(fingerprintPath)
	if err != nil || string(newFingerprint) == string(fingerprint) {
		t.Fatalf("expected fingerprint to update after lockfile change")
	}

	// 4. Missing fingerprint: invalidation triggers fresh npm ci
	_ = os.Remove(fingerprintPath)
	if err := prepareInspectionDependencies(context.Background(), root); err != nil {
		t.Fatalf("prepare failed after fingerprint removal: %v", err)
	}
	if readCalls() != "3" {
		t.Fatalf("expected installer to run when fingerprint is missing, calls: %s", readCalls())
	}
}

func TestDependencyReuseBetweenDevAndVerifierSnapshots(t *testing.T) {
	devWorkspace := t.TempDir()
	verifierSnapshot := t.TempDir()
	bin := t.TempDir()

	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0700); err != nil {
			t.Fatal(err)
		}
	}

	for _, dir := range []string{devWorkspace, verifierSnapshot} {
		if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}

	lockContent := `{"name":"reuse-fixture","version":"1.0.0","lockfileVersion":3,"packages":{}}`
	write(filepath.Join(devWorkspace, "web", "package.json"), `{"name":"web","version":"1.0.0"}`)
	write(filepath.Join(devWorkspace, "web", "package-lock.json"), lockContent)
	write(filepath.Join(devWorkspace, "web", ".gitignore"), "node_modules/\n")

	write(filepath.Join(verifierSnapshot, "web", "package.json"), `{"name":"web","version":"1.0.0"}`)
	write(filepath.Join(verifierSnapshot, "web", "package-lock.json"), lockContent)
	write(filepath.Join(verifierSnapshot, "web", ".gitignore"), "node_modules/\n")

	installerCounter := filepath.Join(bin, "installer-calls")
	npmScript := `#!/bin/sh
count=0
if [ -f "` + installerCounter + `" ]; then
  count=$(cat "` + installerCounter + `")
fi
count=$((count + 1))
echo "$count" > "` + installerCounter + `"
mkdir -p node_modules/web-pkg
echo 'built' > node_modules/web-pkg/index.js
`
	write(filepath.Join(bin, "npm"), npmScript)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	readCalls := func() string {
		data, err := os.ReadFile(installerCounter)
		if err != nil {
			return "0"
		}
		return strings.TrimSpace(string(data))
	}

	// Step 1: Dev runs initial preparation
	if err := prepareInspectionDependencies(context.Background(), devWorkspace); err != nil {
		t.Fatalf("dev prepare failed: %v", err)
	}
	if readCalls() != "1" {
		t.Fatalf("expected 1 installer call in dev, got %s", readCalls())
	}

	// Step 2: Seed warm dependencies from dev to verifier snapshot
	seedWarmDependencies(devWorkspace, verifierSnapshot)

	// Verify target now has warm node_modules
	targetPkg := filepath.Join(verifierSnapshot, "web", "node_modules", "web-pkg", "index.js")
	data, err := os.ReadFile(targetPkg)
	if err != nil || strings.TrimSpace(string(data)) != "built" {
		t.Fatalf("expected seeded file in verifier snapshot, err: %v", err)
	}

	// Verify isolation: verifier node_modules must be a real directory, NOT a symlink
	vInfo, err := os.Lstat(filepath.Join(verifierSnapshot, "web", "node_modules"))
	if err != nil || !vInfo.IsDir() || vInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("verifier node_modules must be an independent directory, not a symlink")
	}

	// Step 3: Verifier runs prepareInspectionDependencies
	// Installer should NOT run because dependencies were warm-seeded and match the lockfile!
	if err := prepareInspectionDependencies(context.Background(), verifierSnapshot); err != nil {
		t.Fatalf("verifier prepare failed: %v", err)
	}
	if readCalls() != "1" {
		t.Fatalf("expected verifier to reuse warm dependencies without running installer, calls: %s", readCalls())
	}
}

func TestDependencyReuseInvalidationAcrossDevAndVerifier(t *testing.T) {
	devWorkspace := t.TempDir()
	verifierSnapshot := t.TempDir()
	bin := t.TempDir()

	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0700); err != nil {
			t.Fatal(err)
		}
	}

	for _, dir := range []string{devWorkspace, verifierSnapshot} {
		if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}

	devLock := `{"name":"dev-lock","version":"1.0.0","lockfileVersion":3,"packages":{}}`
	verifierLock := `{"name":"verifier-lock","version":"2.0.0","lockfileVersion":3,"packages":{}}`

	write(filepath.Join(devWorkspace, "package.json"), `{"name":"root","version":"1.0.0"}`)
	write(filepath.Join(devWorkspace, "package-lock.json"), devLock)
	write(filepath.Join(devWorkspace, ".gitignore"), "node_modules/\n")

	write(filepath.Join(verifierSnapshot, "package.json"), `{"name":"root","version":"2.0.0"}`)
	write(filepath.Join(verifierSnapshot, "package-lock.json"), verifierLock)
	write(filepath.Join(verifierSnapshot, ".gitignore"), "node_modules/\n")

	installerCounter := filepath.Join(bin, "installer-calls")
	npmScript := `#!/bin/sh
count=0
if [ -f "` + installerCounter + `" ]; then
  count=$(cat "` + installerCounter + `")
fi
count=$((count + 1))
echo "$count" > "` + installerCounter + `"
mkdir -p node_modules/installed-pkg
echo "$count" > node_modules/installed-pkg/version.txt
`
	write(filepath.Join(bin, "npm"), npmScript)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Dev runs preparation
	if err := prepareInspectionDependencies(context.Background(), devWorkspace); err != nil {
		t.Fatalf("dev prepare failed: %v", err)
	}

	// Attempt to seed warm dependencies from dev to verifier with MISMATCHED lockfile
	seedWarmDependencies(devWorkspace, verifierSnapshot)

	// Verifier node_modules must NOT have been seeded because lockfiles differed
	if _, err := os.Stat(filepath.Join(verifierSnapshot, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("expected verifier node_modules NOT to be seeded when lockfile changed")
	}

	// Verifier prepare should execute installer afresh
	if err := prepareInspectionDependencies(context.Background(), verifierSnapshot); err != nil {
		t.Fatalf("verifier prepare failed: %v", err)
	}

	vData, err := os.ReadFile(filepath.Join(verifierSnapshot, "node_modules", "installed-pkg", "version.txt"))
	if err != nil || strings.TrimSpace(string(vData)) != "2" {
		t.Fatalf("expected verifier to run installer call 2, got %s", string(vData))
	}
}

func TestDependencyReuseIsolationRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()

	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0700); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}

	lock := `{"name":"symlink-test","version":"1.0.0","lockfileVersion":3,"packages":{}}`
	write(filepath.Join(root, "package.json"), `{"name":"symlink-test","version":"1.0.0"}`)
	write(filepath.Join(root, "package-lock.json"), lock)
	write(filepath.Join(root, ".gitignore"), "node_modules/\n")

	// Symlink node_modules to external dir
	if err := os.Symlink(other, filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}

	err := prepareInspectionDependencies(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "must not use shared links") {
		t.Fatalf("expected shared links rejection error, got: %v", err)
	}
}
