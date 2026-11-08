package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestMissingSourceVerdictQueuesOnlyOneBoundAssessmentRepair(t *testing.T) {
	s, d, origin := stalledReviewFixture(t, "completed")
	missingSourceVerdictRepairContract(t, s, d, origin)
}

func TestMissingSourceVerdictRepairPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, origin, d := phaseLineageSetup(t, database)
		missingSourceVerdictRepairContract(t, s, d, origin)
	})
}

func missingSourceVerdictRepairContract(t *testing.T, s *Server, d *task.TaskDetailResponse, origin db.InboxRequest) {
	t.Helper()
	s.inboxStopping.Store(true)
	verificationReviewReceiptContract(t, s, d, origin)
	ctx := context.Background()
	var err error
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range requests {
		if request.MessageID == d.SourceMessageID {
			origin = request
		}
	}
	if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "verifier-final", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Independent source checks passed; publication remains pending.", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	key := "task-review:" + d.LatestAttempt.ID
	review := origin
	review.RequestID, review.MessageID, review.ReplyID = delegationMessageID(key), delegationMessageID(key+":input"), delegationMessageID(key+":reply")
	review.State, review.CreatedAt, review.Deadline = "queued", time.Now(), time.Now().Add(time.Minute)
	message := &db.ProjectMessage{ID: review.MessageID, RoomID: review.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + d.ID + "]\nReview evidence digest: " + inboxReviewEvidenceDigest(d) + "\n" + strings.Join([]string{inboxReportProvenancePolicy, inboxReviewAssessmentPolicy, inboxVerificationReviewPolicy, inboxVisualReviewPolicy, inboxOperationalReviewPolicy, inboxPublicationReviewPolicy, inboxOwnershipPolicy, inboxReviewProviderPolicy}, "\n"), CreatedAt: review.CreatedAt}
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	if _, _, err := repo.AcceptInboxRequest(ctx, review, message); err != nil {
		t.Fatal(err)
	}
	// The fixture's visual audit can already have generated this review. Model
	// the saved pre-source-policy input, retaining its immutable request identity.
	if err := s.projectStudioRepo.UpdateMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	review.State = "completed"
	if err := repo.FinishInboxRequest(ctx, review, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.saveReviewAssessment(ctx, d, review, inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Explanation: "Publication missing", Revision: "Reconcile existing release"}); err != nil {
		t.Fatal(err)
	}
	if !s.sourceReviewClassificationMissing(ctx, d) {
		t.Fatal("legacy independent source review was not identified")
	}
	if !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		t.Fatal("fixture recovery was not authorized", d.Status, d.Launch)
	}
	before := len(d.Attempts)
	for i := 0; i < 3; i++ {
		s.queueInboxTaskReview(ctx, d)
	}
	expected := delegationMessageID(key + ":source-assessment:" + inboxReviewEvidenceDigest(d))
	count := 0
	for _, q := range reviewRequests(t, s, d.SourceRoomID) {
		if q.RequestID == expected {
			count++
			if q.State != "queued" || s.inboxReviewedTask(ctx, q) == nil {
				t.Fatal("repair was not bound to the current source review")
			}
		}
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if count != 1 || len(after.Attempts) != before || after.Status == task.TaskStatusDone {
		t.Fatal("assessment repair duplicated, executed work or accepted task", count)
	}
	review.RequestID = db.GenerateUUID()
	if _, err := s.saveReviewAssessment(ctx, after, review, inboxReviewEvaluation{Decision: "blocked", SourceReview: "not_checked", RemainingWork: "external", Explanation: "Independent source finding still needs assessment."}); err != nil {
		t.Fatal(err)
	}
	if s.sourceReviewClassificationMissing(ctx, after) {
		t.Fatal("an explicit newer verdict was treated as a missing field")
	}
}

func TestSourceReviewDoesNotAcceptUnpublishedTask(t *testing.T) {
	raw := `{"decision":"revise","source_review":"pass","remaining_work":"publication","explanation":"Source verified; publication remains pending.","revision":"Publish only the independently reviewed source through the authorized release path."}`
	got, err := parseInboxReviewEvaluation(raw)
	if err != nil || got.SourceReview != "pass" || got.Decision != "revise" || got.RemainingWork != "publication" || got.Revision == "" {
		t.Fatalf("source review replaced pending acceptance: %#v %v", got, err)
	}
}

func TestSourceReviewRejectsInvalidOrContradictoryVerdict(t *testing.T) {
	for _, raw := range []string{
		`{"decision":"pass","source_review":"revise","explanation":"Accepted"}`,
		`{"decision":"pass","source_review":"approved","explanation":"Accepted"}`,
		`{"decision":"pass","source_review":true,"explanation":"Accepted"}`,
	} {
		if _, err := parseInboxReviewEvaluation(raw); err == nil {
			t.Fatalf("invalid source verdict accepted: %s", raw)
		}
	}
}
