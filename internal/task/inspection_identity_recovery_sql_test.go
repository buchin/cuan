package task

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	database "github.com/buchin/cuan/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestInspectionIdentityPostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_INSPECTION_IDENTITY_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/cuan_identity_recovery" {
		t.Fatal("dedicated cuan_identity_recovery database required")
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err = database.Run(ctx, conn, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('identity-project','Identity fixture',''); INSERT INTO project_rooms(id,project_id,title) VALUES('91000000-0000-4000-8000-000000000079','identity-project','Identity fixture')`); err != nil {
		t.Fatal(err)
	}
	store := NewDBStore(conn)
	for _, scenario := range []string{"valid", "changed timestamp", "changed commit", "cancelled", "new attempt", "active attempt", "pending launch", "duplicate receipt"} {
		t.Run(scenario, func(t *testing.T) {
			_, item, a, proof := inspectionIdentityFixture(t)
			item.ID = "identity-" + scenario
			item.ProjectID = ""
			item.ProjectIDs = nil
			item.SourceRoomID = ""
			a.TaskID = item.ID
			a.ID = "attempt-" + scenario
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status,attempt_number,created_at,finished_at) VALUES($1,$2,'runner','agent','interrupted',2,$3,$4)`, a.ID, item.ID, a.CreatedAt, a.FinishedAt); err != nil {
				t.Fatal(err)
			}
			proof.SourceAttemptID = "source-" + scenario
			if _, err := conn.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status,attempt_number) VALUES($1,$2,'runner','agent','succeeded',1)`, proof.SourceAttemptID, item.ID); err != nil {
				t.Fatal(err)
			}
			item, err = store.GetTask(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			activity := &TaskActivity{ID: "receipt-" + scenario, TaskID: item.ID, AttemptID: &a.ID, ActivityType: "inspection_identity_reconciled", Author: "system", Message: "Restored identity only", CreatedAt: time.Now().UTC()}
			switch scenario {
			case "changed timestamp":
				_, err = conn.ExecContext(ctx, `UPDATE tasks SET updated_at=updated_at+interval '1 second' WHERE id=$1`, item.ID)
			case "changed commit":
				_, err = conn.ExecContext(ctx, `UPDATE tasks SET commit_sha='other' WHERE id=$1`, item.ID)
			case "cancelled":
				_, err = conn.ExecContext(ctx, `UPDATE task_attempts SET cancel_requested=true WHERE id=$1`, a.ID)
			case "new attempt", "active attempt":
				status := "failed"
				if scenario == "active attempt" {
					status = "running"
				}
				_, err = conn.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status,attempt_number) VALUES($1,$2,'runner','agent',$3,3)`, "new-"+scenario, item.ID, status)
			case "pending launch":
				_, err = conn.ExecContext(ctx, `INSERT INTO task_launches(id,task_id,project_id,request_id,attempt_id,owner_id,room_id,source_request_id,agent_id,instructions_hash,state,available_at,expires_at,created_at,updated_at) VALUES($1,$2,'identity-project','request','future','owner','91000000-0000-4000-8000-000000000079','approved','agent','hash','queued',NOW(),NOW()+interval '1 hour',NOW(),NOW())`, "launch-"+scenario, item.ID)
			case "duplicate receipt":
				err = store.CreateActivity(ctx, activity)
			}
			if err != nil {
				t.Fatal(err)
			}
			changed, err := store.RestoreInspectionIdentity(ctx, item, a, proof, activity)
			if scenario == "valid" {
				if err != nil || !changed {
					t.Fatal("restore failed", changed, err)
				}
				saved, _ := store.GetTask(ctx, item.ID)
				if saved.CommitSHA != proof.SourceCommit || saved.Status != TaskStatusBlocked {
					t.Fatal("wrong task mutation")
				}
				events, _ := store.ListActivities(ctx, item.ID)
				if len(events) != 1 {
					t.Fatal("missing audit")
				}
				persisted, _ := store.GetAttempt(ctx, a.ID)
				if persisted.Status != AttemptStatusInterrupted {
					t.Fatal("history changed")
				}
			} else {
				if changed {
					t.Fatal("unsafe restoration")
				}
				if scenario == "duplicate receipt" && err == nil {
					t.Fatal("expected transaction failure")
				}
				saved, _ := store.GetTask(ctx, item.ID)
				if saved.CommitSHA == proof.SourceCommit {
					t.Fatal("transaction did not roll back")
				}
			}
		})
	}
}
