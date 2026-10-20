package server

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Only an explicitly supplied disposable PostgreSQL database is used. Every test
// creates its own schema; no public/staging application tables are migrated.
func TestInboxLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_LIFECYCLE_SQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL DSN")
	}
	ctx := context.Background()
	admin, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal("open disposable DB")
	}
	defer admin.Close()
	schema := "inbox_test_" + strings.ReplaceAll(db.GenerateUUID(), "-", "")
	if _, e = admin.ExecContext(ctx, `CREATE SCHEMA `+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	parsed, e := url.Parse(dsn)
	if e != nil {
		t.Fatal("invalid test DSN")
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	isolated := parsed.String()
	database, e := sql.Open("pgx", isolated)
	if e != nil {
		t.Fatal("open isolated schema")
	}
	defer database.Close()
	migrations, e := db.LoadMigrations()
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range migrations {
		body := strings.ReplaceAll(m.UpSQL, `CREATE EXTENSION IF NOT EXISTS "pgcrypto";`, "")
		if _, e = database.ExecContext(ctx, body); e != nil {
			t.Fatalf("migration %s: %v", m.Version, e)
		}
	}
	if _, e = database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Fixture','') ON CONFLICT DO NOTHING`); e != nil {
		t.Fatal(e)
	}
	repo := db.NewSQLProjectStudioRepository(database)
	room := db.GenerateUUID()
	now := time.Now().UTC()
	if e = repo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Lifecycle SQL fixture", CreatedAt: now, UpdatedAt: now}); e != nil {
		t.Fatal(e)
	}
	accept := func(offset time.Duration) db.InboxRequest {
		q := db.InboxRequest{RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: "owner_fixture", AgentID: "agent_fixture", PayloadHash: "same", State: "queued", CreatedAt: now.Add(offset), Deadline: now.Add(time.Hour)}
		if _, fresh, e := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", AuthorID: "fixture", AuthorName: "Fixture", Content: "fixture", CreatedAt: q.CreatedAt}); e != nil || !fresh {
			t.Fatal(e)
		}
		return q
	}
	first := accept(0)
	second := accept(time.Millisecond)
	if _, ok, e := repo.ClaimInboxRequest(ctx, room, second.RequestID); e != nil || ok {
		t.Fatal("SQL FIFO violated", e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, e := repo.ClaimInboxRequest(ctx, room, first.RequestID)
			if e != nil {
				t.Error(e)
			}
			if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatal("SQL duplicate claims", wins)
	}
	if e = repo.AppendInboxStep(ctx, room, first.RequestID, "provider", "Fixture progress"); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.CancelInboxRequest(ctx, room, first.RequestID); e != nil {
		t.Fatal(e)
	}
	if _, ok, e := repo.ClaimInboxRequest(ctx, room, second.RequestID); e != nil || ok {
		t.Fatal("SQL claim during cancellation", e)
	}
	first.State = "completed"
	if e = repo.FinishInboxRequest(ctx, first, &db.ProjectMessage{ID: first.ReplyID, RoomID: room, AuthorType: "agent", AuthorName: "Fixture", Content: "late result", CreatedAt: time.Now()}); e != nil {
		t.Fatal(e)
	}
	all, e := repo.ListInboxRequests(ctx, room)
	if e != nil || len(all) != 2 || all[0].State != "cancelled" || len(all[0].Steps) != 2 || all[0].Steps[1].Kind != "finished" || all[0].OwnerID != "owner_fixture" {
		t.Fatal("SQL terminal snapshot", all, e)
	}
	messages, e := repo.ListMessagesByRoomID(ctx, room)
	if e != nil || len(messages) != 2 {
		t.Fatal("SQL late reply survived", e)
	}
	if _, ok, e := repo.ClaimInboxRequest(ctx, room, second.RequestID); e != nil || !ok {
		t.Fatal("SQL did not continue", e)
	}
	// Canonical task idempotency and local-attempt reservation use the real store.
	svc := task.NewService(task.NewDBStore(database), nil)
	var createdID string
	for i := 0; i < 2; i++ {
		created, e := svc.CreateTaskFromMessage(ctx, "fixture", first.MessageID, task.CreateTaskRequest{ProjectID: "default", Title: "SQL source task", Instructions: "Fixture"})
		if e != nil {
			t.Fatal(e)
		}
		if createdID != "" && createdID != created.ID {
			t.Fatal("SQL task duplicated")
		}
		createdID = created.ID
	}
	a, e := svc.CreateAttempt(ctx, createdID, "fixture", task.CreateAttemptRequest{RunnerID: "local_terminal"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CreateAttempt(ctx, createdID, "fixture", task.CreateAttemptRequest{RunnerID: "local_terminal"}); e == nil {
		t.Fatal("SQL duplicate active attempt")
	}
	if _, e = svc.CancelTask(ctx, createdID, "fixture"); e != nil {
		t.Fatal(e)
	}
	zero := 0
	finished, e := svc.UpdateAttemptStatus(ctx, a.ID, "fixture", task.UpdateAttemptRequest{Status: "succeeded", ExitCode: &zero})
	if e != nil || finished.Status != "canceled" {
		t.Fatal("SQL cancel overwritten", e)
	}
	// Separate service/store instances cannot revive a terminal SQL attempt.
	stale := *a
	stale.Status = "running"
	if e = task.NewDBStore(database).UpdateAttempt(ctx, &stale); e == nil {
		t.Fatal("SQL terminal row revived")
	}
	newer, e := svc.CreateAttempt(ctx, createdID, "fixture", task.CreateAttemptRequest{RunnerID: "local_terminal"})
	if e != nil {
		t.Fatal(e)
	}
	doing := task.TaskStatusDoing
	svc.UpdateTask(ctx, createdID, "fixture", task.UpdateTaskRequest{Status: &doing})
	if e = svc.ReconcileLocalTask(ctx, createdID, a.ID); e != nil {
		t.Fatal(e)
	}
	detail, e := svc.GetTaskDetail(ctx, createdID)
	if e != nil || detail.Status != "doing" || detail.LatestAttempt.ID != newer.ID || newer.AttemptNumber != 2 {
		t.Fatal("SQL stale task reconciliation", detail, e)
	}
	// Logs survive reconstructing the service around another DBStore instance.
	if e = svc.AppendAttemptLogs(ctx, newer.ID, []task.LogRecord{{AttemptID: newer.ID, Stream: "stdout", Line: "retained SQL fixture", LoggedAt: time.Now()}}, 1000); e != nil {
		t.Fatal(e)
	}
	reloaded := task.NewService(task.NewDBStore(database), nil)
	retained, e := reloaded.GetAttemptLogs(ctx, newer.ID, 0, 1000)
	if e != nil || len(retained) != 1 || retained[0].Line != "retained SQL fixture" {
		t.Fatal("SQL logs not durable", retained, e)
	}
	// Existing regression launches distinct OS processes and verifies completed
	// replay and unknown provider outcome after actual service-process loss.
	t.Setenv("S264_SQL_DSN", isolated)
	TestS264PostgresProcessRestart(t)
}
