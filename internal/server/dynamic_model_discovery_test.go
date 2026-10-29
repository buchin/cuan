package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	cuanDB "github.com/buchin/cuan/internal/db"
)

func TestAPI_GetAgentModelConfig_DynamicDiscoveryAndMetadata(t *testing.T) {
	srv, repo, _ := setupTestServer(t)

	if err := cuanDB.EnsureBuiltinAgents(context.Background(), repo); err != nil {
		t.Fatalf("failed to seed builtin agents: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent_sm/model-config", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Status          string            `json:"status"`
		AgentID         string            `json:"agent_id"`
		Name            string            `json:"name"`
		Provider        string            `json:"provider"`
		ModelID         string            `json:"model_id"`
		FallbackModelID string            `json:"fallback_model_id"`
		SupportedModels []agent.ModelInfo `json:"supported_models"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.AgentID != "agent_sm" {
		t.Errorf("expected agent_id agent_sm, got %s", res.AgentID)
	}
	if len(res.SupportedModels) == 0 {
		t.Fatal("expected non-empty supported_models")
	}

	// Verify metadata fields on models
	foundVision := false
	for _, m := range res.SupportedModels {
		if m.ID == "" {
			t.Error("model ID must not be empty")
		}
		if m.DisplayName == "" {
			t.Errorf("model %s missing display_name", m.ID)
		}
		if m.Provider == "" {
			t.Errorf("model %s missing provider", m.ID)
		}
		if m.Capabilities.Vision || m.SupportsVision {
			foundVision = true
		}
	}
	if !foundVision {
		t.Error("expected at least one vision-capable model in Google supported_models")
	}
}

func TestAPI_GetAgentModelConfig_PreservedLegacyModel(t *testing.T) {
	srv, repo, _ := setupTestServer(t)

	legacyAgent := &cuanDB.Agent{
		ID:              "agent_legacy_custom",
		Name:            "@legacy",
		Role:            "Legacy Specialist",
		Harness:         "agy",
		Provider:        "agy",
		ModelID:         "legacy-custom-model-2023",
		FallbackModelID: "legacy-fallback-model-2022",
		Enabled:         true,
		Status:          cuanDB.AgentStatusActive,
	}

	if err := repo.CreateAgent(context.Background(), legacyAgent); err != nil {
		t.Fatalf("failed to create legacy agent: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent_legacy_custom/model-config", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		ModelID         string            `json:"model_id"`
		FallbackModelID string            `json:"fallback_model_id"`
		SupportedModels []agent.ModelInfo `json:"supported_models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Ensure legacy models are preserved in supported_models
	foundPrimary := false
	foundFallback := false
	for _, m := range res.SupportedModels {
		if m.ID == "legacy-custom-model-2023" {
			foundPrimary = true
			if !m.Preserved {
				t.Error("expected legacy-custom-model-2023 to have preserved: true")
			}
		}
		if m.ID == "legacy-fallback-model-2022" {
			foundFallback = true
			if !m.Preserved {
				t.Error("expected legacy-fallback-model-2022 to have preserved: true")
			}
		}
	}

	if !foundPrimary {
		t.Error("expected legacy-custom-model-2023 to be preserved in supported_models")
	}
	if !foundFallback {
		t.Error("expected legacy-fallback-model-2022 to be preserved in supported_models")
	}
}

func TestAPI_GetAgentModelConfig_ForcedRefresh(t *testing.T) {
	srv, repo, _ := setupTestServer(t)

	if err := cuanDB.EnsureBuiltinAgents(context.Background(), repo); err != nil {
		t.Fatalf("failed to seed builtin agents: %v", err)
	}

	// Call with ?refresh=true
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/agent_sm/model-config?refresh=true", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on forced refresh, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		Status          string            `json:"status"`
		SupportedModels []agent.ModelInfo `json:"supported_models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(res.SupportedModels) == 0 {
		t.Fatal("expected non-empty supported_models after refresh")
	}
}

func TestAPI_ListProviderModels_ProvidersAndMetadata(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	providersToTest := []string{"google", "gemini", "agy", "antigravity", "commandcode"}

	for _, prov := range providersToTest {
		t.Run(prov, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider="+prov, nil)
			rec := httptest.NewRecorder()
			srv.Router().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 OK for provider %s, got %d: %s", prov, rec.Code, rec.Body.String())
			}

			var resp struct {
				Status    string            `json:"status"`
				Provider  string            `json:"provider"`
				Refreshed bool              `json:"refreshed"`
				Models    []agent.ModelInfo `json:"models"`
			}

			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse JSON response: %v", err)
			}

			if resp.Status != "success" {
				t.Errorf("expected status success, got %s", resp.Status)
			}
			if len(resp.Models) == 0 {
				t.Fatalf("expected models for provider %s to be non-empty", prov)
			}

			for _, m := range resp.Models {
				if m.ID == "" {
					t.Errorf("model missing ID for provider %s", prov)
				}
				if m.DisplayName == "" {
					t.Errorf("model %s missing DisplayName for provider %s", m.ID, prov)
				}
				if m.Provider == "" {
					t.Errorf("model %s missing Provider field", m.ID)
				}
				// Verify capabilities match supports_vision
				if m.Capabilities.Vision != m.SupportsVision {
					t.Errorf("model %s capability vision (%v) does not match supports_vision (%v)",
						m.ID, m.Capabilities.Vision, m.SupportsVision)
				}
			}
		})
	}
}

func TestAPI_ListProviderModels_AllAggregation(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=all", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for provider=all, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status   string            `json:"status"`
		Provider string            `json:"provider"`
		Models   []agent.ModelInfo `json:"models"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Provider != "all" {
		t.Errorf("expected provider all, got %s", resp.Provider)
	}

	// Should contain models from multiple providers
	providersSeen := map[string]bool{}
	for _, m := range resp.Models {
		providersSeen[m.Provider] = true
	}

	if len(providersSeen) < 2 {
		t.Errorf("expected models from at least 2 distinct providers, got %v", providersSeen)
	}
}

func TestAPI_ListProviderModels_CacheHitAndForcedRefresh(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	// Use custom cache to control TTL
	customCache := agent.NewModelCatalogCache(5 * time.Minute)
	srv.SetCatalogCache(customCache)

	// Custom mock provider to count calls
	reg := agent.NewProviderRegistry()
	mock := agent.NewMockProvider()
	reg.Register("mock", mock)
	srv.SetProviderRegistry(reg)

	// 1. First probe: populates cache
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=mock", nil)
	rec1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// Check that cache entry exists
	_, _, _, valid := customCache.GetEntry("mock")
	if !valid {
		t.Fatal("expected mock provider to be cached and valid")
	}

	// 2. Second request without refresh: served from cache
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=mock", nil)
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var resp2 struct {
		Refreshed bool `json:"refreshed"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if resp2.Refreshed {
		t.Error("expected refreshed: false on normal cached request")
	}

	// 3. Third request with refresh=true: forces refresh
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=mock&refresh=true", nil)
	rec3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on refresh, got %d: %s", rec3.Code, rec3.Body.String())
	}

	var resp3 struct {
		Refreshed bool `json:"refreshed"`
	}
	_ = json.Unmarshal(rec3.Body.Bytes(), &resp3)
	if !resp3.Refreshed {
		t.Error("expected refreshed: true when ?refresh=true is provided")
	}
}

func TestAPI_ListProviderModels_ResilientFallback(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	reg := agent.NewProviderRegistry()
	mock := agent.NewMockProvider()
	mock.SetFail(true, "upstream connection timeout (simulated)")
	reg.Register("mock", mock)
	srv.SetProviderRegistry(reg)

	// Even if provider fails, resilient fallback serves default mock models
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=mock", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK due to resilient fallback, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Status string            `json:"status"`
		Models []agent.ModelInfo `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Models) == 0 {
		t.Fatal("expected fallback models returned on provider error")
	}
}

func TestAPI_ListProviderModels_UnsupportedProvider(t *testing.T) {
	srv, _, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models?provider=non_existent_provider_xyz", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for unsupported provider, got %d: %s", rec.Code, rec.Body.String())
	}
}
