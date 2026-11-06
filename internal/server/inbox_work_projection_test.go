package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxWorkProjectionPreservesConversation(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Work", SourceRoomID: "room", SourceMessageID: "source", Instructions: "Approved scope"})
	if err != nil {
		t.Fatal(err)
	}
	a := &task.TaskAttempt{ID: "attempt", TaskID: created.ID, RunnerID: "runner", Status: task.AttemptStatusSucceeded, AttemptNumber: 1, CreatedAt: now}
	if err = s.taskSvc.Store().CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	input := db.ProjectMessage{ID: "input", RoomID: "room", AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review the current result [/tasks/" + created.ID + "]", CreatedAt: now, UpdatedAt: now}
	request := db.InboxRequest{RequestID: "request", OwnerID: "owner", AgentID: "agent", RoomID: "room", MessageID: input.ID, ReplyID: "review-reply", PayloadHash: "hash", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, request, &input); err != nil {
		t.Fatal(err)
	}
	original := []db.ProjectMessage{
		input,
		{ID: "review-reply", RoomID: "room", AuthorType: "agent", Content: "Still checking saved work."},
		{ID: delegationMessageID("task-delivery:" + a.ID), RoomID: "room", AuthorType: "agent", ReplyToID: &created.SourceMessageID, Content: "Files from this work session"},
		{ID: "ordinary", RoomID: "room", AuthorType: "agent", Content: "Why the work needs these changes. [View work](/tasks/" + created.ID + ")"},
		{ID: "user", RoomID: "room", AuthorType: "user", Content: "[View work](/tasks/" + created.ID + ")"},
		{ID: "fake-files", RoomID: "room", AuthorType: "agent", ReplyToID: &created.SourceMessageID, Content: "Files from this work session"},
	}
	projection := s.inboxWorkUpdates(ctx, "room", original)
	if len(projection) != 2 || projection["review-reply"].Kind != "progress" || projection[original[2].ID].Kind != "files" {
		t.Fatal("wrong native work projection", projection)
	}
	for _, id := range []string{"ordinary", "user", "fake-files"} {
		if projection[id] != nil {
			t.Fatal("ordinary speech classified as background", id)
		}
	}
	if len(s.inboxWorkUpdates(ctx, "other-room", original)) != 0 {
		t.Fatal("foreign room work was exposed")
	}
	original[2].ReplyToID = nil
	if s.inboxWorkUpdates(ctx, "room", original)[original[2].ID] != nil {
		t.Fatal("unbound attachment hidden")
	}
	created.Status = task.TaskStatusDone
	if err = s.taskSvc.Store().UpdateTask(ctx, created); err != nil {
		t.Fatal(err)
	}

	if s.inboxWorkUpdates(ctx, "room", original)["review-reply"].Kind != "result" {
		t.Fatal("final outcome hidden")
	}
	// Projection must not rewrite source messages or weaken task acceptance.
	if original[1].Content != "Still checking saved work." {
		t.Fatal("history rewritten")
	}
}
