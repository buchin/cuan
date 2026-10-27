package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func TestServer_TaskTerminalWebSocket_RootAndV1Endpoints(t *testing.T) {
	tempBase := t.TempDir()

	cfg := &config.Config{
		WorkspacesDir: tempBase,
		AppMode:       "studio",
	}

	srv := NewServer(cfg, nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	testPaths := []string{
		"/api/tasks/task-root-1/terminal",
		"/api/v1/tasks/task-v1-1/terminal",
		"/api/projects/tasks/task-alias-1/terminal",
	}

	for _, taskID := range []string{"task-root-1", "task-v1-1", "task-alias-1"} {
		_ = srv.taskSvc.Store().CreateTask(context.Background(), &task.Task{ID: taskID, Title: "Test Task"})
		_, _ = srv.terminalManager.GetOrCreateSession(context.Background(), taskID, "interactive", 80, 24)
	}

	for _, path := range testPaths {
		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + path
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Dial %s failed: %v", path, err)
		}

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("expected 101 Switching Protocols for %s, got %d", path, resp.StatusCode)
		}

		// Read handshake message
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage from %s failed: %v", path, err)
		}

		var msg terminal.TerminalMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal handshake from %s: %v", path, err)
		}
		if msg.Type != "connected" {
			t.Errorf("expected connected msg from %s, got %s", path, msg.Type)
		}

		conn.Close()
	}
}

