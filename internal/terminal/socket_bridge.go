package terminal

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

var safeSocketNameRegex = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

func socketDir() string {
	// Unix domain socket paths have strict length limits: 104 bytes on Darwin/BSD,
	// 108 bytes on Linux. On Darwin, os.TempDir() (/var/folders/...) is ~50 bytes.
	// Prefer /tmp on Unix systems when available to stay safely under 104 bytes.
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat("/tmp"); err == nil && fi.IsDir() {
			return "/tmp"
		}
	}
	return os.TempDir()
}

// SocketPath returns the standard Unix domain socket path for a task terminal.
func SocketPath(taskID string) string {
	safe := safeSocketNameRegex.ReplaceAllString(taskID, "_")
	if len(safe) > 32 {
		h := sha256.Sum256([]byte(taskID))
		safe = fmt.Sprintf("%s_%s", safe[:16], hex.EncodeToString(h[:4]))
	}
	return filepath.Join(socketDir(), fmt.Sprintf("cuan-pty-%s.sock", safe))
}

// SocketExists checks if a terminal Unix socket currently exists and is active.
func SocketExists(taskID string) bool {
	path := SocketPath(taskID)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		_ = os.Remove(path) // prune stale socket file
		return false
	}
	_ = conn.Close()
	return true
}

// SocketBridge exposes a managed terminal Session over a Unix domain socket.
// It allows decoupled daemons (such as cuan-runner) to serve live PTY streams
// to cuan-server with zero downtime during web server deployments.
type SocketBridge struct {
	session    *Session
	socketPath string
	listener   net.Listener
	closed     bool
	mu         sync.Mutex
	conns      map[net.Conn]struct{}
	closeChan  chan struct{}
}

// NewSocketBridge creates and starts listening on a Unix domain socket for a terminal session.
func NewSocketBridge(sess *Session, socketPath string) (*SocketBridge, error) {
	if sess == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}
	if socketPath == "" {
		socketPath = SocketPath(sess.TaskID())
	}

	// Clean up any lingering socket from a previous run
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix socket %s: %w", socketPath, err)
	}

	// Allow same-host processes (cuan-server and cuan-runner) to connect
	_ = os.Chmod(socketPath, 0666)

	sb := &SocketBridge{
		session:    sess,
		socketPath: socketPath,
		listener:   listener,
		conns:      make(map[net.Conn]struct{}),
		closeChan:  make(chan struct{}),
	}

	go sb.acceptLoop()

	return sb, nil
}

// SocketPath returns the filesystem path of the listening Unix socket.
func (sb *SocketBridge) SocketPath() string {
	return sb.socketPath
}

// Close closes the Unix socket listener and unlinks the socket file.
func (sb *SocketBridge) Close() error {
	sb.mu.Lock()
	if sb.closed {
		sb.mu.Unlock()
		return nil
	}
	sb.closed = true
	close(sb.closeChan)

	var err error
	if sb.listener != nil {
		err = sb.listener.Close()
	}
	for conn := range sb.conns {
		_ = conn.Close()
	}
	sb.conns = make(map[net.Conn]struct{})
	sb.mu.Unlock()

	_ = os.Remove(sb.socketPath)
	return err
}

func (sb *SocketBridge) acceptLoop() {
	for {
		conn, err := sb.listener.Accept()
		if err != nil {
			sb.mu.Lock()
			isClosed := sb.closed
			sb.mu.Unlock()
			if isClosed {
				return
			}
			continue
		}

		sb.mu.Lock()
		if sb.closed {
			sb.mu.Unlock()
			_ = conn.Close()
			return
		}
		sb.conns[conn] = struct{}{}
		sb.mu.Unlock()

		go sb.handleConnection(conn)
	}
}

