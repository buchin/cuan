package terminal

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/buchin/cuan/internal/auth"
)

const (
	defaultPingInterval = 15 * time.Second
	writeWait           = 10 * time.Second
	pongWait            = 60 * time.Second
)

// TerminalMessage represents control payloads exchanged over the WebSocket connection.
type TerminalMessage struct {
	Type      string `json:"type"`              // "resize", "stdin", "input", "mode", "ping", "pong", "connected", "error", "status"
	Cols      uint16 `json:"cols,omitempty"`    // for "resize"
	Rows      uint16 `json:"rows,omitempty"`    // for "resize"
	Data      string `json:"data,omitempty"`    // for "stdin" / "input"
	Mode      string `json:"mode,omitempty"`    // for "mode" ("watch" | "interactive")
	Message   string `json:"message,omitempty"` // for "error" or info
	TaskID    string `json:"task_id,omitempty"`
	Session   string `json:"session_id,omitempty"`
	Sequence  uint64 `json:"sequence,omitempty"`
	RequestID string `json:"request_id,omitempty"` // private socket input acknowledgement
}

// HandlerConfig provides dependencies for the terminal WebSocket handler.
type HandlerConfig struct {
	AttachOnly   bool
	Manager      *Manager
	AuthService  *auth.Service
	TaskVerifier func(r *http.Request, taskID string) error
	TaskResolver func(ctx context.Context, rawID string) string
	CheckOrigin  func(r *http.Request) bool
}

// Handler implements the HTTP-to-WebSocket terminal gateway.
type Handler struct {
	attachOnly   bool
	mgr          *Manager
	authSvc      *auth.Service
	taskVerifier func(r *http.Request, taskID string) error
	taskResolver func(ctx context.Context, rawID string) string
	upgrader     websocket.Upgrader
}

// SetTaskResolver assigns a function that maps attempts, messages, or room IDs to a canonical task ID.
func (h *Handler) SetTaskResolver(resolver func(ctx context.Context, rawID string) string) {
	h.taskResolver = resolver
}

// NewHandler constructs a new terminal WebSocket handler.
func NewHandler(cfg HandlerConfig) *Handler {
	checkOrigin := cfg.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			u, err := url.Parse(origin)
			return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
		}
	}

	return &Handler{
		mgr:          cfg.Manager,
		attachOnly:   cfg.AttachOnly,
		authSvc:      cfg.AuthService,
		taskVerifier: cfg.TaskVerifier,
		taskResolver: cfg.TaskResolver,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 15 * time.Second,
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     checkOrigin,
			Subprotocols:    []string{"cuan-terminal"},
		},
	}
}

// ValidateRequestAuth checks if the request is authenticated when an owner is configured.
func (h *Handler) ValidateRequestAuth(r *http.Request) error {
	if h.authSvc == nil {
		return nil
	}

	// If owner is not setup yet, permit access (dev / onboarding mode)
	status, err := h.authSvc.Status(r.Context())
	if err == nil && status.SetupRequired {
		return nil
	}

	token := auth.ExtractToken(r)

	// Also check query param ?token=... (common for browser WebSocket APIs)
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}

	// Also check Sec-WebSocket-Protocol headers (e.g. token.<value>)
	if token == "" {
		protocols := websocket.Subprotocols(r)
		for _, p := range protocols {
			if strings.HasPrefix(p, "token.") {
				token = strings.TrimPrefix(p, "token.")
				break
			}
		}
	}

	if token == "" {
		return fmt.Errorf("authentication required: missing session token")
	}

	_, err = h.authSvc.ValidateToken(r.Context(), token)
	if err != nil {
		return fmt.Errorf("invalid or expired session token: %w", err)
	}

	return nil
}

