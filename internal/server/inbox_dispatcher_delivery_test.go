package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type blockedResultStore struct {
	task.Store
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockedResultStore) GetTask(ctx context.Context, id string) (*task.Task, error) {
	if id == "slow-history" {
		b.once.Do(func() { close(b.entered) })
		select {
		case <-b.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return b.Store.GetTask(ctx, id)
}
func TestInboxDeliveryScanDoesNotDelayNewConversation(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	store := &blockedResultStore{Store: task.NewMemoryStore(), entered: make(chan struct{}), release: make(chan struct{})}
	if err := store.CreateTask(context.Background(), &task.Task{ID: "slow-history", Title: "Historical result", SourceRoomID: "different-room", Status: task.TaskStatusReview, Instructions: "Cuan managed delegation v1.\nApproved work"}); err != nil {
		t.Fatal(err)
	}
	s.taskSvc = task.NewService(store, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer close(store.release)
	s.kickInboxTaskUpdates(ctx)
	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal("slow history was not inspected")
	}
	// A second tick must not create another concurrent history scan.
	s.kickInboxTaskUpdates(ctx)
	script := `#!/bin/sh
printf '%s' '{"event":"result","result":{"status":"SUCCESS","response":"{\"schema_version\":\"1\",\"message\":\"Conversation still responds\",\"actions\":[]}"}}'
`
	if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	reply := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"d77a5880-015d-4911-9d77-6d3c016332a0","message":"Hello"}`)
	if reply.Code != 202 && reply.Code != 200 {
		t.Fatal(reply.Code, reply.Body.String())
	}
	// The test must complete while the unrelated result query remains blocked.
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), s.resolveConversationContext(context.Background(), "agent_lifecycle", "").RoomID)
	if err != nil || len(requests) != 1 {
		t.Fatal(err)
	}
	waitInboxState(t, s, requests[0].RoomID, requests[0].RequestID, "completed")
}
