package server

import (
	"encoding/json"
	"strings"

	"github.com/buchin/cuan/internal/task"
)

type inboxReportedEvidence struct {
	Summary, Verification                     string
	HistoricalSummary, HistoricalVerification string
	HistoricalReport, HistoricalAttemptID     string
	AttemptID, Provenance                     string
}

// Task-level summary/evidence survive retries. Their presence does not bind them
// to a new attempt: only that attempt's own report can do so. History remains
// available for workspace recovery without masquerading as current verification.
func inboxReportEvidence(d *task.TaskDetailResponse) inboxReportedEvidence {
	var out inboxReportedEvidence
	if d == nil || d.LatestAttempt == nil {
		return out
	}
	var latest, previous *task.TaskActivity
	historical := len(d.Attempts) > 1
	for _, a := range d.Activities {
		if a == nil || a.ActivityType != "agent_report" || a.AttemptID == nil || (a.TaskID != "" && a.TaskID != d.ID) {
			continue
		}
		if *a.AttemptID != d.LatestAttempt.ID {
			historical = true
			if previous == nil || a.CreatedAt.After(previous.CreatedAt) || (a.CreatedAt.Equal(previous.CreatedAt) && a.ID > previous.ID) {
				previous = a
			}
			continue
		}
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID) {
			latest = a
		}
	}
	if previous != nil {
		out.HistoricalReport = previous.Message
		out.HistoricalAttemptID = *previous.AttemptID
	}
	if latest != nil {
		out.AttemptID = d.LatestAttempt.ID
		out.Provenance = "current_attempt_report"
		if latest.NewStatus == task.TaskStatusReview || latest.NewStatus == task.TaskStatusDone || latest.NewStatus == task.TaskStatusBlocked {
			out.Summary = d.Summary
			out.Verification = d.CompleteNote
			if strings.TrimSpace(out.Summary) == "" {
				out.Summary = latest.Message
			}
			if strings.TrimSpace(out.Verification) == "" {
				out.Verification = latest.Message
			}
		} else {
			out.Summary = latest.Message
			out.HistoricalSummary = d.Summary
			out.HistoricalVerification = d.CompleteNote
		}
		return out
	}
	if d.LatestAttempt.Status == task.AttemptStatusSucceeded {
		for _, a := range d.Activities {
			if a != nil && a.TaskID == d.ID && a.AttemptID != nil && *a.AttemptID == d.LatestAttempt.ID && a.ActivityType == task.ActivityRunnerFinalization && a.Author == d.LatestAttempt.RunnerID {
				var report task.FinalizationReport
				if json.Unmarshal([]byte(a.Message), &report) == nil && report.ManifestHash != "" {
					out.AttemptID = d.LatestAttempt.ID
					out.Provenance = "current_runner_finalization"
					out.Summary = report.Summary
					out.Verification = report.Verification
					out.HistoricalSummary = d.Summary
					out.HistoricalVerification = d.CompleteNote
					return out
				}
			}
		}
	}
	// Older integrations can complete a single attempt through the runner without
	// an agent-report event. Keep those snapshots explicitly unbound, never assign
	// the latest attempt ID or substitute a report from another attempt.
	if d.LatestAttempt.Status == task.AttemptStatusSucceeded && !historical {
		out.Summary = d.Summary
		out.Verification = d.CompleteNote
		out.Provenance = "legacy_task_snapshot_unbound"
		return out
	}
	out.Provenance = "no_current_attempt_report"
	out.HistoricalSummary = d.Summary
	out.HistoricalVerification = d.CompleteNote
	return out
}

const inboxReportProvenancePolicy = "Report provenance policy: v1"

func inboxReviewNeedsProvenanceRepair(d *task.TaskDetailResponse) bool {
	evidence := inboxReportEvidence(d)
	return evidence.Provenance == "no_current_attempt_report" && evidence.HistoricalAttemptID != ""
}
