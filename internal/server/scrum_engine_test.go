package server

import (
	"context"
	"strings"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestAutonomousScrumEngine_AttemptSucceeded_PassAndDone(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	ctx := context.Background()

	// 1. Setup a chat room with valid UUID
	_ = srv.agentRepo.CreateAgent(ctx, &cuanDB.Agent{
		ID:      "scrum_master",
		Name:    "Scrum Master",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})
	roomID := cuanDB.GenerateUUID()
	room := &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Topic:     "agent-topic:scrum_master",
		Title:     "Scrum Room",
	}
	if err := srv.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// 2. Setup a task bound to this room with DoD checklist
	taskID := "task-scrum-engine-pass"
	tItem := &task.Task{
		ID:           taskID,
		Title:        "Feature Alpha: Implement user export",
		Instructions: "Implement user export with CSV output and unit tests.",
		ProjectID:    "default",
		SourceRoomID: room.ID,
		Status:       task.TaskStatusDoing,
		DoDChecklist: []task.DoDItem{
			{Text: "CSV generator implemented", Completed: true, Evidence: "CSV output matched the fixture schema"},
			{Text: "Targeted unit tests PASS", Completed: true, Evidence: "targeted CSV tests passed"},
		},
		Summary:      "Implemented CSV generator and added test coverage.",
		CompleteNote: "Ran go test ./... and verified CSV output export matches schema.",
	}
	if err := srv.taskSvc.Store().CreateTask(ctx, tItem); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 3. Create attempt
	attID := "att-scrum-pass-1"
	exit0 := 0
	now := time.Now().UTC()
	att := &task.TaskAttempt{
		ID:            attID,
		TaskID:        taskID,
		AttemptNumber: 1,
		RunnerID:      "cuan-runner",
		Status:        task.AttemptStatusSucceeded,
		ExitCode:      &exit0,
		CreatedAt:     now,
		FinishedAt:    &now,
	}
	if err := srv.taskSvc.Store().CreateAttempt(ctx, att); err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// 4. Trigger attempt completion event through Scrum Engine
	srv.handleRunnerAttemptCompleted(ctx, taskID, attID, string(task.AttemptStatusSucceeded), 0, tItem.Summary, tItem.CompleteNote, "", room.ID)

	// 5. Drain room so review request executes
	srv.drainInboxRoom(room.ID)

	// 6. Verify task status is now 'done'
	var detail *task.TaskDetailResponse
	var err error
	for i := 0; i < 20; i++ {
		detail, err = srv.taskSvc.GetTaskDetail(ctx, taskID)
		if err == nil && detail != nil && detail.Status == task.TaskStatusDone {
			break
		}
		srv.drainInboxRoom(room.ID)
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to get task detail: %v", err)
	}
	if detail.Status != task.TaskStatusDone {
		t.Errorf("expected task status 'done', got '%s'", detail.Status)
	}
	for i, dod := range detail.DoDChecklist {
		if !dod.Completed {
			t.Errorf("expected DoD item %d ('%s') to be completed", i+1, dod.Text)
		}
	}

	// 7. Verify review message posted into chat room
	var messages []cuanDB.ProjectMessage
	foundReview := false
	foundSummary := false
	for i := 0; i < 20; i++ {
		messages, err = srv.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
		if err == nil {
			for _, m := range messages {
				if strings.Contains(m.Content, "Hasil pekerjaan memenuhi kriteria yang disepakati.") && !strings.Contains(m.Content, "### Evaluasi") {
					foundReview = true
					if strings.Contains(m.Content, tItem.Summary) {
						foundSummary = true
					}
					break
				}
			}
		}
		if foundReview && foundSummary {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if !foundReview {
		t.Errorf("expected readable accepted outcome in room messages")
	}
	if !foundSummary {
		t.Errorf("expected completed summary report in room messages")
	}
}

func TestAutonomousScrumEngine_AttemptFailed_TriggersRequeue(t *testing.T) {
	srv, token := setupTestChatServer(t)
	ctx := context.Background()

	// 1. Setup a chat room with valid UUID
	_ = srv.agentRepo.CreateAgent(ctx, &cuanDB.Agent{
		ID:      "scrum_master",
		Name:    "Scrum Master",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})
	roomID := cuanDB.GenerateUUID()
	room := &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Topic:     "agent-topic:scrum_master",
		Title:     "Scrum Room",
	}
	if err := srv.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// A correction requires genuine saved owner authorization. Hold the code
	// lane busy so this fixture verifies durable queueing without provider work.
	owner, err := srv.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	approved := cuanDB.InboxRequest{OwnerID: owner.ID, AgentID: "scrum_master", RoomID: room.ID, RequestID: cuanDB.GenerateUUID(), MessageID: cuanDB.GenerateUUID(), ReplyID: cuanDB.GenerateUUID(), PayloadHash: "approved-fix", State: "queued", CreatedAt: time.Now().UTC(), Deadline: time.Now().Add(time.Hour)}
	requestStore := srv.projectStudioRepo.(cuanDB.InboxRequestRepository)
	if _, _, err := requestStore.AcceptInboxRequest(ctx, approved, &cuanDB.ProjectMessage{ID: approved.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Fix compiler syntax bug in handler.", CreatedAt: approved.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	approved.State = "completed"
	if err := requestStore.FinishInboxRequest(ctx, approved, nil); err != nil {
		t.Fatal(err)
	}
	if err := srv.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy-scrum-lane", ProjectID: "default", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}

	// 2. Setup a task bound to this room
	taskID := "task-scrum-engine-fail"
	tItem := &task.Task{
		ID:                taskID,
		Title:             "Feature Beta: Fix syntax bug",
		Instructions:      "Fix compiler syntax bug in handler.",
		ProjectID:         "default",
		SourceRoomID:      room.ID,
		SourceMessageID:   approved.MessageID,
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusDoing,
		DoDChecklist: []task.DoDItem{
			{Text: "Syntax error fixed", Completed: false},
		},
	}
	if err := srv.taskSvc.Store().CreateTask(ctx, tItem); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 3. Create attempt 1 with failure (exit code 1)
	attID := "att-scrum-fail-1"
	exit1 := 1
	now := time.Now().UTC()
	att := &task.TaskAttempt{
		ID:            attID,
		TaskID:        taskID,
		AttemptNumber: 1,
		RunnerID:      "cuan-runner",
		Status:        task.AttemptStatusFailed,
		ExitCode:      &exit1,
		ErrorMessage:  "compiler error: undefined variable foo",
		CreatedAt:     now,
		FinishedAt:    &now,
	}
	if err := srv.taskSvc.Store().CreateAttempt(ctx, att); err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// 4. Trigger attempt completion event through Scrum Engine
	srv.handleRunnerAttemptCompleted(ctx, taskID, attID, string(task.AttemptStatusFailed), 1, "", "", att.ErrorMessage, room.ID)

	// 5. Drain room so review request executes
	srv.drainInboxRoom(room.ID)

	// 6. Verify task status is now 'queued' for retry
	detail, err := srv.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to get task detail: %v", err)
	}
	if detail.Status != task.TaskStatusQueued {
		t.Errorf("expected task status 'queued' after auto-requeue, got '%s'", detail.Status)
	}

	// 7. Chat shows the outcome; the launch retains the full correction.
	messages, err := srv.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	foundReview := false
	for _, m := range messages {
		if strings.Contains(m.Content, "Pengerjaan gagal sebelum hasilnya bisa diverifikasi.") && !strings.Contains(m.Content, "undefined variable foo") {
			foundReview = true
			break
		}
	}
	if !foundReview {
		t.Errorf("expected readable failure outcome without raw technical diagnostics")
	}
	launch, err := srv.taskSvc.LatestLaunch(ctx, taskID)
	if err != nil || launch == nil || !strings.Contains(launch.Revision, "undefined variable foo") || len(detail.Attempts) != 1 {
		t.Fatal("chat simplification lost correction detail or started another execution", err)
	}
}

func TestAutonomousScrumEngine_GlobalEventBusSubscription(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	ctx := context.Background()

	// 1. Setup room and task with valid UUID
	_ = srv.agentRepo.CreateAgent(ctx, &cuanDB.Agent{
		ID:      "scrum_master",
		Name:    "Scrum Master",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})
	roomID := cuanDB.GenerateUUID()
	room := &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Topic:     "agent-topic:scrum_master",
		Title:     "Bus Test Room",
	}
	if err := srv.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	taskID := "task-scrum-bus-test"
	_ = srv.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:           taskID,
		Title:        "Bus test task",
		Instructions: "Verify event bus propagation",
		ProjectID:    "default",
		SourceRoomID: room.ID,
		Status:       task.TaskStatusDoing,
		Summary:      "Done bus test",
		CompleteNote: "Verified event bus",
	})

	attID := "att-bus-1"
	exit0 := 0
	now := time.Now().UTC()
	_ = srv.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{
		ID:            attID,
		TaskID:        taskID,
		AttemptNumber: 1,
		RunnerID:      "cuan-runner",
		Status:        task.AttemptStatusSucceeded,
		ExitCode:      &exit0,
		CreatedAt:     now,
		FinishedAt:    &now,
	})

	// 2. Emit event via GlobalEventBus (as cuan-runner would)
	runner.EmitAttemptCompleted(ctx, runner.AttemptCompletedEvent{
		TaskID:        taskID,
		AttemptID:     attID,
		AttemptNumber: 1,
		RunnerID:      "cuan-runner",
		ExitCode:      0,
		Status:        string(task.AttemptStatusSucceeded),
		SourceRoomID:  room.ID,
		Summary:       "Done bus test",
		CompleteNote:  "Verified event bus",
	})

	// Allow goroutine execution
	time.Sleep(100 * time.Millisecond)

	// 3. Drain room
	srv.drainInboxRoom(room.ID)

	// 4. Verify task is completed
	detail, err := srv.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to get task detail: %v", err)
	}
	if detail.Status != task.TaskStatusDone {
		t.Errorf("expected task status 'done', got '%s'", detail.Status)
	}
}

