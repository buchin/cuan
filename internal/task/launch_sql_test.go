package task

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	database "github.com/buchin/cuan/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestLaunchPostgresContract(t *testing.T) {
	dsn := os.Getenv("CUAN_LAUNCH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	u, e := url.Parse(dsn)
	if e != nil || !strings.HasPrefix(u.Path, "/cuan_launch_queue_") {
		t.Fatal("dedicated cuan_launch_queue_* database required")
	}
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if e = database.Run(ctx, db, "up"); e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('launch-project','Launch fixture','')`); e != nil {
		t.Fatal(e)
	}
	room := "91000000-0000-4000-8000-000000000001"
	if _, e = db.ExecContext(ctx, `INSERT INTO project_rooms(id,project_id,title) VALUES($1,'launch-project','Launch fixture')`, room); e != nil {
		t.Fatal(e)
	}
	store := NewDBStore(db)
	svc := NewService(store, nil)
	created, e := svc.CreateTask(ctx, "owner", CreateTaskRequest{Title: "Authorized fixture", ProjectID: "launch-project", Instructions: "Preserve this scope"})
	if e != nil {
		t.Fatal(e)
	}
	launchContract(t, store, created.ID, room)
	stoppedLaunchBoardContract(t, store, "launch-project")
	attemptPublicationPolicyContract(t, store, "launch-project")
	unstartedWorkspaceRecoveryContract(t, store, "launch-project")
	localInboxAcceptanceContract(t, store, "launch-project")
	latestProgressAttemptContract(t, store, "launch-project")
	operationalAcceptanceAuditContract(t, store, "launch-project")
	if _, e := db.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('page-project','Page fixture','')`); e != nil {
		t.Fatal(e)
	}
	taskPageContract(t, store, "page-project")
	if _, e := db.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('page-launch-project','Page launch fixture','')`); e != nil {
		t.Fatal(e)
	}
	taskPageLaunchContract(t, store, "page-launch-project")
	taskPageScalePostgres(t, db, store)
}
