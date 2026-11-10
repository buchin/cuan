package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestResolvedConversationReleasesRepoCopiesButKeepsMessagesAndProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "project")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
	}
	git("init", "--initial-branch=main")
	os.WriteFile(filepath.Join(source, "result.txt"), []byte("project source"), 0600)
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "Fixture")
	store := project.NewMemoryStore()
	base := filepath.Join(root, "project-data")
	s.projectSvc = project.NewService(store, nil, "fixture-secret", base)
	if err := store.CreateProject(ctx, &project.Project{ID: "resolved-project", Name: "Project", RepoURL: "https://github.com/fixture/project", LocalPath: source, DefaultBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	room := &db.ProjectRoom{ProjectID: "resolved-project", Title: "Work complete"}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute)
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{RoomID: room.ID, AuthorType: "agent", Content: "Delivered result retained", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	item := &task.Task{ID: "resolved-copy-task", ProjectID: "resolved-project", SourceRoomID: room.ID, Title: "Completed", Status: "done", CreatedAt: at, UpdatedAt: at}
	if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	s.cfg.WorkspacesDir = filepath.Join(root, "runner-workspaces")
	os.Mkdir(s.cfg.WorkspacesDir, 0700)
	runnerCopy := filepath.Join(s.cfg.WorkspacesDir, item.ID)
	git("clone", "--local", source, runnerCopy)
	p, err := s.projectSvc.TaskWorkspace(ctx, item.ProjectID, item.Instructions)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := s.projectSvc.ExecutionWorkspace(ctx, item.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(worktree, "result.txt"), []byte("discarded workspace edit"), 0600)
	s.runWorkspaceRetention(ctx, 0, time.Now())
	if _, err := os.Stat(runnerCopy); err != nil {
		t.Fatal("unresolved copy was removed", err)
	}
	item.Status = "blocked"
	if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: "canceled-copy", TaskID: item.ID, Status: "canceled", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := s.projectStudioRepo.SetRoomResolved(ctx, room.ID, &at); err != nil {
		t.Fatal(err)
	}
	s.runWorkspaceRetention(ctx, 0, time.Now())
	for _, path := range []string{runnerCopy, worktree} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("resolved repo copy was not released", path, err)
		}
	}
	if content, err := os.ReadFile(filepath.Join(source, "result.txt")); err != nil || string(content) != "project source" {
		t.Fatal("project repository changed", err)
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil || len(messages) != 1 || messages[0].Content != "Delivered result retained" {
		t.Fatal("chat history changed", err)
	}
	if err := s.projectStudioRepo.SetRoomResolved(ctx, room.ID, nil); err != nil {
		t.Fatal(err)
	}
}