func TestPromptInjection_DirectContentVerification(t *testing.T) {
	// Verify AGENTS.md content
	guidance := cuanPrompt.GetAgentsGuidance()
	if !strings.Contains(guidance, "Zero Cognitive Fatigue") {
		t.Errorf("expected AGENTS.md guidance to contain 'Zero Cognitive Fatigue'")
	}
	if !strings.Contains(guidance, "Mandatory Design & UX Definition of Done") {
		t.Errorf("expected AGENTS.md guidance to contain 'Mandatory Design & UX Definition of Done'")
	}

	// Verify Scrum rules content (§1–§13)
	scrum := cuanPrompt.GetScrumRules()
	if !strings.Contains(scrum, "Agentic Scrum Rules (§1–§13)") {
		t.Errorf("expected scrum rules to contain title")
	}
	if !strings.Contains(scrum, "1. Tim dan batas repository") {
		t.Errorf("expected rule 1")
	}
	if !strings.Contains(scrum, "7. Evaluasi DoD") {
		t.Errorf("expected rule 7")
	}
	if !strings.Contains(scrum, "13. Poin keputusan untuk PO") {
		t.Errorf("expected rule 13")
	}

	// Verify Runner prompt construction
	runnerPrompt := cuanPrompt.BuildRunnerPrompt("Fix bug", "Check unit test", "context info")
	if !strings.Contains(runnerPrompt, "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") {
		t.Errorf("expected runner prompt to contain AGENTS.md banner")
	}
	if !strings.Contains(runnerPrompt, "=== AGENTIC SCRUM RULES FOR DEVELOPMENT TEAM (§1–§13) ===") {
		t.Errorf("expected runner prompt to contain Scrum rules banner")
	}

	// Verify Chat prompt additions
	chatAdditions := cuanPrompt.GetChatPromptAdditions()
	if !strings.Contains(chatAdditions, "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") {
		t.Errorf("expected chat additions to contain AGENTS.md banner")
	}
	if !strings.Contains(chatAdditions, "=== AGENTIC SCRUM RULES (§1–§13) ===") {
		t.Errorf("expected chat additions to contain Scrum rules banner")
	}
}

