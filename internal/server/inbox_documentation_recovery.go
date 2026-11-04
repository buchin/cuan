package server

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

var historicalDocumentationCheck = regexp.MustCompile(`^Markdown-only diff verified: [1-9][0-9]* readable nonempty UTF-8 file\(s\), [0-9]+ deletion\(s\)\. No code/config changes detected against [^\n]+\. Task-specific content and delivery criteria still require evidence\.$`)

// This is a bounded evidence-recovery hint, never proof that the historical
// verifier ran or that the deliverable is correct. No old failed run is replayed.
func documentationRecoveryCandidate(d *task.TaskDetailResponse) bool {
	if d == nil || d.Status != task.TaskStatusBlocked || d.LatestAttempt == nil || d.LatestAttempt.ID == "" || d.LatestAttempt.RunnerID == "" || len(task.AcceptanceGaps(&d.Task)) > 0 {
		return false
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || !strings.HasPrefix(a.ErrorMessage, "Kepatuhan auto_deploy:") || !historicalDocumentationCheck.MatchString(d.TestSummary) {
		return false
	}
	var latest *task.TaskActivity
	for _, event := range d.Activities {
		if event != nil && event.TaskID == d.ID && event.AttemptID != nil && *event.AttemptID == a.ID && event.ActivityType == "agent_report" && (latest == nil || event.CreatedAt.After(latest.CreatedAt) || (event.CreatedAt.Equal(latest.CreatedAt) && event.ID > latest.ID)) {
			latest = event
		}
	}
	return latest != nil && (latest.NewStatus == task.TaskStatusReview || latest.NewStatus == task.TaskStatusDone)
}

func documentationRecoveryRequestID(d *task.TaskDetailResponse) string {
	return delegationMessageID("documentation-evidence-recovery:v1:" + d.ID + ":" + task.LaunchInstructionsHash(d.Instructions))
}

func (s *Server) recoverDocumentationEvidence(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if !documentationRecoveryCandidate(d) || origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return false
	}
	// Existing authorization/cancellation rules remain authoritative. A historical
	// summary cannot grant a new action or reset the code-correction budget.
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	if proof, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err != nil || proof != nil {
		return false
	}
	id := documentationRecoveryRequestID(d)
	previous, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return false
	}
	if previous != nil {
		return previous.State == "queued" || previous.State == "claimed" || previous.State == "started"
	}
	revision := "Scope: read-only repository. Do not push. Do not deploy.\nOne bounded recovery of saved documentation evidence, not another implementation cycle. The historical task-level Markdown verification text is unbound and is only a diagnostic hint; do not claim its checks ran in this attempt. Read the actual existing deliverables and inspect the existing workspace. Do not modify code, configuration, tests, documentation, git index, refs or commits. Do not reproduce prior deployment, migration, payment or other external effects. Independently verify each original acceptance criterion from actual files and reproducible read-only observations. Deliver the existing report as session artifacts under CUAN_TASK_ARTIFACTS_DIR, preserving its source provenance. Report current factual findings with the original criteria unchanged. If a required result or publication is missing, report that precise gap; do not repair it or waive it. The runner must verify unchanged repository state and independent review must assess the actual report. The original failed attempt remains failed." + taskRecoveryContext(d)
	s.startAuthorizedLaunch(ctx, origin, d, id, origin.RequestID, revision)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
		return false
	}
	now := time.Now().UTC()
	_ = s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: delegationMessageID(id + ":notice"), RoomID: d.SourceRoomID, AuthorType: "system", AuthorName: "Cuan", Content: "Cuan memeriksa laporan yang sudah tersimpan sebelum menuntaskan review.\n\n[View work](/tasks/" + d.ID + ")", CreatedAt: now, UpdatedAt: now})
	s.publishInbox(d.SourceRoomID)
	return true
}
