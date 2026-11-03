package runner

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDoDEvaluator_AllCriteriaPass_MarkDone(t *testing.T) {
	evaluator := NewDoDEvaluator()
	// Mock commands: clean working tree, test pass, build 0 error
	evaluator.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(""), nil // Clean
		}
		if name == "go" || strings.HasSuffix(name, "test.sh") || name == "npm" {
			return []byte("PASS"), nil
		}
		return []byte(""), nil
	}

	req := DoDValidationRequest{
		TaskID:        "task_100",
		AttemptID:     "att_100",
		AttemptNumber: 1,
		WorkingDir:    t.TempDir(),
		TaskTitle:     "Refactor cache layer",
		CompleteNote:  "Fixture: isolated command runner output is checked by evaluator.", Summary: "Refactored cache to be modular",
	}

	res, err := evaluator.EvaluateDoD(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected passed=true, got false with gaps: %v", res.Gaps)
	}
	if res.Action != "mark_done" {
		t.Fatalf("expected action mark_done, got %s", res.Action)
	}
	if !res.CleanWorkingTree || !res.TargetedTestsPass || !res.BuildZeroError {
		t.Fatalf("expected all checks true: %+v", res)
	}
	if !strings.Contains(res.Details, "LULUS (100% Kriteria Terpenuhi)") {
		t.Fatalf("expected LULUS in details, got: %s", res.Details)
	}
}

func TestDoDEvaluator_DirtyWorkingTree_AutoRequeue(t *testing.T) {
	evaluator := NewDoDEvaluator()
	evaluator.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(" M internal/runner/runner.go\n?? untracked.txt\n"), nil // Dirty!
		}
		return []byte("PASS"), nil
	}

	req := DoDValidationRequest{
		TaskID:        "task_200",
		AttemptID:     "att_200",
		AttemptNumber: 1, // First attempt
		WorkingDir:    t.TempDir(),
		TaskTitle:     "Update runner",
		CompleteNote:  "Fixture: isolated command runner output is checked by evaluator.", Summary: "Working on runner",
	}

	res, err := evaluator.EvaluateDoD(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatal("expected passed=false for dirty working tree")
	}
	if res.CleanWorkingTree {
		t.Fatal("expected CleanWorkingTree=false")
	}
	if res.Action != "auto_requeue" {
		t.Fatalf("expected action auto_requeue, got %s", res.Action)
	}
	if res.NextAttemptNumber != 2 {
		t.Fatalf("expected NextAttemptNumber=2, got %d", res.NextAttemptNumber)
	}
	if res.NextTimeout != 35*time.Minute {
		t.Fatalf("expected NextTimeout 35m for attempt 2, got %v", res.NextTimeout)
	}
	if !strings.Contains(res.RequeueInstructions, "Working tree tidak bersih") {
		t.Fatalf("expected dirty working tree in instructions, got: %s", res.RequeueInstructions)
	}
}

func TestDoDEvaluator_TestFailure_AutoRequeueWithAdaptiveTimeout(t *testing.T) {
	evaluator := NewDoDEvaluator()
	evaluator.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(""), nil // Clean
		}
		if name == "go" && len(args) > 0 && args[0] == "test" {
			return []byte("FAIL: TestBroken"), errors.New("exit code 1")
		}
		return []byte(""), nil
	}

	// Attempt 2 failing -> Should requeue to Attempt 3 with 25m max timeout
	req := DoDValidationRequest{
		TaskID:        "task_300",
		AttemptID:     "att_300",
		AttemptNumber: 2,
		WorkingDir:    t.TempDir(),
		TaskTitle:     "Fix database race",
		CompleteNote:  "Fixture: isolated command runner output is checked by evaluator.", Summary: "Attempted fix",
	}

	res, err := evaluator.EvaluateDoD(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatal("expected passed=false")
	}
	if res.TargetedTestsPass {
		t.Fatal("expected TargetedTestsPass=false")
	}
	if res.Action != "auto_requeue" {
		t.Fatalf("expected action auto_requeue, got %s", res.Action)
	}
	if res.NextAttemptNumber != 3 {
		t.Fatalf("expected NextAttemptNumber=3, got %d", res.NextAttemptNumber)
	}
	if res.NextTimeout != 45*time.Minute {
		t.Fatalf("expected NextTimeout 45m for attempt 3, got %v", res.NextTimeout)
	}
}

func TestDoDEvaluator_MaxAttemptsReached_MoveToReview(t *testing.T) {
	evaluator := NewDoDEvaluator()
	evaluator.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return []byte(""), nil
		}
		if name == "go" && len(args) > 0 && args[0] == "test" {
			return []byte("FAIL: TestStillFailing"), errors.New("exit code 1")
		}
		return []byte(""), nil
	}

	// Attempt 3 failing -> Max cap reached! Should NOT requeue infinitely. Action must be "review".
	req := DoDValidationRequest{
		TaskID:        "task_400",
		AttemptID:     "att_400",
		AttemptNumber: 3,
		WorkingDir:    t.TempDir(),
		TaskTitle:     "Fix stubborn bug",
		CompleteNote:  "Fixture: isolated command runner output is checked by evaluator.", Summary: "Failed 3rd attempt",
	}

	res, err := evaluator.EvaluateDoD(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatal("expected passed=false")
	}
	if res.Action != "review" {
		t.Fatalf("expected action 'review' when max attempts reached, got %s", res.Action)
	}
	if !strings.Contains(res.Details, "Batas Maksimal Attempt Tercapai") {
		t.Fatalf("expected max attempts message in details, got: %s", res.Details)
	}
}

func TestDoDEvaluator_AttachToEventBus(t *testing.T) {
	bus := NewEventBus()
	evaluator := NewDoDEvaluator()
	evaluator.CustomCommandRunner = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		return []byte(""), nil // all pass
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var capturedRes *DoDValidationResult

	evaluator.AttachToEventBus(bus, func(ctx context.Context, evt AttemptSucceededEvent, res *DoDValidationResult) {
		capturedRes = res
		wg.Done()
	})

	bus.Publish(context.Background(), AttemptSucceededEvent{
		TaskID:        "task_bus_test",
		AttemptID:     "att_bus_1",
		AttemptNumber: 1,
		WorkingDir:    t.TempDir(),
		Status:        "succeeded",
		CompleteNote:  "Fixture: isolated command runner output is checked by evaluator.", Summary: "Event triggered DoD evaluation",
	})

	wg.Wait()

	if capturedRes == nil {
		t.Fatal("expected captured DoD validation result from event bus listener")
	}
	if !capturedRes.Passed {
		t.Fatalf("expected captured DoD validation to pass, got gaps: %v", capturedRes.Gaps)
	}
	if capturedRes.Action != "mark_done" {
		t.Fatalf("expected action mark_done, got %s", capturedRes.Action)
	}
}
