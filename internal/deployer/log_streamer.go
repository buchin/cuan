package deployer

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"
)

// LogStreamer captures stdout and stderr streams in real time, maintains an
// in-memory circular ring buffer for instant streaming, and persists log entries to Store.
type LogStreamer struct {
	mu           sync.RWMutex
	deploymentID string
	store        Store
	buffer       []LogEntry
	maxBuffer    int
	subscribers  map[chan LogEntry]struct{}
	closed       bool
}

// NewLogStreamer creates a new streamer instance for a deployment.
func NewLogStreamer(deploymentID string, store Store, maxBuffer int) *LogStreamer {
	if maxBuffer <= 0 {
		maxBuffer = 1000
	}
	return &LogStreamer{
		deploymentID: deploymentID,
		store:        store,
		buffer:       make([]LogEntry, 0, maxBuffer),
		maxBuffer:    maxBuffer,
		subscribers:  make(map[chan LogEntry]struct{}),
	}
}

// Stream reads from an io.Reader (such as process stdout or stderr pipe) line-by-line
// until EOF and records each line into the in-memory buffer and persistent store.
func (s *LogStreamer) Stream(ctx context.Context, streamType string, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	// Allow lines up to 64KB
	scanner.Buffer(make([]byte, 1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		entry := LogEntry{
			DeploymentID: s.deploymentID,
			Stream:       streamType,
			Message:      line,
			Timestamp:    time.Now().UTC(),
		}
		s.Append(ctx, entry)
	}

	return scanner.Err()
}

// Append adds a single log entry to the in-memory buffer, broadcasts to active subscribers,
// and persists it to the store.
func (s *LogStreamer) Append(ctx context.Context, entry LogEntry) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}

	if len(s.buffer) >= s.maxBuffer {
		s.buffer = s.buffer[1:]
	}
	s.buffer = append(s.buffer, entry)

	// Broadcast to active subscribers without blocking
	for ch := range s.subscribers {
		select {
		case ch <- entry:
		default:
			// Non-blocking drop if consumer is slow
		}
	}
	s.mu.Unlock()

	// Persist to store if provided
	if s.store != nil {
		if ctx == nil {
			ctx = context.Background()
		}
		_ = s.store.AppendLog(ctx, &entry)
	}
}

// GetRecent returns up to 'limit' most recent log entries from the in-memory buffer.
func (s *LogStreamer) GetRecent(limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.buffer) {
		limit = len(s.buffer)
	}
	res := make([]LogEntry, limit)
	copy(res, s.buffer[len(s.buffer)-limit:])
	return res
}

// Subscribe registers a new subscriber channel that receives log entries as they arrive.
// Returns the channel and an unsubscribe cancellation function.
func (s *LogStreamer) Subscribe() (<-chan LogEntry, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan LogEntry, 100)
	if s.closed {
		close(ch)
		return ch, func() {}
	}

	s.subscribers[ch] = struct{}{}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if _, exists := s.subscribers[ch]; exists {
				delete(s.subscribers, ch)
			}
			if !s.closed {
				close(ch)
			}
		})
	}

	return ch, unsubscribe
}

// Close closes the streamer, shuts down all subscriber channels, and stops accepting new logs.
func (s *LogStreamer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	for ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = nil
}
