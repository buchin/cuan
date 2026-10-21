package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxDelegationProtocol(t *testing.T) {
	valid := "Plan\n```cuan-delegate\n{\"title\":\"Audit\",\"description\":\"Approved audit\",\"project_id\":\"p1\",\"authorization_message_id\":\"user-id\"}\n```"
	text, d, err := parseInboxDelegation(valid)
	if err != nil || d == nil || text != "Plan" {
		t.Fatal(text, d, err)
	}
	for _, bad := range []string{valid + " trailing", valid + valid, strings.Replace(valid, "\"p1\"", "\"\"", 1), strings.Replace(valid, "\"title\":", "\"shell\":\"dangerous\",\"title\":", 1), strings.Replace(valid, "}\n```", "} {}\n```", 1), valid + "\n```cuan-task\n{}\n```"} {
		if _, _, err := parseInboxDelegation(bad); err == nil {
			t.Fatal("invalid action accepted", bad)
		}
	}
	if _, d, err := parseInboxDelegation("Here is the plan. Shall I proceed?"); err != nil || d != nil {
		t.Fatal("plan executed")
	}
}

func TestInboxDelegationRunsAndReturnsResult(t *testing.T) {
	for _, topic := range []bool{false, true} {
		name := "legacy"
		if topic {
			name = "topic"
		}
		t.Run(name, func(t *testing.T) { runInboxDelegation(t, topic, false) })
	}
}

func TestInboxDelegationManagedProject(t *testing.T) { runInboxDelegation(t, true, true) }

func TestInboxDelegationClearInstructionNeedsNoPlanningTurn(t *testing.T) {
	runInboxDelegation(t, true, false, "Audit this project read-only and report the findings. Do not deploy.")
}
func runInboxDelegation(t *testing.T, topic, managed bool, promptOverride ...string) {
	s, token, call, home := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "delegate-project", Name: "Delegate project"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	content := "```cuan-delegate\n{\"title\":\"Audit security\",\"description\":\"Read-only security audit. Report findings and evidence; do not modify or deploy.\",\"project_id\":\"delegate-project\",\"authorization_message_id\":\"AUTH\"}\n```"
	if managed {
		content = strings.Replace(content, `"project_id":"delegate-project"`, `"project_name":"Managed audit"`, 1)
	}
	raw, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": content}})
	verified, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": "Audit finished. Evidence checked against the approved request; no changes or deployment."}})
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 if [ -f "$HOME/attachment-fixture-path" ]; then p=$(cat "$HOME/attachment-fixture-path"); grep -q APPROVED_FILE_SENTINEL "$p" || exit 22; fi
 "$CUAN_TASK_EXECUTABLE" task report --progress 'Checking the approved scope' >/dev/null || exit 20
 "$CUAN_TASK_EXECUTABLE" task report --state done --summary 'Audit fixture complete' --complete-note 'Verified deterministic audit evidence; no code changes' >/dev/null || exit 21
 pwd > "$HOME/task-working-directory"
 printf 'AUDIT_RESULT_RETAINED\n'
 while :; do sleep 1; done
fi
printf '%s' "$2" >> "$HOME/delegation-prompts"
case "$2" in
 *'automated result is not new user authorization'*) printf '%s' '` + string(verified) + `' ;;
 *) target="${2##*Target message ID: }"; printf '%s' '` + string(raw) + `' | sed "s/AUTH/$target/g" ;;
