package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func setupFailoverFixture(t *testing.T) (*Server, string, *db.Agent, *agent.ProviderAccount, *agent.ProviderAccount, string, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	create := func(handle string) (*agent.ProviderAccount, *db.Agent, string) {
		a, err := agent.CreateProviderAccount(owner.ID, handle)
		if err != nil {
			t.Fatal(err)
		}
		home, _ := agent.ProviderAccountHome(owner.ID, a.ID)
		os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
		os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600)
		body := `{"name":"` + handle + `","handle":"` + handle + `","provider":"agy","account_id":"` + a.ID + `"}`
		r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		all, err := s.agentRepo.GetAgentByID(context.Background(), "agent_"+strings.TrimPrefix(handle, "@"))
		if err != nil {
			t.Fatal(err)
		}
		a, _ = agent.GetProviderAccount(owner.ID, a.ID)
		return a, all, home
	}
	primary, registered, home := create("@main")
	backup, _, backupHome := create("@backup")
	bin := t.TempDir()
	script := `#!/bin/sh
printf x >> "$HOME/calls"
case "$2" in *"You are @main"*) ;; *) exit 9;; esac
if [ -f "$HOME/limited" ]; then
 if [ -f "$HOME/unsafe" ]; then printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command"}}'; fi
 if [ -f "$HOME/read" ]; then printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"view_file"}}'; fi
 if [ -f "$HOME/auth" ]; then printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"invalid_grant private-token"}}';else printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"RESOURCE_EXHAUSTED private-token"}}';fi
 exit 3
fi
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"Same agent backup answer"}}'
`
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s.cfg.Environment = "staging"
	for _, item := range []*agent.ProviderAccount{primary, backup} {
		file := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", item.ID, "account.json")
		data, _ := os.ReadFile(file)
		var meta map[string]any
		json.Unmarshal(data, &meta)
		meta["state"] = "unknown"
		data, _ = json.Marshal(meta)
		os.WriteFile(file, data, 0600)
	}

	return s, token, registered, primary, backup, home, backupHome
}

func TestInboxQuotaFailover(t *testing.T) {
	for _, scenario := range []string{"success", "read-only", "unsafe", "unknown-tool", "not-opted-in", "backup-quota", "auth", "primary-known-quota", "disabled-backup", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, registered, p, b, home, backupHome := setupFailoverFixture(t)
			owner := p.OwnerID
			if scenario != "not-opted-in" {
				if _, err := agent.SetProviderAccountFailover(owner, b.ID, true); err != nil {
					t.Fatal(err)
				}
			}
			os.WriteFile(filepath.Join(home, "limited"), nil, 0600)
			if scenario == "unsafe" || scenario == "unknown-tool" {
				os.WriteFile(filepath.Join(home, "unsafe"), nil, 0600)
			}
			if scenario == "unknown-tool" {
				bin := strings.Split(os.Getenv("PATH"), ":")[0]
				f := filepath.Join(bin, "agy")
				data, _ := os.ReadFile(f)
				os.WriteFile(f, []byte(strings.ReplaceAll(string(data), "run_command", "unrecognized_tool")), 0700)
			}
			if scenario == "read-only" {
				os.WriteFile(filepath.Join(home, "read"), nil, 0600)
			}
			if scenario == "auth" {
				os.WriteFile(filepath.Join(home, "auth"), nil, 0600)
			}
			if scenario == "backup-quota" {
				os.WriteFile(filepath.Join(backupHome, "limited"), nil, 0600)
			}
			if scenario == "disabled-backup" {
				a, _ := s.agentRepo.GetAgentByID(context.Background(), b.AgentID)
				a.Enabled = false
				a.Status = db.AgentStatusInactive
				if err := s.agentRepo.UpdateAgent(context.Background(), a); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "cancelled" {
				cancel()
			}
			var trace []string
			out, failure := s.executeInboxWithFailover(ctx, owner, registered, "Analyze this project", home, t.TempDir(), scenario == "primary-known-quota", func(k, v string) { trace = append(trace, k+":"+v) })
			success := scenario == "success" || scenario == "read-only" || scenario == "primary-known-quota"
			if success && (out != "Same agent backup answer" || failure != "") {
				t.Fatal(out, failure)
			}
			if !success && (out != "" || failure == "") {
				t.Fatal(out, failure)
			}
			backupCalls, _ := os.ReadFile(filepath.Join(backupHome, "calls"))
			wantBackup := success || scenario == "backup-quota"
			if wantBackup != (string(backupCalls) == "x") {
				t.Fatal("incorrect backup executions", string(backupCalls))
			}
			current, _ := agent.GetProviderAccount(owner, p.ID)
			if current.AgentID != registered.ID || current.ConnectionID != registered.ConnectionID {
				t.Fatal("primary binding changed")
			}
			if strings.Contains(failure+strings.Join(trace, "\n"), "private-token") {
				t.Fatal("diagnostic leaked")
			}
		})
	}
}

