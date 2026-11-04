package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxReviewDoesNotAttributePreviousReportToCurrentAttempt(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	old, current := "older-attempt", "latest-attempt"
	for _, status := range []string{task.AttemptStatusFailed, task.AttemptStatusInterrupted, task.AttemptStatusCanceled, task.AttemptStatusSucceeded} {
		t.Run(status, func(t *testing.T) {
			d := &task.TaskDetailResponse{Task: task.Task{ID: "report-provenance", Summary: "Previous implementation complete", CompleteNote: "Previous checks passed"}, LatestAttempt: &task.TaskAttempt{ID: current, Status: status}, Activities: []*task.TaskActivity{{ID: "old-report", TaskID: "report-provenance", AttemptID: &old, ActivityType: "agent_report", NewStatus: "review", Message: "OLDER_REPORT_WITH_SAVED_WORK", CreatedAt: time.Now()}}}
			var payload map[string]any
			if err := json.Unmarshal([]byte(s.inboxReviewData(context.Background(), d)), &payload); err != nil {
				t.Fatal(err)
			}
			if payload["reported_summary"] != "" || payload["reported_verification"] != "" || payload["reported_attempt_id"] != "" || payload["report_provenance"] != "no_current_attempt_report" {
				t.Fatal("historical claims became current attempt evidence", payload)
			}
			if payload["historical_summary"] != d.Summary || payload["historical_verification"] != d.CompleteNote || payload["historical_report"] != "OLDER_REPORT_WITH_SAVED_WORK" || payload["historical_report_attempt_id"] != old {
				t.Fatal("saved work history was lost", payload)
			}
			if d.Summary != "Previous implementation complete" || len(d.Activities) != 1 {
				t.Fatal("review rewrote retained evidence")
			}
		})
	}
}

func TestInboxReviewReportFallbackRequiresMatchingAttemptAndDeterministicOrder(t *testing.T) {
	current, old := "latest", "old"
	now := time.Now()
	d := &task.TaskDetailResponse{Task: task.Task{ID: "task"}, LatestAttempt: &task.TaskAttempt{ID: current, Status: task.AttemptStatusSucceeded}, Activities: []*task.TaskActivity{
		{ID: "report-z", TaskID: "task", AttemptID: &current, ActivityType: "agent_report", NewStatus: "review", Message: "CURRENT_FINAL", CreatedAt: now},
		{ID: "old", TaskID: "task", AttemptID: &old, ActivityType: "agent_report", Message: "OLD_REPORT", CreatedAt: now.Add(time.Hour)},
		{ID: "report-a", TaskID: "task", AttemptID: &current, ActivityType: "agent_report", NewStatus: "review", Message: "EARLIER_TIE", CreatedAt: now},
		{ID: "other-task", TaskID: "different-task", AttemptID: &current, ActivityType: "agent_report", NewStatus: "review", Message: "OTHER_TASK", CreatedAt: now.Add(time.Hour)},
		nil,
	}}
	for i := 0; i < len(d.Activities); i++ {
		got := inboxReportEvidence(d)
		if got.Summary != "CURRENT_FINAL" || got.Verification != "CURRENT_FINAL" || got.AttemptID != current || got.Provenance != "current_attempt_report" || got.HistoricalReport != "OLD_REPORT" {
			t.Fatalf("activity array order affected review evidence: %+v", got)
		}
		d.Activities = append(d.Activities[1:], d.Activities[0])
	}
}

func TestInboxReviewProgressDoesNotInheritFinalVerification(t *testing.T) {
	id := "progress-attempt"
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "Older final result", CompleteNote: "Older verification"}, LatestAttempt: &task.TaskAttempt{ID: id, Status: task.AttemptStatusFailed}, Activities: []*task.TaskActivity{{AttemptID: &id, ActivityType: "agent_report", NewStatus: "progress", Message: "Inspected the saved workspace"}}}
	got := inboxReportEvidence(d)
	if got.Summary != "Inspected the saved workspace" || got.Verification != "" || got.HistoricalVerification != "Older verification" || got.AttemptID != id {
		t.Fatalf("progress inherited earlier completion claims: %+v", got)
	}
}

func TestInboxReviewSingleLegacySuccessRemainsExplicitlyUnbound(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "Legacy result", CompleteNote: "Legacy checks"}, LatestAttempt: &task.TaskAttempt{ID: "legacy", Status: task.AttemptStatusSucceeded}}
	got := inboxReportEvidence(d)
	if got.Summary != d.Summary || got.Verification != d.CompleteNote || got.AttemptID != "" || got.Provenance != "legacy_task_snapshot_unbound" {
		t.Fatalf("legacy snapshot was discarded or falsely attributed: %+v", got)
	}
}

func TestInboxReviewReconcilesMisattributedHistoricalReportOnlyOnce(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "completed", task.AttemptStatusFailed)
	ctx := context.Background()
	older := "previous-execution"
	if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "previous-report", TaskID: d.ID, AttemptID: &older, ActivityType: "agent_report", NewStatus: "review", Message: "Prior result, not current verification", CreatedAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.queueInboxTaskReview(ctx, d)
	key := "task-review:" + d.LatestAttempt.ID + ":report-provenance:" + inboxReviewEvidenceDigest(d)
	requests := reviewRequests(t, s, old.RoomID)
	var fresh *db.InboxRequest
	for i := range requests {
		if requests[i].RequestID == delegationMessageID(key) {
			fresh = &requests[i]
		}
	}
	if len(requests) != 2 || fresh == nil {
		t.Fatalf("missing bounded provenance review: %+v", requests)
	}
	if bound := s.inboxReviewedTask(ctx, *fresh); bound == nil || bound.ID != d.ID {
		t.Fatal("provenance review lost canonical task binding")
	}
	message, err := s.projectStudioRepo.GetMessageByID(ctx, fresh.MessageID)
	if err != nil || !strings.Contains(message.Content, inboxReportProvenancePolicy) {
		t.Fatal("fresh snapshot missing provenance policy", err)
	}
	beforeHistory := strings.Split(message.Content, "Historical task state")[0]
	if strings.Contains(beforeHistory, d.Summary) || !strings.Contains(beforeHistory, "no_current_attempt_report") {
		t.Fatal("queued review input mislabeled old task fields as current evidence")
	}
	fresh.State = "completed"
	if err := s.projectStudioRepo.(db.InboxRequestRepository).FinishInboxRequest(ctx, *fresh, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		s.queueInboxTaskReview(ctx, d)
	}
	if rows := reviewRequests(t, s, old.RoomID); len(rows) != 2 {
		t.Fatal("unchanged provenance review looped", len(rows))
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(after.Attempts) != 1 || after.LatestAttempt.ID != d.LatestAttempt.ID {
		t.Fatal("provenance repair replayed work", err)
	}
}
