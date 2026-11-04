package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestPMReviewIntentionalPTYExitRequiresSameAttemptFinalReport(t *testing.T) {
	code := 137
	id := "attempt"
	now := time.Now()
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "CSV export verified", CompleteNote: "CSV rows matched fixture", DoDChecklist: []task.DoDItem{{Text: "CSV format", Completed: true, Evidence: "header and rows match fixture"}}}, LatestAttempt: &task.TaskAttempt{ID: id, Status: task.AttemptStatusSucceeded, ExitCode: &code}}
	if inboxReviewExecutionSucceeded(d) {
		t.Fatal("unconfirmed nonzero success accepted")
	}
	d.Activities = []*task.TaskActivity{{ActivityType: "agent_report", AttemptID: &id, NewStatus: "review", CreatedAt: now}}
	if !inboxReviewExecutionSucceeded(d) || evaluateTaskDoDDeterministic(d).Decision != "pass" {
		t.Fatal("validated final report rejected because PTY was stopped")
	}
	other := "old-attempt"
	d.Activities[0].AttemptID = &other
	if inboxReviewExecutionSucceeded(d) {
		t.Fatal("older report accepted")
	}
	d.Activities[0].AttemptID = &id
	d.Activities = append(d.Activities, &task.TaskActivity{ActivityType: "agent_report", AttemptID: &id, NewStatus: "blocked", CreatedAt: now.Add(time.Second)})
	if inboxReviewExecutionSucceeded(d) {
		t.Fatal("explicit blocker accepted")
	}
	d.Activities = d.Activities[:1]
	d.DoDChecklist[0].Evidence = ""
	if evaluateTaskDoDDeterministic(d).Decision == "pass" {
		t.Fatal("signal recovery bypassed acceptance evidence")
	}
	d.LatestAttempt.Status = task.AttemptStatusFailed
	if inboxReviewExecutionSucceeded(d) {
		t.Fatal("failed execution accepted")
	}
}

func TestPMReviewInfrastructureDoesNotExhaustBusinessCorrectionBudget(t *testing.T) {
	exit := 137
	d := &task.TaskDetailResponse{LatestAttempt: &task.TaskAttempt{Status: task.AttemptStatusFailed, AttemptNumber: 6, ErrorMessage: "compiler error", ExitCode: new(int)}}
	d.Attempts = []*task.TaskAttempt{{Status: "interrupted"}, {Status: "failed", ExitCode: &exit}, {Status: "canceled"}, d.LatestAttempt}
	if got := taskRevisionAttemptCount(d); got != 1 {
		t.Fatal(got)
	}
	if decision := evaluateTaskDoDDeterministic(d); decision.Decision != "revise" {
		t.Fatalf("process ordinal spent review budget: %+v", decision)
	}
	d.Attempts = append(d.Attempts, &task.TaskAttempt{Status: "succeeded", ExitCode: &exit})
	if got := taskRevisionAttemptCount(d); got != 2 {
		t.Fatal("validated success incorrectly classified as infra", got)
	}
}

