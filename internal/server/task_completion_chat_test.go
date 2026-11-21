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

func TestTaskCompletionChat_PostVerifyReviewSingleBubbleDeduplication(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "queued")
	ctx := context.Background()
	s.inboxStopping.Store(false)
	s.wireTaskService(s.taskSvc)

	// Set up mock provider binary returning a passing review evaluation
	review := `{"decision":"pass","explanation":"Hasil verifikasi sudah lengkap dan sesuai instruksi.","revision":""}`
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": review}})
	binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	originalBinary, _ := os.ReadFile(binary)
	t.Cleanup(func() {
		if len(originalBinary) > 0 {
			_ = os.WriteFile(binary, originalBinary, 0700)
		}
	})
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' '"+string(response)+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}

	repo := s.projectStudioRepo.(db.InboxLifecycleRepository)
	claimed, ok, err := repo.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	if err != nil || !ok {
		t.Fatal("could not claim review", err)
	}

	// Run inbox request (snapshotReview)
	s.runInboxRequest(repo, claimed)

	// Check task status has transitioned to done
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != task.TaskStatusDone {
		t.Fatalf("expected task status to be done, got %s", after.Status)
	}

	// Check messages in room after review completion
	msgsAfter, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	if err != nil {
		t.Fatal(err)
	}

	// Exactly 1 completion confirmation message should exist in the room
	closingMessages := 0
	for _, m := range msgsAfter {
		if strings.Contains(m.Content, "/tasks/"+d.ID) && (strings.Contains(m.Content, "telah selesai") || strings.Contains(m.Content, "Ringkasan Hasil") || strings.Contains(m.Content, "Hasil verifikasi")) {
			closingMessages++
		}
	}
	if closingMessages != 1 {
		t.Fatalf("expected exactly 1 task completion message in room, got %d. Messages: %+v", closingMessages, msgsAfter)
	}

	// The single message must contain the coordinator explanation and task link
	var completionMsg string
	found := false
	for _, m := range msgsAfter {
		if m.AuthorType == "agent" && strings.Contains(m.Content, "/tasks/"+d.ID) {
			completionMsg = m.Content
			found = true
			break
		}
	}
	if !found || !strings.Contains(completionMsg, "Hasil verifikasi sudah lengkap dan sesuai instruksi.") {
		t.Fatalf("expected completion message to contain coordinator explanation, got: %s", completionMsg)
	}
	if !strings.Contains(completionMsg, "Pekerjaan") || !strings.Contains(completionMsg, "telah selesai") {
		t.Fatalf("expected completion message to contain completion status header, got: %s", completionMsg)
	}
}

func TestTaskCompletionChatHook_IdempotencyWithTitleAndSummary(t *testing.T) {
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
		ID:           "test-task-title-idem",
		Title:        "Agregasi dan Penggantian In-Place Push Notification per Room",
		Status:       task.TaskStatusDone,
		SourceRoomID: roomID,
		Summary:      "Notifikasi web push per percakapan kini hanya memunculkan satu kartu aktif.",
		CompleteNote: "Lolos verifikasi Playwright dan backend dispatch.",
	}

	// Trigger 5 times consecutively
	for i := 0; i < 5; i++ {
		if err := s.PostTaskCompletionMessage(ctx, taskItem); err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}

	msgs, err := studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected exactly 1 message due to idempotency with title/summary, got %d", len(msgs))
	}
}
