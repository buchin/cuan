package server

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func TestPhaseCapacityRecoveryUsesOriginalApproval(t *testing.T) {
	for _, scenario := range []string{"fresh", "stopped legacy", "withdrawn approval", "changed scope", "cancelled failure"} {
		t.Run(scenario, func(t *testing.T) { phaseCapacityRecoveryContract(t, scenario) })
	}
}
func TestPhaseCapacityRecoveryPostgres(t *testing.T) {
	for _, scenario := range []string{"fresh", "stopped legacy", "withdrawn approval", "changed scope"} {
		t.Run(scenario, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { phaseCapacityRecoveryContract(t, scenario, database) })
		})
	}
}
func phaseCapacityRecoveryContract(t *testing.T, scenario string, databases ...*sql.DB) {
	t.Helper()
	s, approval, _, d, _ := phaseLineageFixture(t, databases...)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	var origin db.InboxRequest
	requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	for _, q := range requests {
		if q.MessageID == d.SourceMessageID {
			origin = q
		}
	}
	if origin.RequestID == "" {
		t.Fatal("missing phase context")
	}
	code := 75
	a := &task.TaskAttempt{ID: "quota-result", TaskID: d.ID, Status: task.AttemptStatusFailed, AttemptNumber: 1, ExitCode: &code, ErrorMessage: "The AI provider reported a usage limit. The run was stopped; saved work is preserved.", CreatedAt: now}
	if scenario == "cancelled failure" {
		a.CancelRequested = true
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	d.Status = task.TaskStatusBlocked
	if scenario == "changed scope" {
		d.Instructions += "\nDifferent scope"
	}
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	key := "task-provider-recovery:" + a.ID
	var legacy *task.TaskLaunch
	if scenario == "stopped legacy" {
		start := now
		job := task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: delegationMessageID(key), SourceRequestID: origin.RequestID, OwnerID: origin.OwnerID, RoomID: origin.RoomID, AgentID: origin.AgentID, Revision: "Preserve saved files. Do not push. Do not deploy.", InstructionsHash: task.LaunchInstructionsHash(d.Instructions), CreatedAt: start, AvailableAt: start, ExpiresAt: start.Add(time.Hour)}
		var err error
		legacy, err = s.taskSvc.SaveLaunch(ctx, job)
		if err != nil {
			t.Fatal(err)
		}
		blocker, _ := s.taskSvc.Store().GetTask(ctx, "phase-blocker")
		blocker.Status = task.TaskStatusDone
		if err := s.taskSvc.Store().UpdateTask(ctx, blocker); err != nil {
			t.Fatal(err)
		}
		d.Status = task.TaskStatusQueued
		if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		claimed, err := s.taskSvc.ClaimLaunch(ctx, start, "fixture", legacy.ID)
		if err != nil || claimed == nil {
			t.Fatal(claimed, err)
		}
		if err := s.taskSvc.FinishLaunch(ctx, legacy.ID, claimed.LeaseToken, "stopped", start, "The original user authorization is unavailable or cancelled."); err != nil {
			t.Fatal(err)
		}
		d.Status = task.TaskStatusBlocked
		if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
			t.Fatal(err)
		}
		blocker.Status = task.TaskStatusDoing
		if err := s.taskSvc.Store().UpdateTask(ctx, blocker); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "withdrawn approval" {
		m, _ := s.projectStudioRepo.GetMessageByID(ctx, approval.MessageID)
		m.Content = "Approval withdrawn"
		if err := s.projectStudioRepo.UpdateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	recovered := s.recoverInterruptedInboxTask(ctx, d, origin)
	want := scenario == "fresh" || scenario == "stopped legacy"
	if recovered != want {
		t.Fatalf("recovered=%v scenario=%s", recovered, scenario)
	}
	if !want {
		return
	}
	if legacy != nil {
		key += ":phase-approval:v1"
	}
	id := task.LaunchID(d.ID, delegationMessageID(key))
	saved, err := s.taskSvc.GetLaunch(ctx, id)
	if err != nil || saved == nil || saved.State != "queued" || saved.SourceRequestID != approval.RequestID {
		t.Fatal("lost original approval", saved, err)
	}
	if source := automaticRecoverySource(d, saved.RequestID); source == nil || source.ID != a.ID {
		t.Fatal("lost failed-attempt binding")
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, *saved, d)
	if err != nil || !authorized {
		t.Fatal("recovery rejected real approval", err)
	}
	if limits := executionpolicy.FromInstructions(d.Instructions).Restrict(executionpolicy.FromInstructions(saved.Revision)); !limits.DenyPush || !limits.DenyDeploy {
		t.Fatal("recovery relaxed publication boundary")
	}
	if legacy != nil {
		old, _ := s.taskSvc.GetLaunch(ctx, legacy.ID)
		if old.State != "stopped" || old.SourceRequestID != origin.RequestID || !old.ExpiresAt.Equal(saved.ExpiresAt) {
			t.Fatal("legacy history or expiry changed", old, saved)
		}
	}
	// Replay against the same saved failed attempt must retain exactly this launch.
	if !s.recoverInterruptedInboxTask(ctx, d, origin) {
		t.Fatal("idempotent replay failed")
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(after.Attempts) != 1 || after.LatestAttempt.ID != a.ID {
		t.Fatal("authorization check started duplicate execution")
	}
}
