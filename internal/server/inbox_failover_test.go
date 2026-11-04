package server

import (
	"context"
	"encoding/json"
	"fmt"
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
		file := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", a.ID, "account.json")
		data, _ := os.ReadFile(file)
		var meta map[string]any
		_ = json.Unmarshal(data, &meta)
		meta["state"] = "unknown"
		data, _ = json.Marshal(meta)
		_ = os.WriteFile(file, data, 0600)
		a, _ = agent.GetProviderAccount(owner.ID, a.ID)
		return a, all, home
	}
	primary, registered, home := create("@main")
	backup, _, backupHome := create("@backup")
	bin := t.TempDir()
	script := `#!/bin/sh
printf x >> "$HOME/calls"
case "$2" in *"Your display name: @main"*|*"Your display name: @cuan"*) ;; *) exit 9;; esac
if [ -f "$HOME/timeout" ]; then
 printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"provider timeout: connection timed out"}}'
 exit 3
fi
if [ -f "$HOME/generic-error" ]; then
 printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"unexpected internal provider crash"}}'
 exit 3
fi
if [ -f "$HOME/limited" ]; then
 if [ -f "$HOME/unsafe" ]; then printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command"}}'; fi
 if [ -f "$HOME/read" ]; then printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"view_file"}}'; fi
 if [ -f "$HOME/auth" ]; then printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"invalid_grant private-token"}}';else printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"RESOURCE_EXHAUSTED private-token"}}';fi
 exit 3
fi
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Same agent backup answer\",\"topic_title\":\"\",\"action\":null}"}}'
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
	_, _ = agent.SetProviderAccountFailover(owner.ID, primary.ID, false)
	_, _ = agent.SetProviderAccountFailover(owner.ID, backup.ID, false)
	_, _ = agent.SetProviderAccountModelFallback(owner.ID, primary.ID, false)
	_, _ = agent.SetProviderAccountModelFallback(owner.ID, backup.ID, false)
	primary, _ = agent.GetProviderAccount(owner.ID, primary.ID)
	backup, _ = agent.GetProviderAccount(owner.ID, backup.ID)

	return s, token, registered, primary, backup, home, backupHome
}

func TestInboxQuotaFailover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow failover test in short mode")
	}
	for _, scenario := range []string{"success", "read-only", "native-read-only", "unsafe", "unknown-tool", "not-opted-in", "backup-quota", "backup-timeout", "backup-auth", "backup-generic", "auth", "primary-known-quota", "disabled-backup", "cancelled"} {
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
			if scenario == "read-only" || scenario == "native-read-only" {
				os.WriteFile(filepath.Join(home, "read"), nil, 0600)
			}
			if scenario == "native-read-only" {
				f := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
				data, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f, []byte(strings.ReplaceAll(string(data), "view_file", "read_file")), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "auth" {
				os.WriteFile(filepath.Join(home, "auth"), nil, 0600)
			}
			if scenario == "backup-quota" {
				os.WriteFile(filepath.Join(backupHome, "limited"), nil, 0600)
			}
			if scenario == "backup-timeout" {
				os.WriteFile(filepath.Join(backupHome, "timeout"), nil, 0600)
			}
			if scenario == "backup-auth" {
				os.WriteFile(filepath.Join(backupHome, "limited"), nil, 0600)
				os.WriteFile(filepath.Join(backupHome, "auth"), nil, 0600)
			}
			if scenario == "backup-generic" {
				os.WriteFile(filepath.Join(backupHome, "generic-error"), nil, 0600)
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
			success := scenario == "success" || scenario == "read-only" || scenario == "native-read-only" || scenario == "primary-known-quota"
			if success && (out != "Same agent backup answer" || failure != "") {
				t.Fatal(out, failure)
			}
			if !success && (out != "" || failure == "") {
				t.Fatal(out, failure)
			}
			backupCalls, _ := os.ReadFile(filepath.Join(backupHome, "calls"))
			wantBackup := success || scenario == "backup-quota" || scenario == "backup-timeout" || scenario == "backup-auth" || scenario == "backup-generic"
			if wantBackup != (string(backupCalls) == "x") {
				t.Fatal("incorrect backup executions", string(backupCalls))
			}
			if scenario == "backup-timeout" {
				if !strings.Contains(failure, "timed out") {
					t.Fatalf("expected timeout error, got %q", failure)
				}
				if failure == inboxQuotaNoBackup || strings.Contains(failure, "Provider quota is limited") {
					t.Fatalf("error masked as quota: %q", failure)
				}
			}
			if scenario == "backup-auth" {
				if !strings.Contains(failure, "sign-in status") {
					t.Fatalf("expected auth status error, got %q", failure)
				}
				if failure == inboxQuotaNoBackup || strings.Contains(failure, "Provider quota is limited") {
					t.Fatalf("error masked as quota: %q", failure)
				}
			}
			if scenario == "backup-generic" {
				if !strings.Contains(failure, "The provider could not reply") {
					t.Fatalf("expected provider reply error, got %q", failure)
				}
				if failure == inboxQuotaNoBackup || strings.Contains(failure, "Provider quota is limited") {
					t.Fatalf("error masked as quota: %q", failure)
				}
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
			if len(data.Messages) != 2 || !strings.Contains(w.Body.String(), "Same agent backup answer") || data.Requests[0].AgentID != db.AgentIDCuan {
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
		if len(calls) > 1 || (dir == backupHome && string(calls) != "x") {
			t.Fatal("duplicate execution or no backup reply", string(calls))
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
	body := `{"request_id":"` + db.GenerateUUID() + `","agent_id":"agent_lifecycle","runner_id":"local_terminal"}`
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
	if output != "" || failure != inboxQuotaNoBackup || strings.Contains(failure, "quota") || !proof {
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

func TestAutoModelUsesProviderDefaultWithoutEnablingFallback(t *testing.T) {
	s, _, registered, p, _, home, _ := setupFailoverFixture(t)
	registered.ModelID = ""
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	usage := agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "claude-opus-test", Remaining: 32.6}, {Model: "gemini-test", Remaining: 0.15}, {Model: "claude-sonnet-test", Remaining: 32.6}}}
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage { return usage }
	routes := s.providerRoutes(context.Background(), p.OwnerID, registered, home, false)
	if len(routes) != 1 || routes[0].model != "" {
		t.Fatalf("auto did not preserve provider default: %+v", routes)
	}
	release, err := s.leaseProviderRoute(context.Background(), p.OwnerID, registered, routes[0])
	if err != nil {
		t.Fatal(err)
	}
	release()
	current, err := agent.GetProviderAccount(p.OwnerID, p.ID)
	if err != nil || current.AllowModelFallback {
		t.Fatal("auto selection changed fallback policy", err)
	}
	registered.ModelID = "gemini-test"
	routes = s.providerRoutes(context.Background(), p.OwnerID, registered, home, false)
	if len(routes) != 1 || routes[0].model != "gemini-test" {
		t.Fatal("explicit model ignored", routes)
	}
	registered.ModelID = ""
	usage = agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "gemini-test", Remaining: 0}, {Model: "gemini-image", Remaining: 100}}}
	if routes = s.providerRoutes(context.Background(), p.OwnerID, registered, home, false); len(routes) != 1 || routes[0].model != "" {
		t.Fatal("quota catalog overrode provider default", routes)
	}
}

func TestInboxRecoveryChecksOtherAccountBeforeMoreModels(t *testing.T) {
	for _, failureKind := range []string{"quota", "location", "unsafe-location"} {
		t.Run(failureKind, func(t *testing.T) {
			s, _, registered, p, b, home, backupHome := setupFailoverFixture(t)
			agent.SetProviderAccountFailover(p.OwnerID, b.ID, true)
			agent.SetProviderAccountModelFallback(p.OwnerID, p.ID, true)
			agent.SetProviderAccountModelFallback(p.OwnerID, b.ID, true)
			original := fetchInboxModelUsage
			defer func() { fetchInboxModelUsage = original }()
			fetchInboxModelUsage = func(_ context.Context, _ string, id string) agent.AccountModelUsage {
				models := []agent.ModelUsage{{Model: "claude-first", Remaining: 90}, {Model: "claude-second", Remaining: 80}, {Model: "claude-third", Remaining: 70}, {Model: "claude-fourth", Remaining: 60}}
				if id == b.ID {
					models = []agent.ModelUsage{{Model: "claude-backup", Remaining: 10}}
				}
				return agent.AccountModelUsage{Available: true, Models: models}
			}
			os.WriteFile(filepath.Join(home, "limited"), nil, 0600)
			if failureKind != "quota" {
				f := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
				raw, _ := os.ReadFile(f)
				raw = []byte(strings.ReplaceAll(string(raw), "RESOURCE_EXHAUSTED private-token", "User location is not supported for the API use"))
				if err := os.WriteFile(f, raw, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if failureKind == "unsafe-location" {
				os.WriteFile(filepath.Join(home, "unsafe"), nil, 0600)
			}
			out, failure := s.executeInboxWithFailover(context.Background(), p.OwnerID, registered, "Analyze", home, t.TempDir(), false, nil)
			calls, _ := os.ReadFile(filepath.Join(home, "calls"))
			backup, _ := os.ReadFile(filepath.Join(backupHome, "calls"))
			if string(calls) != "x" {
				t.Fatal("repeated models on failing primary", string(calls))
			}
			if failureKind == "unsafe-location" {
				if out != "" || failure == "" || len(backup) != 0 {
					t.Fatal("unsafe activity replayed", out, failure, string(backup))
				}
			} else if out != "Same agent backup answer" || failure != "" || string(backup) != "x" {
				t.Fatal("permitted backup not used exactly once", out, failure, string(backup))
			}
		})
	}
}

func TestInboxRegionRestrictionCanUseAllowedModelOnSameAccount(t *testing.T) {
	s, _, registered, p, _, home, _ := setupFailoverFixture(t)
	if _, err := agent.SetProviderAccountModelFallback(p.OwnerID, p.ID, true); err != nil {
		t.Fatal(err)
	}
	old := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = old }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
		return agent.AccountModelUsage{Available: true, Models: []agent.ModelUsage{{Model: "gemini-test", Remaining: 90}, {Model: "claude-sonnet-test", Remaining: 80}}}
	}
	script := `#!/bin/sh