func (sb *SocketBridge) handleConnection(conn net.Conn) {
	defer func() {
		sb.mu.Lock()
		delete(sb.conns, conn)
		sb.mu.Unlock()
		_ = conn.Close()
	}()

	subID := generateSessionID()
	sub, history, err := sb.session.Subscribe(subID, ModeInteractive)
	if err != nil {
		return
	}
	defer sb.session.Unsubscribe(subID)

	// Send initial scrollback history to restore client screen state
	if len(history) > 0 {
		if _, wErr := conn.Write(history); wErr != nil {
			return
		}
	}

	done := make(chan struct{})

	// Goroutine 1: PTY -> Client
	go func() {
		defer close(done)
		defer conn.Close() // Wake the input reader when a lagging subscriber is detached.
		for {
			select {
			case <-sb.closeChan:
				return
			case <-sub.CloseChan:
				return
			case chunk, ok := <-sub.SendChan:
				if !ok {
					return
				}
				if _, wErr := conn.Write(chunk); wErr != nil {
					return
				}
			}
		}
	}()

	// Goroutine 2: Client -> PTY (Handles raw keystrokes and JSON control messages)
	buf := make([]byte, 4096)
	var pending []byte
	for {
		n, rErr := conn.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)

			for len(pending) > 0 {
				// Strip leading whitespace / newlines from line delimiters
				trimmed := bytes.TrimLeft(pending, " \t\r\n")
				if consumed := len(pending) - len(trimmed); consumed > 0 {
					pending = pending[consumed:]
				}
				if len(pending) == 0 {
					break
				}

				// Check if pending starts with a JSON control frame
				if pending[0] == '{' {
					dec := json.NewDecoder(bytes.NewReader(pending))
					var msg TerminalMessage
					if err := dec.Decode(&msg); err != nil {
						// Incomplete JSON frame in stream, await more bytes from conn.Read
						if len(pending) > 65536 {
							// Safety guard: drop unparseable chunk up to next '{' or clear
							if idx := bytes.IndexByte(pending[1:], '{'); idx >= 0 {
								pending = pending[1+idx:]
							} else {
								pending = nil
							}
						}
						break
					}

					offset := int(dec.InputOffset())
					if offset <= 0 || offset > len(pending) {
						offset = len(pending)
					}
					pending = pending[offset:]

					if msg.Type != "" {
						switch msg.Type {
						case "resize":
							if msg.Cols > 0 && msg.Rows > 0 {
								_ = sb.session.Resize(msg.Cols, msg.Rows)
							}
						case "stdin", "input":
							if msg.Data != "" {
								written, writeErr := sb.session.WriteInput(subID, []byte(msg.Data))
								if msg.RequestID != "" && len(msg.RequestID) <= 128 {
									mode := "accepted"
									if writeErr != nil || written != len(msg.Data) {
										mode = "failed"
									}
									ack, _ := json.Marshal(TerminalMessage{Type: "stdin_ack", Mode: mode, RequestID: msg.RequestID})
									_, _ = conn.Write(append(ack, '\n'))
								}
							}
						case "ping":
							_, _ = conn.Write([]byte("{\"type\":\"pong\"}\n"))
						}
					}
					continue
				}

				// If pending contains a JSON frame further down, isolate leading bytes
				if idx := bytes.Index(pending, []byte(`{"type":`)); idx >= 0 {
					if idx > 0 {
						leading := pending[:idx]
						if !bytes.Contains(leading, []byte(`"data":`)) && !bytes.Contains(leading, []byte(`"stdin"`)) {
							_, _ = sb.session.WriteInput(subID, leading)
						}
						pending = pending[idx:]
					}
					continue
				}

				// Filter out leaked JSON fragments (e.g. '", "data": ' or '"data":')
				if bytes.Contains(pending, []byte(`"type":"stdin"`)) ||
					bytes.Contains(pending, []byte(`"type": "stdin"`)) ||
					bytes.Contains(pending, []byte(`"data":`)) {
					if idx := bytes.IndexByte(pending, '{'); idx >= 0 {
						pending = pending[idx:]
						continue
					}
					pending = nil
					break
				}

				// Pure raw terminal input
				_, _ = sb.session.WriteInput(subID, pending)
				pending = nil
				break
			}
		}
		if rErr != nil {
			if rErr != io.EOF {
				// connection error
			}
			break
		}
	}

	<-done
}

// SendSocketInput confirms delivery to the PTY, not merely a socket write.
// Keep reading the initial history while awaiting this request's acknowledgement.
// An observation failure is uncertain delivery; never automatically resend it.
func SendSocketInput(taskOrAttemptID, input string) error {
	sockPath := SocketPath(taskOrAttemptID)
	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("dial terminal socket %s: %w", sockPath, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(nonce[:])
	payload, err := json.Marshal(TerminalMessage{Type: "stdin", Data: input, RequestID: id})
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write to terminal socket %s: %w", sockPath, err)
	}
	accepted, _ := json.Marshal(TerminalMessage{Type: "stdin_ack", Mode: "accepted", RequestID: id})
	failed, _ := json.Marshal(TerminalMessage{Type: "stdin_ack", Mode: "failed", RequestID: id})
	var tail []byte
	buffer := make([]byte, 4096)
	for {
		count, readErr := conn.Read(buffer)
		if count > 0 {
			tail = append(tail, buffer[:count]...)
			if bytes.Contains(tail, accepted) {
				return nil
			}
			if bytes.Contains(tail, failed) {
				return fmt.Errorf("terminal input was rejected by the PTY")
			}
			if len(tail) > 8192 {
				tail = tail[len(tail)-8192:]
			}
		}
		if readErr != nil {
			return fmt.Errorf("terminal input delivery was not acknowledged; inspect the current session before retrying: %w", readErr)
		}
	}
}