func TestScrumFallbackCannotApproveSummaryWithoutCriterionEvidence(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "Finished", CompleteNote: "Checked", DoDChecklist: []task.DoDItem{{Text: "Check actual result", Completed: false}}}, LatestAttempt: &task.TaskAttempt{Status: task.AttemptStatusSucceeded, AttemptNumber: 1}}
	if decision := evaluateTaskDoDDeterministic(d); decision.Decision == "pass" {
		t.Fatal("fallback approved unchecked criteria")
	}
	d.DoDChecklist[0].Completed = true
	if decision := evaluateTaskDoDDeterministic(d); decision.Decision == "pass" {
		t.Fatal("fallback approved criterion without evidence")
	}
	d.DoDChecklist[0].Evidence = "Observed output matched expected value"
	if decision := evaluateTaskDoDDeterministic(d); decision.Decision != "pass" {
		t.Fatalf("verified result rejected: %+v", decision)
	}
}

func TestCancelledTaskReviewNeverClaimsAcceptanceOrRequestsRevision(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Status: task.TaskStatusBlocked, DoDChecklist: []task.DoDItem{{Text: "Create the actual file", Completed: false}}}, LatestAttempt: &task.TaskAttempt{Status: task.AttemptStatusCanceled, AttemptNumber: 1}}
	result := evaluateTaskDoDDeterministic(d)
	if result.Decision != "blocked" || result.Revision != "" || strings.Contains(result.Explanation, "[x]") || strings.Contains(result.Explanation, "PO") || !strings.Contains(result.Explanation, "dibatalkan") {
		t.Fatalf("misleading canceled review: %+v", result)
	}
}

