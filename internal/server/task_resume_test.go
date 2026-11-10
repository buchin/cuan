package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestResumeQueuesExistingSupervisionWithoutRestartingDevelopment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	s.cfg.InboxExecutionRole = "api"
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	room := &db.ProjectRoom{ProjectID: "default", Title: "Paused task"}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	origin := db.InboxRequest{RoomID: room.ID, OwnerID: owner.ID, AgentID: "agent_cuan", RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "original", State: "queued", CreatedAt: now.Add(-time.Minute), Deadline: now.Add(time.Hour)}
	if _, _, err := repo.AcceptInboxRequest(ctx, origin, &db.ProjectMessage{ID: origin.MessageID, RoomID: room.ID, AuthorType: "user", Content: "Finish the approved work", CreatedAt: origin.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	origin.State = "completed"
	if err := repo.FinishInboxRequest(ctx, origin, nil); err != nil {
		t.Fatal(err)
	}
	item := &task.Task{ID: "resume-task", SourceRoomID: room.ID, SourceMessageID: origin.MessageID, AssignedAgent: origin.AgentID, Status: "blocked", Instructions: "Original approved scope and DoD", Summary: "Retained report", CompleteNote: "Retained verification", CreatedAt: origin.CreatedAt, UpdatedAt: now}
	if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	attempt := &task.TaskAttempt{ID: "resume-attempt", TaskID: item.ID, AgentName: origin.AgentID, RunnerID: "fixture", Status: "failed", CreatedAt: origin.CreatedAt}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	key := "task-review:" + attempt.ID
	reviewed := db.InboxRequest{RoomID: room.ID, OwnerID: owner.ID, AgentID: origin.AgentID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: "review", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := repo.AcceptInboxRequest(ctx, reviewed, &db.ProjectMessage{ID: reviewed.MessageID, RoomID: room.ID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review current result [/tasks/" + item.ID + "].", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	reviewed.State = "completed"
	if err := repo.FinishInboxRequest(ctx, reviewed, nil); err != nil {
		t.Fatal(err)
	}
	call := func(method, body, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/tasks/"+item.ID+"/resume", strings.NewReader(body))
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "", ""); w.Code != 401 {
		t.Fatal("authentication bypass", w.Code)
	}
	if w := call("GET", "", token); w.Code != 200 || !strings.Contains(w.Body.String(), `"available":true`) {
		t.Fatal("missing resume plan", w.Code, w.Body.String())
	}
	if w := call("POST", `{"attempt_id":"stale"}`, token); w.Code != 409 {
		t.Fatal("stale attempt resumed", w.Code)
	}
	body := `{"attempt_id":"` + attempt.ID + `"}`
	first := call("POST", body, token)
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body.String())
	}
	var receipt struct {
		RequestID string `json:"request_id"`
	}
	json.Unmarshal(first.Body.Bytes(), &receipt)
	duplicate := call("POST", body, token)
	if duplicate.Code != 202 || !strings.Contains(duplicate.Body.String(), receipt.RequestID) {
		t.Fatal("duplicate resume was not replayed", duplicate.Code, duplicate.Body.String())
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Attempts) != 1 || after.LatestAttempt.ID != attempt.ID || after.Instructions != item.Instructions || after.Summary != item.Summary || after.CompleteNote != item.CompleteNote || after.Status != "blocked" {
		t.Fatal("resume restarted execution or rewrote saved work")
	}
	requests, err := repo.ListInboxRequests(ctx, room.ID)
	if err != nil || len(requests) != 3 {
		t.Fatal("duplicate supervision or missing request", len(requests), err)
	}
	projected := &task.TaskItemResponse{Task: after.Task, LatestAttempt: after.LatestAttempt}
	if err := s.projectInboxTaskReviews(ctx, []*task.TaskItemResponse{projected}, requests, time.Now()); err != nil || projected.ReviewState != "queued" {
		t.Fatal("queued supervision stayed invisible", projected.ReviewState, err)
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
	if len(visibleInboxMessages(messages)) != 1 {
		t.Fatal("resume polluted chat with control messages")
	}
	// Completing the owner reassessment must not grant unlimited new requests.
	for _, q := range requests {
		if q.RequestID == receipt.RequestID {
			q.State = "completed"
			if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if plan, err := s.taskResumePlan(ctx, owner.ID, after); err != nil || plan.Available {
		t.Fatal("used evidence snapshot renewed continuation", err)
	}
	if w := call("POST", body, token); w.Code != 409 {
		t.Fatal("used continuation was queued again", w.Code, w.Body.String())
	}
	for _, status := range []string{"done", "cancelled"} {
		copy := *after
		copy.Status = status
		plan, err := s.taskResumePlan(ctx, owner.ID, &copy)
		if err != nil || plan.Available {
			t.Fatal("terminal task resumable", status, err)
		}
	}
	if plan, err := s.taskResumePlan(ctx, "another-owner", after); err != nil || plan.Available {
		t.Fatal("foreign owner obtained a resume plan", err)
	}
}
