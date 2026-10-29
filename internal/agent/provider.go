package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrProviderNotFound       = errors.New("agent/provider: provider not found")
	ErrInvalidCredentials     = errors.New("agent/provider: invalid or missing credentials")
	ErrConnectionVerification = errors.New("agent/provider: connection verification failed")
)

// ModelCapabilities specifies functional multimodal capabilities of an AI model.
type ModelCapabilities struct {
	Vision         bool `json:"vision"`
	SupportsVision bool `json:"supports_vision"`
}

// ModelInfo describes an AI model available through a provider.
type ModelInfo struct {
	ID                         string            `json:"id"`
	Name                       string            `json:"name"`
	DisplayName                string            `json:"display_name,omitempty"`
	Description                string            `json:"description,omitempty"`
	Provider                   string            `json:"provider,omitempty"`
	InputTokenLimit            int               `json:"input_token_limit,omitempty"`
	OutputTokenLimit           int               `json:"output_token_limit,omitempty"`
	SupportedGenerationMethods []string          `json:"supported_generation_methods,omitempty"`
	IsDefault                  bool              `json:"is_default,omitempty"`
	SupportsVision             bool              `json:"supports_vision"`
	Capabilities               ModelCapabilities `json:"capabilities"`
	Preserved                  bool              `json:"preserved,omitempty"`
}

// IsModelVisionCapable determines whether a provider model supports vision (image input).
func IsModelVisionCapable(provider, modelID string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	m := strings.ToLower(strings.TrimSpace(modelID))
	if p == "google" || p == "gemini" {
		return true
	}
	if p == "agy" || p == "antigravity" {
		if strings.Contains(m, "gpt-oss") || strings.Contains(m, "text") || strings.Contains(m, "embedding") {
			return false
		}
		return true
	}
	if p == "commandcode" {
		return IsCommandCodeModelVision(m)
	}
	if p == "claude" {
		// Claude 3+ Sonnet and Opus support vision
		if strings.Contains(m, "sonnet") || strings.Contains(m, "opus") || strings.Contains(m, "claude-3") {
			return true
		}
		return false
	}
	if p == "codex" {
		// Codex typically executes in code environment without vision
		return false
	}
	return false
}

// ConnectionProbeResult encapsulates the diagnostic results of testing provider connectivity and quotas.
type ConnectionProbeResult struct {
	Valid           bool        `json:"valid"`
	Provider        string      `json:"provider"`
	PlanHint        string      `json:"plan_hint,omitempty"`
	LatencyMs       int64       `json:"latency_ms"`
	AvailableModels []ModelInfo `json:"available_models"`
	ErrorMessage    string      `json:"error_message,omitempty"`
	TestedAt        time.Time   `json:"tested_at"`
}

// ContentGenerator defines an interface for generating text content with LLM providers.
type ContentGenerator interface {
	GenerateContent(ctx context.Context, apiKey string, modelID string, systemInstruction string, userPrompt string) (string, error)
}

// Provider represents an LLM API provider (Google Gemini, OpenAI, Mock, etc.).
type Provider interface {
	Name() string
	Type() string
	VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error)
	ListModels(ctx context.Context, credentials string) ([]ModelInfo, error)
}

// ProviderRegistry manages registered LLM providers.
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Register registers a provider implementation under a given key.
func (r *ProviderRegistry) Register(providerType string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[strings.ToLower(strings.TrimSpace(providerType))] = p
}

// Get retrieves a provider by type.
func (r *ProviderRegistry) Get(providerType string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[strings.ToLower(strings.TrimSpace(providerType))]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, providerType)
	}
	return p, nil
}

var (
	defaultRegistryInstance *ProviderRegistry
	defaultRegistryOnce     sync.Once
)

// DefaultRegistry returns the singleton global provider registry populated with default providers.
func DefaultRegistry() *ProviderRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistryInstance = NewProviderRegistry()
		defaultRegistryInstance.Register("google", NewGeminiProvider())
		defaultRegistryInstance.Register("gemini", NewGeminiProvider())
		defaultRegistryInstance.Register("commandcode", NewCommandCodeProvider())
		defaultRegistryInstance.Register("agy", NewAGYProvider())
		defaultRegistryInstance.Register("antigravity", NewAGYProvider())
		defaultRegistryInstance.Register("mock", NewMockProvider())
	})
	return defaultRegistryInstance
}

