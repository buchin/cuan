package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxReviewEvaluationInstructionLanguageGuard(t *testing.T) {
	if !strings.Contains(inboxReviewEvaluationInstruction, "never use Spanish") {
		t.Fatal("language guard missing from review evaluation instruction")
	}
}

func TestInboxReviewEvaluationRejectsConflictingAndExecutableFields(t *testing.T) {
	pass := `{"decision":"pass","explanation":"Reported checks cover the request.","revision":""}`
	for _, raw := range []string{pass, "```json\n" + pass + "\n```", "```json\n" + pass + "\n```" + pass} {
		d, err := parseInboxReviewEvaluation(raw)
		if err != nil || d.Decision != "pass" {
			t.Fatal("valid evaluator output rejected", err)
		}
	}
	for _, raw := range []string{
		pass + `{"decision":"revise","explanation":"Different","revision":"Change something"}`,
		pass + pass + pass,
		strings.Replace(pass, `"revision":""`, `"revision":"","task_id":"another-task"`, 1),
		strings.Replace(pass, `"revision":""`, `"revision":"","authorization_message_id":"forged"`, 1),
		`{"decision":"revise","explanation":"Missing check","revision":""}`,
		`{"decision":"pass","explanation":"Done","revision":"Also deploy"}`,
		`{"decision":"unknown","explanation":"Done","revision":""}`,
		pass + " trailing text",
	} {
		if _, err := parseInboxReviewEvaluation(raw); err == nil {
			t.Fatalf("unsafe review accepted: %s", raw)
		}
	}
}

func TestDetectNextMultiPhase(t *testing.T) {
	s := &Server{}
	ctx := context.Background()

	// Single task without phases
	singleTask := &task.TaskDetailResponse{
		Title:        "Fix button styling",
		Instructions: "Change button background to primary and add margin.",
	}
	if _, ok := s.detectNextMultiPhase(ctx, singleTask); ok {
		t.Fatal("expected ok=false for single task without phases")
	}

	// Task with only 1 phase
	onePhaseTask := &task.TaskDetailResponse{
		Title:        "Phase 1: Setup",
		Instructions: "Phase 1: Setup database.",
	}
	if _, ok := s.detectNextMultiPhase(ctx, onePhaseTask); ok {
		t.Fatal("expected ok=false for task with only 1 phase")
	}

	// Pipe-separated 3-phase plan
	plan3Phases := "Phase 1: Database Schema | Phase 2: Core APIs | Phase 3: Web UI"

	// Current Phase 1 -> Next should be Phase 2
	p1Task := &task.TaskDetailResponse{
		Title:        "Phase 1: Database Schema",
		Instructions: plan3Phases,
	}
	target, ok := s.detectNextMultiPhase(ctx, p1Task)
	if !ok {
		t.Fatal("expected ok=true for Phase 1 of 3-phase plan")
	}
	if target.CurrentPhase != 1 || target.NextPhase != 2 || target.NextTitle != "Core APIs" || target.TotalPhases != 3 {
		t.Fatalf("unexpected target: %+v", target)
	}

	// Current Phase 2 -> Next should be Phase 3
	p2Task := &task.TaskDetailResponse{
		Title:        "Phase 2: Core APIs",
		Instructions: plan3Phases,
	}
	target, ok = s.detectNextMultiPhase(ctx, p2Task)
	if !ok {
		t.Fatal("expected ok=true for Phase 2 of 3-phase plan")
	}
	if target.CurrentPhase != 2 || target.NextPhase != 3 || target.NextTitle != "Web UI" {
		t.Fatalf("unexpected target: %+v", target)
	}

	// Current Phase 3 -> Should be done (no next phase)
	p3Task := &task.TaskDetailResponse{
		Title:        "Phase 3: Web UI",
		Instructions: plan3Phases,
	}
	if _, ok = s.detectNextMultiPhase(ctx, p3Task); ok {
		t.Fatal("expected ok=false for final Phase 3 of 3-phase plan")
	}

	// Indonesian "Fase" format with newlines
	faseIndo := "Fase 1: Migrasi Database\nFase 2: Endpoint REST\nFase 3: Antarmuka Pengguna"
	fase1Task := &task.TaskDetailResponse{
		Title:        "Fase 1: Migrasi Database",
		Instructions: faseIndo,
	}
	target, ok = s.detectNextMultiPhase(ctx, fase1Task)
	if !ok {
		t.Fatal("expected ok=true for Fase 1 in Indonesian format")
	}
	if target.CurrentPhase != 1 || target.NextPhase != 2 || target.NextTitle != "Endpoint REST" {
		t.Fatalf("unexpected target: %+v", target)
	}

	// Untagged title but with Active Phase in instructions
	activePhaseTask := &task.TaskDetailResponse{
		Title:        "Building Authentication System",
		Instructions: "Active Phase: 2\n" + plan3Phases,
	}
	target, ok = s.detectNextMultiPhase(ctx, activePhaseTask)
	if !ok {
		t.Fatal("expected ok=true for task with Active Phase in instructions")
	}
	if target.CurrentPhase != 2 || target.NextPhase != 3 || target.NextTitle != "Web UI" {
		t.Fatalf("unexpected target: %+v", target)
	}

	// Single task that happens to mention bullet point phases in instructions without active phase tag
	bulletPhaseTask := &task.TaskDetailResponse{
		Title:        "Implementasi Tool MCP GrantUserAccess dan CLI di Project Dojo",
		Instructions: "Scope:\n- Phase 1: Tambahkan Tool\n- Phase 2: Sediakan CLI\n- Phase 3: Deploy",
	}
	if _, ok = s.detectNextMultiPhase(ctx, bulletPhaseTask); ok {
		t.Fatal("expected ok=false for single task with untagged bullet phases")
	}
}

