package taskfsm

import (
	"fmt"
	"time"
)

// State represents a valid lifecycle state in the task/sprint lifecycle.
type State string

const (
	// StatePlanning represents the initial sprint planning phase.
	StatePlanning State = "planning"
	// StateApproved represents PO approval of sprint/task scope.
	StateApproved State = "approved"
	// StateRunning represents active implementation in progress.
	StateRunning State = "running"
	// StateReview represents sprint/task review evaluating DoD criteria.
	StateReview State = "review"
	// StateCompleted represents final completed and verified state.
	StateCompleted State = "completed"
	// StateBlocked represents a paused state due to concrete blockers.
	StateBlocked State = "blocked"
	// StateCanceled represents an aborted or canceled task/sprint.
	StateCanceled State = "canceled"
)

// AllStates returns the canonical slice of all recognized states.
func AllStates() []State {
	return []State{
		StatePlanning,
		StateApproved,
		StateRunning,
		StateReview,
		StateCompleted,
		StateBlocked,
		StateCanceled,
	}
}

// IsValidState returns true if s is a recognized lifecycle state.
func IsValidState(s State) bool {
	switch s {
	case StatePlanning, StateApproved, StateRunning, StateReview, StateCompleted, StateBlocked, StateCanceled:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if s is a terminal state (completed or canceled).
func (s State) IsTerminal() bool {
	return s == StateCompleted || s == StateCanceled
}

// Event represents an action or trigger causing a state transition.
type Event string

const (
	// EventApprove moves lifecycle from planning to approved (PO authorization).
	EventApprove Event = "approve"
	// EventStart moves lifecycle from approved to running.
	EventStart Event = "start"
	// EventSubmitReview moves lifecycle from running or blocked to review.
	EventSubmitReview Event = "submit_review"
	// EventComplete moves lifecycle from review to completed after DoD is satisfied.
	EventComplete Event = "complete"
	// EventBlock moves lifecycle from running to blocked when an obstacle arises.
	EventBlock Event = "block"
	// EventResume moves lifecycle from blocked back to running.
	EventResume Event = "resume"
	// EventReturnToDoing returns task from review back to running with gap guidance.
	EventReturnToDoing Event = "return_to_doing"
	// EventCancel aborts the task/sprint from any non-terminal state.
	EventCancel Event = "cancel"
)

// AllEvents returns the canonical slice of all recognized events.
func AllEvents() []Event {
	return []Event{
		EventApprove,
		EventStart,
		EventSubmitReview,
		EventComplete,
		EventBlock,
		EventResume,
		EventReturnToDoing,
		EventCancel,
	}
}

// TransitionRecord tracks the historical audit trail of state changes.
type TransitionRecord struct {
	ID        string    `json:"id,omitempty"`
	From      State     `json:"from"`
	To        State     `json:"to"`
	Event     Event     `json:"event"`
	Reason    string    `json:"reason,omitempty"`
	Actor     string    `json:"actor,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Forced    bool      `json:"forced,omitempty"`
}

// TransitionError indicates an illegal or rejected state transition.
type TransitionError struct {
	From   State  `json:"from"`
	To     State  `json:"to,omitempty"`
	Event  Event  `json:"event,omitempty"`
	Reason string `json:"reason"`
}

func (e *TransitionError) Error() string {
	if e.Event != "" {
		return fmt.Sprintf("taskfsm: illegal transition on event '%s' from state '%s': %s", e.Event, e.From, e.Reason)
	}
	return fmt.Sprintf("taskfsm: illegal transition from state '%s' to '%s': %s", e.From, e.To, e.Reason)
}

// FromCuanTaskStatus maps Cuan task string status to taskfsm.State.
func FromCuanTaskStatus(cuanStatus string) (State, error) {
	switch cuanStatus {
	case "backlog", "planning":
		return StatePlanning, nil
	case "queued", "approved":
		return StateApproved, nil
	case "doing", "running":
		return StateRunning, nil
	case "review":
		return StateReview, nil
	case "done", "completed":
		return StateCompleted, nil
	case "blocked":
		return StateBlocked, nil
	case "failed", "canceled", "cancelled":
		return StateCanceled, nil
	default:
		return "", fmt.Errorf("unknown cuan task status: %q", cuanStatus)
	}
}

// ToCuanTaskStatus maps taskfsm.State to Cuan task string status.
func ToCuanTaskStatus(s State) string {
	switch s {
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
	case StateBlocked:
		return "blocked"
	case StateCanceled:
		return "failed"
	default:
		return string(s)
	}
}

// transitionTargets defines allowed next states for each state.
var transitionTargets = map[State]map[State]bool{
	StatePlanning: {
		StateApproved: true,
		StateCanceled: true,
	},
	StateApproved: {
		StateRunning:  true,
		StatePlanning: true,
		StateBlocked:  true,
		StateCanceled: true,
	},
	StateRunning: {
		StateReview:    true,
		StateCompleted: true,
		StateApproved:  true,
		StateBlocked:   true,
		StateCanceled:  true,
	},
	StateReview: {
		StateCompleted: true,
		StateApproved:  true,
		StateRunning:   true,
		StateBlocked:   true,
		StatePlanning:  true,
		StateCanceled:  true,
	},
	StateBlocked: {
		StateRunning:  true,
		StateApproved: true,
		StateReview:   true,
		StatePlanning: true,
		StateCanceled: true,
	},
	StateCompleted: {},
	StateCanceled:  {},
}

// CanTransition reports whether transitioning from 'from' to 'to' is permitted.
func CanTransition(from, to State) bool {
	if from == to {
		return true
	}
	if allowed, ok := transitionTargets[from]; ok {
		return allowed[to]
	}
	return false
}

// ValidateTransition validates if a transition from 'from' to 'to' is legal.
func ValidateTransition(from, to State) error {
	if !IsValidState(from) {
		return fmt.Errorf("invalid source state: %q", from)
	}
	if !IsValidState(to) {
		return fmt.Errorf("invalid destination state: %q", to)
	}
	if from.IsTerminal() && from != to {
		return fmt.Errorf("cannot transition from terminal state: %q", from)
	}
	if !CanTransition(from, to) {
		return fmt.Errorf("illegal state transition from %q to %q", from, to)
	}
	return nil
}
