package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func setupCommandCodeTestServer(t *testing.T) (*Server, db.AgentRepository, string) {
	t.Helper()

	cfg := &config.Config{
		Environment: "testing",
		AppMode:     "development",
		ServerPort:  "8080",
		AuthSecret:  "test-secret-key-32-chars-long!",
	}

	authStore := auth.NewMemoryStore()
	authSvc := auth.NewService(authStore)
	if err := authSvc.AutoSeedDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("failed to auto seed admin: %v", err)
	}

	repo := db.NewMemoryAgentRepository()
	studioRepo := db.NewMemoryProjectStudioRepository()
	reg := agent.DefaultRegistry()

	srv := NewServer(cfg, nil, authSvc)
	srv.SetAgentRepository(repo)
	srv.SetProjectStudioRepository(studioRepo)
	srv.SetProviderRegistry(reg)

	// Login admin to get valid token
	loginBody := []byte(`{"username":"admin","password":"whosyourdaddy"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("failed to login: %d %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to unmarshal login resp: %v", err)
	}

	return srv, repo, loginResp.Token
}

func TestCommandCode_AuthStatus(t *testing.T) {
	srv, _, _ := setupCommandCodeTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/auth-status?harness=commandcode", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if resp["harness"] != "commandcode" {
		t.Errorf("expected harness commandcode, got %v", resp["harness"])
	}

	// Must NOT return "not verified by this integration"
	msg, _ := resp["message"].(string)
	if strings.Contains(strings.ToLower(msg), "not verified by this integration") {
		t.Errorf("expected clean auth status, got message: %q", msg)
	}
}

func TestCommandCode_AuthInit(t *testing.T) {
	srv, _, token := setupCommandCodeTestServer(t)

	body, _ := json.Marshal(map[string]string{"harness": "commandcode"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/auth-init", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	msg, _ := resp["message"].(string)
	if strings.Contains(strings.ToLower(msg), "not verified by this integration") {
		t.Errorf("expected clean auth init, got message: %q", msg)
	}
}

func TestCommandCode_ListModels(t *testing.T) {
	srv, _, _ := setupCommandCodeTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=commandcode", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status   string            `json:"status"`
		Provider string            `json:"provider"`
		Models   []agent.ModelInfo `json:"models"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Provider != "commandcode" {
		t.Errorf("expected provider commandcode, got %s", resp.Provider)
	}
	if len(resp.Models) == 0 {
		t.Fatal("expected models array to be non-empty")
	}

	hasVisionModel := false
	hasNonVisionModel := false
	for _, m := range resp.Models {
		if m.SupportsVision {
			hasVisionModel = true
		} else {
			hasNonVisionModel = true
		}
	}
	if !hasVisionModel {
		t.Error("expected at least one vision-capable model in models endpoint")
	}
	if !hasNonVisionModel {
		t.Error("expected at least one text-only model in models endpoint")
	}
}

func TestCommandCode_ConnectCLI_Subscription(t *testing.T) {
	srv, repo, token := setupCommandCodeTestServer(t)

	payload := map[string]interface{}{
		"name":      "CC Coder",
		"handle":    "@cc_coder",
		"provider":  "commandcode",
		"auth_type": "subscription",
		"model_id":  "claude-sonnet-5-5",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/connect-cli", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	agentMap, ok := resp["agent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected agent in response: %v", resp)
	}

	agentID := agentMap["id"].(string)
	savedAgent, err := repo.GetAgentByID(context.Background(), agentID)
	if err != nil {
		t.Fatalf("failed to load saved agent: %v", err)
	}
	if savedAgent.Harness != "commandcode" {
		t.Errorf("expected harness commandcode, got %s", savedAgent.Harness)
	}
	if savedAgent.ModelID != "claude-sonnet-5-5" {
		t.Errorf("expected model_id claude-sonnet-5-5, got %s", savedAgent.ModelID)
	}

	// Verify connection endpoint
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+agentID+"/connection/verify", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+token)
	verifyW := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on verify, got %d: %s", verifyW.Code, verifyW.Body.String())
	}
	var verifyResp map[string]interface{}
	_ = json.Unmarshal(verifyW.Body.Bytes(), &verifyResp)
	if verifyResp["connection_state"] != "verified" {
		t.Errorf("expected connection_state verified, got: %v", verifyResp)
	}
}

