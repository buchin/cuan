package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSeparatedDefaultRetainsHistoricalWorkspace(t *testing.T) {
	store := NewMemoryStore()
	secret := "fixture-key"
	source := map[string]any{"cuan_previous_project_ids": []string{"default"}}
	binding, _ := crypto.Encrypt(`{"project_id":"default","revision":"initial","binding":{"local_path":"/retained/task-workspace","repo_url":"https://example.invalid/original","branch":"main"}}`, secret)
	source["cuan_workspace_versions"] = map[string]string{"initial": binding}
	raw, _ := json.Marshal(source)
	p := &Project{ID: "proj_legacy", Name: "Cuan", RepoURL: "https://example.invalid/new", LocalPath: "/new", Metadata: json.RawMessage(`{"cuan_settings":{"workspace_revision":"next"}}`), SourceMetadata: raw}
	if err := store.CreateProject(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	svc := NewService(store, nil, secret, t.TempDir())
	old := SnapshotProjectGuidance("Original approval", &ProjectResponse{ID: "default", RepoURL: "https://example.invalid/original"})
	got, err := svc.TaskWorkspace(context.Background(), p.ID, old)
	if err != nil || got.LocalPath != "/retained/task-workspace" || got.RepoURL != "https://example.invalid/original" {
		t.Fatal("historical binding abandoned", got, err)
	}
	// An unrelated encrypted identity cannot use this migration's compatibility.
	bad, _ := crypto.Encrypt(`{"project_id":"unrelated","revision":"initial","binding":{"local_path":"/wrong"}}`, secret)
	source["cuan_workspace_versions"] = map[string]string{"initial": bad}
	p.SourceMetadata, _ = json.Marshal(source)
	_ = store.UpdateProject(context.Background(), p)
	if _, err = svc.TaskWorkspace(context.Background(), p.ID, old); err == nil {
		t.Fatal("unrelated binding accepted")
	}
}
func TestNormalizeDefaultPostgresRetainsHistory(t *testing.T) {
	dsn := os.Getenv("CUAN_DEFAULT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires empty cuan_default_test_* database")
	}
	u, err := url.Parse(dsn)
	if err != nil || !strings.HasPrefix(u.Path, "/cuan_default_test_") {
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
	store := NewDBStore(database)
	original := &Project{ID: "default", Name: "cuan", RepoURL: "https://example.invalid/cuan", LocalPath: filepath.Join(t.TempDir(), "existing"), EncryptedToken: "retained-ciphertext", Metadata: json.RawMessage(`{"keep":true}`), SourceMetadata: json.RawMessage(`{"keep":"source"}`), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = store.CreateProject(ctx, original); err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO tasks(id,project_id,title,instructions,creation_hash) VALUES('task_old','default','Retained task','Original immutable request','unchanged-hash')`,
		`INSERT INTO task_projects(task_id,project_id) VALUES('task_old','default')`,
		`INSERT INTO project_rooms(id,project_id,title) VALUES('a0000000-0000-4000-8000-000000000001','default','Retained conversation')`,
		`INSERT INTO project_messages(id,room_id,content) VALUES('a0000000-0000-4000-8000-000000000002','a0000000-0000-4000-8000-000000000001','Original approval')`,
		`CREATE TABLE optional_module_history(id text primary key,project_id text REFERENCES projects(id),content text)`,
		`INSERT INTO optional_module_history VALUES('history','default','Retained optional module state')`,
	}
	for _, q := range statements {
		if _, err = database.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := store.GetProject(ctx, "default")
	preview, err := NormalizeDefaultProject(ctx, database, false)
	if err != nil || !preview.Changed || preview.Applied {
		t.Fatal(preview, err)
	}
	same, _ := store.GetProject(ctx, "default")
	if same.Name != before.Name {
		t.Fatal("preview mutated project")
	}
	if _, err = database.Exec(`INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status) VALUES('active','task_old','fixture','fixture','running')`); err != nil {
		t.Fatal(err)
	}
	if _, err = NormalizeDefaultProject(ctx, database, true); err == nil {
		t.Fatal("active work moved")
	}
	database.Exec(`DELETE FROM task_attempts WHERE id='active'`)
	if _, err = database.Exec(`ALTER TABLE optional_module_history ADD CONSTRAINT preserve_fixture CHECK(project_id='default')`); err != nil {
		t.Fatal(err)
	}
	if _, err = NormalizeDefaultProject(ctx, database, true); err == nil {
		t.Fatal("conflicting module history was overwritten")
	}
	unchanged, _ := store.GetProject(ctx, "default")
	var originalCount int
	database.QueryRow(`SELECT count(*) FROM projects`).Scan(&originalCount)
	if unchanged.Name != before.Name || originalCount != 1 {
		t.Fatal("failed separation was not rolled back")
	}
	database.Exec(`ALTER TABLE optional_module_history DROP CONSTRAINT preserve_fixture`)
	done, err := NormalizeDefaultProject(ctx, database, true)
	if err != nil || !done.Applied || done.PreservedProjectID != preview.PreservedProjectID {
		t.Fatal(done, err)
	}
	neutral, err := store.GetProject(ctx, "default")
	if err != nil || neutral.Name != "Default" || neutral.RepoURL != "" || neutral.LocalPath != "" || neutral.EncryptedToken != "" {
		t.Fatal("Default is not neutral", err)
	}
	kept, err := store.GetProject(ctx, done.PreservedProjectID)
	if err != nil || kept.Name != before.Name || kept.RepoURL != before.RepoURL || kept.LocalPath != before.LocalPath || kept.EncryptedToken != before.EncryptedToken || string(kept.Metadata) != string(before.Metadata) {
		t.Fatal("original configuration lost", err)
	}
	for _, table := range []string{"tasks", "task_projects", "project_rooms", "optional_module_history"} {
		var id string
		if err = database.QueryRow("SELECT project_id FROM " + table + " LIMIT 1").Scan(&id); err != nil || id != done.PreservedProjectID {
			t.Fatal("history not moved", table, err)
		}
	}
	var instructions, hash, content string
	database.QueryRow(`SELECT instructions,creation_hash FROM tasks WHERE id='task_old'`).Scan(&instructions, &hash)
	database.QueryRow(`SELECT content FROM project_messages LIMIT 1`).Scan(&content)
	if instructions != "Original immutable request" || hash != "unchanged-hash" || content != "Original approval" {
		t.Fatal("original request or approval was rewritten")
	}
	replay, err := NormalizeDefaultProject(ctx, database, true)
	if err != nil || replay.Changed || replay.Applied {
		t.Fatal("repeat was not a no-op", replay, err)
	}
	var count int
	database.QueryRow(`SELECT count(*) FROM projects`).Scan(&count)
	if count != 2 {
		t.Fatal("duplicate preserved projects", count)
	}
}