// ServeHTTP handles WebSocket connection upgrades for /api/tasks/{id}/terminal.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate request
	if err := h.ValidateRequestAuth(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// 2. Extract Task ID from path or URL query
	taskID := extractTaskID(r)
	if h.taskResolver != nil && taskID != "" {
		taskID = h.taskResolver(r.Context(), taskID)
	}
	if taskID == "" {
		http.Error(w, "task id is required", http.StatusBadRequest)
		return
	}

	// 3. Optional task verifier
	if h.taskVerifier != nil {
		if err := h.taskVerifier(r, taskID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}

	// 4. Parse initial query parameters
	q := r.URL.Query()
	modeStr := strings.ToLower(strings.TrimSpace(q.Get("mode")))
	clientMode := ModeInteractive
	if modeStr == "watch" {
		clientMode = ModeWatch
	}

	var cols uint16 = 80
	if cStr := q.Get("cols"); cStr != "" {
		if c, err := strconv.ParseUint(cStr, 10, 16); err == nil && c > 0 {
			cols = uint16(c)
		}
	}

	var rows uint16 = 24
	if rStr := q.Get("rows"); rStr != "" {
		if rVal, err := strconv.ParseUint(rStr, 10, 16); err == nil && rVal > 0 {
			rows = uint16(rVal)
		}
	}

	// Negotiate subprotocol if requested
	responseHeader := make(http.Header)
	for _, sub := range websocket.Subprotocols(r) {
		if sub == "cuan-terminal" {
			responseHeader.Set("Sec-WebSocket-Protocol", "cuan-terminal")
			break
		}
	}

	// 5. Retrieve or spawn terminal session
	var session *Session
	var err error
	if h.attachOnly || q.Get("attach") == "1" {
		var found bool
		if h.mgr != nil {
			session, found = h.mgr.GetSession(taskID)
		}
		if !found {
			// Check if decoupled cuan-runner has an active PTY socket for this task
			socketID := taskID
			if expected := strings.TrimPrefix(q.Get("session_id"), "runner-sess-"); strings.HasPrefix(expected, "att_") {
				socketID = expected
			}
			if SocketExists(socketID) {
				h.ProxySocketToWebSocket(w, r, taskID, responseHeader)
				return
			}
			http.Error(w, "No live session. Start a task explicitly or open saved output.", http.StatusNotFound)
			return
		}
		if expected := q.Get("session_id"); expected != "" && expected != session.ID() {
			http.Error(w, "The selected session is no longer live.", http.StatusConflict)
			return
		}
	} else {
		if h.mgr != nil {
			session, err = h.mgr.GetOrCreateSession(r.Context(), taskID, clientMode, cols, rows)
		}
	}
	if session == nil {
		if SocketExists(taskID) {
			h.ProxySocketToWebSocket(w, r, taskID, responseHeader)
			return
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to initialize pty session: %v", err), http.StatusInternalServerError)
		} else {
			http.Error(w, "No live session. Start a task explicitly or open saved output.", http.StatusNotFound)
		}
		return
	}

	if cols > 0 && rows > 0 {
		_ = session.Resize(cols, rows)
	}

	// 6. Upgrade connection to WebSocket
	conn, err := h.upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		log.Printf("[TerminalWS] Upgrade failed for task %s: %v", taskID, err)
		return
	}
	defer conn.Close()
	var writeMu sync.Mutex
	writeMessage := func(kind int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		return conn.WriteMessage(kind, data)
	}
	writeJSON := func(value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return writeMessage(websocket.TextMessage, data)
	}

	// 7. Register subscriber
	subID := generateSubscriberID()
	sub, history, err := session.Subscribe(subID, clientMode)
	if err != nil {
		_ = writeJSON(TerminalMessage{
			Type:    "error",
			Message: "session terminated",
		})
		return
	}
	defer session.Unsubscribe(subID)

	// Send handshake connected message
	curCols, curRows := session.PTY().GetSize()
	_ = writeJSON(TerminalMessage{
		Type:    "connected",
		Session: session.ID(),
		TaskID:  taskID,
		Mode:    string(clientMode),
		Cols:    curCols,
		Rows:    curRows,
	})

	// Send initial scrollback history if available
	if len(history) > 0 {
		_ = writeMessage(websocket.BinaryMessage, history)
	}

	// Setup ping/pong heartbeat
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	done := make(chan struct{})
	stop := make(chan struct{})
	var stopOnce sync.Once
	stopPump := func() { stopOnce.Do(func() { close(stop) }) }
	defer stopPump()

	// Background pump: session output -> WebSocket client
	go func() {
		defer close(done)
		defer conn.Close()
		pingTicker := time.NewTicker(defaultPingInterval)
		defer pingTicker.Stop()

		for {
			select {
			case <-stop:
				return
			case chunk, ok := <-sub.SendChan:
				if !ok {
					// Session or subscriber closed
					_ = writeMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"))
					return
				}

				if err := writeMessage(websocket.BinaryMessage, chunk); err != nil {
					return
				}

			case <-pingTicker.C:

				if err := writeMessage(websocket.PingMessage, nil); err != nil {
					return
				}

			case <-sub.CloseChan:
				return
			}
		}
	}()

	// Foreground loop: read WebSocket messages from client -> process or write to PTY stdin
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}

		_ = conn.SetReadDeadline(time.Now().Add(pongWait))

		if messageType == websocket.BinaryMessage {
			if len(payload) >= 5 && payload[0] == 0x01 {
				cols := binary.BigEndian.Uint16(payload[1:3])
				rows := binary.BigEndian.Uint16(payload[3:5])
				if cols > 0 && rows > 0 {
					_ = session.Resize(cols, rows)
				}
				continue
			}
			data := payload
			if len(data) > 0 && data[0] == 0x00 {
				data = data[1:]
			}
			if len(data) > 0 {
				if _, err := session.WriteInput(subID, data); err != nil {
					_ = writeJSON(TerminalMessage{
						Type:    "error",
						Message: err.Error(),
					})
				}
			}
			continue
		}

		if messageType == websocket.TextMessage {
			// Check if payload is JSON control message
			if len(payload) > 0 && payload[0] == '{' {
				dec := json.NewDecoder(bytes.NewReader(payload))
				handledCount := 0
				for {
					var msg TerminalMessage
					if err := dec.Decode(&msg); err != nil {
						break
					}
					if msg.Type != "" {
						handledCount++
						switch msg.Type {
						case "resize":
							if msg.Cols > 0 && msg.Rows > 0 {
								_ = session.Resize(msg.Cols, msg.Rows)
							}
						case "stdin", "input":
							if msg.Data != "" {
								if _, err := session.WriteInput(subID, []byte(msg.Data)); err != nil {
									_ = writeJSON(TerminalMessage{
										Type:    "error",
										Message: err.Error(),
									})
								}
							}
						case "mode":
							newMode := ModeInteractive
							if strings.ToLower(msg.Mode) == "watch" {
								newMode = ModeWatch
							}
							_ = session.SetSubscriberMode(subID, newMode)
							_ = writeJSON(TerminalMessage{
								Type: "status",
								Mode: string(newMode),
							})
						case "takeover":
							_ = session.SetSubscriberMode(subID, ModeInteractive)
							_ = writeJSON(TerminalMessage{
								Type: "status",
								Mode: string(ModeInteractive),
							})
						case "ping":
							_ = writeJSON(TerminalMessage{Type: "pong"})
						}
					}
				}
				if handledCount == 0 && !bytes.HasPrefix(payload, []byte(`{"type":`)) && !bytes.Contains(payload, []byte(`"type":"stdin"`)) && !bytes.Contains(payload, []byte(`"type": "stdin"`)) {
					if _, err := session.WriteInput(subID, payload); err != nil {
						_ = writeJSON(TerminalMessage{
							Type:    "error",
							Message: err.Error(),
						})
					}
				}
			} else {
				// Not a JSON control message -> treat as direct raw text stdin input (e.g. from xterm.js)
				// Ensure no leaked JSON control frames are sent to PTY
				if !bytes.Contains(payload, []byte(`"type":"stdin"`)) && !bytes.Contains(payload, []byte(`"type": "stdin"`)) && !bytes.Contains(payload, []byte(`{"type":`)) {
					if _, err := session.WriteInput(subID, payload); err != nil {
						_ = writeJSON(TerminalMessage{
							Type:    "error",
							Message: err.Error(),
						})
					}
				}
			}
			continue
		}
	}

	stopPump()
	<-done
}

