package task

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	database "github.com/buchin/cuan/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Only an explicitly dedicated database may run this migration/storage check.
func TestTaskWorkspacePostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_TASK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Path != "/cuan_tasks_workspace" {
		t.Fatal("dedicated cuan_tasks_workspace database required")
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	migrations, e := database.LoadMigrations()
	if e != nil {
		t.Fatal(e)
	}
	// Apply the previous schema first and seed an old-style row.
	if _, e = db.ExecContext(ctx, `CREATE TABLE cuan_schema_migrations(version VARCHAR(64) PRIMARY KEY,name VARCHAR(255) NOT NULL,applied_at TIMESTAMPTZ DEFAULT now())`); e != nil {
		t.Fatal(e)
	}
	for _, m := range migrations {
		if m.Version == "000023" {
			break
		}
		if _, e = db.ExecContext(ctx, m.UpSQL); e != nil {
			t.Fatalf("schema %s: %v", m.Version, e)
		}
		if _, e = db.ExecContext(ctx, `INSERT INTO cuan_schema_migrations(version,name) VALUES($1,$2)`, m.Version, m.Name); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.ExecContext(ctx, `INSERT INTO tasks(id,title,instructions,status,priority) VALUES('legacy_task','Legacy task','Original human instructions','backlog','normal')`); e != nil {
		t.Fatal(e)
	}
	if e = database.Run(ctx, db, "up"); e != nil {
		t.Fatal(e)
	}
	store := NewDBStore(db)
	svc := NewService(store, nil)
	legacy, e := store.GetTask(ctx, "legacy_task")
	if e != nil || legacy.Instructions != "Original human instructions" || legacy.Summary != "" || len(legacy.DoDChecklist) != 0 {
		t.Fatal("old task not preserved", legacy, e)
	}
	req := CreateTaskRequest{Title: "Storage fixture", Instructions: "Keep original", DoDChecklist: []DoDItem{{Text: "Verify SQL"}}, RequestID: "10000000-0000-4000-8000-000000000077"}
	created, e := svc.CreateTask(ctx, "owner", req)
	if e != nil {
		t.Fatal(e)
	}
	got, e := store.GetTask(ctx, created.ID)
	if e != nil || got.ProjectID != "" || len(got.DoDChecklist) != 1 {
		t.Fatal("create/get", got, e)
	}
	summary, note := "SQL summary", "SQL verification"
	if _, e = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Summary: &summary, CompleteNote: &note}); e != nil {
		t.Fatal(e)
	}
	// A reconstructed service must recover the original create request after edits.
	replay, e := NewService(store, nil).CreateTask(ctx, "owner", req)
	if e != nil || replay.ID != created.ID || replay.Summary != summary {
		t.Fatal("create replay", replay, e)
	}
	run := CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "fixture", RequestID: "10000000-0000-4000-8000-000000000078", RequestHash: "fixture-hash"}
	attempt, e := svc.CreateAttempt(ctx, created.ID, "owner", run)
	if e != nil {
		t.Fatal(e)
	}
	if e = store.DeleteTask(ctx, created.ID); !errors.Is(e, ErrTaskActive) {
		t.Fatal("active delete", e)
	}
	doing := TaskStatusDoing
	if _, e = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &doing}); e != nil {
		t.Fatal(e)
	}
	if e = svc.RequireAgentReport(ctx, created.ID, attempt.ID, "fixture"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.UpdateAttemptStatus(ctx, attempt.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusRunning}); e != nil {
		t.Fatal(e)
	}
	summaryReport, noteReport := "SQL summary", "SQL verification"
	if _, e = svc.ReportAgent(ctx, created.ID, attempt.ID, "fixture", AgentReport{Summary: &summaryReport, CompleteNote: &noteReport, State: "review"}); e != nil {
		t.Fatal(e)
	}
	zero := 0
	if _, e = svc.UpdateAttemptStatus(ctx, attempt.ID, "runner", UpdateAttemptRequest{Status: AttemptStatusSucceeded, ExitCode: &zero}); e != nil {
		t.Fatal(e)
	}
	if e = svc.ReconcileLocalTask(ctx, created.ID, attempt.ID); e != nil {
		t.Fatal(e)
	}
	got, e = store.GetTask(ctx, created.ID)
	if e != nil || got.Status != TaskStatusReview || got.Summary != summary || got.CompleteNote != note {
		t.Fatal("review persistence", got, e)
	}
	checked := []DoDItem{{Text: "Verify SQL", Completed: true}}
	done := TaskStatusDone
	if _, e = svc.UpdateTask(ctx, created.ID, "owner", UpdateTaskRequest{Status: &done, DoDChecklist: &checked}); e != nil {
		t.Fatal(e)
	}
	if e = svc.ReconcileLocalTask(ctx, created.ID, attempt.ID); e != nil {
		t.Fatal(e)
	}
	got, _ = store.GetTask(ctx, created.ID)
	if got.Status != TaskStatusDone {
		t.Fatal("late reconciliation downgraded approval")
	}
	recorded, e := NewService(store, nil).CreateAttempt(ctx, created.ID, "owner", run)
	if e != nil || recorded.ID != attempt.ID || recorded.Status != AttemptStatusSucceeded || recorded.RequestHash != "fixture-hash" {
		t.Fatal("durable attempt replay", recorded, e)
	}
	listed, e := store.ListTasks(ctx, TaskFilter{})
	if e != nil || len(listed) != 2 {
		t.Fatal("list", e, len(listed))
	}
	if e = svc.DeleteTask(ctx, created.ID); e != nil {
		t.Fatal(e)
	}
	// An old-style insert still works while the additive schema remains installed.
	if _, e = db.ExecContext(ctx, `INSERT INTO tasks(id,title,instructions,status,priority) VALUES('rollback_task','Old binary row','Unchanged','backlog','normal')`); e != nil {
		t.Fatal("rollback insert", e)
	}
	t.Log("PASS schema22→23 retained legacy row; actual DB create/update/list/replay/checklist/approval/delete; late exit preserved Done; old-style SQL remains compatible")
}
