package testutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/testutil"
)

func TestInitGitRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "repo")
	result := testutil.InitGitRepo(t, dir, "main")
	if result != dir {
		t.Fatalf("expected dir %s, got %s", dir, result)
	}

	readme := filepath.Join(dir, "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("failed to read README: %v", err)
	}
	if string(data) != "# Test Repo\n" {
		t.Fatalf("unexpected README content: %s", string(data))
	}
}

func TestSkipIfShort(t *testing.T) {
	// Should not panic or fail
	testutil.SkipIfShort(t, "unit test check")
}
