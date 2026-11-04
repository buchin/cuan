package server

import (
	"context"
	"fmt"
	"github.com/buchin/cuan/internal/db"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestInboxReviewReadOnlyScopeRequiresExactRunnerEvidence(t *testing.T) {
	ctx := context.Background()
	store := task.NewMemoryStore()
	svc := task.NewService(store, nil)
	created, err := svc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Inspect retained work", ProjectID: "project", Instructions: "Approved implementation", ExecutionCategory: task.ExecutionCategoryCode})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	requestID := "91000000-0000-4000-8000-000000000002"
	job, err := store.SaveLaunch(ctx, task.TaskLaunch{TaskID: created.ID, ProjectID: "project", RequestID: requestID, SourceRequestID: "owner", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", AgentID: "agent", InstructionsHash: task.LaunchInstructionsHash(created.Instructions), Revision: "Lanjutkan audit read-only pada repositori. Do not push or deploy.", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimLaunch(ctx, now, "fixture-launch", job.ID); err != nil {
		t.Fatal(err)
	}
	att, err := svc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "fixture", AgentName: "agent", RequestID: requestID})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{taskSvc: svc}
	d, err := svc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	scope, verified, err := s.inboxReviewRepositoryScope(ctx, d)
	if err != nil || !scope.ReadOnlyRepository || verified {
		t.Fatal("scope without receipt accepted", err)
	}
	for _, c := range []struct {
		attempt, author string
		want            bool
	}{{"old-attempt", "fixture", false}, {att.ID, "agent", false}, {att.ID, "fixture", true}} {
		detail := *d
		detail.Activities = []*task.TaskActivity{{ActivityType: "repository_scope_verified", NewStatus: "read_only", AttemptID: &c.attempt, Author: c.author}}
		_, verified, err = s.inboxReviewRepositoryScope(ctx, &detail)
		if err != nil || verified != c.want {
			t.Fatalf("evidence attribution accepted incorrectly: %+v %v", c, err)
		}
	}
}

func TestInboxReviewCannotPassReadOnlyReportWithoutRunnerReceipt(t *testing.T) {
	for _, verified := range []bool{false, true} {
		t.Run(fmt.Sprint("verified=", verified), func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			ctx := context.Background()
			room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
			now := time.Now().UTC()
			item := &task.Task{ID: "readonly-review", ProjectID: "default", SourceRoomID: room.RoomID, SourceMessageID: db.GenerateUUID(), AssignedAgent: "agent_lifecycle", Status: task.TaskStatusReview, Title: "Audit retained project", Instructions: "Scope: read-only repository. Compare HEAD and live health; report findings.", Summary: "HEAD abc differs from live def", CompleteNote: "Read-only findings delivered; health endpoint returned200", DoDChecklist: []task.DoDItem{{Text: "Compare commit identifiers", Completed: true, Evidence: "HEAD abc versus live def"}}, CreatedAt: now, UpdatedAt: now}
			if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			latest := ""
			for i := 0; i < 3; i++ {
				latest = fmt.Sprint("readonly-attempt-", i)
				if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: latest, TaskID: item.ID, RunnerID: "fixture", AgentName: "agent_lifecycle", Status: task.AttemptStatusSucceeded, ExitCode: new(int), CreatedAt: now.Add(time.Duration(i) * time.Second)}); err != nil {
					t.Fatal(err)
				}
			}
			if verified {
				if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "scope-receipt", TaskID: item.ID, AttemptID: &latest, ActivityType: "repository_scope_verified", NewStatus: "read_only", Author: "fixture", CreatedAt: now}); err != nil {
					t.Fatal(err)
				}
			}
			key := "task-review:" + latest
			msg := &db.ProjectMessage{ID: delegationMessageID(key + ":input"), RoomID: room.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + item.ID + "]", CreatedAt: now}
			if err := s.projectStudioRepo.CreateMessage(ctx, msg); err != nil {
				t.Fatal(err)
			}
			q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: delegationMessageID(key), MessageID: msg.ID, ReplyID: delegationMessageID(key + ":reply"), CreatedAt: now, Deadline: now.Add(time.Hour)}
			s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","explanation":"The supplied evidence covers the audit.","revision":""}`)
			after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (after.Status == task.TaskStatusDone) != verified {
				t.Fatalf("model pass bypassed runner evidence: verified=%v status=%s", verified, after.Status)
			}
		})
	}
}
