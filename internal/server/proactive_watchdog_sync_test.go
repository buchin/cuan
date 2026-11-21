package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func setupTestGitRepo(t *testing.T) (remotePath, localPath string) {
	t.Helper()
	root := t.TempDir()
	remotePath = filepath.Join(root, "remote.git")
	localPath = filepath.Join(root, "local")

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

	// Push commit 2 to remote
	_ = os.WriteFile(filepath.Join(seed, "file.txt"), []byte("v2"), 0644)
	run("-C", seed, "add", "file.txt")
	run("-C", seed, "commit", "-m", "Commit 2")
	run("-C", seed, "push", remotePath, "dev")

	return remotePath, localPath
}

func TestWatchdogIdleGitSync_SuccessWhenIdle(t *testing.T) {
	ctx := context.Background()
	remotePath, localPath := setupTestGitRepo(t)

	pstore := project.NewMemoryStore()
	gitClient := git.NewClient()
	projSvc := project.NewService(pstore, gitClient, "secret", t.TempDir())

	err := pstore.CreateProject(ctx, &project.Project{
		ID:            "proj_test",
		Name:          "Test Proj",
		RepoURL:       remotePath,
		DefaultBranch: "dev",
		LocalPath:     localPath,
		CloneStatus:   "synced",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)

	srv := &Server{
		cfg:        &config.Config{Environment: "testing", WorkspacesDir: t.TempDir()},
		projectSvc: projSvc,
		taskSvc:    taskSvc,
	}

	// Local is on commit 1 before watchdog run
	initialSHA, _ := exec.Command("git", "-C", localPath, "rev-parse", "HEAD").Output()

	srv.syncIdleProjectsGit(ctx)

	afterSHA, _ := exec.Command("git", "-C", localPath, "rev-parse", "HEAD").Output()
	if strings.TrimSpace(string(initialSHA)) == strings.TrimSpace(string(afterSHA)) {
		t.Fatalf("expected watchdog to fast-forward local branch when idle, but SHA did not change")
	}

	content, _ := os.ReadFile(filepath.Join(localPath, "file.txt"))
	if string(content) != "v2" {
		t.Fatalf("expected file.txt to be updated to 'v2', got '%s'", string(content))
	}
}

func TestWatchdogIdleGitSync_ActiveRunnerSkips(t *testing.T) {
	ctx := context.Background()
	remotePath, localPath := setupTestGitRepo(t)

	pstore := project.NewMemoryStore()
	gitClient := git.NewClient()
	projSvc := project.NewService(pstore, gitClient, "secret", t.TempDir())

	err := pstore.CreateProject(ctx, &project.Project{
		ID:            "proj_test",
		Name:          "Test Proj",
		RepoURL:       remotePath,
		DefaultBranch: "dev",
		LocalPath:     localPath,
		CloneStatus:   "synced",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	// Create an active task with status 'doing'
	_ = taskStore.CreateTask(ctx, &task.Task{
		ID:        "active_task",
		ProjectID: "proj_test",
		Status:    task.TaskStatusDoing,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	srv := &Server{
		cfg:        &config.Config{Environment: "testing", WorkspacesDir: t.TempDir()},
		projectSvc: projSvc,
		taskSvc:    taskSvc,
	}

	initialSHA, _ := exec.Command("git", "-C", localPath, "rev-parse", "HEAD").Output()

	// Should skip because a task is 'doing' (runner active)
	srv.syncIdleProjectsGit(ctx)

	afterSHA, _ := exec.Command("git", "-C", localPath, "rev-parse", "HEAD").Output()
	if strings.TrimSpace(string(initialSHA)) != strings.TrimSpace(string(afterSHA)) {
		t.Fatalf("watchdog should NOT sync when runner is active")
	}
}

func TestWatchdogIdleGitSync_LockedLeaseSkips(t *testing.T) {
	ctx := context.Background()
	remotePath, localPath := setupTestGitRepo(t)

	pstore := project.NewMemoryStore()
	gitClient := git.NewClient()
	projSvc := project.NewService(pstore, gitClient, "secret", t.TempDir())

	err := pstore.CreateProject(ctx, &project.Project{
		ID:            "proj_test",
		Name:          "Test Proj",
		RepoURL:       remotePath,
		DefaultBranch: "dev",
		LocalPath:     localPath,
		CloneStatus:   "synced",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)

	// Acquire verification lease to simulate an active runner holding the repo lock
	lease, err := git.TryAcquireVerificationLease(ctx, localPath)
	if err != nil {
		t.Fatalf("failed to acquire lease: %v", err)
	}
	defer lease.Close()

	srv := &Server{
		cfg:        &config.Config{Environment: "testing", WorkspacesDir: t.TempDir()},
		projectSvc: projSvc,
		taskSvc:    taskSvc,
	}

	initialSHA, _ := exec.Command("git", "-C", localPath, "rev-parse", "HEAD").Output()

	srv.syncIdleProjectsGit(ctx)

	afterSHA, _ := exec.Command("git", "-C", localPath, "rev-parse", "HEAD").Output()
	if strings.TrimSpace(string(initialSHA)) != strings.TrimSpace(string(afterSHA)) {
		t.Fatalf("watchdog should skip when lease is held")
	}
}
