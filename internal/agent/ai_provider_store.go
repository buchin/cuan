package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AIProviderConfig stores persistent configuration for an AI provider.
type AIProviderConfig struct {
	ID              string    `json:"id"`
	Mode            string    `json:"mode"` // "subscription" | "byok"
	EncryptedAPIKey string    `json:"encrypted_api_key,omitempty"`
	BaseURL         string    `json:"base_url,omitempty"`
	DefaultModel    string    `json:"default_model,omitempty"`
	Enabled         bool      `json:"enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AIProviderDTO represents the public client-safe view of a provider configuration.
type AIProviderDTO struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Mode           string               `json:"mode"` // "subscription" | "byok"
	SupportedModes []string             `json:"supported_modes"`
	HasAPIKey      bool                 `json:"has_api_key"`
	APIKeyMasked   string               `json:"api_key_masked,omitempty"`
	BaseURL        string               `json:"base_url,omitempty"`
	DefaultModel   string               `json:"default_model,omitempty"`
	Models         []ModelInfo          `json:"models"`
	BinaryStatus   *HarnessBinaryStatus `json:"binary_status,omitempty"`
	Status         string               `json:"status"` // "ready", "configured", "unconfigured"
	Enabled        bool                 `json:"enabled"`
}

// AIProviderStore persists provider configurations safely to disk.
type AIProviderStore struct {
	mu       sync.RWMutex
	filePath string
	configs  map[string]AIProviderConfig
}

var (
	defaultAIProviderStore     *AIProviderStore
	defaultAIProviderStoreOnce sync.Once
)

// DefaultAIProviderStore returns the singleton provider store.
func DefaultAIProviderStore() *AIProviderStore {
	defaultAIProviderStoreOnce.Do(func() {
		dir := filepath.Join(ConfigHome(), ".local", "share", "cuan")
		_ = os.MkdirAll(dir, 0755)
		defaultAIProviderStore = NewAIProviderStore(filepath.Join(dir, "ai-providers.json"))
	})
	return defaultAIProviderStore
}

// NewAIProviderStore creates a new AIProviderStore with the specified backing file.
func NewAIProviderStore(filePath string) *AIProviderStore {
	store := &AIProviderStore{
		filePath: filePath,
		configs:  make(map[string]AIProviderConfig),
	}
	_ = store.load()
	return store
}

func (s *AIProviderStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var list []AIProviderConfig
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.configs = make(map[string]AIProviderConfig, len(list))
	for _, cfg := range list {
		s.configs[strings.ToLower(cfg.ID)] = cfg
	}
	return nil
}

func (s *AIProviderStore) saveLocked() error {
	list := make([]AIProviderConfig, 0, len(s.configs))
	for _, cfg := range s.configs {
		list = append(list, cfg)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600)
}

// GetConfig returns the configuration for a provider, or default values if unconfigured.
func (s *AIProviderStore) GetConfig(providerID string) AIProviderConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normID := strings.ToLower(strings.TrimSpace(providerID))
	if cfg, ok := s.configs[normID]; ok {
		return cfg
	}

	defaultMode := "byok"
	if normID == "anthropic" || normID == "google" || normID == "agy" || normID == "openai" {
		defaultMode = "subscription"
	}

	return AIProviderConfig{
		ID:        normID,
		Mode:      defaultMode,
		Enabled:   true,
		UpdatedAt: time.Time{},
	}
}

// SaveConfig updates and persists a provider configuration.
func (s *AIProviderStore) SaveConfig(cfg AIProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	normID := strings.ToLower(strings.TrimSpace(cfg.ID))
	if normID == "" {
		return fmt.Errorf("provider ID cannot be empty")
	}

	cfg.ID = normID
	cfg.UpdatedAt = time.Now().UTC()

	s.configs[normID] = cfg
	return s.saveLocked()
}

// GetDecryptedAPIKey retrieves and decrypts the API key for a provider, checking env vars as fallback.
func (s *AIProviderStore) GetDecryptedAPIKey(providerID string) (string, error) {
	cfg := s.GetConfig(providerID)
	if cfg.EncryptedAPIKey != "" {
		decrypted, err := DecryptCredential(cfg.EncryptedAPIKey)
		if err == nil && decrypted != "" {
			return decrypted, nil
		}
	}

	// Environment variable fallback
	envVal := resolveEnvAPIKey(providerID)
	if envVal != "" {
		return envVal, nil
	}

	return "", nil
}

func resolveEnvAPIKey(providerID string) string {
	switch strings.ToLower(providerID) {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "deepseek":
		return os.Getenv("DEEPSEEK_API_KEY")
	case "anthropic", "claude":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "grok", "xai":
		if v := os.Getenv("GROK_API_KEY"); v != "" {
			return v
		}
		return os.Getenv("XAI_API_KEY")
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	case "groq":
		return os.Getenv("GROQ_API_KEY")
	case "google", "gemini":
		return os.Getenv("GEMINI_API_KEY")
	default:
		return ""
	}
}

// ExportCatalogDTO constructs the complete catalog view of all supported providers.
func (s *AIProviderStore) ExportCatalogDTO(bm *BinaryManager, reg *ProviderRegistry) []AIProviderDTO {
	if bm == nil {
		bm = DefaultBinaryManager()
	}
	if reg == nil {
		reg = DefaultRegistry()
	}

	catalogDefs := []struct {
		id             string
		name           string
		harness        string
		supportedModes []string
		defaultModel   string
	}{
		{id: "anthropic", name: "Anthropic (Claude)", harness: HarnessClaude, supportedModes: []string{"subscription", "byok"}, defaultModel: "claude-sonnet-5-5"},
		{id: "openai", name: "OpenAI", harness: HarnessCodex, supportedModes: []string{"subscription", "byok"}, defaultModel: "gpt-6.1-sol"},
		{id: "grok", name: "xAI (Grok)", harness: HarnessGrok, supportedModes: []string{"subscription", "byok"}, defaultModel: "grok-4.7"},
		{id: "google", name: "Google (Antigravity)", harness: HarnessAGY, supportedModes: []string{"subscription", "byok"}, defaultModel: "gemini-4-neon"},
		{id: "commandcode", name: "CommandCode", harness: "commandcode", supportedModes: []string{"subscription", "byok"}, defaultModel: "commandcode-default"},
		{id: "deepseek", name: "DeepSeek", harness: "", supportedModes: []string{"byok"}, defaultModel: "deepseek-chat"},
		{id: "openrouter", name: "OpenRouter", harness: "", supportedModes: []string{"byok"}, defaultModel: "deepseek/deepseek-r1"},
		{id: "groq", name: "Groq", harness: "", supportedModes: []string{"byok"}, defaultModel: "llama-3.3-70b-versatile"},
		{id: "ollama", name: "Ollama (Local)", harness: "", supportedModes: []string{"byok"}, defaultModel: "llama3"},
	}

	dtos := make([]AIProviderDTO, 0, len(catalogDefs))
	for _, def := range catalogDefs {
		cfg := s.GetConfig(def.id)

		var binStat *HarnessBinaryStatus
		if def.harness != "" {
			stat := bm.DetectHarness(def.harness)
			binStat = &stat
		}

		key, _ := s.GetDecryptedAPIKey(def.id)
		hasKey := (key != "")
		maskedKey := ""
		if hasKey {
			maskedKey = MaskAPIKey(key)
		}

		models := DefaultBYOKModels(def.id)
		if def.id == "google" {
			models = DefaultGeminiModels()
		}

		defaultM := def.defaultModel
		if cfg.DefaultModel != "" {
			defaultM = cfg.DefaultModel
		}

		status := "unconfigured"
		if cfg.Mode == "subscription" {
			if binStat != nil && binStat.Installed {
				status = "ready"
			} else {
				status = "configured"
			}
		} else {
			if hasKey {
				status = "ready"
			}
		}

		dtos = append(dtos, AIProviderDTO{
			ID:             def.id,
			Name:           def.name,
			Mode:           cfg.Mode,
			SupportedModes: def.supportedModes,
			HasAPIKey:      hasKey,
			APIKeyMasked:   maskedKey,
			BaseURL:        cfg.BaseURL,
			DefaultModel:   defaultM,
			Models:         models,
			BinaryStatus:   binStat,
			Status:         status,
			Enabled:        cfg.Enabled,
		})
	}

	return dtos
}
