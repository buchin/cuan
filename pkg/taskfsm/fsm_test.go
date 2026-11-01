package taskfsm

import (
	"errors"
	"sync"
	"testing"
)

func TestAllStatesAndValidation(t *testing.T) {
	states := AllStates()
	if len(states) != 7 {
		t.Fatalf("expected 7 states, got %d", len(states))
	}

	for _, s := range states {
		if !IsValidState(s) {
			t.Errorf("state %q should be valid", s)
		}
	}

	if IsValidState("unknown_state") {
		t.Error("unknown_state should not be valid")
	}

	if !IsTerminal(StateCompleted) || !IsTerminal(StateCanceled) {
		t.Error("completed and canceled must be terminal")
	}
	if IsTerminal(StatePlanning) || IsTerminal(StateRunning) {
		t.Error("planning and running must not be terminal")
	}
}

func TestLifecycleTransitions_HappyPath(t *testing.T) {
	// Lifecycle: planning → approved → running → review → completed
	fsm, err := New(StatePlanning)
	if err != nil {
		t.Fatalf("New(StatePlanning) failed: %v", err)
	}

	if fsm.Current() != StatePlanning {
		t.Fatalf("expected current state planning, got %s", fsm.Current())
	}

	// 1. planning -> approved
	if !fsm.CanTransitionTo(StateApproved) {
		t.Error("should allow transition planning -> approved")
	}
	if err := fsm.Transition(StateApproved, "PO approved sprint backlog"); err != nil {
		t.Fatalf("failed transition to approved: %v", err)
	}

	// 2. approved -> running
	if !fsm.CanTransitionTo(StateRunning) {
		t.Error("should allow transition approved -> running")
	}
	if err := fsm.Transition(StateRunning, "Runner claimed task attempt"); err != nil {
		t.Fatalf("failed transition to running: %v", err)
	}

	// 3. running -> review
	if !fsm.CanTransitionTo(StateReview) {
		t.Error("should allow transition running -> review")
	}
	if err := fsm.Transition(StateReview, "Execution completed, awaiting DoD"); err != nil {
		t.Fatalf("failed transition to review: %v", err)
	}

	// 4. review -> completed
	if !fsm.CanTransitionTo(StateCompleted) {
		t.Error("should allow transition review -> completed")
	}
	if err := fsm.Transition(StateCompleted, "DoD passed 100%"); err != nil {
		t.Fatalf("failed transition to completed: %v", err)
	}

	if fsm.Current() != StateCompleted {
		t.Errorf("expected final state completed, got %s", fsm.Current())
	}

	history := fsm.History()
	if len(history) != 4 {
		t.Fatalf("expected 4 transitions in history, got %d", len(history))
	}
	if history[0].From != StatePlanning || history[0].To != StateApproved {
		t.Errorf("history[0] mismatch: %+v", history[0])
	}
	if history[3].From != StateReview || history[3].To != StateCompleted {
		t.Errorf("history[3] mismatch: %+v", history[3])
	}
}

func TestLifecycleTransitions_Guardrails_IllegalTransitions(t *testing.T) {
	illegalPairs := []struct {
		from State
		to   State
	}{
		// Illegal skips
		{StatePlanning, StateRunning},
		{StatePlanning, StateReview},
		{StatePlanning, StateCompleted},
		{StatePlanning, StateBlocked},

		{StateApproved, StateReview},
		{StateApproved, StateCompleted},

		{StateRunning, StatePlanning}, // cannot skip back to planning without review/blocker

		// From terminal states
		{StateCompleted, StateRunning},
		{StateCompleted, StateReview},
		{StateCompleted, StateApproved},
		{StateCanceled, StateRunning},
		{StateCanceled, StateApproved},
	}

	for _, tc := range illegalPairs {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			if CanTransition(tc.from, tc.to) {
				t.Errorf("CanTransition(%q, %q) returned true, expected false", tc.from, tc.to)
			}
			err := ValidateTransition(tc.from, tc.to)
			if err == nil {
				t.Errorf("ValidateTransition(%q, %q) expected error, got nil", tc.from, tc.to)
			}
			if IsTerminal(tc.from) {
				if !errors.Is(err, ErrTerminalState) {
					t.Errorf("expected ErrTerminalState, got %v", err)
				}
			} else {
				if !errors.Is(err, ErrIllegalTransition) {
					t.Errorf("expected ErrIllegalTransition, got %v", err)
				}
			}
		})
	}
}

