package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func setupTestServer(t *testing.T) (*Server, db.AgentRepository, *agent.MockProvider) {
	t.Helper()

	cfg := &config.Config{
		AppMode:    "production",
		AuthSecret: "test-auth-secret-phrase-for-unit-testing",
	}

	repo := db.NewMemoryAgentRepository()
	mockProvider := agent.NewMockProvider()

	registry := agent.NewProviderRegistry()
	registry.Register("mock", mockProvider)
	registry.Register("google", mockProvider) // Use mock provider for deterministic server handler testing

	srv := NewServer(cfg, nil)
	srv.SetAgentRepository(repo)
	srv.SetProviderRegistry(registry)

	return srv, repo, mockProvider
}

func TestAgentHandlers_CRUD_RootAndV1(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// 1. Create Agent via POST /api/agents
	createPayload := map[string]interface{}{
		"name":          "Architect Alpha",
		"avatar":        "https://avatar.cuan.test/alpha.png",
		"role":          "Senior System Architect",
		"system_prompt": "You are a pragmatic, senior Go engineer and system architect.",
		"model_id":      "gemini-1.5-pro",
		"provider":      "google",
		"status":        "active",
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on POST /api/agents, got %d: %s", w.Code, w.Body.String())
	}

	var createResp struct {
		Status string      `json:"status"`
		Agent  db.AgentDTO `json:"agent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create agent response: %v", err)
	}
	if createResp.Agent.ID == "" || createResp.Agent.Name != "Architect Alpha" {
		t.Errorf("unexpected agent data: %+v", createResp.Agent)
	}
	agentID := createResp.Agent.ID

	// 2. Get Agent via GET /api/v1/agents/{id} (Backward compatibility test)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/agents/%s", agentID), nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/agents/%s, got %d: %s", agentID, w.Code, w.Body.String())
	}

	var getResp struct {
		Status string      `json:"status"`
		Agent  db.AgentDTO `json:"agent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get agent response: %v", err)
	}
	if getResp.Agent.ID != agentID || getResp.Agent.Role != "Senior System Architect" {
		t.Errorf("unexpected agent role or id: %+v", getResp.Agent)
	}

	// 3. Update Agent via PUT /api/agents/{id}
	newRole := "Lead Software Architect"
	newPrompt := "You lead technical architecture design."
	updatePayload := map[string]interface{}{
		"role":          newRole,
		"system_prompt": newPrompt,
	}
	updateBody, _ := json.Marshal(updatePayload)

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/agents/%s", agentID), bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PUT /api/agents/%s, got %d: %s", agentID, w.Code, w.Body.String())
	}

	var updateResp struct {
		Status string      `json:"status"`
		Agent  db.AgentDTO `json:"agent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("failed to decode update agent response: %v", err)
	}
	if updateResp.Agent.Role != newRole || updateResp.Agent.SystemPrompt != newPrompt {
		t.Errorf("update fields mismatch: got %+v", updateResp.Agent)
	}

	// 4. List Agents via GET /api/agents & GET /api/v1/agents
	for _, path := range []string{"/api/agents", "/api/v1/agents"} {
		req = httptest.NewRequest(http.MethodGet, path+"?provider=google&search=Architect", nil)
		w = httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on %s, got %d: %s", path, w.Code, w.Body.String())
		}

		var listResp struct {
			Status string        `json:"status"`
			Agents []db.AgentDTO `json:"agents"`
			Count  int           `json:"count"`
			Total  int           `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("failed to decode list agents response: %v", err)
		}
		if listResp.Count != 1 || listResp.Agents[0].ID != agentID {
			t.Errorf("list mismatch on %s: count=%d, total=%d", path, listResp.Count, listResp.Total)
		}
	}

	// 5. Delete Agent via DELETE /api/v1/agents/{id}
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/agents/%s", agentID), nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on DELETE /api/v1/agents/%s, got %d: %s", agentID, w.Code, w.Body.String())
	}

	// 6. Verify 404 after deletion
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/agents/%s", agentID), nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found after deletion, got %d", w.Code)
	}
}

