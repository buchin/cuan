package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
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

func TestNativeVerifierReviewProjectionPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, origin, d := phaseLineageSetup(t, database)
		s.inboxStopping.Store(true)
		ctx := context.Background()
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		}
		// This test exercises queued role projection only, not verifier admission
		// or source acceptance (covered by the native verification contracts).
		d.Status = task.TaskStatusReview
		must(s.taskSvc.Store().UpdateTask(ctx, &d.Task))
		created, err := s.taskSvc.CreateTask(ctx, origin.OwnerID, task.CreateTaskRequest{ProjectID: d.ProjectID, SourceRoomID: origin.RoomID, Title: "Verifier role projection", ExecutionCategory: task.ExecutionCategoryReadOnly, Instructions: "Approved verification", AssignedAgent: origin.AgentID})
		must(err)
		d = &task.TaskDetailResponse{Task: *created, LatestAttempt: d.LatestAttempt}
		now := time.Now().UTC()
		proof := task.VerificationIntent{SourceAttemptID: d.LatestAttempt.ID, SourceCommit: strings.Repeat("a", 40), InstructionsHash: task.LaunchInstructionsHash(d.Instructions), OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AssessmentRequestID: "projection-assessment", SourceVerificationHash: strings.Repeat("b", 64)}
		raw, _ := json.Marshal(proof)
		launch := task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: task.VerificationRequestID(d.ID, proof.SourceCommit, proof.SourceAttemptID), OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AgentID: origin.AgentID, InstructionsHash: proof.InstructionsHash, Revision: task.VerificationIntentMarker + string(raw), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
		job, err := s.taskSvc.SaveLaunch(ctx, launch)
		must(err)
		claimed, err := s.taskSvc.ClaimLaunch(ctx, now, "projection-fixture", job.ID)
		must(err)
		if claimed == nil {
			t.Fatal("projection fixture launch not claimed")
		}
		attempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: origin.AgentID, RequestID: launch.RequestID})
		must(err)
		check := func(want string) {
			t.Helper()
			items, err := s.inboxLinkedTasks(ctx, origin.RoomID)
			must(err)
			found := false
			for _, item := range items {
				if item.ID == d.ID {
					found = true
					if item.ReviewState != want {
						t.Fatalf("list review=%q want=%q", item.ReviewState, want)
					}
				}
			}
			if !found {
				t.Fatal("native verifier task missing")
			}
			detail, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
			must(err)
			if detail.ReviewState != want {
				t.Fatalf("detail review=%q want=%q", detail.ReviewState, want)
			}
		}
		check("queued")
		attempt.Status, attempt.StartedAt = task.AttemptStatusRunning, &now
		must(s.taskSvc.Store().UpdateAttempt(ctx, attempt))
		check("running")
		// A later queued developer launch must not replace the current verifier role.
		future := launch
		future.RequestID, future.Revision, future.CreatedAt = "future-projection-dev", "Implement the approved work", now.Add(time.Second)
		future.AvailableAt = future.CreatedAt
		_, err = s.taskSvc.SaveLaunch(ctx, future)
		must(err)
		check("running")
		attempt.Status = task.AttemptStatusSucceeded
		must(s.taskSvc.Store().UpdateAttempt(ctx, attempt))
		check("")
	})
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