printf x >> "$HOME/calls"
last=''
for arg in "$@"; do last="$arg"; done
if [ "$last" = claude-sonnet-test ]; then
 printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Recovered on allowed Sonnet\",\"topic_title\":\"\",\"action\":null}"}}'
else
 printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"User location is not supported for the API use"}}'
 exit 3
fi
`
	f := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	if err := os.WriteFile(f, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	out, failure := s.executeInboxWithFailover(context.Background(), p.OwnerID, registered, "Analyze", home, t.TempDir(), false, nil)
	if out != "Recovered on allowed Sonnet" || failure != "" {
		t.Fatal(out, failure)
	}
	profile, err := agent.GetProviderAccount(p.OwnerID, p.ID)
	if err != nil || !time.Now().Before(profile.ModelCooldowns[""]) || !time.Now().Before(profile.ModelCooldowns["gemini-test"]) {
		t.Fatal("failed route not remembered", err)
	}
	if profile.State == "quota_limited" {
		t.Fatal("regional restriction marked as account quota")
	}
	routes := s.providerRoutes(context.Background(), p.OwnerID, registered, home, false)
	for _, route := range routes {
		if route.model == "" || route.model == "gemini-test" {
			t.Fatal("task preflight reused known restricted route", routes)
		}
	}

	calls, _ := os.ReadFile(filepath.Join(home, "calls"))
	if string(calls) != "xxx" {
		t.Fatal("wrong route attempts", string(calls))
	}
}

func TestResilientMultiAccountFailoverAndTrueFailureReporting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)

	create := func(handle string) (*agent.ProviderAccount, *db.Agent, string) {
		a, err := agent.CreateProviderAccount(owner.ID, handle)
		if err != nil {
			t.Fatal(err)
		}
		home, _ := agent.ProviderAccountHome(owner.ID, a.ID)
		_ = os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
		_ = os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600)
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
		file := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", a.ID, "account.json")
		data, _ := os.ReadFile(file)
		var meta map[string]any
		_ = json.Unmarshal(data, &meta)
		meta["state"] = "unknown"
		data, _ = json.Marshal(meta)
		_ = os.WriteFile(file, data, 0600)
		a, _ = agent.GetProviderAccount(owner.ID, a.ID)
		return a, all, home
	}

	_, registered, homePrimary := create("@main")
	b1, _, homeB1 := create("@backup1")
	b2, _, homeB2 := create("@backup2")

	_, _ = agent.SetProviderAccountFailover(owner.ID, b1.ID, true)
	_, _ = agent.SetProviderAccountFailover(owner.ID, b2.ID, true)

	bin := t.TempDir()
	script := `#!/bin/sh
