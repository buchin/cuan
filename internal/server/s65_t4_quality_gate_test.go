package server

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// Regex assertions for forbidden technical jargon that must NEVER leak into the public/inbox layer.
var (
	regexDefinitionOfDone = regexp.MustCompile(`(?i)\bdefinition[\s-]+of[\s-]+done\b`)
	regexDoD              = regexp.MustCompile(`(?i)\bdod\b`)
	regexHeartbeatLost    = regexp.MustCompile(`(?i)\bheartbeat\s+lost\b`)
	regexLeaseExpired     = regexp.MustCompile(`(?i)\blease\s+expired\b`)
	regexAttemptNumber    = regexp.MustCompile(`(?i)\battempt\s*(?:#?\s*\d+)?\b`)
)

func assertZeroJargonLeakage(t *testing.T, contextName, text string) {
	t.Helper()
	patterns := map[string]*regexp.Regexp{
		"Definition of Done": regexDefinitionOfDone,
		"DoD":                regexDoD,
		"heartbeat lost":     regexHeartbeatLost,
		"lease expired":      regexLeaseExpired,
		"attempt #":          regexAttemptNumber,
	}

	for term, re := range patterns {
		if loc := re.FindStringIndex(text); loc != nil {
			matched := text[loc[0]:loc[1]]
			t.Fatalf("[%s] Found forbidden technical jargon '%s' (matched: '%s') in user-facing message:\n%s",
				contextName, term, matched, text)
		}
	}
}

// TestQualityGate_AntiJargonGuardrail_PublicInboxLayer rigorously tests all message formatting,
// review envelopes, notifications, and room messages to ensure zero jargon leakage across the public layer.
func TestQualityGate_AntiJargonGuardrail_PublicInboxLayer(t *testing.T) {
	ctx := context.Background()

	rawTechnicalTitle := "Perbaikan bug Definition of Done dan penanganan heartbeat lost pada attempt #2"
	rawTechnicalSummary := "Evaluasi DoD menemukan lease expired saat attempt 1 dan attempt #2 gagal memenuhi kriteria Definition-of-Done."
	rawTechnicalGaps := []string{
		"Kriteria Definition of Done belum terpenuhi.",
		"Worker heartbeat lost and lease expired during execution.",
		"Batas attempt #3 tercapai.",
	}
	rawTechnicalEvidence := "Log server: DoD check failed; lease expired; runner heartbeat lost"

	// 1. Test FormatBusinessFriendlyMessage (Task Completed)
	completedMsg, nextSteps := runner.FormatBusinessFriendlyMessage(rawTechnicalTitle, rawTechnicalSummary, "Toko Kopi Senja")
	assertZeroJargonLeakage(t, "FormatBusinessFriendlyMessage", completedMsg)
	for i, step := range nextSteps {
		assertZeroJargonLeakage(t, fmt.Sprintf("NextStep[%d]", i), step)
	}

	// 2. Test FormatBusinessFriendlyReviewMessage (Task Review / Action Needed)
	reviewMsg, options := runner.FormatBusinessFriendlyReviewMessage(rawTechnicalTitle, rawTechnicalSummary, rawTechnicalGaps, 2, "Dracin Drama")
	assertZeroJargonLeakage(t, "FormatBusinessFriendlyReviewMessage", reviewMsg)
	for i, opt := range options {
		assertZeroJargonLeakage(t, fmt.Sprintf("ReviewOption[%d]", i), opt)
	}

	// 3. Test Dual-Channel Envelope (BuildTaskReviewEnvelope and JSON Serialization)
	env := runner.BuildTaskReviewEnvelope(rawTechnicalTitle, rawTechnicalSummary, rawTechnicalGaps, 3, false, rawTechnicalEvidence, "dracin")
	assertZeroJargonLeakage(t, "TaskReviewEnvelope.Summary", env.Summary)
	assertZeroJargonLeakage(t, "TaskReviewEnvelope.Draft", env.Draft)
	assertZeroJargonLeakage(t, "TaskReviewEnvelope.Evidence", env.Evidence)

	envJSON, err := env.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize TaskReviewEnvelope to JSON: %v", err)
	}
	assertZeroJargonLeakage(t, "TaskReviewEnvelope.ToJSON", envJSON)

	// 4. Test Server Notification Layer in Room Inbox
	studioRepo := cuanDB.NewMemoryProjectStudioRepository()
	s := &Server{
		projectStudioRepo: studioRepo,
	}
	roomID := cuanDB.GenerateUUID()

	s.sendBusinessFriendlyReviewNotification(ctx, "task_qg_jargon", rawTechnicalTitle, rawTechnicalSummary, roomID, "", 3, rawTechnicalGaps, "dracin")

	msgs, err := studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("Failed to list room messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("Expected at least 1 message delivered to room, got 0")
	}

	for i, m := range msgs {
		assertZeroJargonLeakage(t, fmt.Sprintf("RoomMessage[%d]", i), m.Content)
	}
}

