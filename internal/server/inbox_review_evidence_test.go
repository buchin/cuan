package server

import (
	"encoding/json"
	"github.com/buchin/cuan/internal/task"
	"testing"
)

func TestPublicationPolicyOnlyUpgradeRetainsUnchangedAssessment(t *testing.T) {
	item := &task.TaskDetailResponse{Task: task.Task{ID: "task", ExecutionCategory: task.ExecutionCategoryCode, Instructions: "Original approved source and publication", Summary: "Implemented result", CompleteNote: "Original observed test output"}, LatestAttempt: &task.TaskAttempt{ID: "verifier"}}
	previous := inboxReviewEvidenceDigestForPublicationPolicy(item, previousPublicationReviewPolicy)
	if previous == inboxReviewEvidenceDigest(item) {
		t.Fatal("fixture did not exercise a policy-only digest change")
	}
	record := reviewAssessmentRecord{RequestID: "saved-review", InstructionsHash: task.LaunchInstructionsHash(item.Instructions), EvidenceDigest: previous, Assessment: inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Original evidence gap", Explanation: "Check the existing result", Revision: "Inspect without repeating effects"}}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	item.Activities = []*task.TaskActivity{{TaskID: item.ID, AttemptID: &item.LatestAttempt.ID, ActivityType: reviewAssessmentActivity, Author: "coordinator", Message: string(raw)}}
	snapshot := previousPublicationReviewPolicy + "\nReview evidence digest: " + previous
	if inboxReviewEvidenceChanged(item, snapshot) {
		t.Fatal("unchanged work was sent for review because only the policy label changed")
	}
	if got, found := reviewAssessment(item, "saved-review"); !found || got != record.Assessment {
		t.Fatal("unchanged saved assessment was discarded")
	}
	item.CompleteNote += "; new independently observed failure"
	if !inboxReviewEvidenceChanged(item, snapshot) {
		t.Fatal("actual changed evidence borrowed a historical assessment")
	}
	if _, found := reviewAssessment(item, "saved-review"); found {
		t.Fatal("old assessment survived actual evidence change")
	}
}

func TestPublicationPolicyRefreshTargetsOnlyAffectedV6Work(t *testing.T) {
	for _, tc := range []struct {
		policy            string
		affected, refresh bool
	}{
		{inboxPublicationReviewPolicy, false, false},
		{inboxPublicationReviewPolicy, true, false},
		{previousPublicationReviewPolicy, false, false},
		{previousPublicationReviewPolicy, true, true},
		{"Production publication review policy: v5", false, true},
	} {
		if publicationReviewNeedsRefresh(tc.policy, tc.affected) != tc.refresh {
			t.Fatal("incorrect refresh eligibility", tc)
		}
	}
}

func TestInboxReviewEvidenceRevisionIgnoresAdministrativeChanges(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "Guide delivered", CompleteNote: "Read delivered file: 406 words", DoDChecklist: []task.DoDItem{{Text: "Six stages", Completed: true, Evidence: "Six numbered headings"}}}}
	first := inboxReviewEvidenceDigest(d)
	snapshot := "Review evidence digest: " + first
	d.Status = task.TaskStatusBlocked
	if inboxReviewEvidenceChanged(d, snapshot) {
		t.Fatal("status change must not cause review loop")
	}
	d.CompleteNote += "; attachment downloaded independently"
	if !inboxReviewEvidenceChanged(d, snapshot) {
		t.Fatal("new verification must trigger review")
	}
	if inboxReviewEvidenceChanged(d, "Review evidence digest: "+inboxReviewEvidenceDigest(d)) {
		t.Fatal("same evidence must deduplicate")
	}
	d.DoDChecklist[0].Evidence = "Independent count of six headings"
	if inboxReviewEvidenceDigest(d) == first {
		t.Fatal("checklist evidence absent from binding")
	}
}
func TestInboxReviewEvidenceRevisionDetectsLegacyReportReplacement(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{CompleteNote: "Original native report restored"}}
	if !inboxReviewEvidenceChanged(d, "Verification evidence:\nGeneric evaluator claim") {
		t.Fatal("restored native report was ignored")
	}
	if inboxReviewEvidenceChanged(d, "Verification evidence:\nOriginal native report restored") {
		t.Fatal("unchanged legacy evidence replayed")
	}
}
