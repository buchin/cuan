package project

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// DefaultNormalization is safe to print: it contains no repository credentials,
// paths, conversation text or encrypted workspace bindings.
type DefaultNormalization struct {
	Changed            bool             `json:"changed"`
	Applied            bool             `json:"applied"`
	PreservedProjectID string           `json:"preserved_project_id,omitempty"`
	MovedReferences    map[string]int64 `json:"moved_references,omitempty"`
}

// NormalizeDefaultProject is an explicit maintenance operation, never a startup
// rename. Stop application admission and take a private backup before applying.
// Existing project/task/message IDs and all task instructions are retained.
func NormalizeDefaultProject(ctx context.Context, database *sql.DB, apply bool) (DefaultNormalization, error) {
	out := DefaultNormalization{MovedReferences: map[string]int64{}}
	tx, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='2s'; SET LOCAL statement_timeout='20s'`); err != nil {
		return out, err
	}
	// Serialize this operation, and block new work before checking active attempts.
	if _, err = tx.ExecContext(ctx, `LOCK TABLE projects,tasks,task_attempts,inbox_requests,task_launches IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return out, fmt.Errorf("could not lock application work; leave the original project unchanged")
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM task_attempts WHERE status IN ('running','queued'))+(SELECT count(*) FROM inbox_requests WHERE state IN ('running','queued','cancelling'))+(SELECT count(*) FROM task_launches WHERE state IN ('queued','claimed'))`).Scan(&active); err != nil {
		return out, err
	}
	if active != 0 {
		return out, fmt.Errorf("active or waiting work exists; finish it before normalizing Default")
	}
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT to_jsonb(p) FROM projects p WHERE id='default' FOR UPDATE`).Scan(&raw)
	if err == sql.ErrNoRows {
		out.Changed = true
		if apply {
			_, err = tx.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url,default_branch,project_type,metadata,source_metadata,enabled_modules,source_type) VALUES('default','Default','','main','custom','{}','{}','["tasks","discussions"]','blank')`)
			if err != nil {
				return out, err
			}
			err = tx.Commit()
			out.Applied = err == nil
		}
		return out, err
	}
	if err != nil {
		return out, err
	}
	var previous map[string]json.RawMessage
	if err = json.Unmarshal(raw, &previous); err != nil {
		return out, err
	}
	field := func(key string) string { var v string; _ = json.Unmarshal(previous[key], &v); return v }
	if field("name") == "Default" && field("repo_url") == "" && field("encrypted_token") == "" && field("repo_owner") == "" && field("repo_name") == "" {
		return out, nil
	}
	digest := sha256.Sum256(append([]byte("legacy-default:"), raw...))
	target := "proj_" + hex.EncodeToString(digest[:16])
	out.Changed = true
	out.PreservedProjectID = target
	// Discover all relational project references rather than silently dropping an
	// optional module's history. Identifiers come only from the database catalog.
	rows, err := tx.QueryContext(ctx, `SELECT table_name FROM information_schema.columns WHERE table_schema='public' AND column_name='project_id' AND data_type IN ('text','character varying') ORDER BY table_name`)
	if err != nil {
		return out, err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return out, err
		}
		tables = append(tables, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	quote := func(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
	for _, table := range tables {
		if _, err = tx.ExecContext(ctx, "LOCK TABLE "+quote(table)+" IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			return out, err
		}
		var count int64
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+quote(table)+" WHERE project_id='default'").Scan(&count); err != nil {
			return out, err
		}
		if count > 0 {
			out.MovedReferences[table] = count
		}
	}
	if !apply {
		return out, nil
	}
	// Copy the complete row, including optional fields. Historical encrypted
	// workspace records remain byte-identical and are read through a scoped alias.
	previous["id"], _ = json.Marshal(target)
	var source map[string]json.RawMessage
	_ = json.Unmarshal(previous["source_metadata"], &source)
	if source == nil {
		source = map[string]json.RawMessage{}
	}
	source["cuan_previous_project_ids"], _ = json.Marshal([]string{"default"})
	previous["source_metadata"], _ = json.Marshal(source)
	replacement, _ := json.Marshal(previous)
	if _, err = tx.ExecContext(ctx, `INSERT INTO projects SELECT * FROM jsonb_populate_record(NULL::projects,$1::jsonb)`, string(replacement)); err != nil {
		return out, fmt.Errorf("could not preserve the original project; no changes applied")
	}
	for _, table := range tables {
		if _, err = tx.ExecContext(ctx, "UPDATE "+quote(table)+" SET project_id=$1 WHERE project_id='default'", target); err != nil {
			return out, err
		}
	}
	// Keep the existing Default row/FK identity. All old references now point to
	// the complete preserved project; a clean Default has no inherited repo/path.
	if _, err = tx.ExecContext(ctx, `UPDATE projects SET name='Default',repo_url='',default_branch='main',local_path='',encrypted_token='',token_preview='',repo_owner='',repo_name='',clone_status='pending',project_type='custom',metadata='{}',source_metadata='{}',enabled_modules='["tasks","discussions"]',source_type='blank',is_archived=false,last_fetched_at=NULL,last_error='',updated_at=now() WHERE id='default'`); err != nil {
		return out, err
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	out.Applied = true
	return out, nil
}

func historicalProjectID(p *ProjectResponse, id string) bool {
	if id == p.ID {
		return true
	}
	var source struct {
		Previous []string `json:"cuan_previous_project_ids"`
	}
	if json.Unmarshal(p.SourceMetadata, &source) != nil {
		return false
	}
	for _, previous := range source.Previous {
		if previous == id {
			return true
		}
	}
	return false
}
