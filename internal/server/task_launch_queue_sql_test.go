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

func TestQueuedLaunchPostgresServerRecovery(t *testing.T) {
	dsn := os.Getenv("CUAN_LAUNCH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires dedicated PostgreSQL database")
	}
	u, e := url.Parse(dsn)
	if e != nil || !strings.HasPrefix(u.Path, "/cuan_launch_queue_server_") {
		t.Fatal("dedicated cuan_launch_queue_server_* database required")
	}
	database, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer database.Close()
	if e = db.Run(context.Background(), database, "up"); e != nil {
		t.Fatal(e)
	}
	for _, scenario := range []string{"resume", "cancel", "scope-changed"} {
		t.Run(scenario, func(t *testing.T) {
			if _, e = database.Exec(`TRUNCATE task_launches,tasks,project_rooms,projects CASCADE`); e != nil {
				t.Fatal(e)
			}
			runQueuedLaunchScenario(t, scenario, database)
		})
	}
}
