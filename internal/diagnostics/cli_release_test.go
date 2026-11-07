package diagnostics

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/release"
)

func TestReleaseCLIUsesLiteralArgumentsAndKeepsDiagnosticsPrivate(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".cuan"), 0700)
	marker := filepath.Join(t.TempDir(), "literal-argument")
	unwanted := filepath.Join(t.TempDir(), "must-not-exist")
	literal := "$(touch " + unwanted + ")"
	manifest, _ := json.Marshal(map[string]any{"version": 1, "command": []string{"release.sh", literal}, "timeout_seconds": 5})
	os.WriteFile(filepath.Join(root, release.ManifestPath), manifest, 0600)
	os.WriteFile(filepath.Join(root, "release.sh"), []byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$MARKER\"\nprintf '%s' \"$CLI_FIXTURE_PASSWORD\"\n"), 0700)
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=dev")
	run("config", "user.name", "Fixture Human")
	run("config", "user.email", "human@example.invalid")
	run("add", ".")
	run("commit", "-m", "Maintained release")
	commit := run("rev-parse", "HEAD")
	t.Setenv("MARKER", marker)
	t.Setenv("CLI_FIXTURE_PASSWORD", "fixture-sensitive-value")
	var out, errs bytes.Buffer
	if code := RunReleaseCLI([]string{"inspect", "--path", root, "--commit", commit, "--branch", "dev"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("inspection executed release")
	}
	out.Reset()
	errs.Reset()
	if code := RunReleaseCLI([]string{"run", "--path", root, "--commit", commit, "--branch", "dev", "--logs"}, &out, &errs); code != 0 {
		t.Fatal(code, errs.String())
	}
	data, _ := os.ReadFile(marker)
	if string(data) != literal {
		t.Fatal("command arguments were interpreted by a shell")
	}
	if _, err := os.Stat(unwanted); !os.IsNotExist(err) {
		t.Fatal("shell substitution executed")
	}
	if strings.Contains(out.String()+errs.String(), "fixture-sensitive-value") || !strings.Contains(errs.String(), "[REDACTED]") {
		t.Fatal("private diagnostics exposed")
	}
	var result release.Result
	if json.Unmarshal(out.Bytes(), &result) != nil || result.ExitCode != 0 || !result.SourceUnchanged {
		t.Fatal("structured process result missing")
	}
}

func TestBoundTaskCannotRunMaintainerRelease(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", `{}`)
	var out, errs bytes.Buffer
	if RunReleaseCLI([]string{"run"}, &out, &errs) == 0 || !strings.Contains(errs.String(), "bound task") {
		t.Fatal("worker used maintainer authority")
	}
}

// runtimePathFixture creates a minimal git repo with a manifest requiring tool "fixture-runtime-tool"
// and returns root dir and HEAD commit SHA.
func runtimePathFixture(t *testing.T) (root, commit string) {
	t.Helper()
	root = t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".cuan"), 0700); err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(map[string]any{
		"version":        1,
		"command":        []string{"release.sh"},
		"timeout_seconds": 5,
		"required_tools": []string{"fixture-runtime-tool"},
	})
	os.WriteFile(filepath.Join(root, release.ManifestPath), manifest, 0600)
	os.WriteFile(filepath.Join(root, "release.sh"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=dev")
	run("config", "user.name", "Fixture Human")
	run("config", "user.email", "human@example.invalid")
	run("add", ".")
	run("commit", "-m", "Runtime path fixture")
	commit = run("rev-parse", "HEAD")
	return
}

// TestInspectRuntimePathToolAvailable: tool present in explicit PATH → inspect exits 0.
func TestInspectRuntimePathToolAvailable(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	root, commit := runtimePathFixture(t)

	// Create the tool in a temp dir.
	toolDir := t.TempDir()
	toolBin := filepath.Join(toolDir, "fixture-runtime-tool")
	os.WriteFile(toolBin, []byte("#!/bin/sh\nexit 0\n"), 0700)

	var out, errs bytes.Buffer
	code := RunReleaseCLI([]string{
		"inspect",
		"--path", root,
		"--commit", commit,
		"--branch", "dev",
		"--runtime-path", toolDir,
	}, &out, &errs)
	if code != 0 {
		t.Fatalf("expected exit 0 with tool available, got %d: %s", code, errs.String())
	}
}

// TestInspectRuntimePathToolMissing: tool absent in explicit PATH → inspect exits non-zero with informative message.
func TestInspectRuntimePathToolMissing(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	root, commit := runtimePathFixture(t)

	emptyDir := t.TempDir() // no tools here

	var out, errs bytes.Buffer
	code := RunReleaseCLI([]string{
		"inspect",
		"--path", root,
		"--commit", commit,
		"--branch", "dev",
		"--runtime-path", emptyDir,
	}, &out, &errs)
	if code == 0 {
		t.Fatal("expected non-zero exit when required tool is missing from explicit PATH")
	}
	if !strings.Contains(errs.String(), "fixture-runtime-tool") {
		t.Fatalf("expected informative error naming missing tool, got: %s", errs.String())
	}
}

// TestInspectRuntimePathEmptyFailsWithRequiredTools: --runtime-path "" → fails, does not fall back to host PATH.
func TestInspectRuntimePathEmptyFailsWithRequiredTools(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	root, commit := runtimePathFixture(t)

	// Put the tool on the host PATH so we can prove we do NOT fall back to it.
	toolDir := t.TempDir()
	os.WriteFile(filepath.Join(toolDir, "fixture-runtime-tool"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	t.Setenv("PATH", toolDir+":"+os.Getenv("PATH"))

	var out, errs bytes.Buffer
	code := RunReleaseCLI([]string{
		"inspect",
		"--path", root,
		"--commit", commit,
		"--branch", "dev",
		"--runtime-path", "", // explicit empty PATH
	}, &out, &errs)
	if code == 0 {
		t.Fatal("expected non-zero exit with --runtime-path=\"\" and required tools: must not fall back to host PATH")
	}
}

// TestInspectWithoutRuntimePathSkipsCheck: no --runtime-path flag → inspect succeeds without runtime check (backward-compatible).
func TestInspectWithoutRuntimePathSkipsCheck(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	root, commit := runtimePathFixture(t)

	// Do NOT provide --runtime-path; required tool is not on any PATH.
	// Inspect should still succeed (old behavior: no runtime check).
	var out, errs bytes.Buffer
	code := RunReleaseCLI([]string{
		"inspect",
		"--path", root,
		"--commit", commit,
		"--branch", "dev",
		// no --runtime-path
	}, &out, &errs)
	if code != 0 {
		t.Fatalf("expected exit 0 without --runtime-path (backward-compatible), got %d: %s", code, errs.String())
	}
}
