package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestSystemReviewCannotFabricateMissingOwnerApproval(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "review-project", Name: "Review"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	original := db.GenerateUUID()
	item, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", original, task.CreateTaskRequest{SourceRoomID: room.RoomID, ProjectID: "review-project", Title: "Saved task without original message", Instructions: "Cuan managed delegation v1.\nApproved work; preserve scope.", AssignedAgent: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: db.GenerateUUID()}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [work](/tasks/" + item.ID + ")", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(inboxDelegation{TaskID: item.ID, ProjectID: item.ProjectID, AuthorizationID: original, Revision: "Inspect saved work and verify original criteria."})
	out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(payload)+"\n```")
	if !strings.Contains(out, "approval must come from a user message") {
		t.Fatal(out)
	}
	if launch, err := s.taskSvc.LatestLaunch(ctx, item.ID); err != nil || launch != nil {
		t.Fatal("review created an execution intent without approval", launch, err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if len(current.Attempts) != 0 || current.Status != item.Status || current.AssignedAgent != item.AssignedAgent {
		t.Fatal("system review changed unapproved work")
	}
}
