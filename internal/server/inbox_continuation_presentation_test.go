package server

import (
	"github.com/buchin/cuan/internal/task"
	"strings"
	"testing"
)

func TestContinuationPresentationDoesNotChangeExecutionScope(t *testing.T) {
	technical := "First reconcile the registered project's existing release; retain exact commit receipt and do not deploy again."
	original := &inboxApprovalPlan{Title: "Langkah berikutnya", Description: technical, ProjectID: "project", Continuation: &inboxContinuationPlan{TaskID: "task", AttemptID: "attempt", Revision: technical}}
	shown := presentContinuationPlan(original)
	if shown != nil {
		t.Fatal("internal continuation still asks owner approval")
	}
	if original.Description != technical || original.Title != "Langkah berikutnya" || original.Continuation.Revision != technical {
		t.Fatal("presentation changed saved execution scope")
	}

	body := "Hasil kerja belum terverifikasi, termasuk status rilisnya.\n\nBatas perbaikan otomatis sudah tercapai; hasil sementara dan bukti tetap tersimpan.\n\n[View work](/tasks/task)"
	visible := presentContinuationText(body, original)
	if strings.Contains(visible, "status rilis") || strings.Contains(visible, "Batas perbaikan otomatis") || !strings.Contains(visible, "belum bisa dipastikan sudah diterapkan") || !strings.Contains(visible, "/tasks/task") {
		t.Fatal("legacy technical footer leaked or result lost", visible)
	}
	if presentContinuationText(body, nil) != body {
		t.Fatal("unrelated history rewritten")
	}
	normal := &inboxApprovalPlan{Title: "Original task", Description: "Exact original acceptance scope"}
	if presentContinuationPlan(normal) != normal {
		t.Fatal("ordinary task proposal altered")
	}
}

func TestContinuationSeparatesPlainRecommendationFromBoundRevision(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{ID: "task", ProjectID: "project"}, LatestAttempt: &task.TaskAttempt{ID: "attempt"}}
	decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Revision: "Run exact native verification of the existing deployed source. Do not deploy."}
	plan := parseInboxApprovalPlan(continuationPlan(d, "review-request", decision))
	if plan == nil || plan.Title != "Lanjutkan pemeriksaan" || plan.Description == decision.Revision || plan.Continuation.Revision != decision.Revision || !continuationDescriptionMatches(plan, decision) {
		t.Fatal("plain recommendation lost authoritative scope")
	}
	plan.Continuation.RemainingWork = "implementation"
	if continuationDescriptionMatches(plan, decision) {
		t.Fatal("changed visible promise accepted")
	}
}