func TestLifecycleTransitions_ReviewAutoRequeue(t *testing.T) {
	// Review -> Running (DoD gap fixable auto-requeue)
	fsm, err := New(StateReview)
	if err != nil {
		t.Fatal(err)
	}

	if !fsm.CanTransitionTo(StateRunning) {
		t.Error("review should be allowed to auto-requeue to running")
	}
	if err := fsm.Transition(StateRunning, "DoD evaluator auto-requeue: gap fixable"); err != nil {
		t.Fatalf("failed auto-requeue transition: %v", err)
	}
	if fsm.Current() != StateRunning {
		t.Errorf("expected state running, got %s", fsm.Current())
	}
}

func TestLifecycleTransitions_BlockedAndResume(t *testing.T) {
	fsm, err := New(StateRunning)
	if err != nil {
		t.Fatal(err)
	}

	// Running -> Blocked
	if err := fsm.Transition(StateBlocked, "Waiting for external credentials"); err != nil {
		t.Fatalf("failed transition to blocked: %v", err)
	}

	// Blocked -> Running (unblocked)
	if !fsm.CanTransitionTo(StateRunning) {
		t.Error("blocked should be able to resume to running")
	}
	if err := fsm.Transition(StateRunning, "Credentials supplied, resuming"); err != nil {
		t.Fatalf("failed to resume: %v", err)
	}
	if fsm.Current() != StateRunning {
		t.Errorf("expected running, got %s", fsm.Current())
	}
}

func TestLifecycleTransitions_CanceledFromVariousStates(t *testing.T) {
	cancellableStates := []State{
		StatePlanning,
		StateApproved,
		StateRunning,
		StateReview,
		StateBlocked,
	}

	for _, s := range cancellableStates {
		t.Run("CancelFrom_"+string(s), func(t *testing.T) {
			fsm, err := New(s)
			if err != nil {
				t.Fatal(err)
			}
			if !fsm.CanTransitionTo(StateCanceled) {
				t.Errorf("state %s should allow cancellation", s)
			}
			if err := fsm.Transition(StateCanceled, "User requested cancel"); err != nil {
				t.Errorf("failed to cancel from %s: %v", s, err)
			}
		})
	}
}

func TestFSM_GuardrailPreHooks(t *testing.T) {
	fsm, err := New(StateRunning)
	if err != nil {
		t.Fatal(err)
	}

	// Hook that rejects review if reason doesn't contain DoD
	fsm.AddPreHook(func(from, to State, reason string, metadata map[string]any) error {
		if to == StateReview && reason == "" {
			return errors.New("cannot submit to review without verification reason")
		}
		return nil
	})

	// Attempt without reason -> rejected
	err = fsm.Transition(StateReview, "")
	if err == nil {
		t.Fatal("expected prehook rejection, got nil")
	}
	if !errors.Is(err, ErrPreHookRejected) {
		t.Fatalf("expected ErrPreHookRejected, got %v", err)
	}
	if fsm.Current() != StateRunning {
		t.Errorf("state should remain running after rejected transition, got %s", fsm.Current())
	}

	// Attempt with valid reason -> accepted
	err = fsm.Transition(StateReview, "Verification report attached")
	if err != nil {
		t.Fatalf("expected successful transition with valid reason, got %v", err)
	}
	if fsm.Current() != StateReview {
		t.Errorf("expected state review, got %s", fsm.Current())
	}
}

func TestFSM_PostHooksAndMetadata(t *testing.T) {
	fsm, err := New(StateRunning)
	if err != nil {
		t.Fatal(err)
	}

	var hookInvoked bool
	var capturedFrom, capturedTo State
	var capturedReason string

	fsm.AddPostHook(func(from, to State, reason string, metadata map[string]any) error {
		hookInvoked = true
		capturedFrom = from
		capturedTo = to
		capturedReason = reason
		return nil
	})

	meta := map[string]any{"attempt": 1, "duration_ms": 350}
	err = fsm.TransitionWithMetadata(StateReview, "Finished attempt", meta)
	if err != nil {
		t.Fatalf("transition failed: %v", err)
	}

	if !hookInvoked {
		t.Error("post hook was not invoked")
	}
	if capturedFrom != StateRunning || capturedTo != StateReview {
		t.Errorf("hook captured wrong states: %s -> %s", capturedFrom, capturedTo)
	}
	if capturedReason != "Finished attempt" {
		t.Errorf("hook captured wrong reason: %s", capturedReason)
	}

	hist := fsm.History()
	if len(hist) != 1 || hist[0].Metadata["attempt"] != 1 {
		t.Errorf("metadata not preserved in history: %+v", hist)
	}
}

