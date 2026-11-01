package taskfsm_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/buchin/cuan/pkg/taskfsm"
)

func TestStateMachine_HappyPath(t *testing.T) {
	sm := taskfsm.New(taskfsm.StatePlanning)

	if sm.Current() != taskfsm.StatePlanning {
		t.Fatalf("expected initial state %s, got %s", taskfsm.StatePlanning, sm.Current())
	}

	// 1. Planning -> Approved
	if !sm.Can(taskfsm.EventApprove) {
		t.Fatalf("expected can approve from planning")
	}
	if err := sm.Transition(taskfsm.EventApprove, taskfsm.WithReason("PO approved scope"), taskfsm.WithActor("PO")); err != nil {
		t.Fatalf("failed to approve: %v", err)
	}
	if sm.Current() != taskfsm.StateApproved {
		t.Fatalf("expected state approved, got %s", sm.Current())
	}

	// 2. Approved -> Running
	if !sm.Can(taskfsm.EventStart) {
		t.Fatalf("expected can start from approved")
	}
	if err := sm.Transition(taskfsm.EventStart, taskfsm.WithActor("AgentSession")); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	if sm.Current() != taskfsm.StateRunning {
		t.Fatalf("expected state running, got %s", sm.Current())
	}

	// 3. Running -> Review
	if !sm.Can(taskfsm.EventSubmitReview) {
		t.Fatalf("expected can submit review from running")
	}
	if err := sm.Transition(taskfsm.EventSubmitReview, taskfsm.WithReason("All tasks completed"), taskfsm.WithActor("AgentSession")); err != nil {
		t.Fatalf("failed to submit review: %v", err)
	}
	if sm.Current() != taskfsm.StateReview {
		t.Fatalf("expected state review, got %s", sm.Current())
	}

	// 4. Review -> Completed
	if !sm.Can(taskfsm.EventComplete) {
		t.Fatalf("expected can complete from review")
	}
	if err := sm.Transition(taskfsm.EventComplete, taskfsm.WithReason("DoD 100% verified"), taskfsm.WithActor("PO")); err != nil {
		t.Fatalf("failed to complete: %v", err)
	}
	if sm.Current() != taskfsm.StateCompleted {
		t.Fatalf("expected state completed, got %s", sm.Current())
	}
	if !sm.Current().IsTerminal() {
		t.Fatalf("expected completed to be terminal")
	}

	// Verify history
	hist := sm.History()
	if len(hist) != 4 {
		t.Fatalf("expected 4 history records, got %d", len(hist))
	}
	if hist[0].From != taskfsm.StatePlanning || hist[0].To != taskfsm.StateApproved {
		t.Errorf("unexpected first history record: %+v", hist[0])
	}
	if hist[3].From != taskfsm.StateReview || hist[3].To != taskfsm.StateCompleted {
		t.Errorf("unexpected fourth history record: %+v", hist[3])
	}
}

func TestStateMachine_IllegalTransitions(t *testing.T) {
	sm := taskfsm.New(taskfsm.StatePlanning)

	// Bypassing approval directly to running is strictly forbidden (§3)
	if sm.Can(taskfsm.EventStart) {
		t.Fatalf("illegal transition EventStart should not be permitted in planning")
	}
	err := sm.Transition(taskfsm.EventStart)
	if err == nil {
		t.Fatalf("expected error transitioning with EventStart from planning")
	}
	var transErr *taskfsm.TransitionError
	if !errors.As(err, &transErr) {
		t.Fatalf("expected *taskfsm.TransitionError, got %T", err)
	}

	// Completed state cannot transition anywhere
	smCompleted := taskfsm.New(taskfsm.StateCompleted)
	for _, ev := range taskfsm.AllEvents() {
		if smCompleted.Can(ev) {
			t.Errorf("completed state should not allow event %s", ev)
		}
		if err := smCompleted.Transition(ev); err == nil {
			t.Errorf("expected error transitioning from completed with event %s", ev)
		}
	}
}

func TestStateMachine_BlockedAndResume(t *testing.T) {
	sm := taskfsm.New(taskfsm.StateRunning)

	// Running -> Blocked
	if err := sm.Transition(taskfsm.EventBlock, taskfsm.WithReason("Missing API credentials")); err != nil {
		t.Fatalf("failed to block: %v", err)
	}
	if sm.Current() != taskfsm.StateBlocked {
		t.Fatalf("expected state blocked, got %s", sm.Current())
	}

	// Blocked -> Running (Resume)
	if err := sm.Transition(taskfsm.EventResume, taskfsm.WithReason("Credentials provided")); err != nil {
		t.Fatalf("failed to resume: %v", err)
	}
	if sm.Current() != taskfsm.StateRunning {
		t.Fatalf("expected state running, got %s", sm.Current())
	}

	// Running -> Blocked -> Review (Unfixable blocker escalated to sprint review)
	_ = sm.Transition(taskfsm.EventBlock)
	if err := sm.Transition(taskfsm.EventSubmitReview, taskfsm.WithReason("Unfixable blocker escalated")); err != nil {
		t.Fatalf("failed to escalate blocked to review: %v", err)
	}
	if sm.Current() != taskfsm.StateReview {
		t.Fatalf("expected state review, got %s", sm.Current())
	}
}

