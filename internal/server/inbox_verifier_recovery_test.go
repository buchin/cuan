package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func testNativeVerifierRepair(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse) {
	t.Helper()
	var launchErr error
	ctx := context.WithValue(context.Background(), inboxReviewLaunchErrorKey{}, &launchErr)
	original := d.LatestAttempt.ID
	previous, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, original)
	if err != nil || previous == nil {
		t.Fatal("missing original inspection", err)
	}
	proof := *previous
	proof.VerifierRecoveryAttemptID = original
	proof.ObservedAt = time.Now().UTC()
	if err := task.ValidateInspectionVerifierRecovery(ctx, s.taskSvc.Store(), &d.Task, &proof); err != nil {
		t.Fatal("known native verifier mismatch not recoverable", err)
	}
	for _, bad := range []string{"target", "source", "observation", "revalidation", "preparation"} {
		altered := proof
		switch bad {
		case "target":
			altered.TargetCommit = proof.SourceCommit
		case "source":
			altered.SourceCommit = proof.TargetCommit
		case "observation":
			altered.ObservedAt = previous.ObservedAt
		case "revalidation":
			altered.PreviousAttemptID = "other"
		case "preparation":
			altered.PreparationLaunchID = "other"
		}
		if task.ValidateInspectionVerifierRecovery(ctx, s.taskSvc.Store(), &d.Task, &altered) == nil {
			t.Fatal("verifier repair expanded scope", bad)
		}
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "verifier-repair-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	first := s.recoverLiveResultInspection(ctx, d, origin)
	second := s.recoverLiveResultInspection(ctx, d, origin)
	if !first || !second {
		t.Fatalf("bounded recovery not deduplicated first=%v second=%v direct=%v launchErr=%v", first, second, s.recoverInspectionVerifier(ctx, d, origin), launchErr)
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, proof.RequestID(d.ID)))
	if err != nil || launch.State != "queued" {
		t.Fatal("missing durable recovery", err)
	}
	bound, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || bound == nil || bound.VerifierRecoveryAttemptID != original || bound.PredecessorAttemptID() != original || bound.TargetCommit != previous.TargetCommit {
		t.Fatal("invalid repaired verifier scope", err)
	}
	if err := task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &d.Task, bound); err != nil {
		t.Fatal("repair invalid at claim", err)
	}
	source, _ := s.taskSvc.GetAttempt(ctx, original)
	if source.Status != task.AttemptStatusFailed || source.ErrorMessage != "Live result inspection project checks did not pass" {
		t.Fatal("repair rewrote failed history")
	}
}