func TestAgentConnectionHandlers_EncryptionAndVerification(t *testing.T) {
	srv, repo, mockProvider := setupTestServer(t)

	plaintextKey := "AIzaSyD-GoogleGeminiProTestAPIKeySprint09"

	// 1. Create connection with plaintext api_key via POST /api/agent-connections
	createPayload := map[string]interface{}{
		"user_id":   "buchin-dev",
		"provider":  "google",
		"api_key":   plaintextKey,
		"plan_hint": "BYOK Google AI Studio",
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/agent-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on POST /api/agent-connections, got %d: %s", w.Code, w.Body.String())
	}

	respStr := w.Body.String()
	// Zero plaintext leakage check: response body MUST NOT contain plaintext key!
	if strings.Contains(respStr, plaintextKey) {
		t.Fatalf("SECURITY VIOLATION: response leaked plaintext API key: %s", respStr)
	}

	var connResp struct {
		Status     string                `json:"status"`
		Connection db.AgentConnectionDTO `json:"connection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &connResp); err != nil {
		t.Fatalf("failed to decode create connection response: %v", err)
	}

	connID := connResp.Connection.ID
	if connID == "" {
		t.Fatalf("expected non-empty connection ID")
	}
	if !connResp.Connection.HasCredentials {
		t.Errorf("expected HasCredentials to be true")
	}
	if connResp.Connection.Status != "active" {
		t.Errorf("expected connection status active, got %s", connResp.Connection.Status)
	}

	// Verify encryption in repository / database:
	persisted, err := repo.GetConnectionByID(context.Background(), connID)
	if err != nil {
		t.Fatalf("failed to retrieve connection from repo: %v", err)
	}
	if persisted.CredentialsEncrypted == "" || strings.Contains(persisted.CredentialsEncrypted, plaintextKey) {
		t.Fatalf("persisted credentials in DB not properly encrypted: %s", persisted.CredentialsEncrypted)
	}

	// Verify decryption reproduces original plaintext
	decrypted, err := agent.DecryptCredential(persisted.CredentialsEncrypted)
	if err != nil {
		t.Fatalf("failed to decrypt persisted credential: %v", err)
	}
	if decrypted != plaintextKey {
		t.Fatalf("decrypted credential mismatch: got %q, want %q", decrypted, plaintextKey)
	}

	// 2. Verify connection via POST /api/agent-connections/{id}/verify
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/agent-connections/%s/verify", connID), nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on verify, got %d: %s", w.Code, w.Body.String())
	}

	var verifyResp struct {
		Status     string                      `json:"status"`
		Verified   bool                        `json:"verified"`
		Probe      agent.ConnectionProbeResult `json:"probe"`
		Connection db.AgentConnectionDTO       `json:"connection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("failed to decode verify response: %v", err)
	}

	if !verifyResp.Verified || !verifyResp.Probe.Valid {
		t.Errorf("expected verification success: %+v", verifyResp)
	}
	if len(verifyResp.Probe.AvailableModels) == 0 {
		t.Errorf("expected available models in verification response")
	}

	// 3. Test verification failure when provider fails
	mockProvider.SetFail(true, "Google API Quota Exhausted: 429 RESOURCE_EXHAUSTED")

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/agent-connections/%s/verify", connID), nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on verify failure response, got %d: %s", w.Code, w.Body.String())
	}

	var failResp struct {
		Status     string                      `json:"status"`
		Verified   bool                        `json:"verified"`
		Error      string                      `json:"error"`
		Probe      agent.ConnectionProbeResult `json:"probe"`
		Connection db.AgentConnectionDTO       `json:"connection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &failResp); err != nil {
		t.Fatalf("failed to decode fail response: %v", err)
	}

	if failResp.Verified {
		t.Errorf("expected verified=false on mock failure")
	}
	if failResp.Connection.Status != "invalid" {
		t.Errorf("expected connection status to transition to 'invalid', got %s", failResp.Connection.Status)
	}
	if !strings.Contains(failResp.Error, "Quota Exhausted") {
		t.Errorf("expected error message to contain 'Quota Exhausted', got %s", failResp.Error)
	}

	// Reset mock provider
	mockProvider.SetFail(false, "")

	// 4. Test backward-compatibility endpoints (/api/v1/agents/connections and /api/v1/agent-connections)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/connections?user_id=buchin-dev", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/agents/connections, got %d: %s", w.Code, w.Body.String())
	}

	var listResp struct {
		Status      string                    `json:"status"`
		Connections []db.AgentConnectionDTO   `json:"connections"`
		Count       int                       `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Count != 1 || listResp.Connections[0].ID != connID {
		t.Errorf("expected 1 connection matching ID, got %d", listResp.Count)
	}
}

