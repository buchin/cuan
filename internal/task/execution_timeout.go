package task

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// Ceilings are safety bounds. The runner stops an attempt on progress
	// silence, so a productive agent is not cut off by a fixed guess while a
	// genuinely stuck one still ends after DefaultProgressIdleTimeout.
	DefaultBaseTimeout                = 60 * time.Minute // Attempt 1 (Base Run)
	DefaultRequeueTimeout             = 75 * time.Minute // Attempt 2 (First Requeue / Gap Fixable)
	DefaultMaxCapTimeout              = 90 * time.Minute // Attempt 3+ (Final Requeue / Max Cap)
	DefaultInvestigationTimeout       = 25 * time.Minute // Text-only audit/investigation ceiling
	DefaultVisualInvestigationTimeout = 45 * time.Minute // Visual/multimodal inspection & browser diagnostics
	DefaultGracePeriod                = 30 * time.Second // Termination tolerance grace period before SIGKILL
	MaxAttempts                       = 3                // Maximum attempts allowed per task before review/blocked
)

// DefaultProgressIdleTimeout is how long a task may run without producing new
// output before it is treated as stalled. Wall-clock duration answers "how long
// has this run?" but not "is it still working?", and production evidence showed
// almost every attempt killed by the wall clock was still producing real output
// at the moment it died. Stall detection answers the question that matters.
const DefaultProgressIdleTimeout = 15 * time.Minute

// ProgressIdleTimeout is the longest silence tolerated before execution stops.
func ProgressIdleTimeout() time.Duration {
	if v := os.Getenv("CUAN_TASK_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultProgressIdleTimeout
}

func isVisualInvestigation(titleAndPrompt ...string) bool {
	for _, text := range titleAndPrompt {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "visual") ||
			strings.Contains(lower, "screenshot") ||
			strings.Contains(lower, "tangkapan layar") ||
			strings.Contains(lower, "browser") ||
			strings.Contains(lower, "komparasi") ||
			strings.Contains(lower, "perbandingan") ||
			strings.Contains(lower, ".jpg") ||
			strings.Contains(lower, ".png") ||
			strings.Contains(lower, ".webp") ||
			strings.Contains(lower, "attachment") ||
			strings.Contains(lower, "lampiran") {
			return true
		}
	}
	return false
}

// CalculateAdaptiveTimeout returns progressive adaptive timeout based on attempt count:
// - Attempt 1 (Base): 25m
// - Attempt 2 (First Requeue): 35m
// - Attempt 3+ (Final Requeue / Hard Cap): 45m
func CalculateAdaptiveTimeout(attemptNumber int) time.Duration {
	if v := os.Getenv("CUAN_TASK_TIMEOUT_OVERRIDE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}

	baseTimeout := DefaultBaseTimeout
	if v := os.Getenv("CUAN_TASK_TIMEOUT_BASE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			baseTimeout = d
		}
	}

	requeueTimeout := DefaultRequeueTimeout
	if v := os.Getenv("CUAN_TASK_TIMEOUT_REQUEUE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			requeueTimeout = d
		}
	}

	maxCapTimeout := DefaultMaxCapTimeout
	if v := os.Getenv("CUAN_TASK_TIMEOUT_MAX"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			maxCapTimeout = d
		}
	}

	switch {
	case attemptNumber <= 1:
		return baseTimeout
	case attemptNumber == 2:
		return requeueTimeout
	default:
		return maxCapTimeout
	}
}

// CalculateTaskTimeout returns the hard ceiling for one attempt. The ceiling is
// a safety bound, not the routine stop condition: the runner stops an attempt
// when it goes silent for ProgressIdleTimeout, which is the signal that actually
// distinguishes a working agent from a stuck one. Ceilings therefore leave room
// for productive work that legitimately outlasts a fixed guess.
func CalculateTaskTimeout(lane string, attemptNumber int, titleAndPrompt ...string) time.Duration {
	if v := os.Getenv("CUAN_TASK_TIMEOUT_OVERRIDE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}

	normLane := strings.ToLower(strings.TrimSpace(lane))

	// Explicit readonly lane gets the investigation ceiling, or the longer visual
	// ceiling when the request needs rendered evidence.
	if normLane == "readonly" || normLane == "read_only" {
		if v := os.Getenv("CUAN_TASK_TIMEOUT_INVESTIGATION"); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				return d
			}
		}
		if isVisualInvestigation(titleAndPrompt...) {
			return DefaultVisualInvestigationTimeout
		}
		return DefaultInvestigationTimeout
	}

	// For non-code lane (unspecified / ops), check if title explicitly indicates pure investigation/RCA
	if normLane != "code" {
		titleText := ""
		if len(titleAndPrompt) > 0 {
			titleText = strings.ToLower(titleAndPrompt[0])
		}
		if strings.Contains(titleText, "investigasi") ||
			strings.Contains(titleText, "investigation") ||
			strings.Contains(titleText, "rca") ||
			strings.Contains(titleText, "root cause") {
			if v := os.Getenv("CUAN_TASK_TIMEOUT_INVESTIGATION"); v != "" {
				if d, err := time.ParseDuration(v); err == nil && d > 0 {
					return d
				}
			}
			if isVisualInvestigation(titleAndPrompt...) {
				return DefaultVisualInvestigationTimeout
			}
			return DefaultInvestigationTimeout
		}
	}

	return CalculateAdaptiveTimeout(attemptNumber)
}

// GetGracePeriod returns the grace period duration for two-stage termination (SIGTERM -> SIGKILL).
func GetGracePeriod() time.Duration {
	if v := os.Getenv("CUAN_TASK_GRACE_PERIOD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return DefaultGracePeriod
}

// CanRequeue returns true if the attempt count has not exceeded the max attempts threshold (3).
func CanRequeue(attemptNumber int) bool {
	return attemptNumber < MaxAttemptsFor()
}

// MaxAttemptsFor is the single source of truth for how many business attempts a
// task may consume. Every requeue/retry path must consult it; previously three
// separate checks disagreed and retries could exceed the limit indefinitely.
func MaxAttemptsFor() int {
	if v := os.Getenv("CUAN_TASK_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return MaxAttempts
}

// AttemptDeadline is immutable once claimed; legacy attempts use the same policy.
func AttemptDeadline(t *Task, a *TaskAttempt) time.Time {
	if a.ExecutionDeadline != nil {
		return *a.ExecutionDeadline
	}
	start := a.CreatedAt
	if a.StartedAt != nil {
		start = *a.StartedAt
	}
	if start.IsZero() {
		start = time.Now().UTC()
	}
	return start.Add(CalculateTaskTimeout(t.ExecutionCategory, a.AttemptNumber, t.Title, t.Instructions))
}
