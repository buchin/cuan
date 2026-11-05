package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

func TestBackgroundTasteUsesPersistedMemberWithoutGrantingHTTPIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := &Server{taste: newTasteStore()}
	for member, content := range map[string]string{"member-a": "Use ane and ente in conversation.", "member-b": "Use formal English in correspondence."} {
		if _, err := s.createTasteItem(member, "brand", "Personal voice", content); err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range []string{"member-a", "member-b"} {
		ctx := context.WithValue(context.Background(), tasteMemberKey{}, member)
		if auth.GetOwnerFromContext(ctx) != nil {
			t.Fatal("preference selection granted an HTTP identity")
		}
		prompt := s.formatBrandTastePrompt(ctx)
		if member == "member-a" && (!strings.Contains(prompt, "ane and ente") || strings.Contains(prompt, "formal English")) {
			t.Fatal("member A preferences mixed")
		}
		if member == "member-b" && (!strings.Contains(prompt, "formal English") || strings.Contains(prompt, "ane and ente")) {
			t.Fatal("member B preferences mixed")
		}
	}
}

func TestInboxExplicitPreferenceLearningIsMemberScopedAndRespectsOptOut(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	room := &db.ProjectRoom{ID: db.GenerateUUID(), ProjectID: "default", Title: "Learning", Topic: "agent:agent_lifecycle", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	text := "Mulai sekarang selalu gunakan bahasa santai, pakai ane dan ente dalam percakapan."
	message := &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room.ID, AuthorType: "user", AuthorID: "operator_user", Content: text, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.projectStudioRepo.CreateMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	q := db.InboxRequest{OwnerID: "member-a", RoomID: room.ID, MessageID: message.ID}
	s.learnInboxPreferences(ctx, q)
	profile, _ := s.getTasteProfile("member-a")
	found := false
	for _, item := range profile.Items {
		found = found || strings.Contains(item.Content, "ane dan ente")
	}
	if !found {
		t.Fatal("explicit owner preference was not learned")
	}
	other, _ := s.getTasteProfile("member-b")
	for _, item := range other.Items {
		if strings.Contains(item.Content, "ane dan ente") {
			t.Fatal("another member inherited private preferences")
		}
	}
	before := profile.Total
	s.learnInboxPreferences(ctx, q)
	profile, _ = s.getTasteProfile("member-a")
	if profile.Total != before {
		t.Fatal("replayed request duplicated preference")
	}
	enabled := false
	if _, err := s.updateTasteConfig("member-b", &enabled, nil); err != nil {
		t.Fatal(err)
	}
	q.OwnerID = "member-b"
	s.learnInboxPreferences(ctx, q)
	other, _ = s.getTasteProfile("member-b")
	for _, item := range other.Items {
		if strings.Contains(item.Content, "ane dan ente") {
			t.Fatal("opted-out member learned preferences")
		}
	}
}

func TestInboxWorkerInjectsRequestingMemberTasteIntoProviderPrompt(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	owner, err := s.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.createTasteItem(owner.ID, "brand", "Voice", "Use ane and ente; keep replies conversational and concise."); err != nil {
		t.Fatal(err)
	}
	if _, err := s.createTasteItem("other-member", "brand", "Voice", "OTHER_MEMBER_PRIVATE_VOICE"); err != nil {
		t.Fatal(err)
	}
	w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"e5e82615-16ae-4acf-912e-d6f20effe19e","message":"Hello"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct{ Request db.InboxRequest }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	waitInboxState(t, s, result.Request.RoomID, result.Request.RequestID, "completed")
	prompt, err := os.ReadFile(filepath.Join(home, "prompts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(prompt), "Use ane and ente") || strings.Contains(string(prompt), "OTHER_MEMBER_PRIVATE_VOICE") {
		t.Fatal("background provider did not receive isolated personal taste")
	}
}

func TestTasteRoomReflectionExcludesOtherMembersVisitorsAndAI(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Shared room"}); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	for _, entry := range []struct{ owner, author, id, text string }{
		{"member-a", "user", "operator_user", "Preferensi: gunakan sapaan ane dan ente dalam percakapan"},
		{"member-b", "user", "operator_user", "Preferensi: gunakan bahasa formal Inggris untuk korespondensi"},
		{"", "user", "visitor", "Preferensi: gunakan sapaan pengunjung rahasia"},
		{"member-a", "agent", "agent_lifecycle", "Preferensi: gunakan sapaan buatan model rahasia"},
	} {
		message := &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: entry.author, AuthorID: entry.id, Content: entry.text}
		if entry.owner != "" {
			_, _, err := store.AcceptInboxRequest(ctx, db.InboxRequest{OwnerID: entry.owner, RoomID: room, RequestID: db.GenerateUUID(), MessageID: message.ID}, message)
			if err != nil {
				t.Fatal(err)
			}
		} else if err := s.projectStudioRepo.CreateMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.reflectTaste(ctx, "member-a", ReflectTasteRequest{}); err != nil {
		t.Fatal(err)
	}
	profile, _ := s.getTasteProfile("member-a")
	contents := ""
	for _, item := range profile.Items {
		contents += item.Content + "\n"
	}
	if !strings.Contains(contents, "ane dan ente") || strings.Contains(contents, "formal Inggris") || strings.Contains(contents, "rahasia") {
		t.Fatal("reflection mixed authors or failed to learn own preference")
	}
}
