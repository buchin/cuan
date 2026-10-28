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
func TestInboxModelPassCannotCompleteFailedTask(t *testing.T) {
	runInboxDelegation(t, true, true, "Perform the approved audit and report evidence; do not deploy.", "bad-pass")
}

func TestInboxDeniedReviewGetsOneSnapshotFollowup(t *testing.T) {
	runInboxDelegation(t, true, false, "Audit the approved project read-only; do not deploy.", "review-denied")
}

func TestInboxInterruptionResumesSameApprovedTask(t *testing.T) {
	runInboxDelegation(t, true, true, "Build the approved audit and report evidence; do not deploy.", "interrupt")
}

func runInboxDelegation(t *testing.T, topic, managed bool, promptOverride ...string) {
	if testing.Short() {
		t.Skip("skipping slow delegation test in short mode")
	}
	interrupted := len(promptOverride) > 1 && promptOverride[1] == "interrupt"
	badPass := len(promptOverride) > 1 && promptOverride[1] == "bad-pass"
	failedFirst := interrupted || badPass
	reviewDenied := len(promptOverride) > 1 && promptOverride[1] == "review-denied"
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
	raw, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(content)}})
	verified, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": `{"decision":"pass","explanation":"Audit finished. Evidence checked against the approved request; no changes or deployment.","revision":""}`}})
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
 *'reported_verification'*) printf '%s' '` + string(verified) + `' ;;
 *) target="${2##*Target message ID: }"; printf '%s' '` + string(raw) + `' | sed "s/AUTH/$target/g" ;;
esac
`
	if reviewDenied {
		script = strings.Replace(script, "*'reported_verification'*)", `*'reported_verification'*)
 if [ ! -f "$HOME/review-denied-once" ]; then
  touch "$HOME/review-denied-once"
  printf '%s' '{"event":"result","result":{"status":"SUCCESS","response":"","denied_actions":[{"action":"run_command"}]}}'
  exit 0
 fi
