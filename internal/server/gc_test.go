package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
)

func TestServer_GCSchedulers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-gc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := &Server{
		cfg: &config.Config{
			ProjectsDir: tmpDir,
		},
	}

	// 1. Task workspace GC: with empty/non-existent task-workspaces
	s.runTaskWorkspaceGC()

	// 2. Release GC: non-existent /opt/cuan/releases should return cleanly
	s.runReleaseGC()

	// 3. Log GC: non-existent /var/log/nginx & postgresql should return cleanly
	s.runLogGC()
}

func TestServer_ReleaseGC_RetentionLogic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cuan-release-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 5 dummy release dirs
	now := time.Now()
	for i := 1; i <= 5; i++ {
		relPath := filepath.Join(tmpDir, "rel-"+string(rune('0'+i)))
		if err := os.MkdirAll(relPath, 0755); err != nil {
			t.Fatalf("failed to mkdir: %v", err)
		}
		mod := now.Add(time.Duration(i) * time.Hour)
		_ = os.Chtimes(relPath, mod, mod)
	}
}
