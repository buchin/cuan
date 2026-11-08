package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

var historicalDocumentationCheck = regexp.MustCompile(`^Markdown-only diff verified: [1-9][0-9]* readable nonempty UTF-8 file\(s\), [0-9]+ deletion\(s\)\. No code/config changes detected against [^\n]+\. Task-specific content and delivery criteria still require evidence\.$`)

// This is a bounded evidence-recovery hint, never proof that the historical
// verifier ran or that the deliverable is correct. No old failed run is replayed.
func savedEvidenceRecoveryCandidate(d *task.TaskDetailResponse) bool {
	if d == nil || (d.Status != task.TaskStatusBlocked && d.Status != task.TaskStatusReview) || d.LatestAttempt == nil || d.LatestAttempt.ID == "" || d.LatestAttempt.RunnerID == "" || len(task.AcceptanceGaps(&d.Task)) > 0 {
		return false
	}
	a := d.LatestAttempt
	if operationalSourceCheckMismatch(d) || legacyAccountPublicationMismatch(d) || completedAccountOutcomeCandidate(d) {
		return true
	}
	if a.Status != task.AttemptStatusFailed || a.ExitCode == nil || *a.ExitCode != 0 || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || !strings.HasPrefix(a.ErrorMessage, "Kepatuhan auto_deploy:") || (!historicalDocumentationCheck.MatchString(d.TestSummary) && !incorrectPublicationAuditRecovery(d)) {
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

func savedEvidenceRecoveryRequestID(d *task.TaskDetailResponse) string {
	kind := "documentation-evidence-recovery:v1:"
	if !historicalDocumentationCheck.MatchString(d.TestSummary) {
		kind = "scope-evidence-recovery:v1:"
	}
	return delegationMessageID(kind + d.ID + ":" + task.LaunchInstructionsHash(d.Instructions))
}

// A source verifier proves its bound developer checkpoint, not the customer's
// account outcome. Resolve only that native source; never borrow an unrelated
// historical success or turn the verifier into a new implementation worker.
func (s *Server) accountOutcomeSourceVerification(ctx context.Context, d *task.TaskDetailResponse) *task.VerificationIntent {
	if d == nil || !task.SupportsAccountOutcomeInspection(&d.Task) || (d.Status != task.TaskStatusReview && d.Status != task.TaskStatusBlocked) || len(task.AcceptanceGaps(&d.Task)) > 0 || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return nil
	}
	source, proof := s.verifiedSupplementalSource(ctx, d)
	if source == nil || proof == nil {
		return nil
	}
	// This is an eligibility view of the immutable failed source, not a task
	// status update. Actual observation still needs current owner authority.
	source.Status = task.TaskStatusBlocked
	if !legacyAccountPublicationMismatch(source) {
		return nil
	}
	return proof
}

func (s *Server) recoverSavedEvidence(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	gate := "already_verified"
	defer func() {
		if ctx.Value(inboxDeliveryDiagnosticKey{}) == true && d != nil && d.LatestAttempt != nil && task.SupportsAccountOutcomeInspection(&d.Task) {
			log.Printf("inbox account recovery checkpoint task=%s attempt=%s gate=%s", d.ID, d.LatestAttempt.ID, gate)
		}
	}()
	// A recognized verifier goes straight to review; do not start a second
	// observation merely because its completed outcome is recovery-eligible.
	if s.accountOutcomeIndependentlyVerified(ctx, d) {
		return false
	}
	gate = "candidate"
	verifiedSource := s.accountOutcomeSourceVerification(ctx, d)
	if !savedEvidenceRecoveryCandidate(d) && verifiedSource == nil {
		return false
	}
	gate = "origin_binding"
	if origin.MessageID != d.SourceMessageID || origin.RoomID != d.SourceRoomID || !strings.HasPrefix(d.Instructions, "Cuan managed delegation v1.\n") || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) {
		return false
	}
	gate = "review_authority"
	if !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		return false
	}
	// Existing authorization/cancellation rules remain authoritative. A historical
	// summary cannot grant a new action or reset the code-correction budget.
	gate = "saved_authorization"
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, task.TaskLaunch{OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, CreatedAt: time.Now().UTC()}, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	gate = "source_intent"
	if proof, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); err != nil || (proof != nil && verifiedSource == nil) || (verifiedSource != nil && (verifiedSource.OwnerID != origin.OwnerID || verifiedSource.RoomID != origin.RoomID || verifiedSource.SourceRequestID != origin.RequestID)) {
		return false
	}
	id := savedEvidenceRecoveryRequestID(d)
	gate = "saved_launch_read"
	previous, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return false
	}
	if previous != nil {
		gate = "existing_" + previous.State
		return previous.State == "queued" || previous.State == "claimed" || previous.State == "started"
	}
	revision := "Scope: read-only repository. Do not push. Do not deploy.\nOne bounded recovery of saved task evidence, not another implementation cycle. The historical task-level verification text is unbound and is only a diagnostic hint; do not claim its checks ran in this attempt. Read the actual existing deliverables and inspect the existing workspace. Do not modify code, configuration, tests, documentation, git index, refs or commits. Do not reproduce prior deployment, migration, payment or other external effects. Independently verify each original acceptance criterion from actual files and reproducible read-only observations. Deliver the existing report as session artifacts under CUAN_TASK_ARTIFACTS_DIR, preserving its source provenance. Report current factual findings with the original criteria unchanged. If a required result or publication is missing, report that precise gap; do not repair it or waive it. The runner must verify unchanged repository state and independent review must assess the actual report. Historical attempt outcomes remain unchanged." + taskRecoveryContext(d)
	if task.SupportsAccountOutcomeInspection(&d.Task) {
		revision += "\nOperational account outcome inspection: the original requested outcome concerns the existing account, authentication or entitlement. Retained source changes do not by themselves establish an account repair or a publication requirement. The prior worker reported completion; independent verification is still required. A historical unrelated source-test failure does not prove an account defect. Independently inspect the existing account and entitlement through the project's authorized read-only commands/API. Never run access-grant, activate, revoke, update or another external mutation to obtain proof. Inspect retained repository changes too; unexpected source edits are out-of-scope findings, not accepted work. Verify every original criterion with fresh observations. Do not prepare or run unrelated source regression tests, invent code repairs, publish or waive missing outcome evidence. Retain any historical local source commit and diff as unrequested diagnostic work, not accepted implementation or approved release scope. Reconcile any recorded publication effects without repeating them; never infer that this branch was deployed or that no prior effects occurred."
	}
	gate = "launch_admission"
	s.startAuthorizedLaunch(ctx, origin, d, id, origin.RequestID, revision)
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil || launch == nil || (launch.State != "queued" && launch.State != "claimed" && launch.State != "started") {
		return false
	}
	gate = "admitted"
	now := time.Now().UTC()
	_ = s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: delegationMessageID(id + ":notice"), RoomID: d.SourceRoomID, AuthorType: "system", AuthorName: "Cuan", Content: "Cuan memeriksa laporan yang sudah tersimpan sebelum menuntaskan review.\n\n[View work](/tasks/" + d.ID + ")", CreatedAt: now, UpdatedAt: now})
	s.publishInbox(d.SourceRoomID)
	return true
}

