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

func TestStoppedLaunchDoesNotMisreportRecordedExecution(t *testing.T) {
	for _, state := range []string{task.AttemptStatusRunning, task.AttemptStatusFailed} {
		t.Run(state, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			s.inboxStopping.Store(true)
			ctx := context.Background()
			room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
			now := time.Now().UTC()
			origin := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "fixture", State: "completed", CreatedAt: now.Add(-time.Minute), Deadline: now.Add(time.Hour)}
			if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, origin, &db.ProjectMessage{ID: origin.MessageID, RoomID: room.RoomID, AuthorType: "user", Content: "Original approved work", CreatedAt: origin.CreatedAt}); err != nil {
				t.Fatal(err)
			}
			item := &task.Task{ID: "recorded-execution", ProjectID: "default", SourceRoomID: room.RoomID, SourceMessageID: origin.MessageID, AssignedAgent: origin.AgentID, Instructions: "Cuan managed delegation v1.\nOriginal scope", Status: task.TaskStatusBlocked, CreatedAt: now}
			if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			job, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: db.GenerateUUID(), OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AgentID: origin.AgentID, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			leased, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "fixture-launch", job.ID)
			if err != nil || leased == nil {
				t.Fatal(err)
			}
			attempt, err := s.taskSvc.CreateAttempt(ctx, item.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "fixture", AgentName: origin.AgentID, RequestID: job.RequestID})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
				t.Fatal(err)
			}
			if err := s.taskSvc.FinishLaunch(ctx, job.ID, leased.LeaseToken, "stopped", time.Now(), "STALE_LAUNCH_ADMISSION_ERROR"); err != nil {
				t.Fatal(err)
			}
			if state == task.AttemptStatusFailed {
				if _, err := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: state}); err != nil {
					t.Fatal(err)
				}
			}
			blocked := task.TaskStatusBlocked
			if _, err := s.taskSvc.UpdateTask(ctx, item.ID, "owner", task.UpdateTaskRequest{Status: &blocked}); err != nil {
				t.Fatal(err)
			}

			s.deliverInboxTaskUpdates(ctx)
			messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.RoomID)
			if err != nil {
				t.Fatal(err)
			}
			for _, message := range messages {
				if strings.Contains(message.Content, "STALE_LAUNCH_ADMISSION_ERROR") {
					t.Fatal("admission diagnostic misreported a recorded attempt")
				}
			}
			current, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
			if len(current.Attempts) != 1 || current.LatestAttempt.ID != attempt.ID || current.LatestAttempt.Status != state {
				t.Fatal("delivery changed execution state")
			}
		})
	}
}

func TestStaleInspectionNoticeRequiresDifferentActiveAuthorizedExecution(t *testing.T) {
	for _, c := range []struct {
		name, state               string
		same, foreign, cancelling bool
		want                      int
	}{
		{name: "running", state: task.AttemptStatusRunning},
		{name: "queued", state: task.AttemptStatusQueued},
		{name: "failed", state: task.AttemptStatusFailed, want: 1},
		{name: "same launch", state: task.AttemptStatusRunning, same: true, want: 1},
		{name: "different owner", state: task.AttemptStatusRunning, foreign: true, want: 1},
		{name: "cancelling", state: task.AttemptStatusRunning, cancelling: true, want: 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			s.inboxStopping.Store(true)
			ctx := context.Background()
			room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
			now := time.Now().UTC()
			item := &task.Task{ID: "stale-inspection", ProjectID: "default", Title: "Approved work", Instructions: "Original approved scope", Status: task.TaskStatusDoing, CreatedAt: now}
			if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			bound, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: db.GenerateUUID(), OwnerID: "owner", RoomID: room.RoomID, SourceRequestID: "approved", AgentID: "agent_lifecycle", InstructionsHash: task.LaunchInstructionsHash(item.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			lease, err := s.taskSvc.ClaimLaunch(ctx, now, "fixture", bound.ID)
			if err != nil || lease == nil {
				t.Fatal("claim", err)
			}
			a, err := s.taskSvc.CreateAttempt(ctx, item.ID, "owner", task.CreateAttemptRequest{RunnerID: "fixture", AgentName: bound.AgentID, RequestID: bound.RequestID})
			if err != nil {
				t.Fatal(err)
			}
			a.Status = c.state
			a.CancelRequested = c.cancelling
			if err := s.taskSvc.Store().UpdateAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			job := *bound
			job.ID = "obsolete-inspection"
			job.SourceRequestID = ""
			job.SafeError = staleInspectionLaunchReason
			if c.same {
				job.ID = bound.ID
			}
			if c.foreign {
				job.OwnerID = "other-owner"
			}
			s.publishTaskLaunchOutcome(ctx, job, "stopped", staleInspectionLaunchReason)
			messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.RoomID)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, m := range messages {
				if m.ID == delegationMessageID("launch-outcome:"+job.ID+":stopped") {
					count++
				}
			}
			if count != c.want {
				t.Fatalf("notice count=%d want=%d", count, c.want)
			}
			current, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
			if len(current.Attempts) != 1 || current.LatestAttempt.ID != a.ID || current.LatestAttempt.Status != c.state {
				t.Fatal("notification changed execution")
			}
		})
	}
}
