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
	"github.com/buchin/cuan/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCRMSQLPersistence(t *testing.T) {
	raw := os.Getenv("CUAN_CRM_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("isolated cuan_s25 database required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path != "/cuan_s25" {
		t.Fatal("isolated cuan_s25 required")
	}
	database, err := sql.Open("pgx", raw)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	schema := "crm_test_" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")
	if _, err = database.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer database.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	isolated, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	migration, _ := migrations.FS.ReadFile("000022_contacts_crm.up.sql")
	if _, err = isolated.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	memoryMigration, _ := migrations.FS.ReadFile("000024_contact_memory.up.sql")
	if _, err = isolated.ExecContext(ctx, string(memoryMigration)); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: isolated}
	c, err := s.crmUpsert(ctx, "short", "one", "Test", "test@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.crmUpsert(ctx, "short", "one", "", "")
	if err != nil || c.ID != again.ID || again.Email != c.Email {
		t.Fatal("reconnect lost persisted identity", err)
	}
	other, err := s.crmUpsert(ctx, "other", "one", "", c.Email)
	if err != nil || c.ID == other.ID {
		t.Fatal("email merged", err)
	}
	if _, err = isolated.ExecContext(ctx, `UPDATE crm_contacts SET tags='["VIP"]' WHERE id=$1`, c.ID); err != nil {
		t.Fatal(err)
	}
	restarted := &Server{db: isolated}
	sess, err := restarted.crmSession(ctx, c.SessionUUID)
	if err != nil || sess.Email != c.Email {
		t.Fatal("restart lost identity", err)
	}
	list, err := restarted.crmList(ctx)
	if err != nil || len(list) != 2 {
		t.Fatal("list persistence", err)
	}
	found := false
	for _, v := range list {
		if v.ID == c.ID && len(v.Tags) == 1 && v.Tags[0] == "VIP" {
			found = true
		}
	}
	if !found {
		t.Fatal("tag persistence failed")
	}
	now := time.Now().UTC()
	note := ContactMemory{ID: db.GenerateUUID(), ContactID: c.ID, Kind: "fact", Body: "Child likes dinosaurs", RecordedBy: "You", CreatedAt: now, UpdatedAt: now}
	if _, err = s.memorySave(ctx, note); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.memorySave(ctx, note); err != nil {
		t.Fatal("idempotent restart", err)
	}
	hits, err := restarted.searchContactMemory(ctx, "", "dinosaur")
	if err != nil || len(hits) != 1 || hits[0].ContactID != c.ID {
		t.Fatal("persisted search", hits, err)
	}
	scoped, err := restarted.memoryList(ctx, c.ID, "dinosaur", 0)
	if err != nil || len(scoped) != 1 {
		t.Fatal("persisted notes", err)
	}
	if _, err = isolated.ExecContext(ctx, `UPDATE contact_memory SET deleted_at=NOW(),body='[deleted]' WHERE id=$1`, note.ID); err != nil {
		t.Fatal(err)
	}
	hits, err = restarted.searchContactMemory(ctx, "", "dinosaur")
	if err != nil || len(hits) != 0 {
		t.Fatal("deleted note recall", err)
	}
	if _, err = restarted.memorySave(ctx, note); err == nil {
		t.Fatal("deleted note resurrected")
	}
	t.Log("Actual PostgreSQL: additive schema, reconnect, separate email identities, fresh server lookup, tags; isolated schema removed")
}
