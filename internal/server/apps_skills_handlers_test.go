package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
)

func setupTestServerForAppsSkills(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		AuthSecret: "test-auth-secret-key-cuan-32b-length",
	}
	s := NewServer(cfg, nil)
	return s
}

func TestAppsAndSkills_EndToEndEndpoints(t *testing.T) {
	s := setupTestServerForAppsSkills(t)

	// 1. GET /api/v1/apps
	req := httptest.NewRequest(http.MethodGet, "/api/v1/apps", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/apps, got %d: %s", w.Code, w.Body.String())
	}

	var appsResp struct {
		Status string     `json:"status"`
		Apps   []*mcp.App `json:"apps"`
		Total  int        `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&appsResp); err != nil {
		t.Fatalf("failed to decode GET /api/v1/apps response: %v", err)
	}
	if appsResp.Status != "success" || appsResp.Total == 0 {
		t.Fatalf("expected success and non-empty apps list: %+v", appsResp)
	}

	// Verify Google Sheets is listed
	var foundSheets bool
	for _, app := range appsResp.Apps {
		if app.ID == "google_sheets" {
			foundSheets = true
			break
		}
	}
	if !foundSheets {
		t.Errorf("expected google_sheets in apps catalog")
	}

	// 2. POST /api/v1/apps/google_sheets/connect
	connectPayload := []byte(`{"account": "pemilik.warung@gmail.com"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps/google_sheets/connect", bytes.NewReader(connectPayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from connect, got %d: %s", w.Code, w.Body.String())
	}

	var connectResp struct {
		Status string   `json:"status"`
		App    *mcp.App `json:"app"`
	}
	if err := json.NewDecoder(w.Body).Decode(&connectResp); err != nil {
		t.Fatalf("failed to decode connect response: %v", err)
	}
	if connectResp.App.Status != "connected" || connectResp.App.Account != "pemilik.warung@gmail.com" {
		t.Errorf("expected connected status and account, got %+v", connectResp.App)
	}

	// 3. POST /api/v1/apps/google_sheets/disconnect
	req = httptest.NewRequest(http.MethodPost, "/api/v1/apps/google_sheets/disconnect", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from disconnect, got %d: %s", w.Code, w.Body.String())
	}

	var disconnectResp struct {
		Status string   `json:"status"`
		App    *mcp.App `json:"app"`
	}
	if err := json.NewDecoder(w.Body).Decode(&disconnectResp); err != nil {
		t.Fatalf("failed to decode disconnect response: %v", err)
	}
	if disconnectResp.App.Status != "disconnected" || disconnectResp.App.Account != "" {
		t.Errorf("expected disconnected status, got %+v", disconnectResp.App)
	}

	// 4. GET /api/v1/skills
	req = httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/skills, got %d: %s", w.Code, w.Body.String())
	}

	var skillsResp struct {
		Status string       `json:"status"`
		Skills []*mcp.Skill `json:"skills"`
		Total  int          `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&skillsResp); err != nil {
		t.Fatalf("failed to decode skills response: %v", err)
	}
	if skillsResp.Total < 2 {
		t.Errorf("expected at least 2 skills in catalog, got %d", skillsResp.Total)
	}

	// 5. GET /api/v1/agents/agent_sm/skills
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent_sm/skills", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET agent skills, got %d: %s", w.Code, w.Body.String())
	}

	var agentSkillsResp struct {
		Status  string               `json:"status"`
		AgentID string               `json:"agent_id"`
		Skills  []mcp.AgentSkillView `json:"skills"`
		Total   int                  `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&agentSkillsResp); err != nil {
		t.Fatalf("failed to decode agent skills response: %v", err)
	}
	if agentSkillsResp.AgentID != "agent_sm" || len(agentSkillsResp.Skills) < 2 {
		t.Errorf("unexpected agent skills response: %+v", agentSkillsResp)
	}

	// 6. PUT /api/v1/agents/agent_sm/skills (toggle skill)
	updatePayload := []byte(`{
		"skills": [
			{"skill_id": "vision_receipt_extractor", "enabled": false},
			{"skill_id": "google_sheets_sync", "enabled": true}
		]
	}`)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/agents/agent_sm/skills", bytes.NewReader(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from PUT agent skills, got %d: %s", w.Code, w.Body.String())
	}

	var putResp struct {
		Status  string               `json:"status"`
		AgentID string               `json:"agent_id"`
		Skills  []mcp.AgentSkillView `json:"skills"`
	}
	if err := json.NewDecoder(w.Body).Decode(&putResp); err != nil {
		t.Fatalf("failed to decode PUT response: %v", err)
	}

	for _, sk := range putResp.Skills {
		if sk.SkillID == "vision_receipt_extractor" && sk.Enabled {
			t.Errorf("expected vision_receipt_extractor to be disabled for agent_sm")
		}
		if sk.SkillID == "google_sheets_sync" && !sk.Enabled {
			t.Errorf("expected google_sheets_sync to be enabled for agent_sm")
		}
	}
}

