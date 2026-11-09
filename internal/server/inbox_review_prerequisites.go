package server

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/task"
)

const inboxPrerequisiteReviewPolicy = "Native prerequisite review: v1"

const inboxProjectCheckFailurePolicy = "Native project check correction: v2"

// Keep the failure tail rather than successful setup chatter. Full output stays
// in the native record and is available through the bound task tools.
func nativeCheckDiagnostic(summary string) string {
	const limit = 2400
	if len(summary) <= limit {
		return summary
	}
	lines := strings.SplitN(summary, "\n", 2)
	tail := summary
	if len(tail) > 1800 {
		tail = tail[len(tail)-1800:]
		for len(tail) > 0 && !utf8.RuneStart(tail[0]) {
			tail = tail[1:]
		}
	}
	return truncateProjectText(lines[0], 200) + "\n[Earlier output retained in runner_verification; showing failure tail]\n" + tail
}

// Only a native, current developer completion can establish a source-test gap.
// Inspection failures retain their separate read-only recovery boundaries.
func nativeProjectCheckFailure(d *task.TaskDetailResponse) string {
	if d == nil || d.LatestAttempt == nil || d.ExecutionCategory != task.ExecutionCategoryCode || d.Status == task.TaskStatusDone || d.Status == "cancelled" || d.Status == "canceled" {
		return ""
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ErrorMessage != "Project verification failed" || a.ExitCode == nil || *a.ExitCode != 0 || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || a.RunnerID == "" || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return ""
	}
	for _, e := range d.Activities {
		if e == nil || e.ID != "verification:"+a.ID || e.TaskID != d.ID || e.AttemptID == nil || *e.AttemptID != a.ID || e.Author != a.RunnerID || e.ActivityType != task.ActivityTypeRunnerVerification || e.CreatedAt.Before(*a.FinishedAt) {
			continue
		}
		var record task.RunnerVerificationRecord
		if json.Unmarshal([]byte(e.Message), &record) == nil && strings.HasPrefix(record.Summary, "Project verification:") && !task.PassingProjectCheckSummary(record.Summary) {
			return record.Summary
		}
	}
	return ""
}

// A completed worker report cannot override failed native checks. Restrict this
// classification to this exact runner completion; prose and historical reports
// cannot manufacture either a failure or another continuation.
func nativePHPPrerequisiteFailure(d *task.TaskDetailResponse) string {
	if d == nil || d.LatestAttempt == nil || d.Status == task.TaskStatusDone || d.Status == "cancelled" || d.Status == "canceled" {
		return ""
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || a.RunnerID == "" || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return ""
	}
	diagnostic := a.ErrorMessage
	if diagnostic != "Registered PHPUnit target or runtime is unavailable outside the private workspace" && !strings.HasPrefix(diagnostic, "Registered PHPUnit target ") && !strings.HasPrefix(diagnostic, "Registered PHPUnit runtime ") {
		return ""
	}
	for _, e := range d.Activities {
		if e == nil || e.ID != "verification:"+a.ID || e.TaskID != d.ID || e.AttemptID == nil || *e.AttemptID != a.ID || e.Author != a.RunnerID || e.ActivityType != task.ActivityTypeRunnerVerification || e.CreatedAt.Before(*a.FinishedAt) {
			continue
		}
		var record task.RunnerVerificationRecord
		if json.Unmarshal([]byte(e.Message), &record) == nil && !task.PassingProjectCheckSummary(record.Summary) {
			return diagnostic
		}
	}
	return ""
}

func requireNativePrerequisiteReview(d *task.TaskDetailResponse, decision inboxReviewEvaluation) inboxReviewEvaluation {
	diagnostic := nativePHPPrerequisiteFailure(d)
	if decision.Decision != "pass" || diagnostic == "" {
		return decision
	}
	decision.Decision, decision.RequestedOutcome, decision.SourceReview = "revise", "unfinished", "not_checked"
	decision.RemainingWork = "implementation"
	decision.Finding = "Pemeriksaan hasil belum berjalan karena prasyarat pengujian belum tersedia."
	decision.Explanation = "Hasil investigasi tersimpan. Cuan perlu menyiapkan prasyarat pengujian dan memeriksa hasilnya sebelum menyatakan selesai."
	decision.Revision = "Native PHPUnit prerequisites failed after the worker's final report. Preserve the original findings and scope. Reconcile the current private workspace, registered test targets and saved Composer lock before choosing preparation. The runner now prepares locked Composer dependencies without scripts/plugins for registered PHP checks; inspect its saved setup diagnostic first. If a registered target is absent from this source, report the exact source/configuration gap; never invent replacement tests, waive checks or change project settings merely to pass. Do not repeat the account investigation, login effects, publication or branch resets solely for missing test prerequisites. Run the registered checks with actual results and submit a factual handoff. Original acceptance and independent verification remain required. Native diagnostic: " + truncateProjectText(diagnostic, 1800)
	decision.ManagerAction = "replan"
	decision.ManagerReason = "Replace repeated repository synchronization with scoped preparation of the missing native test prerequisites."
	return decision
}
