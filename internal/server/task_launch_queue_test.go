package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func TestQueuedLaunchRecoversAfterRouteReleaseAndReconstruction(t *testing.T) {
	for _, scenario := range []string{"resume", "cancel", "scope-changed"} {
		t.Run(scenario, func(t *testing.T) { runQueuedLaunchScenario(t, scenario, nil) })
	}
}
func runQueuedLaunchScenario(t *testing.T, scenario string, database *sql.DB) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	var tasks task.Store = task.NewMemoryStore()
	if database != nil {
		tasks = task.NewDBStore(database)
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
		if _, e := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Default',''),('queued-project','Queued project','')`); e != nil {
			t.Fatal(e)
		}
	}
	s.taskSvc = task.NewService(tasks, nil)
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "queued-project", Name: "Queued project"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	account, e := agent.CreateProviderAccount(owner.ID, "queued fixture")
	if e != nil {
		t.Fatal(e)
	}
	home, _ := agent.ProviderAccountHome(owner.ID, account.ID)
	os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
	os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600)
	if w := call("POST", "/api/v1/agents/connect-cli", `{"name":"@queued","handle":"@queued","provider":"agy","account_id":"`+account.ID+`"}`); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	metaPath := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", account.ID, "account.json")
	raw, _ := os.ReadFile(metaPath)
	var meta map[string]any
	json.Unmarshal(raw, &meta)
	meta["state"] = "unknown"
	raw, _ = json.Marshal(meta)
	os.WriteFile(metaPath, raw, 0600)
	available := false
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
		remaining := 0.0
		if available {
			remaining = 80
		}
		return agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "claude-sonnet-test", Remaining: remaining}}}
	}
	script := `#!/bin/sh
echo started >> "$HOME/queued-executions"
"$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Queued work complete' --complete-note 'Verified one execution after route release' >/dev/null || exit 21
while :;do sleep 1;done
`
	os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700)
	room := s.resolveConversationContext(ctx, "agent_queued", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_queued", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: "authorized"}
	_, _, e = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Run a read-only audit. Do not deploy.", CreatedAt: now})
	if e != nil {
		t.Fatal(e)
	}
	d := inboxDelegation{Title: "Read-only audit", Description: "Audit and report evidence. Do not deploy.", ProjectID: "queued-project", AuthorizationID: q.MessageID}
	raw, _ = json.Marshal(d)
	content := "```cuan-delegate\n" + string(raw) + "\n```"
	if out := s.applyInboxDelegation(ctx, q, content); !strings.Contains(out, "waiting") {
		t.Fatal(out)
	}
	list, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(list) != 1 {
		t.Fatal("task not retained")
	}
	id := list[0].ID
	detail, _ := s.taskSvc.GetTaskDetail(ctx, id)
	if len(detail.Attempts) != 0 || detail.Launch == nil || detail.Launch.State != "queued" {
		t.Fatal("attempt started before route available")
	}
	if scenario == "cancel" {
		w := call("POST", "/api/v1/tasks/"+id+"/cancel", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if scenario == "scope-changed" {
		changed := "Different scope"
		s.taskSvc.UpdateTask(ctx, id, "owner", task.UpdateTaskRequest{Instructions: &changed})
	}
	// Reconstruct service/manager/server over the same persisted stores. No in-memory pending list is reused.
	recovered, _ := setupTestChatServer(t)
	recovered.cfg.Environment = "staging"
	recovered.authSvc = s.authSvc
	recovered.projectStudioRepo = s.projectStudioRepo
	if database != nil {
		recovered.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
	}
	recovered.projectSvc = s.projectSvc
	recovered.taskSvc = task.NewService(tasks, nil)
	if database != nil {
		recovered.taskSvc = task.NewService(task.NewDBStore(database), nil)
	}
	recovered.SetAgentRepository(s.agentRepo)
	recovered.SetTerminalManager(terminal.NewManager(terminal.ManagerConfig{WorkspacesDir: t.TempDir()}))
	t.Cleanup(func() { recovered.terminalManager.CloseAll() })
	api := httptest.NewServer(recovered.Router())
	defer api.Close()
	recovered.taskAgentURL = api.URL
	available = true
	recovered.drainTaskLaunches(ctx, now.Add(time.Minute))
	job, e := recovered.taskSvc.LatestLaunch(ctx, id)
	if e != nil || job == nil {
		t.Fatal(job, e)
	}
	if scenario == "cancel" {
		if job.State != "cancelled" {
			t.Fatal("cancelled job revived")
		}
		return
	}
	if scenario == "scope-changed" {
		if job.State != "stopped" || !strings.Contains(job.SafeError, "scope has changed") {
			t.Fatal(job)
		}
		if _, e := os.Stat(filepath.Join(home, "queued-executions")); !os.IsNotExist(e) {
			t.Fatal("changed scope executed")
		}
		return
	}
	if job.State != "started" {
		t.Fatal("dispatcher did not start work", job)
	}

	result := waitWorkspaceTask(t, recovered, id, "succeeded")
	if result.Status != "done" || len(result.Attempts) != 1 {
		t.Fatal(result.Status, len(result.Attempts))
	}
	if out := recovered.applyInboxDelegation(ctx, q, content); !strings.Contains(out, "already handled") {
		t.Fatal(out)
	}
	calls, _ := os.ReadFile(filepath.Join(home, "queued-executions"))
	if string(calls) != "started\n" {
		t.Fatal("duplicate queued execution", string(calls))
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	starts := 0
	for _, m := range messages {
		if strings.HasPrefix(m.Content, "Work started") {
			starts++
		}
	}
	if starts != 1 {
		t.Fatal("missing or duplicate launch notification", starts)
	}
}
