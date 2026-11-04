package db

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"os"
	"strings"
	"testing"
	"time"
)

func TestInboxDispatchPostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_LIFECYCLE_SQL_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable PostgreSQL required")
	}
	if !strings.Contains(dsn, "/cuan_launch_queue_") {
		t.Fatal("dedicated cuan_launch_queue database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err = Run(ctx, database, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('dispatch-fixture','Dispatch','')`); err != nil {
		t.Fatal(err)
	}
	repo := NewSQLProjectStudioRepository(database)
	now := time.Now().UTC()
	room := GenerateUUID()
	if err = repo.CreateRoom(ctx, &ProjectRoom{ID: room, ProjectID: "dispatch-fixture", Title: "Queued", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err = repo.CreateRoom(ctx, &ProjectRoom{ID: GenerateUUID(), ProjectID: "dispatch-fixture", Title: "Idle", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	q := InboxRequest{RoomID: room, RequestID: GenerateUUID(), MessageID: GenerateUUID(), ReplyID: GenerateUUID(), PayloadHash: "fixture", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err = repo.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", Content: "Fixture", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	check := func(count int) {
		t.Helper()
		ids, e := repo.ListInboxDispatchRooms(ctx, time.Now())
		if e != nil || len(ids) != count {
			t.Fatal(ids, e)
		}
		if count == 1 && ids[0] != room {
			t.Fatal(ids)
		}
	}
	check(1)
	q, _, err = repo.ClaimInboxRequest(ctx, room, q.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.AppendInboxStep(ctx, room, q.RequestID, "finished", "Fixture"); err != nil {
		t.Fatal(err)
	}
	q.State = "failed"
	if err = repo.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	check(1) // Recent failure remains discoverable for bounded safe-retry evaluation.
	ids, err := repo.ListInboxDispatchRooms(ctx, time.Now().Add(3*time.Minute))
	if err != nil || len(ids) != 0 {
		t.Fatal(ids, err)
	}
}
