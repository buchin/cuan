package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

type dummyPreflightAdapter struct{}

func (d *dummyPreflightAdapter) Name() string             { return "dummy" }
func (d *dummyPreflightAdapter) Version() (string, error) { return "1.0", nil }
func (d *dummyPreflightAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
}

func setupRunnerPreflightFixture(t *testing.T) (remotePath, localPath string) {
	t.Helper()
	root := t.TempDir()
	remotePath = filepath.Join(root, "remote.git")
	localPath = filepath.Join(root, "base")

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return strings.TrimSpace(string(out))
	}

	run("init", "--bare", remotePath)
	seed := filepath.Join(root, "seed")
	run("init", "-b", "dev", seed)
	run("-C", seed, "config", "user.name", "Tester")
	run("-C", seed, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("v1"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Commit 1")
	run("-C", seed, "push", remotePath, "dev")

	run("clone", "-b", "dev", remotePath, localPath)
	run("-C", localPath, "config", "user.name", "Tester")
	run("-C", localPath, "config", "user.email", "tester@example.com")

	// Advance remote with commit 2
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("v2"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Commit 2")
	run("-C", seed, "push", remotePath, "dev")

	return remotePath, localPath
}

func TestRunAttempt_PreflightSyncFastForwardsBaseRepo(t *testing.T) {
	remotePath, baseRepo := setupRunnerPreflightFixture(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	workspacesDir := filepath.Join(t.TempDir(), "workspaces")
	r := NewRunner(&config.Config{
		RunnerID:        "test-runner",
		RunnerServerURL: api.URL,
		WorkspacesDir:   workspacesDir,
	}, &dummyPreflightAdapter{}, api.Client())

	claim := ClaimTaskResponse{
		Claimed:       true,
		OwnerID:       "owner",
		WorkingDir:    baseRepo,
		DefaultBranch: "dev",
		RepoURL:       remotePath,
		Task: &task.Task{
			ID:                "test-task",
			ProjectID:         "test-proj",
			Title:             "Test Preflight Sync",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		Attempt: &task.TaskAttempt{
			ID:            "test-attempt",
			TaskID:        "test-task",
			AttemptNumber: 1,
		},
		TaskAgent: &TaskAgentCapability{
			API:   api.URL + "/report",
			Token: "token",
		},
	}

	// Verify before runAttempt, baseRepo is on commit 1 (file.txt has v1)
	c1, _ := os.ReadFile(filepath.Join(baseRepo, "file.txt"))
	if string(c1) != "v1" {
		t.Fatalf("expected v1 before run, got %s", string(c1))
	}

	r.runAttempt(context.Background(), claim)

	// Verify after runAttempt, baseRepo was fast-forwarded to commit 2 (file.txt has v2)
	c2, _ := os.ReadFile(filepath.Join(baseRepo, "file.txt"))
	if string(c2) != "v2" {
		t.Fatalf("expected base repo to be fast-forwarded to v2, got %s", string(c2))
	}
}

func TestRunAttempt_PreflightSyncNonBlockingOnDirtyBase(t *testing.T) {
	remotePath, baseRepo := setupRunnerPreflightFixture(t)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	// Dirty working tree in baseRepo
	_ = os.WriteFile(filepath.Join(baseRepo, "dirty.txt"), []byte("uncommitted change"), 0644)

	workspacesDir := filepath.Join(t.TempDir(), "workspaces")
	r := NewRunner(&config.Config{
		RunnerID:        "test-runner",
		RunnerServerURL: api.URL,
		WorkspacesDir:   workspacesDir,
	}, &dummyPreflightAdapter{}, api.Client())

	claim := ClaimTaskResponse{
		Claimed:       true,
		OwnerID:       "owner",
		WorkingDir:    baseRepo,
		DefaultBranch: "dev",
		RepoURL:       remotePath,
		Task: &task.Task{
			ID:                "test-task-dirty",
			ProjectID:         "test-proj",
			Title:             "Test Dirty Preflight Sync",
			ExecutionCategory: task.ExecutionCategoryCode,
		},
		Attempt: &task.TaskAttempt{
			ID:            "test-attempt-dirty",
			TaskID:        "test-task-dirty",
			AttemptNumber: 1,
		},
		TaskAgent: &TaskAgentCapability{
			API:   api.URL + "/report",
			Token: "token",
		},
	}

	// runAttempt should not panic or fail workspace setup
	r.runAttempt(context.Background(), claim)

	// Verify dirty file is still intact
	dirtyContent, err := os.ReadFile(filepath.Join(baseRepo, "dirty.txt"))
	if err != nil || string(dirtyContent) != "uncommitted change" {
		t.Fatalf("dirty file corrupted or missing: %v", err)
	}
}
