package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxPlanApprovalStartsPersistedScopeOnceWithoutRewriting(t *testing.T) {
	runInboxPlanApproval(t, false)
}
func TestInboxPlanDeferralNeverAuthorizesWorkAndCanBeApprovedLater(t *testing.T) {
	runInboxPlanApproval(t, true)
}
func TestDeniedToolRecoveryProposalRequiresSeparatePersistedApproval(t *testing.T) {
	runInboxPlanApproval(t, false, true)
}
func runInboxPlanApproval(t *testing.T, deferFirst bool, deniedFirst ...bool) {
	if testing.Short() {
		t.Skip("skipping slow plan approval test in short mode")
	}
	s, _, call, home := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "plan-project", Name: "Plan project"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	plan := inboxApprovalPlan{Title: "Verify navigation", Description: "Read-only navigation audit. Report evidence. Do not modify or deploy.", ProjectID: "plan-project"}
	encoded, _ := json.Marshal(plan)
	content := "Here is the proposed audit.\n```cuan-approval\n" + string(encoded) + "\n```"
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(content)}})
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 guide="${3##*Read the session guidance at }";guide="${guide%% before proceeding*}"
 grep -q 'Read-only navigation audit. Report evidence. Do not modify or deploy.' "$guide" || exit 21
 "$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Audit complete' --complete-note 'Read-only fixture verified; no deployment' >/dev/null || exit 22
 while :;do sleep 1;done
fi
echo provider >> "$HOME/plan-provider-calls"
printf '%s' '` + string(response) + `'
`
	if len(deniedFirst) > 0 && deniedFirst[0] {
		script = strings.Replace(script, "echo provider >>", `if [ ! -f "$HOME/plan-denied-once" ]; then
 touch "$HOME/plan-denied-once"
 printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"","denied_actions":[{"action":"blocked-command"}]}}'
 exit 0
