// Package taskfsm provides a lightweight, zero-dependency finite state machine (FSM)
// for task lifecycle orchestration following Agentic Scrum principles.
//
// Lifecycle flow:
//
//	planning → approved → running → review → completed
//	   ↓          ↓          ↓         ↓
//	canceled   canceled   blocked   blocked
//	              ↓          ↓         ↓
//	           planning   running   running
package taskfsm

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// State represents a discrete phase in the task lifecycle.
type State string

const (
	// StatePlanning represents task conception, backlog definition, and sprint planning.
	StatePlanning State = "planning"

	// StateApproved indicates scope approval by Product Owner / coordinator, ready for execution.
	StateApproved State = "approved"

	// StateRunning indicates active execution by an agent runner.
	StateRunning State = "running"

	// StateReview indicates execution completion undergoing verification or DoD evaluation.
	StateReview State = "review"

	// StateCompleted is a terminal state indicating all acceptance criteria and DoD are met.
	StateCompleted State = "completed"

	// StateCanceled is a terminal state indicating cancellation or abandonment.
	StateCanceled State = "canceled"

	// StateBlocked indicates progress is halted pending external resolution.
	StateBlocked State = "blocked"
)

// Sentinel errors returned by taskfsm operations.
var (
	ErrInvalidState       = errors.New("invalid task state")
	ErrIllegalTransition  = errors.New("illegal state transition")
	ErrTerminalState      = errors.New("cannot transition from terminal state")
	ErrPreHookRejected    = errors.New("transition rejected by guardrail hook")
	ErrSelfTransition     = errors.New("self-transition not permitted for this state")
)

// transitionMatrix defines allowed next states for each state.
var transitionMatrix = map[State][]State{
	StatePlanning: {
		StateApproved,
		StateCanceled,
	},
	StateApproved: {
		StateRunning,
		StatePlanning, // scope revision
		StateBlocked,  // prerequisite missing
		StateCanceled,
	},
	StateRunning: {
		StateReview,    // execution completed, evaluation pending
		StateCompleted, // DoD passed 100% via automated evaluation
		StateApproved,  // auto-requeued for next attempt
		StateBlocked,   // obstacle encountered
		StateCanceled,  // user requested cancellation
		StateRunning,   // checkpoint, progress update, or retry
	},
	StateReview: {
		StateCompleted, // DoD passed 100%
		StateApproved,  // auto-requeued to queue for next attempt
		StateRunning,   // DoD gap detected (agent-fixable immediate retry)
		StateBlocked,   // max attempts or unfixable blocker
		StatePlanning,  // returned for re-planning in next sprint
		StateCanceled,  // rejected
	},
	StateBlocked: {
		StateRunning,  // blocker resolved, resume execution
		StateApproved, // re-queued to queue
		StateReview,   // escalate blocker to review
		StatePlanning, // return to planning
		StateCanceled, // abandon task
	},
	StateCompleted: {}, // terminal
	StateCanceled:  {}, // terminal
}

// AllStates returns all valid lifecycle states in logical progression order.
func AllStates() []State {
	return []State{
		StatePlanning,
		StateApproved,
		StateRunning,
		StateReview,
		StateCompleted,
		StateCanceled,
		StateBlocked,
	}
}

// IsValidState reports whether s is a recognized task lifecycle state.
func IsValidState(s State) bool {
	switch s {
	case StatePlanning, StateApproved, StateRunning, StateReview,
		StateCompleted, StateCanceled, StateBlocked:
		return true
	default:
		return false
	}
}

// IsTerminal reports whether s is a terminal state (completed or canceled).
func IsTerminal(s State) bool {
	return s == StateCompleted || s == StateCanceled
}

// AllowedNextStates returns a slice of states that can be legally transitioned to from fromState.
func AllowedNextStates(fromState State) []State {
	targets, ok := transitionMatrix[fromState]
	if !ok {
		return nil
	}
	res := make([]State, len(targets))
	copy(res, targets)
	return res
}

