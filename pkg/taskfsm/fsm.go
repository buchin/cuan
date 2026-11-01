package taskfsm

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// transitionKey identifies a transition from a state via an event.
type transitionKey struct {
	from  State
	event Event
}

// guardKey identifies a guard between two states.
type guardKey struct {
	from State
	to   State
}

// standardTransitions defines the canonical lifecycle transition table.
var standardTransitions = map[transitionKey]State{
	{from: StatePlanning, event: EventApprove}:       StateApproved,
	{from: StatePlanning, event: EventCancel}:        StateCanceled,
	{from: StateApproved, event: EventStart}:         StateRunning,
	{from: StateApproved, event: EventCancel}:        StateCanceled,
	{from: StateRunning, event: EventSubmitReview}:   StateReview,
	{from: StateRunning, event: EventBlock}:          StateBlocked,
	{from: StateRunning, event: EventCancel}:         StateCanceled,
	{from: StateBlocked, event: EventResume}:         StateRunning,
	{from: StateBlocked, event: EventSubmitReview}:   StateReview,
	{from: StateBlocked, event: EventCancel}:         StateCanceled,
	{from: StateReview, event: EventComplete}:        StateCompleted,
	{from: StateReview, event: EventReturnToDoing}:   StateRunning,
	{from: StateReview, event: EventCancel}:          StateCanceled,
}

// GuardFunc allows caller to inject custom preconditions before a transition executes.
type GuardFunc func(from State, to State, event Event, reason, actor string) error

// ListenerFunc receives a notification when a transition successfully occurs.
type ListenerFunc func(record TransitionRecord)

// TransitionOption configures metadata for a state transition.
type TransitionOption func(*transitionOptions)

type transitionOptions struct {
	reason string
	actor  string
}

// WithReason provides context or justification for the transition.
func WithReason(reason string) TransitionOption {
	return func(o *transitionOptions) {
		o.reason = reason
	}
}

// WithActor records the human or agent initiating the transition.
func WithActor(actor string) TransitionOption {
	return func(o *transitionOptions) {
		o.actor = actor
	}
}

// StateMachine manages the lifecycle state transitions with guardrails.
type StateMachine struct {
	mu             sync.RWMutex
	current        State
	history        []TransitionRecord
	guards         map[guardKey][]GuardFunc
	listeners      []ListenerFunc
	stateListeners map[State][]ListenerFunc
}

// New creates a new StateMachine starting at the specified state (defaulting to StatePlanning).
func New(initial State) *StateMachine {
	if !IsValidState(initial) {
		initial = StatePlanning
	}
	return &StateMachine{
		current:        initial,
		history:        make([]TransitionRecord, 0),
		guards:         make(map[guardKey][]GuardFunc),
		listeners:      make([]ListenerFunc, 0),
		stateListeners: make(map[State][]ListenerFunc),
	}
}

// Current returns the current state.
func (sm *StateMachine) Current() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// History returns a copy of all transitions recorded by this state machine.
func (sm *StateMachine) History() []TransitionRecord {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	copied := make([]TransitionRecord, len(sm.history))
	copy(copied, sm.history)
	return copied
}

// Can returns true if the event can be validly triggered from the current state.
func (sm *StateMachine) Can(event Event) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	target, exists := standardTransitions[transitionKey{from: sm.current, event: event}]
	if !exists {
		return false
	}

	// Verify guards if any
	gKey := guardKey{from: sm.current, to: target}
	if guards, ok := sm.guards[gKey]; ok {
		for _, g := range guards {
			if err := g(sm.current, target, event, "", ""); err != nil {
				return false
			}
		}
	}

	return true
}

// CanTransitionTo returns true if there is a valid transition path from current state to target.
func (sm *StateMachine) CanTransitionTo(target State) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for k, dest := range standardTransitions {
		if k.from == sm.current && dest == target {
			return true
		}
	}
	return false
}

// ValidEvents returns a list of events currently valid from the current state.
func (sm *StateMachine) ValidEvents() []Event {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var events []Event
	for k := range standardTransitions {
		if k.from == sm.current {
			events = append(events, k.event)
		}
	}
	return events
}

// ValidTransitions returns a list of target states reachable from the current state.
func (sm *StateMachine) ValidTransitions() []State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var states []State
	seen := make(map[State]bool)
	for k, dest := range standardTransitions {
		if k.from == sm.current && !seen[dest] {
			seen[dest] = true
			states = append(states, dest)
		}
	}
	return states
}

