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
	// A prior same-conversation task anchors a follow-up that omits its name.
	_, e = s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "alpha", Title: "Bound work", SourceRoomID: room, SourceMessageID: msg.ID})
	if e != nil {
		t.Fatal(e)
	}
	follow := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "user", Content: "Continue the approved work", CreatedAt: time.Now().Add(time.Minute)}
	s.projectStudioRepo.CreateMessage(ctx, &follow)
	followQ := db.InboxRequest{RoomID: room, MessageID: follow.ID}
	dir, _ = s.inboxProjectContext(ctx, followQ)
	if dir != repo {
		t.Fatal("follow-up lost bound project", dir)
	}
	// An explicit different project still takes precedence over retained context.
	follow.Content = "inspect beta"
	explicit := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "user", Content: follow.Content, CreatedAt: follow.CreatedAt}
	s.projectStudioRepo.CreateMessage(ctx, &explicit)
	_, snapshot = s.inboxProjectContext(ctx, db.InboxRequest{RoomID: room, MessageID: explicit.ID})
	if !strings.Contains(snapshot, "beta [beta]") {
		t.Fatal("explicit project overridden", snapshot)
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
	if dir != "" || !strings.Contains(snapshot, "ambiguous") {
		t.Fatal("ambiguous project selected", dir, snapshot)
	}
}

func TestInboxProjectCLIUsesWorkspaceAndRetainsProfile(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	script := "#!/bin/sh\npython3 - \"$*\" <<'PY'\nimport os,sys,json\nopen(os.path.join(os.environ['HOME'],'cli-args'),'w').write(sys.argv[1])\nprint(json.dumps({'event':'result','result':{'status':'SUCCESS','response':json.dumps({'reply':os.environ['HOME']+'\\n'+os.getcwd(),'topic_title':'','action':None})}}))\nPY\n"
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s, _ := setupTestChatServer(t)
	s.cfg.Environment = "staging"
	out, failure := s.executeInboxCLI(context.Background(), "pin2", "agy", "Inspect README", home, workspace, nil)
	args, _ := os.ReadFile(filepath.Join(home, "cli-args"))
	argText := string(args)
	if failure != "" || out != home+"\n"+workspace || strings.Contains(argText, "--dangerously-skip-permissions") || !strings.Contains(argText, "--mode plan") || !strings.Contains(argText, "--sandbox") || !strings.Contains(argText, "--json-schema") || !strings.Contains(argText, "Do not use terminal") {
		t.Fatal(out, failure)
	}
}

func TestInboxProjectContextMetricsAndBlockerDiagnostics(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	os.Mkdir(repo, 0700)
	c := exec.Command("git", "-C", repo, "init", "-q")
	_ = c.Run()
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Test Repo\n"), 0600)
	_ = exec.Command("git", "-C", repo, "add", "README.md").Run()
	_ = exec.Command("git", "-C", repo, "-c", "user.name=test", "-c", "user.email=test@test.local", "commit", "-m", "init").Run()

	s, _ := setupTestChatServer(t)
	s.cfg.Environment = "staging"
	store := project.NewMemoryStore()
	s.projectSvc = project.NewService(store, nil, "", root)
	_ = store.CreateProject(ctx, &project.Project{ID: "p_diag", Name: "Diagnostic Project", LocalPath: repo})

	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)
	t1, _ := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "p_diag", Title: "Completed work"})
	statusDoing := task.TaskStatusDoing
	statusReview := task.TaskStatusReview
	statusDone := task.TaskStatusDone
	note := "Verified completion evidence"
	_, _ = s.taskSvc.UpdateTask(ctx, t1.ID, "user", task.UpdateTaskRequest{Status: &statusDoing})
	_, _ = s.taskSvc.UpdateTask(ctx, t1.ID, "user", task.UpdateTaskRequest{Status: &statusReview, CompleteNote: &note})
	_, _ = s.taskSvc.UpdateTask(ctx, t1.ID, "user", task.UpdateTaskRequest{Status: &statusDone})

	t2, _ := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "p_diag", Title: "Failing database migration"})
	statusBlocked := task.TaskStatusBlocked
	_, _ = s.taskSvc.UpdateTask(ctx, t2.ID, "user", task.UpdateTaskRequest{Status: &statusDoing})
	_, _ = s.taskSvc.UpdateTask(ctx, t2.ID, "user", task.UpdateTaskRequest{Status: &statusBlocked})

	room := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "p_diag", Title: "SM Room", Topic: "discussion"})
	msg := db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "user", Content: "Status check", CreatedAt: time.Now()}
	s.projectStudioRepo.CreateMessage(ctx, &msg)

	q := db.InboxRequest{RoomID: room, MessageID: msg.ID}
	dir, snapshot := s.inboxProjectContext(ctx, q)
	if dir != repo {
		t.Fatalf("expected workspace %s, got %s", repo, dir)
	}

	if !strings.Contains(snapshot, "Sprint & Backlog Metrics: Total=2, Done=1, Review=0, Doing=0, Blocked=1, Backlog=0") {
		t.Fatalf("missing sprint metrics in snapshot: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Active Blockers / Failed Attempts Needing Review:") || !strings.Contains(snapshot, "Failing database migration") {
		t.Fatalf("missing blocker diagnostics in snapshot: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Live Deployment Environment: env=staging") {
		t.Fatalf("missing live deployment environment in snapshot: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Git Working State:") {
		t.Fatalf("missing git working state in snapshot: %s", snapshot)
	}
}
