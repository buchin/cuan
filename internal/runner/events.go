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
	EventTaskAttemptFailed    = "task:attempt_failed"
	EventTaskAttemptCompleted = "task:attempt_completed"
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

// AttemptCompletedEvent contains comprehensive metadata emitted when an attempt finishes execution
// (both exit 0 / succeeded and exit non-zero / failed).
type AttemptCompletedEvent struct {
	EventID       string         `json:"event_id"`
	EventType     string         `json:"event_type"` // task:attempt_completed, task:attempt_succeeded, task:attempt_failed
	TaskID        string         `json:"task_id"`
	AttemptID     string         `json:"attempt_id"`
	AttemptNumber int            `json:"attempt_number"`
	RunnerID      string         `json:"runner_id"`
	ExitCode      int            `json:"exit_code"`
	WorkingDir    string         `json:"working_dir"`
	BranchName    string         `json:"branch_name"`
	CommitSHA     string         `json:"commit_sha"`
	ExecutionLane string         `json:"execution_lane"`
	Status        string         `json:"status"` // succeeded, failed, canceled
	Summary       string         `json:"summary"`
	CompleteNote  string         `json:"complete_note"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	SourceRoomID  string         `json:"source_room_id,omitempty"`
	EmittedAt     time.Time      `json:"emitted_at"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// EventListener is a callback function for attempt completion events.
type EventListener func(ctx context.Context, evt AttemptSucceededEvent) error

// AttemptCompletedListener is a callback function for attempt finished events (success or failure).
type AttemptCompletedListener func(ctx context.Context, evt AttemptCompletedEvent) error

// EventBus provides a thread-safe in-process event publish/subscribe broker.
type EventBus struct {
	mu                 sync.RWMutex
	listeners          []EventListener
	completedListeners []AttemptCompletedListener
}

// NewEventBus creates a new EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		listeners:          make([]EventListener, 0),
		completedListeners: make([]AttemptCompletedListener, 0),
	}
}

// Subscribe registers a legacy success event listener.
func (b *EventBus) Subscribe(listener EventListener) {
	if listener == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, listener)
}

// SubscribeCompleted registers an event listener for all attempt completions (succeeded or failed).
func (b *EventBus) SubscribeCompleted(listener AttemptCompletedListener) {
	if listener == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.completedListeners = append(b.completedListeners, listener)
}

// Publish dispatches a success event to all registered legacy listeners and completed listeners concurrently.
func (b *EventBus) Publish(ctx context.Context, evt AttemptSucceededEvent) {
	b.mu.RLock()
	listeners := make([]EventListener, len(b.listeners))
	copy(listeners, b.listeners)
	compListeners := make([]AttemptCompletedListener, len(b.completedListeners))
	copy(compListeners, b.completedListeners)
	b.mu.RUnlock()

	for _, l := range listeners {
		go func(fn EventListener) {
			_ = fn(ctx, evt)
		}(l)
	}

	compEvt := AttemptCompletedEvent{
		EventID:       evt.EventID,
		EventType:     evt.EventType,
		TaskID:        evt.TaskID,
		AttemptID:     evt.AttemptID,
		AttemptNumber: evt.AttemptNumber,
		WorkingDir:    evt.WorkingDir,
		BranchName:    evt.BranchName,
		CommitSHA:     evt.CommitSHA,
		ExecutionLane: evt.ExecutionLane,
		Status:        evt.Status,
		Summary:       evt.Summary,
		CompleteNote:  evt.CompleteNote,
		EmittedAt:     evt.EmittedAt,
		Metadata:      evt.Metadata,
	}
	for _, l := range compListeners {
		go func(fn AttemptCompletedListener) {
			_ = fn(ctx, compEvt)
		}(l)
	}
}

// PublishCompleted dispatches an attempt completed event to all registered completed listeners concurrently.
func (b *EventBus) PublishCompleted(ctx context.Context, evt AttemptCompletedEvent) {
	b.mu.RLock()
	compListeners := make([]AttemptCompletedListener, len(b.completedListeners))
	copy(compListeners, b.completedListeners)
	legacyListeners := make([]EventListener, len(b.listeners))
	copy(legacyListeners, b.listeners)
	b.mu.RUnlock()

	for _, l := range compListeners {
		go func(fn AttemptCompletedListener) {
			_ = fn(ctx, evt)
		}(l)
	}

	// If succeeded, also trigger legacy success listeners
	if evt.Status == "succeeded" {
		legacyEvt := AttemptSucceededEvent{
			EventID:       evt.EventID,
			EventType:     EventTaskAttemptSucceeded,
			TaskID:        evt.TaskID,
			AttemptID:     evt.AttemptID,
			AttemptNumber: evt.AttemptNumber,
			WorkingDir:    evt.WorkingDir,
			BranchName:    evt.BranchName,
			CommitSHA:     evt.CommitSHA,
			ExecutionLane: evt.ExecutionLane,
			Status:        evt.Status,
			Summary:       evt.Summary,
			CompleteNote:  evt.CompleteNote,
			EmittedAt:     evt.EmittedAt,
			Metadata:      evt.Metadata,
		}
		for _, l := range legacyListeners {
			go func(fn EventListener) {
				_ = fn(ctx, legacyEvt)
			}(l)
		}
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

// PublishSyncCompleted dispatches an event synchronously to all registered completion listeners.
func (b *EventBus) PublishSyncCompleted(ctx context.Context, evt AttemptCompletedEvent) error {
	b.mu.RLock()
	compListeners := make([]AttemptCompletedListener, len(b.completedListeners))
	copy(compListeners, b.completedListeners)
	b.mu.RUnlock()

	var firstErr error
	for _, l := range compListeners {
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

// EmitAttemptCompleted publishes a task:attempt_completed event to the GlobalEventBus.
func EmitAttemptCompleted(ctx context.Context, evt AttemptCompletedEvent) {
	if evt.EventID == "" {
		evt.EventID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if evt.EventType == "" {
		if evt.Status == "succeeded" {
			evt.EventType = EventTaskAttemptSucceeded
		} else {
			evt.EventType = EventTaskAttemptFailed
		}
	}
	if evt.EmittedAt.IsZero() {
		evt.EmittedAt = time.Now().UTC()
	}
	GlobalEventBus.PublishCompleted(ctx, evt)
}
