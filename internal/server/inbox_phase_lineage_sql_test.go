package server

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestApprovedPhasePostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_LIFECYCLE_SQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL DSN")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Path != "/cuan_s25" {
		t.Fatal("isolated cuan_s25 database required")
	}
	ctx := context.Background()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "phase_test_" + strings.ReplaceAll(db.GenerateUUID(), "-", "")
	if _, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	database, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	migrations, err := db.LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations {
		if _, err = database.ExecContext(ctx, migration.UpSQL); err != nil {
			t.Fatalf("migration %s: %v", migration.Version, err)
		}
	}
	s, q, parent, child, launch := phaseLineageFixture(t, database)
	restarted := &Server{taskSvc: s.taskSvc, projectStudioRepo: s.projectStudioRepo, projectSvc: s.projectSvc}
	_, authorized, err := restarted.authorizeSavedTaskLaunch(ctx, *launch, child)
	if err != nil || !authorized {
		t.Fatal("Postgres phase approval failed", err)
	}
	_ = s.enqueueInboxAutoDelegation(ctx, q, parent, "Phase accepted")
	var count int
	if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE source_room_id=$1`, q.RoomID).Scan(&count); err != nil || count != 2 {
		t.Fatal("phase replay duplicated tasks", count, err)
	}
}
