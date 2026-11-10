package server

import (
	"context"
	"encoding/json"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// Timing is an observation, never acceptance evidence. Read only a bounded tail
// of the current attempt and expose typed runner fields, not terminal output.
func (s *Server) inboxReviewTiming(ctx context.Context, d *task.TaskDetailResponse) map[string]any {
	if d == nil || d.LatestAttempt == nil || s.taskSvc == nil {
		return nil
	}
	logs, err := s.taskSvc.GetRecentAttemptLogs(ctx, d.LatestAttempt.ID, 32)
	result := reviewAttemptTiming(d.LatestAttempt, logs)
	if err != nil {
		result["agent_stage_state"] = "unavailable"
	}
	return result
}

func reviewAttemptTiming(a *task.TaskAttempt, logs []*task.LogRecord) map[string]any {
	result := map[string]any{"attempt_id": a.ID, "provenance": "current_runner_observation", "agent_stage_state": "unknown"}
	var total int64
	if a.StartedAt != nil && a.FinishedAt != nil && !a.FinishedAt.Before(*a.StartedAt) {
		total = a.FinishedAt.Sub(*a.StartedAt).Milliseconds()
		result["attempt_elapsed_ms"] = total
	}
	for _, log := range logs {
		if log == nil || log.AttemptID != a.ID || log.Stream != "system" || a.StartedAt == nil || log.LoggedAt.Before(*a.StartedAt) || a.FinishedAt != nil && log.LoggedAt.After(*a.FinishedAt) {
			continue
		}
		var record struct {
			Event     string `json:"event"`
			Stage     string `json:"stage"`
			State     string `json:"state"`
			Outcome   string `json:"outcome"`
			ElapsedMS *int64 `json:"elapsed_ms"`
		}
		if len(log.Line) > 1024 || json.Unmarshal([]byte(log.Line), &record) != nil || record.Event != "execution_stage" || record.Stage != "agent" || record.State != "finished" || record.ElapsedMS == nil || *record.ElapsedMS < 0 {
			continue
		}
		// Native elapsed time cannot exceed the entire attempt. Allow timestamp
		// serialization precision, but never accept worker-reported timing text.
		if total > 0 && *record.ElapsedMS > total+int64(time.Second/time.Millisecond) {
			continue
		}
		switch record.Outcome {
		case "returned", "timeout", "cancelled", "error", "interrupted", "capacity", "failed":
		default:
			continue
		}
		result["agent_stage_state"] = "observed"
		result["agent_elapsed_ms"] = *record.ElapsedMS
		result["agent_outcome"] = record.Outcome
	}
	return result
}
