package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SkipIfShort skips the calling test if the -test.short flag was passed.
func SkipIfShort(t testing.TB, reason string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping in short mode: " + reason)
	}
}

// InitGitRepo initializes a local git repository with an initial commit for testing.
func InitGitRepo(t testing.TB, dir string, branch string) string {
	t.Helper()
	if branch == "" {
		branch = "main"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("testutil: create git dir: %v", err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("testutil: git %v failed: %v, output: %s", args, err, string(out))
		}
	}
	run("init")
	run("config", "user.name", "Cuan Test")
	run("config", "user.email", "test@cuan.local")
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("testutil: write README: %v", err)
	}
	run("add", ".")
	run("commit", "-m", "Initial commit")
	run("branch", "-M", branch)
	return dir
}