// MockProvider provides a fully controllable mock provider for unit tests and offline environments.
type MockProvider struct {
	NameVal         string
	TypeVal         string
	Models          []ModelInfo
	FailNext        bool
	FailureReason   string
	SimulatedDelay  time.Duration
	PlanHintVal     string
	mu              sync.Mutex
}

// NewMockProvider creates a new mock provider populated with standard Gemini models.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		NameVal:     "Mock Provider",
		TypeVal:     "mock",
		PlanHintVal: "Mock / Developer Tier",
		Models: []ModelInfo{
			{
				ID:               "gemini-1.5-pro",
				Name:             "models/gemini-1.5-pro",
				DisplayName:      "Gemini 1.5 Pro (Mock)",
				Description:      "Flagship multimodal reasoning model",
				Provider:         "mock",
				InputTokenLimit:  2097152,
				OutputTokenLimit: 8192,
				IsDefault:        true,
				SupportsVision:   true,
				Capabilities: ModelCapabilities{
					Vision:         true,
					SupportsVision: true,
				},
			},
			{
				ID:               "gemini-1.5-flash",
				Name:             "models/gemini-1.5-flash",
				DisplayName:      "Gemini 1.5 Flash (Mock)",
				Description:      "Fast and versatile performance across a diverse variety of tasks",
				Provider:         "mock",
				InputTokenLimit:  1048576,
				OutputTokenLimit: 8192,
				IsDefault:        false,
				SupportsVision:   true,
				Capabilities: ModelCapabilities{
					Vision:         true,
					SupportsVision: true,
				},
			},
			{
				ID:               "gemini-2.0-flash",
				Name:             "models/gemini-2.0-flash",
				DisplayName:      "Gemini 2.0 Flash (Mock)",
				Description:      "Next-generation ultra-fast multimodal model",
				Provider:         "mock",
				InputTokenLimit:  1048576,
				OutputTokenLimit: 8192,
				IsDefault:        false,
				SupportsVision:   true,
				Capabilities: ModelCapabilities{
					Vision:         true,
					SupportsVision: true,
				},
			},
		},
	}
}

func (m *MockProvider) Name() string {
	if m.NameVal == "" {
		return "Mock Provider"
	}
	return m.NameVal
}

func (m *MockProvider) Type() string {
	if m.TypeVal == "" {
		return "mock"
	}
	return m.TypeVal
}

func (m *MockProvider) SetFail(fail bool, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailNext = fail
	m.FailureReason = reason
}

func (m *MockProvider) VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.SimulatedDelay > 0 {
		select {
		case <-time.After(m.SimulatedDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	trimmed := strings.TrimSpace(credentials)
	if trimmed == "" {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     m.Type(),
			ErrorMessage: "missing or empty API key",
			TestedAt:     time.Now().UTC(),
		}, ErrInvalidCredentials
	}

	if m.FailNext {
		msg := m.FailureReason
		if msg == "" {
			msg = "simulated provider verification failure"
		}
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     m.Type(),
			ErrorMessage: msg,
			TestedAt:     time.Now().UTC(),
		}, errors.New(msg)
	}

	// If credentials indicate invalid key in test scenarios
	if strings.HasPrefix(trimmed, "invalid") || strings.HasPrefix(trimmed, "expired") {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     m.Type(),
			ErrorMessage: "invalid or expired API key",
			TestedAt:     time.Now().UTC(),
		}, ErrConnectionVerification
	}

	return &ConnectionProbeResult{
		Valid:           true,
		Provider:        m.Type(),
		PlanHint:        m.PlanHintVal,
		LatencyMs:       15,
		AvailableModels: m.Models,
		TestedAt:        time.Now().UTC(),
	}, nil
}

func (m *MockProvider) ListModels(ctx context.Context, credentials string) ([]ModelInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.FailNext {
		return nil, errors.New(m.FailureReason)
	}
	return m.Models, nil
}

func (m *MockProvider) GenerateContent(ctx context.Context, apiKey string, modelID string, systemInstruction string, userPrompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.FailNext {
		msg := m.FailureReason
		if msg == "" {
			msg = "simulated provider generation failure"
		}
		return "", errors.New(msg)
	}

	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		return "", ErrInvalidCredentials
	}

	if isInvalidKey(trimmed) {
		return "", ErrConnectionVerification
	}

	return generateOfflineMockContent(modelID, systemInstruction, userPrompt), nil
}

