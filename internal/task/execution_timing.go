package task

import (
	"encoding/json"
	"time"
)

// ExecutionTiming is measured by the runner, not estimated by the worker. Nil
// means unobserved; zero is a recorded duration. It conveys no acceptance credit.
type ExecutionTiming struct {
	AgentElapsedMS         *int64 `json:"agent_elapsed_ms,omitempty"`
	ProjectChecksElapsedMS *int64 `json:"project_checks_elapsed_ms,omitempty"`
}

func (m *ExecutionTiming) ValidFor(start, finish *time.Time) bool {
	if m == nil || start == nil || finish == nil || finish.Before(*start) || (m.AgentElapsedMS == nil && m.ProjectChecksElapsedMS == nil) {
		return false
	}
	remaining := finish.Sub(*start).Milliseconds() + 1000
	for _, value := range []*int64{m.AgentElapsedMS, m.ProjectChecksElapsedMS} {
		if value != nil {
			if *value < 0 || *value > remaining {
				return false
			}
			remaining -= *value
		}
	}
	return true
}

// Timing shares the current runner verification receipt, surviving log retention.
func AttemptExecutionTiming(d *TaskDetailResponse) *ExecutionTiming {
	if d == nil || d.LatestAttempt == nil || d.LatestAttempt.RunnerID == "" {
		return nil
	}
	a := d.LatestAttempt
	for _, e := range d.Activities {
		if e == nil || e.TaskID != d.ID || e.AttemptID == nil || *e.AttemptID != a.ID || e.ID != "verification:"+a.ID || e.Author != a.RunnerID || e.ActivityType != ActivityTypeRunnerVerification {
			continue
		}
		var record RunnerVerificationRecord
		if json.Unmarshal([]byte(e.Message), &record) == nil && record.Timing.ValidFor(a.StartedAt, a.FinishedAt) {
			return record.Timing
		}
	}
	return nil
}
