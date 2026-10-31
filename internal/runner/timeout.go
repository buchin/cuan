package runner

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultBaseTimeout    = 15 * time.Minute // Attempt 1 (Base Run)
	DefaultRequeueTimeout = 20 * time.Minute // Attempt 2 (First Requeue / Gap Fixable)
	DefaultMaxCapTimeout  = 25 * time.Minute // Attempt 3+ (Final Requeue / Max Cap)
	DefaultGracePeriod    = 20 * time.Second // Termination tolerance grace period before SIGKILL
	MaxAttempts           = 3                // Maximum attempts allowed per task before review/blocked
)

// CalculateAdaptiveTimeout returns progressive adaptive timeout based on attempt count:
// - Attempt 1 (Base): 15m
// - Attempt 2 (First Requeue): 20m
// - Attempt 3+ (Final Requeue / Hard Cap): 25m
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
	maxAtt := MaxAttempts
	if v := os.Getenv("CUAN_TASK_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxAtt = n
		}
	}
	return attemptNumber < maxAtt
}