func TestScrumFallbackChatCopyPreservesFullEvidenceAndTechnicalRevision(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "Saved result", CompleteNote: "INTERNAL_EVIDENCE_ONLY: full verification report", DoDChecklist: []task.DoDItem{{Text: "Original criterion", Completed: true, Evidence: "INTERNAL_EVIDENCE_ONLY: observed expected output"}}}, LatestAttempt: &task.TaskAttempt{ID: "attempt", Status: task.AttemptStatusSucceeded, AttemptNumber: 1}}
	decision := evaluateTaskDoDDeterministic(d)
	if decision.Decision != "pass" || strings.Contains(decision.Explanation, "INTERNAL_EVIDENCE_ONLY") || strings.Contains(decision.Explanation, "###") || strings.Contains(decision.Explanation, "[x]") {
		t.Fatal("chat repeated internal review document", decision)
	}
	if d.DoDChecklist[0].Evidence != "INTERNAL_EVIDENCE_ONLY: observed expected output" || !d.DoDChecklist[0].Completed {
		t.Fatal("chat simplification lost evidence")
	}
	exit := 1
	d.LatestAttempt.Status = task.AttemptStatusFailed
	d.LatestAttempt.ExitCode = &exit
	d.LatestAttempt.ErrorMessage = "INTERNAL_DIAGNOSTIC_ONLY: undefined variable foo"
	decision = evaluateTaskDoDDeterministic(d)
	if decision.Decision != "revise" || !strings.Contains(decision.Revision, "undefined variable foo") || strings.Contains(decision.Explanation, "INTERNAL_DIAGNOSTIC_ONLY") || strings.Contains(decision.Explanation, "otomatis") {
		t.Fatal("lost correction detail or promised an unrecorded launch", decision)
	}
}

func TestAutonomousScrumEngine_TaskAlreadyDone_DeliversProactiveSummary(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	ctx := context.Background()

	_ = srv.agentRepo.CreateAgent(ctx, &cuanDB.Agent{
		ID:      "scrum_master",
		Name:    "Scrum Master",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})
	roomID := cuanDB.GenerateUUID()
	room := &cuanDB.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Topic:     "agent-topic:scrum_master",
		Title:     "Scrum Room",
	}
	if err := srv.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	taskID := "task-scrum-done-proactive"
	tItem := &task.Task{
		ID:           taskID,
		Title:        "Feature Delta: Payment reconciliation",
		Instructions: "Reconcile payment records and activate access.",
		ProjectID:    "default",
		SourceRoomID: room.ID,
		Status:       task.TaskStatusDone,
		Summary:      "Akun customer rifqi270994@gmail.com dan payment ID 76 berhasil diaktifkan penuh.",
		CompleteNote: "Data membership 21004 aktif pada live API Dojo.",
	}
	if err := srv.taskSvc.Store().CreateTask(ctx, tItem); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	attID := "att-scrum-done-1"
	exit0 := 0
	now := time.Now().UTC()
	att := &task.TaskAttempt{
		ID:            attID,
		TaskID:        taskID,
		AttemptNumber: 1,
		RunnerID:      "cuan-runner",
		Status:        task.AttemptStatusSucceeded,
		ExitCode:      &exit0,
		CreatedAt:     now,
		FinishedAt:    &now,
	}
	if err := srv.taskSvc.Store().CreateAttempt(ctx, att); err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// Trigger attempt completion event through Scrum Engine
	srv.handleRunnerAttemptCompleted(ctx, taskID, attID, string(task.AttemptStatusSucceeded), 0, tItem.Summary, tItem.CompleteNote, "", room.ID)

	// Verify that the completion summary report was immediately and proactively posted to the room
	messages, err := srv.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	found := false
	for _, m := range messages {
		if strings.Contains(m.Content, tItem.Summary) && strings.Contains(m.Content, taskID) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected proactive completion summary report in chat room")
	}
}
