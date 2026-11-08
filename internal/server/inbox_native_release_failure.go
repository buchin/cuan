package server

import (
	"context"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/task"
)

// Only the current failed native attempt contributes diagnostics. Historical
// developer output must never be mistaken for the cause of a later failure.
func (s *Server) inboxNativeReleaseFailure(ctx context.Context, d *task.TaskDetailResponse) any {
	if s.taskSvc == nil || d == nil || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusFailed {
		return nil
	}
	id := d.LatestAttempt.ID
	intent, err := task.NativeReleaseIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, id)
	if err != nil || intent == nil {
		return nil
	}
	logs, err := s.taskSvc.Store().GetLogs(ctx, id, 0, 1000)
	if err != nil {
		return map[string]any{"attempt_id": id, "diagnostic_state": "unavailable"}
	}
	return map[string]any{"attempt_id": id, "diagnostic_state": "observed", "untrusted_process_output": nativeReleaseFailureOutput(id, logs), "acceptance_evidence": false}
}

func nativeReleaseFailureOutput(attemptID string, logs []*task.LogRecord) string {
	var output string
	afterResult := false
	for _, record := range logs {
		if record == nil || record.AttemptID != attemptID || record.Stream != "system" {
			continue
		}
		// The runner emits its already environment-redacted tail immediately
		// after this marker. Do not ingest arbitrary stdout or provider prose.
		if record.Line == "Native release process ended; reconciling its saved result and production state." {
			output = ""
			afterResult = true
			continue
		}
		if afterResult {
			output = truncateProjectText(agent.MaskSecrets(record.Line, nil), 6000)
			afterResult = false
		}
	}
	return output
}
