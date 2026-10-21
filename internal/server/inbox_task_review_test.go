package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"testing"
)

func TestTaskReviewPromptOnlyUsesInternalSnapshot(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	for _, author := range []string{"user", "agent", "system"} {
		id := db.GenerateUUID()
		m := &db.ProjectMessage{ID: id, RoomID: room.RoomID, AuthorType: author, AuthorID: "cuan_work_review", Content: "CANONICAL_TASK_SNAPSHOT"}
		if err := s.projectStudioRepo.CreateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
		q := db.InboxRequest{MessageID: id, RoomID: room.RoomID}
		got, ok := s.inboxTaskReviewPrompt(ctx, q)
		if ok != (author == "system") {
			t.Fatal("review role spoofed", author)
		}
		if ok && got != "CANONICAL_TASK_SNAPSHOT" {
			t.Fatal("snapshot altered or conversation replayed")
		}
		q.RoomID = "another-room"
		if _, ok := s.inboxTaskReviewPrompt(ctx, q); ok {
			t.Fatal("foreign snapshot accepted")
		}
	}
}
