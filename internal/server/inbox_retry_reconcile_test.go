package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxReconciliationRejectsToolsAndUnscopedActions(t *testing.T) {
	for _, scenario := range []string{"answer", "no_attachment", "read_attachment", "search_tool", "tool", "delegation", "memory", "reconcile"} {
		t.Run(scenario, func(t *testing.T) {
			bin, home := t.TempDir(), t.TempDir()
			reply := coordinatorJSON("Current evidence checked; previous actions were not replayed.", "", nil)
			if scenario == "delegation" {
				reply = coordinatorJSON("Starting duplicate work", "delegation", map[string]string{"title": "Duplicate", "description": "Repeat previous action", "project_id": "default", "authorization_message_id": "source"})
			}
			if scenario == "memory" {
				reply = coordinatorJSON("Saving again", "memory", map[string]string{"content": "Unexpected mutation"})
			}
			if scenario == "reconcile" {
				reply = coordinatorJSON("I will verify current state before continuing the approved work.", "delegation", inboxDelegation{Reconcile: true, Title: "Verify approved work", Description: "Check current state and finish only missing original work.", ProjectID: "default", AuthorizationID: "source"})
			}
			result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": reply}})
			prefix := ""
			if scenario == "tool" {
				prefix = `{"event":"step_update","step_update":{"step_index":1,"step_type":"tool","state":"DONE","tool_name":"run_command"}}` + "\n"
			} else if scenario == "read_attachment" {
				prefix = `{"event":"step_update","step_update":{"step_index":1,"step_type":"tool","state":"DONE","tool_name":"read_file","tool_info":{"parameters":{"Path":"receipt.jpg"}}}}` + "\n"
			} else if scenario == "search_tool" {
				prefix = `{"event":"step_update","step_update":{"step_index":1,"step_type":"tool","state":"DONE","tool_name":"grep_search","tool_info":{"parameters":{"Query":"secret"}}}}` + "\n"
			}
			attachmentDir := t.TempDir()
			attachment := filepath.Join(attachmentDir, "receipt.jpg")
			if err := os.WriteFile(attachment, []byte("private receipt fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			attachments := []string{attachment}
			addDirCheck := `previous=""
saw_attachment_dir=0
for arg in "$@"; do
 if [ "$previous" = "--add-dir" ]; then
  [ "$arg" = "` + attachmentDir + `" ] || exit 25
  saw_attachment_dir=1
 fi
 previous="$arg"
done
[ "$saw_attachment_dir" = "1" ] || exit 27
`
			if scenario == "no_attachment" {
				attachments = nil
				addDirCheck = `for arg in "$@"; do [ "$arg" != "--add-dir" ] || exit 25; done
`
			}
			schemaCheck := `previous=""
for arg in "$@"; do
 if [ "$previous" = "--json-schema" ]; then
  case "$arg" in *'"payload":{"type":"object","properties":'*) ;; *) exit 26;; esac
 fi
 previous="$arg"
done
`
			script := "#!/bin/sh\n" + schemaCheck + addDirCheck + "cat <<'RESULT'\n" + prefix + string(result) + "\nRESULT\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			ctx := context.WithValue(context.Background(), inboxRetryReconcileKey{}, true)
			out, failure := s.executeInboxCLI(ctx, "@cuan", "agy", "Saved facts", home, "/tmp/unused-workspace", nil, attachments...)
			if scenario == "answer" || scenario == "no_attachment" || scenario == "read_attachment" || scenario == "reconcile" {
				if out == "" || failure != "" {
					t.Fatal("fresh answer rejected", scenario, failure)
				}
			} else if failure == "" || out != "" {
				t.Fatal("uncertain activity replay was accepted", scenario)
			}
		})
	}
}

func TestInboxDirectRetryDeadlineIsIndependentOfFormatRepair(t *testing.T) {
	t.Setenv("INBOX_CLI_DIRECT_RECOVERY_TIMEOUT", "")
	t.Setenv("INBOX_DIRECT_RECOVERY_TIMEOUT", "")
	parent, stop := context.WithTimeout(context.Background(), 6*time.Minute)
	defer stop()
	ctx, cancel := inboxDirectReplyContext(parent)
	defer cancel()
	deadline, _ := ctx.Deadline()
	if left := time.Until(deadline); left < 124*time.Second || left > 125*time.Second {
		t.Fatal("fresh retry inherited formatter deadline", left)
	}
	short, shortCancel := context.WithTimeout(parent, time.Second)
	defer shortCancel()
	bounded, boundedCancel := inboxDirectReplyContext(short)
	defer boundedCancel()
	a, _ := short.Deadline()
	b, _ := bounded.Deadline()
	if a != b {
		t.Fatal("retry renewed the original deadline")
	}
}

func TestInboxReconciliationContextExcludesUnrelatedProjectBacklog(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	roomID := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	room, _ := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	room.ProjectID = "bound-project"
	if err := s.projectStudioRepo.UpdateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "bound-project", Name: "Project"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: roomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "scope", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: roomID, AuthorType: "user", Content: "ORIGINAL_APPROVED_REQUEST", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "unrelated", ProjectID: "bound-project", SourceRoomID: "other-room", Title: "UNRELATED_PROJECT_TASK", Status: task.TaskStatusDoing, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	packet, err := s.inboxContext(ctx, q, true)
	if err != nil || strings.Contains(packet, "UNRELATED_PROJECT_TASK") || strings.Contains(packet, "Contact memory") || !strings.Contains(packet, "ORIGINAL_APPROVED_REQUEST") || !strings.Contains(packet, "An empty list means no task is recorded here") {
		t.Fatal("reconciliation mixed unrelated work", err, packet)
	}
}

func TestInboxReconciliationDelegatesOnceWithOriginalBinding(t *testing.T) {
	reconciliationDelegationContract(t, nil)
}

func TestInboxReconciliationDelegationPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { reconciliationDelegationContract(t, database) })
}

