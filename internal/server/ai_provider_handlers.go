package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

// handleListAIProviders returns the catalog of AI providers, active modes, models, and binary statuses.
// Routes: GET /api/v1/ai/providers, GET /api/ai/providers
func (s *Server) handleListAIProviders(w http.ResponseWriter, r *http.Request) {
	store := s.AIProviderStore()
	bm := s.BinaryManager()
	reg := s.ProviderRegistry()

	catalog := store.ExportCatalogDTO(bm, reg)
	respondJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"providers": catalog,
	})
}

// handleSaveAIProvider saves a provider configuration and encrypts any submitted API key.
// Routes: POST /api/v1/ai/providers, POST /api/ai/providers
func (s *Server) handleSaveAIProvider(w http.ResponseWriter, r *http.Request) {
	if !s.requireOwnerRole(w, r) {
		return
	}
	var input struct {
		Provider     string  `json:"provider"`
		ID           string  `json:"id"`
		Mode         string  `json:"mode"`
		APIKey       string  `json:"api_key"`
		BaseURL      string  `json:"base_url"`
		DefaultModel *string `json:"default_model"`
		ClearAPIKey  bool    `json:"clear_api_key"`
		Enabled      *bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	providerID := strings.TrimSpace(input.Provider)
	if providerID == "" {
		providerID = strings.TrimSpace(input.ID)
	}
	if providerID == "" {
		respondJSONError(w, http.StatusBadRequest, "provider is required")
		return
	}

	store := s.AIProviderStore()
	existing := store.GetConfig(providerID)

	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode == "" {
		mode = existing.Mode
	}
	if mode == "" {
		mode = "byok"
	}

	encryptedKey := existing.EncryptedAPIKey
	rawKey := strings.TrimSpace(input.APIKey)
	if input.ClearAPIKey {
		if rawKey != "" {
			respondJSONError(w, http.StatusBadRequest, "Choose either a replacement key or disconnect")
			return
		}
		encryptedKey = ""
	}
	if rawKey != "" {
		enc, err := agent.EncryptCredential(rawKey)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to encrypt credential: "+err.Error())
			return
		}
		encryptedKey = enc
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" && input.BaseURL == "" {
		baseURL = existing.BaseURL
	}

	defaultModel := existing.DefaultModel
	if input.DefaultModel != nil {
		defaultModel = strings.TrimSpace(*input.DefaultModel)
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	} else if existing.ID != "" {
		enabled = existing.Enabled
	}

	cfg := agent.AIProviderConfig{
		ID:              providerID,
		Mode:            mode,
		EncryptedAPIKey: encryptedKey,
		BaseURL:         baseURL,
		DefaultModel:    defaultModel,
		Enabled:         enabled,
	}

	if err := store.SaveConfig(cfg); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to save provider config: "+err.Error())
		return
	}

	// Return updated DTO (ensuring API key is safely masked)
	catalog := store.ExportCatalogDTO(s.BinaryManager(), s.ProviderRegistry())
	var updatedDTO *agent.AIProviderDTO
	for i := range catalog {
		if strings.EqualFold(catalog[i].ID, providerID) {
			updatedDTO = &catalog[i]
			break
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"provider": updatedDTO,
	})
}

