package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAIProviderStore_SaveAndGet(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-ai-store-test-*")
	if err != nil {
		t.Fatalf("mkdir temp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "ai-providers.json")
	store := NewAIProviderStore(filePath)

	// Encrypt a test key
	encKey, err := EncryptCredential("sk-test-deepseek-1234567890abcdef")
	if err != nil {
		t.Fatalf("EncryptCredential failed: %v", err)
	}

	err = store.SaveConfig(AIProviderConfig{
		ID:              "deepseek",
		Mode:            "byok",
		EncryptedAPIKey: encKey,
		DefaultModel:    "deepseek-chat",
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify persistence: create new store instance pointing to same file
	store2 := NewAIProviderStore(filePath)
	cfg := store2.GetConfig("deepseek")
	if cfg.ID != "deepseek" || cfg.Mode != "byok" || cfg.DefaultModel != "deepseek-chat" {
		t.Errorf("unexpected config: %+v", cfg)
	}

	decrypted, err := store2.GetDecryptedAPIKey("deepseek")
	if err != nil {
		t.Fatalf("GetDecryptedAPIKey failed: %v", err)
	}
	if decrypted != "sk-test-deepseek-1234567890abcdef" {
		t.Errorf("decrypted key mismatch, got %q", decrypted)
	}

	// Export catalog DTO
	dtos := store2.ExportCatalogDTO(nil, nil)
	if len(dtos) == 0 {
		t.Fatal("expected non-empty catalog DTOs")
	}

	var dsDTO *AIProviderDTO
	for i := range dtos {
		if dtos[i].ID == "deepseek" {
			dsDTO = &dtos[i]
			break
		}
	}
	if dsDTO == nil {
		t.Fatal("deepseek DTO not found in catalog")
	}
	if !dsDTO.HasAPIKey {
		t.Errorf("expected HasAPIKey to be true")
	}
	if dsDTO.APIKeyMasked != "sk-...cdef" {
		t.Errorf("expected masked key 'sk-...cdef', got %q", dsDTO.APIKeyMasked)
	}
	if dsDTO.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", dsDTO.Status)
	}
}