fi
echo provider >>`, 1)
	}
	os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700)
	post := func(payload map[string]string) db.InboxRequest {
		t.Helper()
		raw, _ := json.Marshal(payload)
		w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", string(raw))
		if w.Code != 200 && w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var got struct{ Request db.InboxRequest }
		json.Unmarshal(w.Body.Bytes(), &got)
		return got.Request
	}
	first := post(map[string]string{"request_id": db.GenerateUUID(), "message": "Propose a navigation audit and ask before starting."})
	waitInboxState(t, s, first.RoomID, first.RequestID, "completed")
	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 0 {
		t.Fatal("proposal launched work")
	}
	dto, err := s.getFormattedMessagesResult(ctx, first.RoomID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range dto {
		if m.UUID == first.ReplyID && (m.ApprovalPlan == nil || m.ApprovalPlan.Description != plan.Description) {
			t.Fatal("displayed plan differs")
		}
	}
	// Ordinary user text cannot impersonate a completed agent proposal.
	bad, _ := json.Marshal(map[string]string{"request_id": db.GenerateUUID(), "message": "Approve", "approve_message_id": first.MessageID})
	if w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", string(bad)); w.Code != 409 {
		t.Fatal("forged plan approved", w.Code)
	}
	if got := s.conversationalApprovalID(ctx, first.RoomID, "Gas"); got != first.ReplyID {
		t.Fatal("natural approval not bound", got)
	}
	if got := s.conversationalApprovalID(ctx, first.RoomID, "What does this change?"); got != "" {
		t.Fatal("question treated as consent")
	}
	approvalPayload := map[string]string{"request_id": db.GenerateUUID(), "message": "Gas"}
	if deferFirst {
		deferred := post(map[string]string{"request_id": db.GenerateUUID(), "message": "Not now", "approve_message_id": first.ReplyID, "plan_action": "defer"})
		waitInboxState(t, s, deferred.RoomID, deferred.RequestID, "completed")
		repeated := post(map[string]string{"request_id": db.GenerateUUID(), "message": "Not now", "approve_message_id": first.ReplyID, "plan_action": "defer"})
		if repeated.RequestID != deferred.RequestID || repeated.MessageID != deferred.MessageID {
			t.Fatal("deferral duplicated")
		}
		tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
		if len(tasks) != 0 {
			t.Fatal("deferral started work")
		}
		if got := s.conversationalApprovalID(ctx, first.RoomID, "Gas"); got != "" {
			t.Fatal("consent attached across intervening messages")
		}
		stored, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, deferred.RoomID)
		for _, q := range stored {
			if q.RequestID == deferred.RequestID {
				deferred = q
			}
		}
		action := inboxDelegation{Title: plan.Title, Description: plan.Description, ProjectID: plan.ProjectID, AuthorizationID: deferred.MessageID}
		raw, _ := json.Marshal(action)
		if out := s.applyInboxDelegation(ctx, deferred, "```cuan-delegate\n"+string(raw)+"\n```"); !strings.Contains(out, "does not authorize") {
			t.Fatal(out)
		}
		messages, _ := s.getFormattedMessagesResult(ctx, first.RoomID, "")
		found := false
		for _, m := range messages {
			if m.UUID == first.ReplyID {
				found = m.PlanDeferred && !m.ApprovalAccepted
			}
		}
		if !found {
			t.Fatal("deferral state lost")
		}
		approvalPayload["message"] = "Approve this plan"
		approvalPayload["approve_message_id"] = first.ReplyID
	}
	approved := post(approvalPayload)
	waitInboxState(t, s, approved.RoomID, approved.RequestID, "completed")
	tasks, _ = s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 1 {
		t.Fatal("expected one task", len(tasks))
	}
	result := waitWorkspaceTask(t, s, tasks[0].ID, "succeeded")
	if !strings.Contains(result.Instructions, plan.Description) {
		t.Fatal("approved scope lost")
	}
	calls, _ := os.ReadFile(filepath.Join(home, "plan-provider-calls"))
	if string(calls) != "provider\n" {
		t.Fatal("approval unnecessarily invoked model", string(calls))
	}
	replay := post(map[string]string{"request_id": db.GenerateUUID(), "message": "Approve this plan", "approve_message_id": first.ReplyID})
	if replay.RequestID != approved.RequestID || replay.MessageID != approved.MessageID {
		t.Fatal("repeated approval not idempotent")
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, result.ID)
	if len(detail.Attempts) != 1 {
		t.Fatal("duplicate execution")
	}
	stored, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, approved.RoomID)
	for _, q := range stored {
		if q.RequestID == approved.RequestID {
			approved = q
		}
	}
	if deferFirst {
		raw, _ := json.Marshal(map[string]string{"request_id": db.GenerateUUID(), "message": "Not now", "approve_message_id": first.ReplyID, "plan_action": "defer"})
		if w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", string(raw)); w.Code != 409 {
			t.Fatal("deferral claimed to stop approved work", w.Code)
		}
	}
	// A later model response cannot substitute different scope for this approval.
	d := inboxDelegation{Title: plan.Title, Description: "Deploy unrelated changes", ProjectID: plan.ProjectID, AuthorizationID: approved.MessageID}
	raw, _ := json.Marshal(d)
	if out := s.applyInboxDelegation(ctx, approved, "```cuan-delegate\n"+string(raw)+"\n```"); !strings.Contains(out, "differs from the approved plan") {
		t.Fatal(out)
	}
	dto, _ = s.getFormattedMessagesResult(ctx, first.RoomID, "")
	for _, m := range dto {
		if m.UUID == first.ReplyID && !m.ApprovalAccepted {
			t.Fatal("approved plan still appears pending")
		}
		if strings.Contains(m.Content, "CUAN_MESSAGE_V1") {
			t.Fatal("private envelope exposed")
		}
	}
}

func TestInboxPlanApprovalWorksInCustomerRoom(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow plan approval test in short mode")
	}
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "plan-project", Name: "Plan project"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL

	custRoomID := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        custRoomID,
		ProjectID: "default",
		Title:     "Beckthe98",
		Topic:     "",
	}); err != nil {
		t.Fatal(err)
	}

	plan := inboxApprovalPlan{Title: "Add deploy wrapper", Description: "Add scripts/deploy.sh wrapper.", ProjectID: "plan-project"}
	encoded, _ := json.Marshal(plan)
	content := "Here is the proposed wrapper.\n```cuan-approval\n" + string(encoded) + "\n```"
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(content)}})
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 "$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Done' --complete-note 'Added wrapper' >/dev/null || exit 22
 exit 0
fi
printf '%s' '` + string(response) + `'
`
	os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700)

	post := func(payload map[string]string) db.InboxRequest {
		t.Helper()
		raw, _ := json.Marshal(payload)
		w := call("POST", "/api/v1/chat/conversations/"+custRoomID+"/messages", string(raw))
		if w.Code != 200 && w.Code != 202 {
			t.Fatalf("expected 200/202, got %d: %s", w.Code, w.Body.String())
		}
		var got struct{ Request db.InboxRequest }
		json.Unmarshal(w.Body.Bytes(), &got)
		return got.Request
	}

	first := post(map[string]string{"request_id": db.GenerateUUID(), "message": "@lifecycle cek script deploy"})
	waitInboxState(t, s, first.RoomID, first.RequestID, "completed")

	// Clicking Approve sends "Approve this plan" with approve_message_id and no @mention.
	approved := post(map[string]string{"request_id": db.GenerateUUID(), "message": "Approve this plan", "approve_message_id": first.ReplyID})
	waitInboxState(t, s, approved.RoomID, approved.RequestID, "completed")

	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 1 {
		t.Fatalf("expected one task from customer room plan approval, got %d", len(tasks))
	}
	if tasks[0].AssignedAgent != "agent_lifecycle" {
		t.Fatalf("expected assigned agent agent_lifecycle, got %q", tasks[0].AssignedAgent)
	}
}