func TestApplyInboxReviewEvaluationMultiPhaseAutoAdvancement(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Setup conversation room
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")

	// 2. Setup user approval message
	userMsgID := db.GenerateUUID()
	approvedPayload, _ := json.Marshal(attachmentMessage{Approval: &approvedInboxPlan{Action: "approve", MessageID: db.GenerateUUID(), Plan: inboxApprovalPlan{Title: "Phase 1: Database Setup", Description: "Phase 1: Database Setup | Phase 2: API Endpoints | Phase 3: Frontend View", ProjectID: "default"}}})
	userMsg := &db.ProjectMessage{
		ID:         userMsgID,
		RoomID:     room.RoomID,
		AuthorType: "user",
		AuthorID:   "owner",
		Content:    attachmentEnvelope + string(approvedPayload),
		CreatedAt:  time.Now().UTC().Add(-time.Minute),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatal(err)
	}

	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		now := time.Now().UTC()
		_, _, _ = store.AcceptInboxRequest(ctx, db.InboxRequest{
			OwnerID:   "owner",
			AgentID:   "agent_lifecycle",
			RoomID:    room.RoomID,
			RequestID: "req_" + userMsgID,
			MessageID: userMsgID,
			State:     "completed",
			CreatedAt: now,
			Deadline:  now.Add(time.Hour),
		}, userMsg)
	}

	// 3. Create Phase 1 task with multi-phase instructions
	plan3Phases := "Cuan managed delegation v1.\nPhase 1: Database Setup | Phase 2: API Endpoints | Phase 3: Frontend View\n\nApproval message: " + userMsgID
	task1, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", userMsgID, task.CreateTaskRequest{
		SourceRoomID:  room.RoomID,
		ProjectID:     "default",
		Title:         "Phase 1: Database Setup",
		Instructions:  plan3Phases,
		AssignedAgent: "agent_lifecycle",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 4. Create attempt and complete Phase 1 task
	att, err := s.taskSvc.CreateAttempt(ctx, task1.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatal(err)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusSucceeded,
	})

	reviewStatus := task.TaskStatusReview
	_, err = s.taskSvc.UpdateTask(ctx, task1.ID, "agent_lifecycle", task.UpdateTaskRequest{
		Status: &reviewStatus,
	})
	if err != nil {
		t.Fatal(err)
	}

	doneStatus := task.TaskStatusDone
	summary := "Database tables created successfully."
	completeNote := "Tested migration and verification scripts."
	_, err = s.taskSvc.UpdateTask(ctx, task1.ID, "agent_lifecycle", task.UpdateTaskRequest{
		Status:       &doneStatus,
		Summary:      &summary,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 5. Construct review request matching canonical review key
	key := "task-review:" + att.ID
	reqID := delegationMessageID(key)
	inputMsgID := delegationMessageID(key + ":input")
	replyMsgID := delegationMessageID(key + ":reply")

	reviewMsg := &db.ProjectMessage{
		ID:         inputMsgID,
		RoomID:     room.RoomID,
		AuthorType: "system",
		AuthorID:   "cuan_work_review",
		Content:    "Review result for [/tasks/" + task1.ID + "]",
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, reviewMsg); err != nil {
		t.Fatal(err)
	}

	nowAuto := time.Now().UTC()
	q := db.InboxRequest{
		OwnerID:   "owner",
		AgentID:   "agent_lifecycle",
		RoomID:    room.RoomID,
		RequestID: reqID,
		MessageID: inputMsgID,
		ReplyID:   replyMsgID,
		CreatedAt: nowAuto,
		Deadline:  nowAuto.Add(10 * time.Minute),
	}

	// 6. Apply passing review evaluation
	evalJSON := `{"decision":"pass","explanation":"Phase 1 verification passed completely.","revision":""}`
	result := s.applyInboxReviewEvaluation(ctx, q, evalJSON)

	// 7. Verify result text mentions Phase 1 completion and auto-advance to Phase 2
	if !strings.Contains(result, "Phase 1 verified complete") {
		t.Fatalf("result missing Phase 1 completion text: %s", result)
	}
	if !strings.Contains(result, "Phase 2: API Endpoints") {
		t.Fatalf("result missing Phase 2 auto-advance text: %s", result)
	}

	// 8. Verify auto-delegation system message was recorded in the room
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	foundAutoMsg := false
	for _, m := range messages {
		if m.AuthorType == "system" && m.AuthorID == "cuan_auto_delegation" {
			foundAutoMsg = true
			if !strings.Contains(m.Content, "Phase 2: API Endpoints") {
				t.Fatalf("auto-delegation message content missing Phase 2: %s", m.Content)
			}
		}
	}
	if !foundAutoMsg {
		t.Fatal("expected cuan_auto_delegation system message in room")
	}

	// 9. Verify Phase 2 task was created
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room.RoomID})
	if err != nil {
		t.Fatal(err)
	}
	foundP2Task := false
	for _, tItem := range tasks {
		if strings.Contains(tItem.Title, "Phase 2: API Endpoints") {
			foundP2Task = true
		}
	}
	if !foundP2Task {
		t.Fatal("expected Phase 2 task to be created in task service")
	}
}

