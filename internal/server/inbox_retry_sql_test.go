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
	if reevaluated, fresh, err := repo.RetryInboxRequest(ctx, room, child.RequestID, source.OwnerID); err != nil || !fresh || !db.InboxRetryNeedsReconciliation(reevaluated) {
		t.Fatal("unknown result must use reconciliation rather than replay", err)
	} else {
		reevaluated.State = "completed"
		if err := repo.FinishInboxRequest(ctx, reevaluated, nil); err != nil {
			t.Fatal(err)
		}
	}
	capped := accept(3 * time.Millisecond)
	if _, ok, err := repo.ClaimInboxRequest(ctx, room, capped.RequestID); err != nil || !ok {
		t.Fatal("claim capped fixture", err)
	}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.AppendInboxStep(ctx, room, capped.RequestID, "context", "Concurrent stage"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 120; i++ {
		if err := repo.AppendInboxStep(ctx, room, capped.RequestID, "runtime_tool_started", "Fixture"); err != nil {
			t.Fatal(err)
		}
	}
	// A journal failure must roll back both state and the live projection.
	if _, err := database.ExecContext(ctx, `CREATE FUNCTION reject_stage_fixture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture'; END $$; CREATE TRIGGER reject_stage BEFORE INSERT ON inbox_request_steps FOR EACH ROW EXECUTE FUNCTION reject_stage_fixture()`); err != nil {
		t.Fatal(err)
	}
	failedFinal := capped
	failedFinal.State = "interrupted"
	if err := repo.FinishInboxRequest(ctx, failedFinal, nil); err == nil {
		t.Fatal("journal failure accepted finalization")
	}
	var stillRunning string
	if err := database.QueryRowContext(ctx, `SELECT state FROM inbox_requests WHERE room_id=$1 AND request_id=$2`, room, capped.RequestID).Scan(&stillRunning); err != nil || stillRunning != "running" {
		t.Fatal("partial finalization", stillRunning, err)
	}
	if _, err := database.ExecContext(ctx, `DROP TRIGGER reject_stage ON inbox_request_steps; DROP FUNCTION reject_stage_fixture()`); err != nil {
		t.Fatal(err)
	}
	capped.State = "interrupted"
	if err := repo.FinishInboxRequest(ctx, capped, nil); err != nil {
		t.Fatal(err)
	}
	finalRows, err := repo.ListInboxRequests(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	finalFound := false
	for _, row := range finalRows {
		if row.RequestID == capped.RequestID {
			finalFound = len(row.Steps) == 100 && row.Steps[99].Kind == "finished" && row.Steps[99].Seq == 133
		}
	}
	if !finalFound {
		t.Fatal("SQL final timestamp lost at timeline cap")
	}
	page, err := repo.ListInboxSteps(ctx, room, capped.RequestID, "owner_fixture", 100, 100)
	if err != nil || len(page.Steps) != 33 || page.Steps[0].Seq != 101 || page.Steps[32].Kind != "finished" {
		t.Fatal("SQL journal lost late stages", page, err)
	}
	if _, err := repo.ListInboxSteps(ctx, room, capped.RequestID, "foreign", 0, 100); err == nil {
		t.Fatal("foreign owner read trace")
	}
	autoSource := accept(time.Since(now))
	if _, ok, err := repo.ClaimInboxRequest(ctx, room, autoSource.RequestID); err != nil || !ok {
		t.Fatal("claim automatic fixture", err)
	}
	repo.AppendInboxStep(ctx, room, autoSource.RequestID, "recovery_policy", "safe-reply-v1")
	repo.AppendInboxStep(ctx, room, autoSource.RequestID, "recovery_safe", "No action was performed.")
	autoSource.State = "failed"
	autoSource.SafeError = "The provider could not reply."
	if err := repo.FinishInboxRequest(ctx, autoSource, nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2100 * time.Millisecond)
	wins = 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			var child db.InboxRequest
			var fresh bool
			var err error
			if index%2 == 0 {
				child, fresh, err = repo.AutoRetryInboxRequest(ctx, room, autoSource.RequestID, autoSource.OwnerID)
			} else {
				child, fresh, err = repo.RetryInboxRequest(ctx, room, autoSource.RequestID, autoSource.OwnerID)
			}
			if err != nil {
				t.Error(err)
				return
			}
			if child.RequestID != db.InboxRetryID(autoSource.RequestID) || child.MessageID != autoSource.MessageID {
				t.Error("automatic correlation lost")
			}
			if fresh {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("auto/manual race created %d children", wins)
	}
	// Reapply the new migration in this disposable schema after restoring the
	// pre-journal table shape, to prove existing capped history is not fabricated.
	if _, err := database.ExecContext(ctx, `DROP TABLE inbox_request_steps`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if strings.HasPrefix(migration.Version, "000036") || migration.Name == "inbox_step_journal" {
			if _, err := database.ExecContext(ctx, migration.UpSQL); err != nil {
				t.Fatal("journal upgrade", err)
			}
		}
	}
	upgraded, err := repo.ListInboxRequests(ctx, room)
	if err != nil {
		t.Fatal(err)
	}
	marked := false
	for _, row := range upgraded {
		if row.RequestID == capped.RequestID {
			for _, stage := range row.Steps {
				if stage.Kind == "trace_history_incomplete" {
					marked = true
				}
			}
			row.State = "failed"
			row.SafeError = "Provider is unavailable."
			if db.InboxRecoveryAction(row) != "check_status" {
				t.Fatal("incomplete legacy history authorized replay")
			}
		}
	}
	if !marked {
		t.Fatal("legacy truncation not recorded")
	}
	upgradedPage, err := repo.ListInboxSteps(ctx, room, capped.RequestID, "owner_fixture", 133, 100)
	if err != nil || len(upgradedPage.Steps) != 1 || upgradedPage.Steps[0].Kind != "trace_history_incomplete" {
		t.Fatal("upgrade marker not archived", upgradedPage, err)
	}
	t.Log("SQL retry: twelve simultaneous clicks, one new attempt, original user message retained; ownership and unknown outcome enforced; no inference")
}
