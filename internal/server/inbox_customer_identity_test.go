package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestInboxUsesBoundCustomerIdentityWithoutRepeatingEmailQuestion(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	contact, err := s.crmUpsert(ctx, "identity-guest", "Dojo", "Customer", "customer@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.crmUpsert(ctx, "other-identity", "Dojo", "Other", "other@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	room := &db.ProjectRoom{ID: contact.SessionUUID, ProjectID: "default", Title: contact.Name, Topic: contact.Product, CreatedAt: now, UpdatedAt: now}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	q := db.InboxRequest{RoomID: room.ID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: "owner", AgentID: "agent", State: "queued", CreatedAt: now, Deadline: now.Add(time.Minute)}
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: room.ID, AuthorType: "user", AuthorID: "operator_user", Content: "@cuan aktifkan akses untuk user ini di dojo", CreatedAt: now, UpdatedAt: now}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}
	prompt, err := s.inboxContext(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{contact.ID, contact.Email, "same contact shown in the Inbox header", "does not prove ownership", "Dojo"} {
		if !strings.Contains(prompt, value) {
			t.Fatal("bound customer context missing", value)
		}
	}
	if strings.Contains(prompt, "No exact contact name/email/ID matched") || strings.Contains(prompt, other.Email) {
		t.Fatal("bound contact ignored in favor of directory")
	}
	contacts, _ := s.crmList(ctx)
	history := []db.ProjectMessage{{ID: "old", AuthorType: "user", Content: other.Email}, *message}
	matched := matchingInboxMemoryContacts(contacts, q, history)
	if len(matched) != 1 || matched[0].ID != contact.ID {
		t.Fatal("old incidental contact replaced current customer")
	}
	history[1].Content = "Check " + other.Email
	matched = matchingInboxMemoryContacts(contacts, q, history)
	if len(matched) != 1 || matched[0].ID != other.ID {
		t.Fatal("explicit target was overridden by bound contact")
	}
	public := q
	public.OwnerID = ""
	if s.inboxCustomerIdentityContext(ctx, public) != "" {
		t.Fatal("private identity injected into unauthenticated context")
	}
}