func TestApplyInboxReviewEvaluation_RevisionIncludesExplanation(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")

	userMsgID := db.GenerateUUID()
	now := time.Now().UTC()
	userMsg := &db.ProjectMessage{
		ID:         userMsgID,
		RoomID:     room.RoomID,
		AuthorType: "user",
		AuthorID:   "owner",
		Content:    "Fix navigation layout",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatal(err)
	}

	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		_, _, _ = store.AcceptInboxRequest(ctx, db.InboxRequest{
			OwnerID:   "owner",
			AgentID:   "agent_lifecycle",
			RoomID:    room.RoomID,
			RequestID: "req_" + userMsgID,
			MessageID: userMsgID,
			State:     "completed",
			CreatedAt: now.Add(-time.Minute),
			Deadline:  now.Add(time.Hour),
		}, userMsg)
	}

	task1, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", userMsgID, task.CreateTaskRequest{
		SourceRoomID:  room.RoomID,
		ProjectID:     "default",
		Title:         "Fix navigation layout",
		Instructions:  "Cuan managed delegation v1.\nFix navbar items.\n\nApproval message: " + userMsgID,
		AssignedAgent: "agent_lifecycle",
		DoDChecklist: []task.DoDItem{
			{Text: "Fix navbar on tasks page", Completed: true},
			{Text: "Eliminate navbar search on other pages", Completed: false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	att, err := s.taskSvc.CreateAttempt(ctx, task1.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatal(err)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusSucceeded,
	})

	key := "task-review:" + att.ID
	reqID := delegationMessageID(key)
	inputMsgID := delegationMessageID(key + ":input")
	replyMsgID := delegationMessageID(key + ":reply")

	reviewMsg := &db.ProjectMessage{
		ID:         inputMsgID,
		RoomID:     room.RoomID,
		AuthorType: "system",
		AuthorID:   "cuan_work_review",
		Content:    "Review result for [/tasks/" + task1.ID + "]",
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, reviewMsg); err != nil {
		t.Fatal(err)
	}

	nowAuto := time.Now().UTC()
	q := db.InboxRequest{
		OwnerID:   "owner",
		AgentID:   "agent_lifecycle",
		RoomID:    room.RoomID,
		RequestID: reqID,
		MessageID: inputMsgID,
		ReplyID:   replyMsgID,
		CreatedAt: nowAuto,
		Deadline:  nowAuto.Add(10 * time.Minute),
	}

	// Keep the actual correction durably queued. A stopped/failed launch must
	// instead show its own outcome, without promising that a correction is active.
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "revision-explanation-busy", ProjectID: "default", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	evalJSON := `{"decision":"revise","explanation":"### Evaluasi Kriteria Selesai (DoD)\n**Status:** BELUM LULUS\n\n- [ ] **Eliminasi navbar search**: Belum selesai di halaman Inbox.","revision":"Gap: eliminate search in Inbox.\n- [ ] Eliminate search in Inbox"}`
	result := s.applyInboxReviewEvaluation(ctx, q, evalJSON)
	t.Logf("applyInboxReviewEvaluation result: %s", result)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(task1.ID, q.RequestID))
	if err != nil || launch.State != "queued" {
		t.Fatal("fixture did not queue the claimed correction", err)
	}

	if !strings.Contains(result, "Evaluasi Kriteria Selesai (DoD)") {
		t.Fatalf("expected result to include structured DoD explanation, got: %s", result)
	}
	if !strings.Contains(result, "Eliminasi navbar search") {
		t.Fatalf("expected result to include gap item, got: %s", result)
	}

	// Verify revision feedback did not expand the approved acceptance checklist
	updatedTask, err := s.taskSvc.GetTaskDetail(ctx, task1.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundNewDoD := false
	for _, item := range updatedTask.DoDChecklist {
		if strings.Contains(item.Text, "Eliminate search in Inbox") {
			foundNewDoD = true
			break
		}
	}
	if foundNewDoD || len(updatedTask.DoDChecklist) != 2 {
		t.Fatalf("revision expanded original acceptance criteria: %+v", updatedTask.DoDChecklist)
	}
}