func extractTaskID(r *http.Request) string {
	// 1. Try query parameter ?task_id=... first (e.g. project terminal routing or explicit task_id)
	if qID := strings.TrimSpace(r.URL.Query().Get("task_id")); qID != "" {
		return qID
	}

	// 2. Try URL path /api/tasks/{id}/terminal or /api/v1/tasks/{id}/terminal
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if (part == "tasks") && i+1 < len(parts) {
			next := parts[i+1]
			if next != "terminal" {
				return next
			}
		}
	}

	if qID := strings.TrimSpace(r.URL.Query().Get("id")); qID != "" {
		return qID
	}

	return ""
}

func generateSubscriberID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "sub_" + hex.EncodeToString(b)
}

func (h *Handler) proxySocketToWebSocket(w http.ResponseWriter, r *http.Request, taskID string, responseHeader http.Header) {
	h.ProxySocketToWebSocket(w, r, taskID, responseHeader)
}

// ProxySocketToWebSocket bridges a WebSocket client directly to a decoupled Unix domain socket.
// This allows cuan-server to seamlessly proxy terminal connections to cuan-runner or detached project workers,
// surviving web server restarts and deployments without disrupting active task agent execution or project shells.
func (h *Handler) ProxySocketToWebSocket(w http.ResponseWriter, r *http.Request, taskID string, responseHeader http.Header) {
	socketID := taskID
	expected := strings.TrimPrefix(r.URL.Query().Get("session_id"), "runner-sess-")
	if expected != "" && !strings.HasPrefix(taskID, "project-term-") {
		if !strings.HasPrefix(expected, "att_") && !strings.HasPrefix(expected, "proj-") {
			http.Error(w, "The selected terminal session is no longer live", http.StatusConflict)
			return
		}
		if strings.HasPrefix(expected, "att_") {
			socketID = expected
		}
	}
	conn, err := h.upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		log.Printf("[TerminalWS] Upgrade failed for runner task %s: %v", taskID, err)
		return
	}
	defer conn.Close()

	sockPath := SocketPath(socketID)
	unixConn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		log.Printf("[TerminalWS] Dial runner socket failed for task %s (%s): %v", taskID, sockPath, err)
		_ = conn.WriteJSON(TerminalMessage{
			Type:    "error",
			Message: "runner terminal session is unavailable",
		})
		return
	}
	defer unixConn.Close()

	q := r.URL.Query()
	ackOutput := q.Get("output_ack") == "1"
	outputFlow := newTerminalOutputFlow()
	var cols uint16 = 80
	if cStr := q.Get("cols"); cStr != "" {
		if c, err := strconv.ParseUint(cStr, 10, 16); err == nil && c > 0 {
			cols = uint16(c)
		}
	}

	var rows uint16 = 24
	if rStr := q.Get("rows"); rStr != "" {
		if rVal, err := strconv.ParseUint(rStr, 10, 16); err == nil && rVal > 0 {
			rows = uint16(rVal)
		}
	}

	// Immediately notify runner PTY of client dimensions before streaming history
	if cols > 0 && rows > 0 {
		resizePayload, _ := json.Marshal(TerminalMessage{
			Type: "resize",
			Cols: cols,
			Rows: rows,
		})
		_, _ = unixConn.Write(append(resizePayload, '\n'))
	}

	var writeMu sync.Mutex
	writeWS := func(kind int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
		return conn.WriteMessage(kind, data)
	}

	// Send handshake connected message
	_ = writeWS(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"connected","session_id":"%s","task_id":"%s","mode":"interactive","cols":%d,"rows":%d,"output_ack":%t}`, socketID, taskID, cols, rows, ackOutput)))

	// Setup ping/pong
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	done := make(chan struct{})
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			close(done)
			_ = unixConn.Close()
			_ = conn.Close()
		})
	}

	// Goroutine 1: Runner Unix Socket -> WebSocket
	go func() {
		defer closeBoth()
		buf := make([]byte, 4096)
		for {
			n, rErr := unixConn.Read(buf)
			if n > 0 {
				send := func() error { return writeWS(websocket.BinaryMessage, buf[:n]) }
				var err error
				if ackOutput {
					err = outputFlow.write(done, pongWait, send)
					if err != nil && strings.Contains(err.Error(), "acknowledgement timed out") {
						// Browser may be backgrounded or busy rendering; do not kill the connection.
						outputFlow.unblock()
						err = nil
					}
				} else {
					err = send()
				}
				if err != nil {
					return
				}
			}
			if rErr != nil {
				return
			}
		}
	}()

	// Heartbeat pinger
	go func() {
		ticker := time.NewTicker(defaultPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeMu.Lock()
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				writeMu.Unlock()
				if err != nil {
					closeBoth()
					return
				}
			}
		}
	}()

	// Goroutine 2: WebSocket -> Runner Unix Socket
	defer closeBoth()
	for {
		msgType, data, rErr := conn.ReadMessage()
		if rErr != nil {
			break
		}
		if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
			if ackOutput && msgType == websocket.TextMessage {
				var control TerminalMessage
				if json.Unmarshal(data, &control) == nil && control.Type == "output_ack" {
					outputFlow.acknowledge(control.Sequence)
					continue // Browser paint credit is never PTY input.
				}
			}
			toWrite := data
			trimmed := bytes.TrimLeft(data, " \t\r\n")
			if len(trimmed) > 0 && trimmed[0] == '{' && !bytes.HasSuffix(data, []byte("\n")) {
				toWrite = append(data, '\n')
			}
			if _, wErr := unixConn.Write(toWrite); wErr != nil {
				break
			}
		}
	}
}
