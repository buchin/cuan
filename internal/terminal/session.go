package terminal

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"time"
)

// Mode represents the client or session operational mode.
type Mode string

const (
	ModeWatch       Mode = "watch"       // Read-only live output stream
	ModeInteractive Mode = "interactive" // Bidirectional PTY (stdin + stdout/stderr + resize)
)

var (
	ErrReadOnlySession = errors.New("terminal is in watch mode (read-only); take over to send input")
	ErrSessionClosed   = errors.New("terminal session is closed")
)

// Subscriber represents a client receiver connected to a terminal session.
type Subscriber struct {
	ID        string
	Mode      Mode
	SendChan  chan []byte
	CloseChan chan struct{}
}

// Session manages an active PTY process, historical output buffer, and client subscribers.
type Session struct {
	id            string
	taskID        string
	pty           *PTYProcess
	defaultMode   Mode
	createdAt     time.Time
	lastActivity  time.Time
	mu            sync.RWMutex
	subscribers   map[string]*Subscriber
	historyBuffer *ringBuffer
	closed        bool
	closeChan     chan struct{}
	onEmpty       func(s *Session)
}

// NewSession creates and starts a managed terminal session for a spawned PTY process.
func NewSession(id, taskID string, proc *PTYProcess, defaultMode Mode) *Session {
	if defaultMode == "" {
		defaultMode = ModeInteractive
	}

	s := &Session{
		id:            id,
		taskID:        taskID,
		pty:           proc,
		defaultMode:   defaultMode,
		createdAt:     time.Now().UTC(),
		lastActivity:  time.Now().UTC(),
		subscribers:   make(map[string]*Subscriber),
		historyBuffer: newRingBuffer(128 * 1024), // 128 KB scrollback retention
		closeChan:     make(chan struct{}),
	}

	// Start background reader loop pumping PTY output to all subscribers
	go s.readPump()

	return s
}

// ID returns the unique session ID.
func (s *Session) ID() string {
	return s.id
}

// TaskID returns the associated task ID.
func (s *Session) TaskID() string {
	return s.taskID
}

// PTY returns the underlying PTYProcess.
func (s *Session) PTY() *PTYProcess {
	return s.pty
}

// CreatedAt returns the session spawn timestamp.
func (s *Session) CreatedAt() time.Time {
	return s.createdAt
}

// LastActivity returns the last interaction timestamp.
func (s *Session) LastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastActivity
}

// SetOnEmpty sets a callback triggered when all subscribers disconnect.
func (s *Session) SetOnEmpty(fn func(s *Session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onEmpty = fn
}

// Subscribe registers a new subscriber to receive output stream.
// It immediately pushes any buffered terminal history to the subscriber channel.
func (s *Session) Subscribe(subID string, clientMode Mode) (*Subscriber, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, nil, ErrSessionClosed
	}

	if clientMode == "" {
		clientMode = s.defaultMode
	}

	sub := &Subscriber{
		ID:        subID,
		Mode:      clientMode,
		SendChan:  make(chan []byte, 256),
		CloseChan: make(chan struct{}),
	}

	s.subscribers[subID] = sub
	s.lastActivity = time.Now().UTC()

	// Snapshot historical buffer for the new client
	history := s.historyBuffer.Bytes()

	return sub, history, nil
}

// Unsubscribe removes a client subscriber.
func (s *Session) Unsubscribe(subID string) {
	s.mu.Lock()
	sub, exists := s.subscribers[subID]
	if exists {
		delete(s.subscribers, subID)
		close(sub.CloseChan)
	}
	remaining := len(s.subscribers)
	onEmpty := s.onEmpty
	s.mu.Unlock()

	if exists && remaining == 0 && onEmpty != nil {
		onEmpty(s)
	}
}

// SubscriberCount returns active client count.
func (s *Session) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// SetSubscriberMode updates the operational mode for a specific subscriber.
func (s *Session) SetSubscriberMode(subID string, mode Mode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrSessionClosed
	}

	sub, exists := s.subscribers[subID]
	if !exists {
		return errors.New("subscriber not found")
	}

	sub.Mode = mode
	s.lastActivity = time.Now().UTC()
	return nil
}

// WriteInput writes input to the PTY from a specific subscriber.
// If the subscriber is in ModeWatch, input is rejected with ErrReadOnlySession.
func (s *Session) WriteInput(subID string, data []byte) (int, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, ErrSessionClosed
	}

	sub, exists := s.subscribers[subID]
	if exists && sub.Mode == ModeWatch {
		s.mu.Unlock()
		return 0, ErrReadOnlySession
	}

	s.lastActivity = time.Now().UTC()
	s.mu.Unlock()

	return s.pty.Write(data)
}

// Resize updates the terminal size.
func (s *Session) Resize(cols, rows uint16) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrSessionClosed
	}
	s.lastActivity = time.Now().UTC()
	s.mu.Unlock()

	return s.pty.SetSize(cols, rows)
}

// Close terminates the session and its PTY process.
func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.closeChan)

	// Close all subscriber channels
	for _, sub := range s.subscribers {
		close(sub.CloseChan)
	}
	s.subscribers = make(map[string]*Subscriber)
	s.mu.Unlock()

	return s.pty.Close()
}

// IsClosed returns true if the session has terminated.
func (s *Session) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// readPump continuously reads from the PTY master descriptor and fans out to all subscribers.
func (s *Session) readPump() {
	buf := make([]byte, 4096)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			s.mu.Lock()
			s.historyBuffer.Write(chunk)
			s.lastActivity = time.Now().UTC()

			for _, sub := range s.subscribers {
				select {
				case sub.SendChan <- chunk:
				default:
					// Dropping or non-blocking if channel is full
				}
			}
			s.mu.Unlock()
		}

		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, ErrPTYClosed) || errors.Is(err, io.ErrClosedPipe) {
				_ = s.Close()
				return
			}
			// Check if process has exited
			if s.pty.IsClosed() {
				_ = s.Close()
				return
			}
		}
	}
}

// ringBuffer implements an in-memory fixed-capacity circular byte buffer for scrollback history.
type ringBuffer struct {
	buf   []byte
	size  int
	start int
	count int
	mu    sync.Mutex
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{
		buf:  make([]byte, capacity),
		size: capacity,
	}
}

func (r *ringBuffer) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	l := len(p)
	if l == 0 {
		return 0, nil
	}

	if l >= r.size {
		// Truncate to the last r.size bytes
		p = p[l-r.size:]
		copy(r.buf, p)
		r.start = 0
		r.count = r.size
		return l, nil
	}

	for _, b := range p {
		pos := (r.start + r.count) % r.size
		r.buf[pos] = b
		if r.count < r.size {
			r.count++
		} else {
			r.start = (r.start + 1) % r.size
		}
	}
	return l, nil
}

func (r *ringBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return nil
	}

	out := make([]byte, r.count)
	if r.start+r.count <= r.size {
		copy(out, r.buf[r.start:r.start+r.count])
	} else {
		firstPart := r.size - r.start
		copy(out[:firstPart], r.buf[r.start:])
		copy(out[firstPart:], r.buf[:r.count-firstPart])
	}
	return bytes.TrimRight(out, "\x00")
}