func TestAgentConnectionHandlers_ImmediateVerificationOnCreate(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// Create with verify_now: true and valid key
	payload := map[string]interface{}{
		"user_id":    "buchin-dev",
		"provider":   "google",
		"api_key":    "valid-mock-key-immediate-verify",
		"verify_now": true,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on immediate verify create, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status     string                      `json:"status"`
		Verified   bool                        `json:"verified"`
		Probe      agent.ConnectionProbeResult `json:"probe"`
		Connection db.AgentConnectionDTO       `json:"connection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	if !resp.Verified || !resp.Probe.Valid {
		t.Errorf("expected verified=true in immediate create response")
	}
	if resp.Connection.Status != "active" {
		t.Errorf("expected active status, got %s", resp.Connection.Status)
	}
}

func TestAgentHandlers_ValidationAndErrors(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// 1. Create Agent without name -> 400 Bad Request
	body, _ := json.Marshal(map[string]interface{}{
		"name": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty agent name, got %d", w.Code)
	}

	// 2. Get non-existent agent -> 404 Not Found
	req = httptest.NewRequest(http.MethodGet, "/api/agents/non-existent-agent-id", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}

	// 3. Update non-existent agent -> 404 Not Found
	req = httptest.NewRequest(http.MethodPut, "/api/agents/non-existent-agent-id", bytes.NewReader([]byte(`{"name":"test"}`)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}

	// 4. Delete non-existent agent -> 404 Not Found
	req = httptest.NewRequest(http.MethodDelete, "/api/agents/non-existent-agent-id", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}
}

func TestAgentConnectionHandlers_ValidationAndErrors(t *testing.T) {
	srv, _, mockProvider := setupTestServer(t)

	// 1. Missing credentials -> 400 Bad Request
	body, _ := json.Marshal(map[string]interface{}{
		"user_id":  "test-user",
		"provider": "google",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing credentials, got %d", w.Code)
	}

	// 2. Create with immediate verify failing -> 422 Unprocessable Entity
	mockProvider.SetFail(true, "Authentication failed: invalid token")
	body, _ = json.Marshal(map[string]interface{}{
		"user_id":    "test-user",
		"provider":   "google",
		"api_key":    "bad-token-xyz",
		"verify_now": true,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/agent-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 Unprocessable Entity on verify failure, got %d: %s", w.Code, w.Body.String())
	}

	// Reset mock provider
	mockProvider.SetFail(false, "")

	// 3. Create normal connection then test GET and DELETE connection endpoints
	createBody, _ := json.Marshal(map[string]interface{}{
		"user_id":  "user-to-delete",
		"provider": "google",
		"api_key":  "AIzaSy-sample-key",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/agent-connections", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w.Code)
	}

	var created struct {
		Connection db.AgentConnectionDTO `json:"connection"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	targetID := created.Connection.ID

	// GET /api/agent-connections/{id}
	req = httptest.NewRequest(http.MethodGet, "/api/agent-connections/"+targetID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK on GET connection, got %d", w.Code)
	}

	// DELETE /api/agent-connections/{id}
	req = httptest.NewRequest(http.MethodDelete, "/api/agent-connections/"+targetID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK on DELETE connection, got %d", w.Code)
	}

	// Verify 404
	req = httptest.NewRequest(http.MethodGet, "/api/agent-connections/"+targetID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after connection delete, got %d", w.Code)
	}
}

func TestAgentHandlers_S23_1_FuralStyleRegistry(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// 1. POST /api/v1/agents with Fural entity format { id, name, harness, connection_id, enabled }
	createPayload := map[string]interface{}{
		"name":          "@dev",
		"harness":       "agy",
		"connection_id": "conn_dev_session_123",
		"enabled":       true,
	}
	body, _ := json.Marshal(createPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on POST /api/v1/agents, got %d: %s", w.Code, w.Body.String())
	}

	var createResp struct {
		Status string      `json:"status"`
		Agent  db.AgentDTO `json:"agent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create agent response: %v", err)
	}

	agent := createResp.Agent
	if agent.ID == "" {
		t.Errorf("expected generated agent id")
	}
	if agent.Name != "@dev" {
		t.Errorf("expected agent name '@dev', got %q", agent.Name)
	}
	if agent.Harness != "agy" {
		t.Errorf("expected harness 'agy', got %q", agent.Harness)
	}
	if agent.ConnectionID != "conn_dev_session_123" {
		t.Errorf("expected connection_id 'conn_dev_session_123', got %q", agent.ConnectionID)
	}
	if !agent.Enabled {
		t.Errorf("expected enabled true")
	}

	agentID := agent.ID

	// 2. GET /api/v1/agents/{id}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/agents/%s, got %d", agentID, w.Code)
	}

	var getResp struct {
		Status string      `json:"status"`
		Agent  db.AgentDTO `json:"agent"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &getResp)
	if getResp.Agent.Name != "@dev" || getResp.Agent.Harness != "agy" || getResp.Agent.ConnectionID != "conn_dev_session_123" || !getResp.Agent.Enabled {
		t.Errorf("get agent mismatch: %+v", getResp.Agent)
	}

	// 3. GET /api/v1/agents (List)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/v1/agents, got %d", w.Code)
	}

	var listResp struct {
		Status string        `json:"status"`
		Agents []db.AgentDTO `json:"agents"`
		Count  int           `json:"count"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Count < 1 {
		t.Errorf("expected at least 1 agent, got %d", listResp.Count)
	}

	// 4. PATCH /api/v1/agents/{id} to update harness and toggle enabled
	patchPayload := map[string]interface{}{
		"name":    "@worker_flash",
		"harness": "claude",
		"enabled": false,
	}
	patchBody, _ := json.Marshal(patchPayload)
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/agents/"+agentID, bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PATCH /api/v1/agents/%s, got %d: %s", agentID, w.Code, w.Body.String())
	}

	var patchResp struct {
		Status string      `json:"status"`
		Agent  db.AgentDTO `json:"agent"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &patchResp)
	if patchResp.Agent.Name != "@worker_flash" {
		t.Errorf("expected name '@worker_flash', got %q", patchResp.Agent.Name)
	}
	if patchResp.Agent.Harness != "claude" {
		t.Errorf("expected harness 'claude', got %q", patchResp.Agent.Harness)
	}
	if patchResp.Agent.Enabled {
		t.Errorf("expected enabled false after patch")
	}

	// 5. DELETE /api/v1/agents/{id}
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/agents/"+agentID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on DELETE /api/v1/agents/%s, got %d", agentID, w.Code)
	}

	// Verify 404 after delete
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after agent deletion, got %d", w.Code)
	}
}

func TestServer_HarnessAuthEndpoints(t *testing.T) {
	srv := NewServer(&config.Config{}, nil)

	// 1. GET /api/v1/agents/auth-status?harness=claude
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/auth-status?harness=claude", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on auth-status, got %d", w.Code)
	}

	var statusResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("failed to decode auth-status json: %v", err)
	}
	if statusResp["status"] != "success" || statusResp["harness"] != "claude" {
		t.Errorf("unexpected status response: %v", statusResp)
	}

	// 2. POST /api/v1/agents/auth-init (non-agy harness)
	initBody, _ := json.Marshal(map[string]string{"harness": "codex"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agents/auth-init", bytes.NewReader(initBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on auth-init, got %d", w.Code)
	}

	// 3. POST /api/v1/agents/auth-code (validation error on empty code)
	codeBody, _ := json.Marshal(map[string]string{"session_id": "test", "code": ""})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/agents/auth-code", bytes.NewReader(codeBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty code, got %d", w.Code)
	}
}