`, 1)
	}
	if failedFirst {
		script = strings.Replace(script, `if [ "$1" = "--dangerously-skip-permissions" ]; then`, `if [ "$1" = "--dangerously-skip-permissions" ]; then
 if [ ! -f "$HOME/interrupted-once" ]; then
  printf 'PRESERVED_TASK_ARTIFACT' > retained-before-interruption.txt
  pwd > "$HOME/interruption-workspace"
  touch "$HOME/interrupted-once"
  exit 137
 fi
 grep -q PRESERVED_TASK_ARTIFACT retained-before-interruption.txt || exit 25`, 1)
		if badPass {
			script = strings.Replace(script, "exit 137", "exit 3", 1)
		}
	}
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
	var retainedPath string
	var retainedBefore os.FileInfo
	if failedFirst {
		failed := waitWorkspaceTask(t, s, tasks[0].ID, "failed")
		if failed.Summary != "" || failed.CompleteNote != "" {
			t.Fatal("fixture must interrupt before final report")
		}
		workspace, err := os.ReadFile(filepath.Join(home, "interruption-workspace"))
		if err != nil {
			t.Fatal(err)
		}
		retainedPath = filepath.Join(strings.TrimSpace(string(workspace)), "retained-before-interruption.txt")
		retainedBefore, err = os.Stat(retainedPath)
		if err != nil {
			t.Fatal(err)
		}
		s.deliverInboxTaskUpdates(ctx)
	}
	result := waitWorkspaceTask(t, s, tasks[0].ID, "succeeded")
	if failedFirst {
		after, err := os.Stat(retainedPath)
		if err != nil || !os.SameFile(retainedBefore, after) {
			t.Fatal("existing artifact replaced", err)
		}
		if len(result.Attempts) != 2 {
			t.Fatal("recovery did not continue same task", len(result.Attempts))
		}
	}
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
	if reviewDenied {
		waitInboxState(t, s, accepted.Request.RoomID, reviewID, "failed")
		s.deliverInboxTaskUpdates(ctx)
		reviewID = delegationMessageID("task-review:" + result.LatestAttempt.ID + ":snapshot")
	}
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
	for _, text := range []string{humanPrompt, "reported_verification", "Do not use tools"} {
		if !strings.Contains(string(prompt), text) {
			t.Fatal("missing relevant context", text)
		}
	}
	if failedFirst || reviewDenied {
		return
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

func TestInboxDelegationContinuationInSameRoomAllowsRetry(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "p1", Name: "One"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()

	// Initial task created in the room
	initMsgID := db.GenerateUUID()
	initReq := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: initMsgID, ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "init", CreatedAt: now, Deadline: now.Add(time.Hour)}
	initMsg := &db.ProjectMessage{ID: initMsgID, RoomID: room.RoomID, AuthorType: "user", Content: "Start initial task", CreatedAt: now}
	s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, initReq, initMsg)

	initDelegation := inboxDelegation{Action: "run", Title: "Initial Task", Description: "Initial task description", ProjectID: "p1", AuthorizationID: initMsgID}
	raw, _ := json.Marshal(initDelegation)
	out := s.applyInboxDelegation(ctx, initReq, "```cuan-delegate\n"+string(raw)+"\n```")
	if !strings.Contains(out, "started") {
		t.Fatalf("expected initial task to start, got: %s", out)
	}

	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	existingTaskID := tasks[0].ID

	// Mark task as blocked (e.g. after attempts reached max or error)
	statusBlocked := task.TaskStatusBlocked
	s.taskSvc.UpdateTask(ctx, existingTaskID, "coordinator", task.UpdateTaskRequest{Status: &statusBlocked})
	s.terminalManager.CloseSession(existingTaskID)
	for i := 0; i < 50; i++ {
		if active, _ := s.terminalManager.ProjectExecutionActive("p1"); !active {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// User sends a new message in the same room ("Gas", "Lanjut")
	continuationMsgID := db.GenerateUUID()
	continuationReq := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: continuationMsgID, ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "cont", CreatedAt: now.Add(time.Minute), Deadline: now.Add(2 * time.Hour)}
	continuationMsg := &db.ProjectMessage{ID: continuationMsgID, RoomID: room.RoomID, AuthorType: "user", Content: "Gas", CreatedAt: now.Add(time.Minute)}
	s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, continuationReq, continuationMsg)

	// Coordinator delegates continuation for the existing task with new user message as AuthorizationID
	continuationDelegation := inboxDelegation{
		Action:          "run",
		TaskID:          existingTaskID,
		ProjectID:       "p1",
		AuthorizationID: continuationMsgID,
		Revision:        "Continue previous work with fixes",
	}
	rawCont, _ := json.Marshal(continuationDelegation)
	contOut := s.applyInboxDelegation(ctx, continuationReq, "```cuan-delegate\n"+string(rawCont)+"\n```")

	if strings.Contains(contOut, "outside this approved conversation scope") {
		t.Fatalf("continuation wrongly rejected as outside scope: %s", contOut)
	}
	if strings.Contains(contOut, "retry limit") {
		t.Fatalf("continuation wrongly rejected due to retry limit: %s", contOut)
	}
	if !strings.Contains(contOut, "started") {
		t.Fatalf("expected continuation to start, got: %s", contOut)
	}

	// Verify task status was reset to doing or review
	updatedDetail, _ := s.taskSvc.GetTaskDetail(ctx, existingTaskID)
	if updatedDetail.Status != task.TaskStatusDoing && updatedDetail.Status != task.TaskStatusReview {
		t.Fatalf("expected task status to be doing or review, got: %s", updatedDetail.Status)
	}
}

func TestInboxDelegation_CrossRoomTaskDelegationAllowedForOwnerAndTeam(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "p1", Name: "Project One"})
	projects.CreateProject(ctx, &project.Project{ID: "p2", Name: "Project Two"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())

	ownerID := "owner_team_1"
	now := time.Now().UTC()

	// Room 1: Initial task creation
	room1ID := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        room1ID,
		ProjectID: "p1",
		Title:     "Room 1",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	initMsgID := db.GenerateUUID()
	initReq := db.InboxRequest{
		OwnerID:     ownerID,
		AgentID:     "agent_lifecycle",
		RoomID:      room1ID,
		RequestID:   db.GenerateUUID(),
		MessageID:   initMsgID,
		ReplyID:     db.GenerateUUID(),
		State:       "queued",
		PayloadHash: "init",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
	}
	initMsg := &db.ProjectMessage{
		ID:         initMsgID,
		RoomID:     room1ID,
		AuthorType: "user",
		AuthorName: "Team Lead",
		Content:    "Start feature implementation",
		CreatedAt:  now,
	}
	s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, initReq, initMsg)

	initDelegation := inboxDelegation{
		Action:          "save",
		Title:           "Feature Implementation",
		Description:     "Implement feature.\n- [ ] Task 1 done",
		ProjectID:       "p1",
		AuthorizationID: initMsgID,
	}
	rawInit, _ := json.Marshal(initDelegation)
	outInit := s.applyInboxDelegation(ctx, initReq, "```cuan-delegate\n"+string(rawInit)+"\n```")
	if strings.Contains(outInit, "could not") {
		t.Fatalf("failed to create initial task in room1: %s", outInit)
	}

	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{ProjectID: "p1"})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("could not retrieve saved task: %v", err)
	}
	existingTaskID := tasks[0].ID

	// Room 2: Different room context (cross-room / cross-session)
	room2ID := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        room2ID,
		ProjectID: "p2",
		Title:     "Discussion in Room 2",
		CreatedAt: now.Add(time.Hour),
		UpdatedAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	crossRoomMsgID := db.GenerateUUID()
	crossRoomReq := db.InboxRequest{
		OwnerID:     ownerID,
		AgentID:     "agent_lifecycle",
		RoomID:      room2ID,
		RequestID:   db.GenerateUUID(),
		MessageID:   crossRoomMsgID,
		ReplyID:     db.GenerateUUID(),
		State:       "queued",
		PayloadHash: "cross",
		CreatedAt:   now.Add(2 * time.Hour),
		Deadline:    now.Add(4 * time.Hour),
	}
	crossRoomMsg := &db.ProjectMessage{
		ID:         crossRoomMsgID,
		RoomID:     room2ID,
		AuthorType: "user",
		AuthorName: "Team Member",
		Content:    "Lanjut perbaikan untuk task ini",
		CreatedAt:  now.Add(2 * time.Hour),
	}
	s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, crossRoomReq, crossRoomMsg)

	// Cross-room delegation for the task created in Room 1
	crossDelegation := inboxDelegation{
		Action:          "run",
		TaskID:          existingTaskID,
		AuthorizationID: crossRoomMsgID,
		Revision:        "Fix edge cases in feature.\n- [ ] Edge cases handled",
	}
	rawCross, _ := json.Marshal(crossDelegation)
	outCross := s.applyInboxDelegation(ctx, crossRoomReq, "```cuan-delegate\n"+string(rawCross)+"\n```")
	t.Logf("outCross: %s", outCross)

	if strings.Contains(outCross, "outside this approved conversation scope") {
		t.Fatalf("cross-room delegation wrongly rejected with scope error: %s", outCross)
	}
	if strings.Contains(outCross, "belongs to another scope") {
		t.Fatalf("cross-room delegation wrongly rejected with belongs to another scope: %s", outCross)
	}
	if !strings.Contains(outCross, "started") && !strings.Contains(outCross, "The work is") {
		t.Fatalf("expected cross-room delegation to succeed, got: %s", outCross)
	}

	// Verify DoD checklist was expanded with the new revision item
	taskDetail, err := s.taskSvc.GetTaskDetail(ctx, existingTaskID)
	if err != nil {
		t.Fatalf("could not load updated task: %v", err)
	}
	hasEdgeCases := false
	for _, item := range taskDetail.DoDChecklist {
		if strings.Contains(item.Text, "Edge cases handled") {
			hasEdgeCases = true
			break
		}
	}
	if !hasEdgeCases {
		t.Fatalf("expected updated DoD checklist to include new item from revision, got: %+v", taskDetail.DoDChecklist)
	}
}

func TestInboxDelegation_ArchivedDefaultFallbackToActiveProject(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	// Archive the default project and register an active project
	projects.CreateProject(ctx, &project.Project{ID: "default", Name: "Default", IsArchived: true})
	projects.CreateProject(ctx, &project.Project{ID: "proj_cuan_active", Name: "Cuan Active"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())

	resolved := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: resolved.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "approval", CreatedAt: now, Deadline: now.Add(time.Hour)}

	planPayload := attachmentMessage{
		Text: "Approved: Account UI Cleanup",
		Approval: &approvedInboxPlan{
			Action:    "approve",
			MessageID: db.GenerateUUID(),
			Plan: inboxApprovalPlan{
				Title:       "Account UI Cleanup",
				Description: "Cleanup redundant account popover menu items",
				ProjectName: "Default",
			},
		},
	}
	rawMsg, _ := json.Marshal(planPayload)
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: attachmentEnvelope + string(rawMsg), CreatedAt: now}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}

	delegation := inboxDelegation{
		Action:          "run",
		Title:           "Account UI Cleanup",
		Description:     "Cleanup redundant account popover menu items",
		ProjectName:     "Default",
		AuthorizationID: q.MessageID,
	}
	raw, _ := json.Marshal(delegation)
	out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(raw)+"\n```")

	if strings.Contains(out, "could not be prepared") || strings.Contains(out, "archived project") {
		t.Fatalf("delegation should not fail on archived Default project: %s", out)
	}
	if !strings.Contains(out, "started") {
		t.Fatalf("expected delegation to start work on active project, got: %s", out)
	}
}

