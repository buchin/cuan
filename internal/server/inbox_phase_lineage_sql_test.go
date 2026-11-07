package server

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func withPhasePostgres(t *testing.T, check func(*sql.DB)) {
	t.Helper()
	dsn := os.Getenv("CUAN_LIFECYCLE_SQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL DSN")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Path != "/cuan_s25" && !strings.HasPrefix(parsed.Path, "/cuan_lifecycle_test_")) {
		t.Fatal("isolated cuan_s25 or disposable cuan_lifecycle_test_* database required")
	}
	ctx := context.Background()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err = admin.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS "pgcrypto" WITH SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	schema := "phase_test_" + strings.ReplaceAll(db.GenerateUUID(), "-", "")
	if _, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	query := parsed.Query()
	query.Set("search_path", schema+",public")
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
	check(database)
}

func TestApprovedPhasePostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		ctx := context.Background()
		s, q, parent, child, launch := phaseLineageFixture(t, database)
		if len(child.DoDChecklist) != 1 || child.DoDChecklist[0].Text != "API" || child.DoDChecklist[0].Completed || child.DoDChecklist[0].Evidence != "" {
			t.Fatal("persisted next phase lost fresh acceptance", child.DoDChecklist)
		}
		restarted := &Server{taskSvc: s.taskSvc, projectStudioRepo: s.projectStudioRepo, projectSvc: s.projectSvc}
		_, authorized, err := restarted.authorizeSavedTaskLaunch(ctx, *launch, child)
		if err != nil || !authorized {
			t.Fatal("Postgres phase approval failed", err)
		}
		original := append([]task.DoDItem(nil), child.DoDChecklist...)
		for _, criteria := range [][]task.DoDItem{nil, {{Text: "Unapproved outcome"}}} {
			child.DoDChecklist = criteria
			if err := s.taskSvc.Store().UpdateTask(ctx, &child.Task); err != nil {
				t.Fatal(err)
			}
			saved, err := s.taskSvc.GetTaskDetail(ctx, child.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, allowed, err := restarted.authorizeSavedTaskLaunch(ctx, *launch, saved); err != nil || allowed {
				t.Fatal("changed persisted phase acceptance authorized", allowed, err)
			}
		}
		child.DoDChecklist = original
		if err := s.taskSvc.Store().UpdateTask(ctx, &child.Task); err != nil {
			t.Fatal(err)
		}
		_ = s.enqueueInboxAutoDelegation(ctx, q, parent, "Phase accepted")
		var count int
		if err = database.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE source_room_id=$1`, q.RoomID).Scan(&count); err != nil || count != 2 {
			t.Fatal("phase replay duplicated tasks", count, err)
		}

	})
}

func TestApprovedPhaseRecoveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { phaseHandoffRecoveryContract(t, database) })
}

func TestApprovedPhaseProvenancePostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, origin, parent := phaseLineageSetup(t, database)
		s.inboxStopping.Store(true)
		ctx := context.Background()
		older := &task.TaskAttempt{ID: "att-" + db.GenerateUUID(), TaskID: parent.ID, RunnerID: "fixture", AgentName: origin.AgentID, Status: task.AttemptStatusSucceeded, CreatedAt: parent.LatestAttempt.CreatedAt.Add(-time.Hour)}
		if err := s.taskSvc.Store().CreateAttempt(ctx, older); err != nil {
			t.Fatal(err)
		}
		if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "act-" + db.GenerateUUID(), TaskID: parent.ID, AttemptID: &older.ID, ActivityType: "agent_report", NewStatus: "review", Author: origin.AgentID, Message: "Earlier verification, not the current attempt", CreatedAt: older.CreatedAt}); err != nil {
			t.Fatal(err)
		}
		row, err := s.taskSvc.Store().GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		row.Status = task.TaskStatusReview
		if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
			t.Fatal(err)
		}
		parent, err = s.taskSvc.GetTaskDetail(ctx, parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		key := "task-review:" + parent.LatestAttempt.ID
		old := origin
		old.RequestID = delegationMessageID(key)
		old.MessageID = delegationMessageID(key + ":input")
		old.ReplyID = delegationMessageID(key + ":reply")
		old.PayloadHash = old.RequestID
		old.State = "completed"
		old.CreatedAt = time.Now().UTC()
		repo := s.projectStudioRepo.(db.InboxRequestRepository)
		if _, _, err := repo.AcceptInboxRequest(ctx, old, &db.ProjectMessage{ID: old.MessageID, RoomID: old.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + parent.ID + "]\nReview evidence digest: " + inboxReviewEvidenceDigest(parent), CreatedAt: old.CreatedAt}); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			s.queueInboxTaskReview(ctx, parent)
		}
		requests, err := repo.ListInboxRequests(ctx, origin.RoomID)
		if err != nil || len(requests) != 3 {
			t.Fatal("SQL provenance review duplicated or missing", len(requests), err)
		}
		freshID := delegationMessageID(key + ":report-provenance:" + inboxReviewEvidenceDigest(parent))
		var fresh *db.InboxRequest
		for i := range requests {
			if requests[i].RequestID == freshID {
				fresh = &requests[i]
			}
		}
		if fresh == nil || s.inboxReviewedTask(ctx, *fresh) == nil {
			t.Fatal("SQL review lost canonical task binding")
		}
		after, err := s.taskSvc.GetTaskDetail(ctx, parent.ID)
		if err != nil || len(after.Attempts) != 2 || after.LatestAttempt.ID != parent.LatestAttempt.ID {
			t.Fatal("SQL provenance reconciliation created an execution attempt", err)
		}
	})
}