// Transition moves the state machine to a new state based on the triggered event.
func (sm *StateMachine) Transition(event Event, opts ...TransitionOption) error {
	opt := &transitionOptions{}
	for _, o := range opts {
		o(opt)
	}

	sm.mu.Lock()

	target, exists := standardTransitions[transitionKey{from: sm.current, event: event}]
	if !exists {
		currentState := sm.current
		sm.mu.Unlock()
		return &TransitionError{
			From:   currentState,
			Event:  event,
			Reason: fmt.Sprintf("no valid transition on event '%s' from state '%s'", event, currentState),
		}
	}

	// Check guards
	gKey := guardKey{from: sm.current, to: target}
	if guards, ok := sm.guards[gKey]; ok {
		for _, g := range guards {
			if err := g(sm.current, target, event, opt.reason, opt.actor); err != nil {
				currentState := sm.current
				sm.mu.Unlock()
				return &TransitionError{
					From:   currentState,
					To:     target,
					Event:  event,
					Reason: fmt.Sprintf("guard check rejected transition: %v", err),
				}
			}
		}
	}

	prev := sm.current
	sm.current = target

	record := TransitionRecord{
		From:      prev,
		To:        target,
		Event:     event,
		Reason:    opt.reason,
		Actor:     opt.actor,
		Timestamp: time.Now().UTC(),
		Forced:    false,
	}
	sm.history = append(sm.history, record)

	// Collect listeners to fire outside lock
	var notifyListeners []ListenerFunc
	notifyListeners = append(notifyListeners, sm.listeners...)
	if sl, ok := sm.stateListeners[target]; ok {
		notifyListeners = append(notifyListeners, sl...)
	}

	sm.mu.Unlock()

	for _, l := range notifyListeners {
		l(record)
	}

	return nil
}

// Force manually overrides the state machine to target state with explicit audit logging.
func (sm *StateMachine) Force(target State, opts ...TransitionOption) error {
	if !IsValidState(target) {
		return &TransitionError{
			To:     target,
			Reason: fmt.Sprintf("invalid state '%s'", target),
		}
	}

	opt := &transitionOptions{}
	for _, o := range opts {
		o(opt)
	}

	sm.mu.Lock()
	prev := sm.current
	sm.current = target

	record := TransitionRecord{
		From:      prev,
		To:        target,
		Event:     Event("force"),
		Reason:    opt.reason,
		Actor:     opt.actor,
		Timestamp: time.Now().UTC(),
		Forced:    true,
	}
	sm.history = append(sm.history, record)

	var notifyListeners []ListenerFunc
	notifyListeners = append(notifyListeners, sm.listeners...)
	if sl, ok := sm.stateListeners[target]; ok {
		notifyListeners = append(notifyListeners, sl...)
	}
	sm.mu.Unlock()

	for _, l := range notifyListeners {
		l(record)
	}

	return nil
}

// AddGuard registers a custom precondition function between two states.
func (sm *StateMachine) AddGuard(from, to State, guard GuardFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	key := guardKey{from: from, to: to}
	sm.guards[key] = append(sm.guards[key], guard)
}

// OnTransition registers a listener callback invoked upon every valid transition.
func (sm *StateMachine) OnTransition(listener ListenerFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.listeners = append(sm.listeners, listener)
}

// OnState registers a listener callback invoked whenever entering a specific target state.
func (sm *StateMachine) OnState(state State, listener ListenerFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.stateListeners[state] = append(sm.stateListeners[state], listener)
}

// stateMachineJSON helper for marshaling/unmarshaling.
type stateMachineJSON struct {
	Current State              `json:"current"`
	History []TransitionRecord `json:"history,omitempty"`
}

// MarshalJSON serializes the state machine to JSON.
func (sm *StateMachine) MarshalJSON() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return json.Marshal(stateMachineJSON{
		Current: sm.current,
		History: sm.history,
	})
}

// UnmarshalJSON deserializes the state machine from JSON.
func (sm *StateMachine) UnmarshalJSON(data []byte) error {
	var aux stateMachineJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if !IsValidState(aux.Current) {
		aux.Current = StatePlanning
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.current = aux.Current
	sm.history = aux.History
	if sm.guards == nil {
		sm.guards = make(map[guardKey][]GuardFunc)
	}
	if sm.listeners == nil {
		sm.listeners = make([]ListenerFunc, 0)
	}
	if sm.stateListeners == nil {
		sm.stateListeners = make(map[State][]ListenerFunc)
	}
	return nil
}