func TestStateMachine_ReviewReturnToDoing(t *testing.T) {
	// Rule §9: Task returned from review to doing with gap guidance
	sm := taskfsm.New(taskfsm.StateReview)

	if err := sm.Transition(taskfsm.EventReturnToDoing, taskfsm.WithReason("Missing documentation"), taskfsm.WithActor("SM")); err != nil {
		t.Fatalf("failed to return to doing: %v", err)
	}
	if sm.Current() != taskfsm.StateRunning {
		t.Fatalf("expected state running after return to doing, got %s", sm.Current())
	}
}

func TestStateMachine_Cancellation(t *testing.T) {
	states := []taskfsm.State{
		taskfsm.StatePlanning,
		taskfsm.StateApproved,
		taskfsm.StateRunning,
		taskfsm.StateBlocked,
		taskfsm.StateReview,
	}

	for _, s := range states {
		sm := taskfsm.New(s)
		if !sm.Can(taskfsm.EventCancel) {
			t.Errorf("state %s should allow cancellation", s)
		}
		if err := sm.Transition(taskfsm.EventCancel, taskfsm.WithReason("User cancelled")); err != nil {
			t.Errorf("failed to cancel from %s: %v", s, err)
		}
		if sm.Current() != taskfsm.StateCanceled {
			t.Errorf("expected state canceled, got %s", sm.Current())
		}
	}
}

func TestStateMachine_Guards(t *testing.T) {
	sm := taskfsm.New(taskfsm.StateRunning)

	allowReview := false
	sm.AddGuard(taskfsm.StateRunning, taskfsm.StateReview, func(from, to taskfsm.State, event taskfsm.Event, reason, actor string) error {
		if !allowReview {
			return errors.New("DoD criteria not yet fulfilled")
		}
		return nil
	})

	if sm.Can(taskfsm.EventSubmitReview) {
		t.Fatalf("guard should prevent EventSubmitReview when allowReview is false")
	}

	err := sm.Transition(taskfsm.EventSubmitReview)
	if err == nil {
		t.Fatalf("expected error from guard")
	}

	allowReview = true
	if !sm.Can(taskfsm.EventSubmitReview) {
		t.Fatalf("guard should allow EventSubmitReview when allowReview is true")
	}
	if err := sm.Transition(taskfsm.EventSubmitReview); err != nil {
		t.Fatalf("unexpected transition error: %v", err)
	}
}

func TestStateMachine_Listeners(t *testing.T) {
	sm := taskfsm.New(taskfsm.StatePlanning)

	var transitionCalled bool
	var stateRunningCalled bool

	sm.OnTransition(func(record taskfsm.TransitionRecord) {
		transitionCalled = true
	})

	sm.OnState(taskfsm.StateRunning, func(record taskfsm.TransitionRecord) {
		stateRunningCalled = true
	})

	_ = sm.Transition(taskfsm.EventApprove)
	if !transitionCalled {
		t.Fatalf("expected transition listener to be called")
	}
	if stateRunningCalled {
		t.Fatalf("stateRunning listener should not be called yet")
	}

	_ = sm.Transition(taskfsm.EventStart)
	if !stateRunningCalled {
		t.Fatalf("expected stateRunning listener to be called")
	}
}

func TestStateMachine_Force(t *testing.T) {
	sm := taskfsm.New(taskfsm.StatePlanning)

	if err := sm.Force(taskfsm.StateReview, taskfsm.WithReason("Manual override for testing"), taskfsm.WithActor("Admin")); err != nil {
		t.Fatalf("force transition failed: %v", err)
	}
	if sm.Current() != taskfsm.StateReview {
		t.Fatalf("expected state review, got %s", sm.Current())
	}
	hist := sm.History()
	if len(hist) != 1 || !hist[0].Forced {
		t.Fatalf("expected forced history record, got %+v", hist)
	}
}

func TestStateMachine_JSONSerialization(t *testing.T) {
	sm := taskfsm.New(taskfsm.StatePlanning)
	_ = sm.Transition(taskfsm.EventApprove, taskfsm.WithReason("Sprint approved"), taskfsm.WithActor("PO"))

	bytes, err := json.Marshal(sm)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	sm2 := taskfsm.New(taskfsm.StatePlanning)
	if err := json.Unmarshal(bytes, sm2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if sm2.Current() != taskfsm.StateApproved {
		t.Fatalf("expected unmarshaled state approved, got %s", sm2.Current())
	}
	if len(sm2.History()) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(sm2.History()))
	}
}

func TestStateMachine_Concurrency(t *testing.T) {
	sm := taskfsm.New(taskfsm.StateRunning)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.Current()
			_ = sm.Can(taskfsm.EventSubmitReview)
			_ = sm.History()
			_ = sm.ValidEvents()
			_ = sm.ValidTransitions()
		}()
	}
	wg.Wait()
}

func BenchmarkStateMachine_Can(b *testing.B) {
	sm := taskfsm.New(taskfsm.StateRunning)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sm.Can(taskfsm.EventSubmitReview)
	}
}

func BenchmarkStateMachine_Transition(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sm := taskfsm.New(taskfsm.StatePlanning)
		_ = sm.Transition(taskfsm.EventApprove)
		_ = sm.Transition(taskfsm.EventStart)
	}
}
