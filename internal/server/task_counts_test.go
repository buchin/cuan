package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestTaskProgressCountsThousandsAndMirrors(t *testing.T) {
	ctx := context.Background()
	store := task.NewMemoryStore()
	old := db.NewMemoryProjectStudioRepository()
	s := &Server{taskSvc: task.NewService(store, nil), projectStudioRepo: old}
	room := db.GenerateUUID()
	for i := 0; i < 7000; i++ {
		status := []string{"done", "review", "doing", "queued", "blocked", "canceled", "cancelled"}[i%7]
		err := store.CreateTask(ctx, &task.Task{ID: fmt.Sprintf("93000000-0000-4000-8000-%012d", i), ProjectID: "large", ProjectIDs: []string{"large", "shared", "large"}, SourceRoomID: room, Status: status})
		if err != nil {
			t.Fatal(err)
		}
	}
	// A mirrored record must not double-count or replace the accepted canonical status.
	if err := old.CreateTask(ctx, &db.ProjectTask{ID: "93000000-0000-4000-8000-000000000000", ProjectID: "large", RoomID: &room, Title: "Mirror", Status: "doing"}); err != nil {
		t.Fatal(err)
	}
	if err := old.CreateTask(ctx, &db.ProjectTask{ID: db.GenerateUUID(), ProjectID: "large", RoomID: &room, Title: "Legacy only", Status: "done"}); err != nil {
		t.Fatal(err)
	}
	counts, err := s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := taskCounts{Total: 7001, Done: 1001, Review: 1000, Active: 1000, Queued: 1000, Failed: 1000, Canceled: 2000}
	if counts.Rooms[room] != want || counts.Projects["large"] != want {
		t.Fatalf("room=%+v project=%+v want=%+v", counts.Rooms[room], counts.Projects["large"], want)
	}
	if c := counts.Projects["shared"]; c.Total != 7000 || c.Done != 1000 || c.Canceled != 2000 {
		t.Fatalf("linked project: %+v", c)
	}
}

func TestTaskProgressCountsPostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_PROGRESS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	u, err := url.Parse(dsn)
	if err != nil || !strings.HasPrefix(u.Path, "/cuan_progress_test_") {
		t.Fatal("dedicated cuan_progress_test_* database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err = db.Run(ctx, database, "up"); err != nil {
		t.Fatal(err)
	}
	// Real production schema, thousands of rows, canonical/legacy overlap and multi-project links.
	_, err = database.ExecContext(ctx, `
 INSERT INTO projects(id,name,repo_url) VALUES('large','Large',''),('shared','Shared','');
 INSERT INTO project_rooms(id,project_id,title) VALUES('91000000-0000-4000-8000-000000000001','large','Progress');
 INSERT INTO tasks(id,project_id,source_room_id,title,instructions,status,priority,assigned_agent)
 SELECT 'progress-'||i,'large','91000000-0000-4000-8000-000000000001'::uuid,'Task','Private body',
 (ARRAY['done','review','doing','queued','blocked','canceled','cancelled'])[1+(i%7)],'normal','agy' FROM generate_series(0,6999) i;
 INSERT INTO task_projects(task_id,project_id) SELECT id,'shared' FROM tasks;
 INSERT INTO project_tasks(id,project_id,room_id,title,status)
 VALUES('92000000-0000-4000-8000-000000000001','large','91000000-0000-4000-8000-000000000001','Legacy','done');
 INSERT INTO tasks(id,project_id,source_room_id,title,instructions,status,priority,assigned_agent)
 VALUES('92000000-0000-4000-8000-000000000001','large','91000000-0000-4000-8000-000000000001','Canonical','Body','review','normal','agy');
 INSERT INTO project_tasks(id,project_id,room_id,title,status)
 VALUES('92000000-0000-4000-8000-000000000002','large','91000000-0000-4000-8000-000000000001','Legacy only','done');`)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{db: database}
	counts, err := s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := taskCounts{Total: 7002, Done: 1001, Review: 1001, Active: 1000, Queued: 1000, Failed: 1000, Canceled: 2000}
	if counts.Rooms["91000000-0000-4000-8000-000000000001"] != want || counts.Projects["large"] != want {
		t.Fatalf("counts: %+v", counts)
	}
	if counts.Projects["shared"].Total != 7000 {
		t.Fatalf("linked project %+v", counts.Projects["shared"])
	}
}
