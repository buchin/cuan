package server

import (
	"github.com/buchin/cuan/internal/task"
	"testing"
)

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
