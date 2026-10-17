package deployer

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestParseSQLStatements(t *testing.T) {
	sqlInput := `
-- Line comment 1
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE
);

/* Multi-line block comment
   spanning several lines
   with a semicolon ; here! */
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    user_id INTEGER,
    title TEXT,
    content TEXT DEFAULT 'Hello; world! Escaped ''single'' quotes; here',
    created_at TIMESTAMP
);

# MySQL style line comment
INSERT INTO users (id, name, email) VALUES (1, 'Buchin', 'buchin@example.com');
INSERT INTO posts (id, user_id, title, content) VALUES (1, 1, 'First Post', "Double \"quoted\" text; with semicolon");
`

	stmts := ParseSQLStatements(sqlInput)
	if len(stmts) != 4 {
		t.Fatalf("expected 4 statements, got %d. Statements: %+v", len(stmts), stmts)
	}

	if !strings.HasPrefix(stmts[0], "CREATE TABLE users") {
		t.Errorf("stmt 0 mismatch: %s", stmts[0])
	}
	if !strings.HasPrefix(stmts[1], "CREATE TABLE posts") {
		t.Errorf("stmt 1 mismatch: %s", stmts[1])
	}
	if !strings.HasPrefix(stmts[2], "INSERT INTO users") {
		t.Errorf("stmt 2 mismatch: %s", stmts[2])
	}
	if !strings.HasPrefix(stmts[3], "INSERT INTO posts") {
		t.Errorf("stmt 3 mismatch: %s", stmts[3])
	}

	// Verify that semicolons inside string literals did not prematurely split
	if !strings.Contains(stmts[1], "Hello; world! Escaped ''single'' quotes; here") {
		t.Errorf("semicolon inside string literal was split improperly in stmt 1: %s", stmts[1])
	}
	if !strings.Contains(stmts[3], `with semicolon`) {
		t.Errorf("semicolon inside double quote was split improperly in stmt 3: %s", stmts[3])
	}
}

func TestDatabaseProvisioner_SQLiteWithSchemaImport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-provisioner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	memRepo := db.NewMemoryProvisionedDatabaseRepository()
	prov := NewDatabaseProvisioner(ProvisionerConfig{
		DBRepo:         memRepo,
		VaultSecret:    "test-secret-key-32-bytes-long!",
		ProjectBaseDir: tempDir,
	})

	ctx := context.Background()
	projectID := "proj_sqlite_demo"
	projectDir := filepath.Join(tempDir, projectID)

	schemaSQL := `
CREATE TABLE authors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

CREATE TABLE articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id INTEGER,
    title TEXT NOT NULL,
    body TEXT,
    FOREIGN KEY(author_id) REFERENCES authors(id)
);

INSERT INTO authors (name, email) VALUES ('Buchin', 'buchin@dropsugar.com');
INSERT INTO articles (author_id, title, body) VALUES (1, 'Cuan Deployer', 'Turnkey automated provisioning');
`

	req := ProvisionRequest{
		ProjectID:    projectID,
		Engine:       "sqlite",
		DBName:       "cuan_test_sqlite",
		ProjectDir:   projectDir,
		SQLContent:   schemaSQL,
		ImportSchema: true,
	}

	res, err := prov.Provision(ctx, req)
	if err != nil {
		t.Fatalf("provision failed: %v", err)
	}

	if res.Database == nil {
		t.Fatalf("expected non-nil database record")
	}
	if res.Database.Engine != "sqlite" {
		t.Errorf("expected engine=sqlite, got: %s", res.Database.Engine)
	}
	if res.Database.Status != db.DBStatusReady {
		t.Errorf("expected status=ready, got: %s", res.Database.Status)
	}
	if !res.Database.SchemaImported {
		t.Errorf("expected SchemaImported=true")
	}

	// Verify imported tables
	if res.TablesCount != 2 {
		t.Errorf("expected 2 tables created, got: %d", res.TablesCount)
	}
	expectedTables := []string{"articles", "authors"}
	for i, tbl := range expectedTables {
		if i >= len(res.ImportedTables) || res.ImportedTables[i] != tbl {
			t.Errorf("expected table %s, got: %+v", tbl, res.ImportedTables)
		}
	}

	// Direct sqlite file verification
	sqlitePath := filepath.Join(projectDir, "data", "cuan_test_sqlite.sqlite")
	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		t.Fatalf("sqlite file not found on disk at %s", sqlitePath)
	}

	directConn, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("open sqlite file failed: %v", err)
	}
	defer directConn.Close()

	var authorCount, articleCount int
	if err := directConn.QueryRow("SELECT COUNT(*) FROM authors").Scan(&authorCount); err != nil {
		t.Fatalf("query authors count failed: %v", err)
	}
	if authorCount != 1 {
		t.Errorf("expected 1 author inserted, got: %d", authorCount)
	}

	if err := directConn.QueryRow("SELECT COUNT(*) FROM articles").Scan(&articleCount); err != nil {
		t.Fatalf("query articles count failed: %v", err)
	}
	if articleCount != 1 {
		t.Errorf("expected 1 article inserted, got: %d", articleCount)
	}

	// Verify repository record
	saved, err := memRepo.GetDatabaseByID(ctx, res.Database.ID)
	if err != nil {
		t.Fatalf("failed to retrieve database from repository: %v", err)
	}
	if saved.ProjectID != projectID {
		t.Errorf("saved project id mismatch: %s", saved.ProjectID)
	}

	// Verify public DTO has zero credentials leaked
	dto := res.PublicDTO
	if dto.HasPassword != false {
		t.Errorf("sqlite should not have password, got has_password=true")
	}
}