func TestFailoverPreferenceOwnerAPI(t *testing.T) {
	s, token, _, p, b, _, _ := setupFailoverFixture(t)
	request := func(token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/api/v1/provider-accounts/"+b.ID+"/failover", strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := request("", `{"enabled":true}`); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(token, `{}`); w.Code != 400 {
		t.Fatal(w.Code)
	}
	for _, value := range []string{"true", "false"} {
		w := request(token, `{"enabled":`+value+`}`)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct {
			Account agent.ProviderAccount `json:"account"`
		}
		json.Unmarshal(w.Body.Bytes(), &result)
		if result.Account.AllowFailover != (value == "true") {
			t.Fatal(w.Body.String())
		}
	}
	if _, err := agent.SetProviderAccountFailover("another-owner", b.ID, true); err == nil {
		t.Fatal("cross-owner account allowed")
	}
	if _, err := agent.ProviderAccountForConnection(p.OwnerID, p.AgentID, p.ConnectionID); err != nil {
		t.Fatal("primary binding lost")
	}
}

func TestInboxModelQuotaRouting(t *testing.T) {
	for _, scenario := range []string{"same-account-other-model", "highest-quota-first", "model-policy-off", "model-cooldown"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, registered, p, b, home, backupHome := setupFailoverFixture(t)
			owner := p.OwnerID
			registered.ModelID = "gemini-test"
			agent.SetProviderAccountFailover(owner, b.ID, true)
			agent.SetProviderAccountModelFallback(owner, p.ID, scenario != "model-policy-off")
			agent.SetProviderAccountModelFallback(owner, b.ID, true)
			oldFetch := fetchInboxModelUsage
			defer func() { fetchInboxModelUsage = oldFetch }()
			fetchInboxModelUsage = func(_ context.Context, _ string, id string) agent.AccountModelUsage {
				models := []agent.ModelUsage{{Model: "gemini-test", Remaining: 0}, {Model: "claude-sonnet-test", Remaining: 80}}
				if scenario == "highest-quota-first" {
					models = []agent.ModelUsage{{Model: "gemini-test", Remaining: 30}}
					if id == b.ID {
						models[0].Remaining = 90
					}
				}
				if id == b.ID && scenario != "highest-quota-first" {
					return agent.AccountModelUsage{}
				}
				return agent.AccountModelUsage{Available: true, Models: models}
			}
			if scenario == "model-cooldown" {
				agent.RecordProviderModelQuota(owner, p.ID, "claude-sonnet-test")
			}
			var trace []string
			out, failure := s.executeInboxWithFailover(context.Background(), owner, registered, "Analyze", home, t.TempDir(), false, func(k, v string) { trace = append(trace, k+":"+v) })
			if out == "" || failure != "" {
				t.Fatal(out, failure)
			}
			primaryCalls, _ := os.ReadFile(filepath.Join(home, "calls"))
			backupCalls, _ := os.ReadFile(filepath.Join(backupHome, "calls"))
			if scenario == "same-account-other-model" {
				if string(primaryCalls) != "x" || len(backupCalls) > 0 || !strings.Contains(strings.Join(trace, "\n"), "claude-sonnet-test") {
					t.Fatal("did not route same-account Sonnet", trace)
				}
			} else if len(primaryCalls) > 0 || string(backupCalls) != "x" {
				t.Fatal("did not choose eligible backup first", scenario)
			}
		})
	}
}