func TestPMReviewCapacityWaitPreservesTaskAndRetriesSameReview(t *testing.T) {
	s, token, registered, primary, backup, _, _ := setupFailoverFixture(t)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []*agent.ProviderAccount{primary, backup} {
		if err := agent.RecordProviderAccountQuotaWithDuration(p.OwnerID, p.ID, time.Hour, ""); err != nil {
			t.Fatal(err)
		}
	}
	room := s.resolveConversationContext(ctx, registered.ID, "")
	origin := db.InboxRequest{OwnerID: owner.ID, AgentID: registered.ID, RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: time.Now(), Deadline: time.Now().Add(time.Hour), PayloadHash: db.GenerateUUID()}
	repo := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := repo.AcceptInboxRequest(ctx, origin, &db.ProjectMessage{ID: origin.MessageID, RoomID: room.RoomID, AuthorType: "user", Content: "Verify approved CSV export", CreatedAt: origin.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	taskID, attemptID := "pm-capacity-task", "pm-capacity-attempt"
	saved := &task.Task{ID: taskID, ProjectID: "default", SourceRoomID: room.RoomID, SourceMessageID: origin.MessageID, AssignedAgent: registered.ID, Status: "review", Summary: "CSV export", CompleteNote: "Unverified output", Instructions: "Verify approved CSV export", DoDChecklist: []task.DoDItem{{Text: "Output matches CSV schema"}}}
	if err := s.taskSvc.Store().CreateTask(ctx, saved); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: attemptID, TaskID: taskID, AgentName: registered.ID, Status: "succeeded", ExitCode: new(int), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	d, _ := s.taskSvc.GetTaskDetail(ctx, taskID)
	s.queueInboxTaskReview(ctx, d)
	requestID := delegationMessageID("task-review:" + attemptID)
	deadline := time.Now().Add(5 * time.Second)
	for {
		requests, _ := repo.ListInboxRequests(ctx, room.RoomID)
		found := false
		for _, q := range requests {
			if q.RequestID == requestID && q.State == "queued" && q.SafeError == inboxQuotaNoBackup {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("review lost instead of capacity waiting: %+v", requests)
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.drainInboxRoom(room.RoomID)
	after, _ := s.taskSvc.GetTaskDetail(ctx, taskID)
	if after.Status != "review" || len(after.Attempts) != 1 || after.CompleteNote != saved.CompleteNote {
		t.Fatalf("capacity changed work: %+v", after)
	}
	requests, _ := repo.ListInboxRequests(ctx, room.RoomID)
	if len(requests) != 2 {
		t.Fatal("review duplicated", len(requests))
	}
	if _, err := repo.CancelInboxRequest(ctx, room.RoomID, requestID); err != nil {
		t.Fatal(err)
	}
	s.drainInboxRoom(room.RoomID)
}

func TestPMReviewMarkdownResultIsAppliedAndAcceptedResultStaysFinal(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	// Native provider emits a valid plain review rather than schema JSON.
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	script := `#!/bin/sh
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"### Evaluasi Kriteria Selesai (Definition of Done)\n**Status:** LULUS\n\n**Bukti yang Terverifikasi:**\n- [x] CSV fixture matched.\n\n**Tindakan:** Menyatakan selesai."}}'
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	taskID, attemptID := "pm-markdown-task", "pm-markdown-attempt"
	item := &task.Task{ID: taskID, ProjectID: "default", SourceRoomID: room.RoomID, AssignedAgent: "agent_lifecycle", Status: "review", Summary: "CSV export", CompleteNote: "Header and row fixture matched", DoDChecklist: []task.DoDItem{{Text: "CSV output", Completed: true, Evidence: "header and row fixture"}}}
	if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: attemptID, TaskID: taskID, AgentName: "agent_lifecycle", Status: "succeeded", ExitCode: new(int), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	d, _ := s.taskSvc.GetTaskDetail(ctx, taskID)
	s.queueInboxTaskReview(ctx, d)
	requestID := delegationMessageID("task-review:" + attemptID)
	waitInboxState(t, s, room.RoomID, requestID, "completed")
	after, _ := s.taskSvc.GetTaskDetail(ctx, taskID)
	if after.Status != "done" {
		t.Fatal("plain review was displayed but not applied", after.Status)
	}
	requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, room.RoomID)
	after.CompleteNote += "; retained evidence"
	s.queueInboxTaskReview(ctx, after)
	unchanged, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, room.RoomID)
	if len(unchanged) != len(requests) {
		t.Fatal("accepted work re-reviewed")
	}
	q := requests[0]
	out := s.applyInboxReviewEvaluation(ctx, q, `{"decision":"revise","explanation":"Late rejection","revision":"Repeat checks"}`)
	final, _ := s.taskSvc.GetTaskDetail(ctx, taskID)
	if final.Status != "done" || strings.Contains(out, "Late rejection") {
		t.Fatal("late review reopened accepted work", out)
	}
}
