package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DefaultCatalogCacheTTL defines the standard TTL for model discovery caching (7 minutes, within 5-10m specification).
const DefaultCatalogCacheTTL = 7 * time.Minute

type catalogCacheEntry struct {
	cachedAt  time.Time
	expiresAt time.Time
	models    []ModelInfo
}

// ModelCatalogCache is a thread-safe in-memory cache for provider model catalogs.
type ModelCatalogCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]catalogCacheEntry
}

// NewModelCatalogCache creates a new catalog cache instance with the specified TTL.
func NewModelCatalogCache(ttl time.Duration) *ModelCatalogCache {
	if ttl <= 0 {
		ttl = DefaultCatalogCacheTTL
	}
	return &ModelCatalogCache{
		ttl:     ttl,
		entries: make(map[string]catalogCacheEntry),
	}
}

var (
	defaultCatalogCacheInstance *ModelCatalogCache
	defaultCatalogCacheOnce     sync.Once
)

// DefaultCatalogCache returns the singleton global catalog cache.
func DefaultCatalogCache() *ModelCatalogCache {
	defaultCatalogCacheOnce.Do(func() {
		defaultCatalogCacheInstance = NewModelCatalogCache(DefaultCatalogCacheTTL)
	})
	return defaultCatalogCacheInstance
}

// TTL returns the configured cache TTL.
func (c *ModelCatalogCache) TTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ttl
}

// SetTTL updates the cache TTL.
func (c *ModelCatalogCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl > 0 {
		c.ttl = ttl
	}
}

// Get retrieves cached models for a normalized provider if the cache entry has not expired.
func (c *ModelCatalogCache) Get(provider string) ([]ModelInfo, bool) {
	key := NormalizeProviderKey(provider)
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}

	result := make([]ModelInfo, len(entry.models))
	copy(result, entry.models)
	return result, true
}

// GetEntry retrieves detailed cache status for testing and diagnostics.
func (c *ModelCatalogCache) GetEntry(provider string) (models []ModelInfo, cachedAt, expiresAt time.Time, valid bool) {
	key := NormalizeProviderKey(provider)
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, time.Time{}, time.Time{}, false
	}
	valid = time.Now().Before(entry.expiresAt)
	result := make([]ModelInfo, len(entry.models))
	copy(result, entry.models)
	return result, entry.cachedAt, entry.expiresAt, valid
}

// Set saves models for a provider with the configured TTL.
func (c *ModelCatalogCache) Set(provider string, models []ModelInfo) {
	key := NormalizeProviderKey(provider)
	now := time.Now()
	cloned := make([]ModelInfo, len(models))
	copy(cloned, models)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = catalogCacheEntry{
		cachedAt:  now,
		expiresAt: now.Add(c.ttl),
		models:    cloned,
	}
}

// Invalidate removes cached entries for a provider to force a fresh probe.
func (c *ModelCatalogCache) Invalidate(provider string) {
	key := NormalizeProviderKey(provider)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all cached entries.
func (c *ModelCatalogCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]catalogCacheEntry)
}

// NormalizeProviderKey normalizes provider names and aliases.
func NormalizeProviderKey(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "antigravity":
		return "agy"
	case "gemini":
		return "google"
	default:
		return p
	}
}

