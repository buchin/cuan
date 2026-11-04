package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTaskPageMemoryContract(t *testing.T) { taskPageContract(t, NewMemoryStore(), "page-project") }
func taskPageContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	svc := NewService(store, nil)
	start := time.Now().UTC().Add(-time.Hour)
	const total = 74
	for i := 0; i < total; i++ {
		state := []string{"todo", "doing", "done", "review", "blocked", "backlog", "queued"}[i%7]
		at := start.Add(time.Duration(i) * time.Second)
		task := &Task{ID: fmt.Sprintf("page_fixture_%04d", i), ProjectID: project, Title: fmt.Sprintf("Work %04d", i), Instructions: "Approved scope.\n\nProject guidance snapshot:\n```cuan-project-context\nprivate-environment-snapshot", Status: state, Priority: "normal", CreatedAt: at, UpdatedAt: at, Summary: strings.Repeat("Result ", 100), CompleteNote: strings.Repeat("Evidence ", 100), DoDChecklist: []DoDItem{{Text: "Original criterion"}}}
		if i%2 == 0 {
			task.Priority = "high"
		}
		if err := store.CreateTask(ctx, task); err != nil {
			t.Fatal(err)
		}
		if i%7 == 1 {
			if err := store.CreateAttempt(ctx, &TaskAttempt{ID: task.ID + "_attempt", TaskID: task.ID, AgentName: "agent", RunnerID: "fixture", Status: "running", CreatedAt: at}); err != nil {
				t.Fatal(err)
			}
		}
	}
	get := func(o PageOptions) *TaskPage {
		t.Helper()
		o.ProjectID = project
		p, err := svc.ListTaskPage(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := get(PageOptions{PageSize: 25})
	if len(first.Tasks) != 25 || first.Total != total || first.ScopeTotal != total || first.Counts["all"] != total {
		t.Fatalf("incorrect bounded page: rows%d total%d scope%d", len(first.Tasks), first.Total, first.ScopeTotal)
	}
	if first.Tasks[0].ID != "page_fixture_0073" {
		t.Fatal("default order", first.Tasks[0].ID)
	}
	seen := map[string]bool{}
	for page := 1; page <= 3; page++ {
		p := get(PageOptions{Page: page, PageSize: 25})
		for _, row := range p.Tasks {
			if seen[row.ID] {
				t.Fatal("duplicate across pages", row.ID)
			}
			seen[row.ID] = true
			if strings.Contains(row.Instructions, "private-environment") || len([]rune(row.Summary)) > 241 || len(row.DoDChecklist) != 0 {
				t.Fatal("board payload retained detail/snapshot")
			}
		}
	}
	if len(seen) != total {
		t.Fatal("tasks omitted", len(seen))
	}
	last := get(PageOptions{Page: 999, PageSize: 25})
	if last.Page != 3 || len(last.Tasks) != 24 {
		t.Fatal("stale page not clamped", last.Page, len(last.Tasks))
	}
	match := get(PageOptions{Search: "Work 0001"})
	if match.Total != 1 || match.Tasks[0].ID != "page_fixture_0001" || match.Counts["all"] != total {
		t.Fatal("search/totals incorrect")
	}
	high := get(PageOptions{Priority: "high", Sort: "title"})
	if high.Total != 37 || high.Tasks[0].Title != "Work 0000" {
		t.Fatal("priority/order incorrect")
	}
	status := get(PageOptions{Status: "done"})
	if status.Total != 11 || status.Counts["done"] != 11 {
		t.Fatal("status counts incorrect", status.Total, status.Counts)
	}
	active := get(PageOptions{Status: "active"})
	if active.Total != first.Counts["active"] {
		t.Fatal("active counts/page disagree")
	}
	percent := get(PageOptions{Search: "%"})
	if percent.Total != 0 {
		t.Fatal("search wildcard was not literal")
	}
	none, err := svc.ListTaskPage(ctx, PageOptions{ProjectID: "missing", PageSize: 25})
	if err != nil || none.Total != 0 || len(none.Tasks) != 0 {
		t.Fatal("empty scope failed", err)
	}
	// The ordinary detail response must retain the original task data.
	full, err := svc.GetTaskDetail(ctx, first.Tasks[0].ID)
	if err != nil || !strings.Contains(full.Instructions, "private-environment") || len(full.DoDChecklist) != 1 {
		t.Fatal("page mutated stored detail", err)
	}

	for i, c := range []struct{ status, attempt, kind string }{{"review", "succeeded", "answer"}, {"doing", "failed", ""}, {"review", "canceled", ""}, {"done", "failed", ""}, {"queued", "", ""}} {
		id := fmt.Sprintf("group_fixture_%d", i)
		at := time.Now().UTC().Truncate(time.Microsecond)
		if err := store.CreateTask(ctx, &Task{ID: id, ProjectID: project, Title: fmt.Sprintf("Group fixture %d", i), Instructions: "Scope", Status: c.status, Priority: "normal", CreatedAt: at, UpdatedAt: at}); err != nil {
			t.Fatal(err)
		}
		if c.attempt != "" {
			attempt := &TaskAttempt{ID: id + "_attempt", TaskID: id, RunnerID: "fixture", AgentName: "agent", Status: c.attempt, CreatedAt: at}
			if err := store.CreateAttempt(ctx, attempt); err != nil {
				t.Fatal(err)
			}
			if c.kind != "" {
				for _, activity := range []*TaskActivity{{ID: id + "_report", TaskID: id, AttemptID: &attempt.ID, ActivityType: "agent_report", NewStatus: "review", Author: "fixture", CreatedAt: at}, {ID: id + "_kind", TaskID: id, AttemptID: &attempt.ID, ActivityType: "agent_result_kind", NewStatus: c.kind, Author: "fixture", CreatedAt: at}} {
					if err := store.CreateActivity(ctx, activity); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	for status, want := range map[string]int{"completed": 2, "attention": 1, "work_active": 1, "active": 1, "dibatalkan_review": 3} {
		page := get(PageOptions{Search: "Group fixture", Status: status})
		if page.Total != want {
			t.Fatalf("group %s has %d tasks; want %d", status, page.Total, want)
		}
	}
}

type batchAttemptCountingStore struct {
	*MemoryStore
	single, batch int
}

func (s *batchAttemptCountingStore) GetLatestAttempt(ctx context.Context, id string) (*TaskAttempt, error) {
	s.single++
	return s.MemoryStore.GetLatestAttempt(ctx, id)
}
func (s *batchAttemptCountingStore) LatestAttempts(ctx context.Context, ids []string) (map[string]*TaskAttempt, error) {
	s.batch++
	return s.MemoryStore.LatestAttempts(ctx, ids)
}
func TestTaskListBatchesLatestAttempts(t *testing.T) {
	store := &batchAttemptCountingStore{MemoryStore: NewMemoryStore()}
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("batch_%d", i)
		if err := store.CreateTask(ctx, &Task{ID: id, Title: id, Status: "todo", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := NewService(store, nil).ListTasks(ctx, TaskFilter{})
	if err != nil || len(rows) != 1000 {
		t.Fatal("list failed", err)
	}
	if store.single != 0 || store.batch != 1 {
		t.Fatalf("N+1 latest attempt reads: single%d batch%d", store.single, store.batch)
	}
}

func taskPageScalePostgres(t *testing.T, database *sql.DB, store Store) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('page-scale','Scale fixture','');
 INSERT INTO tasks(id,project_id,title,instructions,status,priority,created_at,updated_at)
 SELECT 'scale_'||n,'page-scale','Work '||n,repeat('scope ',1000),'todo','normal',NOW(),NOW() FROM generate_series(1,7000)n;
 INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status,created_at)
 SELECT 'scale_attempt_'||n,'scale_'||n,'fixture','agent','succeeded',NOW() FROM generate_series(1,7000)n;`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	page, err := NewService(store, nil).ListTaskPage(ctx, PageOptions{ProjectID: "page-scale", PageSize: 25, Page: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tasks) != 25 || page.Total != 7000 || page.Counts["all"] != 7000 {
		t.Fatal("scale query lost boundedness/counts")
	}
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 100000 {
		t.Fatalf("25-row board payload too large: %d", len(raw))
	}
	t.Logf("7000-task PostgreSQL page: rows=%d total=%d JSON=%d bytes elapsed=%s", len(page.Tasks), page.Total, len(raw), time.Since(started))
}
