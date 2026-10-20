package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"strings"
	"testing"
	"time"
)

func TestInboxContextContainsSafePriorOutcomes(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	now := time.Now().UTC()
	room := &db.ProjectRoom{ID: db.GenerateUUID(), ProjectID: "default", Title: "Outcomes", CreatedAt: now, UpdatedAt: now}
	s.projectStudioRepo.CreateRoom(ctx, room)
	prior := db.InboxRequest{RoomID: room.ID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: "owner", AgentID: "agent_sm", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	store.AcceptInboxRequest(ctx, prior, &db.ProjectMessage{ID: prior.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Read my project", CreatedAt: now, UpdatedAt: now})
	prior, _, _ = store.ClaimInboxRequest(ctx, room.ID, prior.RequestID)
	store.AppendInboxStep(ctx, room.ID, prior.RequestID, "runtime_permission_denied", "Tool permission denied")
	store.AppendInboxStep(ctx, room.ID, prior.RequestID, "runtime_session", "private-session-do-not-include")
	prior.State = "failed"
	prior.SafeError = "The tool permission was denied."
	store.FinishInboxRequest(ctx, prior, nil)
	target := db.InboxRequest{RoomID: room.ID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: "owner", AgentID: "agent_sm", State: "queued", CreatedAt: now.Add(time.Second), Deadline: now.Add(time.Hour)}
	store.AcceptInboxRequest(ctx, target, &db.ProjectMessage{ID: target.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Analyze the last error", CreatedAt: target.CreatedAt, UpdatedAt: target.CreatedAt})
	prompt, err := s.inboxContext(ctx, target)
	if err != nil || !strings.Contains(prompt, "Recorded request outcomes") || !strings.Contains(prompt, `"permission_denied":true`) || strings.Contains(prompt, "private-session") {
		t.Fatal("safe outcomes context", err)
	}
}
