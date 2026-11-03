package terminal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHandler_WebSocketInteractiveIO(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	h := NewHandler(HandlerConfig{
		Manager: mgr,
	})

	server := httptest.NewServer(h)
	defer server.Close()

	// Convert http URL to ws URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tasks/task-ws-io/terminal"

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// 1. Read initial connected handshake message
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read connected message: %v", err)
	}

	var msg TerminalMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal connected msg: %v", err)
	}
	if msg.Type != "connected" {
		t.Fatalf("expected type connected, got %s", msg.Type)
	}
	if msg.TaskID != "task-ws-io" {
		t.Fatalf("expected taskID task-ws-io, got %s", msg.TaskID)
	}

	// 2. Send input command via JSON stdin
	cmdInput := TerminalMessage{
		Type: "input",
		Data: "echo 'WS_INTERACTIVE_OK'\n",
	}
	if err := conn.WriteJSON(cmdInput); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// 3. Read output from websocket
	found := false
	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, outPayload, rErr := conn.ReadMessage()
		if rErr != nil {
			continue
		}
		if strings.Contains(string(outPayload), "WS_INTERACTIVE_OK") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("did not observe WS_INTERACTIVE_OK in WebSocket stream")
	}
}

func TestHandler_WebSocketWatchModeAndTakeover(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	h := NewHandler(HandlerConfig{
		Manager: mgr,
	})

	server := httptest.NewServer(h)
	defer server.Close()

	// Connect with ?mode=watch
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tasks/task-watch-takeover/terminal?mode=watch"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws: %v", err)
	}
	defer conn.Close()

	// Read handshake
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read handshake: %v", err)
	}
	var msg TerminalMessage
	_ = json.Unmarshal(payload, &msg)
	if msg.Mode != "watch" {
		t.Fatalf("expected mode watch, got %s", msg.Mode)
	}

	// In watch mode: writing input should return error message
	if err := conn.WriteJSON(TerminalMessage{Type: "input", Data: "echo 'bad'\n"}); err != nil {
		t.Fatalf("write input: %v", err)
	}

	// Wait for error response
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var errMsg TerminalMessage
	for {
		_, errPayload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read error message: %v", err)
		}
		if err := json.Unmarshal(errPayload, &errMsg); err == nil && errMsg.Type != "" {
			break
		}
	}
	if errMsg.Type != "error" || !strings.Contains(errMsg.Message, "watch mode") {
		t.Fatalf("expected watch mode error, got: %s (msg: %s)", errMsg.Type, errMsg.Message)
	}

	// Now send takeover command
	if err := conn.WriteJSON(TerminalMessage{Type: "takeover"}); err != nil {
		t.Fatalf("write takeover: %v", err)
	}

	// Status response confirming interactive
	var statusMsg TerminalMessage
	for {
		_, statusPayload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read status: %v", err)
		}
		if err := json.Unmarshal(statusPayload, &statusMsg); err == nil && statusMsg.Type != "" {
			break
		}
	}
	if statusMsg.Type != "status" || statusMsg.Mode != "interactive" {
		t.Fatalf("expected status interactive, got: %v", statusMsg)
	}

	// Now input must work
	if err := conn.WriteJSON(TerminalMessage{Type: "input", Data: "echo 'TAKEOVER_SUCCESS'\n"}); err != nil {
		t.Fatalf("write after takeover: %v", err)
	}

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, chunk, rErr := conn.ReadMessage()
		if rErr != nil {
			continue
		}
		if strings.Contains(string(chunk), "TAKEOVER_SUCCESS") {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("did not observe TAKEOVER_SUCCESS in output stream")
	}
}

func TestHandler_WebSocketResizeAndPingPong(t *testing.T) {
	tempBase := t.TempDir()

	mgr := NewManager(ManagerConfig{
		WorkspacesDir: tempBase,
	})
	defer mgr.CloseAll()

	h := NewHandler(HandlerConfig{
		Manager: mgr,
	})

	server := httptest.NewServer(h)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tasks/task-resize-ping/terminal"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Drain handshake
	_, _, _ = conn.ReadMessage()

	// 1. Send Ping
	if err := conn.WriteJSON(TerminalMessage{Type: "ping"}); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	// Read Pong
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var gotPong bool
	for !gotPong {
		_, pongPayload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read pong: %v", err)
		}
		var pongMsg TerminalMessage
		if err := json.Unmarshal(pongPayload, &pongMsg); err == nil && pongMsg.Type == "pong" {
			gotPong = true
			break
		}
	}

	// 2. Send Resize
	if err := conn.WriteJSON(TerminalMessage{Type: "resize", Cols: 110, Rows: 35}); err != nil {
		t.Fatalf("write resize: %v", err)
	}

	// Verify manager has updated PTY dimensions
	time.Sleep(50 * time.Millisecond)
	sess, exists := mgr.GetSession("task-resize-ping")
	if !exists {
		t.Fatal("session not found")
	}
	cols, rows := sess.PTY().GetSize()
	if cols != 110 || rows != 35 {
		t.Fatalf("expected 110x35, got %dx%d", cols, rows)
	}
}

