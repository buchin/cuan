package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxContinuationRequiresCurrentSavedAssessment(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	decision := inboxReviewEvaluation{Decision: "revise", Explanation: "Evidence is missing.", Revision: "Run only the missing original checks; preserve saved work.", RemainingWork: "verification"}
	if _, err := s.saveReviewAssessment(ctx, d, q, decision); err != nil {
		t.Fatal(err)
	}
	blocked := task.TaskStatusBlocked
	if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal(err)
	}
	plan := parseInboxApprovalPlan(continuationPlan(d, q.RequestID, decision))
	if plan == nil {
		t.Fatal("no executable proposal")
	}
	if err := s.validateInboxContinuation(ctx, q.RoomID, plan); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	proposalRequest := q
	proposalRequest.RequestID, proposalRequest.MessageID, proposalRequest.ReplyID = db.GenerateUUID(), db.GenerateUUID(), db.GenerateUUID()
	proposalRequest.State = "queued"
	if _, _, err := store.AcceptInboxRequest(ctx, proposalRequest, &db.ProjectMessage{ID: proposalRequest.MessageID, RoomID: q.RoomID, AuthorType: "system", Content: "Saved recommendation", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	proposalRequest.State = "completed"
	if err := store.FinishInboxRequest(ctx, proposalRequest, &db.ProjectMessage{ID: proposalRequest.ReplyID, RoomID: q.RoomID, AuthorType: "agent", AuthorID: q.AgentID, Content: continuationPlan(d, q.RequestID, decision), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.persistedInboxPlan(ctx, q.RoomID, q.OwnerID, q.AgentID, proposalRequest.ReplyID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.persistedInboxPlan(ctx, q.RoomID, "different-owner", q.AgentID, proposalRequest.ReplyID); err == nil {
		t.Fatal("foreign owner approved proposal")
	}
	raw := approvedPlanDelegation(&approvedInboxPlan{Plan: *plan}, "owner-approval")
	_, parsed, err := parseInboxDelegation(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed == nil || parsed.TaskID != d.ID || parsed.Revision != decision.Revision || parsed.AuthorizationID != "owner-approval" || parsed.Title != "" {
		t.Fatal("approval did not continue the original task", raw)
	}
	original, _ := json.Marshal(plan)
	for _, status := range []string{task.TaskStatusDone, "cancelled", task.TaskStatusDoing} {
		item, err := s.taskSvc.Store().GetTask(ctx, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		item.Status = status
		if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		if s.validateInboxContinuation(ctx, q.RoomID, plan) == nil {
			t.Fatal("accepted proposal after task state changed", status)
		}
	}
	if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked}); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"revision", "description", "attempt", "assessment", "room"} {
		var altered inboxApprovalPlan
		_ = json.Unmarshal(original, &altered)
		room := q.RoomID
		switch scenario {
		case "revision":
			altered.Continuation.Revision = "Do unrelated work."
		case "description":
			altered.Description = "A different visible promise."
		case "attempt":
			altered.Continuation.AttemptID = "old-attempt"
		case "assessment":
			altered.Continuation.AssessmentRequestID = "not-saved"
		case "room":
			room = "other-room"
		}
		if s.validateInboxContinuation(ctx, room, &altered) == nil {
			t.Fatal("accepted altered proposal", scenario)
		}
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: "newer-attempt", TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if s.validateInboxContinuation(ctx, q.RoomID, plan) == nil {
		t.Fatal("accepted stale attempt proposal")
	}
}
