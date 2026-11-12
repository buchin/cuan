package task

import (
	"testing"
	"time"
)

func timeAt(minute int) time.Time {
	return time.Date(2026, 11, 12, 0, minute, 0, 0, time.UTC)
}

func TestClassifyAttemptFailure(t *testing.T) {
	cases := map[string]AttemptFailureClass{
		"Kepatuhan auto_deploy: production publication could not be independently verified": FailureGate,
		"Kepatuhan auto_deploy: verified production deployment receipt is required":         FailureGate,
		"Registered PHPUnit target or runtime is unavailable":                               FailurePrerequisite,
		"locked frontend dependency preparation failed":                                     FailurePrerequisite,
		"Project verification failed":                                                       FailureQuality,
		"Live result inspection project checks did not pass":                                 FailureQuality,
		"":                                                                                  FailureUnknown,
		"something entirely new":                                                            FailureUnknown,
	}
	for message, want := range cases {
		if got := ClassifyAttemptFailure(message); got != want {
			t.Fatalf("ClassifyAttemptFailure(%q) = %q, want %q", message, got, want)
		}
	}
}

// Retrying a gate defect or a missing prerequisite reproduces the same failure,
// which is what produced 138 over-limit attempts in production.
func TestRetryWorthyStopsUnattainableCauses(t *testing.T) {
	if RetryWorthy("Kepatuhan auto_deploy: production publication could not be independently verified") {
		t.Fatal("gate defect must not be retried automatically")
	}
	if RetryWorthy("Registered PHPUnit target or runtime is unavailable") {
		t.Fatal("missing prerequisite must not be retried automatically")
	}
	if !RetryWorthy("Project verification failed") {
		t.Fatal("quality failure should remain retryable")
	}
}

func TestRepeatedFailureDetectsIdenticalReasons(t *testing.T) {
	first := &TaskAttempt{ErrorMessage: "Project verification failed", CreatedAt: timeAt(1)}
	second := &TaskAttempt{ErrorMessage: "  Project   verification failed ", CreatedAt: timeAt(2)}
	if !RepeatedFailure(second, first) {
		t.Fatal("identical normalised reasons must be detected")
	}
	other := &TaskAttempt{ErrorMessage: "Live result inspection project checks did not pass", CreatedAt: timeAt(3)}
	if RepeatedFailure(other, second) {
		t.Fatal("different reasons must not be treated as repeated")
	}
}

func TestBusinessAttemptsIgnoresInfrastructureNoise(t *testing.T) {
	exit137 := 137
	attempts := []*TaskAttempt{
		{ID: "a1", Status: AttemptStatusFailed, ErrorMessage: "Project verification failed", CreatedAt: timeAt(1)},
		{ID: "a2", Status: AttemptStatusInterrupted, CreatedAt: timeAt(2)},
		{ID: "a3", Status: AttemptStatusFailed, ExitCode: &exit137, ErrorMessage: "runner killed", CreatedAt: timeAt(3)},
		{ID: "a4", Status: AttemptStatusFailed, ErrorMessage: "tests failed", CreatedAt: timeAt(4)},
	}
	count, latest, previous := businessAttempts(attempts)
	if count != 2 {
		t.Fatalf("business attempts = %d, want 2", count)
	}
	if latest == nil || latest.ID != "a4" {
		t.Fatalf("latest = %v, want a4", latest)
	}
	if previous == nil || previous.ID != "a1" {
		t.Fatalf("previous = %v, want a1", previous)
	}
}

func TestMaxAttemptsSharedSource(t *testing.T) {
	t.Setenv("CUAN_TASK_MAX_ATTEMPTS", "")
	if MaxAttemptsFor() != MaxAttempts {
		t.Fatalf("default budget = %d, want %d", MaxAttemptsFor(), MaxAttempts)
	}
	t.Setenv("CUAN_TASK_MAX_ATTEMPTS", "5")
	if MaxAttemptsFor() != 5 {
		t.Fatal("env override must be honoured")
	}
	if !CanRequeue(4) || CanRequeue(5) {
		t.Fatal("CanRequeue must follow the shared budget")
	}
}
