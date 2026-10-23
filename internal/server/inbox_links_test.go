package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestResolveInternalCuanLinksInConversation(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()

	// 1. Setup target room with some messages
	targetRoomID := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        targetRoomID,
		ProjectID: "default",
		Title:     "Audited UI Room",
		Topic:     "discussion",
	})
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     targetRoomID,
		AuthorType: "user",
		Content:    "Navbar button overlap bug on mobile",
		CreatedAt:  time.Now().Add(-5 * time.Minute),
	})
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     targetRoomID,
		AuthorType: "agent",
		Content:    "Inspecting navbar styles and layout",
		CreatedAt:  time.Now().Add(-4 * time.Minute),
	})

	// 2. Setup target task
	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)
	createdTask, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID:    "default",
		Title:        "Fix mobile navbar layout",
		Instructions: "Fixed CSS padding and flex-wrap on header",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Current active room
	currentRoomID := db.GenerateUUID()

	// Test case A: User pastes full domain URL to room
	userPromptWithRoomURL := "Cek percakapan ini apakah normal: https://cuan.dojo.cc/inbox/" + targetRoomID
	resolvedRoom := s.resolveInternalCuanLinks(ctx, userPromptWithRoomURL, currentRoomID)

	if !strings.Contains(resolvedRoom, "Referenced Internal Cuan Conversation: Room "+targetRoomID) {
		t.Fatalf("expected resolved room link header, got: %s", resolvedRoom)
	}
	if !strings.Contains(resolvedRoom, "Navbar button overlap bug on mobile") {
		t.Fatalf("expected referenced message content in resolved link, got: %s", resolvedRoom)
	}

	// Test case B: User pastes relative task link
	userPromptWithTaskURL := "Kenapa task /tasks/" + createdTask.ID + " belum jalan?"
	resolvedTask := s.resolveInternalCuanLinks(ctx, userPromptWithTaskURL, currentRoomID)

	if !strings.Contains(resolvedTask, "Referenced Internal Cuan Task: ID "+createdTask.ID) {
		t.Fatalf("expected resolved task link header, got: %s", resolvedTask)
	}
	if !strings.Contains(resolvedTask, "Fix mobile navbar layout") {
		t.Fatalf("expected task title in resolved link, got: %s", resolvedTask)
	}

	// Test case C: Self room link is ignored (no recursion into current active room)
	selfPrompt := "Check https://cuan.dojo.cc/inbox/" + currentRoomID
	resolvedSelf := s.resolveInternalCuanLinks(ctx, selfPrompt, currentRoomID)
	if resolvedSelf != "" {
		t.Fatalf("expected self room to not be duplicated, got: %s", resolvedSelf)
	}
}
