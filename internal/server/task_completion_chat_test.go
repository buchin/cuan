package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestTaskCompletionChatHook_PostsClosingMessageOnTransitionToDone(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	studioRepo := db.NewMemoryProjectStudioRepository()
	tSvc := task.NewService(taskStore, nil)

	s := &Server{
		taskSvc:           tSvc,
		projectStudioRepo: studioRepo,
	}
	s.wireTaskService(tSvc)

	roomID := db.GenerateUUID()
	userMsgID := db.GenerateUUID()
	now := time.Now().UTC()

	// Initial chat message from user
	if err := studioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         userMsgID,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		AuthorName: "Owner",
		Content:    "Tolong perbaiki notifikasi",
		CreatedAt:  now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("failed to create user message: %v", err)
	}

	// Create task in backlog status
	created, err := tSvc.CreateTask(ctx, "operator_user", task.CreateTaskRequest{
		Title:           "Perbaikan Notifikasi Selesai",
		Instructions:    "Cuan managed delegation v1.\nFix silent finish",
		Status:          task.TaskStatusBacklog,
		SourceRoomID:    roomID,
		SourceMessageID: userMsgID,
		AssignedAgent:   "agent_cuan",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Move task to review
	reviewStatus := task.TaskStatusReview
	summary := "Akar masalah silent finish diperbaiki dengan event hook penutup chat."
	completeNote := "Lolos verifikasi unit test backend dan UI visual Playwright."
	reviewed, err := tSvc.UpdateTask(ctx, created.ID, "runner", task.UpdateTaskRequest{
		Status:       &reviewStatus,
		Summary:      &summary,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatalf("failed to update task to review: %v", err)
	}

	// Verify no completion message posted yet in room
	msgs, err := studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message in room before done, got %d", len(msgs))
	}

	// Transition task to done
	doneStatus := task.TaskStatusDone
	_, err = tSvc.UpdateTask(ctx, reviewed.ID, "coordinator", task.UpdateTaskRequest{
		Status: &doneStatus,
	})
	if err != nil {
		t.Fatalf("failed to update task to done: %v", err)
	}

	// Verify closing message is posted to room
	msgs, err = studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to list messages after done: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages in room after done, got %d", len(msgs))
	}

	closingMsg := msgs[1]
	if closingMsg.AuthorType != "agent" {
		t.Errorf("expected AuthorType 'agent', got %q", closingMsg.AuthorType)
	}
	if closingMsg.AuthorName != "@cuan" {
		t.Errorf("expected AuthorName '@cuan', got %q", closingMsg.AuthorName)
	}
	if !strings.Contains(closingMsg.Content, "Perbaikan Notifikasi Selesai") {
		t.Errorf("expected content to mention task title, got %q", closingMsg.Content)
	}
	if !strings.Contains(closingMsg.Content, summary) {
		t.Errorf("expected content to include summary, got %q", closingMsg.Content)
	}
	if !strings.Contains(closingMsg.Content, completeNote) {
		t.Errorf("expected content to include complete note, got %q", closingMsg.Content)
	}
	if !strings.Contains(closingMsg.Content, "/tasks/"+created.ID) {
		t.Errorf("expected content to include task link, got %q", closingMsg.Content)
	}
}

func TestTaskCompletionChatHook_IdempotencyNoDuplicates(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	studioRepo := db.NewMemoryProjectStudioRepository()
	tSvc := task.NewService(taskStore, nil)

	s := &Server{
		taskSvc:           tSvc,
		projectStudioRepo: studioRepo,
	}
	s.wireTaskService(tSvc)

	roomID := db.GenerateUUID()
	taskItem := &task.Task{
		ID:           "test-task-idem",
		Title:        "Tugas Idempoten",
		Status:       task.TaskStatusDone,
		SourceRoomID: roomID,
		Summary:      "Ringkasan idempoten",
		CompleteNote: "Bukti idempoten",
	}

	// Trigger multiple times
	for i := 0; i < 3; i++ {
		if err := s.PostTaskCompletionMessage(ctx, taskItem); err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}

	msgs, err := studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 message due to idempotency, got %d", len(msgs))
	}
}

func TestTaskCompletionChatHook_NoSourceRoomNoChat(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	studioRepo := db.NewMemoryProjectStudioRepository()
	tSvc := task.NewService(taskStore, nil)

	s := &Server{
		taskSvc:           tSvc,
		projectStudioRepo: studioRepo,
	}
	s.wireTaskService(tSvc)

	taskItem := &task.Task{
		ID:           "test-task-noroom",
		Title:        "Tugas Tanpa Room",
		Status:       task.TaskStatusDone,
		SourceRoomID: "",
		Summary:      "Ringkasan",
		CompleteNote: "Bukti",
	}

	if err := s.PostTaskCompletionMessage(ctx, taskItem); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
