package server

import (
	"context"
	"encoding/json"
	"fmt"
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
	// A lost HTTP response can be retried after execution has already changed
	// the task. A previously accepted decision must remain readable/idempotent;
	// it must not be confused with a fresh approval of stale work.
	approval := q
	approval.RequestID = delegationMessageID("approve-plan:" + q.RoomID + ":" + proposalRequest.ReplyID)
	approval.MessageID, approval.ReplyID = db.GenerateUUID(), db.GenerateUUID()
	approval.State = "queued"
	decisionEnvelope, _ := json.Marshal(attachmentMessage{Text: "Approved: " + plan.Title, Approval: &approvedInboxPlan{MessageID: proposalRequest.ReplyID, Plan: *plan}})
	if _, _, err := store.AcceptInboxRequest(ctx, approval, &db.ProjectMessage{ID: approval.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: attachmentEnvelope + string(decisionEnvelope), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.persistedInboxPlan(ctx, q.RoomID, q.OwnerID, q.AgentID, proposalRequest.ReplyID); err != nil {
		t.Fatal("accepted decision replay lost its receipt", err)
	}
	if s.validateInboxContinuation(ctx, q.RoomID, plan) == nil {
		t.Fatal("receipt replay authorized a new stale execution")
	}
}

func TestInboxContinuationApprovalHTTPReplaySurvivesTaskAdvance(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	s.inboxStopping.Store(true) // Test durable HTTP admission without provider work.
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	now := time.Now().UTC()
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	source := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "source", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := store.AcceptInboxRequest(ctx, source, &db.ProjectMessage{ID: source.MessageID, RoomID: room, AuthorType: "user", Content: "Check the saved work.", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	source.State = "completed"
	if err := store.FinishInboxRequest(ctx, source, nil); err != nil {
		t.Fatal(err)
	}
	item := &task.Task{ID: "continuation-http-task", ProjectID: "default", SourceRoomID: room, SourceMessageID: source.MessageID, Instructions: "Cuan managed delegation v1.\nCheck the saved work.", Title: "Check saved work", Status: task.TaskStatusBlocked, CreatedAt: now, UpdatedAt: now}
	if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: "http-source-attempt", TaskID: item.ID, Status: task.AttemptStatusFailed, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	proposal := source
	proposal.RequestID, proposal.MessageID, proposal.ReplyID = db.GenerateUUID(), db.GenerateUUID(), db.GenerateUUID()
	proposal.PayloadHash, proposal.State = "proposal", "queued"
	if _, _, err := store.AcceptInboxRequest(ctx, proposal, &db.ProjectMessage{ID: proposal.MessageID, RoomID: room, AuthorType: "system", Content: "Review saved work.", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	decision := inboxReviewEvaluation{Decision: "revise", Explanation: "A check is missing.", Revision: "Run the missing original check and report its actual result.", RemainingWork: "verification"}
	if _, err := s.saveReviewAssessment(ctx, d, proposal, decision); err != nil {
		t.Fatal(err)
	}
	proposal.State = "completed"
	content := continuationPlan(d, proposal.RequestID, decision)
	if err := store.FinishInboxRequest(ctx, proposal, &db.ProjectMessage{ID: proposal.ReplyID, RoomID: room, AuthorType: "agent", AuthorID: source.AgentID, Content: content, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	post := func() db.InboxRequest {
		t.Helper()
		body := fmt.Sprintf(`{"request_id":%q,"message":"Approve","approve_message_id":%q}`, db.GenerateUUID(), proposal.ReplyID)
		w := call("POST", "/api/v1/chat/conversations/"+room+"/messages", body)
		if w.Code != 200 && w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result struct{ Request db.InboxRequest }
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Request
	}
	accepted := post()
	item.Status = task.TaskStatusDoing
	if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		replay := post()
		if replay.RequestID != accepted.RequestID || replay.MessageID != accepted.MessageID {
			t.Fatal("approval retry duplicated admission")
		}
	}
	requests, err := store.ListInboxRequests(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 {
		t.Fatal("replay inserted another request", len(requests))
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil || len(after.Attempts) != 1 || after.Status != task.TaskStatusDoing {
		t.Fatal("HTTP replay changed execution", err)
	}
}