func TestExtractDoDFromDescription(t *testing.T) {
	// Test numbered list
	desc1 := `Revisi UI Tasks:
1. Eliminasi ikon kaca pembesar di top bar
2. Ganti dropdown select dengan horizontal scrollable filter chips
3. Sederhanakan label Agent @sm
4. Pastikan build lulus verifikasi`
	items1 := extractDoDFromDescription(desc1)
	if len(items1) != 4 {
		t.Fatalf("expected 4 items from numbered list, got %d", len(items1))
	}
	if items1[0].Text != "Eliminasi ikon kaca pembesar di top bar" {
		t.Errorf("unexpected item 0: %s", items1[0].Text)
	}
	if items1[0].Completed {
		t.Errorf("expected item 0 to not be completed")
	}

	// Test markdown checkboxes
	desc2 := `Checklist pengerjaan:
- [ ] Task unit test passed
- [x] Backend endpoint verified
- [ ] Responsive design checked`
	items2 := extractDoDFromDescription(desc2)
	if len(items2) != 3 {
		t.Fatalf("expected 3 items from checkboxes, got %d", len(items2))
	}
	if !items2[1].Completed {
		t.Errorf("expected item 1 to be completed")
	}

	// Test explicit DoD header with bullets
	desc3 := `Implementation plan:
DoD:
- Verify API status 200
- Run smoke tests`
	items3 := extractDoDFromDescription(desc3)
	if len(items3) != 2 {
		t.Fatalf("expected 2 items from DoD block, got %d", len(items3))
	}
}

