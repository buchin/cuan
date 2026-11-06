package task

import "strings"

// FinalAgentReportState identifies the latest durable report for one execution.
// It is shared by harness lifecycle adapters and the runner, and is deliberately
// independent of process exit, publication, verification and task acceptance.
func (d *TaskDetailResponse) FinalAgentReportState(attemptID string) string {
	if d == nil || attemptID == "" || strings.TrimSpace(d.Summary) == "" || strings.TrimSpace(d.CompleteNote) == "" {
		return ""
	}
	for i := len(d.Activities) - 1; i >= 0; i-- {
		a := d.Activities[i]
		if a == nil || a.AttemptID == nil || *a.AttemptID != attemptID || a.ActivityType != "agent_report" {
			continue
		}
		switch a.NewStatus {
		case TaskStatusReview, TaskStatusDone, TaskStatusBlocked:
			return a.NewStatus
		default:
			return ""
		}
	}
	return ""
}
