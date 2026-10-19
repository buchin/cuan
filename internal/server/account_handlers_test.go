package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/terminal"
)

func TestS26AccountAPIAuthOwnerAndQuotaIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	for _, route := range []string{"/api/v1/provider-accounts", "/api/v1/provider-accounts/acct_00000000000000000000000000000000/auth-init"} {
		method := "POST"
		if route == "/api/v1/provider-accounts" {
			method = "GET"
		}
		w := request(method, route, `{}`, "")
		if w.Code != 401 {
			t.Fatalf("owner gate %d", w.Code)
		}
	}
	w := request("POST", "/api/v1/provider-accounts", `{"label":"Google kerja"}`, token)
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Account agent.ProviderAccount `json:"account"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal("bad create response")
	}
	id := response.Account.ID
	// A different authenticated owner sees no accounts and cannot operate this one.
	store := auth.NewMemoryStore()
	service := auth.NewService(store)
	other, err := service.Setup(context.Background(), "other@example.test", "other-password-fixture")
	if err != nil {
		t.Fatal(err)
	}
	original := s.authSvc
	s.authSvc = service
	w = request("POST", "/api/v1/provider-accounts/"+id+"/auth-init", `{}`, other.Token)
	if w.Code != 404 {
		t.Fatalf("owner isolation %d", w.Code)
	}
	s.authSvc = original
	// Existing service credential and quota must not prevent a fresh isolated login.
	credential := filepath.Join(t.TempDir(), "legacy-credential")
	os.WriteFile(credential, []byte("legacy-fixture-credential-value"), 0600)
	t.Setenv("ANTIGRAVITY_TOKEN_PATH", credential)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	cli := `#!/bin/sh