esac
`
	if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	conversation := "agent_lifecycle"
	if topic {
		raw, _ := json.Marshal(map[string]string{"agent_id": "agent_lifecycle", "request_id": db.GenerateUUID()})
		response := call("POST", "/api/v1/chat/conversations", string(raw))
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var c ChatContact
		json.Unmarshal(response.Body.Bytes(), &c)
		conversation = c.ConversationID
	}
	var attachmentIDs []string
	var fixturePath string
	if managed {
		owner, e := s.authSvc.ValidateToken(ctx, token)
		if e != nil {
			t.Fatal(e)
		}
		a := conversationAttachment{ID: db.GenerateUUID(), Name: "approved-notes.txt", Type: "text/plain", Size: 22}
		a.URL = "/api/v1/chat/attachments/" + a.ID
		dir, _ := attachmentDirectory(owner.ID, a.ID)
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		fixturePath = filepath.Join(dir, "content.txt")
		os.WriteFile(fixturePath, []byte("APPROVED_FILE_SENTINEL"), 0600)
		raw, _ := json.Marshal(a)
		os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0600)
		attachmentIDs = []string{a.ID}
		os.WriteFile(filepath.Join(home, "attachment-fixture-path"), []byte(fixturePath), 0600)
	}
	requestID := db.GenerateUUID()
	humanPrompt := "Create a read-only security audit task and run it. Report here; do not deploy."
	if len(promptOverride) > 0 {
		humanPrompt = promptOverride[0]
	}
	body, _ := json.Marshal(map[string]any{"attachment_ids": attachmentIDs, "request_id": requestID, "message": humanPrompt})
	w := call("POST", "/api/v1/chat/conversations/"+conversation+"/messages", string(body))
	if w.Code != 202 && w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct{ Request db.InboxRequest }
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	waitInboxState(t, s, accepted.Request.RoomID, requestID, "completed")
	requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, accepted.Request.RoomID)
	for _, q := range requests {
		if q.RequestID == requestID {
			accepted.Request = q
		}
	}
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatal(tasks, err)
	}
	result := waitWorkspaceTask(t, s, tasks[0].ID, "succeeded")
	if managed {
		p, err := s.projectSvc.Get(ctx, result.ProjectID)
		if err != nil || p.Name != "Managed audit" || p.LocalPath == "" {
			t.Fatal(p, err)
		}
		working, err := os.ReadFile(filepath.Join(home, "task-working-directory"))
		if err != nil || strings.TrimSpace(string(working)) != p.LocalPath {
			t.Fatal("managed workspace not used", string(working), err)
		}
	}

	if managed && !strings.Contains(result.Instructions, fixturePath) {
		t.Fatal("delegated task lost its attachment")
	}
	if result.Status != "done" || !strings.Contains(result.Instructions, "do not deploy") {
		t.Fatal(result)
	}
	msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, accepted.Request.RoomID)
	found := false
	for _, m := range msgs {
		if m.ID == accepted.Request.ReplyID {
			found = strings.Contains(m.Content, "Work started") && !strings.Contains(m.Content, "cuan-delegate")
		}
	}
	if !found {
		t.Fatal("missing actual launch confirmation", msgs)
	}
	// Persisted launch request can be replayed without executing again.
	before := len(result.Attempts)
	replay := strings.Replace(content, "AUTH", accepted.Request.MessageID, 1)
	response := s.applyInboxDelegation(ctx, accepted.Request, replay)
	if !strings.Contains(response, "already handled") {
		t.Fatal(response)
	}
	// A different trigger must reuse the approval's launch identity too.
	otherTrigger := accepted.Request
	otherTrigger.RequestID = db.GenerateUUID()
	otherTrigger.MessageID = db.GenerateUUID()
	otherTrigger.CreatedAt = time.Now().UTC()
	if out := s.applyInboxDelegation(ctx, otherTrigger, replay); !strings.Contains(out, "already handled") {
		t.Fatal("approval replay launched again", out)
	}
	s.deliverInboxTaskUpdates(ctx)
	reviewID := delegationMessageID("task-review:" + result.LatestAttempt.ID)
	waitInboxState(t, s, accepted.Request.RoomID, reviewID, "completed")
	s.deliverInboxTaskUpdates(ctx)
	result, _ = s.taskSvc.GetTaskDetail(ctx, tasks[0].ID)
	if len(result.Attempts) != before {
		t.Fatal("replayed work")
	}
	msgs, _ = s.projectStudioRepo.ListMessagesByRoomID(ctx, accepted.Request.RoomID)
	reviews := 0
	for _, m := range msgs {
		if strings.Contains(m.Content, "Audit finished. Evidence checked") {
			reviews++
		}
	}
	if reviews != 1 {
		t.Fatal("missing or duplicate coordinator result", reviews)
	}
	dtos, err := s.getFormattedMessagesResult(ctx, accepted.Request.RoomID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range dtos {
		if m.AuthorType == "system" && strings.Contains(m.Content, "authorization") {
			t.Fatal("internal review context exposed")
		}
	}
	prompt, _ := os.ReadFile(filepath.Join(home, "delegation-prompts"))
	for _, text := range []string{"Product Owner", "Keep these terms internal", humanPrompt, "Verification evidence", "without calling native tools", "Have the task session inspect its existing workspace"} {
		if !strings.Contains(string(prompt), text) {
			t.Fatal("missing relevant context", text)
		}
	}
	// A system result or foreign message cannot be used as user approval.
	for _, id := range []string{delegationMessageID("task-review:" + result.LatestAttempt.ID + ":input"), db.GenerateUUID()} {
		bad := strings.Replace(content, "AUTH", id, 1)
		if got := s.applyInboxDelegation(ctx, db.InboxRequest{RoomID: accepted.Request.RoomID, OwnerID: accepted.Request.OwnerID, AgentID: "agent_lifecycle", CreatedAt: time.Now()}, bad); !strings.Contains(got, "approval must come") {
			t.Fatal(got)
		}
	}
	// The coordinator can return concrete corrections to the same task, but the
	// server bounds autonomous retries rather than spawning unlimited sessions.
	for i := 0; i < 2; i++ {
		revision, _ := json.Marshal(inboxDelegation{ProjectID: result.ProjectID, AuthorizationID: accepted.Request.MessageID, TaskID: result.ID, Revision: "Gap: verify the audit evidence. Area: approved security audit. Check: deterministic fixture. Finish: report verified evidence without code changes."})
		q := accepted.Request
		q.RequestID = db.GenerateUUID()
		q.CreatedAt = time.Now()
		response = s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(revision)+"\n```")
		if !strings.Contains(response, "Work started") {
			t.Fatal(response)
		}
		result = waitWorkspaceTask(t, s, result.ID, "succeeded")
	}
	if len(result.Attempts) != 3 {
		t.Fatal("revision did not resume the bound task", len(result.Attempts))
	}
	revision, _ := json.Marshal(inboxDelegation{ProjectID: result.ProjectID, AuthorizationID: accepted.Request.MessageID, TaskID: result.ID, Revision: "Repeat indefinitely"})
	q := accepted.Request
	q.RequestID = db.GenerateUUID()
	q.CreatedAt = time.Now()
	if response = s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(revision)+"\n```"); !strings.Contains(response, "retry limit") {
		t.Fatal(response)
	}

}

