package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestModelCatalogCache_TTL_And_Expiration(t *testing.T) {
	cache := NewModelCatalogCache(80 * time.Millisecond)

	models := []ModelInfo{
		{
			ID:          "test-model-1",
			Name:        "test-model-1",
			DisplayName: "Test Model 1",
			Provider:    "google",
		},
	}

	cache.Set("google", models)

	// Immediately read: should hit cache
	cached, ok := cache.Get("google")
	if !ok {
		t.Fatal("expected cache hit immediately after Set")
	}
	if len(cached) != 1 || cached[0].ID != "test-model-1" {
		t.Fatalf("unexpected cached content: %+v", cached)
	}

	// Verify alias normalization (gemini -> google)
	cachedAlias, okAlias := cache.Get("gemini")
	if !okAlias || len(cachedAlias) != 1 {
		t.Fatalf("expected alias 'gemini' to hit 'google' cache, got ok=%v", okAlias)
	}

	// Wait for TTL expiration
	time.Sleep(100 * time.Millisecond)

	_, okAfter := cache.Get("google")
	if okAfter {
		t.Fatal("expected cache miss after TTL expiration")
	}
}

func TestModelCatalogCache_ForcedRefresh(t *testing.T) {
	cache := NewModelCatalogCache(5 * time.Minute)
	reg := NewProviderRegistry()

	mock := NewMockProvider()
	reg.Register("mock", mock)

	ctx := context.Background()

	// Initial fetch: populates cache
	initialModels, err := cache.DiscoverModels(ctx, reg, "mock", false)
	if err != nil {
		t.Fatalf("initial DiscoverModels failed: %v", err)
	}
	if len(initialModels) != 3 {
		t.Fatalf("expected 3 models from mock, got %d", len(initialModels))
	}

	// Modify mock provider models
	mock.mu.Lock()
	mock.Models = []ModelInfo{
		{
			ID:             "new-mock-v2",
			Name:           "new-mock-v2",
			DisplayName:    "New Mock V2",
			Provider:       "mock",
			SupportsVision: true,
		},
	}
	mock.mu.Unlock()

	// Without refresh: should return stale cache (3 models)
	cachedModels, err := cache.DiscoverModels(ctx, reg, "mock", false)
	if err != nil {
		t.Fatalf("cached DiscoverModels failed: %v", err)
	}
	if len(cachedModels) != 3 {
		t.Fatalf("expected cached models count 3, got %d", len(cachedModels))
	}

	// With refresh=true: should force fresh probe and return new-mock-v2 (1 model)
	refreshedModels, err := cache.DiscoverModels(ctx, reg, "mock", true)
	if err != nil {
		t.Fatalf("refreshed DiscoverModels failed: %v", err)
	}
	if len(refreshedModels) != 1 || refreshedModels[0].ID != "new-mock-v2" {
		t.Fatalf("expected refreshed model new-mock-v2, got: %+v", refreshedModels)
	}
	if !refreshedModels[0].Capabilities.Vision {
		t.Fatal("expected refreshed model to have vision capability enabled")
	}
}

func TestModelCatalogCache_ThreadSafety(t *testing.T) {
	cache := NewModelCatalogCache(50 * time.Millisecond)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			prov := fmt.Sprintf("provider-%d", id%5)
			models := []ModelInfo{
				{
					ID:       fmt.Sprintf("model-%d", id),
					Name:     fmt.Sprintf("model-%d", id),
					Provider: prov,
				},
			}
			cache.Set(prov, models)
			_, _ = cache.Get(prov)
			if id%3 == 0 {
				cache.Invalidate(prov)
			}
			if id%10 == 0 {
				cache.Clear()
			}
		}(i)
	}

	wg.Wait()
}

