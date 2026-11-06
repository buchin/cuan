package server

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/db"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskPushConnectionRecoveryRealGit(t *testing.T)        { runTaskPushRecovery(t, false) }
func TestGitHubQueuedProcessContinuationRealGit(t *testing.T)   { runTaskPushRecovery(t, true) }
func TestNativeTaskPushUsesActualExecutionRealGit(t *testing.T) { runTaskPushRecovery(t, false, true) }
func runTaskPushRecovery(t *testing.T, automatic bool, nativeMode ...bool) {
	native := len(nativeMode) > 0 && nativeMode[0]
	if testing.Short() {
		t.Skip("skipping slow real git push recovery test in short mode")
	}
	s, ownerToken, _, _ := inboxFixture(t)
	ctx := context.Background()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	remote := filepath.Join(root, "remote.git")
	actualGit, e := exec.LookPath("git")
	if e != nil {
		t.Fatal(e)
	}
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command(actualGit, args...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--bare", remote)
	run("init", "--initial-branch=main", source)
	os.WriteFile(filepath.Join(source, "README"), []byte("initial"), 0600)
	run("-C", source, "add", "README")
	run("-C", source, "commit", "-m", "Fixture initial")
	run("-C", source, "update-ref", "refs/remotes/origin/main", "HEAD")
	projects := project.NewMemoryStore()
	p := &project.Project{ID: "repo-task-project", Name: "Repo task", RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main", UpdatedAt: time.Now()}
	projects.CreateProject(ctx, p)
	client := cuangit.NewClient()
	if native {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer fixture-token" {
				t.Error("wrong GitHub identity")
			}
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "cuan-project-context") {
				t.Error("private project snapshot sent to GitHub")
			}
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "GET" {
				io.WriteString(w, `[]`)
				return
			}
			w.WriteHeader(201)
			io.WriteString(w, `{"number":7,"html_url":"https://github.com/fixture/repo/pull/7","state":"open"}`)
		}))
		defer api.Close()
		client.SetAPIBaseURL(api.URL)
	}
	svc := project.NewService(projects, client, s.cfg.AuthSecret, filepath.Join(root, "workspaces"))
	s.SetProjectService(svc)
	s.taskSvc = task.NewService(task.NewMemoryStore(), svc)
	if native {
		t.Setenv("GH_TOKEN", "ambient-wrong-owner")
		t.Setenv("GITHUB_TOKEN", "ambient-wrong-owner")
		s.cfg.WorkspacesDir = filepath.Join(root, "native-workspaces")
		s.runnerSvc = runner.NewService(runner.NewMemoryStore(s.taskSvc.Store()), s.taskSvc.Store())
		if _, e := s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: "native-push", Hostname: "fixture"}, time.Minute); e != nil {
			t.Fatal(e)
		}
	}
	owner, _ := s.authSvc.ValidateToken(ctx, ownerToken)
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: "approved"}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Create one result and save it", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	created, e := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{SourceRoomID: q.RoomID, SourceMessageID: q.MessageID, AssignedAgent: q.AgentID, ProjectID: p.ID, Title: "Approved repository work", Instructions: "Create one result and save it"})
	if e != nil {
		t.Fatal(e)
	}
	runnerID := "local_terminal"
	if native {
		runnerID = "native-push"
	}
	attempt, e := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: runnerID, AgentName: "agent_lifecycle"})
	if e != nil {
		t.Fatal(e)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	issueCtx := ctx
	nativeWorkspace := ""
	if native {
		nativeWorkspace = filepath.Join(s.cfg.WorkspacesDir, created.ID)
		run("clone", source, nativeWorkspace)
		run("-C", nativeWorkspace, "remote", "set-url", "origin", "https://github.com/fixture/repo.git")
		run("-C", nativeWorkspace, "checkout", "-b", "task/"+created.ID)
		issueCtx = context.WithValue(ctx, taskAgentWorkspaceKey{}, source)
	}
	_, _, capability, cleanup, e := s.issueTaskAgent(issueCtx, created.ID, attempt.ID, "agent_lifecycle")
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	operation := "push"
	push := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/task-agent/"+created.ID+"/"+operation, nil)
		r.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	first := push()
	if first.Code != 409 || !strings.Contains(first.Body.String(), "github_connection_required") {
		t.Fatal(first.Code, first.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	response, _ := svc.TaskWorkspace(ctx, p.ID, detail.Instructions)
	workspace, e := svc.TaskExecutionWorkspace(ctx, created.ID, detail.Instructions, response)
	if e != nil {
		t.Fatal(e)
	}
	if native {
		// Retain a conflicting server worktree to prove it is never published.
		os.WriteFile(filepath.Join(workspace, "decoy.txt"), []byte("stale server worktree"), 0600)
		workspace = nativeWorkspace
	}
	os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("one delivery"), 0600)
	run("-C", workspace, "add", "result.txt")
	run("-C", workspace, "commit", "-m", "Implement once")
	commit := run("-C", workspace, "rev-parse", "HEAD")
	quote := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
	bin := t.TempDir()
	wrapper := "#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\na=[" + quote(remote) + " if x=='https://github.com/fixture/repo.git' else x for x in a]\nos.execv(" + quote(actualGit) + ",['git']+a)\n"
	os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	path, _ := githubConnectionPath(owner.ID)
	if e = cuangit.SaveConnection(path, s.cfg.AuthSecret, "fixture-token", cuangit.Account{Login: "fixture"}); e != nil {
		t.Fatal(e)
	}
	if automatic {
		failed := task.AttemptStatusFailed
		if _, err := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: failed}); err != nil {
			t.Fatal(err)
		}
		blocked := task.TaskStatusBlocked
		if _, err := s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &blocked}); err != nil {
			t.Fatal(err)
		}
		pending, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
		script := `#!/bin/sh
set -eu
test "$(cat result.txt)" = 'one delivery'
echo continued >> "$HOME/github-continuations"
"$CUAN_TASK_EXECUTABLE" task push
"$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Existing result saved to GitHub' --complete-note 'Verified retained result, one implementation commit, and task push'
while :;do sleep 0.1;done
`
		os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
		api := httptest.NewServer(s.Router())
		defer api.Close()
		s.taskAgentURL = api.URL
		for i := 0; i < 2; i++ {
			if err := s.queueGitHubRecovery(ctx, pending, owner.ID); err != nil {
				t.Fatal(err)
			}
		}
		s.drainTaskLaunches(ctx, time.Now().UTC())
		job, _ := s.taskSvc.LatestLaunch(ctx, created.ID)
		if job == nil || job.State != "started" {
			t.Fatalf("queued continuation not started: %+v", job)
		}
		completed := waitWorkspaceTask(t, s, created.ID, task.AttemptStatusSucceeded)
		// Inbox work waits for coordinator acceptance after a successful process;
		// an agent's done report and push alone must not bypass that review.
		if completed.Status != task.TaskStatusReview || len(completed.Attempts) != 2 {
			t.Fatal("continuation did not preserve same task for review exactly once", completed.Status, len(completed.Attempts))
		}
		s.drainTaskLaunches(ctx, time.Now().UTC())
		calls, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), "github-continuations"))
		if string(calls) != "continued\n" {
			t.Fatal("continuation duplicated", string(calls))
		}
	} else {
		for i := 0; i < 2; i++ {
			saved := push()
			if saved.Code != 200 || !strings.Contains(saved.Body.String(), commit) {
				t.Fatal(saved.Code, saved.Body.String())
			}
		}
		if native {
			operation = "publish"
			if published := push(); published.Code != 200 || !strings.Contains(published.Body.String(), `"number":7`) {
				t.Fatal("bound publication route failed", published.Code, published.Body.String())
			}
			operation = "push"
			branch := "task/" + created.ID
			if got := run("--git-dir", remote, "rev-parse", "refs/heads/"+branch); got != commit {
				t.Fatal("wrong checkout published", got, commit)
			}
			run("-C", workspace, "remote", "set-url", "origin", "https://github.com/other/repo.git")
			if rejected := push(); rejected.Code != 409 {
				t.Fatal("foreign repository accepted", rejected.Code)
			}
			run("-C", workspace, "remote", "set-url", "origin", "https://github.com/fixture/repo.git")
			run("-C", workspace, "checkout", "main")
			if rejected := push(); rejected.Code != 409 {
				t.Fatal("changed native branch accepted", rejected.Code)
			}
			run("-C", workspace, "checkout", branch)
		}
	}
	final, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	required, resolved := 0, 0
	for _, event := range final.Activities {
		if event.ActivityType == "github_connection_required" {
			required++
		}
		if event.ActivityType == "github_connection_resolved" {
			resolved++
		}
	}
	if required != 1 || resolved != 1 {
		t.Fatal("prerequisite state duplicated", required, resolved)
	}
	if strings.Count(run("-C", workspace, "log", "--format=%s"), "Implement once") != 1 {
		t.Fatal("push recovery duplicated implementation")
	}
	if final.Instructions != detail.Instructions {
		t.Fatal("connection changed original instructions")
	}
}