func TestInboxDelegation_PopulatesDoDChecklist(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	resolved := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: resolved.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "approval", CreatedAt: now, Deadline: now.Add(time.Hour)}
	planPayload := attachmentMessage{
		Text: "Approved: Tasks UI Mobile Cleanup",
		Approval: &approvedInboxPlan{
			Action:    "approve",
			MessageID: db.GenerateUUID(),
			Plan: inboxApprovalPlan{
				Title:       "Tasks UI Mobile Cleanup",
				Description: "Revisi UI Tasks:\n1. Eliminasi ikon search\n2. Ganti dropdown select dengan filter chips\n3. Sederhanakan label Agent @sm\n4. Pastikan build lulus verifikasi",
				ProjectName: "Default",
			},
		},
	}
	rawMsg, _ := json.Marshal(planPayload)
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: attachmentEnvelope + string(rawMsg), CreatedAt: now}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}

	delegation := inboxDelegation{
		Action:          "run",
		Title:           "Tasks UI Mobile Cleanup",
		Description:     "Revisi UI Tasks:\n1. Eliminasi ikon search\n2. Ganti dropdown select dengan filter chips\n3. Sederhanakan label Agent @sm\n4. Pastikan build lulus verifikasi",
		ProjectName:     "Default",
		AuthorizationID: q.MessageID,
	}
	raw, _ := json.Marshal(delegation)
	out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(raw)+"\n```")
	if strings.Contains(out, "could not") {
		t.Fatalf("unexpected delegation failure: %s", out)
	}

	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: q.RoomID})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("expected created task, err: %v", err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, tasks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.DoDChecklist) != 4 {
		t.Fatalf("expected 4 DoD checklist items populated on task, got %d", len(detail.DoDChecklist))
	}
	if detail.DoDChecklist[0].Text != "Eliminasi ikon search" {
		t.Errorf("unexpected DoD item 0: %s", detail.DoDChecklist[0].Text)
	}
}

