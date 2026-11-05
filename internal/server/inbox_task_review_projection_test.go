package server

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxTaskReviewProjectionTracksOnlyCurrentActiveCoordinator(t *testing.T) {
	for _, scenario := range []string{"running", "queued", "completed", "interrupted", "expired", "unstarted", "older attempt", "user message", "cancelled task", "cancelled attempt", "done"} {
		t.Run(scenario, func(t *testing.T) {
			s, _ := setupTestChatServer(t)
			ctx := context.Background()
			now := time.Now().UTC()
			item := &task.TaskItemResponse{Task: task.Task{ID: "task", Status: task.TaskStatusReview}, LatestAttempt: &task.TaskAttempt{ID: "attempt", Status: task.AttemptStatusSucceeded}}
			q := db.InboxRequest{RequestID: delegationMessageID("task-review:attempt"), MessageID: db.GenerateUUID(), RoomID: db.GenerateUUID(), State: "running", StartedAt: &now, Deadline: now.Add(time.Minute)}
			message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review the result for [Task](/tasks/task). Report provenance: current_attempt_report; reported attempt: attempt.", CreatedAt: now}
			want := ""
			switch scenario {
			case "running":
				want = "running"
			case "queued":
				q.State = "queued"
				q.StartedAt = nil
				want = "queued"
			case "completed", "interrupted":
				q.State = scenario
			case "expired":
				q.Deadline = now.Add(-time.Second)
			case "unstarted":
				q.StartedAt = nil
			case "older attempt":
				q.RequestID = delegationMessageID("task-review:old")
				message.Content = "Review [Task](/tasks/task); reported attempt: old."
			case "user message":
				message.AuthorType = "user"
			case "cancelled task":
				item.Status = "cancelled"
			case "cancelled attempt":
				item.LatestAttempt.Status = task.AttemptStatusCanceled
			case "done":
				item.Status = task.TaskStatusDone
			}
			if err := s.projectStudioRepo.CreateMessage(ctx, message); err != nil {
				t.Fatal(err)
			}
			items := []*task.TaskItemResponse{item}
			if err := s.projectInboxTaskReviews(ctx, items, []db.InboxRequest{q}, now); err != nil {
				t.Fatal(err)
			}
			if item.ReviewState != want {
				t.Fatalf("state=%q want=%q", item.ReviewState, want)
			}
			q.State = "completed"
			if err := s.projectInboxTaskReviews(ctx, items, []db.InboxRequest{q}, now); err != nil {
				t.Fatal(err)
			}
			if item.ReviewState != "" {
				t.Fatal("finished review left a live indicator")
			}
		})
	}
}

func TestInboxTaskReviewProgressPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, origin, d := phaseLineageSetup(t, database)
		s.inboxStopping.Store(true)
		ctx := context.Background()
		d.Status = task.TaskStatusReview
		if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		q := db.InboxRequest{RoomID: origin.RoomID, OwnerID: origin.OwnerID, AgentID: origin.AgentID, RequestID: delegationMessageID("task-review:" + d.LatestAttempt.ID), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "review-progress", State: "queued", CreatedAt: now, Deadline: now.Add(time.Minute)}
		message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorID: "cuan_work_review", AuthorType: "system", Content: "Review [Result](/tasks/" + d.ID + ").", CreatedAt: now, UpdatedAt: now}
		store := s.projectStudioRepo.(db.InboxLifecycleRepository)
		if _, fresh, err := store.AcceptInboxRequest(ctx, q, message); err != nil || !fresh {
			t.Fatal("accept review", fresh, err)
		}
		check := func(want string) {
			t.Helper()
			items, err := s.inboxLinkedTasks(ctx, origin.RoomID)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range items {
				if item.ID == d.ID {
					if item.ReviewState != want {
						t.Fatalf("review=%q want=%q", item.ReviewState, want)
					}
					if item.Status != task.TaskStatusReview || item.LatestAttempt.Status != task.AttemptStatusSucceeded {
						t.Fatal("projection changed task outcome")
					}
					return
				}
			}
			t.Fatal("task missing")
		}
		check("queued")
		claimed, fresh, err := store.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
		if err != nil || !fresh {
			t.Fatal("claim review", fresh, err)
		}
		check("running")
		claimed.State = "completed"
		if err := store.FinishInboxRequest(ctx, claimed, nil); err != nil {
			t.Fatal(err)
		}
		check("")
	})
}