func TestFSM_ConcurrentSafety(t *testing.T) {
	fsm, err := New(StateRunning)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	// Concurrently read state, transitions, history, and perform self-transitions
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 15; j++ {
				_ = fsm.Current()
				_ = fsm.CanTransitionTo(StateReview)
				_ = fsm.AllowedTransitions()
				_ = fsm.History()
				// self transition allowed for running (progress)
				_ = fsm.Transition(StateRunning, "progress heartbeat")
			}
		}(i)
	}
	wg.Wait()

	if fsm.Current() != StateRunning {
		t.Errorf("expected state running, got %s", fsm.Current())
	}
}

func TestFSM_WithReopenAllowed(t *testing.T) {
	fsm, err := New(StateCompleted, WithReopenAllowed(true))
	if err != nil {
		t.Fatal(err)
	}

	if !fsm.CanTransitionTo(StatePlanning) {
		t.Error("expected reopen to planning to be allowed when WithReopenAllowed is true")
	}
	if err := fsm.Transition(StatePlanning, "Sprint re-planning for new scope"); err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	if fsm.Current() != StatePlanning {
		t.Errorf("expected planning, got %s", fsm.Current())
	}
}

func TestFSM_StatusMapping(t *testing.T) {
	cases := []struct {
		cuanStatus string
		expected   State
	}{
		{"backlog", StatePlanning},
		{"planning", StatePlanning},
		{"queued", StateApproved},
		{"pending", StateApproved},
		{"approved", StateApproved},
		{"doing", StateRunning},
		{"running", StateRunning},
		{"review", StateReview},
		{"done", StateCompleted},
		{"completed", StateCompleted},
		{"succeeded", StateCompleted},
		{"failed", StateCanceled},
		{"canceled", StateCanceled},
		{"blocked", StateBlocked},
	}

	for _, tc := range cases {
		s, err := FromCuanTaskStatus(tc.cuanStatus)
		if err != nil {
			t.Errorf("FromCuanTaskStatus(%q) error: %v", tc.cuanStatus, err)
		}
		if s != tc.expected {
			t.Errorf("FromCuanTaskStatus(%q) = %q, expected %q", tc.cuanStatus, s, tc.expected)
		}
	}

	_, err := FromCuanTaskStatus("invalid_xyz")
	if err == nil {
		t.Error("expected error for invalid Cuan status")
	}

	// Test ToCuanTaskStatus
	if ToCuanTaskStatus(StatePlanning) != "backlog" {
		t.Errorf("expected backlog, got %s", ToCuanTaskStatus(StatePlanning))
	}
	if ToCuanTaskStatus(StateApproved) != "queued" {
		t.Errorf("expected queued, got %s", ToCuanTaskStatus(StateApproved))
	}
	if ToCuanTaskStatus(StateRunning) != "doing" {
		t.Errorf("expected doing, got %s", ToCuanTaskStatus(StateRunning))
	}
	if ToCuanTaskStatus(StateReview) != "review" {
		t.Errorf("expected review, got %s", ToCuanTaskStatus(StateReview))
	}
	if ToCuanTaskStatus(StateCompleted) != "done" {
		t.Errorf("expected done, got %s", ToCuanTaskStatus(StateCompleted))
	}
}

func TestFSM_ResetAndInvalidInitial(t *testing.T) {
	_, err := New(State("invalid_state"))
	if err == nil {
		t.Error("expected error on invalid initial state")
	}

	fsm, err := New(StatePlanning)
	if err != nil {
		t.Fatal(err)
	}

	if err := fsm.Transition(StateApproved, "Approved"); err != nil {
		t.Fatal(err)
	}
	if len(fsm.History()) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(fsm.History()))
	}

	if err := fsm.Reset(StatePlanning); err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	if fsm.Current() != StatePlanning {
		t.Errorf("expected current state planning, got %s", fsm.Current())
	}
	if len(fsm.History()) != 0 {
		t.Errorf("expected history cleared on reset, got %d", len(fsm.History()))
	}

	if err := fsm.Reset(State("bad_state")); err == nil {
		t.Error("expected error resetting to bad state")
	}
}

func BenchmarkFSM_Transitions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fsm, _ := New(StatePlanning)
		_ = fsm.Transition(StateApproved)
		_ = fsm.Transition(StateRunning)
		_ = fsm.Transition(StateReview)
		_ = fsm.Transition(StateCompleted)
	}
}