func reconciliationDelegationContract(t *testing.T, database *sql.DB) {
	s, token, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	if database != nil {
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Fixture','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
		s.taskSvc = task.NewService(task.NewDBStore(database), nil)
	}
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	now := time.Now().UTC()
	source := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "source", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := store.AcceptInboxRequest(ctx, source, &db.ProjectMessage{ID: source.MessageID, RoomID: room, AuthorType: "user", Content: "Perform the approved verification and finish only the remaining work.", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	source.State = "interrupted"
	if err := store.FinishInboxRequest(ctx, source, nil); err != nil {
		t.Fatal(err)
	}
	retry, _, err := s.projectStudioRepo.(db.InboxRetryRepository).RetryInboxRequest(ctx, room, source.RequestID, source.OwnerID)
	if err != nil || !db.InboxRetryNeedsReconciliation(retry) {
		t.Fatal(err)
	}
	// Keep execution queued behind unrelated work so this test checks the
	// durable handoff without starting a native task process.
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "occupied-migration", ProjectID: "default", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode, Title: "Apply database schema migration"}); err != nil {
		t.Fatal(err)
	}
	d := inboxDelegation{Reconcile: true, ExecutionCategory: task.ExecutionCategoryOps, Title: "Verify original work", Description: "Check actual current state and finish only the remaining approved work.\n- [ ] Verified evidence", ProjectID: "default", AuthorizationID: source.MessageID}
	raw, _ := json.Marshal(d)
	content := "I will check current state first.\n```cuan-delegate\n" + string(raw) + "\n```"
	out := s.applyInboxDelegation(ctx, retry, content)
	if strings.Contains(out, "could not") || !strings.Contains(out, "View work") {
		t.Fatal("reconciliation did not hand off", out)
	}
	created, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+source.MessageID)
	if err != nil || created.SourceMessageID != source.MessageID || created.SourceRoomID != room || created.ExecutionCategory != task.ExecutionCategoryOps || !strings.Contains(created.Instructions, inboxReconciliationExecutionContract) {
		t.Fatal("lost original scope or reconcile-first contract", err)
	}
	if created.LatestAttempt != nil || created.Launch == nil || created.Launch.State != "queued" {
		t.Fatalf("unexpected native execution: attempt=%v launch=%v", created.LatestAttempt != nil, created.Launch)
	}
	launch, err := s.taskSvc.LatestLaunch(ctx, created.ID)
	if err != nil || !strings.Contains(launch.Revision, inboxReconciliationExecutionContract) {
		t.Fatal("launch lost reconciliation precondition")
	}
	_ = s.applyInboxDelegation(ctx, retry, content)
	items, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room})
	if len(items) != 1 {
		t.Fatal("retry created replacement tasks", len(items))
	}
	other := source
	other.RequestID, other.MessageID, other.ReplyID = db.GenerateUUID(), db.GenerateUUID(), db.GenerateUUID()
	other.CreatedAt = now.Add(-time.Minute)
	other.State = "queued"
	if _, _, err := store.AcceptInboxRequest(ctx, other, &db.ProjectMessage{ID: other.MessageID, RoomID: room, AuthorType: "user", Content: "An unrelated approved scope.", CreatedAt: other.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	other.State = "completed"
	if err := store.FinishInboxRequest(ctx, other, nil); err != nil {
		t.Fatal(err)
	}
	otherTask := &task.Task{ID: "msg_" + other.MessageID, ProjectID: "default", SourceRoomID: room, SourceMessageID: other.MessageID, Title: "Other approved work", Status: task.TaskStatusBlocked, Instructions: "Original other scope"}
	if err := s.taskSvc.Store().CreateTask(ctx, otherTask); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"", otherTask.ID} {
		wrong := d
		wrong.TaskID = taskID
		wrong.AuthorizationID = other.MessageID
		wrong.Revision = "Continue unrelated work"
		body, _ := json.Marshal(wrong)
		out := s.applyInboxDelegation(ctx, retry, "```cuan-delegate\n"+string(body)+"\n```")
		if !strings.Contains(out, "could not start") {
			t.Fatal("retry borrowed another scope", out)
		}
		untouched, _ := s.taskSvc.GetTaskDetail(ctx, otherTask.ID)
		if untouched.LatestAttempt != nil || untouched.Status != task.TaskStatusBlocked || untouched.Instructions != "Original other scope" {
			t.Fatal("unrelated task mutated")
		}
	}
	for _, status := range []string{task.TaskStatusDone, "cancelled", task.TaskStatusDoing} {
		item, _ := s.taskSvc.Store().GetTask(ctx, created.ID)
		item.Status = status
		if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		_ = s.applyInboxDelegation(ctx, retry, content)
		after, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
		if after.Status != status || after.LatestAttempt != nil {
			t.Fatal("retry changed terminal/active work", status)
		}
	}
}

