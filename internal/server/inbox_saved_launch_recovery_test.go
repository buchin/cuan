package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestSavedLaunchRequiresPersistedOwnerAuthorization(t *testing.T) {
	for _, scenario := range []string{"original", "missing", "system", "foreign-owner", "future-message", "fresh-owner", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			ctx := context.Background()
			room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
			now := time.Now().UTC()
			q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now.Add(-time.Minute), Deadline: now.Add(time.Hour), PayloadHash: db.GenerateUUID()}
			j := task.TaskLaunch{OwnerID: "owner", RoomID: room.RoomID, RequestID: q.RequestID, SourceRequestID: q.RequestID, TaskID: "existing-task", CreatedAt: now}
			d := &task.TaskDetailResponse{SourceRoomID: room.RoomID, SourceMessageID: q.MessageID}
			author := "user"
			at := q.CreatedAt
			switch scenario {
			case "system":
				author = "system"
			case "foreign-owner":
				q.OwnerID = "other"
			case "future-message":
				at = now.Add(time.Minute)
			case "fresh-owner":
				j.SourceRequestID = "prior-approval"
				d.SourceMessageID = "prior-message"
			}
			if scenario != "missing" {
				_, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: author, Content: "Continue approved work", CreatedAt: at})
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "cancelled" {
				// Use the same production cancellation path as a stopped conversation.
				if _, err := s.projectStudioRepo.(db.InboxLifecycleRepository).CancelInboxRequest(ctx, q.RoomID, q.RequestID); err != nil {
					t.Fatal(err)
				}
			}
			_, authorized, err := s.authorizeSavedTaskLaunch(ctx, j, d)
			want := scenario == "original" || scenario == "fresh-owner"
			if err != nil || authorized != want {
				t.Fatalf("authorized=%v want=%v err=%v", authorized, want, err)
			}
		})
	}
}
