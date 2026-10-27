package server

import (
	"context"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

var taskOutputANSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// Retain bounded, redacted complete lines across browser/server restarts.
// Oversized lines are omitted so split credentials are never retained partially.
type taskOutputRecorder struct {
	mu             sync.Mutex
	service        *task.Service
	attemptID      string
	pending        []byte
	skipping       bool
	secrets        []string
	failed         bool
	quotaTail      string
	quotaDetected  bool
	onQuota        func()
	regionDetected bool
	onRegion       func()
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
	if !r.quotaDetected && !r.regionDetected {
		r.quotaTail += string(chunk)
		if len(r.quotaTail) > 8192 {
			r.quotaTail = r.quotaTail[len(r.quotaTail)-8192:]
		}
		plain := strings.Join(strings.Fields(taskOutputANSI.ReplaceAllString(r.quotaTail, "")), " ")
		if strings.Contains(plain, "⚠ Individual quota reached.") && strings.Contains(plain, "Please upgrade your subscription") {
			r.quotaDetected = true
			if r.onQuota != nil {
				go r.onQuota()
			}
		}
		if strings.Contains(plain, "⚠ FAILED_PRECONDITION (code 400): User location is not supported for the API use.") {
			r.regionDetected = true
			if r.onRegion != nil {
				go r.onRegion()
			}
		}
	}
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
	if r.regionDetected {
		return "The AI provider reported a location restriction. The run was stopped; inspect saved progress before continuing approved work with an allowed model route."
	}
	if r.quotaDetected {
		return "The AI provider reported a usage limit. The run was stopped; inspect saved progress and uncertain effects before continuing unfinished approved work."
	}
	if r.failed {
		return "Some output could not be saved. Review the work before accepting this task."
	}
	return ""
}