func TestCommandCode_ConnectCLI_BYOK(t *testing.T) {
	srv, repo, token := setupCommandCodeTestServer(t)

	// 1. Missing / short key should fail
	badPayload := map[string]interface{}{
		"name":          "CC BYOK Bad",
		"handle":        "@cc_byok_bad",
		"provider":      "commandcode",
		"auth_type":     "byok",
		"session_token": "short",
	}
	b, _ := json.Marshal(badPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/connect-cli", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for short key, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Valid key
	goodPayload := map[string]interface{}{
		"name":          "CC BYOK Good",
		"handle":        "@cc_byok_good",
		"provider":      "commandcode",
		"auth_type":     "byok",
		"session_token": "cc_live_valid_api_key_12345",
		"model_id":      "deepseek/deepseek-v4-pro",
	}
	b, _ = json.Marshal(goodPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agents/connect-cli", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	agentMap := resp["agent"].(map[string]interface{})
	agentID := agentMap["id"].(string)

	savedAgent, _ := repo.GetAgentByID(context.Background(), agentID)
	conn, err := repo.GetConnectionByID(context.Background(), savedAgent.ConnectionID)
	if err != nil {
		t.Fatalf("failed to get connection: %v", err)
	}
	if conn.CredentialsEncrypted == "" {
		t.Error("expected credentials to be stored encrypted in connection")
	}
	if conn.PlanHint != "CommandCode custom provider (BYOK)" {
		t.Errorf("expected BYOK plan hint, got %s", conn.PlanHint)
	}

	// Verify connection endpoint
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+agentID+"/connection/verify", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+token)
	verifyW := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on verify, got %d: %s", verifyW.Code, verifyW.Body.String())
	}
	var verifyResp map[string]interface{}
	_ = json.Unmarshal(verifyW.Body.Bytes(), &verifyResp)
	if verifyResp["connection_state"] != "verified" {
		t.Errorf("expected connection_state verified, got: %v", verifyResp)
	}
}

func TestCommandCode_VisionCheck_InboxCLI(t *testing.T) {
	srv, _, _ := setupCommandCodeTestServer(t)

	// Create temp image file
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "sample.png")
	if err := os.WriteFile(imgPath, []byte("fake png content"), 0644); err != nil {
		t.Fatalf("failed to create temp image: %v", err)
	}

	// 1. Non-vision model (deepseek-v4-pro) with image attachment must be rejected with informative error
	ctxNonVision := context.WithValue(context.Background(), inboxModelKey{}, "deepseek/deepseek-v4-pro")
	out, failure := srv.executeInboxCLI(ctxNonVision, "cc-agent", "commandcode", "Analyze this image", tmpDir, tmpDir, nil, imgPath)
	if out != "" {
		t.Errorf("expected empty output on rejection, got %s", out)
	}
	if !strings.Contains(strings.ToLower(failure), "vision") {
		t.Errorf("expected failure message to mention vision, got: %q", failure)
	}

	// 2. Vision model (claude-sonnet-5-5) with image attachment passes vision check (proceeds to CLI invocation)
	// (Note: in unit test environment CLI execution may exit or fail auth, but must not be rejected at the vision gate)
	ctxVision := context.WithValue(context.Background(), inboxModelKey{}, "claude-sonnet-5-5")
	_, failureVision := srv.executeInboxCLI(ctxVision, "cc-agent", "commandcode", "Analyze this image", tmpDir, tmpDir, nil, imgPath)
	if strings.Contains(strings.ToLower(failureVision), "does not support image input") {
		t.Errorf("vision model should not be rejected by vision check, got: %q", failureVision)
	}
}
