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

func TestCancelledTaskRejectsAutomaticCoordinatorContinuation(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "p1", Name: "One"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	accept := func(author string, at time.Time) db.InboxRequest {
		q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: at, Deadline: at.Add(time.Hour), PayloadHash: db.GenerateUUID()}
		_, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: author, Content: "Continue the approved scope", CreatedAt: at})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	original := accept("user", time.Now().UTC().Add(-time.Minute))
	saved, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", original.MessageID, task.CreateTaskRequest{SourceRoomID: room.RoomID, ProjectID: "p1", Title: "Audit", Instructions: "Audit only.\nApproval message: " + original.MessageID, AssignedAgent: original.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.taskSvc.CreateAttempt(ctx, saved.ID, "owner", task.CreateAttemptRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.CancelTask(ctx, saved.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	automatic := accept("system", time.Now().UTC().Add(time.Second))
	d := inboxDelegation{TaskID: saved.ID, ProjectID: "p1", AuthorizationID: original.MessageID, Revision: "Verify saved work; preserve original scope"}
	raw, _ := json.Marshal(d)
	out := s.applyInboxDelegation(ctx, automatic, "```cuan-delegate\n"+string(raw)+"\n```")
	if !strings.Contains(out, "was cancelled") {
		t.Fatal(out)
	}
	// A launch persisted before cancellation must also be rejected at drain time.
	now := time.Now().UTC()
	job, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: saved.ID, ProjectID: "p1", RequestID: automatic.RequestID, OwnerID: original.OwnerID, RoomID: room.RoomID, SourceRequestID: original.RequestID, AgentID: original.AgentID, Revision: d.Revision, InstructionsHash: task.LaunchInstructionsHash(detail.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := s.taskSvc.ClaimLaunch(ctx, now, db.GenerateUUID(), job.ID)
	if err != nil || claimed == nil {
		t.Fatal("launch not claimed", err)
	}
	if out := s.executeQueuedTaskLaunch(ctx, *claimed, false); !strings.Contains(out, "was cancelled") {
		t.Fatal(out)
	}
	job, err = s.taskSvc.GetLaunch(ctx, job.ID)
	if err != nil || job.State != "stopped" {
		t.Fatal("cancelled launch not stopped", err)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, saved.ID)
	if len(after.Attempts) != 1 {
		t.Fatal("cancelled work restarted", len(after.Attempts))
	}
	detail.Activities = append(detail.Activities, &task.TaskActivity{ID: "checkpoint", AttemptID: &detail.LatestAttempt.ID, ActivityType: "agent_report", NewStatus: "progress", Message: "SAVED_WORK_BEFORE_CANCEL"})
	if !strings.Contains(taskRecoveryContext(detail), "SAVED_WORK_BEFORE_CANCEL") {
		t.Fatal("cancellation lost saved checkpoint")
	}
	if !strings.Contains(taskRecoveryContext(detail), "Do not revise or restart") {
		t.Fatal("review omits cancellation")
	}
	if s.taskContinuationAfterCancel(ctx, original, detail) {
		t.Fatal("old approval reversed cancellation")
	}
	fresh := accept("user", time.Now().UTC().Add(time.Second))
	if !s.taskContinuationAfterCancel(ctx, fresh, detail) {
		t.Fatal("fresh authenticated user continuation rejected")
	}
	forged := fresh
	forged.RequestID = db.GenerateUUID()
	if s.taskContinuationAfterCancel(ctx, forged, detail) {
		t.Fatal("unaccepted user instruction accepted")
	}
}
