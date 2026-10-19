package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxProjectScopeSnapshotAndWorkspace(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	os.Mkdir(repo, 0700)
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, e := c.CombinedOutput(); e != nil {
			t.Fatal(e, string(out))
		}
	}
	run("init", "-q")
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("Project alpha architecture evidence"), 0600)
	os.WriteFile(filepath.Join(repo, ".env"), []byte("PRIVATE_SENTINEL"), 0600)
	secret := filepath.Join(root, "secret")
	os.WriteFile(secret, []byte("SYMLINK_SENTINEL"), 0600)
	os.Mkdir(filepath.Join(repo, "docs"), 0700)
	os.Symlink(secret, filepath.Join(repo, "docs", "linked.md"))
	run("add", "README.md", ".env", "docs/linked.md")
	s, _ := setupTestChatServer(t)
	store := project.NewMemoryStore()
	s.projectSvc = project.NewService(store, nil, "", root)
	for _, p := range []*project.Project{{ID: "alpha", Name: "alpha", RepoOwner: "test", RepoName: "alpha", RepoURL: "https://github.com/test/alpha", LocalPath: repo}, {ID: "beta", Name: "beta", RepoOwner: "test", RepoName: "beta", RepoURL: "https://github.com/test/beta", LocalPath: filepath.Join(root, "missing")}} {
		if e := store.CreateProject(ctx, p); e != nil {
			t.Fatal(e)
		}
	}
	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)
	_, e := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "alpha", Title: "Alpha backlog evidence"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "beta", Title: "OTHER_PROJECT_SENTINEL"})
	if e != nil {
		t.Fatal(e)
	}
	room := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "DM", Topic: "agent:pin2"})
	msg := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "user", Content: "cek repo alpha sarankan sprint", CreatedAt: time.Now()}
	s.projectStudioRepo.CreateMessage(ctx, &msg)
	q := db.InboxRequest{RoomID: room, MessageID: msg.ID}
	dir, snapshot := s.inboxProjectContext(ctx, q)
	if dir != repo || !strings.Contains(snapshot, "architecture evidence") || !strings.Contains(snapshot, "Alpha backlog evidence") {
		t.Fatal(dir, snapshot)
	}
	for _, bad := range []string{"PRIVATE_SENTINEL", "SYMLINK_SENTINEL", "OTHER_PROJECT_SENTINEL"} {
		if strings.Contains(snapshot, bad) {
			t.Fatal("Cross-boundary data in snapshot", bad)
		}
	}
	// A later queued request cannot choose this turn's project.
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "user", Content: "cek beta", CreatedAt: time.Now().Add(time.Second)})
	dir, _ = s.inboxProjectContext(ctx, q)
	if dir != repo {
		t.Fatal("future message changed project")
	}
	// Project rooms stay pinned even when another project is named in the message.
	pinned := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: pinned, ProjectID: "alpha", Title: "Project", Topic: "discussion"})
	q.RoomID = pinned
	dir, _ = s.inboxProjectContext(ctx, q)
	if dir != repo {
		t.Fatal("pinned room lost project")
	}
	// Missing target must not silently select another workspace.
	q.RoomID = room
	q.MessageID = db.GenerateUUID()
	dir, snapshot = s.inboxProjectContext(ctx, q)
	if dir != "" || !strings.Contains(snapshot, "ambigu") {
		t.Fatal("ambiguous project selected", dir, snapshot)
	}
}

func TestInboxProjectCLIUsesWorkspaceAndRetainsProfile(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n%s\\n%s' \"$HOME\" \"$PWD\" \"$*\"\n"
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s, _ := setupTestChatServer(t)
	s.cfg.Environment = "staging"
	out, failure := s.executeInboxCLI(context.Background(), "pin2", "agy", "Inspect README", home, workspace, nil)
	if failure != "" || !strings.HasPrefix(out, home+"\n"+workspace+"\n") || !strings.Contains(out, "--mode plan --sandbox") || strings.Contains(out, "Jangan gunakan tools") {
		t.Fatal(out, failure)
	}
}
