package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func TestGitHubRecoveryQueuesOnlySameApprovedScope(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: "approved"}
	_, _, e := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Approved repository work", CreatedAt: now})
	if e != nil {
		t.Fatal(e)
	}
	store := task.NewMemoryStore()
	s.taskSvc = task.NewService(store, nil)
	item := &task.Task{ID: "waiting-github", ProjectID: "default", Title: "Approved work", Instructions: "Approved repository work", AssignedAgent: q.AgentID, SourceRoomID: q.RoomID, SourceMessageID: q.MessageID, Status: "blocked", UpdatedAt: now}
	if e = store.CreateTask(ctx, item); e != nil {
		t.Fatal(e)
	}
	a := &task.TaskAttempt{ID: "attempt-github", TaskID: item.ID, Status: task.AttemptStatusFailed, CreatedAt: now}
	if e = store.CreateAttempt(ctx, a); e != nil {
		t.Fatal(e)
	}
	if e = s.taskSvc.RecordGitHubPrerequisite(ctx, item.ID, a.ID, false); e != nil {
		t.Fatal(e)
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
	changed := *detail
	changed.Instructions = "Different work"
	if e = s.queueGitHubRecovery(ctx, &changed, owner.ID); e != nil {
		t.Fatal(e)
	}
	if job, _ := s.taskSvc.LatestLaunch(ctx, item.ID); job != nil {
		t.Fatal("changed scope launched")
	}
	if e = s.queueGitHubRecovery(ctx, detail, "other-owner"); e != nil {
		t.Fatal(e)
	}
	if job, _ := s.taskSvc.LatestLaunch(ctx, item.ID); job != nil {
		t.Fatal("wrong owner launched")
	}
	for i := 0; i < 2; i++ {
		if e = s.queueGitHubRecovery(ctx, detail, owner.ID); e != nil {
			t.Fatal(e)
		}
	}
	job, e := s.taskSvc.LatestLaunch(ctx, item.ID)
	if e != nil || job == nil || job.State != "queued" || job.Revision != githubContinuation {
		t.Fatal("continuation not retained", e)
	}
	reconstructed := task.NewService(store, nil)
	again, _ := reconstructed.LatestLaunch(ctx, item.ID)
	if again == nil || again.ID != job.ID {
		t.Fatal("reconstruction lost deduplicated launch")
	}
	if len(detail.Attempts) != 1 {
		t.Fatal("queueing directly launched duplicate attempts")
	}
}
