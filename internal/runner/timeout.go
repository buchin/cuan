package runner

import (
	"time"

	"github.com/buchin/agentrunner"
)

const (
	DefaultBaseTimeout    = agentrunner.DefaultBaseTimeout
	DefaultRequeueTimeout = agentrunner.DefaultRequeueTimeout
	DefaultMaxCapTimeout  = agentrunner.DefaultMaxCapTimeout
	DefaultGracePeriod    = agentrunner.DefaultGracePeriod
	MaxAttempts           = agentrunner.MaxAttempts
)

// CalculateAdaptiveTimeout returns progressive adaptive timeout based on attempt count
// powered by the standalone github.com/buchin/agentrunner library.
func CalculateAdaptiveTimeout(attemptNumber int) time.Duration {
	return agentrunner.CalculateAdaptiveTimeout(attemptNumber)
}

// GetGracePeriod returns the grace period duration for two-stage termination (SIGTERM -> SIGKILL).
func GetGracePeriod() time.Duration {
	return agentrunner.GetGracePeriod()
}

// CanRequeue returns true if the attempt count has not exceeded the max attempts threshold (3).
func CanRequeue(attemptNumber int) bool {
	return agentrunner.CanRequeue(attemptNumber)
}
