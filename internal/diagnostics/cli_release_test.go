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
