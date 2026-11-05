package server

import "time"

// Storage and execution must agree on the same request lifetime. Never renew a
// claimed request by deriving a later local deadline from a new process start.
func inboxRequestExecutionDeadline(now, durable time.Time, maximum time.Duration) time.Time {
	deadline := now.Add(maximum)
	if !durable.IsZero() && durable.Before(deadline) {
		return durable
	}
	return deadline
}
