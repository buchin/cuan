package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func inboxFixture(t *testing.T) (*Server, string, func(string, string, string) *httptest.ResponseRecorder, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := t.TempDir()
	script := "#!/bin/sh\nif ! mkdir \"$HOME/active\" 2>/dev/null; then echo concurrent >> \"$HOME/overlap\"; exit 9; fi\ntrap 'rmdir \"$HOME/active\"' EXIT\nprintf '\\n---TURN---\\n%s\\n' \"$2\" >> \"$HOME/prompts\"\nsleep 0.15\nprintf '%s' '{\"event\":\"result\",\"result\":{\"status\":\"SUCCESS\",\"response\":\"{\\\"reply\\\":\\\"FIXTURE answer\\\",\\\"topic_title\\\":\\\"\\\",\\\"action\\\":null}\"}}' \n"
	if e := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s, token := setupTestChatServer(t)
	s.cfg.Environment = "staging"
	registry := db.NewMemoryAgentRepository()
	registry.CreateAgent(context.Background(), &db.Agent{ID: "agent_lifecycle", Name: "@lifecycle", Harness: "agy", Provider: "agy", Enabled: true, Status: "active"})
	s.SetAgentRepository(registry)
	if s.terminalManager != nil {
		s.terminalManager.CloseAll()
	}
	s.SetTerminalManager(terminal.NewManager(terminal.ManagerConfig{WorkspacesDir: t.TempDir()}))
	t.Cleanup(func() {
		// Stop background fixture writers before TempDir removes their HOME.
		s.inboxStopping.Store(true)
		s.inboxCancels.Range(func(_, value any) bool {
			if cancel, ok := value.(context.CancelFunc); ok {
				cancel()
			}
			return true
		})
		s.terminalManager.CloseAll()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.DrainInboxWorker(ctx); err != nil {
			t.Errorf("fixture Inbox workers did not stop: %v", err)
		}
	})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	return s, token, call, home
}
func waitInboxState(t *testing.T, s *Server, room, id, state string) {
	t.Helper()
	end := time.Now().Add(4 * time.Second)
	for {
		all, e := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), room)
		if e != nil {
			t.Fatal(e)
		}
		for _, q := range all {
			if q.RequestID == id && q.State == state {
				return
			}
		}
		if time.Now().After(end) {
			t.Fatalf("expected %s; got %+v", state, all)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func TestInboxLifecycleContextFIFOStreamAndTask(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	path := "/api/v1/chat/conversations/agent_lifecycle/messages"
	firstID, secondID := db.GenerateUUID(), db.GenerateUUID()
	post := func(id, text string) db.InboxRequest {
		w := call("POST", path, `{"request_id":"`+id+`","message":"`+text+`"}`)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var out struct{ Request db.InboxRequest }
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out.Request
	}
	first := post(firstID, "FIRST context detail")
	second := post(secondID, "SECOND future follow-up")
	waitInboxState(t, s, first.RoomID, second.RequestID, "completed")
	prompts, _ := os.ReadFile(filepath.Join(home, "prompts"))
	turns := strings.Split(string(prompts), "---TURN---")
	if len(turns) != 3 {
		t.Fatalf("execution count %d", len(turns)-1)
	}
	if strings.Contains(turns[1], "SECOND future") || !strings.Contains(turns[2], "FIRST context") || !strings.Contains(turns[2], "FIXTURE answer") {
		t.Fatal("history boundary/context incorrect", string(prompts))
	}
	if _, e := os.Stat(filepath.Join(home, "overlap")); !os.IsNotExist(e) {
		t.Fatal("overlapping provider processes")
	}
	// EventSource-equivalent request must authenticate via header, never URL token.
	streamPath := "/api/v1/chat/conversations/agent_lifecycle/events"
	for _, suffix := range []string{"", "?token=" + token} {
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, httptest.NewRequest("GET", streamPath+suffix, nil))
		if w.Code != 401 {
			t.Fatal("unauthenticated stream", w.Code)
		}
	}
	live := httptest.NewServer(s.Router())
	defer live.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", live.URL+streamPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var data struct {
				Messages []ChatMessageDTO
				Requests []db.InboxRequest
			}
			if e := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data); e != nil {
				t.Fatal(e)
			}
			if len(data.Messages) != 4 || len(data.Requests) != 2 || len(data.Requests[0].Steps) == 0 {
				t.Fatal("incomplete recovery snapshot", line)
			}
			found = true
			break
		}
	}
	if found {
		// Unchanged room events must keep the connection alive without replaying
		// the full history or triggering another browser render.
		s.publishInbox(first.RoomID)
		heartbeat := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: snapshot") {
				t.Fatal("unchanged history was retransmitted")
			}
			if line == ": heartbeat" {
				heartbeat = true
				break
			}
		}
		if !heartbeat {
			t.Fatal("missing unchanged-history heartbeat", scanner.Err())
		}
	}
	response.Body.Close()
	cancel()
	if !found {
		t.Fatal("no stream snapshot")
	}
	// Canonical message -> task is idempotent and requires a real project.
	s.projectSvc.Store().CreateProject(context.Background(), &project.Project{ID: "fixture-project", Name: "Fixture", ProjectType: "blank", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	taskPath := "/api/v1/chat/conversations/agent_lifecycle/messages/" + first.MessageID + "/task"
	for i := 0; i < 3; i++ {
		w := call("POST", taskPath, `{"project_id":"fixture-project","title":"Implement fixture"}`)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	tasks, e := s.taskSvc.ListTasks(context.Background(), task.TaskFilter{})
	if e != nil || len(tasks) != 1 {
		t.Fatal("task duplication", len(tasks), e)
	}
	created := tasks[0]
	if created.Status != "backlog" || !strings.Contains(created.Instructions, "FIRST context detail") {
		t.Fatal("wrong task source", created)
	}
	prepareTaskReporter(t, s)
	runPath := "/api/v1/tasks/" + created.ID + "/run-agent"
	w := call("POST", runPath, `{"agent_id":"agent_lifecycle","runner_id":"local_terminal"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", runPath, `{"agent_id":"agent_lifecycle","runner_id":"local_terminal"}`)
	if w.Code != 409 {
		t.Fatal("duplicate launch accepted", w.Code, w.Body.String())
	}
	end := time.Now().Add(3 * time.Second)
	for {
		detail, e := s.taskSvc.GetTaskDetail(context.Background(), created.ID)
		if e != nil {
			t.Fatal(e)
		}
		if detail.LatestAttempt != nil && detail.LatestAttempt.Status == "succeeded" {
			if detail.Status != "review" {
				t.Fatal("successful process must await review", detail.Status)
			}
			break
		}
		if time.Now().After(end) {
			t.Fatal("attempt did not finish", detail)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func TestInboxLifecycleCancellationAndQueuedRecovery(t *testing.T) {
	s, _, call, home := inboxFixture(t)
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	// A real child and its sleep must be terminated, not merely hidden in the UI.
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\necho started >> \"$HOME/calls\"\nsleep 30\nprintf late\n"), 0700)
	path := "/api/v1/chat/conversations/agent_lifecycle/messages"
	ids := []string{db.GenerateUUID(), db.GenerateUUID()}
	var first db.InboxRequest
	for i, id := range ids {
		w := call("POST", path, `{"request_id":"`+id+`","message":"cancel fixture"}`)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var out struct{ Request db.InboxRequest }
		json.Unmarshal(w.Body.Bytes(), &out)
		if i == 0 {
			first = out.Request
		}
	}
	end := time.Now().Add(2 * time.Second)
	for {
		if raw, _ := os.ReadFile(filepath.Join(home, "calls")); len(raw) > 0 {
			break
		}
		if time.Now().After(end) {
			t.Fatal("child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, id := range []string{ids[1], ids[0]} {
		w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/requests/"+id+"/cancel", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	waitInboxState(t, s, first.RoomID, ids[0], "cancelled")
	waitInboxState(t, s, first.RoomID, ids[1], "cancelled")
	raw, _ := os.ReadFile(filepath.Join(home, "calls"))
	if strings.Count(string(raw), "started") != 1 {
		t.Fatal("cancelled queue was executed")
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), first.RoomID)
	if len(messages) != 2 {
		t.Fatal("late reply survived cancel")
	}
	// A queued message survives losing the original HTTP worker and is picked up by a new drain.
	stored, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), first.RoomID)
	next := db.InboxRequest{RoomID: first.RoomID, OwnerID: stored[0].OwnerID, AgentID: "agent_lifecycle", RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: time.Now(), Deadline: time.Now().Add(time.Hour)}
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	repo.AcceptInboxRequest(context.Background(), next, &db.ProjectMessage{ID: next.MessageID, RoomID: next.RoomID, Content: "queued before restart", AuthorType: "user", CreatedAt: next.CreatedAt})
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nprintf '%s' '{\"event\":\"result\",\"result\":{\"status\":\"SUCCESS\",\"response\":\"{\\\"reply\\\":\\\"recovered fixture\\\",\\\"topic_title\\\":\\\"\\\",\\\"action\\\":null}\"}}' \n"), 0700)
	s.drainInboxRoom(first.RoomID)
	waitInboxState(t, s, first.RoomID, next.RequestID, "completed")
}

func TestInboxLifecycleNegativeCasesAndRetainedTaskOutput(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	path := "/api/v1/chat/conversations/agent_lifecycle/messages"
	id := db.GenerateUUID()
	w := call("POST", path, `{"request_id":"`+id+`","message":"Source work"}`)
	var accepted struct{ Request db.InboxRequest }
	json.Unmarshal(w.Body.Bytes(), &accepted)
	waitInboxState(t, s, accepted.Request.RoomID, id, "completed")
	s.projectSvc.Store().CreateProject(ctx, &project.Project{ID: "fixture-project", Name: "Fixture", ProjectType: "blank", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	taskPath := "/api/v1/chat/conversations/agent_lifecycle/messages/" + accepted.Request.MessageID + "/task"
	for _, c := range []struct {
		body   string
		status int
	}{{`{"title":"No project"}`, 400}, {`{"project_id":"missing","title":"Bad project"}`, 404}, {`{"project_id":"fixture-project","title":"Bad agent","assignee":"missing"}`, 400}} {
		if w := call("POST", taskPath, c.body); w.Code != c.status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	other := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: other, ProjectID: "default", Title: "Other room", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	if w := call("POST", "/api/v1/chat/conversations/"+other+"/messages/"+accepted.Request.MessageID+"/task", `{"project_id":"fixture-project","title":"Wrong source"}`); w.Code != 404 {
		t.Fatal("source crossed rooms", w.Code)
	}
	if w := call("POST", "/api/v1/chat/conversations/"+other+"/requests/"+id+"/cancel", ""); w.Code != 404 {
		t.Fatal("cancel crossed rooms", w.Code)
	}
	w = call("POST", taskPath, `{"project_id":"fixture-project","title":"Captured task"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	created := tasks[0]
	a, e := s.taskSvc.CreateAttempt(ctx, created.ID, "fixture", task.CreateAttemptRequest{RunnerID: "local_terminal"})
	if e != nil {
		t.Fatal(e)
	}
	recorder := newTaskOutputRecorder(s.taskSvc, a.ID)
	recorder.append([]byte("token=fixture-sensitive-"))
	recorder.append([]byte("secret\nExpected output\n"))
	recorder.flush()
	logs, e := s.taskSvc.GetAttemptLogs(ctx, a.ID, 0, 1000)
	if e != nil || len(logs) != 2 {
		t.Fatal("output not retained", logs, e)
	}
	if strings.Contains(logs[0].Line, "fixture-sensitive") || !strings.Contains(logs[0].Line, "REDACTED") {
		t.Fatal("split credential retained", logs[0].Line)
	}
	wrong := call("GET", "/api/v1/tasks/not-the-source-task/attempts/"+a.ID+"/logs", "")
	if wrong.Code != 404 {
		t.Fatal("attempt logs crossed tasks", wrong.Code)
	}
	s.taskSvc.CancelTask(ctx, created.ID, "fixture")
	// A missing local CLI produces a failed attempt and blocked task, not doing.
	os.Remove(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"))
	w = call("POST", "/api/v1/tasks/"+created.ID+"/run-agent", `{"agent_id":"agent_lifecycle","binary":"agy","runner_id":"local_terminal"}`)
	if w.Code != 500 {
		t.Fatal(w.Code, w.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if detail.Status != "blocked" || detail.LatestAttempt.Status != "failed" {
		t.Fatal("failed launch left task active", detail)
	}
	if w := call("GET", "/api/v1/tasks/nonexistent-task/terminal", ""); w.Code != 404 {
		t.Fatal("unbound terminal accepted", w.Code)
	}
}

func TestInboxLifecycleToolPermissionDeniedRecoversDirectly(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	// First invocation simulates denied tool execution. Analysis recovery also simulates repeated denial.
	// When direct response recovery runs (detected via "Direct coordinator response mode" in prompt),
	// it answers directly without calling tools.
	script := `#!/bin/sh
printf x >> "$HOME/calls"
case "$2" in
*"Direct coordinator response mode"*)
	printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Direct recovered reply after permission denied.\",\"topic_title\":\"\",\"action\":null}"}}'
	;;
*)
	printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"step_type":"tool","state":"DONE","tool_name":"run_command"}}' '{"event":"result","result":{"status":"SUCCESS","response":"","denied_actions":[{"action":"run_command"}]}}'
	;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/agent_lifecycle/messages"
	id := db.GenerateUUID()
	w := call("POST", path, `{"request_id":"`+id+`","message":"Investigate permission issue"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct{ Request db.InboxRequest }
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	waitInboxState(t, s, accepted.Request.RoomID, id, "completed")
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), accepted.Request.RoomID)
	if err != nil || len(messages) < 2 {
		t.Fatalf("expected reply message, got err=%v messages=%+v", err, messages)
	}
	last := messages[len(messages)-1]
	if !strings.Contains(last.Content, "Direct recovered reply after permission denied.") {
		t.Fatalf("expected recovered reply, got %s", last.Content)
	}
	stored, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), accepted.Request.RoomID)
	if err != nil || len(stored) == 0 {
		t.Fatal("missing stored request")
	}
	foundRecovery := false
	for _, req := range stored {
		if req.RequestID == id {
			if req.SafeError != "" {
				t.Fatalf("expected empty safe error on successful recovery, got: %s", req.SafeError)
			}
			for _, step := range req.Steps {
				if step.Kind == "response_recovery" {
					foundRecovery = true
					break
				}
			}
		}
	}
	if !foundRecovery {
		t.Fatal("missing response_recovery step in request steps")
	}
}

func TestInboxActiveProjectAndTaskContext(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// 1. Create a project with environment and settings
	meta, _ := json.Marshal(map[string]any{
		"cuan_settings": project.Settings{
			Goal:         "E-commerce portal",
			Instructions: "Laravel 11 backend",
			Environment:  "DB_HOST=127.0.0.1\nSTRIPE_SECRET=sk_live_12345\nDOJO_URL=https://dojo.cc",
		},
	})
	projID := "proj_dojo_store"
	err := s.projectSvc.Store().CreateProject(ctx, &project.Project{
		ID:            projID,
		Name:          "Dojo Store",
		RepoURL:       "git@github.com:example/dojo.git",
		DefaultBranch: "main",
		Metadata:      meta,
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// 2. Create a room bound to that project
	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: projID,
		Title:     "General Chat",
	}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// 3. Create a recent task in that room with DoD
	tCreated, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID:    projID,
		SourceRoomID: room.ID,
		Title:        "Implementasi MCP Payment",
		Instructions: "Buat tool pembayaran",
		DoDChecklist: []task.DoDItem{
			{Text: "Lolos unit test", Completed: true},
			{Text: "CLI artisan fallback php artisan pay", Completed: false},
		},
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	q := db.InboxRequest{
		RoomID:    room.ID,
		MessageID: "msg_target",
	}

	out := s.inboxActiveProjectAndTaskContext(ctx, q)

	if !strings.Contains(out, "Bound Project Context (Active Room Project)") {
		t.Fatalf("expected bound project context header, got: %s", out)
	}
	if !strings.Contains(out, "Dojo Store") {
		t.Fatalf("expected project name in context, got: %s", out)
	}
	if !strings.Contains(out, "DB_HOST, STRIPE_SECRET, DOJO_URL") {
		t.Fatalf("expected extracted env keys in context, got: %s", out)
	}
	if strings.Contains(out, "sk_live_12345") {
		t.Fatalf("secret env values must NEVER be leaked in prompt context!")
	}
	if !strings.Contains(out, tCreated.ID) || !strings.Contains(out, "Implementasi MCP Payment") {
		t.Fatalf("expected recent task in context, got: %s", out)
	}
	if !strings.Contains(out, "CLI artisan fallback php artisan pay") {
		t.Fatalf("expected unfulfilled DoD in context, got: %s", out)
	}
}
