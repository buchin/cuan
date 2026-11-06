package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestReviewAssessmentPreservesRejectedGapAtCorrectionLimit(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	for _, id := range []string{"earlier-one", "earlier-two"} {
		if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	raw := `{"decision":"revise","finding":"Bukti screenshot aplikasi nyata belum tersedia.","explanation":"Saya akan melanjutkan pekerjaan secara otomatis.","revision":"Render the changed application at390px and desktop; inspect screenshots and report actual findings rather than an HTML mockup."}`
	out := s.applyInboxReviewEvaluation(ctx, q, raw)
	if !strings.Contains(out, "Bukti screenshot aplikasi nyata belum tersedia.") || strings.Contains(out, "Batas perbaikan otomatis") || strings.Contains(out, "akan melanjutkan") {
		t.Fatal("limit lost concrete gap or promised execution", out)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	assessment, ok := reviewAssessment(after, q.RequestID)
	if !ok || !strings.Contains(assessment.Revision, "rather than an HTML mockup") || assessment.Decision != "revise" {
		t.Fatal("rejected assessment discarded", assessment)
	}
	if checkpoint := taskRecoveryContext(after); !strings.Contains(checkpoint, "Remaining review gap") || !strings.Contains(checkpoint, "rather than an HTML mockup") {
		t.Fatal("next execution lost the saved review gap")
	}
	if len(after.Attempts) != 3 || after.Status != task.TaskStatusBlocked {
		t.Fatal("receipt reset correction budget or accepted work")
	}
	if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); err == nil {
		t.Fatal("exhausted budget created execution")
	}
	// A retry of this exact review uses its durable assessment, not a new outcome.
	s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"Everything passed.","explanation":"Accepted","revision":""}`)
	repeated, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	count := 0
	for _, a := range repeated.Activities {
		if a.ActivityType == reviewAssessmentActivity {
			count++
		}
	}
	if count != 1 || repeated.Status != task.TaskStatusBlocked {
		t.Fatal("retry changed assessment or duplicated receipt")
	}
	repeated.TestSummary = "Different current verification"
	if _, ok := reviewAssessment(repeated, q.RequestID); ok {
		t.Fatal("old assessment survived changed evidence")
	}
}

type assessmentFailureStore struct{ task.Store }

func (s assessmentFailureStore) CreateActivity(ctx context.Context, a *task.TaskActivity) error {
	if a.ActivityType == reviewAssessmentActivity {
		return errors.New("temporary assessment storage failure")
	}
	return s.Store.CreateActivity(ctx, a)
}
func TestReviewAssessmentStorageFailureDefersWithoutChangingTask(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	base := s.taskSvc.Store()
	s.taskSvc = task.NewService(assessmentFailureStore{Store: base}, s.projectSvc)
	retry := false
	ctx := context.WithValue(context.Background(), inboxHandoffPersistenceKey{}, &retry)
	out := s.applyInboxReviewEvaluation(ctx, q, `{"decision":"revise","finding":"Evidence is missing.","explanation":"I will continue automatically.","revision":"Inspect original evidence."}`)
	if !retry || strings.Contains(out, "I will continue") || !strings.Contains(out, "belum tersimpan") {
		t.Fatal("failed persistence produced final progress promise", out)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if after.Status != d.Status || len(after.Attempts) != 1 {
		t.Fatal("unsaved assessment changed task")
	}
	if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); err == nil {
		t.Fatal("unsaved assessment launched execution")
	}
	s.taskSvc = task.NewService(base, s.projectSvc)
}

func TestLegacyAssessmentRecoveryIsBoundedAndNeverExecutesWork(t *testing.T) {
	s, d, _ := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	// This latest successful attempt has its own report; only the assessment was lost.
	id := d.LatestAttempt.ID
	if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "current-final", TaskID: d.ID, AttemptID: &id, ActivityType: "agent_report", NewStatus: task.TaskStatusDone, Message: "Actual report", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	// Make the old review current under the pre-assessment policy.
	repo := s.projectStudioRepo
	key := "task-review:" + id
	old, _ := repo.GetMessageByID(ctx, delegationMessageID(key+":input"))
	old.Content += "\nReview evidence digest: " + inboxReviewEvidenceDigest(d) + "\n" + inboxReportProvenancePolicy
	if err := repo.UpdateMessage(ctx, old); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		s.queueInboxTaskReview(ctx, d)
	}
	reviews := reviewRequests(t, s, d.SourceRoomID)
	expected := delegationMessageID(key + ":assessment:" + inboxReviewEvidenceDigest(d))
	count := 0
	for _, q := range reviews {
		if q.RequestID == expected {
			count++
			if q.State != "queued" {
				t.Fatal("assessment executed original provider")
			}
			if s.inboxReviewedTask(ctx, q) == nil {
				t.Fatal("assessment review not bound to exact task")
			}
		}
	}
	if count != 1 {
		t.Fatal("assessment repair not queued exactly once", count)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(after.Attempts) != 1 {
		t.Fatal("assessment-only repair created execution attempt")
	}
}

func TestRetiredTaskWithSuccessfulAttemptDoesNotRunNewOrStaleReview(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	row, _ := s.taskSvc.Store().GetTask(ctx, d.ID)
	row.Status = "cancelled"
	if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	before := len(reviewRequests(t, s, d.SourceRoomID))
	s.queueInboxTaskReview(ctx, current)
	if after := len(reviewRequests(t, s, d.SourceRoomID)); after != before {
		t.Fatal("retired task created another review")
	}
	if snapshot, ok := s.inboxTaskReviewPrompt(ctx, q); !ok || snapshot != "" {
		t.Fatal("stale review would call provider for retired task")
	}
	outcome := s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"All passed.","explanation":"Accepted","revision":""}`)
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if after.Status != "cancelled" || !strings.Contains(outcome, "dibatalkan") || len(after.Attempts) != 1 {
		t.Fatal("stale pass reopened retired scope")
	}
	for _, a := range after.Activities {
		if a.ActivityType == reviewAssessmentActivity {
			t.Fatal("retired scope stored a new active assessment")
		}
	}
}
