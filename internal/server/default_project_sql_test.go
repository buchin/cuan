package server

import (
	"context"
	"database/sql"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestInboxInitializationPreservesDefaultPostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_DEFAULT_ROOMS_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires empty cuan_default_test_* database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || !strings.HasPrefix(parsed.Path, "/cuan_default_test_") {
		t.Fatal("dedicated database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	var tables int
	if err = database.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public'`).Scan(&tables); err != nil || tables != 0 {
		t.Fatal("database must be empty", err)
	}
	if err = db.Run(ctx, database, "up"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	s := NewServer(&config.Config{Environment: "staging", AuthSecret: "fixture-only-secret", WorkspacesDir: t.TempDir()}, database)
	defer s.terminalManager.CloseAll()
	verify := func(name, repo string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			s.ensureDefaultAgentRooms(ctx)
		}
		p, err := s.projectSvc.Get(ctx, "default")
		if err != nil || p.Name != name || p.RepoURL != repo {
			t.Fatal("Inbox initialization overwrote project identity", p, err)
		}
	}
	verify("Default", "")
	// Existing installations are separated explicitly; opening chat must never
	// replace any configured project name/repository, including historical data.
	if _, err = database.Exec(`UPDATE projects SET name='Existing project',repo_url='https://example.invalid/existing' WHERE id='default'`); err != nil {
		t.Fatal(err)
	}
	verify("Existing project", "https://example.invalid/existing")
}
