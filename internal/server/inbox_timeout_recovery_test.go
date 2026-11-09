package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestInboxTimedOutOperatorTurnRecoversOnceWithoutManualRetry(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	t.Setenv("INBOX_LIFECYCLE_TIMEOUT", "3s")
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	script := `#!/bin/sh
if mkdir "$HOME/first-timeout" 2>/dev/null; then
 exec sleep 10
fi
printf '%s' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Recovered from current saved state.\",\"topic_title\":\"\",\"action\":null}"}}'
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"`+db.GenerateUUID()+`","message":"Explain the saved request; do not create tasks or contact anyone."}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct{ Request db.InboxRequest }
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	end := time.Now().Add(20 * time.Second)
	var rows []db.InboxRequest
	for time.Now().Before(end) {
		var err error
		rows, err = store.ListInboxRequests(context.Background(), accepted.Request.RoomID)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 2 && rows[1].State == "completed" {
			break
		}
		// Equivalent to the periodic dispatcher; no manual retry or new user message.
		s.kickInboxRoom(accepted.Request.RoomID)
		time.Sleep(100 * time.Millisecond)
	}
	if len(rows) != 2 || rows[0].State != "interrupted" || rows[1].State != "completed" || !db.InboxRetryNeedsReconciliation(rows[1]) || rows[1].MessageID != rows[0].MessageID {
		t.Fatalf("automatic reconciliation failed: %+v", rows)
	}
	if rows[1].RequestID != db.InboxRetryID(rows[0].RequestID) {
		t.Fatal("lost deterministic retry identity")
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), accepted.Request.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	userCount, replyCount := 0, 0
	for _, m := range messages {
		if m.AuthorType == "user" {
			userCount++
		}
		if strings.Contains(m.Content, "Recovered from current saved state.") {
			replyCount++
		}
	}
	if userCount != 1 || replyCount != 1 {
		t.Fatal("duplicated messages", userCount, replyCount)
	}
	if db.InboxAutomaticRecoveryCandidate(rows, time.Now()) != nil {
		t.Fatal("automatic recovery loop")
	}
}