// CanTransition reports whether transitioning from `from` to `to` is legally permitted.
func CanTransition(from, to State) bool {
	if !IsValidState(from) || !IsValidState(to) {
		return false
	}
	allowed := transitionMatrix[from]
	for _, target := range allowed {
		if target == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error describing why a transition from `from` to `to` is invalid.
func ValidateTransition(from, to State) error {
	if !IsValidState(from) {
		return fmt.Errorf("%w: unknown source state %q", ErrInvalidState, from)
	}
	if !IsValidState(to) {
		return fmt.Errorf("%w: unknown destination state %q", ErrInvalidState, to)
	}
	if IsTerminal(from) {
		return fmt.Errorf("%w: state %q is terminal; cannot transition to %q", ErrTerminalState, from, to)
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: cannot transition from %q to %q (allowed: %v)",
			ErrIllegalTransition, from, to, AllowedNextStates(from))
	}
	return nil
}

// TransitionRecord stores an immutable audit log entry of a state transition.
type TransitionRecord struct {
	From      State          `json:"from"`
	To        State          `json:"to"`
	Timestamp time.Time      `json:"timestamp"`
	Reason    string         `json:"reason,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// TransitionHook is a callback invoked during state transitions.
// If a pre-hook returns an error, the transition is aborted.
type TransitionHook func(from, to State, reason string, metadata map[string]any) error

// FSM is a thread-safe finite state machine engine for task lifecycle management.
type FSM struct {
	mu           sync.RWMutex
	current      State
	history      []TransitionRecord
	preHooks     []TransitionHook
	postHooks    []TransitionHook
	reopenAllowed bool
}

// Option configures an FSM instance.
type Option func(*FSM)

// WithReopenAllowed enables reopening from terminal states to planning.
func WithReopenAllowed(allow bool) Option {
	return func(f *FSM) {
		f.reopenAllowed = allow
	}
}

// New creates a new FSM instance initialized to initialState.
// If initialState is empty, StatePlanning is used as default.
func New(initialState State, opts ...Option) (*FSM, error) {
	if initialState == "" {
		initialState = StatePlanning
	}
	if !IsValidState(initialState) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidState, initialState)
	}

	f := &FSM{
		current: initialState,
		history: make([]TransitionRecord, 0, 8),
	}

	for _, opt := range opts {
		opt(f)
	}

	return f, nil
}

// Current returns the current state of the FSM.
func (f *FSM) Current() State {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.current
}

// CanTransitionTo reports whether the FSM can transition from its current state to target.
func (f *FSM) CanTransitionTo(target State) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.reopenAllowed && IsTerminal(f.current) && target == StatePlanning {
		return true
	}
	return CanTransition(f.current, target)
}

// AllowedTransitions returns all states the FSM can currently transition into.
func (f *FSM) AllowedTransitions() []State {
	f.mu.RLock()
	defer f.mu.RUnlock()

	allowed := AllowedNextStates(f.current)
	if f.reopenAllowed && IsTerminal(f.current) {
		allowed = append(allowed, StatePlanning)
	}
	return allowed
}

// Transition performs a validated state transition with an optional reason.
func (f *FSM) Transition(target State, reason ...string) error {
	r := ""
	if len(reason) > 0 {
		r = strings.Join(reason, " ")
	}
	return f.TransitionWithMetadata(target, r, nil)
}

// TransitionWithMetadata performs a validated state transition with audit metadata.
func (f *FSM) TransitionWithMetadata(target State, reason string, metadata map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Handle explicit reopen override if enabled
	if f.reopenAllowed && IsTerminal(f.current) && target == StatePlanning {
		// permitted reopen
	} else {
		if err := ValidateTransition(f.current, target); err != nil {
			return err
		}
	}

	// Execute pre-hooks (guardrails)
	for _, hook := range f.preHooks {
		if err := hook(f.current, target, reason, metadata); err != nil {
			return fmt.Errorf("%w: %v", ErrPreHookRejected, err)
		}
	}

	from := f.current
	f.current = target

	record := TransitionRecord{
		From:      from,
		To:        target,
		Timestamp: time.Now().UTC(),
		Reason:    reason,
		Metadata:  metadata,
	}
	f.history = append(f.history, record)

	// Execute post-hooks (notifications / listeners)
	for _, hook := range f.postHooks {
		_ = hook(from, target, reason, metadata)
	}

	return nil
}

// AddPreHook adds a guardrail hook executed before transition.
func (f *FSM) AddPreHook(hook TransitionHook) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.preHooks = append(f.preHooks, hook)
}

// AddPostHook adds an observer hook executed after transition completes.
func (f *FSM) AddPostHook(hook TransitionHook) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.postHooks = append(f.postHooks, hook)
}

// History returns a copy of all recorded state transitions.
func (f *FSM) History() []TransitionRecord {
	f.mu.RLock()
	defer f.mu.RUnlock()
	res := make([]TransitionRecord, len(f.history))
	copy(res, f.history)
	return res
}

// Reset moves the FSM to a specific state and clears history (for testing or reuse).
func (f *FSM) Reset(state State) error {
	if !IsValidState(state) {
		return fmt.Errorf("%w: %q", ErrInvalidState, state)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current = state
	f.history = f.history[:0]
	return nil
}

// FromCuanTaskStatus maps Cuan internal task status strings to taskfsm.State.
func FromCuanTaskStatus(cuanStatus string) (State, error) {
	switch strings.ToLower(strings.TrimSpace(cuanStatus)) {
	case "planning", "backlog":
		return StatePlanning, nil
	case "approved", "queued", "pending":
		return StateApproved, nil
	case "running", "doing":
		return StateRunning, nil
	case "review":
		return StateReview, nil
	case "completed", "done", "succeeded":
		return StateCompleted, nil
	case "canceled", "failed", "interrupted":
		return StateCanceled, nil
	case "blocked":
		return StateBlocked, nil
	default:
		return "", fmt.Errorf("%w: unknown Cuan status %q", ErrInvalidState, cuanStatus)
	}
}

// ToCuanTaskStatus maps taskfsm.State to Cuan internal canonical task status.
func ToCuanTaskStatus(state State) string {
	switch state {
	case StatePlanning:
		return "backlog"
	case StateApproved:
		return "queued"
	case StateRunning:
		return "doing"
	case StateReview:
		return "review"
	case StateCompleted:
		return "done"
	case StateCanceled:
		return "failed"
	case StateBlocked:
		return "blocked"
	default:
		return string(state)
	}
}
