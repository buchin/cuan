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