func TestInboxDelegation_PreservesAIProseExplanation(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	resolved := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{
		OwnerID:     "owner",
		AgentID:     "agent_lifecycle",
		RoomID:      resolved.RoomID,
		RequestID:   db.GenerateUUID(),
		MessageID:   db.GenerateUUID(),
		ReplyID:     db.GenerateUUID(),
		State:       "queued",
		PayloadHash: "approval",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
	}
	planPayload := attachmentMessage{
		Text: "Approved: Investigasi terminal",
		Approval: &approvedInboxPlan{
			Action:    "approve",
			MessageID: q.MessageID,
			Plan: inboxApprovalPlan{
				Title:       "Investigasi terminal",
				Description: "Periksa input terminal dan websocket stream",
				ProjectName: "Default",
			},
		},
	}
	rawMsg, _ := json.Marshal(planPayload)
	message := &db.ProjectMessage{
		ID:         q.MessageID,
		RoomID:     q.RoomID,
		AuthorType: "user",
		Content:    attachmentEnvelope + string(rawMsg),
		CreatedAt:  now,
	}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}

	explanation := "Penyebabnya adalah koneksi stdin sedang menunggu proses selesai."
	delegation := inboxDelegation{
		Action:          "run",
		Title:           "Investigasi terminal",
		Description:     "Periksa input terminal dan websocket stream",
		ProjectName:     "Default",
		AuthorizationID: q.MessageID,
	}
	raw, _ := json.Marshal(delegation)
	combinedInput := explanation + "\n\n```cuan-delegate\n" + string(raw) + "\n```"

	out := s.applyInboxDelegation(ctx, q, combinedInput)
	if !strings.HasPrefix(out, explanation) {
		t.Fatalf("expected output to preserve AI explanation as prefix, got: %s", out)
	}
	if !strings.Contains(out, "View work") {
		t.Fatalf("expected output to contain task link, got: %s", out)
	}
}