func TestInboxFailoverSingleDurableReply(t *testing.T) {
	s, token, _, p, b, home, backupHome := setupFailoverFixture(t)
	agent.SetProviderAccountFailover(p.OwnerID, b.ID, true)
	os.WriteFile(filepath.Join(home, "limited"), nil, 0600)
	request := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/chat/conversations/agent_main/messages", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	request("GET", "")
	id := db.GenerateUUID()
	body := `{"message":"Explain the project","request_id":"` + id + `"}`
	if w := request("POST", body); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	for deadline := time.Now().Add(3 * time.Second); ; {
		w := request("GET", "")
		var data struct {
			Messages []struct {
				AuthorID string `json:"author_id"`
				Content  string `json:"content"`
			}
			Requests []db.InboxRequest
		}
		json.Unmarshal(w.Body.Bytes(), &data)
		if len(data.Requests) == 1 && data.Requests[0].State == "completed" {
			if len(data.Messages) != 2 || !strings.Contains(w.Body.String(), "Same agent backup answer") || data.Requests[0].AgentID != "agent_main" {
				t.Fatal(w.Body.String())
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(w.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w := request("POST", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, dir := range []string{home, backupHome} {
		calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
		if string(calls) != "x" {
			t.Fatal("duplicate execution", string(calls))
		}
	}
}

func TestTaskProviderPreflightChoosesBackupWithoutChangingAgent(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	account, _ := agent.CreateProviderAccount(owner.ID, "task backup")
	home, _ := agent.ProviderAccountHome(owner.ID, account.ID)
	os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
	os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600)
	if w := call("POST", "/api/v1/agents/connect-cli", `{"name":"@reserve","handle":"@reserve","provider":"agy","account_id":"`+account.ID+`"}`); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	file := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", account.ID, "account.json")
	raw, _ := os.ReadFile(file)
	var meta map[string]any
	json.Unmarshal(raw, &meta)
	meta["state"] = "unknown"
	raw, _ = json.Marshal(meta)
	os.WriteFile(file, raw, 0600)
	agent.SetProviderAccountFailover(owner.ID, account.ID, true)
	agent.SetProviderAccountModelFallback(owner.ID, account.ID, true)
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
		return agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "claude-sonnet-test", Remaining: 80}}}
	}
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/task-argv\"\nsleep 0.1\n"), 0700)
	created := workspaceTask(t, call)
	body := `{"request_id":"` + db.GenerateUUID() + `","agent_id":"agent_lifecycle"}`
	path := "/api/v1/tasks/" + created.ID + "/run-agent"
	w := call("POST", path, body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for end := time.Now().Add(2 * time.Second); ; {
		raw, err := os.ReadFile(filepath.Join(home, "task-argv"))
		if err == nil && strings.Contains(string(raw), "--model\nclaude-sonnet-test") {
			break
		}
		if time.Now().After(end) {
			t.Fatal("task not started in selected backup profile")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w := call("POST", path, body); w.Code != 200 || !strings.Contains(w.Body.String(), `"replayed":true`) {
		t.Fatal("duplicate launch", w.Code, w.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(context.Background(), created.ID)
	if detail.LatestAttempt.AgentName != "agent_lifecycle" {
		t.Fatal("agent identity changed")
	}
	if !strings.Contains(mustJSON(detail), "Provider preflight selected allowed backup account") {
		t.Fatal("route not recorded")
	}
}
func mustJSON(value any) string { raw, _ := json.Marshal(value); return string(raw) }

func TestInboxUsesSameAccountWhileAnotherSessionRuns(t *testing.T) {
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "2")
	s, _, registered, p, _, home, backupHome := setupFailoverFixture(t)
	runningTask, e := agent.TryLeaseProviderAccount(p.OwnerID, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer runningTask()
	output, failure := s.executeInboxWithFailover(context.Background(), p.OwnerID, registered, "Read this project", home, t.TempDir(), false, nil)
	if output != "Same agent backup answer" || failure != "" {
		t.Fatal("chat blocked by existing account session", output, failure)
	}
	if _, e := os.Stat(filepath.Join(backupHome, "calls")); !os.IsNotExist(e) {
		t.Fatal("unnecessary account switch")
	}
	second, e := agent.TryLeaseProviderAccount(p.OwnerID, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	proof := false
	output, failure = s.executeInboxWithFailover(context.Background(), p.OwnerID, registered, "Read this project", home, t.TempDir(), false, func(k, v string) {
		if k == "recovery_safe" {
			proof = true
		}
	})
	if output != "" || !strings.Contains(failure, "session slots") || strings.Contains(failure, "quota") || !proof {
		t.Fatal("capacity misreported as quota", output, failure, proof)
	}
	second()
	output, failure = s.executeInboxWithFailover(context.Background(), p.OwnerID, registered, "Read this project", home, t.TempDir(), false, nil)
	if output == "" || failure != "" {
		t.Fatal("released slot not reusable", failure)
	}
	calls, _ := os.ReadFile(filepath.Join(home, "calls"))
	if string(calls) != "xx" {
		t.Fatal("provider invoked while full", string(calls))
	}
}
