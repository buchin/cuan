package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestNativePTYReceivesOnlySavedProjectCredentials(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("DOJO_VPS_PASSWORD", "ambient-rejected")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "foreign-rejected")
	script := `#!/bin/sh
 test "$DOJO_VPS_PASSWORD" = project-authorized || exit 31
 test "$DOJO_DEPLOY_HOST" = project-target || exit 32
 test -z "$AWS_SECRET_ACCESS_KEY" || exit 33
 printf verified > "$WORKSPACE_DIR/delegation-receipt"
 `
	if err := os.WriteFile(filepath.Join(root, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer api.Close()
	instance := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL}, nil, api.Client())
	current := &task.Task{ID: "project-environment"}
	attempt := &task.TaskAttempt{ID: "project-environment-attempt", TaskID: current.ID}
	claim := ClaimTaskResponse{Task: current, Attempt: attempt, Environment: "DOJO_VPS_PASSWORD=project-authorized\nDOJO_DEPLOY_HOST=project-target"}
	var logs int64
	result, err := instance.executeWithPTY(context.Background(), claim, current, attempt, "Fixture only", root, map[string]string{"HOME": root}, 3*time.Second, &logs)
	if err != nil || result == nil || result.ExitCode != 0 {
		t.Fatalf("project environment did not reach native PTY: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "delegation-receipt")); err != nil || string(data) != "verified" {
		t.Fatal("missing subprocess environment receipt", err)
	}
}
