package server

import (
	"context"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

const inboxIndependentCodeReviewPolicy = "Independent code execution review: v1"

func (s *Server) independentCodeReviewMissing(ctx context.Context, d *task.TaskDetailResponse) bool {
	return d != nil && d.Status != task.TaskStatusDone && d.ExecutionCategory == task.ExecutionCategoryCode &&
		!executionpolicy.FromInstructions(d.Instructions).ReadOnlyRepository &&
		s.verificationReviewContext(ctx, d) == nil && s.liveInspectionReviewContext(ctx, d) == nil
}

func (s *Server) requireIndependentCodeReview(ctx context.Context, d *task.TaskDetailResponse, assessment inboxReviewEvaluation) inboxReviewEvaluation {
	if assessment.Decision != "pass" || !inboxReviewExecutionSucceeded(d) || len(task.AcceptanceGaps(&d.Task)) > 0 || !s.independentCodeReviewMissing(ctx, d) {
		return assessment
	}
	return inboxReviewEvaluation{
		Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "verification",
		Finding:       "Hasil kode belum diperiksa dalam sesi verifikasi terpisah.",
		Explanation:   "Hasil kode belum diperiksa dalam sesi verifikasi terpisah.",
		Revision:      "Verify the exact implemented commit in a separate read-only session against every original requirement and saved project instruction. Inspect the actual code and run relevant checks independently; worker claims and previous test output alone are not acceptance. Preserve repository, configuration, refs, commits and production. Report observed evidence and any concrete implementation gap without changing scope.",
		ManagerAction: "continue", ManagerReason: "Independent execution evidence for the implemented commit is still missing.",
	}
}