func TestHandler_WebSocketRunnerSocketProxy(t *testing.T) {
	taskID := "task-runner-proxy-test"
	tmpDir, err := os.MkdirTemp("", "cuan-proxy-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Spawn a decoupled runner PTY session
	proc, err := SpawnPTY(ctx, PTYConfig{
		TaskID:      taskID,
		WorkDir:     tmpDir,
		Shell:       "/bin/sh",
		InitialCols: 80,
		InitialRows: 24,
	})
	if err != nil {
		t.Fatalf("spawn pty: %v", err)
	}
	defer proc.Close()

	runnerSess := NewSession("runner-sess-1", taskID, proc, ModeInteractive)
	defer runnerSess.Close()

	// Expose via standard Unix domain socket path
	sockPath := SocketPath("att_proxy_binding_test")
	bridge, err := NewSocketBridge(runnerSess, sockPath)
	if err != nil {
		t.Fatalf("new socket bridge: %v", err)
	}
	defer bridge.Close()

	// 2. Setup Handler with attachOnly=true and NO local sessions in manager
	mgr := NewManager(ManagerConfig{WorkspacesDir: tmpDir})
	defer mgr.CloseAll()

	handler := NewHandler(HandlerConfig{
		AttachOnly: true,
		Manager:    mgr,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/tasks/" + taskID + "/terminal?mode=interactive&attach=1&session_id=runner-sess-att_proxy_binding_test"

	// 3. Connect browser WebSocket client
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws to handler: %v", err)
	}
	defer wsConn.Close()

	// 4. Send shell command through WebSocket
	echoCmd := "echo RUNNER_WS_PROXY_VERIFIED\n"
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte(echoCmd)); err != nil {
		t.Fatalf("write echo cmd over ws: %v", err)
	}

	// 5. Read response back from WebSocket
	found := false
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		_ = wsConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msg, rErr := wsConn.ReadMessage()
		if len(msg) > 0 {
			if strings.Contains(string(msg), "RUNNER_WS_PROXY_VERIFIED") {
				found = true
				break
			}
		}
		if rErr != nil && !strings.Contains(rErr.Error(), "timeout") {
			break
		}
	}

	if !found {
		t.Errorf("expected to read RUNNER_WS_PROXY_VERIFIED through WebSocket proxy")
	}

	// 6. Test deploy resilience: disconnect client, reconnect new client while runner keeps running
	_ = wsConn.Close()

	wsConn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("reconnect ws after deploy simulation: %v", err)
	}
	defer wsConn2.Close()

	// Second client should receive scrollback with the verified output
	receivedScrollback := false
	deadline2 := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline2) {
		_ = wsConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msg, rErr := wsConn2.ReadMessage()
		if len(msg) > 0 {
			if strings.Contains(string(msg), "RUNNER_WS_PROXY_VERIFIED") {
				receivedScrollback = true
				break
			}
		}
		if rErr != nil && !strings.Contains(rErr.Error(), "timeout") {
			break
		}
	}

	if !receivedScrollback {
		t.Errorf("second client expected to receive scrollback history after reconnect")
	}
}

func TestHandler_ReconnectCannotSelectReplacementSession(t *testing.T) {
	mgr := NewManager(ManagerConfig{WorkspacesDir: t.TempDir()})
	defer mgr.CloseAll()
	server := httptest.NewServer(NewHandler(HandlerConfig{Manager: mgr}))
	defer server.Close()
	base := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tasks/task-reconnect-binding/terminal"
	conn, _, err := websocket.DefaultDialer.Dial(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	replacement, response, err := websocket.DefaultDialer.Dial(base+"?attach=1&reconnect=1&session_id=expired-session", nil)
	if replacement != nil {
		replacement.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("stale reconnect must fail closed, got response=%v err=%v", response, err)
	}
}
