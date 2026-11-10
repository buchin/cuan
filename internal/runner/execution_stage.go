package runner

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

type executionStageRecord struct {
	Event     string    `json:"event"`
	Stage     string    `json:"stage"`
	State     string    `json:"state"`
	Outcome   string    `json:"outcome,omitempty"`
	Deadline  time.Time `json:"deadline"`
	ElapsedMS int64     `json:"elapsed_ms"`
}

func agentStageRecord(ctx context.Context, started time.Time, finished bool, result *agent.ExecutionResult, runErr error) executionStageRecord {
	deadline, _ := ctx.Deadline()
	record := executionStageRecord{Event: "execution_stage", Stage: "agent", State: "started", Deadline: deadline}
	if !finished {
		return record
	}
	record.State = "finished"
	record.ElapsedMS = max(0, time.Since(started).Milliseconds())
	record.Outcome = "returned"
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(runErr, context.DeadlineExceeded) || result != nil && result.Status == "TIMEOUT":
		record.Outcome = "timeout"
	case errors.Is(ctx.Err(), context.Canceled) || errors.Is(runErr, context.Canceled) || result != nil && result.Status == "CANCELED":
		record.Outcome = "cancelled"
	case runErr != nil:
		record.Outcome = "error"
	case result != nil && result.Status == "INTERRUPTED":
		record.Outcome = "interrupted"
	case result != nil && result.Status == "QUOTA_LIMITED":
		record.Outcome = "capacity"
	case result != nil && (result.Status == "FAILED" || result.ExitCode != 0):
		record.Outcome = "failed"
	}
	return record
}
func (r *Runner) logAgentStage(ctx context.Context, attempt string, started time.Time, finished bool, result *agent.ExecutionResult, runErr error) executionStageRecord {
	record := agentStageRecord(ctx, started, finished, result, runErr)
	data, _ := json.Marshal(record)
	// Observation may outlive a cancelled agent; it grants no more execution time
	// and never copies provider text, prompts, shell commands or credentials.
	observation, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	r.streamLogLine(observation, attempt, "system", string(data))
	return record
}