func TestInboxDelegation_CustomerRoomAllowsMultiProjectDelegation(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// Register a target project "dojo"
	projectsStore := project.NewMemoryStore()
	if err := projectsStore.CreateProject(ctx, &project.Project{
		ID:   "proj_dojo_test",
		Name: "dojo",
	}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projectsStore, nil, "", t.TempDir())

	// Simulate a visitor/customer room with ProjectID "default"
	roomID := db.GenerateUUID()
	now := time.Now().UTC()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Visitor #test",
		Topic:     "",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	q := db.InboxRequest{
		OwnerID:     "owner",
		AgentID:     "agent_lifecycle",
		RoomID:      roomID,
		RequestID:   db.GenerateUUID(),
		MessageID:   db.GenerateUUID(),
		ReplyID:     db.GenerateUUID(),
		State:       "queued",
		PayloadHash: "approval",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
	}

	planPayload := attachmentMessage{
		Text: "Approved: Aktivasi Akses PAY-74",
		Approval: &approvedInboxPlan{
			Action:    "approve",
			MessageID: q.MessageID,
			Plan: inboxApprovalPlan{
				Title:       "Aktivasi Akses PAY-74",
				Description: "Verifikasi payment dan aktifkan akses di Dojo",
				ProjectID:   "proj_dojo_test",
				ProjectName: "dojo",
			},
		},
	}
	rawMsg, _ := json.Marshal(planPayload)
	message := &db.ProjectMessage{
		ID:         q.MessageID,
		RoomID:     q.RoomID,
		AuthorType: "user",
		Content:    attachmentEnvelope + string(rawMsg),
		CreatedAt:  now,
	}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}

	// Delegate using project name "dojo"
	delegation := inboxDelegation{
		Action:          "run",
		Title:           "Aktivasi Akses PAY-74",
		Description:     "Verifikasi payment dan aktifkan akses di Dojo",
		ProjectID:       "proj_dojo_test",
		ProjectName:     "dojo",
		AuthorizationID: q.MessageID,
	}
	raw, _ := json.Marshal(delegation)
	out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(raw)+"\n```")
	if strings.Contains(out, "could not") || strings.Contains(out, "another project") {
		t.Fatalf("expected delegation to succeed from default room to dojo project, got: %s", out)
	}
	if !strings.Contains(out, "View work") {
		t.Fatalf("expected view work message, got: %s", out)
	}

	// Verify room's project was updated to target project
	updatedRoom, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedRoom.ProjectID != "proj_dojo_test" {
		t.Fatalf("expected room project_id to be updated to proj_dojo_test, got: %s", updatedRoom.ProjectID)
	}
}

func TestInboxDelegation_ResolvesProjectByName(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	projectsStore := project.NewMemoryStore()
	if err := projectsStore.CreateProject(ctx, &project.Project{
		ID:   "proj_dojo_uuid",
		Name: "dojo",
	}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projectsStore, nil, "", t.TempDir())

	roomID := db.GenerateUUID()
	now := time.Now().UTC()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Visitor #test2",
		Topic:     "",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	q := db.InboxRequest{
		OwnerID:     "owner",
		AgentID:     "agent_lifecycle",
		RoomID:      roomID,
		RequestID:   db.GenerateUUID(),
		MessageID:   db.GenerateUUID(),
		ReplyID:     db.GenerateUUID(),
		State:       "queued",
		PayloadHash: "approval",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
	}

	planPayload := attachmentMessage{
		Text: "Approved: Cek Akses User",
		Approval: &approvedInboxPlan{
			Action:    "approve",
			MessageID: q.MessageID,
			Plan: inboxApprovalPlan{
				Title:       "Cek Akses User",
				Description: "Periksa akses lisensi akun di dojo",
				ProjectID:   "dojo", // Passed as project name
				ProjectName: "dojo",
			},
		},
	}
	rawMsg, _ := json.Marshal(planPayload)
	message := &db.ProjectMessage{
		ID:         q.MessageID,
		RoomID:     q.RoomID,
		AuthorType: "user",
		Content:    attachmentEnvelope + string(rawMsg),
		CreatedAt:  now,
	}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}

	delegation := inboxDelegation{
		Action:          "run",
		Title:           "Cek Akses User",
		Description:     "Periksa akses lisensi akun di dojo",
		ProjectID:       "dojo", // Passed as project name
		ProjectName:     "dojo",
		AuthorizationID: q.MessageID,
	}
	raw, _ := json.Marshal(delegation)
	out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(raw)+"\n```")
	if strings.Contains(out, "could not") || strings.Contains(out, "another project") {
		t.Fatalf("expected delegation to resolve project name 'dojo' to uuid, got: %s", out)
	}
	if !strings.Contains(out, "View work") {
		t.Fatalf("expected view work message, got: %s", out)
	}
}
