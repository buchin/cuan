package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func TestParseInboxBrain(t *testing.T) {
	// 1. Valid list
	validList := "Berikut penyedia model:\n```cuan-brain\n{\"action\":\"list\"}\n```"
	clean, act, err := parseInboxBrain(validList)
	if err != nil || act == nil {
		t.Fatalf("expected valid brain parse, got error: %v", err)
	}
	if clean != "Berikut penyedia model:" {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if act.Action != "list" {
		t.Fatalf("unexpected action: %s", act.Action)
	}

	// 2. Valid set_model
	validSet := "Set model default:\n```cuan-brain\n{\"action\":\"set_model\",\"provider\":\"openai\",\"model\":\"gpt-4o\"}\n```"
	clean, act, err = parseInboxBrain(validSet)
	if err != nil || act == nil {
		t.Fatalf("expected valid set_model parse, got: %v", err)
	}
	if act.Action != "set_model" || act.Provider != "openai" || act.Model != "gpt-4o" {
		t.Fatalf("unexpected set_model action: %+v", act)
	}

	// 3. Invalid: missing model on set_model
	badSet := "```cuan-brain\n{\"action\":\"set_model\",\"provider\":\"openai\",\"model\":\"\"}\n```"
	if _, _, err := parseInboxBrain(badSet); err == nil {
		t.Fatal("expected error for set_model without model")
	}

	// 4. Invalid: missing provider on configure
	badCfg := "```cuan-brain\n{\"action\":\"configure\",\"provider\":\"\"}\n```"
	if _, _, err := parseInboxBrain(badCfg); err == nil {
		t.Fatal("expected error for configure without provider")
	}
}

func TestApplyInboxBrain(t *testing.T) {
	tmpDir := t.TempDir()
	storeFile := filepath.Join(tmpDir, "ai-providers.json")
	store := agent.NewAIProviderStore(storeFile)

	srv := &Server{
		aiProviderStore: store,
	}

	ctx := context.Background()
	q := db.InboxRequest{
		RoomID:    "room_test",
		OwnerID:   "usr_test",
		MessageID: "msg_test",
	}

	// 1. List catalog
	listReq := "Daftar AI:\n```cuan-brain\n{\"action\":\"list\"}\n```"
	listResp := srv.applyInboxBrain(ctx, q, listReq)
	if !strings.Contains(listResp, "AI Brains & Model Providers") {
		t.Fatalf("expected AI catalog header, got: %s", listResp)
	}

	// 2. Set default model
	setReq := "Atur model:\n```cuan-brain\n{\"action\":\"set_model\",\"provider\":\"openai\",\"model\":\"gpt-4o-mini\"}\n```"
	setResp := srv.applyInboxBrain(ctx, q, setReq)
	if !strings.Contains(setResp, "gpt-4o-mini") || !strings.Contains(setResp, "openai") {
		t.Fatalf("expected model set confirmation, got: %s", setResp)
	}

	// Verify persistence in store
	cfg := store.GetConfig("openai")
	if cfg.DefaultModel != "gpt-4o-mini" {
		t.Fatalf("expected persisted model 'gpt-4o-mini', got: %s", cfg.DefaultModel)
	}

	// 3. Configure BYOK provider
	cfgReq := "Simpan kunci:\n```cuan-brain\n{\"action\":\"configure\",\"provider\":\"deepseek\",\"mode\":\"byok\",\"api_key\":\"sk-test-deepseek-key-12345\"}\n```"
	cfgResp := srv.applyInboxBrain(ctx, q, cfgReq)
	if !strings.Contains(cfgResp, "deepseek") || !strings.Contains(cfgResp, "berhasil diperbarui") {
		t.Fatalf("expected configure confirmation, got: %s", cfgResp)
	}

	// Verify decrypted key
	decrypted, err := store.GetDecryptedAPIKey("deepseek")
	if err != nil || decrypted != "sk-test-deepseek-key-12345" {
		t.Fatalf("expected decrypted key 'sk-test-deepseek-key-12345', got: %s (err: %v)", decrypted, err)
	}
}