func TestAppsAndSkills_RootAliases(t *testing.T) {
	s := setupTestServerForAppsSkills(t)

	// GET /api/apps
	req := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /api/apps alias, got %d", w.Code)
	}

	// GET /api/skills
	req = httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /api/skills alias, got %d", w.Code)
	}

	// GET /api/agents/agent_dev/skills
	req = httptest.NewRequest(http.MethodGet, "/api/agents/agent_dev/skills", nil)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 from /api/agents/agent_dev/skills alias, got %d", w.Code)
	}
}

func TestDynamicSkillsAndRemoteMCP_Endpoints(t *testing.T) {
	s := setupTestServerForAppsSkills(t)
	s.mcpRegistry.SetSkillsDir(t.TempDir())

	// 1. POST /api/v1/skills/install (content)
	skillPayload := []byte(`{
		"id": "skill_marketing",
		"name": "Marketing Expert",
		"description": "Menulis copy iklan persuasif",
		"category": "marketing",
		"content": "# Marketing SOP\nTulis dengan AIDA framework."
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/skills/install", bytes.NewReader(skillPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from skill install, got %d: %s", w.Code, w.Body.String())
	}

	var installResp struct {
		Status string     `json:"status"`
		Skill  *mcp.Skill `json:"skill"`
	}
	if err := json.NewDecoder(w.Body).Decode(&installResp); err != nil {
		t.Fatal(err)
	}
	if installResp.Skill.ID != "skill_marketing" || installResp.Skill.Name != "Marketing Expert" {
		t.Fatalf("unexpected skill installed: %+v", installResp.Skill)
	}

	// 2. DELETE /api/v1/skills/skill_marketing
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/skills/skill_marketing", nil)
	delW := httptest.NewRecorder()
	s.router.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from skill delete, got %d: %s", delW.Code, delW.Body.String())
	}

	// 3. POST /api/v1/apps/connect-remote
	remoteMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp/tools" {
			json.NewEncoder(rw).Encode(map[string]any{
				"tools": []mcp.ToolDefinition{
					{Name: "tebar_blast", Description: "Blast WhatsApp message"},
				},
			})
			return
		}
		http.NotFound(rw, r)
	}))
	defer remoteMock.Close()

	remotePayload := []byte(`{
		"id": "tebar_app",
		"name": "Tebar Gateway",
		"base_url": "` + remoteMock.URL + `",
		"transport": "http"
	}`)
	mcpReq := httptest.NewRequest(http.MethodPost, "/api/v1/apps/connect-remote", bytes.NewReader(remotePayload))
	mcpReq.Header.Set("Content-Type", "application/json")
	mcpW := httptest.NewRecorder()
	s.router.ServeHTTP(mcpW, mcpReq)

	if mcpW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from remote MCP connect, got %d: %s", mcpW.Code, mcpW.Body.String())
	}

	var mcpResp struct {
		Status string     `json:"status"`
		App    *mcp.App   `json:"app"`
		Tools  []struct{} `json:"tools"`
	}
	if err := json.NewDecoder(mcpW.Body).Decode(&mcpResp); err != nil {
		t.Fatal(err)
	}
	if mcpResp.App.ID != "tebar_app" || len(mcpResp.Tools) != 1 {
		t.Fatalf("unexpected app or tools: %+v", mcpResp)
	}

	// 4. DELETE /api/v1/apps/tebar_app/custom
	delAppReq := httptest.NewRequest(http.MethodDelete, "/api/v1/apps/tebar_app/custom", nil)
	delAppW := httptest.NewRecorder()
	s.router.ServeHTTP(delAppW, delAppReq)
	if delAppW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from app delete, got %d: %s", delAppW.Code, delAppW.Body.String())
	}
}

func TestRemoteMCP_StreamableHTTP_Endpoint(t *testing.T) {
	s := setupTestServerForAppsSkills(t)

	remoteMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" || r.Method != http.MethodPost {
			http.NotFound(rw, r)
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		switch req.Method {
		case "initialize":
			rw.Header().Set("Mcp-Session-Id", "sess-srv-1")
			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"protocolVersion": "2025-03-26"},
			})
		case "notifications/initialized":
			rw.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") != "sess-srv-1" {
				http.Error(rw, "missing session id", http.StatusBadRequest)
				return
			}
			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"tools": []map[string]any{
						{"name": "tebar_schedule_post", "description": "Schedule social post"},
					},
				},
			})
		default:
			http.Error(rw, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer remoteMock.Close()

	remotePayload := []byte(`{
		"id": "tebar_streamable",
		"name": "Tebar MCP",
		"base_url": "` + remoteMock.URL + `/mcp",
		"transport": "http",
		"api_key": "tb_live_test"
	}`)
	mcpReq := httptest.NewRequest(http.MethodPost, "/api/v1/apps/connect-remote", bytes.NewReader(remotePayload))
	mcpReq.Header.Set("Content-Type", "application/json")
	mcpW := httptest.NewRecorder()
	s.router.ServeHTTP(mcpW, mcpReq)

	if mcpW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from streamable remote MCP connect, got %d: %s", mcpW.Code, mcpW.Body.String())
	}
}

func TestRemoteMCP_PersistenceAndStartupRecovery(t *testing.T) {
	var authHeaderSeen atomic.Value
	remoteMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		authHeaderSeen.Store(r.Header.Get("Authorization"))
		if r.URL.Path == "/mcp/tools" && r.Method == http.MethodGet {
			_ = json.NewEncoder(rw).Encode(map[string]any{
				"tools": []mcp.ToolDefinition{
					{Name: "tebar_post", Description: "Create Tebar post"},
				},
			})
			return
		}
		http.NotFound(rw, r)
	}))
	defer remoteMock.Close()

	// 1. Register Remote MCP on initial server instance
	s1 := setupTestServerForAppsSkills(t)
	sharedRepo := s1.remoteMCPRepo

	payload := []byte(`{
		"id": "mcp_tebar",
		"name": "Tebar",
		"description": "Tebar MCP Connector",
		"base_url": "` + remoteMock.URL + `",
		"transport": "http",
		"api_key": "tb_secret_123"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/apps/connect-remote", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s1.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from connect-remote, got %d: %s", w.Code, w.Body.String())
	}

	// Verify persisted in repo with encrypted key
	saved, err := sharedRepo.GetRemoteMCPConnector(context.Background(), "mcp_tebar")
	if err != nil {
		t.Fatalf("expected connector to be persisted in repo: %v", err)
	}
	if saved.APIKeyEncrypted == "" || saved.APIKeyEncrypted == "tb_secret_123" {
		t.Fatalf("expected API key to be stored encrypted, got %q", saved.APIKeyEncrypted)
	}

	// 2. Simulate server restart with fresh in-memory registry and shared repo
	s2 := setupTestServerForAppsSkills(t)
	s2.remoteMCPRepo = sharedRepo
	s2.RestoreRemoteMCPConnectors(context.Background())

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/apps", nil)
	listW := httptest.NewRecorder()
	s2.router.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/v1/apps, got %d", listW.Code)
	}
	var appsResp struct {
		Apps []*mcp.App `json:"apps"`
	}
	if err := json.NewDecoder(listW.Body).Decode(&appsResp); err != nil {
		t.Fatal(err)
	}
	var restored *mcp.App
	for _, a := range appsResp.Apps {
		if a.ID == "mcp_tebar" {
			restored = a
			break
		}
	}
	if restored == nil || restored.Status != "connected" {
		t.Fatalf("expected mcp_tebar to be restored as connected, got %+v", restored)
	}
	if gotAuth, _ := authHeaderSeen.Load().(string); gotAuth != "Bearer tb_secret_123" {
		t.Fatalf("expected decrypted Bearer token on reconnect, got %q", gotAuth)
	}

	// 3. Unregister removes from both registry and DB
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/apps/mcp_tebar/custom", nil)
	delW := httptest.NewRecorder()
	s2.router.ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from DELETE /api/v1/apps/mcp_tebar/custom, got %d: %s", delW.Code, delW.Body.String())
	}

	// Simulate another restart: connector must stay deleted
	s3 := setupTestServerForAppsSkills(t)
	s3.remoteMCPRepo = sharedRepo
	s3.RestoreRemoteMCPConnectors(context.Background())
	if _, err := s3.MCPRegistry().GetApp("mcp_tebar"); err == nil {
		t.Fatalf("expected deleted connector not to reappear after restart")
	}

	// 4. Unreachable endpoint at boot records error status without blocking startup
	offlineServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	offlineURL := offlineServer.URL
	offlineServer.Close()

	if err := sharedRepo.UpsertRemoteMCPConnector(context.Background(), &cuanDB.RemoteMCPConnector{
		ID:        "mcp_offline",
		Name:      "Offline MCP",
		BaseURL:   offlineURL,
		Transport: "http",
		Enabled:   true,
	}); err != nil {
		t.Fatal(err)
	}

	s4 := setupTestServerForAppsSkills(t)
	s4.remoteMCPRepo = sharedRepo
	s4.RestoreRemoteMCPConnectors(context.Background())

	offlineApp, err := s4.MCPRegistry().GetApp("mcp_offline")
	if err != nil {
		t.Fatalf("expected unreachable connector to still be registered on boot: %v", err)
	}
	if offlineApp.Status != "error" || offlineApp.Error == "" {
		t.Fatalf("expected status=error with non-empty Error, got %+v", offlineApp)
	}
}
