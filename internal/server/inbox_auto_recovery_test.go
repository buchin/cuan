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

func TestInboxAutomaticRecoveryAcrossServerRestart(t *testing.T) {
	for _, outcome := range []string{"success", "fails-again"} {
		t.Run(outcome, func(t *testing.T) {
			s, _, call, home := inboxFixture(t)
			script := `#!/usr/bin/env python3
import json,sys,pathlib,os
p=pathlib.Path(os.environ['HOME'])/'automatic-calls'
n=int(p.read_text()) if p.exists() else 0
p.write_text(str(n+1))
assert 'AUTOMATIC_ORIGINAL_INTENT' in sys.argv[2]
result={'status':'ERROR','message':'Temporary provider failure'}
if n>0 and '` + outcome + `'=='success':
 result={'status':'SUCCESS','response':json.dumps({'reply':'Recovered the original answer.','topic_title':'','action':None})}
print(json.dumps({'event':'result','result':result}))
`
			binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"`+db.GenerateUUID()+`","message":"AUTOMATIC_ORIGINAL_INTENT"}`)
			if w.Code != 202 {
				t.Fatal(w.Body.String())
			}
			var accepted struct{ Request db.InboxRequest }
			json.Unmarshal(w.Body.Bytes(), &accepted)
			waitInboxState(t, s, accepted.Request.RoomID, accepted.Request.RequestID, "failed")
			rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), accepted.Request.RoomID)
			dtos := inboxRecoveryRequests(rows)
			if len(dtos) != 1 || dtos[0].RecoveryAction != "recovering" {
				t.Fatal("safe failure not preparing recovery")
			}
			// Reconstruct the host around the durable request store and original account.
			s.inboxStopping.Store(true)
			next, _ := setupTestChatServer(t)
			next.cfg.Environment = "staging"
			next.SetProjectStudioRepository(s.projectStudioRepo)
			next.SetAgentRepository(s.agentRepo)
			time.Sleep(2100 * time.Millisecond) // bounded recovery backoff, not an upstream wait
			next.drainInboxRoom(accepted.Request.RoomID)
			expected := "completed"
			if outcome == "fails-again" {
				expected = "failed"
			}
			childID := db.InboxRetryID(accepted.Request.RequestID)
			waitInboxState(t, next, accepted.Request.RoomID, childID, expected)
			// Repeated dispatcher scans must not make a third attempt.
			next.drainInboxRoom(accepted.Request.RoomID)
			next.drainInboxRoom(accepted.Request.RoomID)
			rows, _ = next.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), accepted.Request.RoomID)
			if len(rows) != 2 || rows[1].MessageID != rows[0].MessageID {
				t.Fatal("duplicate work or lost context")
			}
			count, _ := os.ReadFile(filepath.Join(home, "automatic-calls"))
			if string(count) != "2" {
				t.Fatal("automatic recovery budget exceeded", string(count))
			}
			messages, _ := next.projectStudioRepo.ListMessagesByRoomID(context.Background(), accepted.Request.RoomID)
			expectedMessages := 1
			if outcome == "success" {
				expectedMessages = 2
			}
			if len(messages) != expectedMessages {
				t.Fatal("user message duplicated or reply missing", len(messages))
			}
			if rows[1].Steps[0].Kind != "retry_auto" {
				t.Fatal("automatic recovery not recorded")
			}
		})
	}
}
