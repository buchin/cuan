package server

import (
	"context"
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
}

func TestApplyInboxReviewEvaluationMultiPhaseAutoAdvancement(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Setup conversation room
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")

	// 2. Setup user approval message
	userMsgID := db.GenerateUUID()
	userMsg := &db.ProjectMessage{
		ID:         userMsgID,
		RoomID:     room.RoomID,
		AuthorType: "user",
		AuthorID:   "owner",
		Content:    "Approve plan",
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

	evalJSON := `{"decision":"revise","explanation":"### Evaluasi Kriteria Selesai (DoD)\n**Status:** BELUM LULUS\n\n- [ ] **Eliminasi navbar search**: Belum selesai di halaman Inbox.","revision":"Gap: eliminate search in Inbox.\n- [ ] Eliminate search in Inbox"}`
	result := s.applyInboxReviewEvaluation(ctx, q, evalJSON)
	t.Logf("applyInboxReviewEvaluation result: %s", result)

	if !strings.Contains(result, "Evaluasi Kriteria Selesai (DoD)") {
		t.Fatalf("expected result to include structured DoD explanation, got: %s", result)
	}
	if !strings.Contains(result, "Eliminasi navbar search") {
		t.Fatalf("expected result to include gap item, got: %s", result)
	}

	// Verify new DoD item from revision was merged into task.DoDChecklist
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
	if !foundNewDoD {
		t.Fatalf("expected revision DoD item to be merged into task.DoDChecklist, got: %+v", updatedTask.DoDChecklist)
	}
}
