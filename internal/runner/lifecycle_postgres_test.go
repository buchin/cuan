package runner

import (
	"context"
	"database/sql"
	"errors"
	database "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecutionLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_EXECUTION_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Hostname() != "127.0.0.1" || !strings.HasPrefix(u.Path, "/cuan_execution_") {
		t.Fatal("dedicated loopback cuan_execution_* database required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err = database.Run(ctx, db, "up"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('execution-project','Execution fixture','')`); err != nil {
		t.Fatal(err)
	}
	ts := task.NewDBStore(db)
	rs := NewDBStore(db)
	svc := NewService(rs, ts)
	now := time.Now().UTC()
	item := &task.Task{ID: "sql-execution", ProjectID: "execution-project", Title: "Scoped change", Instructions: "Check output", Status: task.TaskStatusQueued, Priority: task.PriorityNormal, Summary: "Scoped work complete", CompleteNote: "Fixture output verified", DoDChecklist: []task.DoDItem{{Text: "Output", Completed: true, Evidence: "Actual fixture output matched"}}, CreatedAt: now, UpdatedAt: now}
	if err = ts.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	a := &task.TaskAttempt{ID: "sql-execution-attempt", TaskID: item.ID, AgentName: "agy", Status: task.AttemptStatusQueued, AttemptNumber: 1, CreatedAt: now}
	if err = ts.CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = rs.RegisterOrUpdateHeartbeat(ctx, &RunnerInfo{ID: "sql-runner", Status: StatusIdle, SupportedAgents: []string{"agy"}}); err != nil {
		t.Fatal(err)
	}
	claim, err := svc.Claim(ctx, ClaimTaskRequest{RunnerID: "sql-runner", SupportedAgents: []string{"agy"}}, time.Minute)
	if err != nil || claim.Attempt == nil {
		t.Fatalf("claim: %+v %v", claim, err)
	}
	if claim.Attempt.ExecutionDeadline == nil || claim.Attempt.StartedAt == nil || claim.Attempt.ExecutionDeadline.Sub(*claim.Attempt.StartedAt) != 25*time.Minute {
		t.Fatalf("missing shared deadline: %+v", claim.Attempt)
	}
	persisted, err := ts.GetAttempt(ctx, a.ID)
	if err != nil || persisted.ExecutionDeadline == nil {
		t.Fatalf("deadline persistence: %+v %v", persisted, err)
	}
	if _, err = rs.RenewLease(ctx, a.ID, "foreign", time.Minute); !errors.Is(err, task.ErrAttemptFinal) {
		t.Fatalf("foreign heartbeat accepted: %v", err)
	}
	summary := "Scoped work checked in SQL transaction"
	if _, err = task.NewService(ts, nil).ReportAgent(ctx, item.ID, a.ID, "agy", task.AgentReport{State: "progress", Summary: &summary, Progress: "Validated fixture output"}); err != nil {
		t.Fatal(err)
	}
	req := lifecycleResult()
	req.RunnerID = "sql-runner"
	req.TestSummary = "Project verification: SQL fixture passed"
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			other := NewService(NewDBStore(db), task.NewDBStore(db))
			_, e := other.Complete(ctx, a.ID, req)
			results <- e
		}()
	}
	accepted := 0
	for i := 0; i < 2; i++ {
		e := <-results
		if e == nil {
			accepted++
		} else if !errors.Is(e, task.ErrAttemptFinal) && !errors.Is(e, task.ErrRequestConflict) {
			t.Fatal(e)
		}
	}
	if accepted != 1 {
		t.Fatalf("concurrent SQL completions accepted %d times", accepted)
	}
	activities, err := ts.ListActivities(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	evidence := task.AttemptVerificationEvidence(&task.TaskDetailResponse{Task: *item, LatestAttempt: a, Activities: activities})
	if evidence.Current != req.TestSummary || evidence.AttemptID != a.ID {
		t.Fatalf("SQL completion lost bound verification: %+v", evidence)
	}
	verificationCount := 0
	for _, activity := range activities {
		if activity.ActivityType == task.ActivityTypeRunnerVerification {
			verificationCount++
		}
	}
	if verificationCount != 1 {
		t.Fatalf("SQL completion persisted %d verification records", verificationCount)
	}
	after, err := ts.GetTask(ctx, item.ID)
	if err != nil || after.Status != task.TaskStatusDone || after.DoDChecklist[0].Evidence == "" {
		t.Fatalf("atomic completion: %+v %v", after, err)
	}
	if _, err = svc.Complete(ctx, a.ID, req); !errors.Is(err, task.ErrAttemptFinal) {
		t.Fatalf("duplicate completion accepted: %v", err)
	}
	if _, err = rs.RenewLease(ctx, a.ID, req.RunnerID, time.Minute); !errors.Is(err, task.ErrAttemptFinal) {
		t.Fatalf("ended lease revived: %v", err)
	}
	// Expired ownership must not let a delayed success write a task result.
	second := &task.Task{ID: "sql-expired", ProjectID: item.ProjectID, Title: "Expired", Instructions: "Preserve state", Status: task.TaskStatusDoing, Priority: task.PriorityNormal, CreatedAt: now, UpdatedAt: now}
	if err = ts.CreateTask(ctx, second); err != nil {
		t.Fatal(err)
	}
	expiry, deadline := now.Add(-time.Minute), now.Add(time.Minute)
	expired := &task.TaskAttempt{ID: "sql-expired-attempt", TaskID: second.ID, AgentName: "agy", RunnerID: "sql-runner", Status: task.AttemptStatusRunning, AttemptNumber: 1, CreatedAt: now, LeaseExpiresAt: &expiry, ExecutionDeadline: &deadline}
	if err = ts.CreateAttempt(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Complete(ctx, expired.ID, req); !errors.Is(err, task.ErrAttemptFinal) {
		t.Fatalf("expired completion: %v", err)
	}
	if _, err = rs.MarkInterruptedLeases(ctx); err != nil {
		t.Fatal(err)
	}
	blocked, _ := ts.GetTask(ctx, second.ID)
	if blocked.Status != task.TaskStatusBlocked {
		t.Fatalf("expired lease not blocked: %s", blocked.Status)
	}
	// Migration is additive and reversible without changing task/attempt history.
	if _, err = db.ExecContext(ctx, `ALTER TABLE task_attempts DROP COLUMN execution_deadline; ALTER TABLE task_attempts ADD COLUMN execution_deadline TIMESTAMPTZ`); err != nil {
		t.Fatal(err)
	}
	legacy, err := ts.GetAttempt(ctx, a.ID)
	if err != nil || legacy.Status != task.AttemptStatusSucceeded {
		t.Fatalf("legacy row lost: %+v %v", legacy, err)
	}
}