func TestServer_TaskTerminalWebSocket_InteractiveTakeOverAndResize(t *testing.T) {
	tempBase := t.TempDir()

	cfg := &config.Config{
		WorkspacesDir: tempBase,
		AppMode:       "studio",
	}

	srv := NewServer(cfg, nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	taskID := "task-interactive-drill"
	_ = srv.taskSvc.Store().CreateTask(context.Background(), &task.Task{ID: taskID, Title: "Test Task"})
	_, _ = srv.terminalManager.GetOrCreateSession(context.Background(), taskID, "interactive", 80, 24)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/tasks/" + taskID + "/terminal"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	// 1. Drain handshake
	_, _, _ = conn.ReadMessage()

	// 2. Resize
	if err := conn.WriteJSON(terminal.TerminalMessage{Type: "resize", Cols: 100, Rows: 30}); err != nil {
		t.Fatalf("write resize: %v", err)
	}

	// 3. Send echo
	marker := "CUAN_STUDIO_TERMINAL_VERIFIED"
	if err := conn.WriteJSON(terminal.TerminalMessage{Type: "input", Data: "echo '" + marker + "'\n"}); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// 4. Verify output received
	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, chunk, rErr := conn.ReadMessage()
		if rErr != nil {
			continue
		}
		if strings.Contains(string(chunk), marker) {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("did not observe %s in terminal WebSocket stream", marker)
	}

	// 5. Query active sessions status endpoint
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/terminal/sessions", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/terminal/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from sessions status, got %d", resp.StatusCode)
	}

	var statusResp struct {
		Count    int `json:"count"`
		Sessions []struct {
			TaskID  string `json:"task_id"`
			Cols    uint16 `json:"cols"`
			Rows    uint16 `json:"rows"`
			WorkDir string `json:"work_dir"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode sessions status: %v", err)
	}

	if statusResp.Count < 1 {
		t.Fatalf("expected at least 1 active session, got %d", statusResp.Count)
	}

	foundTask := false
	for _, s := range statusResp.Sessions {
		if s.TaskID == taskID {
			foundTask = true
			if s.Cols != 100 || s.Rows != 30 {
				t.Errorf("expected 100x30, got %dx%d", s.Cols, s.Rows)
			}
			expectedWorkDir := filepath.Join(tempBase, taskID)
			if s.WorkDir != expectedWorkDir {
				t.Errorf("expected workDir %s, got %s", expectedWorkDir, s.WorkDir)
			}
		}
	}
	if !foundTask {
		t.Errorf("task %s not found in sessions status", taskID)
	}

	// 6. Close session via DELETE endpoint
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/"+taskID+"/terminal", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /api/tasks/{id}/terminal: %v", err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete session, got %d", delResp.StatusCode)
	}
}

func TestServer_TaskTerminalWebSocket_WatchMode(t *testing.T) {
	tempBase := t.TempDir()

	cfg := &config.Config{
		WorkspacesDir: tempBase,
	}

	srv := NewServer(cfg, nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	taskID := "task-watch-drill"
	_ = srv.taskSvc.Store().CreateTask(context.Background(), &task.Task{ID: taskID, Title: "Test Task"})
	_, _ = srv.terminalManager.GetOrCreateSession(context.Background(), taskID, "watch", 80, 24)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/tasks/" + taskID + "/terminal?mode=watch"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	// Drain handshake
	_, payload, _ := conn.ReadMessage()
	var msg terminal.TerminalMessage
	_ = json.Unmarshal(payload, &msg)
	if msg.Mode != "watch" {
		t.Fatalf("expected watch mode, got %s", msg.Mode)
	}

	// Writing input in watch mode must return error
	_ = conn.WriteJSON(terminal.TerminalMessage{Type: "input", Data: "echo 'blocked'\n"})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, errChunk, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	var errMsg terminal.TerminalMessage
	_ = json.Unmarshal(errChunk, &errMsg)
	if errMsg.Type != "error" {
		t.Fatalf("expected error message in watch mode, got: %s", errMsg.Type)
	}
}

func TestServer_TaskTerminalWebSocket_Authentication(t *testing.T) {
	tempBase := t.TempDir()

	memStore := auth.NewMemoryStore()
	authSvc := auth.NewService(memStore)
	authResp, err := authSvc.Setup(context.Background(), "buchin@dropsugar.com", "supersecretpassword123")
	if err != nil {
		t.Fatalf("auth setup failed: %v", err)
	}
	validToken := authResp.Token

	cfg := &config.Config{
		WorkspacesDir: tempBase,
	}

	srv := NewServer(cfg, nil, authSvc)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	taskID := "task-auth-drill"
	_ = srv.taskSvc.Store().CreateTask(context.Background(), &task.Task{ID: taskID, Title: "Test Task"})
	_, _ = srv.terminalManager.GetOrCreateSession(context.Background(), taskID, "interactive", 80, 24)
	baseURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/tasks/task-auth-drill/terminal"

	// 1. Unauthenticated request without token must fail (401 Unauthorized)
	_, resp, err := websocket.DefaultDialer.Dial(baseURL, nil)
	if err == nil {
		t.Fatal("expected dial error without token, got nil")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// 2. Authenticated request with query parameter ?token=...
	wsURLWithToken := baseURL + "?token=" + validToken
	conn1, resp1, err := websocket.DefaultDialer.Dial(wsURLWithToken, nil)
	if err != nil {
		t.Fatalf("dial with query token failed: %v", err)
	}
	if resp1.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101 Switching Protocols, got %d", resp1.StatusCode)
	}
	conn1.Close()

	// 3. Authenticated request with Authorization header
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+validToken)
	conn2, resp2, err := websocket.DefaultDialer.Dial(baseURL, header)
	if err != nil {
		t.Fatalf("dial with auth header failed: %v", err)
	}
	if resp2.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101 Switching Protocols, got %d", resp2.StatusCode)
	}
	conn2.Close()

	// 4. Authenticated request with WebSocket subprotocol (token.<token>)
	dialer := &websocket.Dialer{
		Subprotocols: []string{"cuan-terminal", "token." + validToken},
	}
	conn3, resp3, err := dialer.Dial(baseURL, nil)
	if err != nil {
		t.Fatalf("dial with subprotocol failed: %v", err)
	}
	if resp3.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101 Switching Protocols, got %d", resp3.StatusCode)
	}
	conn3.Close()
}

func TestServer_RunTaskAgentEndpoint(t *testing.T) {
	srv, token := setupTestChatServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	taskID := "task-sprint22-runner"
	_ = srv.taskSvc.Store().CreateTask(context.Background(), &task.Task{ID: taskID, Title: "Test Task"})
	_ = srv.agentRepo.CreateAgent(context.Background(), &cuanDB.Agent{
		ID: "worker-flash", Name: "Worker Flash", Harness: "agy", Enabled: true, Status: cuanDB.AgentStatusActive,
	})

	// 1. Trigger POST /api/v1/tasks/{id}/run-agent
	runURL := ts.URL + "/api/v1/tasks/" + taskID + "/run-agent"
	reqPayload := `{"agent_id":"worker-flash","binary":"agy","prompt":"Write unit test for S22"}`
	req, err := http.NewRequest("POST", runURL, strings.NewReader(reqPayload))
	if err != nil {
		t.Fatalf("create POST /run-agent request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /run-agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /run-agent, got %d", resp.StatusCode)
	}

	var runResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&runResp); err != nil {
		t.Fatalf("unmarshal run-agent response: %v", err)
	}

	if runResp["status"] != "running" {
		t.Errorf("expected status 'running', got %v", runResp["status"])
	}
	if runResp["session_id"] == nil || runResp["session_id"] == "" {
		t.Errorf("expected non-empty session_id")
	}
	if runResp["pid"] == nil || runResp["pid"].(float64) <= 0 {
		t.Errorf("expected valid pid > 0, got %v", runResp["pid"])
	}

	// 2. Connect to the WebSocket terminal for this task
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/tasks/" + taskID + "/terminal?token=" + token
	conn, wsResp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial /terminal WebSocket failed: %v", err)
	}
	defer conn.Close()

	if wsResp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 Switching Protocols, got %d", wsResp.StatusCode)
	}

	// Read handshake connected message
	_, hsPayload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket handshake failed: %v", err)
	}
	var hsMsg terminal.TerminalMessage
	if err := json.Unmarshal(hsPayload, &hsMsg); err != nil || hsMsg.Type != "connected" {
		t.Errorf("expected connected handshake, got %s", hsMsg.Type)
	}

	// 3. Check history endpoint
	histURL := ts.URL + "/api/v1/tasks/" + taskID + "/run-agent/history"
	histReq, err := http.NewRequest("GET", histURL, nil)
	if err != nil {
		t.Fatalf("create GET /run-agent/history request failed: %v", err)
	}
	histReq.Header.Set("Authorization", "Bearer "+token)
	histResp, err := http.DefaultClient.Do(histReq)
	if err != nil {
		t.Fatalf("GET /run-agent/history failed: %v", err)
	}
	defer histResp.Body.Close()

	if histResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /run-agent/history, got %d", histResp.StatusCode)
	}

	var histData map[string]interface{}
	if err := json.NewDecoder(histResp.Body).Decode(&histData); err != nil {
		t.Fatalf("unmarshal history data: %v", err)
	}
	if histData["count"].(float64) < 1 {
		t.Errorf("expected at least 1 history record, got %v", histData["count"])
	}

	// Clean up session
	_ = srv.TerminalManager().CloseSession(taskID)
}

func TestServer_AgentCLIOnboardingAndBinariesEndpoints(t *testing.T) {
	tempBase := t.TempDir()

	cfg := &config.Config{
		WorkspacesDir: tempBase,
		AppMode:       "studio",
	}

	srv := NewServer(cfg, nil)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// 1. GET /api/v1/agents/binaries
	respBins, err := http.Get(ts.URL + "/api/v1/agents/binaries")
	if err != nil {
		t.Fatalf("GET /agents/binaries failed: %v", err)
	}
	defer respBins.Body.Close()

	if respBins.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /agents/binaries, got %d", respBins.StatusCode)
	}

	var binsData map[string]interface{}
	if err := json.NewDecoder(respBins.Body).Decode(&binsData); err != nil {
		t.Fatalf("unmarshal binaries: %v", err)
	}
	if binsData["count"].(float64) < 1 {
		t.Errorf("expected detected host binaries, got %v", binsData["count"])
	}

	// 2. POST /api/v1/agents/pair
	pairPayload := `{"device_code":"CUAN-TEST-99","member_id":"buchin","agent_id":"worker-flash","provider":"antigravity"}`
	respPair, err := http.Post(ts.URL+"/api/v1/agents/pair", "application/json", strings.NewReader(pairPayload))
	if err != nil {
		t.Fatalf("POST /agents/pair failed: %v", err)
	}
	defer respPair.Body.Close()

	if respPair.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /agents/pair, got %d", respPair.StatusCode)
	}

	var pairData map[string]interface{}
	if err := json.NewDecoder(respPair.Body).Decode(&pairData); err != nil {
		t.Fatalf("unmarshal pair data: %v", err)
	}
	if pairData["status"] != "paired" {
		t.Errorf("expected status 'paired', got %v", pairData["status"])
	}
	sessObj := pairData["session"].(map[string]interface{})
	sessID := sessObj["id"].(string)

	// 3. GET /api/v1/agents/sessions
	respList, err := http.Get(ts.URL + "/api/v1/agents/sessions?member_id=buchin")
	if err != nil {
		t.Fatalf("GET /agents/sessions failed: %v", err)
	}
	defer respList.Body.Close()

	if respList.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /agents/sessions, got %d", respList.StatusCode)
	}

	// 4. DELETE /api/v1/agents/sessions/{id}
	reqDel, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/agents/sessions/"+sessID, nil)
	respDel, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatalf("DELETE /agents/sessions failed: %v", err)
	}
	defer respDel.Body.Close()

	if respDel.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete session, got %d", respDel.StatusCode)
	}
}

func TestServer_TaskTerminal_RunTaskAgent_BYOAgentHarness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping BYO agent CLI harness test in short mode")
	}
	repo := cuanDB.NewMemoryAgentRepository()
	studioRepo := cuanDB.NewMemoryProjectStudioRepository()

	byoAgent := &cuanDB.Agent{
		ID:           cuanDB.GenerateUUID(),
		Name:         "@dev",
		Harness:      "agy",
		ConnectionID: "conn_byo_runner_test",
		Enabled:      true,
		Provider:     "agy",
		Status:       cuanDB.AgentStatusActive,
	}
	if err := repo.CreateAgent(context.Background(), byoAgent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	taskID := cuanDB.GenerateUUID()
	projectID := cuanDB.GenerateUUID()
	err := studioRepo.CreateTask(context.Background(), &cuanDB.ProjectTask{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Sprint 23 BYO Runner Task",
		Description: "Implement automated test runner",
		Status:      "todo",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	srv, token := setupTestChatServer(t)
	srv.SetAgentRepository(repo)
	srv.SetProjectStudioRepository(studioRepo)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Trigger run-agent with registered BYO Agent
	runPayload := map[string]interface{}{
		"agent_id": byoAgent.ID,
		"prompt":   "Execute S23.4 BYO Agent Runner",
	}
	bodyBytes, _ := json.Marshal(runPayload)

	req, err := http.NewRequest("POST", ts.URL+"/api/v1/tasks/"+taskID+"/run-agent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("create POST /run-agent request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /run-agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /run-agent, got %d", resp.StatusCode)
	}

	var runResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&runResp); err != nil {
		t.Fatalf("failed to decode run-agent response: %v", err)
	}

	if runResp["status"] != "running" {
		t.Errorf("expected status 'running', got %v", runResp["status"])
	}
	if runResp["agent_name"] != "@dev" {
		t.Errorf("expected agent_name '@dev', got %v", runResp["agent_name"])
	}
	if runResp["agent_id"] != byoAgent.ID {
		t.Errorf("expected agent_id %s, got %v", byoAgent.ID, runResp["agent_id"])
	}

	// Verify task in studioRepo was updated to running with assigned agent
	updatedTask, err := studioRepo.GetTaskByID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if updatedTask.AssigneeID == nil || *updatedTask.AssigneeID != byoAgent.ID {
		t.Errorf("expected assignee_id %s, got %v", byoAgent.ID, updatedTask.AssigneeID)
	}
	if updatedTask.ExecutionStatus != "running" && updatedTask.ExecutionStatus != "failed" && updatedTask.ExecutionStatus != "completed" {
		t.Errorf("expected valid execution_status, got %s", updatedTask.ExecutionStatus)
	}
}

func TestServer_TaskTerminal_RunTaskAgent_CommandCodeHarness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	repo := cuanDB.NewMemoryAgentRepository()
	studioRepo := cuanDB.NewMemoryProjectStudioRepository()

	ccAgent := &cuanDB.Agent{
		ID:           cuanDB.GenerateUUID(),
		Name:         "@dev2",
		Harness:      "commandcode",
		ConnectionID: "conn_cc_runner_test",
		Enabled:      true,
		Provider:     "commandcode",
		Status:       cuanDB.AgentStatusActive,
	}
	if err := repo.CreateAgent(context.Background(), ccAgent); err != nil {
		t.Fatalf("failed to seed agent: %v", err)
	}

	taskID := cuanDB.GenerateUUID()
	projectID := cuanDB.GenerateUUID()
	if err := studioRepo.CreateTask(context.Background(), &cuanDB.ProjectTask{
		ID:              taskID,
		ProjectID:       projectID,
		Title:           "CommandCode Terminal Task",
		Status:          "todo",
		ExecutionStatus: "idle",
	}); err != nil {
		t.Fatalf("failed to seed task: %v", err)
	}

	srv, token := setupTestChatServer(t)
	srv.SetAgentRepository(repo)
	srv.SetProjectStudioRepository(studioRepo)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"agent_id": ccAgent.ID,
		"prompt":   "echo commandcode ready",
	})
	req, err := http.NewRequest("POST", ts.URL+"/api/v1/tasks/"+taskID+"/run-agent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("create POST /run-agent request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /run-agent failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for CommandCode agent /run-agent, got %d", resp.StatusCode)
	}

	var runResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&runResp); err != nil {
		t.Fatalf("failed to decode run-agent response: %v", err)
	}

	if runResp["status"] != "running" {
		t.Errorf("expected status 'running', got %v", runResp["status"])
	}
	if runResp["agent_name"] != "@dev2" {
		t.Errorf("expected agent_name '@dev2', got %v", runResp["agent_name"])
	}
}

func TestS25NoImplicitTaskWorker(t *testing.T) {
	srv := NewServer(&config.Config{AppMode: "studio"}, nil)
	for _, payload := range []string{`{}`, `{"agent_id":"worker-flash"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/test/run-agent", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatal("unregistered worker was launched")
		}
	}
}