// An overbroad legacy audit reopened some non-publication work. The later
// process exited successfully but was rejected solely for a deployment outside
// its original scope. Reinspect saved outcomes without changing failed history.
func incorrectPublicationAuditRecovery(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.TestSummary != "No supported project verifier was available; agent report evidence retained." || !executionpolicy.LegacyScopeDoesNotRequestPublication(d.Instructions) || !executionpolicy.FromInstructions(d.Instructions).DenyDeploy {
		return false
	}
	if d.LatestAttempt.ErrorMessage != "Kepatuhan auto_deploy: verified production deployment receipt is missing for this exact task, attempt and repository commit; report text is not deployment evidence" {
		return false
	}
	for _, audit := range d.Activities {
		if audit == nil || audit.TaskID != d.ID || audit.AttemptID == nil || audit.ActivityType != task.ActivityPublicationAcceptanceAudit || audit.Author != "coordinator" || audit.NewStatus != "review_required" || !audit.CreatedAt.Before(d.LatestAttempt.CreatedAt) {
			continue
		}
		for _, source := range d.Attempts {
			if source != nil && source.ID == *audit.AttemptID && source.ID != d.LatestAttempt.ID && source.TaskID == d.ID && source.Status == task.AttemptStatusSucceeded && source.ExitCode != nil && *source.ExitCode == 0 && !source.CancelRequested && source.CanceledAt == nil && source.FinishedAt != nil && !source.FinishedAt.After(audit.CreatedAt) {
				return true
			}
		}
	}
	return false
}

// A legacy code default cannot turn a completed account operation into a source
// repair mandate. Recover only current native prerequisite failures, not tests,
// source changes, cancellation or a worker's generic claim of completion.
func operationalSourceCheckMismatch(d *task.TaskDetailResponse) bool {
	return d != nil && d.Status == task.TaskStatusBlocked && d.ExecutionCategory == task.ExecutionCategoryCode && d.CommitSHA == "" && d.DiffSummary == "" && len(task.AcceptanceGaps(&d.Task)) == 0 && task.IsAccountOutcome(&d.Task) && nativePHPPrerequisiteFailure(d) != "" && task.AttemptChangeEvidence(d).RunnerChangeRecord == nil
}

func completedAccountOutcomeCandidate(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.Status != task.TaskStatusReview || !task.IsAccountOutcome(&d.Task) || len(task.AcceptanceGaps(&d.Task)) > 0 || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return false
	}
	a := d.LatestAttempt
	return a.Status == task.AttemptStatusSucceeded && !a.CancelRequested && a.CanceledAt == nil && a.FinishedAt != nil && a.ExitCode != nil && *a.ExitCode == 0 && a.ErrorMessage == ""
}