// handleTestAIProvider tests connectivity and reports response latency in milliseconds.
// Routes: POST /api/v1/ai/providers/test, POST /api/ai/providers/test
func (s *Server) handleTestAIProvider(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider string `json:"provider"`
		ID       string `json:"id"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url"`
		Model    string `json:"model"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	providerID := strings.TrimSpace(input.Provider)
	if providerID == "" {
		providerID = strings.TrimSpace(input.ID)
	}
	if providerID == "" {
		respondJSONError(w, http.StatusBadRequest, "provider is required")
		return
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		storedKey, err := s.AIProviderStore().GetDecryptedAPIKey(providerID)
		if err == nil && storedKey != "" {
			apiKey = storedKey
		}
	}

	reg := s.ProviderRegistry()
	provider, err := reg.Get(providerID)
	if err != nil {
		provider = agent.NewUniversalBYOKProvider(providerID)
	}

	if input.BaseURL != "" {
		if byok, ok := provider.(agent.AIProviderClient); ok {
			byok.SetBaseURL(input.BaseURL)
		}
	}

	start := time.Now()
	probe, err := provider.VerifyConnection(r.Context(), apiKey)
	measuredLatency := time.Since(start).Milliseconds()
	if measuredLatency <= 0 {
		measuredLatency = 5
	}

	if probe != nil && probe.LatencyMs > 0 {
		measuredLatency = probe.LatencyMs
	}

	if err != nil || (probe != nil && !probe.Valid) {
		errMsg := "connection verification failed"
		if err != nil {
			errMsg = err.Error()
		}
		if probe != nil && probe.ErrorMessage != "" {
			errMsg = probe.ErrorMessage
		}
		respondJSON(w, http.StatusOK, map[string]any{
			"status":        "error",
			"valid":         false,
			"provider":      providerID,
			"latency_ms":    measuredLatency,
			"error_message": errMsg,
			"tested_at":     time.Now().UTC(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"valid":      true,
		"provider":   providerID,
		"plan_hint":  probe.PlanHint,
		"latency_ms": measuredLatency,
		"models":     probe.AvailableModels,
		"tested_at":  probe.TestedAt,
	})
}

// handleFetchAIProviderModels discovers models dynamically from provider endpoint or catalog cache.
// Routes: POST /api/v1/ai/providers/models, POST /api/ai/providers/models
func (s *Server) handleFetchAIProviderModels(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Provider string `json:"provider"`
		ID       string `json:"id"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url"`
		Refresh  bool   `json:"refresh"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && err != io.EOF {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	providerID := strings.TrimSpace(input.Provider)
	if providerID == "" {
		providerID = strings.TrimSpace(input.ID)
	}
	if providerID == "" {
		providerID = "claude"
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		if storedKey, err := s.AIProviderStore().GetDecryptedAPIKey(providerID); err == nil && storedKey != "" {
			apiKey = storedKey
		}
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		if cfg := s.AIProviderStore().GetConfig(providerID); cfg.BaseURL != "" {
			baseURL = cfg.BaseURL
		}
	}

	models := s.fetchDynamicProviderModels(r.Context(), providerID, baseURL, apiKey, input.Refresh)

	respondJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"provider": providerID,
		"models":   models,
	})
}

func (s *Server) fetchDynamicProviderModels(ctx context.Context, providerID, baseURL, apiKey string, refresh bool) []agent.ModelInfo {
	p := strings.ToLower(strings.TrimSpace(providerID))

	if baseURL == "" {
		switch p {
		case "anthropic", "claude":
			baseURL = "https://api.anthropic.com"
		case "openai", "codex":
			baseURL = "https://api.openai.com/v1"
		case "xai", "grok":
			baseURL = "https://api.x.ai/v1"
		case "deepseek":
			baseURL = "https://api.deepseek.com"
		case "groq":
			baseURL = "https://api.groq.com/openai/v1"
		case "openrouter", "opencode":
			baseURL = "https://openrouter.ai/api/v1"
		}
	}

	// Live remote probe if apiKey and baseURL are provided
	if apiKey != "" && baseURL != "" {
		cleanBase := strings.TrimRight(baseURL, "/")
		urls := []string{cleanBase + "/models", cleanBase + "/v1/models"}
		isAnthropic := p == "anthropic" || p == "claude" || strings.Contains(strings.ToLower(baseURL), "anthropic")
		if isAnthropic {
			urls = []string{cleanBase + "/v1/models", cleanBase + "/models"}
		}

		client := &http.Client{Timeout: 8 * time.Second}
		for _, targetURL := range urls {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
			if err != nil {
				continue
			}
			req.Header.Set("Accept", "application/json")
			if isAnthropic {
				req.Header.Set("x-api-key", apiKey)
				req.Header.Set("anthropic-version", "2023-06-01")
			} else {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}

			resp, err := client.Do(req)
			if err != nil || resp == nil {
				continue
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				parsed := parseDiscoveredModels(body, p)
				if len(parsed) > 0 {
					return parsed
				}
			}
		}
	}

	// CatalogCache fallback
	if s.CatalogCache() != nil {
		if list, err := s.CatalogCache().DiscoverModels(ctx, s.ProviderRegistry(), p, refresh); err == nil && len(list) > 0 {
			return list
		}
	}

	// Built-in defaults
	switch p {
	case "anthropic", "claude":
		return agent.DefaultBYOKModels("anthropic")
	case "openai", "codex":
		return agent.DefaultBYOKModels("openai")
	case "grok", "xai":
		return agent.DefaultBYOKModels("grok")
	case "google", "agy":
		return agent.DefaultGeminiModels()
	case "commandcode":
		return agent.DefaultCommandCodeModels()
	case "opencode", "openrouter":
		return agent.DefaultBYOKModels("openrouter")
	default:
		return agent.DefaultBYOKModels("openai")
	}
}

type rawModelResp struct {
	Data   []rawModelItem `json:"data"`
	Models []rawModelItem `json:"models"`
}

type rawModelItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	ContextWindow int    `json:"context_window"`
	ContextLength int    `json:"context_length"`
	MaxContext    int    `json:"max_context"`
}

func parseDiscoveredModels(body []byte, provider string) []agent.ModelInfo {
	var resp rawModelResp
	var items []rawModelItem

	if err := json.Unmarshal(body, &resp); err == nil {
		if len(resp.Data) > 0 {
			items = resp.Data
		} else if len(resp.Models) > 0 {
			items = resp.Models
		}
	}

	if len(items) == 0 {
		var arr []rawModelItem
		if err := json.Unmarshal(body, &arr); err == nil {
			items = arr
		}
	}

	if len(items) == 0 {
		return nil
	}

	var results []agent.ModelInfo
	for _, it := range items {
		mid := strings.TrimSpace(it.ID)
		if mid == "" {
			mid = strings.TrimSpace(it.Name)
		}
		if mid == "" {
			continue
		}
		name := it.DisplayName
		if name == "" {
			name = it.Name
		}
		if name == "" {
			name = mid
		}
		limit := it.ContextWindow
		if limit == 0 {
			limit = it.ContextLength
		}
		if limit == 0 {
			limit = it.MaxContext
		}
		if limit == 0 {
			limit = 128000
		}

		vision := agent.IsModelVisionCapable(provider, mid)
		results = append(results, agent.ModelInfo{
			ID:               mid,
			Name:             mid,
			DisplayName:      name,
			Provider:         provider,
			InputTokenLimit:  limit,
			OutputTokenLimit: 8192,
			SupportsVision:   vision,
			Capabilities: agent.ModelCapabilities{
				Vision:         vision,
				SupportsVision: vision,
			},
		})
	}
	return results
}

// handleInstallHarnessBinary triggers 1-click user-space installation or update of a CLI harness binary.
// Routes: POST /api/v1/ai/providers/install-binary, POST /api/ai/providers/install-binary
func (s *Server) handleInstallHarnessBinary(w http.ResponseWriter, r *http.Request) {
	if !s.requireOwnerRole(w, r) {
		return
	}
	var input struct {
		Binary  string `json:"binary"`
		Name    string `json:"name"`
		Harness string `json:"harness"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	binaryName := strings.TrimSpace(input.Binary)
	if binaryName == "" {
		binaryName = strings.TrimSpace(input.Name)
	}
	if binaryName == "" {
		binaryName = strings.TrimSpace(input.Harness)
	}
	if binaryName == "" {
		respondJSONError(w, http.StatusBadRequest, "binary name is required (claude, codex, agy)")
		return
	}

	bm := s.BinaryManager()
	stat, err := bm.InstallBinary(r.Context(), binaryName)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to install binary: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"binary":    stat.Name,
		"installed": stat.Installed,
		"path":      stat.Path,
		"version":   stat.Version,
		"source":    stat.Source,
		"message":   fmt.Sprintf("Binary %s installed successfully to %s", stat.Name, stat.Path),
	})
}

