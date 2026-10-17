package terminal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	_, errPayload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read error message: %v", err)
	}
	var errMsg TerminalMessage
	_ = json.Unmarshal(errPayload, &errMsg)
	if errMsg.Type != "error" || !strings.Contains(errMsg.Message, "watch mode") {
		t.Fatalf("expected watch mode error, got: %s (msg: %s)", errMsg.Type, errMsg.Message)
	}

	// Now send takeover command
	if err := conn.WriteJSON(TerminalMessage{Type: "takeover"}); err != nil {
		t.Fatalf("write takeover: %v", err)
	}

	// Status response confirming interactive
	_, statusPayload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	var statusMsg TerminalMessage
	_ = json.Unmarshal(statusPayload, &statusMsg)
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
	_, pongPayload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var pongMsg TerminalMessage
	_ = json.Unmarshal(pongPayload, &pongMsg)
	if pongMsg.Type != "pong" {
		t.Fatalf("expected pong, got: %s", pongMsg.Type)
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
