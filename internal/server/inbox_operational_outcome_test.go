package server

import (
	"context"
	"github.com/buchin/cuan/internal/task"
	"strings"
	"testing"
)

func TestOperationalOutcomeDoesNotAcceptSafeStopAsCompletion(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{ExecutionCategory: task.ExecutionCategoryOps, Instructions: inboxReconciliationExecutionContract}}
	for _, outcome := range []string{"", "needs_input", "unfinished", "completed"} {
		decision := requireOperationalOutcome(d, inboxReviewEvaluation{Decision: "pass", RequestedOutcome: outcome, Explanation: "Need the exact account and product."})
		if (decision.Decision == "pass") != (outcome == "completed") {
			t.Fatal(outcome, decision)
		}
	}
	d.ExecutionCategory = task.ExecutionCategoryReadOnly
	if requireOperationalOutcome(d, inboxReviewEvaluation{Decision: "pass"}).Decision != "pass" {
		t.Fatal("ordinary audit changed")
	}
}
func TestOperationalAcceptanceAuditRechecksEvidenceWithoutExecution(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	d.Instructions += "\n" + inboxReconciliationExecutionContract
	d.ExecutionCategory = task.ExecutionCategoryOps
	d.Status = task.TaskStatusDone
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if _, err := s.saveReviewAssessment(ctx, d, q, inboxReviewEvaluation{Decision: "pass", Explanation: "Stopped safely with unmatched records."}); err != nil {
		t.Fatal(err)
	}
	s.reconcileOperationalAcceptances(ctx)
	s.reconcileOperationalAcceptances(ctx)
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status != task.TaskStatusReview || len(after.Attempts) != 1 {
		t.Fatal("audit restarted execution or failed to restore review", err)
	}
	s.queueInboxTaskReview(ctx, after)
	reviews := reviewRequests(t, s, q.RoomID)
	if len(reviews) != 2 {
		t.Fatal("missing bounded policy review", len(reviews))
	}
	latest := reviews[1]
	if s.inboxReviewedTask(ctx, latest) == nil {
		t.Fatal("audit review not bound to original task")
	}
	output := s.applyInboxReviewEvaluation(ctx, latest, `{"decision":"blocked","requested_outcome":"needs_input","finding":"No unambiguous record.","remaining_work":"external","explanation":"Which account email and product belong to this transfer?","revision":""}`)
	after, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if after.Status != task.TaskStatusBlocked || len(after.Attempts) != 1 || !strings.Contains(output, "Which account") {
		t.Fatal("blocked outcome lost or operation relaunched")
	}
	if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, latest.RequestID)); err == nil {
		t.Fatal("review created business execution")
	}
}
