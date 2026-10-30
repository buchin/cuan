package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/config"
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
