package server

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestProjectWorkflowKeepsTurnAcrossNativeStagesPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		ctx := context.Background()
		now := time.Now().UTC()
		store := task.NewDBStore(database)
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		}
		_, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('workflow-project','Workflow fixture','')`)
		must(err)
		room, message := db.GenerateUUID(), db.GenerateUUID()
		_, err = database.ExecContext(ctx, `INSERT INTO project_rooms(id,project_id,title) VALUES($1,'workflow-project','Workflow fixture')`, room)
		must(err)
		_, err = database.ExecContext(ctx, `INSERT INTO project_messages(id,room_id,author_type,author_id,content) VALUES($1,$2,'user','owner','Implement approved work')`, message, room)
		must(err)
		owner := &task.Task{ID: "workflow-owner", ProjectID: "workflow-project", SourceRoomID: room, SourceMessageID: message, Instructions: "Cuan managed delegation v1.\nImplement approved work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode, AssignedAgent: "agy", CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
		other := &task.Task{ID: "workflow-other", ProjectID: owner.ProjectID, Instructions: "Different queued work", Status: task.TaskStatusQueued, ExecutionCategory: task.ExecutionCategoryCode, AssignedAgent: "agy", CreatedAt: now, UpdatedAt: now}
		must(store.CreateTask(ctx, owner))
		must(store.CreateTask(ctx, other))
		start := now.Add(-time.Minute)
		source := &task.TaskAttempt{ID: "workflow-source", TaskID: owner.ID, RunnerID: "native", AgentName: "agy", Status: task.AttemptStatusRunning, StartedAt: &start, CreatedAt: start}
		must(store.CreateAttempt(ctx, source))
		outsider := &task.TaskAttempt{ID: "workflow-outsider", TaskID: other.ID, AgentName: "agy", Status: task.AttemptStatusQueued, CreatedAt: now}
		must(store.CreateAttempt(ctx, outsider))
		rs := runner.NewService(runner.NewDBStore(database), store, nil)
		_, err = rs.Complete(ctx, source.ID, runner.CompleteAttemptRequest{RunnerID: "native", Status: task.AttemptStatusSucceeded, TestSummary: "Project verification: fixture (exit 0)"})
		must(err)
		blocked, err := store.IsProjectLaneActive(ctx, owner.ProjectID, "code", other.ID)
		must(err)
		if !blocked {
			t.Fatal("project released its turn between dev and review")
		}
		_, err = rs.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: "next", SupportedAgents: []string{"agy"}}, time.Minute)
		must(err)
		first, err := rs.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "next", SupportedAgents: []string{"agy"}}, time.Minute)
		must(err)
		if first.Claimed {
			t.Fatal("another task cut into workflow handoff", first.Task.ID)
		}
		ts := task.NewService(store, nil)
		job, err := ts.SaveLaunch(ctx, task.TaskLaunch{TaskID: owner.ID, ProjectID: owner.ProjectID, RequestID: db.GenerateUUID(), OwnerID: "owner", RoomID: room, SourceRequestID: "original", AgentID: "agy", InstructionsHash: task.LaunchInstructionsHash(owner.Instructions), CreatedAt: time.Now().UTC(), AvailableAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)})
		must(err)
		claimed, err := ts.ClaimLaunch(ctx, time.Now().UTC(), "dispatch", job.ID)
		must(err)
		if claimed == nil {
			t.Fatal("own next stage could not claim its turn")
		}
		// This fixture exercises scheduling; typed source/authorization admission
		// is covered by the native verifier/publication protocol fixtures.
		var nextID string
		must(database.QueryRowContext(ctx, `SELECT attempt_id FROM task_launches WHERE id=$1`, job.ID).Scan(&nextID))
		next := &task.TaskAttempt{ID: nextID, TaskID: owner.ID, RunnerID: "next", AgentName: "agy", Status: task.AttemptStatusQueued, CreatedAt: time.Now().UTC()}
		must(store.CreateAttempt(ctx, next))
		phase, err := rs.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "next", SupportedAgents: []string{"agy"}}, time.Minute)
		must(err)
		if !phase.Claimed || phase.Attempt.ID != next.ID {
			t.Fatal("workflow continuation lost to older queued outsider", phase)
		}
		_, err = rs.Complete(ctx, next.ID, runner.CompleteAttemptRequest{RunnerID: "next", Status: task.AttemptStatusSucceeded, TestSummary: "Project verification: fixture (exit 0)"})
		must(err)
		must(store.CreateActivity(ctx, &task.TaskActivity{ID: "owner-only-decision", TaskID: owner.ID, AttemptID: &next.ID, Author: "coordinator", ActivityType: "coordinator_review_assessment", Message: `{"assessment":{"manager_action":"needs_owner","requested_outcome":"needs_input"}}`, CreatedAt: time.Now().UTC()}))
		blocked, err = store.IsProjectLaneActive(ctx, owner.ProjectID, "code", other.ID)
		must(err)
		if blocked {
			t.Fatal("owner decision retained project turn")
		}
		outsiderClaim, err := rs.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "next", SupportedAgents: []string{"agy"}}, time.Minute)
		must(err)
		if !outsiderClaim.Claimed || outsiderClaim.Attempt.ID != outsider.ID {
			t.Fatal("next work could not proceed after owner decision", outsiderClaim)
		}
	})
}
