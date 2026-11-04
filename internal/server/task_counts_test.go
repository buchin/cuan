package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

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

	// Cancellation changes the presentation, not the persisted review state.
	_, err = database.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,status,runner_id,agent_name,created_at) VALUES('canceled-review','progress-1','canceled','fixture','agy',NOW()),('accepted-cancel','progress-0','canceled','fixture','agy',NOW())`)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := changed.Projects["large"]; c.Canceled != 2001 || c.Review != 1000 || c.Done != 1001 {
		t.Fatalf("canceled review counts %+v", c)
	}
	var saved string
	if err = database.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id='progress-1'`).Scan(&saved); err != nil || saved != "review" {
		t.Fatalf("history changed: %s %v", saved, err)
	}
	_, err = database.ExecContext(ctx, `INSERT INTO task_launches(id,task_id,project_id,request_id,attempt_id,owner_id,room_id,source_request_id,agent_id,instructions_hash,state,available_at,expires_at,created_at,updated_at) VALUES('resume-launch','progress-1','large','resume-request','','fixture','91000000-0000-4000-8000-000000000001','source-request','agy','fixture','queued',NOW(),NOW()+INTERVAL '1 hour',NOW(),NOW())`)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := queued.Projects["large"]; c.Canceled != 2000 || c.Queued != 1001 || c.Review != 1000 {
		t.Fatalf("queued resume counts %+v", c)
	}
	if _, err = database.ExecContext(ctx, `UPDATE task_launches SET state='started' WHERE id='resume-launch'`); err != nil {
		t.Fatal(err)
	}

	_, err = database.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,status,runner_id,agent_name,created_at) VALUES('resumed-review','progress-1','succeeded','fixture','agy',NOW()+INTERVAL '1 minute')`)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := resumed.Projects["large"]; c.Canceled != 2000 || c.Review != 1001 || c.Done != 1001 {
		t.Fatalf("resumed counts %+v", c)
	}
	if counts.Projects["shared"].Total != 7000 {
		t.Fatalf("linked project %+v", counts.Projects["shared"])
	}
}

func TestTaskProgressCanceledReviewIsNotPendingReview(t *testing.T) {
	ctx := context.Background()
	store := task.NewMemoryStore()
	s := &Server{taskSvc: task.NewService(store, nil)}
	cases := []struct{ status, attempt, launch, want string }{
		{"review", "canceled", "", "canceled"},
		{"blocked", "canceled", "stopped", "canceled"},
		{"done", "canceled", "stopped", "done"},
		{"review", "succeeded", "", "review"},
		{"review", "canceled", "queued", "queued"},
		{"blocked", "queued", "", "queued"},
		{"review", "failed", "", "blocked"},
		{"doing", "running", "started", "doing"},
	}
	for i, c := range cases {
		if got := taskCountStatus(c.status, c.attempt, c.launch); got != c.want {
			t.Errorf("%+v => %s", c, got)
		}
		id := fmt.Sprintf("cancel-%d", i)
		if err := store.CreateTask(ctx, &task.Task{ID: id, ProjectID: "p", SourceRoomID: "room", Status: c.status}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateAttempt(ctx, &task.TaskAttempt{ID: id + "-attempt", TaskID: id, Status: c.attempt}); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// No launch records above: cancellation stays excluded, even when saved as review.
	if c := counts.Rooms["room"]; c.Total != 8 || c.Done != 1 || c.Canceled != 3 || c.Review != 1 || c.Queued != 1 || c.Failed != 1 || c.Active != 1 {
		t.Fatalf("counts %+v", c)
	}
	// A newer successful attempt supersedes the canceled attempt without accepting it.
	if err := store.CreateAttempt(ctx, &task.TaskAttempt{ID: "replacement", TaskID: "cancel-0", Status: "succeeded", CreatedAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	counts, err = s.readTaskCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := counts.Rooms["room"]; c.Canceled != 2 || c.Review != 2 || c.Done != 1 {
		t.Fatalf("resumed counts %+v", c)
	}
}

func TestInboxTaskCounterCanceledReviewSnapshot(t *testing.T) {
	s, token := setupTestChatServer(t)
	read := func(path string) []byte {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var list struct {
		Contacts []ChatContact `json:"contacts"`
	}
	if err := json.Unmarshal(read("/api/v1/chat/contacts"), &list); err != nil || len(list.Contacts) == 0 {
		t.Fatal("missing contacts", err)
	}
	room := list.Contacts[0].ConversationID
	ctx := context.Background()
	store := s.taskSvc.Store()
	if err := store.CreateTask(ctx, &task.Task{ID: "canceled-review-counter", ProjectID: "default", SourceRoomID: room, Title: "Preserved partial work", Status: "review"}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAttempt(ctx, &task.TaskAttempt{ID: "canceled-review-attempt", TaskID: "canceled-review-counter", Status: "canceled", CancelRequested: true}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(read("/api/v1/chat/contacts"), &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range list.Contacts {
		if c.ConversationID == room {
			found = true
			if c.TaskCount != 1 || c.CanceledTasks != 1 || c.ReviewTasks != 0 {
				t.Fatalf("list counters %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("room absent")
	}
	var snapshot struct {
		Contact ChatContact `json:"contact"`
	}
	if err := json.Unmarshal(read("/api/v1/chat/conversations/"+room+"/messages"), &snapshot); err != nil {
		t.Fatal(err)
	}
	if c := snapshot.Contact; c.TaskCount != 1 || c.CanceledTasks != 1 || c.ReviewTasks != 0 {
		t.Fatalf("snapshot counters %+v", c)
	}
	saved, err := store.GetTask(ctx, "canceled-review-counter")
	if err != nil || saved.Status != "review" {
		t.Fatalf("saved review changed: %+v %v", saved, err)
	}
}
