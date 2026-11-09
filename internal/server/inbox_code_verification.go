package server

import (
	"context"
	"encoding/json"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

// Ready developer output has one next stage: independent inspection. Preserve
// the native source checks/identity instead of asking a model to choose a role.
func (s *Server) inboxCodeVerificationHandoff(ctx context.Context, q db.InboxRequest) (string, bool) {
	d := s.inboxReviewedTask(ctx, q)
	if d == nil || d.ResultKind != "implementation" || !releaseObjectID.MatchString(d.CommitSHA) || !task.ProjectChecksPassed(d) || !inboxReviewExecutionSucceeded(d) || inboxReportEvidence(d).Provenance != "current_attempt_report" || !s.independentCodeReviewMissing(ctx, d) || db.InboxRetryNeedsReconciliation(q) || isCustomerReplyDelivery(q, d) {
		return "", false
	}
	if changed, err := s.inboxReviewOwnerContextChanged(ctx, q); changed || err != nil {
		return "", false
	}
	live, liveErr := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	verification, verificationErr := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if liveErr != nil || verificationErr != nil || live != nil || verification != nil {
		return "", false
	}
	decision := s.requireIndependentCodeReview(ctx, d, inboxReviewEvaluation{Decision: "pass"})
	raw, err := json.Marshal(decision)
	return string(raw), err == nil && decision.Decision == "revise" && decision.RemainingWork == "verification"
}

const inboxIndependentCodeReviewPolicy = "Independent code execution review: v3"

func (s *Server) independentCodeReviewMissing(ctx context.Context, d *task.TaskDetailResponse) bool {
	return d != nil && d.Status != task.TaskStatusDone && d.ExecutionCategory == task.ExecutionCategoryCode &&
		!executionpolicy.FromInstructions(d.Instructions).ReadOnlyRepository && !task.IsAccountOutcome(&d.Task) && !s.accountOutcomeIndependentlyVerified(ctx, d) &&
		s.verificationReviewContext(ctx, d) == nil && s.liveInspectionReviewContext(ctx, d) == nil
}

func (s *Server) requireIndependentCodeReview(ctx context.Context, d *task.TaskDetailResponse, assessment inboxReviewEvaluation) inboxReviewEvaluation {
	// Pending acceptance evidence must not send an implemented result back to
	// its author just to claim independent verification. Exact-source native
	// admission still checks the result; only the verifier can establish gaps.
	accountMissing := d != nil && task.IsAccountOutcome(&d.Task) && !s.accountOutcomeIndependentlyVerified(ctx, d)
	// An unclassified revision must never silently become another writable dev
	// attempt. For a natively tested source, inspect it before deciding a fix.
	unclassified := assessment.Decision == "revise" && assessment.RemainingWork == "" && assessment.SourceReview != "revise" && assessment.RequestedOutcome != "needs_input" && assessment.ManagerAction != "wait" && assessment.ManagerAction != "needs_owner" && d != nil && d.ResultKind == "implementation" && task.ProjectChecksPassed(d)
	if (assessment.Decision != "pass" && !unclassified) || !inboxReviewExecutionSucceeded(d) || (!accountMissing && !s.independentCodeReviewMissing(ctx, d)) {
		return assessment
	}
	if accountMissing {
		return inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification", Finding: "Hasil akses pelanggan belum diperiksa secara independen.", Explanation: "Akses yang dilaporkan masih perlu diperiksa sebelum dinyatakan selesai.", Revision: "Independently observe the existing account and entitlement using authorized read-only project tools. Preserve all prior effects; do not grant, activate, revoke or update access again. Verify every original criterion and report factual evidence. Do not change source, run unrelated implementation tests or publish.", ManagerAction: "continue", ManagerReason: "Independent read-only account outcome evidence is still required."}
	}
	return inboxReviewEvaluation{
		Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification",
		Finding:       "Hasil kode belum diperiksa dalam sesi verifikasi terpisah.",
		Explanation:   "Hasil kode belum diperiksa dalam sesi verifikasi terpisah.",
		Revision:      "Verify the exact implemented commit in a separate read-only session against every original requirement and saved project instruction. Inspect the actual code and run relevant checks independently; worker claims and previous test output alone are not acceptance. Preserve repository, configuration, refs, commits and production. Report observed evidence and any concrete implementation gap without changing scope.",
		ManagerAction: "continue", ManagerReason: "Independent execution evidence for the implemented commit is still missing.",
	}
}
