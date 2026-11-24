package server

import (
	"context"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestParseInboxBrandMemory(t *testing.T) {
	// 1. Valid list
	validList := "Pedoman brand saat ini:\n```cuan-brand-memory\n{\"action\":\"list\"}\n```"
	clean, act, err := parseInboxBrandMemory(validList)
	if err != nil || act == nil {
		t.Fatalf("expected valid brand memory parse, got error: %v", err)
	}
	if clean != "Pedoman brand saat ini:" {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if act.Action != "list" {
		t.Fatalf("unexpected action: %s", act.Action)
	}

	// 2. Valid add
	validAdd := "Simpan gaya bahasa:\n```cuan-brand-memory\n{\"action\":\"add\",\"category\":\"brand\",\"topic\":\"Tone of Voice\",\"content\":\"Gunakan bahasa yang santun dan profesional.\"}\n```"
	clean, act, err = parseInboxBrandMemory(validAdd)
	if err != nil || act == nil {
		t.Fatalf("expected valid add parse, got: %v", err)
	}
	if act.Action != "add" || act.Category != "brand" || act.Topic != "Tone of Voice" || act.Content != "Gunakan bahasa yang santun dan profesional." {
		t.Fatalf("unexpected add action: %+v", act)
	}

	// 3. Invalid: missing content on add
	badAdd := "```cuan-brand-memory\n{\"action\":\"add\",\"category\":\"brand\",\"content\":\"\"}\n```"
	if _, _, err := parseInboxBrandMemory(badAdd); err == nil {
		t.Fatal("expected error for add without content")
	}

	// 4. Invalid: missing ID on update
	badUpdate := "```cuan-brand-memory\n{\"action\":\"update\",\"content\":\"Baru\"}\n```"
	if _, _, err := parseInboxBrandMemory(badUpdate); err == nil {
		t.Fatal("expected error for update without ID")
	}

	// 5. Invalid: missing ID on delete
	badDelete := "```cuan-brand-memory\n{\"action\":\"delete\"}\n```"
	if _, _, err := parseInboxBrandMemory(badDelete); err == nil {
		t.Fatal("expected error for delete without ID")
	}
}

func TestApplyInboxBrandMemory(t *testing.T) {
	srv := &Server{
		taste: newTasteStore(),
	}

	ctx := context.Background()
	owner := "usr_brand_test"
	q := db.InboxRequest{
		RoomID:    "room_test",
		OwnerID:   owner,
		MessageID: "msg_test",
	}

	// 1. List brand memory (returns default template items)
	listReq := "Daftar pedoman:\n```cuan-brand-memory\n{\"action\":\"list\"}\n```"
	listResp := srv.applyInboxBrandMemory(ctx, q, listReq)
	if !strings.Contains(listResp, "Brand Guidelines & Tone of Voice") || !strings.Contains(listResp, "Standar Teknis") {
		t.Fatalf("expected brand and technical headers, got: %s", listResp)
	}

	// 2. Add brand memory item
	addReq := "Tambahkan pedoman:\n```cuan-brand-memory\n{\"action\":\"add\",\"category\":\"brand\",\"topic\":\"Gaya Sapaan\",\"content\":\"Selalu menyapa pengguna dengan Halo Rekan Cuan.\"}\n```"
	addResp := srv.applyInboxBrandMemory(ctx, q, addReq)
	if !strings.Contains(addResp, "Gaya Sapaan") || !strings.Contains(addResp, "Halo Rekan Cuan") {
		t.Fatalf("expected add confirmation, got: %s", addResp)
	}

	// Verify it shows up in profile
	profile, err := srv.getTasteProfile(owner)
	if err != nil {
		t.Fatalf("failed to get profile: %v", err)
	}
	var foundID string
	for _, it := range profile.BrandItems {
		if it.Topic == "Gaya Sapaan" {
			foundID = it.ID
			break
		}
	}
	if foundID == "" {
		t.Fatal("new brand item not found in profile")
	}

	// 3. Update the brand item
	updReq := "Perbarui pedoman:\n```cuan-brand-memory\n{\"action\":\"update\",\"id\":\"" + foundID + "\",\"content\":\"Selalu menyapa dengan Halo Sobat Cuan.\"}\n```"
	updResp := srv.applyInboxBrandMemory(ctx, q, updReq)
	if !strings.Contains(updResp, "Halo Sobat Cuan") {
		t.Fatalf("expected updated content, got: %s", updResp)
	}

	// 4. Delete the brand item
	delReq := "Hapus pedoman:\n```cuan-brand-memory\n{\"action\":\"delete\",\"id\":\"" + foundID + "\"}\n```"
	delResp := srv.applyInboxBrandMemory(ctx, q, delReq)
	if !strings.Contains(delResp, "berhasil dihapus") {
		t.Fatalf("expected delete confirmation, got: %s", delResp)
	}

	// 5. Reset profile
	resetReq := "Reset ke default:\n```cuan-brand-memory\n{\"action\":\"reset\"}\n```"
	resetResp := srv.applyInboxBrandMemory(ctx, q, resetReq)
	if !strings.Contains(resetResp, "direset ke standar bawaan") {
		t.Fatalf("expected reset confirmation, got: %s", resetResp)
	}
}
