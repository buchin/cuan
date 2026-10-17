package server

import (
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

