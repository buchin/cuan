package server

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestRecoveryCandidatesPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		ctx := context.Background()
		if _, err := database.ExecContext(ctx, `INSERT INTO tasks(id,title,instructions,status) SELECT 'settled-'||n,'Accepted result','Approved work','done' FROM generate_series(1,7000) n`); err != nil {
			t.Fatal(err)
		}
		store := task.NewDBStore(database)
		now := time.Now().UTC()
		for _, row := range []struct{ id, status, attempt, runner string }{
			{"active-with-stale-board", task.TaskStatusQueued, task.AttemptStatusRunning, "daemon"},
			{"stale-doing", task.TaskStatusDoing, task.AttemptStatusFailed, "local_terminal"},
			{"local-queue", task.TaskStatusQueued, task.AttemptStatusQueued, "local_terminal"},
			{"daemon-queue", task.TaskStatusQueued, task.AttemptStatusQueued, "daemon"},
		} {
			if err := store.CreateTask(ctx, &task.Task{ID: row.id, Title: "Scoped work", Instructions: "Original scope", Status: row.status, Priority: task.PriorityNormal, CreatedAt: now, UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			if err := store.CreateAttempt(ctx, &task.TaskAttempt{ID: "att-" + row.id, TaskID: row.id, AgentName: "agy", RunnerID: row.runner, Status: row.attempt, AttemptNumber: 1, CreatedAt: now}); err != nil {
				t.Fatal(err)
			}
		}
		ids, err := store.RecoveryTaskIDs(ctx)
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(ids)
		if fmt.Sprint(ids) != "[active-with-stale-board local-queue stale-doing]" {
			t.Fatal("SQL recovery selected history or omitted live work", ids)
		}
		rows, err := task.NewService(store, nil).RecoveryTasks(ctx)
		if err != nil || len(rows) != 3 {
			t.Fatalf("SQL recovery payload: %d %v", len(rows), err)
		}
		var settled int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE status='done'`).Scan(&settled); err != nil || settled != 7000 {
			t.Fatal("selection mutated settled work", settled, err)
		}
	})
}
