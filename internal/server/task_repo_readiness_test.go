package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v: %s", args, err, string(out))
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")
}

func TestTaskRepoReadiness_HealsStaleLocalPath(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	prepareTaskReporter(t, s)

	projectID := "proj_test_readiness"
	stalePath := filepath.Join(t.TempDir(), "pruned-release", "data", "projects", projectID)
	realDir := filepath.Join(t.TempDir(), "projects", projectID)
	initTestGitRepo(t, realDir)

	stubBin := t.TempDir()
	gitStub := "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"fetch\" ]; then exit 0; fi\ndone\nexec /usr/bin/git \"$@\"\n"
	if err := os.WriteFile(filepath.Join(stubBin, "git"), []byte(gitStub), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubBin+":"+os.Getenv("PATH"))

	now := time.Now().UTC()
	proj := &project.Project{
		ID:            projectID,
		Name:          "readiness-test",
		RepoURL:       "https://github.com/test/readiness.git",
		DefaultBranch: "main",
		LocalPath:     stalePath, // points to missing pruned path!
		CloneStatus:   "synced",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.projectSvc.Store().CreateProject(t.Context(), proj); err != nil {
		t.Fatal(err)
	}

	// Make realDir accessible via baseDir candidate
	baseDir := filepath.Dir(realDir)
	s.projectSvc = project.NewService(s.projectSvc.Store(), nil, s.cfg.AuthSecret, baseDir)

	// Create a task bound to this project
	createTaskPayload := `{"request_id":"30000000-0000-4000-8000-000000000001","project_id":"` + projectID + `","title":"Readiness task","instructions":"Do the task"}`
	w := call("POST", "/api/v1/tasks", createTaskPayload)
	if w.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d %s", w.Code, w.Body.String())
	}
	var created task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// Trigger run-agent (this should heal the stale path and start the session, NOT 409)
	runAgentPayload := `{"agent_id":"agent_lifecycle","prompt":"Start task"}`
	runResp := call("POST", "/api/v1/tasks/"+created.ID+"/run-agent", runAgentPayload)
	if runResp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after healing stale repo path, got %d: %s", runResp.Code, runResp.Body.String())
	}

	// Verify project local_path was healed in store
	updatedProj, err := s.projectSvc.Store().GetProject(t.Context(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedProj.LocalPath != realDir {
		t.Errorf("expected local_path to be healed to %q, got %q", realDir, updatedProj.LocalPath)
	}
}

func TestTaskRepoReadiness_AutoSyncWhenNotSynced(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	prepareTaskReporter(t, s)

	projectID := "proj_test_autosync"
	baseDir := t.TempDir()
	realDir := filepath.Join(baseDir, projectID)
	initTestGitRepo(t, realDir)

	// Create fake git wrapper in a temporary bin to stub out remote fetch during unit test
	stubBin := t.TempDir()
	gitStub := "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"fetch\" ]; then exit 0; fi\ndone\nexec /usr/bin/git \"$@\"\n"
	if err := os.WriteFile(filepath.Join(stubBin, "git"), []byte(gitStub), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubBin+":"+os.Getenv("PATH"))

	s.projectSvc = project.NewService(s.projectSvc.Store(), nil, s.cfg.AuthSecret, baseDir)

	now := time.Now().UTC()
	proj := &project.Project{
		ID:            projectID,
		Name:          "autosync-test",
		RepoOwner:     "test",
		RepoName:      "autosync",
		RepoURL:       "https://github.com/test/autosync.git",
		DefaultBranch: "main",
		LocalPath:     realDir,
		CloneStatus:   "pending", // Not synced yet!
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.projectSvc.Store().CreateProject(t.Context(), proj); err != nil {
		t.Fatal(err)
	}

	createTaskPayload := `{"request_id":"30000000-0000-4000-8000-000000000002","project_id":"` + projectID + `","title":"Autosync task","instructions":"Sync and do task"}`
	w := call("POST", "/api/v1/tasks", createTaskPayload)
	if w.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d %s", w.Code, w.Body.String())
	}
	var created task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// Trigger run-agent (auto-sync triggers and sets status to synced)
	runAgentPayload := `{"agent_id":"agent_lifecycle","prompt":"Start task"}`
	runResp := call("POST", "/api/v1/tasks/"+created.ID+"/run-agent", runAgentPayload)
	if runResp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK after auto-sync, got %d: %s", runResp.Code, runResp.Body.String())
	}

	// Verify project clone_status became synced
	updatedProj, err := s.projectSvc.Store().GetProject(t.Context(), projectID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedProj.CloneStatus != "synced" {
		t.Errorf("expected clone_status to become 'synced', got %q", updatedProj.CloneStatus)
	}
}
