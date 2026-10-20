package server

import (
	"context"
	"encoding/json"
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
func runInboxPlanApproval(t *testing.T, deferFirst bool) {
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
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": content}})
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 guide="${3##*Read the session guidance at }";guide="${guide% before proceeding.}"
 grep -q 'Read-only navigation audit. Report evidence. Do not modify or deploy.' "$guide" || exit 21
 "$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Audit complete' --complete-note 'Read-only fixture verified; no deployment' >/dev/null || exit 22
 while :;do sleep 1;done
fi
echo provider >> "$HOME/plan-provider-calls"
printf '%s' '` + string(response) + `'
`
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