func TestInboxReconciliationRejectsTopicMutation(t *testing.T) {
	raw := `{"reply":"Saved state checked.","topic_title":"Changed topic","action":null}`
	if output, err := decodeInboxCoordinatorResponseForMode(raw, true, true); err == nil || output != "" {
		t.Fatal("retry changed the conversation topic")
	}
}

func TestInboxReplyWorkspaceIsIsolatedAndTrusted(t *testing.T) {
	home := t.TempDir()
	dir, err := prepareInboxReplyWorkspace(home)
	if err != nil || dir == home {
		t.Fatal("no isolated reply workspace", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("reply workspace contains context", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".gemini", "antigravity-cli", "settings.json"))
	if err != nil || !strings.Contains(string(raw), dir) {
		t.Fatal("headless workspace trust missing", err)
	}
	other := t.TempDir()
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareInboxReplyWorkspace(home); err == nil {
		t.Fatal("symlinked workspace accepted")
	}
}

func TestInboxReconciliationInvalidFormatDoesNotRepeatEvaluation(t *testing.T) {
	s, token, _, _, _, home, backupHome := setupFailoverFixture(t)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		s.Router().ServeHTTP(res, req)
		return res
	}
	s.cfg.InboxExecutionRole = "api"
	accepted := call("POST", "/api/v1/chat/conversations/agent_main/messages", `{"request_id":"`+db.GenerateUUID()+`","message":"Inspect the saved work; no external effects."}`)
	if accepted.Code != 202 {
		t.Fatal(accepted.Code)
	}
	var body struct{ Request db.InboxRequest }
	if err := json.Unmarshal(accepted.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	original := body.Request
	owner, err := s.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	original.OwnerID = owner.ID
	original.State = "interrupted"
	if err := store.FinishInboxRequest(context.Background(), original, nil); err != nil {
		t.Fatal(err)
	}
	retry, _, err := s.projectStudioRepo.(db.InboxRetryRepository).RetryInboxRequest(context.Background(), original.RoomID, original.RequestID, original.OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": `{"reply":"Checking saved work.","topic_title":"","action":{"kind":"delegation","payload":{"reconcile":true,"scope":"unsupported PRIVATE_DATA"}}}}`}})
	binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	script := "#!/bin/sh\nprintf x >> \"$HOME/retry-calls\"\ncat <<'RESULT'\n" + string(response) + "\nRESULT\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	s.cfg.InboxExecutionRole = "worker"
	q, claimed, err := store.ClaimInboxRequest(context.Background(), retry.RoomID, retry.RequestID)
	if err != nil || !claimed {
		t.Fatal("claim failed", err)
	}
	s.runInboxRequest(store, q)
	waitInboxState(t, s, q.RoomID, q.RequestID, "failed")
	calls, _ := os.ReadFile(filepath.Join(home, "retry-calls"))
	backupCalls, _ := os.ReadFile(filepath.Join(backupHome, "retry-calls"))
	if len(calls)+len(backupCalls) != 2 {
		requests, _ := store.ListInboxRequests(context.Background(), q.RoomID)
		t.Fatal("expected one evaluation and one format repair", len(calls), len(backupCalls), requests)
	}
	requests, _ := store.ListInboxRequests(context.Background(), q.RoomID)
	for _, request := range requests {
		for _, step := range request.Steps {
			if strings.Contains(step.Text, "PRIVATE_DATA") {
				t.Fatal("private response escaped diagnostics")
			}
		}
	}
}

func TestInboxReconciliationAllowsFollowUpQuestionInSameRoom(t *testing.T) {
	s, room, _, home := inboxFixture(t)
	store := s.projectStudioRepo.(db.InboxRequestRepository)
	now := time.Now().UTC()
	ctx := context.Background()

	// 1. Initial task approved and created
	authMsgID := db.GenerateUUID()
	originalTask := &task.Task{
		ID:                "msg_" + authMsgID,
		ProjectID:         "default",
		SourceRoomID:      room,
		SourceMessageID:   authMsgID,
		Title:             "Original approved task",
		Instructions:      "Implement the original feature",
		Status:            task.TaskStatusBlocked,
		ExecutionCategory: task.ExecutionCategoryCode,
		CreatedAt:         now.Add(-time.Hour),
		UpdatedAt:         now.Add(-time.Hour),
	}
	if err := s.taskSvc.Store().CreateTask(ctx, originalTask); err != nil {
		t.Fatal(err)
	}

	// 2. User sends follow-up chat message in the same room ("Kenapa belum selesai?")
	followUpMsgID := db.GenerateUUID()
	followUpReqID := db.GenerateUUID()
	followUpReq := db.InboxRequest{
		RoomID:      room,
		RequestID:   followUpReqID,
		MessageID:   followUpMsgID,
		ReplyID:     db.GenerateUUID(),
		OwnerID:     "owner",
		AgentID:     "agent_lifecycle",
		State:       "interrupted",
		PayloadHash: "chat-followup",
		CreatedAt:   now.Add(-5 * time.Minute),
		Deadline:    now.Add(time.Hour),
	}
	followUpMsg := &db.ProjectMessage{
		ID:         followUpMsgID,
		RoomID:     room,
		AuthorType: "user",
		Content:    "Kenapa masih belum selesai?",
		CreatedAt:  now.Add(-5 * time.Minute),
	}
	if _, _, err := store.AcceptInboxRequest(ctx, followUpReq, followUpMsg); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishInboxRequest(ctx, followUpReq, nil); err != nil {
		t.Fatal(err)
	}

	// 3. Follow-up turn times out and is retried with reconciliation
	retry, _, err := s.projectStudioRepo.(db.InboxRetryRepository).RetryInboxRequest(ctx, room, followUpReq.RequestID, "owner")
	if err != nil || !db.InboxRetryNeedsReconciliation(retry) {
		t.Fatalf("expected retry with reconciliation: err=%v, retry=%+v", err, retry)
	}

	// 4. Coordinator delegates to reconcile the original unfinished task
	d := inboxDelegation{
		Reconcile:       true,
		TaskID:          originalTask.ID,
		ProjectID:       "default",
		AuthorizationID: authMsgID,
		Revision:        "Check current state and finish missing acceptance criteria.",
	}
	status, err := s.prepareReconciliationDelegation(ctx, retry, &d)
	if err != nil {
		t.Fatalf("prepareReconciliationDelegation failed: %v", err)
	}
	if status != "" {
		t.Fatalf("unexpected status returned: %s", status)
	}
	if d.TaskID != originalTask.ID || d.ProjectID != "default" {
		t.Fatalf("unexpected delegation mutation: %+v", d)
	}
	_ = home
}
