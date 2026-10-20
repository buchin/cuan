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
)

// Only an explicitly supplied disposable PostgreSQL database is used. Every test
// creates its own schema; no public/staging application tables are migrated.
func TestInboxRetryPostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_LIFECYCLE_SQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL DSN")
	}
	parsedDSN, err := url.Parse(dsn)
	if err != nil || parsedDSN.Path != "/cuan_s25" {
		t.Fatal("isolated cuan_s25 database required")
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

	source := accept(0)
	source.State = "failed"
	source.SafeError = "The chat sandbox is unavailable."
	if err := repo.FinishInboxRequest(ctx, source, nil); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			child, fresh, err := repo.RetryInboxRequest(ctx, room, source.RequestID, source.OwnerID)
			if err != nil {
				t.Error(err)
				return
			}
			if child.MessageID != source.MessageID || child.RequestID != db.InboxRetryID(source.RequestID) {
				t.Error("correlation lost")
			}
			if fresh {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("created %d retry rows", wins)
	}
	requests, err := repo.ListInboxRequests(ctx, room)
	if err != nil || len(requests) != 2 {
		t.Fatal(requests, err)
	}
	messages, err := repo.ListMessagesByRoomID(ctx, room)
	if err != nil || len(messages) != 1 {
		t.Fatal("user message duplicated", err)
	}
	if _, _, err := repo.RetryInboxRequest(ctx, room, source.RequestID, "other"); err == nil {
		t.Fatal("cross-owner retry")
	}
	if _, _, err := repo.RetryInboxRequest(ctx, room, db.InboxRetryID(source.RequestID), source.OwnerID); err == nil {
		t.Fatal("queued request retried")
	}
	child := requests[1]
	child.State = "failed"
	child.SafeError = "The result is unknown."
	if err := repo.FinishInboxRequest(ctx, child, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.RetryInboxRequest(ctx, room, child.RequestID, source.OwnerID); err == nil {
		t.Fatal("unknown result retried")
	}
	t.Log("SQL retry: twelve simultaneous clicks, one new attempt, original user message retained; ownership and unknown outcome enforced; no inference")
}
