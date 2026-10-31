package runner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event type constants
const (
	EventTaskAttemptSucceeded = "task:attempt_succeeded"
)

// AttemptSucceededEvent contains metadata emitted when a worker successfully completes a task attempt execution.
type AttemptSucceededEvent struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"` // task:attempt_succeeded
	TaskID        string         `json:"task_id"`
	AttemptID     string         `json:"attempt_id"`
	AttemptNumber int            `json:"attempt_number"`
	WorkingDir    string         `json:"working_dir"`
	BranchName    string         `json:"branch_name"`
	CommitSHA     string         `json:"commit_sha"`
	ExecutionLane string         `json:"execution_lane"`
	Status        string         `json:"status"` // succeeded
	Summary       string         `json:"summary"`
	CompleteNote  string         `json:"complete_note"`
	EmittedAt     time.Time      `json:"emitted_at"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// EventListener is a callback function for attempt completion events.
type EventListener func(ctx context.Context, evt AttemptSucceededEvent) error

// EventBus provides a thread-safe in-process event publish/subscribe broker.
type EventBus struct {
	mu        sync.RWMutex
	listeners []EventListener
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make([]EventListener, 0),
	}
}

// Subscribe registers an event listener.
func (b *EventBus) Subscribe(listener EventListener) {
	if listener == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, listener)
}

// Publish dispatches an event to all registered listeners concurrently.
func (b *EventBus) Publish(ctx context.Context, evt AttemptSucceededEvent) {
	b.mu.RLock()
	listeners := make([]EventListener, len(b.listeners))
	copy(listeners, b.listeners)
	b.mu.RUnlock()

	for _, l := range listeners {
		go func(fn EventListener) {
			_ = fn(ctx, evt)
		}(l)
	}
}

// PublishSync dispatches an event synchronously to all registered listeners.
func (b *EventBus) PublishSync(ctx context.Context, evt AttemptSucceededEvent) error {
	b.mu.RLock()
	listeners := make([]EventListener, len(b.listeners))
	copy(listeners, b.listeners)
	b.mu.RUnlock()

	var firstErr error
	for _, l := range listeners {
		if err := l(ctx, evt); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// GlobalEventBus is the shared default event bus instance.
var GlobalEventBus = NewEventBus()

// EmitAttemptSucceeded publishes a task:attempt_succeeded event to the GlobalEventBus.
func EmitAttemptSucceeded(ctx context.Context, evt AttemptSucceededEvent) {
	if evt.EventID == "" {
		evt.EventID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if evt.EventType == "" {
		evt.EventType = EventTaskAttemptSucceeded
	}
	if evt.EmittedAt.IsZero() {
		evt.EmittedAt = time.Now().UTC()
	}
	GlobalEventBus.Publish(ctx, evt)
}