func TestDatabaseProvisioner_RollbackOnBadSQL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-rollback-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	memRepo := db.NewMemoryProvisionedDatabaseRepository()
	prov := NewDatabaseProvisioner(ProvisionerConfig{
		DBRepo:         memRepo,
		VaultSecret:    "test-secret-key-32-bytes-long!",
		ProjectBaseDir: tempDir,
	})

	ctx := context.Background()
	projectID := "proj_rollback_demo"
	projectDir := filepath.Join(tempDir, projectID)

	badSQL := `
CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    name TEXT
);

THIS IS SYNTACTICALLY INVALID SQL THAT SHOULD TRIGGER ROLLBACK;

CREATE TABLE orders (
    id INTEGER PRIMARY KEY
);
`

	req := ProvisionRequest{
		ProjectID:    projectID,
		Engine:       "sqlite",
		DBName:       "cuan_rollback_db",
		ProjectDir:   projectDir,
		SQLContent:   badSQL,
		ImportSchema: true,
	}

	_, err = prov.Provision(ctx, req)
	if err == nil {
		t.Fatalf("expected error from bad SQL, got nil")
	}

	// Verify that table 'products' was rolled back and does not exist
	sqlitePath := filepath.Join(projectDir, "data", "cuan_rollback_db.sqlite")
	directConn, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	defer directConn.Close()

	tables, err := queryTables(ctx, directConn, db.DBEngineSQLite)
	if err != nil {
		t.Fatalf("query tables failed: %v", err)
	}
	if len(tables) != 0 {
		t.Fatalf("expected 0 tables due to transactional rollback, got %d (%v)", len(tables), tables)
	}
}

func TestDatabaseProvisioner_PostgresAndMySQL_SimulatedAndValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-pg-mysql-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	memRepo := db.NewMemoryProvisionedDatabaseRepository()
	prov := NewDatabaseProvisioner(ProvisionerConfig{
		DBRepo:         memRepo,
		VaultSecret:    "test-vault-secret-32-characters!",
		ProjectBaseDir: tempDir,
		MockMode:       true,
	})

	ctx := context.Background()

	// 1. PostgreSQL provisioning in offline mode
	pgReq := ProvisionRequest{
		ProjectID:  "proj_pg_demo",
		Engine:     "postgres",
		DBName:     "cuan_pg_db",
		DBUser:     "cuan_pg_usr",
		DBPassword: "SecretPostgresPassword123!",
		SQLContent: `
CREATE TABLE customers (id SERIAL PRIMARY KEY, name VARCHAR(100));
CREATE TABLE invoices (id SERIAL PRIMARY KEY, customer_id INT, amount NUMERIC(10,2));
`,
		ImportSchema: true,
	}

	pgRes, err := prov.Provision(ctx, pgReq)
	if err != nil {
		t.Fatalf("provision postgres failed: %v", err)
	}
	if pgRes.Database.Engine != db.DBEnginePostgres {
		t.Errorf("expected engine=postgres, got %s", pgRes.Database.Engine)
	}
	if pgRes.TablesCount != 2 {
		t.Errorf("expected 2 tables, got: %d", pgRes.TablesCount)
	}
	if !pgRes.PublicDTO.HasPassword {
		t.Errorf("expected HasPassword=true")
	}

	// Verify AES-256-GCM password decryption
	decryptedPass, err := pgRes.Database.DecryptPassword("test-vault-secret-32-characters!")
	if err != nil {
		t.Fatalf("decrypt password failed: %v", err)
	}
	if decryptedPass != "SecretPostgresPassword123!" {
		t.Errorf("password mismatch: got %s", decryptedPass)
	}

	// 2. MySQL provisioning in offline mode
	myReq := ProvisionRequest{
		ProjectID:  "proj_my_demo",
		Engine:     "mysql",
		DBName:     "cuan_my_db",
		DBUser:     "cuan_my_usr",
		DBPassword: "SecretMySQLPassword123!",
		SQLContent: `
CREATE TABLE ` + "`" + `categories` + "`" + ` (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100));
CREATE TABLE ` + "`" + `items` + "`" + ` (id INT AUTO_INCREMENT PRIMARY KEY, cat_id INT, title VARCHAR(255));
`,
		ImportSchema: true,
	}

	myRes, err := prov.Provision(ctx, myReq)
	if err != nil {
		t.Fatalf("provision mysql failed: %v", err)
	}
	if myRes.Database.Engine != db.DBEngineMySQL {
		t.Errorf("expected engine=mysql, got %s", myRes.Database.Engine)
	}
	if myRes.TablesCount != 2 {
		t.Errorf("expected 2 tables, got: %d", myRes.TablesCount)
	}

	// 3. Identifier validation: SQL Injection protection
	maliciousReq := ProvisionRequest{
		ProjectID: "proj_hacker",
		Engine:    "postgres",
		DBName:    "db; DROP DATABASE master; --",
	}
	_, err = prov.Provision(ctx, maliciousReq)
	if err == nil {
		t.Fatalf("expected error on malicious identifier, got nil")
	}
}
