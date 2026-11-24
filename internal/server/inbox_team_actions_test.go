package server

import (
	"context"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

func TestParseInboxTeam(t *testing.T) {
	// 1. Valid list
	validList := "Berikut daftar tim:\n```cuan-team\n{\"action\":\"list\"}\n```"
	clean, act, err := parseInboxTeam(validList)
	if err != nil || act == nil {
		t.Fatalf("expected valid team parse, got error: %v", err)
	}
	if clean != "Berikut daftar tim:" {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if act.Action != "list" {
		t.Fatalf("unexpected action: %s", act.Action)
	}

	// 2. Valid invite
	validInvite := "Undangan anggota tim:\n```cuan-team\n{\"action\":\"invite\",\"email\":\"alice@example.com\",\"role\":\"member\"}\n```"
	clean, act, err = parseInboxTeam(validInvite)
	if err != nil || act == nil {
		t.Fatalf("expected valid invite parse, got: %v", err)
	}
	if act.Action != "invite" || act.Email != "alice@example.com" || act.Role != "member" {
		t.Fatalf("unexpected invite action: %+v", act)
	}

	// 3. Invalid: invite without email
	badInvite := "```cuan-team\n{\"action\":\"invite\",\"email\":\"\"}\n```"
	if _, _, err := parseInboxTeam(badInvite); err == nil {
		t.Fatal("expected error for invite without email")
	}

	// 4. Invalid JSON
	badJSON := "```cuan-team\n{invalid json}\n```"
	if _, _, err := parseInboxTeam(badJSON); err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestApplyInboxTeam(t *testing.T) {
	memStore := auth.NewMemoryStore()
	authSvc := auth.NewService(memStore)
	ctx := context.Background()

	setupResp, err := authSvc.Setup(ctx, "admin@cuan.local", "StrongPass123!")
	if err != nil {
		t.Fatalf("failed to setup auth: %v", err)
	}

	srv := &Server{
		authSvc: authSvc,
	}

	q := db.InboxRequest{
		RoomID:    "room_test",
		OwnerID:   setupResp.Owner.ID,
		MessageID: "msg_test",
	}

	// 1. List team members
	listReq := "Tampilkan tim:\n```cuan-team\n{\"action\":\"list\"}\n```"
	listResp := srv.applyInboxTeam(ctx, q, listReq)
	if !strings.Contains(listResp, "Anggota Tim Terdaftar") || !strings.Contains(listResp, "admin@cuan.local") {
		t.Fatalf("expected team list to contain admin email, got: %s", listResp)
	}

	// 2. Invite new member
	inviteReq := "Undang staf baru:\n```cuan-team\n{\"action\":\"invite\",\"email\":\"staff@cuan.local\",\"role\":\"member\"}\n```"
	inviteResp := srv.applyInboxTeam(ctx, q, inviteReq)
	if !strings.Contains(inviteResp, "staff@cuan.local") || !strings.Contains(inviteResp, "Token Undangan") {
		t.Fatalf("expected invite confirmation, got: %s", inviteResp)
	}

	// 3. List again - should show invite pending
	listAgain := srv.applyInboxTeam(ctx, q, listReq)
	if !strings.Contains(listAgain, "Undangan Menunggu Konfirmasi") || !strings.Contains(listAgain, "staff@cuan.local") {
		t.Fatalf("expected pending invite in list, got: %s", listAgain)
	}
}
