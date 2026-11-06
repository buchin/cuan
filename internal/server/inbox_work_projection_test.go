package server

import (
	"context"
	"database/sql"
	"strings"
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

func TestInboxWorkProjectionNativeLaunchNotices(t *testing.T) {
	for _, test := range []struct {
		name, reason         string
		continuation, hidden bool
	}{
		{"unresolved stale", staleInspectionLaunchReason, false, false},
		{"superseded inspection", staleInspectionLaunchReason, true, true},
		{"publication preparation", task.InspectionPublicationPreparationError, true, true},
		{"replaced recovery", supersededRecoveryLaunchReason, true, true},
		{"authorization failure", "The original user authorization is unavailable or cancelled.", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			d, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Work", SourceRoomID: "room", SourceMessageID: "source", Instructions: "Scope"})
			if err != nil {
				t.Fatal(err)
			}
			store := s.taskSvc.Store().(task.LaunchStore)
			job := task.TaskLaunch{TaskID: d.ID, ProjectID: "project", RequestID: "inspection", OwnerID: "owner", RoomID: "room", SourceRequestID: "source-request", AgentID: "agent", InstructionsHash: "scope", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
			saved, err := store.SaveLaunch(ctx, job)
			if err != nil {
				t.Fatal(err)
			}
			claimed, err := store.ClaimLaunch(ctx, now, "worker")
			if err != nil || claimed == nil {
				t.Fatal(claimed, err)
			}
			if err = store.FinishLaunch(ctx, claimed.ID, claimed.LeaseToken, "stopped", now, test.reason); err != nil {
				t.Fatal(err)
			}
			if test.continuation {
				job.RequestID = "continuation"
				job.CreatedAt = now.Add(time.Second)
				job.AvailableAt = job.CreatedAt
				if _, err = store.SaveLaunch(ctx, job); err != nil {
					t.Fatal(err)
				}
			}
			notice := db.ProjectMessage{ID: delegationMessageID(saved.RequestID + ":notice"), RoomID: "room", AuthorType: "system", Content: "Checking work"}
			outcome := db.ProjectMessage{ID: delegationMessageID("launch-outcome:" + saved.ID + ":stopped"), RoomID: "room", AuthorType: "agent", Content: test.reason}
			detail, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			s.projectInboxLaunchDiagnostics(ctx, detail)
			foundDiagnostic := false
			for _, a := range detail.Activities {
				if a.ID == "launch:"+saved.ID && strings.Contains(a.Message, test.reason) {
					foundDiagnostic = true
				}
			}
			if !foundDiagnostic {
				t.Fatal("launch evidence missing from Detail")
			}
			savedActivities, _ := s.taskSvc.Store().ListActivities(ctx, d.ID)
			for _, a := range savedActivities {
				if a.ActivityType == "launch_diagnostic" {
					t.Fatal("projection mutated history")
				}
			}
			ordinary := db.ProjectMessage{ID: "ordinary", RoomID: "room", AuthorType: "agent", Content: test.reason}
			got := s.inboxWorkUpdates(ctx, "room", []db.ProjectMessage{notice, outcome, ordinary})
			if got[notice.ID] == nil || got[notice.ID].TaskID != d.ID {
				t.Fatal("native notice not projected without attempt", got)
			}
			if (got[outcome.ID] != nil) != test.hidden {
				t.Fatal("failure visibility", got, test)
			}
			if got[ordinary.ID] != nil {
				t.Fatal("ordinary speech hidden")
			}
			notice.AuthorType = "user"
			if s.inboxWorkUpdates(ctx, "room", []db.ProjectMessage{notice})[notice.ID] != nil {
				t.Fatal("user speech hidden")
			}
			if len(s.inboxWorkUpdates(ctx, "other-room", []db.ProjectMessage{outcome})) != 0 {
				t.Fatal("cross-room projection")
			}
		})
	}
}

func TestInboxLaunchProjectionPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, _, _, child, _ := phaseLineageFixture(t, database)
		refs, err := s.taskSvc.Store().(task.InboxLaunchRefStore).InboxLaunchRefs(context.Background(), child.SourceRoomID)
		if err != nil || len(refs) == 0 {
			t.Fatal(refs, err)
		}
		found := false
		for _, ref := range refs {
			if ref.TaskID == child.ID {
				found = true
				if ref.ID == "" || ref.RequestID == "" || ref.CreatedAt.IsZero() {
					t.Fatal("incomplete ref", ref)
				}
			}
		}
		if !found {
			t.Fatal("missing child launch")
		}
		other, err := s.taskSvc.Store().(task.InboxLaunchRefStore).InboxLaunchRefs(context.Background(), "11111111-1111-4111-8111-111111111111")
		if err != nil || len(other) != 0 {
			t.Fatal("foreign room", other, err)
		}
	})
}