func TestInboxPlanApprovalWithPriorAttachmentAndQuote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow plan approval test in short mode")
	}
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "plan-project", Name: "Plan project"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL

	// Upload a valid attachment for an earlier message in the conversation.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	f, _ := mw.CreateFormFile("file", "context.txt")
	f.Write([]byte("prior context"))
	mw.Close()
	upReq := httptest.NewRequest("POST", "/api/v1/chat/attachments", &body)
	upReq.Header.Set("Content-Type", mw.FormDataContentType())
	upReq.Header.Set("Authorization", "Bearer "+token)
	upRec := httptest.NewRecorder()
	s.Router().ServeHTTP(upRec, upReq)
	if upRec.Code != 201 {
		t.Fatalf("upload failed: %d %s", upRec.Code, upRec.Body.String())
	}
	var att conversationAttachment
	if err := json.Unmarshal(upRec.Body.Bytes(), &att); err != nil {
		t.Fatal(err)
	}

	custRoomID := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        custRoomID,
		ProjectID: "default",
		Title:     "Customer Room",
		Topic:     "",
	}); err != nil {
		t.Fatal(err)
	}

	plan := inboxApprovalPlan{Title: "Check deploy script", Description: "Verify deploy script configuration.", ProjectID: "plan-project"}
	encoded, _ := json.Marshal(plan)
	content := "Here is the proposed plan.\n```cuan-approval\n" + string(encoded) + "\n```"
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(content)}})
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 "$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Done' --complete-note 'Verified' >/dev/null || exit 22
 exit 0
fi
printf '%s' '` + string(response) + `'
`
	os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700)

	post := func(payload map[string]any) (int, string, db.InboxRequest) {
		t.Helper()
		raw, _ := json.Marshal(payload)
		w := call("POST", "/api/v1/chat/conversations/"+custRoomID+"/messages", string(raw))
		var got struct{ Request db.InboxRequest }
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		return w.Code, w.Body.String(), got.Request
	}

	// 1. User sends a message with an attachment and quote context.
	code, bodyStr, first := post(map[string]any{
		"request_id":     db.GenerateUUID(),
		"message":        "> prior quote\n\n@lifecycle cek script deploy",
		"attachment_ids": []string{att.ID},
	})
	if code != 200 && code != 202 {
		t.Fatalf("initial message failed: %d %s", code, bodyStr)
	}
	waitInboxState(t, s, first.RoomID, first.RequestID, "completed")

	// 2. Approving with a new attachment_ids slice on the approval request itself must be rejected clearly.
	badCode, badBody, _ := post(map[string]any{
		"request_id":         db.GenerateUUID(),
		"message":            "Approve this plan",
		"approve_message_id": first.ReplyID,
		"attachment_ids":     []string{att.ID},
	})
	if badCode != 400 || !strings.Contains(badBody, "Approve a plan without new attachments") {
		t.Fatalf("expected 400 without new attachments error, got %d: %s", badCode, badBody)
	}

	// 3. Approving a non-agent message in a customer room must fail with agent-conversation error, not attachment error.
	nonAgentCode, nonAgentBody, _ := post(map[string]any{
		"request_id":         db.GenerateUUID(),
		"message":            "Approve this plan",
		"approve_message_id": first.MessageID,
	})
	if nonAgentCode != 400 || strings.Contains(nonAgentBody, "attachments") {
		t.Fatalf("expected 400 agent-conversation error without attachment mention, got %d: %s", nonAgentCode, nonAgentBody)
	}

	// 4. Approving the agent plan in a room with prior attachments & quote succeeds when attachment_ids is empty.
	okCode, okBody, approved := post(map[string]any{
		"request_id":         db.GenerateUUID(),
		"message":            "Approve this plan",
		"approve_message_id": first.ReplyID,
		"attachment_ids":     []string{},
	})
	if okCode != 200 && okCode != 202 {
		t.Fatalf("expected 200/202 on approval, got %d: %s", okCode, okBody)
	}
	waitInboxState(t, s, approved.RoomID, approved.RequestID, "completed")

	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task from approval in room with prior attachments, got %d", len(tasks))
	}
}
