package server

import (
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestFormatConversationHistoryContext_IncludesPrecedingTurns(t *testing.T) {
	now := time.Now()
	messages := []db.ProjectMessage{
		{
			ID:         "msg-1",
			AuthorType: "user",
			Content:    "Bg tab section yang unused kok opaque ya",
			CreatedAt:  now.Add(-2 * time.Minute),
		},
		{
			ID:         "msg-2",
			AuthorType: "agent",
			Content:    "Bisa kita ubah background tab section yang tidak aktif menjadi transparan.\n```cuan-delegate\n{\"title\":\"Fix tab\"}\n```",
			CreatedAt:  now.Add(-1 * time.Minute),
		},
		{
			ID:         "msg-3",
			AuthorType: "user",
			Content:    "Benahin",
			CreatedAt:  now,
		},
	}

	result := formatConversationHistoryContext(messages, "msg-3", 10)

	if !strings.Contains(result, "## Authoritative Conversation History") {
		t.Fatalf("missing section header: %s", result)
	}
	if !strings.Contains(result, "**User**: Bg tab section yang unused kok opaque ya") {
		t.Fatalf("missing user's original issue: %s", result)
	}
	if !strings.Contains(result, "**User**: Benahin") {
		t.Fatalf("missing authorization message: %s", result)
	}
	// cuan-delegate block should be stripped from coordinator turn
	if strings.Contains(result, "```cuan-delegate") {
		t.Fatalf("coordinator delegate block was not stripped: %s", result)
	}
}

func TestFormatConversationHistoryContext_LimitsMaxTurns(t *testing.T) {
	now := time.Now()
	messages := make([]db.ProjectMessage, 15)
	for i := 0; i < 15; i++ {
		messages[i] = db.ProjectMessage{
			ID:         db.GenerateUUID(),
			AuthorType: "user",
			Content:    "Message " + string(rune('A'+i)),
			CreatedAt:  now.Add(time.Duration(i) * time.Minute),
		}
	}

	authID := messages[14].ID
	result := formatConversationHistoryContext(messages, authID, 5)

	if strings.Contains(result, "Message A") {
		t.Fatalf("expected old messages beyond maxTurns to be excluded: %s", result)
	}
	if !strings.Contains(result, "Message O") {
		t.Fatalf("expected latest message to be included: %s", result)
	}
}