// Independent observations require a saved native verification grant, never a
// worker role label or generic test prose. They never establish publication.
func (s *Server) accountOutcomeIndependentlyVerified(ctx context.Context, d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || !task.SupportsAccountOutcomeInspection(&d.Task) || !inboxReviewExecutionSucceeded(d) || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return false
	}
	store, ok := s.taskSvc.Store().(task.AttemptLaunchStore)
	if !ok {
		return false
	}
	launch, err := store.LaunchForAttempt(ctx, d.LatestAttempt.ID)
	if err != nil || launch == nil || launch.TaskID != d.ID || launch.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) {
		return false
	}
	dedicated := launch.RequestID == savedEvidenceRecoveryRequestID(d) && strings.Contains(launch.Revision, "Operational account outcome inspection:")
	if !dedicated && (!task.IsAccountOutcome(&d.Task) || !managerAccountObservationGrant(d, launch)) {
		return false
	}
	origin, authorized, authErr := s.authorizeSavedTaskLaunch(ctx, *launch, d)
	if authErr != nil || !authorized || !s.taskContinuationAfterCancel(ctx, origin, d) {
		return false
	}
	_, unchanged, err := s.inboxReviewRepositoryScope(ctx, d)
	return err == nil && unchanged
}

// Old code-default execution may have repaired incidental project tests and
// then failed publication. Preserve that source without approving or releasing
// it; original account criteria still need a separate read-only observation.
func legacyAccountPublicationMismatch(d *task.TaskDetailResponse) bool {
	if d == nil || d.LatestAttempt == nil || d.Status != task.TaskStatusBlocked || d.ExecutionCategory != task.ExecutionCategoryCode || !task.SupportsAccountOutcomeInspection(&d.Task) || len(task.AcceptanceGaps(&d.Task)) > 0 || inboxReportEvidence(d).Provenance != "current_attempt_report" {
		return false
	}
	a := d.LatestAttempt
	if a.Status != task.AttemptStatusFailed || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || a.ExitCode == nil || *a.ExitCode != 0 || a.ErrorMessage != task.PublicationVerificationUnavailableMessage {
		return false
	}
	checks := task.AttemptVerificationEvidence(d)
	changes := task.AttemptChangeEvidence(d)
	return checks.Author == a.RunnerID && task.PassingProjectCheckSummary(checks.Current) && changes.Provenance == "current_runner_branch_diff" && changes.RunnerChangeRecord != nil && changes.ResultCommit == d.CommitSHA && releaseObjectID.MatchString(d.CommitSHA)
}

func (s *Server) accountOutcomeReviewContext(ctx context.Context, d *task.TaskDetailResponse) map[string]any {
	if d == nil || !task.SupportsAccountOutcomeInspection(&d.Task) {
		return nil
	}
	return map[string]any{"original_scope": "existing_account_data_operation", "independently_observed": s.accountOutcomeIndependentlyVerified(ctx, d), "source_publication_requested": false, "retained_source": "Any historical local source changes remain diagnostic, outside the requested account outcome and not approved for publication. Preserve them; do not release them or repeat account effects to obtain evidence."}
}

// Admission already checked the assessment's then-current evidence digest. Here
// validate that exact immutable grant against its predecessor, not today's
// changed report digest. A new report cannot retroactively authorize itself.
func managerAccountObservationGrant(d *task.TaskDetailResponse, launch *task.TaskLaunch) bool {
	if d == nil || d.LatestAttempt == nil || launch == nil || launch.RoomID != d.SourceRoomID || launch.ProjectID != d.ProjectID || launch.AgentID != d.AssignedAgent {
		return false
	}
	var parent *task.TaskAttempt
	for _, a := range d.Attempts {
		if a == nil || a.ID == d.LatestAttempt.ID || a.TaskID != d.ID || a.FinishedAt == nil || a.FinishedAt.After(launch.CreatedAt) || !a.CreatedAt.Before(d.LatestAttempt.CreatedAt) {
			continue
		}
		if parent == nil || a.CreatedAt.After(parent.CreatedAt) {
			parent = a
		}
	}
	if parent == nil || parent.CancelRequested || parent.CanceledAt != nil {
		return false
	}
	revision := launch.Revision
	if strings.HasPrefix(revision, "Binding approved continuation:\n") {
		_, tail, found := strings.Cut(revision, "\n\nAutomatic correction feedback:\n")
		if !found {
			return false
		}
		revision = tail
	}
	for _, event := range d.Activities {
		if event == nil || event.TaskID != d.ID || event.AttemptID == nil || *event.AttemptID != parent.ID || event.Author != "coordinator" || event.ActivityType != reviewAssessmentActivity || event.CreatedAt.Before(*parent.FinishedAt) || event.CreatedAt.After(launch.CreatedAt) {
			continue
		}
		var saved reviewAssessmentRecord
		if json.Unmarshal([]byte(event.Message), &saved) != nil || saved.RequestID != launch.RequestID || saved.InstructionsHash != launch.InstructionsHash || len(saved.EvidenceDigest) != 64 {
			continue
		}
		a := saved.Assessment
		if a.Decision == "revise" && a.RemainingWork == "verification" && a.RequestedOutcome != "needs_input" && a.ManagerReason != "" && (a.ManagerAction == "continue" || a.ManagerAction == "replan") && revision == verificationReviewRevision(a.Revision) {
			return true
		}
	}
	return false
}
