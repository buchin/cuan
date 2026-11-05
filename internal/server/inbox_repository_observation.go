package server

import (
	"encoding/hex"
	"encoding/json"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// Failed scope observations explain what survived. They never substitute for a
// successful execution, a final report, or accepted repository verification.
func failedRepositoryObservation(d *task.TaskDetailResponse, readOnly bool) map[string]any {
	if !readOnly || d == nil || d.LatestAttempt == nil {
		return nil
	}
	a := d.LatestAttempt
	if (a.Status != task.AttemptStatusFailed && a.Status != task.AttemptStatusInterrupted) || a.CancelRequested || a.CanceledAt != nil || a.RunnerID == "" {
		return nil
	}
	for _, event := range d.Activities {
		if event == nil || event.ID != "repository-observation:"+a.ID || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != a.ID || event.ActivityType != "repository_scope_observed" || event.Author != a.RunnerID || event.NewStatus != a.Status {
			continue
		}
		var observation runner.RepositoryVerification
		if json.Unmarshal([]byte(event.Message), &observation) != nil {
			continue
		}
		validHash := func(value string) bool {
			_, err := hex.DecodeString(value)
			return len(value) == 64 && err == nil
		}
		if !validHash(observation.Baseline) || (observation.Final != "" && !validHash(observation.Final)) {
			continue
		}
		state := "unavailable"
		if observation.Final != "" {
			state = "changed"
			if observation.Unchanged() {
				state = "unchanged"
			}
		}
		return map[string]any{"attempt_id": a.ID, "execution_status": a.Status, "repository_state_at_observation": state, "observed_at": event.CreatedAt}
	}
	return nil
}
