package runner

import (
	"github.com/buchin/cuan/internal/task"
	"time"
)

const (
	DefaultBaseTimeout          = task.DefaultBaseTimeout
	DefaultRequeueTimeout       = task.DefaultRequeueTimeout
	DefaultMaxCapTimeout        = task.DefaultMaxCapTimeout
	DefaultInvestigationTimeout = task.DefaultInvestigationTimeout
	DefaultGracePeriod          = task.DefaultGracePeriod
	MaxAttempts                 = task.MaxAttempts
)

func CalculateAdaptiveTimeout(n int) time.Duration { return task.CalculateAdaptiveTimeout(n) }
func CalculateTaskTimeout(lane string, n int, text ...string) time.Duration {
	return task.CalculateTaskTimeout(lane, n, text...)
}
func GetGracePeriod() time.Duration { return task.GetGracePeriod() }
func CanRequeue(n int) bool         { return task.CanRequeue(n) }