if [ "$1" = "--print" ]; then echo '429 quota private-marker' >&2; exit 1; fi
printf 'https://accounts.google.com/o/oauth2/v2/auth?client_id=fixture&redirect_uri=https%%3A%%2F%%2Fantigravity.google%%2Foauth-callback&response_type=code\n'
while read value; do :; done
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(cli), 0700); err != nil {
		t.Fatal(err)
	}
	w = request("POST", "/api/v1/provider-accounts/"+id+"/auth-init", `{}`, token)
	if w.Code != 200 {
		t.Fatalf("new account blocked by legacy quota: %d %s", w.Code, w.Body.String())
	}
	var session struct {
		ID string `json:"session_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &session)
	if session.ID == "" {
		t.Fatal("no OAuth session")
	}
	defer agent.CancelAntigravityAuthSession(session.ID)
	w = request("POST", "/api/v1/agents/auth-code?cancel=true", `{"session_id":"`+session.ID+`"}`, token)
	if w.Code != 404 {
		t.Fatal("legacy endpoint cancelled scoped session")
	}
	w = request("POST", "/api/v1/provider-accounts/"+id+"/auth-code?cancel=true", `{"session_id":"`+session.ID+`"}`, token)
	if w.Code != 200 {
		t.Fatalf("cancel %d", w.Code)
	}
	if data, _ := os.ReadFile(credential); string(data) != "legacy-fixture-credential-value" {
		t.Fatal("legacy credential changed")
	}
	w = request("GET", "/api/v1/provider-accounts", "", token)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-marker") || strings.Contains(w.Body.String(), "legacy-fixture") {
		t.Fatal("account DTO unsafe")
	}
}

func TestS26RecreateAgentDoesNotCollideWithRetainedConnection(t *testing.T) {
	s, token := setupTestChatServer(t)
	repo := s.agentRepo
	old := &db.AgentConnection{ID: "conn_agent_dev", UserID: "default", Provider: "mock", Status: db.ConnectionStatusActive}
	if err := repo.CreateConnection(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	create := func() string {
		r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(`{"name":"@dev","handle":"@dev","provider":"mock"}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		if w.Code != 201 {
			t.Fatalf("recreation failed: %d %s", w.Code, w.Body.String())
		}
		var response struct {
			Connection struct {
				ID string `json:"id"`
			} `json:"connection"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Connection.ID
	}
	first := create()
	if first == old.ID || !strings.HasPrefix(first, "conn_") {
		t.Fatal("retained connection reused")
	}
	if err := repo.DeleteAgent(context.Background(), "agent_dev"); err != nil {
		t.Fatal(err)
	}
	second := create()
	if second == old.ID || second == first {
		t.Fatal("recreated agent collided with retained connection")
	}
	for _, id := range []string{old.ID, first, second} {
		if _, err := repo.GetConnectionByID(context.Background(), id); err != nil {
			t.Fatal("prior connection unexpectedly removed")
		}
	}
}

type s26ConnectionOverride struct {
	db.AgentRepository
	connection *db.AgentConnection
}

func (r *s26ConnectionOverride) GetConnectionByID(context.Context, string) (*db.AgentConnection, error) {
	return r.connection.Clone(), nil
}

func TestS26DedicatedAgentRegistrationAndRuntimeIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	owner, err := s.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	profiles := []*agent.ProviderAccount{}
	for _, label := range []string{"profile-first", "profile-second"} {
		a, err := agent.CreateProviderAccount(owner.ID, label)
		if err != nil {
			t.Fatal(err)
		}
		home, _ := agent.ProviderAccountHome(owner.ID, a.ID)
		os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
		os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-not-real-provider-credential"), 0600)
		os.WriteFile(filepath.Join(home, "fixture-marker"), []byte(label), 0600)
		profiles = append(profiles, a)
	}
	request := func(handle, profile string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"name": handle, "handle": handle, "provider": "agy", "account_id": profile})
		r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	for i, handle := range []string{"@first", "@second"} {
		w := request(handle, profiles[i].ID)
		if w.Code != 201 {
			t.Fatalf("register %d %s", w.Code, w.Body.String())
		}
	}
	if w := request("@borrowed", profiles[0].ID); w.Code != 409 {
		t.Fatalf("shared connection accepted %d", w.Code)
	}
	if w := request("@first", profiles[0].ID); w.Code != 200 {
		t.Fatalf("same successful registration retry failed %d %s", w.Code, w.Body.String())
	}
	if w := request("@first", profiles[1].ID); w.Code != 409 {
		t.Fatal("retry crossed profile")
	}
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\n/bin/cat \"$HOME/fixture-marker\"\nprintf '%s' \"$HOME\" > \"$HOME/task-used-profile\"\n"), 0700)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s.cfg.Environment = "staging"
	for i, id := range []string{"agent_first", "agent_second"} {
		a, err := s.agentRepo.GetAgentByID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		home, err := s.agentProfileHome(context.Background(), a)
		if err != nil {
			t.Fatal(err)
		}
		expected, _ := agent.ProviderAccountHome(owner.ID, profiles[i].ID)
		if home != expected {
			t.Fatal("runtime uses wrong profile")
		}
		out := s.executeAgentContextualResponse(context.Background(), a.Name, "agy", "fixture prompt", RoleFriendly, home)
		if !strings.Contains(out, []string{"profile-first", "profile-second"}[i]) {
			t.Fatal("actual child did not use dedicated HOME")
		}

		// Exercise the actual Inbox HTTP route as well as the child helper.
		r := httptest.NewRequest("POST", "/api/v1/chat/conversations/"+id+"/messages", strings.NewReader(`{"message":"implement fixture routing check"}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), []string{"profile-first", "profile-second"}[i]) {
			t.Fatalf("Inbox profile routing failed: %d %s", w.Code, w.Body.String())
		}
	}
	s.terminalManager = terminal.NewManager(terminal.ManagerConfig{WorkspacesDir: t.TempDir()})
	defer s.terminalManager.CloseAll()
	for i, id := range []string{"agent_first", "agent_second"} {
		home, _ := agent.ProviderAccountHome(owner.ID, profiles[i].ID)
		proof := filepath.Join(home, "task-used-profile")
		os.Remove(proof)
		r := httptest.NewRequest("POST", "/api/v1/tasks/profile-fixture-"+id+"/run-agent", strings.NewReader(`{"agent_id":"`+id+`","prompt":"fixture"}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("task profile launch %d %s", w.Code, w.Body.String())
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			if raw, err := os.ReadFile(proof); err == nil {
				if string(raw) != home {
					t.Fatal("task used another HOME")
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("task did not execute in its dedicated profile")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	a, _ := s.agentRepo.GetAgentByID(context.Background(), "agent_first")

	original := a.Clone()
	conn, _ := s.agentRepo.GetConnectionByID(context.Background(), a.ConnectionID)
	originalRepo := s.agentRepo
	for _, mutate := range []func(*db.AgentConnection){
		func(c *db.AgentConnection) { c.UserID = "wrong-owner" },
		func(c *db.AgentConnection) { c.Provider = "claude" },
	} {
		bad := conn.Clone()
		mutate(bad)
		s.agentRepo = &s26ConnectionOverride{AgentRepository: originalRepo, connection: bad}
		if _, err := s.agentProfileHome(context.Background(), a); err == nil {
			t.Fatal("wrong owner/type connection accepted")
		}
	}
	s.agentRepo = originalRepo
	a.Harness = "claude"
	if _, err := s.agentProfileHome(context.Background(), a); err == nil {
		t.Fatal("wrong harness accepted")
	}
	a = original.Clone()

	profileHome, _ := agent.ProviderAccountHome(owner.ID, profiles[0].ID)
	metadataPath := filepath.Join(filepath.Dir(profileHome), "account.json")
	metadata, _ := os.ReadFile(metadataPath)
	var stored map[string]any
	json.Unmarshal(metadata, &stored)
	stored["provider"] = "claude"
	invalidMetadata, _ := json.Marshal(stored)
	os.WriteFile(metadataPath, invalidMetadata, 0600)
	if _, err := s.agentProfileHome(context.Background(), a); err == nil {
		t.Fatal("wrong profile provider accepted")
	}
	os.WriteFile(metadataPath, metadata, 0600)
	// Even with valid global credentials, missing dedicated credentials must fail.
	t.Setenv("ANTIGRAVITY_TOKEN_PATH", agent.AntigravityCredentialsFilePath(t.TempDir()))
	os.MkdirAll(filepath.Dir(os.Getenv("ANTIGRAVITY_TOKEN_PATH")), 0700)
	os.WriteFile(os.Getenv("ANTIGRAVITY_TOKEN_PATH"), []byte("global-fixture-must-not-be-used"), 0600)
	home, _ := agent.ProviderAccountHome(owner.ID, profiles[0].ID)
	credential := agent.AntigravityCredentialsFilePath(home)
	raw, _ := os.ReadFile(credential)
	os.Remove(credential)
	if _, err := s.agentProfileHome(context.Background(), a); err == nil {
		t.Fatal("missing dedicated credential fell back to global")
	}
	os.WriteFile(credential, raw, 0600)
	a.ConnectionID = "conn_profile_missing"
	if _, err := s.agentProfileHome(context.Background(), a); err == nil {
		t.Fatal("missing new connection fell back to legacy")
	}
	a2, _ := s.agentRepo.GetAgentByID(context.Background(), "agent_second")
	a.ConnectionID = a2.ConnectionID
	if _, err := s.agentProfileHome(context.Background(), a); err == nil {
		t.Fatal("agent borrowed another profile")
	}
}

type s26FailConnectionRepository struct{ db.AgentRepository }

func (r *s26FailConnectionRepository) CreateConnection(context.Context, *db.AgentConnection) error {
	return errors.New("fixture storage failure")
}
func TestS26FailedRegistrationPreservesLoginAndUnbinds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	original := s.agentRepo
	owner, err := s.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := agent.CreateProviderAccount(owner.ID, "failed-registration-fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := agent.ProviderAccountHome(owner.ID, profile.ID)
	os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
	os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-not-real-provider-credential"), 0600)
	s.SetAgentRepository(&s26FailConnectionRepository{AgentRepository: original})
	body, _ := json.Marshal(map[string]string{"name": "@save_retry", "handle": "@save_retry", "provider": "agy", "account_id": profile.ID})
	r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(string(body)))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 500 {
		t.Fatalf("expected storage failure %d", w.Code)
	}
	if _, err := original.GetAgentByID(context.Background(), "agent_save_retry"); err == nil {
		t.Fatal("failed registration left roster row")
	}
	saved, err := agent.GetProviderAccount(owner.ID, profile.ID)
	if err != nil || saved.ConnectionID != "" || !saved.CredentialsPresent {
		t.Fatal("failed registration removed login or retained binding")
	}
	s.SetAgentRepository(original)
	r = httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(string(body)))
	r.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("retry failed %d %s", w.Code, w.Body.String())
	}
}

func TestS26LegacyPinCheckSurvivesRosterReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	s.agentRepo.CreateAgent(context.Background(), &db.Agent{ID: "agent_pin1", Name: "@pin1", Harness: "agy", Provider: "agy", ConnectionID: "conn_agent_pin1", Enabled: true, Status: db.AgentStatusActive})
	dir := t.TempDir()
	credential := filepath.Join(dir, "legacy-fixture")
	os.WriteFile(credential, []byte("fixture-legacy-credential-value"), 0600)
	t.Setenv("ANTIGRAVITY_TOKEN_PATH", credential)
	t.Setenv("PATH", dir)
	os.WriteFile(filepath.Join(dir, "agy"), []byte("#!/bin/sh\nprintf 'CUAN_AUTH_OK\\n'\n"), 0700)
	call := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	w := call("POST", "/api/v1/agents/agent_pin1/connection/verify")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"connection_state":"verified"`) {
		t.Fatalf("probe %d %s", w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		w = call("GET", "/api/v1/agents")
		var data struct {
			Agents []struct {
				ID      string     `json:"id"`
				State   string     `json:"connection_state"`
				Checked *time.Time `json:"last_checked_at"`
			} `json:"agents"`
		}
		json.Unmarshal(w.Body.Bytes(), &data)
		found := false
		for _, a := range data.Agents {
			if a.ID == "agent_pin1" {
				found = true
				if a.State != "verified" || a.Checked == nil {
					t.Fatal("roster reload forgot successful check")
				}
			}
		}
		if !found {
			t.Fatal("agent disappeared")
		}
	}
	// A later quota failure must replace the old success, retaining credentials.
	os.WriteFile(filepath.Join(dir, "agy"), []byte("#!/bin/sh\nprintf '429 quota' >&2\nexit 1\n"), 0700)
	w = call("POST", "/api/v1/agents/agent_pin1/connection/verify")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"connection_state":"quota_limited"`) {
		t.Fatalf("quota probe %d %s", w.Code, w.Body.String())
	}
	w = call("GET", "/api/v1/agents")
	if !strings.Contains(w.Body.String(), `"connection_state":"quota_limited"`) {
		t.Fatal("roster kept stale success")
	}
	if raw, _ := os.ReadFile(credential); string(raw) != "fixture-legacy-credential-value" {
		t.Fatal("status check changed credential")
	}
}