func TestServer_GetTaskRunHistory_LiveSessionAttemptResolution(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Seed task
	detail, err := srv.taskSvc.CreateTask(context.Background(), "owner", task.CreateTaskRequest{
		Title: "History Test Task",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	attempt, err := srv.taskSvc.CreateAttempt(context.Background(), detail.ID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "agent_dev",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// Create a dummy session directly in terminalManager
	sess, err := srv.terminalManager.GetOrCreateSession(context.Background(), detail.ID, terminal.ModeInteractive, 80, 24)
	if err != nil {
		t.Fatalf("failed to create terminal session: %v", err)
	}
	defer sess.Close()

	// Query GET /api/v1/tasks/{id}/run-agent/history
	req := httptest.NewRequest("GET", "/api/v1/tasks/"+detail.ID+"/run-agent/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res struct {
		TaskID      string                 `json:"task_id"`
		LiveSession map[string]interface{} `json:"live_session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if res.LiveSession == nil {
		t.Fatalf("expected live_session to be populated, got nil")
	}
	if res.LiveSession["session_id"] != sess.ID() {
		t.Errorf("expected session_id %s, got %v", sess.ID(), res.LiveSession["session_id"])
	}
	if res.LiveSession["attempt_id"] != attempt.ID {
		t.Errorf("expected attempt_id %s, got %v", attempt.ID, res.LiveSession["attempt_id"])
	}
}

