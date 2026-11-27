package runner

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// adaptiveDeadlineContext wraps a parent context with a deadline that can be
// extended on observed activity up to maxCeiling, while still canceling
// immediately when idle or when the safety ceiling is reached.
type adaptiveDeadlineContext struct {
	parent     context.Context
	mu         sync.Mutex
	deadline   time.Time
	maxCeiling time.Time
	timer      *time.Timer
	done       chan struct{}
	err        error
}

func newAdaptiveDeadlineContext(parent context.Context, initialDeadline, maxCeiling time.Time) (*adaptiveDeadlineContext, context.CancelFunc) {
	if maxCeiling.Before(initialDeadline) {
		maxCeiling = initialDeadline
	}
	c := &adaptiveDeadlineContext{
		parent:     parent,
		deadline:   initialDeadline,
		maxCeiling: maxCeiling,
		done:       make(chan struct{}),
	}
	wait := time.Until(initialDeadline)
	if wait < 0 {
		wait = 0
	}
	c.timer = time.AfterFunc(wait, func() {
		c.finish(context.DeadlineExceeded)
	})
	go func() {
		select {
		case <-parent.Done():
			c.finish(parent.Err())
		case <-c.done:
		}
	}()
	return c, func() {
		c.finish(context.Canceled)
	}
}

func (c *adaptiveDeadlineContext) finish(err error) {
	c.mu.Lock()
	if c.err == nil {
		if err == nil {
			err = context.Canceled
		}
		c.err = err
		if c.timer != nil {
			c.timer.Stop()
		}
		close(c.done)
	}
	c.mu.Unlock()
}

func (c *adaptiveDeadlineContext) Extend(newDeadline time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	if newDeadline.After(c.maxCeiling) {
		newDeadline = c.maxCeiling
	}
	if !newDeadline.After(c.deadline) {
		return
	}
	c.deadline = newDeadline
	if c.timer != nil {
		c.timer.Reset(time.Until(newDeadline))
	}
}

func (c *adaptiveDeadlineContext) Deadline() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deadline, true
}

func (c *adaptiveDeadlineContext) Done() <-chan struct{} {
	return c.done
}

func (c *adaptiveDeadlineContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *adaptiveDeadlineContext) Value(key any) any {
	return c.parent.Value(key)
}

// Independent checks need time after the agent exits. This partitions the
// existing execution window; neither preparation nor model routing can extend it.
func inspectionReportingDeadline(overall, now time.Time, reserveLimit time.Duration, projectChecks bool) time.Time {
	if reserveLimit <= 0 || !overall.After(now) {
		return overall
	}
	reserve := overall.Sub(now) / 5
	if projectChecks {
		// Full native project checks can consume their five-minute command
		// budget. Share the remaining window equally, capped by that budget,
		// rather than giving model investigation four times as much time.
		reserve = overall.Sub(now) / 2
	}
	if reserve > reserveLimit {
		reserve = reserveLimit
	}
	return overall.Add(-reserve)
}

const inspectionReportingTimeout = task.InspectionReportingTimeout

// Persist the same upper bound used by native verification execution. Recognizing
// a marker only shortens a deadline; launch validation still grants capabilities.
func claimExecutionDeadline(t *task.Task, a *task.TaskAttempt, started time.Time, revision string) time.Time {
	deadline := started.Add(task.CalculateTaskTimeout(t.ExecutionCategory, a.AttemptNumber, t.Title, t.Instructions))
	verification := strings.Contains(revision, task.VerificationIntentMarker)
	deadline = verificationDeadline(deadline, started, verification || strings.Contains(revision, task.LiveInspectionMarker))
	if verification && strings.Contains(revision, `"finalize_attempt_id":`) && started.Add(30*time.Second).Before(deadline) {
		deadline = started.Add(30 * time.Second)
	}
	return deadline
}