printf x >> "$HOME/calls"
if [ -f "$HOME/timeout" ]; then
  printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"provider timeout: connection timed out"}}'
  exit 3
fi
if [ -f "$HOME/limited" ]; then
  printf '%s\n' '{"event":"result","result":{"status":"ERROR","message":"RESOURCE_EXHAUSTED private-token"}}'
  exit 3
fi
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Reply from backup 2\",\"topic_title\":\"\",\"action\":null}"}}'
`
	_ = os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s.cfg.Environment = "staging"

	t.Run("primary-quota-backup1-timeout-backup2-success", func(t *testing.T) {
		_ = os.Remove(filepath.Join(homePrimary, "calls"))
		_ = os.Remove(filepath.Join(homeB1, "calls"))
		_ = os.Remove(filepath.Join(homeB2, "calls"))

		routes := s.providerRoutes(context.Background(), owner.ID, registered, homePrimary, false)
		if len(routes) < 3 {
			t.Fatalf("expected at least 3 routes, got %d", len(routes))
		}
		firstBackupAccountID := routes[1].accountID
		firstBackupHome := homeB1
		secondBackupHome := homeB2
		if firstBackupAccountID == b2.ID {
			firstBackupHome = homeB2
			secondBackupHome = homeB1
		}

		_ = os.WriteFile(filepath.Join(homePrimary, "limited"), nil, 0600)
		_ = os.WriteFile(filepath.Join(firstBackupHome, "timeout"), nil, 0600)
		_ = os.Remove(filepath.Join(secondBackupHome, "limited"))
		_ = os.Remove(filepath.Join(secondBackupHome, "timeout"))

		var trace []string
		out, failure := s.executeInboxWithFailover(context.Background(), owner.ID, registered, "Hello", homePrimary, t.TempDir(), false, func(k, v string) {
			trace = append(trace, k)
		})
		if out != "Reply from backup 2" || failure != "" {
			t.Fatalf("expected failover to second backup success, got out=%q, failure=%q", out, failure)
		}
		callsP, _ := os.ReadFile(filepath.Join(homePrimary, "calls"))
		callsFirst, _ := os.ReadFile(filepath.Join(firstBackupHome, "calls"))
		callsSecond, _ := os.ReadFile(filepath.Join(secondBackupHome, "calls"))
		if string(callsP) != "x" || string(callsFirst) != "x" || string(callsSecond) != "x" {
			t.Fatalf("unexpected call counts: P=%s, First=%s, Second=%s", callsP, callsFirst, callsSecond)
		}
	})

	t.Run("primary-quota-backup1-timeout-backup2-quota-true-failure-reported", func(t *testing.T) {
		_ = os.Remove(filepath.Join(homePrimary, "calls"))
		_ = os.Remove(filepath.Join(homeB1, "calls"))
		_ = os.Remove(filepath.Join(homeB2, "calls"))

		_ = os.WriteFile(filepath.Join(homePrimary, "limited"), nil, 0600)
		_ = os.WriteFile(filepath.Join(homeB1, "timeout"), nil, 0600)
		_ = os.WriteFile(filepath.Join(homeB2, "limited"), nil, 0600)

		out, failure := s.executeInboxWithFailover(context.Background(), owner.ID, registered, "Hello", homePrimary, t.TempDir(), false, nil)
		if out != "" {
			t.Fatalf("expected no output, got %q", out)
		}
		if !strings.Contains(failure, "timed out") {
			t.Fatalf("expected timeout failure reported, got %q", failure)
		}
		if failure == inboxQuotaNoBackup || strings.Contains(failure, "Provider quota is limited") {
			t.Fatalf("error was improperly masked into quota exhaustion: %q", failure)
		}
	})

	t.Run("pool-alignment-active-agent-runnable", func(t *testing.T) {
		b3, boundAgent, _ := create("@backup3")
		boundAgent.Status = db.AgentStatusActive
		boundAgent.Harness = "antigravity"
		_ = s.agentRepo.UpdateAgent(context.Background(), boundAgent)
		_, _ = agent.SetProviderAccountFailover(owner.ID, b3.ID, true)

		routes := s.providerRoutes(context.Background(), owner.ID, registered, homePrimary, false)
		found := false
		for _, r := range routes {
			if r.accountID == b3.ID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected aligned runnable backup account %s in routes, got %+v", b3.ID, routes)
		}
	})
}

func TestFacadeRunsNewBrainWithoutCreatingPersona(t *testing.T) {
	s, _, _, p, b, _, _ := setupFailoverFixture(t)
	owner := p.OwnerID
	if err := agent.DeleteProviderAccount(owner, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := agent.DeleteProviderAccount(owner, b.ID); err != nil {
		t.Fatal(err)
	}
	create := func(label string) (*agent.ProviderAccount, string) {
		a, err := agent.CreateProviderAccount(owner, label)
		if err != nil {
			t.Fatal(err)
		}
		home, _ := agent.ProviderAccountHome(owner, a.ID)
		if err := os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600); err != nil {
			t.Fatal(err)
		}
		// Persist native verification evidence without registering another persona.
		file := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", a.ID, "account.json")
		data, _ := os.ReadFile(file)
		var meta map[string]any
		json.Unmarshal(data, &meta)
		meta["state"] = "unknown"
		data, _ = json.Marshal(meta)
		os.WriteFile(file, data, 0600)
		a, _ = agent.GetProviderAccount(owner, a.ID)
		return a, home
	}
	first, home := create("Fresh sign-in")
	second, secondHome := create("Second sign-in")
	facade := &db.Agent{ID: db.AgentIDCuan, Name: "@cuan", Provider: "agy", Harness: "agy", Enabled: true}
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage { return agent.AccountModelUsage{} }
	routes := s.providerRoutes(context.Background(), owner, facade, "", false)
	if len(routes) != 2 {
		t.Fatalf("unbound brains excluded: %+v", routes)
	}
	for _, route := range routes {
		release, err := s.leaseProviderRoute(context.Background(), owner, facade, route)
		if err != nil {
			t.Fatal(err)
		}
		release()
		wrong := route
		wrong.home = t.TempDir()
		if release, err := s.leaseProviderRoute(context.Background(), owner, facade, wrong); err == nil {
			release()
			t.Fatal("profile path substitution accepted")
		}
		if release, err := s.leaseProviderRoute(context.Background(), "another-owner", facade, route); err == nil {
			release()
			t.Fatal("other owner's brain accepted")
		}
	}
	// The same direct profile is forbidden to historical personas and impostor names.
	ordinary := &db.Agent{ID: "agent_other", Name: "@cuan", Provider: "agy", Harness: "agy"}
	if _, err := s.inboxAccountProfileHome(context.Background(), owner, ordinary, first); err == nil {
		t.Fatal("display name granted facade privileges")
	}
	partial := *first
	partial.AgentID = "missing_agent"
	if _, err := s.inboxAccountProfileHome(context.Background(), owner, facade, &partial); err == nil {
		t.Fatal("partial binding bypassed")
	}
	bad := *first
	bad.Provider = "commandcode"
	if _, err := s.inboxAccountProfileHome(context.Background(), owner, facade, &bad); err == nil {
		t.Fatal("provider mismatch bypassed")
	}
	bad = *first
	bad.State = "unavailable"
	if _, err := s.inboxAccountProfileHome(context.Background(), owner, facade, &bad); err == nil {
		t.Fatal("unavailable brain selected")
	}
	out, failure := s.executeInboxWithFailover(context.Background(), owner, facade, "Analyze", "", t.TempDir(), false, nil)
	if out == "" || failure != "" {
		t.Fatal(out, failure)
	}
	firstCalls, _ := os.ReadFile(filepath.Join(home, "calls"))
	secondCalls, _ := os.ReadFile(filepath.Join(secondHome, "calls"))
	if len(firstCalls)+len(secondCalls) != 1 {
		t.Fatal("expected exactly one provider execution")
	}
	for _, id := range []string{first.ID, second.ID} {
		current, _ := agent.GetProviderAccount(owner, id)
		if current.AgentID != "" || current.ConnectionID != "" || current.AllowModelFallback || current.AllowFailover {
			t.Fatal("routing altered user settings", current)
		}
	}
}

func TestInboxBoundedQuotaRouteCheckKeepsWorkPending(t *testing.T) {
	s, _, registered, primary, _, home, _ := setupFailoverFixture(t)
	owner := primary.OwnerID
	if _, err := agent.SetProviderAccountModelFallback(owner, primary.ID, true); err != nil {
		t.Fatal(err)
	}
	models := []agent.ModelUsage{}
	for i := 0; i < 10; i++ {
		models = append(models, agent.ModelUsage{Model: fmt.Sprintf("gemini-test-%02d", i), Remaining: 90 - float64(i)})
	}
	registered.ModelID = models[0].Model
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
		return agent.AccountModelUsage{Available: true, Models: models}
	}
	if err := os.WriteFile(filepath.Join(home, "limited"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	raw, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	recording := "#!/bin/sh\nlast=''\nfor arg in \"$@\"; do last=\"$arg\"; done\nprintf '%s\\n' \"$last\" >> \"$HOME/models\"\n"
	if err := os.WriteFile(binary, []byte(strings.Replace(string(raw), "#!/bin/sh\n", recording, 1)), 0700); err != nil {
		t.Fatal(err)
	}
	routes := s.providerRoutes(context.Background(), owner, registered, home, false)
	if len(routes) != 10 {
		t.Fatalf("expected10 model routes, got%d", len(routes))
	}
	var steps []string
	out, failure := s.executeInboxWithFailover(context.Background(), owner, registered, "Evaluate the saved task report", home, t.TempDir(), false, func(kind, _ string) { steps = append(steps, kind) })
	if out != "" || failure != inboxQuotaNoBackup {
		t.Fatalf("bounded quota check became permanent failure: output=%q failure=%q", out, failure)
	}
	calls, _ := os.ReadFile(filepath.Join(home, "calls"))
	if len(calls) != 8 {
		t.Fatalf("wrong bounded route budget: attempts%d", len(calls))
	}
	if !strings.Contains(strings.Join(steps, ","), "capacity_wait") {
		t.Fatal("no capacity backoff evidence")
	}
	if err := os.Remove(filepath.Join(home, "limited")); err != nil {
		t.Fatal(err)
	}
	out, failure = s.executeInboxWithFailover(context.Background(), owner, registered, "Evaluate the saved task report", home, t.TempDir(), false, nil)
	if out != "Same agent backup answer" || failure != "" {
		t.Fatalf("remaining route never reached: %q %q", out, failure)
	}
	triedRaw, _ := os.ReadFile(filepath.Join(home, "models"))
	tried := strings.Fields(string(triedRaw))
	seen := map[string]bool{}
	if len(tried) != 9 {
		t.Fatalf("expected8 failed models +1 remaining success, got%d", len(tried))
	}
	for _, model := range tried {
		if seen[model] {
			t.Fatal("cooled-down model was replayed", model)
		}
		seen[model] = true
	}
}