// NormalizeModelInfo ensures all metadata fields on a ModelInfo are populated cleanly.
func NormalizeModelInfo(m *ModelInfo, defaultProvider string) {
	if m.Provider == "" {
		m.Provider = defaultProvider
	}
	if m.ID == "" {
		m.ID = m.Name
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if m.DisplayName == "" {
		m.DisplayName = m.Name
	}
	// Check vision capability
	if !m.SupportsVision {
		m.SupportsVision = IsModelVisionCapable(m.Provider, m.ID)
	}
	m.Capabilities = ModelCapabilities{
		Vision:         m.SupportsVision,
		SupportsVision: m.SupportsVision,
	}
}

// PreserveAgentModels guarantees 100% backward compatibility by preserving
// an agent's configured model_id and fallback_model_id in the supported_models list.
func PreserveAgentModels(models []ModelInfo, provider, modelID, fallbackModelID string) []ModelInfo {
	hasModel := func(id string) bool {
		for _, m := range models {
			if m.ID == id {
				return true
			}
		}
		return false
	}

	result := make([]ModelInfo, len(models))
	copy(result, models)

	trimmedModel := strings.TrimSpace(modelID)
	if trimmedModel != "" && !hasModel(trimmedModel) {
		vision := IsModelVisionCapable(provider, trimmedModel)
		preserved := ModelInfo{
			ID:             trimmedModel,
			Name:           trimmedModel,
			DisplayName:    fmt.Sprintf("%s (Tersimpan)", trimmedModel),
			Description:    "Model tersimpan pada konfigurasi agen (tidak ada dalam katalog aktif)",
			Provider:       provider,
			SupportsVision: vision,
			Capabilities: ModelCapabilities{
				Vision:         vision,
				SupportsVision: vision,
			},
			Preserved: true,
		}
		result = append([]ModelInfo{preserved}, result...)
	}

	trimmedFallback := strings.TrimSpace(fallbackModelID)
	if trimmedFallback != "" && trimmedFallback != trimmedModel && !hasModel(trimmedFallback) {
		vision := IsModelVisionCapable(provider, trimmedFallback)
		preservedFallback := ModelInfo{
			ID:             trimmedFallback,
			Name:           trimmedFallback,
			DisplayName:    fmt.Sprintf("%s (Tersimpan / Cadangan)", trimmedFallback),
			Description:    "Model cadangan tersimpan pada konfigurasi agen (tidak ada dalam katalog aktif)",
			Provider:       provider,
			SupportsVision: vision,
			Capabilities: ModelCapabilities{
				Vision:         vision,
				SupportsVision: vision,
			},
			Preserved: true,
		}
		result = append(result, preservedFallback)
	}

	return result
}

// DiscoverModels fetches models with thread-safe caching, forced refresh bypass,
// and resilient fallbacks.
func (c *ModelCatalogCache) DiscoverModels(ctx context.Context, reg *ProviderRegistry, provider string, refresh bool) ([]ModelInfo, error) {
	normKey := NormalizeProviderKey(provider)
	if normKey == "" {
		normKey = "commandcode"
	}

	// 1. Check cache if not forcing refresh
	if !refresh {
		if cached, ok := c.Get(normKey); ok && len(cached) > 0 {
			return cached, nil
		}
	} else {
		c.Invalidate(normKey)
	}

	// 2. Attempt active probing from provider registry
	var probedModels []ModelInfo
	var probeErr error

	if reg != nil {
		prov, err := reg.Get(normKey)
		if err != nil && normKey != provider {
			prov, err = reg.Get(provider)
		}
		if err == nil && prov != nil {
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			probedModels, probeErr = prov.ListModels(probeCtx, "")
			cancel()
		} else {
			probeErr = err
		}
	}

	// 3. If probe succeeded with valid models, normalize, cache and return
	if probeErr == nil && len(probedModels) > 0 {
		for i := range probedModels {
			NormalizeModelInfo(&probedModels[i], normKey)
		}
		c.Set(normKey, probedModels)
		return probedModels, nil
	}

	// 4. Resilient Fallback A: Stale cache (even if expired)
	c.mu.RLock()
	staleEntry, hasStale := c.entries[normKey]
	c.mu.RUnlock()
	if hasStale && len(staleEntry.models) > 0 {
		staleCopy := make([]ModelInfo, len(staleEntry.models))
		copy(staleCopy, staleEntry.models)
		return staleCopy, nil
	}

	// 5. Resilient Fallback B: Built-in default catalog per provider
	var fallbackModels []ModelInfo
	switch normKey {
	case "agy":
		fallbackModels = DefaultAGYModels()
	case "google":
		fallbackModels = DefaultGeminiModels()
	case "commandcode":
		fallbackModels = DefaultCommandCodeModels()
	case "mock":
		fallbackModels = NewMockProvider().Models
	default:
		if probeErr != nil {
			return nil, probeErr
		}
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, provider)
	}

	for i := range fallbackModels {
		NormalizeModelInfo(&fallbackModels[i], normKey)
	}
	c.Set(normKey, fallbackModels)
	return fallbackModels, nil
}