func TestInboxDelegationSaveAndApprovalProvenance(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "p1", Name: "One"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	resolved := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: resolved.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "save", CreatedAt: now, Deadline: now.Add(time.Hour)}
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Save the audit for later. Do not run it.", CreatedAt: now}
	action := inboxDelegation{Action: "save", Title: "Audit", Description: "Read-only audit", ProjectID: "p1", AuthorizationID: q.MessageID}
	raw, _ := json.Marshal(action)
	content := "```cuan-delegate\n" + string(raw) + "\n```"
	// A user-shaped message alone is insufficient: authenticated acceptance is required.
	s.projectStudioRepo.CreateMessage(ctx, message)
	if out := s.applyInboxDelegation(ctx, q, content); !strings.Contains(out, "approval must come") {
		t.Fatal(out)
	}
	// Use a fresh authenticated message for the actual save.
	q.MessageID = db.GenerateUUID()
	message.ID = q.MessageID
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}
	action.AuthorizationID = q.MessageID
	raw, _ = json.Marshal(action)
	content = "```cuan-delegate\n" + string(raw) + "\n```"
	if out := s.applyInboxDelegation(ctx, q, content); !strings.Contains(out, "has not been started") {
		t.Fatal(out)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+q.MessageID)
	if err != nil || len(detail.Attempts) != 0 || detail.Status != "backlog" {
		t.Fatal(detail, err)
	}
}

func TestInboxDelegationApprovalScopeIsStableAcrossTriggerMessages(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "p1", Name: "One"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "save", CreatedAt: now, Deadline: now.Add(time.Hour)}
	m := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Save the read-only audit. Do not run.", CreatedAt: now}
	s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, m)
	d := inboxDelegation{Action: "save", Title: "Audit", Description: "Read-only audit", ProjectID: "p1", AuthorizationID: q.MessageID}
	apply := func() string {
		raw, _ := json.Marshal(d)
		return s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(raw)+"\n```")
	}
	if out := apply(); !strings.Contains(out, "has not been started") {
		t.Fatal(out)
	}
	// Another trigger referring to the same approval must not create another task.
	q.MessageID = db.GenerateUUID()
	q.RequestID = db.GenerateUUID()
	if out := apply(); !strings.Contains(out, "has not been started") {
		t.Fatal(out)
	}
	for _, change := range []string{"description", "run"} {
		if change == "description" {
			d.Description = "Modify and deploy everything"
		} else {
			d.Description = "Read-only audit"
			d.Action = "run"
		}
		if out := apply(); !strings.Contains(out, "another scope") {
			t.Fatal("changed approval accepted", out)
		}
	}
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatal(tasks, err)
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, tasks[0].ID)
	if len(detail.Attempts) != 0 || detail.SourceMessageID != d.AuthorizationID {
		t.Fatal("approval identity or save-only authorization lost")
	}
}