// handleGetAgentProvider returns the AI provider and model binding for a specific agent.
// Routes: GET /api/v1/agents/{id}/provider, GET /api/agents/{id}/provider
func (s *Server) handleGetAgentProvider(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	entity, err := s.agentRepo.GetAgentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get agent: "+err.Error())
		return
	}

	provider := entity.Provider
	if provider == "" {
		provider = entity.Harness
	}
	if provider == "" {
		provider = "google"
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":            "success",
		"agent_id":          entity.ID,
		"provider":          provider,
		"harness":           entity.Harness,
		"model_id":          entity.ModelID,
		"fallback_model_id": entity.FallbackModelID,
	})
}

// handleUpdateAgentProvider binds an AI provider and model to a specific agent.
// Routes: PUT /api/v1/agents/{id}/provider, PUT /api/agents/{id}/provider
func (s *Server) handleUpdateAgentProvider(w http.ResponseWriter, r *http.Request) {
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "agent repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	entity, err := s.agentRepo.GetAgentByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrAgentNotFound) {
			respondJSONError(w, http.StatusNotFound, "agent not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get agent: "+err.Error())
		return
	}

	var input struct {
		Provider        string `json:"provider"`
		Harness         string `json:"harness"`
		ModelID         string `json:"model_id"`
		FallbackModelID string `json:"fallback_model_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Provider != "" {
		entity.Provider = strings.TrimSpace(input.Provider)
	}
	if input.Harness != "" {
		entity.Harness = strings.TrimSpace(input.Harness)
	}
	if input.ModelID != "" {
		entity.ModelID = strings.TrimSpace(input.ModelID)
	}
	if input.FallbackModelID != "" {
		entity.FallbackModelID = strings.TrimSpace(input.FallbackModelID)
	}

	entity.UpdatedAt = time.Now().UTC()

	if err := s.agentRepo.UpdateAgent(r.Context(), entity); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to update agent provider: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"agent":  entity.ToDTO(),
	})
}
