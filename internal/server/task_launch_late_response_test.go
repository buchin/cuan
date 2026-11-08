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

func TestLateLaunchErrorPreservesRecordedExecution(t *testing.T) {
	for _, state := range []string{task.AttemptStatusQueued, task.AttemptStatusRunning, task.AttemptStatusFailed, task.AttemptStatusSucceeded, "unbound"} {
		t.Run(state, func(t *testing.T) { lateLaunchResponseContract(t, nil, state) })
	}
}

func TestLateLaunchErrorPreservesRecordedExecutionPostgres(t *testing.T) {
	for _, state := range []string{task.AttemptStatusRunning, task.AttemptStatusFailed, "unbound"} {
		t.Run(state, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { lateLaunchResponseContract(t, database, state) })
		})
	}
}

func lateLaunchResponseContract(t *testing.T, database *sql.DB, state string) {
	t.Helper()
	s, _, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	if database != nil {
		s.taskSvc = task.NewService(task.NewDBStore(database), nil)
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Default','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	item := &task.Task{ID: "late-response", ProjectID: "default", AssignedAgent: "agent_lifecycle", SourceRoomID: room.RoomID, Title: "Approved work", Instructions: "Preserve original work", Status: task.TaskStatusQueued, CreatedAt: now, UpdatedAt: now}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.taskSvc.Store().CreateTask(ctx, item))
	j, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: db.GenerateUUID(), SourceRequestID: db.GenerateUUID(), OwnerID: "owner", RoomID: room.RoomID, AgentID: item.AssignedAgent, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	must(err)
	lease, err := s.taskSvc.ClaimLaunch(ctx, now, "late-dispatcher", j.ID)
	must(err)
	if lease == nil {
		t.Fatal("launch not claimable")
	}
	if state != "unbound" {
		a, err := s.taskSvc.CreateAttempt(ctx, item.ID, j.OwnerID, task.CreateAttemptRequest{RunnerID: "fixture", AgentName: item.AssignedAgent, RequestID: j.RequestID})
		must(err)
		a.Status = state
		must(s.taskSvc.Store().UpdateAttempt(ctx, a))
		if state == task.AttemptStatusFailed || state == task.AttemptStatusSucceeded {
			status := task.TaskStatusBlocked
			if state == task.AttemptStatusSucceeded {
				status = task.TaskStatusReview
			}
			row, err := s.taskSvc.Store().GetTask(ctx, item.ID)
			must(err)
			row.Status = status
			must(s.taskSvc.Store().UpdateTask(ctx, row))
		}
	}
	before, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	messagesBefore, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.RoomID)
	must(err)
	// Reproduce a dispatcher receiving a stale HTTP rejection after another
	// dispatcher already saved this exact request's execution.
	out := s.finishQueuedTaskLaunch(ctx, *lease, true, "stopped", "the saved result inspection no longer matches this task and running release")
	after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	saved, err := s.taskSvc.GetLaunch(ctx, j.ID)
	must(err)
	messagesAfter, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.RoomID)
	must(err)
	if len(before.Attempts) != len(after.Attempts) || (state != "unbound" && before.Status != after.Status) {
		t.Fatal("late response changed execution or task")
	}
	if state == "unbound" {
		if saved.State != "stopped" || saved.SafeError == "" || !strings.Contains(out, "could not start") {
			t.Fatal("real admission failure was hidden")
		}
		return
	}
	want := "stopped"
	if state == task.AttemptStatusQueued || state == task.AttemptStatusRunning {
		want = "started"
	}
	if saved.State != want || saved.SafeError != "" || !strings.Contains(out, "recorded execution") || after.LatestAttempt.ID != before.LatestAttempt.ID || after.LatestAttempt.Status != state {
		t.Fatalf("late error replaced execution: %+v %s", saved, out)
	}
	if len(messagesAfter) != len(messagesBefore) {
		t.Fatal("late launch error added a misleading failure bubble")
	}
}
