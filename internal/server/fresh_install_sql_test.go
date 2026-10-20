package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// This uses only a disposable database and deterministic provider executable.
// Unlike the older fixtures, it does not seed an owner or substitute memory stores.
func TestFreshInstallPostgresGoalLifecycle(t *testing.T) {
	dsn := os.Getenv("CUAN_FRESH_INSTALL_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires dedicated fresh-install PostgreSQL database")
	}
	u, err := url.Parse(dsn)
	if err != nil || !strings.HasPrefix(u.Path, "/cuan_fresh_install_") {
		t.Fatal("dedicated cuan_fresh_install_* database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	var count int
	if err = database.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("database must be empty", count, err)
	}
	if err = db.Run(ctx, database, "up"); err != nil {
		t.Fatal(err)
	}
	oldUsage := fetchInboxModelUsage
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage { return agent.AccountModelUsage{} }
	defer func() { fetchInboxModelUsage = oldUsage }()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	bin := t.TempDir()
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	cfg := &config.Config{Environment: "staging", AuthSecret: strings.Repeat("fixture-only-", 4), WorkspacesDir: filepath.Join(home, "workspaces")}
	s := NewServer(cfg, database)
	defer func() { s.terminalManager.CloseAll() }()
	token := ""
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		out := httptest.NewRecorder()
		s.Router().ServeHTTP(out, req)
		return out
	}
	status := call("GET", "/api/v1/auth/status", "")
	var initial struct {
		SetupRequired   bool `json:"setup_required"`
		OwnerConfigured bool `json:"owner_configured"`
	}
	if err = json.Unmarshal(status.Body.Bytes(), &initial); err != nil || !initial.SetupRequired || initial.OwnerConfigured {
		t.Fatal("fresh database skipped owner setup", status.Body.String(), err)
	}
	if unauthorized := call("GET", "/api/v1/tasks", ""); unauthorized.Code != 401 {
		t.Fatal("unowned workspace exposed", unauthorized.Code)
	}
	setup := call("POST", "/api/v1/auth/setup", `{"email":"fresh@example.invalid","password":"fixture-password-12345"}`)
	if setup.Code != 201 {
		t.Fatal("owner setup", setup.Code, setup.Body.String())
	}
	var session struct {
		Token string `json:"token"`
	}
	json.Unmarshal(setup.Body.Bytes(), &session)
	token = session.Token
	if token == "" {
		t.Fatal("missing owner session")
	}
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if again := call("POST", "/api/v1/auth/setup", `{"email":"other@example.invalid","password":"fixture-password-12345"}`); again.Code != 403 {
		t.Fatal("second owner setup", again.Code)
	}
	p, err := s.projectSvc.Get(ctx, "default")
	if err != nil || p.Name != "Default" || p.RepoURL != "" {
		t.Fatal("fresh default project is not neutral", p, err)
	}
	if noAgent := call("POST", "/api/v1/studio/goals", `{"request_id":"`+db.GenerateUUID()+`","goal":"Help me"}`); noAgent.Code != 503 || !strings.Contains(noAgent.Body.String(), "Settings") {
		t.Fatal("missing account guidance", noAgent.Code, noAgent.Body.String())
	}
	account, err := agent.CreateProviderAccount(owner.ID, "fresh fixture")
	if err != nil {
		t.Fatal(err)
	}
	accountHome, err := agent.ProviderAccountHome(owner.ID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(agent.AntigravityConfigDirPath(accountHome), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(agent.AntigravityCredentialsFilePath(accountHome), []byte("fixture-only-credential-value"), 0600); err != nil {
		t.Fatal(err)
	}
	delegated, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": "```cuan-delegate\n{\"title\":\"Fresh work\",\"description\":\"Verify the requested result. Do not deploy.\",\"project_id\":\"default\",\"authorization_message_id\":\"AUTH\"}\n```"}})
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 echo executed >> "$HOME/fresh-executions"
 "$CUAN_TASK_EXECUTABLE" task report --progress 'Checking original instruction' >/dev/null || exit 20
 "$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Fresh install work complete' --complete-note 'Verified one execution; no deployment' >/dev/null || exit 21
 while :; do sleep 1; done
fi
case "$2" in
 *'automated result is not new user authorization'*) printf '%s' '{"event":"result","result":{"status":"SUCCESS","response":"Verified fresh-install result. No deployment."}}';;
 *) target="${2##*Target message ID: }"; printf '%s' '` + string(delegated) + `' | sed "s/AUTH/$target/g";;
esac
`
	if err = os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	connected := call("POST", "/api/v1/agents/connect-cli", `{"name":"@manager","handle":"@manager","provider":"agy","account_id":"`+account.ID+`"}`)
	if connected.Code != 201 {
		t.Fatal("connect account", connected.Code, connected.Body.String())
	}
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	id := db.GenerateUUID()
	body := `{"request_id":"` + id + `","goal":"Verify the requested result now. Do not deploy."}`
	accepted := call("POST", "/api/v1/studio/goals", body)
	if accepted.Code != 202 {
		t.Fatal("Studio acceptance", accepted.Code, accepted.Body.String())
	}
	var result struct {
		ConversationID string `json:"conversation_id"`
	}
	json.Unmarshal(accepted.Body.Bytes(), &result)
	waitInboxState(t, s, result.ConversationID, id, "completed")
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatal("one delegated task required", len(tasks), err)
	}
	finished := waitWorkspaceTask(t, s, tasks[0].ID, "succeeded")
	if finished.Status != "done" || !strings.Contains(finished.Instructions, "Do not deploy") {
		t.Fatal("result lost scope or completion", finished)
	}
	s.deliverInboxTaskUpdates(ctx)
	waitInboxState(t, s, result.ConversationID, delegationMessageID("task-review:"+finished.LatestAttempt.ID), "completed")
	if replay := call("POST", "/api/v1/studio/goals", body); replay.Code != 202 {
		t.Fatal("goal replay", replay.Code)
	}
	renamed := "My general work"
	saved, err := s.projectSvc.Store().GetProject(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	saved.Name = renamed
	saved.RepoURL = "https://example.invalid/my-project"
	if err = s.projectSvc.Store().UpdateProject(ctx, saved); err != nil {
		t.Fatal(err)
	}
	s.terminalManager.CloseAll()
	s = NewServer(cfg, database)
	if err = db.Run(ctx, database, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.authSvc.ValidateToken(ctx, token); err != nil {
		t.Fatal("owner session lost on reconstruction", err)
	}
	preserved, err := s.projectSvc.Get(ctx, "default")
	if err != nil || preserved.Name != renamed || preserved.RepoURL != saved.RepoURL {
		t.Fatal("restart rewrote project", preserved, err)
	}
	tasks, err = s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatal("task lost or duplicated", len(tasks), err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, tasks[0].ID)
	if err != nil || detail.Status != "done" || len(detail.Attempts) != 1 {
		t.Fatal("execution not durable", detail, err)
	}
	effects, err := os.ReadFile(filepath.Join(accountHome, "fresh-executions"))
	if err != nil || string(effects) != "executed\n" {
		t.Fatal("duplicate or missing execution", err)
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, result.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := 0
	for _, m := range messages {
		if m.Content == "Verified fresh-install result. No deployment." {
			outcomes++
		}
	}
	if outcomes != 1 {
		t.Fatal("final outcome not durable/unique", outcomes)
	}
	t.Log("Fresh schema25 -> owner setup -> account connection -> Studio goal -> one verified task/result -> reconstruction retention passed; no paid provider invocation")
	time.Sleep(20 * time.Millisecond)
}
