package server

import (
	"context"
	"strings"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestServer_DynamicContext_NonOlshopGreeting(t *testing.T) {
	ctx := context.Background()
	studioRepo := cuanDB.NewMemoryProjectStudioRepository()
	s := &Server{
		projectStudioRepo: studioRepo,
	}

	roomA := cuanDB.GenerateUUID()

	// Notify review for "dracin" project
	s.sendBusinessFriendlyReviewNotification(ctx, "task_dracin_1", "Optimasi Buffer Subtitle", "Subtitle buffering telah diminimalisir", roomA, "", 2, []string{
		"Perlu verifikasi codec di aplikasi mobile",
	}, "dracin")

	msgs, err := studioRepo.ListMessagesByRoomID(ctx, roomA)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	content := msgs[0].Content
	// Must NOT contain "toko Anda"
	if strings.Contains(strings.ToLower(content), "toko anda") {
		t.Fatalf("greeting for non-olshop project 'dracin' must NOT contain 'toko Anda', got:\n%s", content)
	}

	// Must contain project name "dracin"
	if !strings.Contains(content, "dracin") {
		t.Fatalf("expected project name 'dracin' in content, got:\n%s", content)
	}
}

func TestServer_InvestigationTask_ReaperTimeoutIs5m(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// Create investigation task created 6 minutes ago (exceeding 5m timeout)
	now := time.Now().UTC()
	sixMinutesAgo := now.Add(-6 * time.Minute)

	tInv := &task.Task{
		ID:                "task_inv_reap",
		Title:             "Investigasi timeout payment gateway",
		Instructions:      "Analisis log error",
		ExecutionCategory: task.ExecutionCategoryReadOnly,
		Status:            task.TaskStatusDoing,
		CreatedAt:         sixMinutesAgo,
		UpdatedAt:         sixMinutesAgo,
	}
	_ = s.taskSvc.Store().CreateTask(ctx, tInv)

	attInv := &task.TaskAttempt{
		ID:        "att_inv_reap",
		TaskID:    tInv.ID,
		Status:    task.AttemptStatusRunning,
		RunnerID:  "cuan-runner",
		CreatedAt: sixMinutesAgo,
	}
	_ = s.taskSvc.Store().CreateAttempt(ctx, attInv)

	// Normal code task created 6 minutes ago (within default 20m timeout)
	tCode := &task.Task{
		ID:                "task_code_stay",
		Title:             "Implementasi fitur checkout",
		Instructions:      "Buat kode handler checkout",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusDoing,
		CreatedAt:         sixMinutesAgo,
		UpdatedAt:         sixMinutesAgo,
	}
	_ = s.taskSvc.Store().CreateTask(ctx, tCode)

	attCode := &task.TaskAttempt{
		ID:        "att_code_stay",
		TaskID:    tCode.ID,
		Status:    task.AttemptStatusRunning,
		RunnerID:  "cuan-runner",
		CreatedAt: sixMinutesAgo,
	}
	_ = s.taskSvc.Store().CreateAttempt(ctx, attCode)

	// Run ReconcileZombieAttempts with the real per-attempt ceiling.
	err := s.taskSvc.ReconcileZombieAttempts(ctx, 20*time.Minute, func(taskID string) bool { return true })
	if err != nil {
		t.Fatalf("ReconcileZombieAttempts error: %v", err)
	}

	// Both tasks are still inside their declared ceiling, so a young attempt must
	// survive. The wall clock is a safety bound; progress decides the stop.
	reapedInv, err := s.taskSvc.Store().GetTask(ctx, tInv.ID)
	if err != nil {
		t.Fatalf("GetTask error: %v", err)
	}
	if reapedInv.Status != task.TaskStatusDoing {
		t.Fatalf("expected investigation task to stay doing inside its ceiling, got '%s'", reapedInv.Status)
	}

	stayCode, err := s.taskSvc.Store().GetTask(ctx, tCode.ID)
	if err != nil {
		t.Fatalf("GetTask error: %v", err)
	}
	if stayCode.Status != task.TaskStatusDoing {
		t.Fatalf("expected code task to stay doing inside its ceiling, got '%s'", stayCode.Status)
	}
}

func TestServer_AutoApplyAndDeploy_SkipsInvestigationTask(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// Investigation task marked done with no PR/commits
	tInv := &task.Task{
		ID:                "task_inv_skip",
		Title:             "Investigasi pemadaman pihak ketiga",
		Instructions:      "Lakukan audit log koneksi",
		Summary:           "RCA menunjukkan pemadaman eksternal",
		CompleteNote:      "Laporan investigasi selesai",
		ExecutionCategory: task.ExecutionCategoryReadOnly,
		Status:            task.TaskStatusDone,
		BranchName:        "task/task_inv_skip",
	}
	_ = s.taskSvc.Store().CreateTask(ctx, tInv)

	// autoApplyAndDeployTask should gracefully skip without crashing or erroring
	s.autoApplyAndDeployTask(ctx, tInv.ID, "proj_test")

	// Task remains done and completeNote is not contaminated by fake deploy message
	res, err := s.taskSvc.Store().GetTask(ctx, tInv.ID)
	if err != nil {
		t.Fatalf("GetTask error: %v", err)
	}
	if strings.Contains(res.CompleteNote, "Otomatis memicu live deployment") {
		t.Fatalf("investigation task must NOT trigger auto_deploy note, got: %s", res.CompleteNote)
	}
}

func TestDualChannelEnvelope_OutputFormatAndJargonFilter(t *testing.T) {
	title := "Investigasi issue Definition of Done dan heartbeat lost pada attempt #1"
	summary := "Evaluasi DoD menemukan gap"
	gaps := []string{"lease expired: connection to worker timed out"}

	env := runner.BuildTaskReviewEnvelope(title, summary, gaps, 1, false, "Bukti RCA server log", "dracin")

	if env.RequiresDeploy {
		t.Fatal("expected RequiresDeploy = false")
	}

	jsonStr, err := env.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	// Ensure no forbidden technical jargon
	for _, f := range []string{"definition of done", "dod", "heartbeat lost", "lease expired", "attempt #"} {
		if strings.Contains(strings.ToLower(jsonStr), f) {
			t.Fatalf("found technical jargon %q in dual-channel envelope JSON:\n%s", f, jsonStr)
		}
	}

	if strings.Contains(strings.ToLower(jsonStr), "toko anda") {
		t.Fatalf("non-olshop project 'dracin' must not contain 'toko Anda', got:\n%s", jsonStr)
	}
}