func TestModelCatalogCache_ResilientFallback(t *testing.T) {
	cache := NewModelCatalogCache(10 * time.Millisecond)
	reg := NewProviderRegistry()

	mock := NewMockProvider()
	reg.Register("mock", mock)

	ctx := context.Background()

	// Initial probe to populate cache
	models, err := cache.DiscoverModels(ctx, reg, "mock", false)
	if err != nil || len(models) == 0 {
		t.Fatalf("setup probe failed: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Simulate provider failure on next probe
	mock.SetFail(true, "upstream rate limit exceeded (HTTP 429)")

	// DiscoverModels should fallback to stale cache gracefully without error
	fallbackModels, err := cache.DiscoverModels(ctx, reg, "mock", false)
	if err != nil {
		t.Fatalf("expected resilient fallback, got error: %v", err)
	}
	if len(fallbackModels) == 0 {
		t.Fatal("expected non-empty fallback models from stale cache")
	}

	// Unknown provider with no cache should cleanly return error
	_, errUnknown := cache.DiscoverModels(ctx, reg, "unknown_custom_provider", false)
	if errUnknown == nil {
		t.Fatal("expected error for completely unknown provider with no fallback")
	}
}

func TestAGYProvider_ParsingAndDefaults(t *testing.T) {
	sampleOutput := `Fetching available models...
gemini-3.8-flash-high	Gemini 3.8 Flash (High)
gemini-3.8-flash-medium	Gemini 3.8 Flash (Medium)
claude-sonnet-4-6	Claude Sonnet 4.6 (Thinking)
gpt-oss-120b-medium	GPT-OSS 120B (Medium)
`
	parsed := ParseAGYModelsOutput(sampleOutput)
	if len(parsed) != 4 {
		t.Fatalf("expected 4 parsed models, got %d", len(parsed))
	}

	m0 := parsed[0]
	if m0.ID != "gemini-3.8-flash-high" {
		t.Errorf("expected ID gemini-3.8-flash-high, got %s", m0.ID)
	}
	if m0.DisplayName != "Gemini 3.8 Flash (High)" {
		t.Errorf("expected DisplayName Gemini 3.8 Flash (High), got %s", m0.DisplayName)
	}
	if !m0.SupportsVision || !m0.Capabilities.Vision {
		t.Errorf("expected vision support for gemini-3.8-flash-high")
	}
	if !m0.IsDefault {
		t.Errorf("expected gemini-3.8-flash-high to be default")
	}

	// Check text-only model
	m3 := parsed[3]
	if m3.ID != "gpt-oss-120b-medium" {
		t.Errorf("expected ID gpt-oss-120b-medium, got %s", m3.ID)
	}
	if m3.SupportsVision || m3.Capabilities.Vision {
		t.Errorf("expected gpt-oss-120b-medium to be text-only (non-vision)")
	}

	// Test DefaultAGYModels
	defaults := DefaultAGYModels()
	if len(defaults) < 5 {
		t.Fatalf("expected at least 5 default AGY models, got %d", len(defaults))
	}
}

func TestCommandCode_ParsingAndDefaults(t *testing.T) {
	sampleOutput := `Available models  ·  94 models

Open Source

deepseek/deepseek-v4-pro               hybrid-attention long-context reasoning
deepseek/deepseek-v4-flash-vision-exp  fast hybrid-attention reasoning with vision
moonshotai/kimi-k2.7-code              improved long-horizon coding with vision

Anthropic

claude-sonnet-5-5                      best combo of speed & intelligence (recommended)
`
	parsed := ParseCommandCodeModelsOutput(sampleOutput)
	if len(parsed) < 4 {
		t.Fatalf("expected at least 4 parsed models, got %d", len(parsed))
	}

	// Ensure commandcode-default was injected
	hasDefault := false
	for _, m := range parsed {
		if m.ID == "commandcode-default" && m.IsDefault {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Fatal("expected commandcode-default to be included as default")
	}

	defaults := DefaultCommandCodeModels()
	if len(defaults) < 10 {
		t.Fatalf("expected comprehensive default commandcode models, got %d", len(defaults))
	}
}

func TestPreserveAgentModels(t *testing.T) {
	initial := []ModelInfo{
		{
			ID:          "gemini-1.5-pro",
			Name:        "gemini-1.5-pro",
			DisplayName: "Gemini 1.5 Pro",
			Provider:    "google",
		},
		{
			ID:          "gemini-1.5-flash",
			Name:        "gemini-1.5-flash",
			DisplayName: "Gemini 1.5 Flash",
			Provider:    "google",
		},
	}

	// Case 1: Model already in catalog: no extra preserved model
	preserved1 := PreserveAgentModels(initial, "google", "gemini-1.5-pro", "gemini-1.5-flash")
	if len(preserved1) != 2 {
		t.Fatalf("expected unchanged length 2, got %d", len(preserved1))
	}

	// Case 2: Deprecated / custom model_id and fallback_model_id not in catalog
	preserved2 := PreserveAgentModels(initial, "google", "custom-legacy-model-v0", "custom-fallback-v0")
	if len(preserved2) != 4 {
		t.Fatalf("expected length 4 after preserving both models, got %d", len(preserved2))
	}

	// First should be the preserved primary model
	pModel := preserved2[0]
	if pModel.ID != "custom-legacy-model-v0" || !pModel.Preserved {
		t.Fatalf("expected preserved primary model at index 0, got: %+v", pModel)
	}
	if !pModel.Capabilities.Vision {
		t.Fatal("expected google custom model to default to vision true")
	}

	// Last should be the preserved fallback model
	pFallback := preserved2[3]
	if pFallback.ID != "custom-fallback-v0" || !pFallback.Preserved {
		t.Fatalf("expected preserved fallback model at last index, got: %+v", pFallback)
	}
}
