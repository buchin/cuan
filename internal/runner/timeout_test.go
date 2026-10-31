package runner

import (
	"os"
	"testing"
	"time"
)

func TestCalculateAdaptiveTimeout_ProgressiveScale(t *testing.T) {
	// Clean env overrides
	os.Unsetenv("CUAN_TASK_TIMEOUT_OVERRIDE")
	os.Unsetenv("CUAN_TASK_TIMEOUT_BASE")
	os.Unsetenv("CUAN_TASK_TIMEOUT_REQUEUE")
	os.Unsetenv("CUAN_TASK_TIMEOUT_MAX")

	// Attempt 1: Base Run -> 25 minutes
	if d := CalculateAdaptiveTimeout(1); d != 25*time.Minute {
		t.Fatalf("expected 25m for attempt 1, got %v", d)
	}
	if d := CalculateAdaptiveTimeout(0); d != 25*time.Minute {
		t.Fatalf("expected 25m for attempt 0 (base), got %v", d)
	}

	// Attempt 2: First Requeue -> 35 minutes
	if d := CalculateAdaptiveTimeout(2); d != 35*time.Minute {
		t.Fatalf("expected 35m for attempt 2, got %v", d)
	}

	// Attempt 3: Final Requeue / Max Cap -> 45 minutes
	if d := CalculateAdaptiveTimeout(3); d != 45*time.Minute {
		t.Fatalf("expected 45m for attempt 3, got %v", d)
	}

	// Attempt 4+: Max Cap -> 45 minutes
	if d := CalculateAdaptiveTimeout(4); d != 45*time.Minute {
		t.Fatalf("expected 45m for attempt 4, got %v", d)
	}
}

func TestCalculateAdaptiveTimeout_EnvOverride(t *testing.T) {
	os.Setenv("CUAN_TASK_TIMEOUT_BASE", "10m")
	os.Setenv("CUAN_TASK_TIMEOUT_REQUEUE", "16m")
	os.Setenv("CUAN_TASK_TIMEOUT_MAX", "22m")
	defer func() {
		os.Unsetenv("CUAN_TASK_TIMEOUT_BASE")
		os.Unsetenv("CUAN_TASK_TIMEOUT_REQUEUE")
		os.Unsetenv("CUAN_TASK_TIMEOUT_MAX")
	}()

	if d := CalculateAdaptiveTimeout(1); d != 10*time.Minute {
		t.Fatalf("expected 10m for attempt 1, got %v", d)
	}
	if d := CalculateAdaptiveTimeout(2); d != 16*time.Minute {
		t.Fatalf("expected 16m for attempt 2, got %v", d)
	}
	if d := CalculateAdaptiveTimeout(3); d != 22*time.Minute {
		t.Fatalf("expected 22m for attempt 3, got %v", d)
	}
}

func TestGetGracePeriod_DefaultAndOverride(t *testing.T) {
	os.Unsetenv("CUAN_TASK_GRACE_PERIOD")
	if gp := GetGracePeriod(); gp != 30*time.Second {
		t.Fatalf("expected default grace period 30s, got %v", gp)
	}

	os.Setenv("CUAN_TASK_GRACE_PERIOD", "35s")
	defer os.Unsetenv("CUAN_TASK_GRACE_PERIOD")
	if gp := GetGracePeriod(); gp != 35*time.Second {
		t.Fatalf("expected overridden grace period 35s, got %v", gp)
	}
}

func TestCanRequeue_MaxAttemptsGuard(t *testing.T) {
	os.Unsetenv("CUAN_TASK_MAX_ATTEMPTS")

	if !CanRequeue(1) {
		t.Fatal("expected attempt 1 to be requeueable")
	}
	if !CanRequeue(2) {
		t.Fatal("expected attempt 2 to be requeueable")
	}
	if CanRequeue(3) {
		t.Fatal("expected attempt 3 to reach max cap and NOT be requeueable")
	}
	if CanRequeue(4) {
		t.Fatal("expected attempt 4 to NOT be requeueable")
	}
}
