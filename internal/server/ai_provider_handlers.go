package server

import (
	"encoding/json"
	"errors"
	"fmt"
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
		Provider     string `json:"provider"`
		ID           string `json:"id"`
		Mode         string `json:"mode"`
		APIKey       string `json:"api_key"`
		BaseURL      string `json:"base_url"`
		DefaultModel string `json:"default_model"`
		Enabled      *bool  `json:"enabled"`
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

	defaultModel := strings.TrimSpace(input.DefaultModel)
	if defaultModel == "" {
		defaultModel = existing.DefaultModel
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
