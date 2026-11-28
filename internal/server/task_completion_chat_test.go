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

	if err := studioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "proj_test",
		Title:     "Room Test",
	}); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

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

	// Room must remain active / unresolved immediately upon task completion
	checkRoom, err := studioRepo.GetRoomByID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to get room: %v", err)
	}
	if checkRoom.ResolvedAt != nil {
		t.Fatalf("expected room to remain active/unresolved immediately upon task completion, got resolved_at=%v", checkRoom.ResolvedAt)
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

	// Post-verify review: this fixture represents a finished native read-only audit.
	d.ExecutionCategory = task.ExecutionCategoryReadOnly
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	finished := time.Now().UTC()
	d.LatestAttempt.FinishedAt = &finished
	if err := s.taskSvc.Store().UpdateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}
	for _, a := range []*task.TaskActivity{
		{ID: "reconciled-current-report", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_report", Author: d.AssignedAgent, NewStatus: task.TaskStatusReview, Message: d.Summary + "\n" + d.CompleteNote, CreatedAt: finished},
		{ID: "reconciled-unchanged-receipt", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "repository_scope_verified", Author: d.LatestAttempt.RunnerID, NewStatus: "read_only", Message: "Native fixture verified unchanged audit scope", CreatedAt: finished},
	} {
		if err := s.taskSvc.Store().CreateActivity(ctx, a); err != nil {
			t.Fatal(err)
		}
	}

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

func TestPostTaskCompletionMessage_DevNullWithImageAndApprovalCard(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	if err := studioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Topic:     "agent:agent_cuan",
		Title:     "DevNull Daily Ship Log",
	}); err != nil {
		t.Fatalf("create room: %v", err)
	}
	// Seed an inbox request in the room so taskArtifactOwner can resolve owner even when SourceMessageID == ""
	seedReq := db.InboxRequest{
		OwnerID:   "owner_test",
		AgentID:   "agent_cuan",
		RoomID:    roomID,
		RequestID: db.GenerateUUID(),
		MessageID: db.GenerateUUID(),
		ReplyID:   db.GenerateUUID(),
		State:     "completed",
	}
	if _, _, err := studioRepo.AcceptInboxRequest(ctx, seedReq, &db.ProjectMessage{
		ID:         seedReq.MessageID,
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "operator_user",
		Content:    "seed",
	}); err != nil {
		t.Fatalf("seed request: %v", err)
	}

	rawSummary := `Draft Daily Ship Log untuk dev-null.sh (47 commit dalam 24 jam terakhir) siap ditinjau sebagai Approval Proposal. Belum diposting ke dev-null.sh (menunggu persetujuan Anda).

--- DRAFT NARASI SIAP POSTING ---
Shipped Cuan (24h, 47 commits): Merapikan navigasi, mempercepat deploy, dan memperkuat pemulihan otomatis agen.

Deployments lebih cepat & jelas: short_sha & ikon status sejajar.
Ketahanan agen: chained auto-recovery untuk verifikasi terinterupsi.

Coba & ikuti perkembangannya:
https://cuan.work

--- REKOMENDASI GAMBAR ---
• Rekomendasi: screenshots/s70-desktop-inbox-mail.png (terlampir sebagai preview).
• Alternatif: posting text-only tanpa gambar.

Balas "Setuju (pakai gambar)", "Setuju (text-only)", atau sertakan revisi sebelum scripts/post-to-devnull.sh dijalankan.`

	created, err := tSvc.CreateTask(ctx, "scheduler", task.CreateTaskRequest{
		ProjectID:     "default",
		Title:         "📢 DevNull Daily Ship Log & Story Drafter",
		Instructions:  "Susun draft Daily Ship Log untuk dev-null.sh.",
		Status:        task.TaskStatusQueued,
		SourceRoomID:  roomID,
		AssignedAgent: "agent_cuan",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	exitZero := 0
	now := time.Now().UTC()
	att := &task.TaskAttempt{
		ID:        db.GenerateUUID(),
		TaskID:    created.ID,
		RunnerID:  "runner-01",
		AgentName: "agent_cuan",
		Status:    task.AttemptStatusSucceeded,
		ExitCode:  &exitZero,
		CreatedAt: now,
		StartedAt: &now,
		FinishedAt: &now,
	}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	// Create a valid 1x1 PNG in a temp workspace root and publish it as a task artifact
	workDir := t.TempDir()
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
		0xb0, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(workDir+"/preview.png", pngBytes, 0600); err != nil {
		t.Fatalf("write png: %v", err)
	}
	root, err := os.OpenRoot(workDir)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer root.Close()
	artifacts, err := readTaskArtifacts(root, created.ID, att.ID, []string{"preview.png"})
	if err != nil {
		t.Fatalf("readTaskArtifacts: %v", err)
	}

	detailBeforeDone, err := tSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatalf("get task detail: %v", err)
	}
	owner, err := s.taskArtifactOwner(ctx, detailBeforeDone)
	if err != nil || owner != "owner_test" {
		t.Fatalf("expected taskArtifactOwner=owner_test even without SourceMessageID, got %q err=%v", owner, err)
	}
	if err := s.publishTaskArtifacts(ctx, detailBeforeDone, owner, artifacts); err != nil {
		t.Fatalf("publishTaskArtifacts: %v", err)
	}

	reviewStatus := task.TaskStatusReview
	if _, err := tSvc.UpdateTask(ctx, created.ID, "coordinator", task.UpdateTaskRequest{Status: &reviewStatus}); err != nil {
		t.Fatalf("move task to review: %v", err)
	}
	doneStatus := task.TaskStatusDone
	completeNote := "Draft dan screenshot diverifikasi."
	updated, err := tSvc.UpdateTask(ctx, created.ID, "coordinator", task.UpdateTaskRequest{
		Status:       &doneStatus,
		Summary:      &rawSummary,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatalf("update task: %v", err)
	}

	if err := s.PostTaskCompletionMessage(ctx, updated); err != nil {
		t.Fatalf("PostTaskCompletionMessage: %v", err)
	}

	dtos, err := s.getFormattedMessagesResult(ctx, roomID, "")
	if err != nil {
		t.Fatalf("getFormattedMessagesResult: %v", err)
	}
	var completionDTO *ChatMessageDTO
	for i := range dtos {
		if dtos[i].ApprovalPlan != nil && strings.Contains(dtos[i].ApprovalPlan.Description, "Shipped Cuan (24h, 47 commits)") {
			completionDTO = &dtos[i]
			break
		}
	}
	if completionDTO == nil {
		t.Fatalf("expected formatted DevNull completion DTO in room, got: %+v", dtos)
	}
	if len(completionDTO.Attachments) != 1 || !completionDTO.HasAttachment {
		t.Fatalf("expected 1 image attachment on completion message, got %+v", completionDTO.Attachments)
	}
	if completionDTO.ApprovalPlan == nil || completionDTO.ApprovalTitle != "Posting Daily Ship Log ke dev-null.sh" {
		t.Fatalf("expected interactive approval plan on completion message, got plan=%+v title=%q", completionDTO.ApprovalPlan, completionDTO.ApprovalTitle)
	}
	for _, forbidden := range []string{"--- DRAFT NARASI SIAP POSTING ---", "--- REKOMENDASI GAMBAR ---", "scripts/post-to-devnull.sh", "short_sha", "chained auto-recovery", "Ringkasan Hasil", "Bukti Verifikasi", `Balas "Setuju`} {
		if strings.Contains(completionDTO.Content, forbidden) || strings.Contains(completionDTO.ApprovalPlan.Description, forbidden) {
			t.Fatalf("completion content/plan should not contain %q, got content=%s desc=%s", forbidden, completionDTO.Content, completionDTO.ApprovalPlan.Description)
		}
	}
	if approved, err := s.persistedInboxPlan(ctx, roomID, "owner_test", "agent_cuan", completionDTO.UUID); err != nil || approved == nil {
		t.Fatalf("expected persistedInboxPlan to accept completion approval card, got err=%v", err)
	}
}

func TestReadTaskArtifacts_RejectsInvalidImageBytes(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(workDir+"/broken.png", []byte("not a png image"), 0600); err != nil {
		t.Fatalf("write broken png: %v", err)
	}
	root, err := os.OpenRoot(workDir)
	if err != nil {
		t.Fatalf("open root: %v", err)
	}
	defer root.Close()
	if _, err := readTaskArtifacts(root, "task-1", "att-1", []string{"broken.png"}); err == nil {
		t.Fatalf("expected readTaskArtifacts to reject corrupt png bytes")
	}
}