func TestApplyInboxReviewEvaluation_CodePassNeedsVerifierAndAppendsLink(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()

	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")

	userMsgID := db.GenerateUUID()
	now := time.Now().UTC()
	userMsg := &db.ProjectMessage{
		ID:         userMsgID,
		RoomID:     room.RoomID,
		AuthorType: "user",
		AuthorID:   "owner",
		Content:    "Build contact form",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatal(err)
	}

	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		_, _, _ = store.AcceptInboxRequest(ctx, db.InboxRequest{
			OwnerID:   "owner",
			AgentID:   "agent_lifecycle",
			RoomID:    room.RoomID,
			RequestID: "req_" + userMsgID,
			MessageID: userMsgID,
			State:     "completed",
			CreatedAt: now.Add(-time.Minute),
			Deadline:  now.Add(time.Hour),
		}, userMsg)
	}

	task1, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", userMsgID, task.CreateTaskRequest{
		SourceRoomID:  room.RoomID,
		ProjectID:     "default",
		Title:         "Build contact form",
		Instructions:  "Cuan managed delegation v1.\nBuild form.\n\nApproval message: " + userMsgID,
		AssignedAgent: "agent_lifecycle",
		DoDChecklist: []task.DoDItem{
			{Text: "Input validation", Completed: true, Evidence: "Invalid input rejected in form fixture"},
			{Text: "Submit button", Completed: true, Evidence: "Submit event captured by fixture"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	att, err := s.taskSvc.CreateAttempt(ctx, task1.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatal(err)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusSucceeded,
	})

	reviewStatus := task.TaskStatusReview
	summary := "Contact form completed."
	completeNote := "Tested inputs and validation."
	_, err = s.taskSvc.UpdateTask(ctx, task1.ID, "agent_lifecycle", task.UpdateTaskRequest{
		Status:       &reviewStatus,
		Summary:      &summary,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatal(err)
	}

	key := "task-review:" + att.ID
	reqID := delegationMessageID(key)
	inputMsgID := delegationMessageID(key + ":input")
	replyMsgID := delegationMessageID(key + ":reply")

	reviewMsg := &db.ProjectMessage{
		ID:         inputMsgID,
		RoomID:     room.RoomID,
		AuthorType: "system",
		AuthorID:   "cuan_work_review",
		Content:    "Review result for [/tasks/" + task1.ID + "]",
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, reviewMsg); err != nil {
		t.Fatal(err)
	}

	q := db.InboxRequest{
		OwnerID:   "owner",
		AgentID:   "agent_lifecycle",
		RoomID:    room.RoomID,
		RequestID: reqID,
		MessageID: inputMsgID,
		ReplyID:   replyMsgID,
		CreatedAt: now,
		Deadline:  now.Add(10 * time.Minute),
	}

	evalJSON := `{"decision":"pass","explanation":"### Evaluasi Kriteria Selesai (DoD)\n**Status:** LULUS\n\n- [x] Input validation diverifikasi.\n- [x] Submit button diverifikasi.","revision":""}`
	result := s.applyInboxReviewEvaluation(ctx, q, evalJSON)

	if !strings.Contains(result, "sesi pemeriksaan terpisah") {
		t.Fatalf("expected missing verifier explanation, got: %s", result)
	}
	if !strings.Contains(result, "/tasks/"+task1.ID) {
		t.Fatalf("expected result to include task link /tasks/%s, got: %s", task1.ID, result)
	}

	// A model's pass and completed developer checklist cannot replace verification.
	updatedTask, err := s.taskSvc.GetTaskDetail(ctx, task1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTask.Status == task.TaskStatusDone {
		t.Fatal("code accepted without independent verifier")
	}
	for _, item := range updatedTask.DoDChecklist {
		if !item.Completed {
			t.Fatalf("expected all DoD items to be completed, got item uncompleted: %+v", item)
		}
	}
}

func TestApplyInboxReviewEvaluation_FallbackTextReview(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()

	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")

	userMsgID := db.GenerateUUID()
	now := time.Now().UTC()
	userMsg := &db.ProjectMessage{
		ID:         userMsgID,
		RoomID:     room.RoomID,
		AuthorType: "user",
		AuthorID:   "owner",
		Content:    "Build footer layout",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, userMsg); err != nil {
		t.Fatal(err)
	}

	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		_, _, _ = store.AcceptInboxRequest(ctx, db.InboxRequest{
			OwnerID:   "owner",
			AgentID:   "agent_lifecycle",
			RoomID:    room.RoomID,
			RequestID: "req_" + userMsgID,
			MessageID: userMsgID,
			State:     "completed",
			CreatedAt: now.Add(-time.Minute),
			Deadline:  now.Add(time.Hour),
		}, userMsg)
	}

	task1, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", userMsgID, task.CreateTaskRequest{
		SourceRoomID:  room.RoomID,
		ProjectID:     "default",
		Title:         "Build footer layout",
		Instructions:  "Cuan managed delegation v1.\nBuild footer.\n\nApproval message: " + userMsgID,
		AssignedAgent: "agent_lifecycle",
		DoDChecklist: []task.DoDItem{
			{Text: "Footer links", Completed: true, Evidence: "Footer hrefs matched expected destinations"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	att, err := s.taskSvc.CreateAttempt(ctx, task1.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_lifecycle",
	})
	if err != nil {
		t.Fatal(err)
	}
	s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{
		Status: task.AttemptStatusSucceeded,
	})

	reviewStatus := task.TaskStatusReview
	summary := "Footer done."
	completeNote := "Checked footer links."
	_, err = s.taskSvc.UpdateTask(ctx, task1.ID, "agent_lifecycle", task.UpdateTaskRequest{
		Status:       &reviewStatus,
		Summary:      &summary,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatal(err)
	}

	key := "task-review:" + att.ID
	reqID := delegationMessageID(key)
	inputMsgID := delegationMessageID(key + ":input")
	replyMsgID := delegationMessageID(key + ":reply")

	reviewMsg := &db.ProjectMessage{
		ID:         inputMsgID,
		RoomID:     room.RoomID,
		AuthorType: "system",
		AuthorID:   "cuan_work_review",
		Content:    "Review result for [/tasks/" + task1.ID + "]",
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, reviewMsg); err != nil {
		t.Fatal(err)
	}

	q := db.InboxRequest{
		OwnerID:   "owner",
		AgentID:   "agent_lifecycle",
		RoomID:    room.RoomID,
		RequestID: reqID,
		MessageID: inputMsgID,
		ReplyID:   replyMsgID,
		CreatedAt: now,
		Deadline:  now.Add(10 * time.Minute),
	}

	// Plain text without JSON (fallback text review)
	plainText := "### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** LULUS\n\n**Bukti yang Terverifikasi:**\n- [x] Footer links telah diuji.\n\n**Tindakan:** Menyatakan selesai."
	result := s.applyInboxReviewEvaluation(ctx, q, plainText)

	if !strings.Contains(result, "sesi pemeriksaan terpisah") {
		t.Fatalf("fallback pass bypassed verifier gate: %s", result)
	}
	if !strings.Contains(result, "/tasks/"+task1.ID) {
		t.Fatalf("expected result to include task link, got: %s", result)
	}

	// Legacy plain-text acceptance must enforce the same independent gate.
	updatedTask, err := s.taskSvc.GetTaskDetail(ctx, task1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTask.Status == task.TaskStatusDone {
		t.Fatal("fallback text accepted code without independent verifier")
	}
}

func TestParseFallbackTextReview_NoRawJSONLeak(t *testing.T) {
	// Simulated raw output that contains two JSONs with minor difference
	raw := "{\n  \"decision\": \"revise\",\n  \"explanation\": \"### Evaluasi Kriteria Selesai\\n**Status:** BELUM LULUS\\n- [ ] Gap 1\",\n  \"revision\": \"Fix gap 1\"\n}\n{\"decision\":\"revise\",\"explanation\":\"### Evaluasi Kriteria Selesai\\n**Status:** BELUM LULUS,\\n- [ ] Gap 1\",\"revision\":\"Fix gap 1\"}"

	eval, err := parseFallbackTextReview(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Decision != "revise" {
		t.Errorf("expected decision revise, got %s", eval.Decision)
	}
	// Verify raw JSON envelope is NOT present in Explanation
	if strings.HasPrefix(eval.Explanation, "{") || strings.Contains(eval.Explanation, `"decision":`) || strings.Contains(eval.Explanation, `"explanation":`) {
		t.Errorf("expected clean explanation without raw JSON envelopes, got: %s", eval.Explanation)
	}
	if !strings.Contains(eval.Explanation, "Evaluasi Kriteria Selesai") {
		t.Errorf("expected explanation to contain markdown text, got: %s", eval.Explanation)
	}
}

func TestParseInboxReviewEvaluation_PassNormalizesWaitAndNeedsInput(t *testing.T) {
	raw := `{"decision":"pass","explanation":"All DoD passed. Draft reply provided.","finding":"All DoD passed.","revision":"","manager_action":"wait","manager_reason":"Waiting for customer response","remaining_work":"external","requested_outcome":"needs_input"}`
	eval, err := parseInboxReviewEvaluation(raw)
	if err != nil {
		t.Fatalf("unexpected error parsing pass evaluation with manager_action: %v", err)
	}
	if eval.Decision != "pass" {
		t.Fatalf("expected decision pass, got %s", eval.Decision)
	}
	if eval.ManagerAction != "" || eval.ManagerReason != "" {
		t.Fatalf("expected manager action to be cleared on pass, got %s", eval.ManagerAction)
	}
	if eval.RemainingWork != "none" {
		t.Fatalf("expected remaining work none on pass, got %s", eval.RemainingWork)
	}
	if eval.RequestedOutcome != "completed" {
		t.Fatalf("expected requested outcome completed on pass, got %s", eval.RequestedOutcome)
	}
}

func TestRequireOperationalOutcome_PassWithAllDoDSatisfiedPreservesPass(t *testing.T) {
	exitZero := 0
	d := &task.TaskDetailResponse{
		Task: task.Task{
			ID:                "op-task",
			ExecutionCategory: task.ExecutionCategoryOps,
			Summary:           "Work summary",
			CompleteNote:      "Verification note",
			DoDChecklist: []task.DoDItem{
				{Text: "Criterion 1", Evidence: "Evidence 1", Completed: true},
				{Text: "Criterion 2", Evidence: "Evidence 2", Completed: true},
			},
		},
		LatestAttempt: &task.TaskAttempt{
			ID:       "att-1",
			Status:   task.AttemptStatusSucceeded,
			ExitCode: &exitZero,
		},
	}
	input := inboxReviewEvaluation{
		Decision:         "pass",
		RequestedOutcome: "needs_input",
		RemainingWork:    "external",
		ManagerAction:    "wait",
		ManagerReason:    "Waiting for customer",
		Explanation:      "Completed all work",
	}
	got := requireOperationalOutcome(d, input)
	if got.Decision != "pass" {
		t.Fatalf("expected decision pass, got %s", got.Decision)
	}
	if got.RequestedOutcome != "completed" {
		t.Fatalf("expected requested outcome completed, got %s", got.RequestedOutcome)
	}
	if got.RemainingWork != "none" {
		t.Fatalf("expected remaining work none, got %s", got.RemainingWork)
	}
	if got.ManagerAction != "" {
		t.Fatalf("expected manager action empty, got %s", got.ManagerAction)
	}
}

func TestInboxReviewedTask_RetryResumeVariations(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "completed", task.AttemptStatusSucceeded)
	ctx := context.Background()
	digest := inboxReviewEvidenceDigest(d)

	for _, suffix := range []string{
		":reconcile:" + digest + ":retry:resume:retry",
		":evaluation:" + digest,
		":evaluation:" + digest + ":retry:resume",
		":evidence:" + digest + ":evaluation:" + digest,
		":reconcile:" + digest + ":retry:resume:retry:evaluation:" + digest,
		":evidence:" + digest + ":evaluation-v2:" + digest,
		":reconcile:" + digest + ":retry:resume:retry:evaluation-v2:" + digest,
	} {
		key := "task-review:" + d.LatestAttempt.ID + suffix
		req := db.InboxRequest{
			RoomID:    old.RoomID,
			RequestID: delegationMessageID(key),
			MessageID: delegationMessageID(key + ":input"),
		}
		msg := &db.ProjectMessage{
			ID:         req.MessageID,
			RoomID:     req.RoomID,
			AuthorType: "system",
			AuthorID:   "cuan_work_review",
			Content:    "Review the result for [" + d.Title + "](/tasks/" + d.ID + ").",
		}
		if err := s.projectStudioRepo.CreateMessage(ctx, msg); err != nil {
			t.Fatal(err)
		}
		bound := s.inboxReviewedTask(ctx, req)
		if bound == nil || bound.ID != d.ID {
			t.Fatalf("suffix %s failed to bind task: got %+v", suffix, bound)
		}
	}
}