// TestQualityGate_DualPath_CodeFixRequiresLiveDeploy tests Path (a):
// A task modifying repository code on an auto_deploy enabled project MUST require deployment
// and cannot be completed without verified deployment.
func TestQualityGate_DualPath_CodeFixRequiresLiveDeploy(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Create project with auto_deploy enabled
	proj := &project.Project{
		ID:        "proj_qg_autodeploy",
		Name:      "Cuan Shop",
		RepoName:  "cuan-shop",
		Metadata:  json.RawMessage(`{"auto_deploy":true,"deploy_provider":"script"}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.projectSvc.Store().CreateProject(ctx, proj); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// 2. Create Code-Fix Task
	codeTask := &task.Task{
		ID:                "task_qg_codefix",
		ProjectID:         proj.ID,
		Title:             "Perbaikan Bug Keranjang Belanja",
		Instructions:      "Modifikasi handler checkout dan validasi stok",
		Summary:           "Bug checkout berhasil diperbaiki",
		CompleteNote:      "Unit tests pass",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusDoing,
		BranchName:        "task/task_qg_codefix",
	}
	if err := s.taskSvc.Store().CreateTask(ctx, codeTask); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	attCode := &task.TaskAttempt{
		ID:            "att_qg_codefix",
		TaskID:        codeTask.ID,
		AttemptNumber: 1,
		Status:        task.AttemptStatusRunning,
		RunnerID:      "cuan-runner-1",
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attCode); err != nil {
		t.Fatalf("Failed to create attempt: %v", err)
	}

	// Case A: Attempting to complete code task with requires_deploy=true but without deployment confirmation
	// Runner service must reject it with auto_deploy compliance error.
	runnerStore := runner.NewMemoryStore(s.taskSvc.Store())
	runnerSvc := runner.NewService(runnerStore, s.taskSvc.Store(), s.projectSvc)
	hasCodeChanges := true
	requiresDeploy := true
	diffSummary := "modified: internal/cart/checkout.go (+12, -4)"
	commitSHA := "abcdef123456"

	reqFail := runner.CompleteAttemptRequest{
		RunnerID:                "cuan-runner-1",
		Status:                  task.AttemptStatusSucceeded,
		ExitCode:                0,
		AgentReportConfirmed:    true,
		AgentReportRequired:     true,
		HasCodeChanges:          &hasCodeChanges,
		RequiresDeploy:          &requiresDeploy,
		DiffSummary:             diffSummary,
		CommitSHA:               commitSHA,
		LiveDeploymentConfirmed: false, // NOT deployed yet!
	}

	completedAttFail, err := runnerSvc.Complete(ctx, attCode.ID, reqFail)
	if err != nil {
		t.Fatalf("CompleteAttempt unexpected error: %v", err)
	}
	if completedAttFail.Status != task.AttemptStatusFailed {
		t.Fatalf("Expected code-fix task to fail compliance gate when not deployed live, got status: %s", completedAttFail.Status)
	}
	if !strings.Contains(completedAttFail.ErrorMessage, "auto_deploy") {
		t.Fatalf("Expected auto_deploy error message, got: %s", completedAttFail.ErrorMessage)
	}

	// Case B: Now provide verified live deployment on attempt 2
	attCode2 := &task.TaskAttempt{
		ID:            "att_qg_codefix_2",
		TaskID:        codeTask.ID,
		AttemptNumber: 2,
		Status:        task.AttemptStatusRunning,
		RunnerID:      "cuan-runner-1",
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attCode2); err != nil {
		t.Fatalf("Failed to create attempt 2: %v", err)
	}

	reqSuccess := runner.CompleteAttemptRequest{
		RunnerID:                "cuan-runner-1",
		Status:                  task.AttemptStatusSucceeded,
		ExitCode:                0,
		AgentReportConfirmed:    true,
		AgentReportRequired:     true,
		HasCodeChanges:          &hasCodeChanges,
		RequiresDeploy:          &requiresDeploy,
		DiffSummary:             diffSummary,
		CommitSHA:               commitSHA,
		LiveDeploymentConfirmed: true, // Deployed live and verified!
	}

	completedAttSuccess, err := runnerSvc.Complete(ctx, attCode2.ID, reqSuccess)
	if err != nil {
		t.Fatalf("CompleteAttempt unexpected error: %v", err)
	}
	if completedAttSuccess.Status != task.AttemptStatusSucceeded {
		t.Fatalf("Expected code-fix task to succeed when deployment is confirmed, got: %s (err: %s)",
			completedAttSuccess.Status, completedAttSuccess.ErrorMessage)
	}
}

// TestQualityGate_DualPath_InvestigationTaskCompletesWithoutDeploy tests Path (b):
// A task performing investigation / RCA / analysis without code changes MUST complete successfully
// as 'done' without being forced to trigger live deployment.
func TestQualityGate_DualPath_InvestigationTaskCompletesWithoutDeploy(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Create project with auto_deploy enabled
	proj := &project.Project{
		ID:        "proj_qg_investigation",
		Name:      "Cuan Analytics",
		RepoName:  "cuan-analytics",
		Metadata:  json.RawMessage(`{"auto_deploy":true}`),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.projectSvc.Store().CreateProject(ctx, proj); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// 2. Create Investigation Task (Read-Only / No Code)
	invTask := &task.Task{
		ID:                "task_qg_rca",
		ProjectID:         proj.ID,
		Title:             "Investigasi Pemadaman Pihak Ketiga dan Audit Log Transaksi",
		Instructions:      "Analisis log API gateway dan buat laporan RCA",
		Summary:           "RCA: Kegagalan transaksi disebabkan pemadaman sementara server gateway eksternal",
		CompleteNote:      "Laporan investigasi dan rekomendasi konfigurasi timeout selesai",
		ExecutionCategory: task.ExecutionCategoryReadOnly,
		Status:            task.TaskStatusDoing,
		BranchName:        "task/task_qg_rca",
	}
	if err := s.taskSvc.Store().CreateTask(ctx, invTask); err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	attInv := &task.TaskAttempt{
		ID:            "att_qg_rca",
		TaskID:        invTask.ID,
		AttemptNumber: 1,
		Status:        task.AttemptStatusRunning,
		RunnerID:      "cuan-runner-1",
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attInv); err != nil {
		t.Fatalf("Failed to create attempt: %v", err)
	}

	// Runner Service completion for investigation task:
	// RequiresDeploy is false, HasCodeChanges is false, LiveDeploymentConfirmed is false.
	runnerStore := runner.NewMemoryStore(s.taskSvc.Store())
	runnerSvc := runner.NewService(runnerStore, s.taskSvc.Store(), s.projectSvc)
	noCodeChanges := false
	noDeploy := false

	req := runner.CompleteAttemptRequest{
		RunnerID:                "cuan-runner-1",
		Status:                  task.AttemptStatusSucceeded,
		ExitCode:                0,
		AgentReportConfirmed:    true,
		AgentReportRequired:     true,
		HasCodeChanges:          &noCodeChanges,
		RequiresDeploy:          &noDeploy,
		DiffSummary:             "",
		CommitSHA:               "",
		LiveDeploymentConfirmed: false, // Investigation has NO live deploy!
	}

	completedAtt, err := runnerSvc.Complete(ctx, attInv.ID, req)
	if err != nil {
		t.Fatalf("CompleteAttempt unexpected error: %v", err)
	}

	// Investigation attempt MUST succeed without being rejected by auto_deploy gate
	if completedAtt.Status != task.AttemptStatusSucceeded {
		t.Fatalf("Expected investigation attempt to succeed without live deploy, got: %s (err: %s)",
			completedAtt.Status, completedAtt.ErrorMessage)
	}

	// Now verify server autoApplyAndDeployTask gracefully skips auto-deploy for investigation
	s.autoApplyAndDeployTask(ctx, invTask.ID, proj.ID)

	updatedTask, err := s.taskSvc.Store().GetTask(ctx, invTask.ID)
	if err != nil {
		t.Fatalf("Failed to get updated task: %v", err)
	}

	// Task must NOT have complete_note contaminated with fake auto-deploy note
	if strings.Contains(updatedTask.CompleteNote, "Otomatis memicu live deployment") {
		t.Fatalf("Investigation task must NOT contain auto-deploy note in CompleteNote, got: %s", updatedTask.CompleteNote)
	}
}
