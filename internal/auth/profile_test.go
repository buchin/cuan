package auth

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
	"testing"
)

func TestUsernamePersistsWithoutChangingSessions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{`CREATE TABLE owners (id TEXT PRIMARY KEY,email TEXT,username TEXT UNIQUE,password_hash TEXT,is_setup BOOLEAN,created_at DATETIME,updated_at DATETIME)`, `CREATE TABLE owner_sessions(token_hash TEXT PRIMARY KEY,owner_id TEXT,expires_at DATETIME,created_at DATETIME)`} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(NewDBStore(db))
	ctx := context.Background()
	if err = svc.AutoSeedDefaultAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := svc.Login(ctx, "admin", "whosyourdaddy")
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.UpdateUsername(ctx, before.Owner.ID, "new-admin", "whosyourdaddy"); err != nil {
		t.Fatal(err)
	}
	after, err := svc.ValidateToken(ctx, before.Token)
	if err != nil || after.ID != before.Owner.ID || after.Username != "new-admin" {
		t.Fatal(after, err)
	}
	if _, err = svc.Login(ctx, "new-admin", "whosyourdaddy"); err != nil {
		t.Fatal(err)
	}
}
