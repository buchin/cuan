package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"strings"
	"testing"
	"time"
)

func TestStoppedLaunchForegroundReplyIsNotRepeated(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{RoomID: room.RoomID, OwnerID: "owner", AgentID: "agent_lifecycle", RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "fixture", CreatedAt: now, Deadline: now.Add(time.Hour)}
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Approved work", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	job := task.TaskLaunch{ID: "launch", TaskID: "task-fixture", RoomID: q.RoomID, AgentID: q.AgentID, SourceRequestID: q.RequestID, State: "stopped", SafeError: "Project files unavailable"}
	check := func(expected int) {
		t.Helper()
		messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
		count := 0
		for _, m := range messages {
			if m.ID == delegationMessageID("launch-outcome:"+job.ID+":stopped") {
				count++
			}
		}
		if count != expected {
			t.Fatal("unexpected duplicate outcome count", count)
		}
	}
	s.publishTaskLaunchOutcome(ctx, job, "stopped", "Second notification")
	check(0)
	q.State = "completed"
	reply := &db.ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, AuthorType: "agent", Content: "Work is saved. " + job.SafeError + "\n[View work](/tasks/" + job.TaskID + ")", CreatedAt: now}
	if err := repo.FinishInboxRequest(ctx, q, reply); err != nil {
		t.Fatal(err)
	}
	s.publishTaskLaunchOutcome(ctx, job, "stopped", "Second notification")
	check(0)
	// A different blocker after the original waiting reply still needs delivery.
	job.ID = "different-launch"
	job.SafeError = "Different blocker"
	for i := 0; i < 2; i++ {
		s.publishTaskLaunchOutcome(ctx, job, "stopped", job.SafeError)
	}
	check(1)
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	for _, m := range messages {
		if strings.Contains(m.Content, "Second notification") {
			t.Fatal("duplicated foreground outcome")
		}
	}
}

func TestSavedWorkCompletionReachesInboxWithoutInventingAnAttempt(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	store := task.NewMemoryStore()
	s.taskSvc = task.NewService(store, nil)
	item := &task.Task{ID: "saved-project-setup", ProjectID: "default", Title: "Prepare projects", SourceRoomID: room.RoomID, Instructions: "Cuan managed delegation v1.\nOriginal approved scope", Status: task.TaskStatusDone, Summary: "Default is ready for general work. Cuan retains the connected repository.", CompleteNote: "Project records and retained history verified.", UpdatedAt: time.Now().UTC()}
	if err := store.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		s.deliverInboxTaskUpdates(ctx)
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.RoomID)
	count := 0
	for _, m := range messages {
		if strings.Contains(m.Content, item.Summary) {
			count++
			if m.AuthorType != "system" {
				t.Fatal("completion falsely claimed as a provider report")
			}
		}
	}
	if count != 1 {
		t.Fatal("completion missing or duplicated", count)
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if len(detail.Attempts) != 0 {
		t.Fatal("completion invented an execution")
	}
}
