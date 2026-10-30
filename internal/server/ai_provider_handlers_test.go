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
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func setupAIProviderTestServer(t *testing.T) (*Server, db.AgentRepository, string, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "cuan-server-ai-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	tempBinDir := filepath.Join(tempDir, "bin")
	_ = os.MkdirAll(tempBinDir, 0755)
	tempStoreFile := filepath.Join(tempDir, "ai-providers.json")

	cfg := &config.Config{
		AppMode:    "production",
		AuthSecret: "test-auth-secret-phrase-for-ai-providers-testing",
	}

	repo := db.NewMemoryAgentRepository()
	registry := agent.NewProviderRegistry()
	agent.RegisterBYOKProviders(registry)

	bm := agent.NewBinaryManager(tempBinDir)
	store := agent.NewAIProviderStore(tempStoreFile)

	srv := NewServer(cfg, nil)
	srv.SetAgentRepository(repo)
	srv.SetProviderRegistry(registry)
	srv.SetBinaryManager(bm)
	srv.SetAIProviderStore(store)

	return srv, repo, tempDir, tempBinDir
}

func TestAIProviderHandlers_ListProviders(t *testing.T) {
	srv, _, tempDir, _ := setupAIProviderTestServer(t)
	defer os.RemoveAll(tempDir)

	for _, route := range []string{"/api/v1/ai/providers", "/api/ai/providers"} {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GET %s returned code %d, body: %s", route, w.Code, w.Body.String())
		}

		var resp struct {
			Status    string                `json:"status"`
			Providers []agent.AIProviderDTO `json:"providers"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response for %s: %v", route, err)
		}

		if resp.Status != "success" {
			t.Errorf("expected status 'success', got %q", resp.Status)
		}

		if len(resp.Providers) == 0 {
			t.Fatalf("expected non-empty providers catalog")
		}

		// Verify expected providers are present
		foundDeepSeek := false
		foundOpenAI := false
		foundAnthropic := false
		for _, p := range resp.Providers {
			if p.ID == "deepseek" {
				foundDeepSeek = true
				if len(p.Models) == 0 {
					t.Errorf("expected models for DeepSeek")
				}
				if len(p.SupportedModes) != 1 || p.SupportedModes[0] != "byok" {
					t.Errorf("unexpected supported modes for DeepSeek: %v", p.SupportedModes)
				}
			}
			if p.ID == "openai" {
				foundOpenAI = true
				if p.BinaryStatus == nil {
					t.Errorf("expected BinaryStatus for OpenAI harness")
				}
			}
			if p.ID == "anthropic" {
				foundAnthropic = true
				if p.BinaryStatus == nil {
					t.Errorf("expected BinaryStatus for Anthropic harness")
				}
			}
		}

		if !foundDeepSeek || !foundOpenAI || !foundAnthropic {
			t.Errorf("missing core providers: deepseek=%v, openai=%v, anthropic=%v",
				foundDeepSeek, foundOpenAI, foundAnthropic)
		}
	}
}

func TestAIProviderHandlers_SaveProviderAndEncryption(t *testing.T) {
	srv, _, tempDir, _ := setupAIProviderTestServer(t)
	defer os.RemoveAll(tempDir)

	rawSecretKey := "sk-test-live-deepseek-api-key-998877"
	payload := map[string]any{
		"provider":      "deepseek",
		"mode":          "byok",
		"api_key":       rawSecretKey,
		"default_model": "deepseek-chat",
		"enabled":       true,
	}
	bodyBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/ai/providers returned code %d, body: %s", w.Code, w.Body.String())
	}

	rawResp := w.Body.String()
	// Security Assertion: raw secret key must NEVER be leaked in JSON response
	if strings.Contains(rawResp, rawSecretKey) {
		t.Fatalf("SECURITY VIOLATION: raw API key leaked in response: %s", rawResp)
	}

	var resp struct {
		Status   string              `json:"status"`
		Provider agent.AIProviderDTO `json:"provider"`
	}
	if err := json.Unmarshal([]byte(rawResp), &resp); err != nil {
		t.Fatalf("failed to parse save response: %v", err)
	}

	if resp.Provider.ID != "deepseek" {
		t.Errorf("expected provider ID 'deepseek', got %q", resp.Provider.ID)
	}
	if !resp.Provider.HasAPIKey {
		t.Errorf("expected HasAPIKey to be true")
	}
	if resp.Provider.APIKeyMasked != "sk-...8877" {
		t.Errorf("expected masked API key 'sk-...8877', got %q", resp.Provider.APIKeyMasked)
	}

	// Verify encryption on disk: file must contain ciphertext, not plaintext
	storeFile := filepath.Join(tempDir, "ai-providers.json")
	fileBytes, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("failed to read persisted store file: %v", err)
	}
	if strings.Contains(string(fileBytes), rawSecretKey) {
		t.Fatalf("SECURITY VIOLATION: raw API key found in plaintext in store file: %s", string(fileBytes))
	}

	// Decrypt via store and assert match
	decrypted, err := srv.AIProviderStore().GetDecryptedAPIKey("deepseek")
	if err != nil {
		t.Fatalf("GetDecryptedAPIKey failed: %v", err)
	}
	if decrypted != rawSecretKey {
		t.Errorf("decrypted key mismatch: got %q, want %q", decrypted, rawSecretKey)
	}
}

func TestAIProviderHandlers_TestLatencyEndpoint(t *testing.T) {
	srv, _, tempDir, _ := setupAIProviderTestServer(t)
	defer os.RemoveAll(tempDir)

	// 1. Test with valid mock key
	payload := map[string]any{
		"provider": "deepseek",
		"api_key":  "mock-valid-key-12345",
	}
	bodyBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/ai/providers/test returned code %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status    string            `json:"status"`
		Valid     bool              `json:"valid"`
		Provider  string            `json:"provider"`
		LatencyMs int64             `json:"latency_ms"`
		Models    []agent.ModelInfo `json:"models"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode test probe response: %v", err)
	}

	if !resp.Valid {
		t.Errorf("expected valid probe result for mock key")
	}
	if resp.LatencyMs <= 0 {
		t.Errorf("expected positive latency, got %d", resp.LatencyMs)
	}
	if len(resp.Models) == 0 {
		t.Errorf("expected models returned from probe")
	}

	// 2. Test with invalid key
	invalidPayload := map[string]any{
		"provider": "deepseek",
		"api_key":  "invalid-bad-key",
	}
	invBytes, _ := json.Marshal(invalidPayload)

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/test", bytes.NewReader(invBytes))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, req2)

	var invResp struct {
		Valid        bool   `json:"valid"`
		ErrorMessage string `json:"error_message"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &invResp)
	if invResp.Valid {
		t.Errorf("expected invalid probe result for bad key")
	}
	if invResp.ErrorMessage == "" {
		t.Errorf("expected error message for invalid key")
	}
}

func TestAIProviderHandlers_InstallBinaryEndpoint(t *testing.T) {
	srv, _, tempDir, tempBinDir := setupAIProviderTestServer(t)
	defer os.RemoveAll(tempDir)

	payload := map[string]any{
		"binary": "claude",
	}
	bodyBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/install-binary", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/ai/providers/install-binary returned code %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status    string `json:"status"`
		Binary    string `json:"binary"`
		Installed bool   `json:"installed"`
		Path      string `json:"path"`
		Version   string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse install response: %v", err)
	}

	if resp.Status != "success" || !resp.Installed {
		t.Errorf("expected status success and installed true, got %+v", resp)
	}
	if !strings.HasPrefix(resp.Path, tempBinDir) {
		t.Errorf("expected installed path to reside in %s, got %s", tempBinDir, resp.Path)
	}

	// Verify file is executable
	fi, err := os.Stat(resp.Path)
	if err != nil {
		t.Fatalf("stat failed on installed file: %v", err)
	}
	if fi.Mode()&0111 == 0 {
		t.Errorf("expected executable permissions on installed binary %s", resp.Path)
	}
}

func TestAIProviderHandlers_AgentProviderBinding(t *testing.T) {
	srv, repo, tempDir, _ := setupAIProviderTestServer(t)
	defer os.RemoveAll(tempDir)

	// Create test agent
	testAgent := &db.Agent{
		ID:              "agent-test-binding-1",
		Name:            "CS Automation Agent",
		Role:            "Customer Service",
		Provider:        "google",
		Harness:         "agy",
		ModelID:         "gemini-1.5-pro",
		FallbackModelID: "gemini-1.5-flash",
		Status:          "active",
	}
	if err := repo.CreateAgent(context.Background(), testAgent); err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}

	// 1. GET /api/v1/agents/{id}/provider
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-test-binding-1/provider", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET agent provider returned %d: %s", w.Code, w.Body.String())
	}

	var getResp struct {
		Status   string `json:"status"`
		AgentID  string `json:"agent_id"`
		Provider string `json:"provider"`
		ModelID  string `json:"model_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &getResp)
	if getResp.Provider != "google" || getResp.ModelID != "gemini-1.5-pro" {
		t.Errorf("unexpected initial provider binding: %+v", getResp)
	}

	// 2. PUT /api/v1/agents/{id}/provider: switch to DeepSeek V3 / R1
	updatePayload := map[string]any{
		"provider":          "deepseek",
		"model_id":          "deepseek-chat",
		"fallback_model_id": "deepseek-reasoner",
	}
	upBytes, _ := json.Marshal(updatePayload)

	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/agents/agent-test-binding-1/provider", bytes.NewReader(upBytes))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("PUT agent provider returned %d: %s", w2.Code, w2.Body.String())
	}

	// 3. GET /api/v1/agents/{id}/provider again to verify update
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent-test-binding-1/provider", nil)
	w3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w3, req3)

	_ = json.Unmarshal(w3.Body.Bytes(), &getResp)
	if getResp.Provider != "deepseek" {
		t.Errorf("expected updated provider 'deepseek', got %q", getResp.Provider)
	}
	if getResp.ModelID != "deepseek-chat" {
		t.Errorf("expected updated model_id 'deepseek-chat', got %q", getResp.ModelID)
	}
}
