package diagnostics

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

func TestProjectGuidanceMCPPostgres(t *testing.T) {
	dsn := os.Getenv("CUAN_GUIDANCE_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated guidance fixture not configured")
	}
	if !strings.Contains(dsn, "127.0.0.1") || !strings.Contains(dsn, "cuan_guidance_fixture") {
		t.Fatal("only isolated local fixture permitted")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	_, err = database.ExecContext(ctx, `CREATE TABLE projects(id text PRIMARY KEY,name text,repo_url text,default_branch text,metadata jsonb); CREATE TABLE tasks(id text PRIMARY KEY,project_id text,title text,instructions text,status text,priority text,dod_checklist jsonb,created_at timestamptz,updated_at timestamptz); INSERT INTO projects VALUES('fixture-project','Fixture','https://github.com/buchin/cuan','dev','{"cuan_settings":{"goal":"original outcome","instructions":"standing rules"}}')`)
	if err != nil {
		t.Fatal(err)
	}
	platformDBOnce.Do(func() {})
	previous := platformDB
	platformDB = database
	defer func() { platformDB = previous }()
	result, err := CreateTaskDraft(ctx, "Fixture", "Scoped work", "approved request", "normal", nil)
	if err != nil {
		t.Fatal(err)
	}
	id := result.(map[string]any)["id"].(string)
	var instructions string
	if err := database.QueryRowContext(ctx, "SELECT instructions FROM tasks WHERE id=$1", id).Scan(&instructions); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(instructions, "original outcome") || !strings.Contains(instructions, "standing rules") || !strings.Contains(instructions, "approved request") {
		t.Fatal("MCP task lost its project guidance")
	}
	if _, err := database.ExecContext(ctx, `UPDATE projects SET metadata='{"cuan_settings":{"goal":"new outcome","instructions":"new rules"}}'`); err != nil {
		t.Fatal(err)
	}
	var original string
	database.QueryRowContext(ctx, "SELECT instructions FROM tasks WHERE id=$1", id).Scan(&original)
	if original != instructions {
		t.Fatal("existing task snapshot was changed")
	}
	result, err = CreateTaskDraft(ctx, "fixture-project", "Next work", "new request", "normal", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.(map[string]any)["instructions"].(string), "new outcome") {
		t.Fatal("new MCP draft missed updated guidance")
	}
}
