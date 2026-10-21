package server

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

// Retain bounded, redacted complete lines across browser/server restarts.
// Oversized lines are omitted so split credentials are never retained partially.
type taskOutputRecorder struct {
	mu        sync.Mutex
	service   *task.Service
	attemptID string
	pending   []byte
	skipping  bool
	secrets   []string
	failed    bool
}

func newTaskOutputRecorder(s *task.Service, id string) *taskOutputRecorder {
	r := &taskOutputRecorder{service: s, attemptID: id}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok && terminal.IsSensitiveKey(key) && len(value) >= 4 {
			r.secrets = append(r.secrets, value)
		}
	}
	return r
}
func (r *taskOutputRecorder) append(chunk []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := []string{}
	for _, c := range chunk {
		if c == '\n' {
			if r.skipping {
				lines = append(lines, "[Output line too long; not saved]")
			} else {
				lines = append(lines, string(r.pending))
			}
			r.pending = r.pending[:0]
			r.skipping = false
			continue
		}
		if r.skipping {
			continue
		}
		if len(r.pending) >= 8192 {
			r.pending = r.pending[:0]
			r.skipping = true
			continue
		}
		r.pending = append(r.pending, c)
	}
	r.save(lines)
}
func (r *taskOutputRecorder) flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) > 0 && !r.skipping {
		r.save([]string{string(r.pending)})
	}
	r.pending = nil
}
func (r *taskOutputRecorder) save(lines []string) {
	logs := []task.LogRecord{}
	for _, line := range lines {
		line = agent.MaskSecrets(line, r.secrets)
		logs = append(logs, task.LogRecord{AttemptID: r.attemptID, Stream: "stdout", Line: line, LoggedAt: time.Now().UTC()})
	}
	if len(logs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.service.AppendAttemptLogs(ctx, r.attemptID, logs, 1000); err != nil {
		r.failed = true
	}
}
func (r *taskOutputRecorder) failureNote() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failed {
		return "Some output could not be saved. Review the work before accepting this task."
	}
	return ""
}
