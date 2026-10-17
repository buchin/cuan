package deployer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/buchin/cuan/internal/db"
)

var (
	ErrInvalidSQLIdentifier = errors.New("invalid database identifier: must be alphanumeric and underscore (1-63 chars)")
	ErrDatabaseAlreadyExists = errors.New("database already exists")
	ErrSchemaExecutionFailed = errors.New("schema execution failed")
	ErrNoSQLStatementsFound  = errors.New("no valid SQL statements found in schema file")
	validIdentifierRegex     = regexp.MustCompile(`^[a-zA-Z0-9_]{1,63}$`)
)

// ProvisionerConfig defines configuration for DatabaseProvisioner.
type ProvisionerConfig struct {
	DBRepo           db.ProvisionedDatabaseRepository
	VaultSecret      string
	ProjectBaseDir   string
	PostgresAdminURL string
	MySQLAdminDSN    string
	MockMode         bool
}

// ProvisionRequest specifies parameters to provision a database.
type ProvisionRequest struct {
	ProjectID    string  `json:"project_id"`
	ServerID     *string `json:"server_id,omitempty"`
	Engine       string  `json:"engine"` // sqlite, mysql, postgres
	DBName       string  `json:"db_name,omitempty"`
	DBUser       string  `json:"db_user,omitempty"`
	DBPassword   string  `json:"db_password,omitempty"`
	ProjectDir   string  `json:"project_dir,omitempty"`
	SQLFilePath  string  `json:"sql_file_path,omitempty"`
	SQLContent   string  `json:"sql_content,omitempty"`
	ImportSchema bool    `json:"import_schema"`
}

// ProvisionResult encapsulates the provisioned database and verification details.
type ProvisionResult struct {
	Database        *db.ProvisionedDatabase    `json:"database"`
	PublicDTO       db.ProvisionedDatabaseDTO  `json:"public_dto"`
	TablesCount     int                        `json:"tables_count"`
	ImportedTables  []string                   `json:"imported_tables"`
	TotalStatements int                        `json:"total_statements"`
	ExecutionLog    []string                   `json:"execution_log"`
}

// DatabaseProvisioner orchestrates isolated database provisioning and SQL schema importing.
type DatabaseProvisioner struct {
	cfg ProvisionerConfig
	mu  sync.RWMutex
}

// NewDatabaseProvisioner constructs a DatabaseProvisioner instance.
func NewDatabaseProvisioner(cfg ProvisionerConfig) *DatabaseProvisioner {
	if cfg.ProjectBaseDir == "" {
		cfg.ProjectBaseDir = filepath.Join("data", "projects")
	}
	if cfg.VaultSecret == "" {
		cfg.VaultSecret = "cuan-default-database-provisioner-vault-key"
	}
	return &DatabaseProvisioner{
		cfg: cfg,
	}
}

// Provision creates an isolated database (SQLite, MySQL, or PostgreSQL) and optionally imports SQL schema.
func (p *DatabaseProvisioner) Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if strings.TrimSpace(req.ProjectID) == "" {
		return nil, db.ErrEmptyDBProjectID
	}

	normEngine, err := db.NormalizeDBEngine(req.Engine)
	if err != nil {
		return nil, err
	}

	projectDir := req.ProjectDir
	if projectDir == "" {
		projectDir = filepath.Join(p.cfg.ProjectBaseDir, req.ProjectID)
	}

	// Generate safe identifiers if not specified
	dbName := strings.TrimSpace(req.DBName)
	if dbName == "" {
		safeProject := sanitizeIdentifier(req.ProjectID)
		if len(safeProject) > 16 {
			safeProject = safeProject[:16]
		}
		randomSuffix, _ := GenerateSessionSalt()
		if len(randomSuffix) > 8 {
			randomSuffix = randomSuffix[:8]
		}
		dbName = fmt.Sprintf("cuan_%s_%s", safeProject, randomSuffix)
	}

	dbUser := strings.TrimSpace(req.DBUser)
	dbPassword := strings.TrimSpace(req.DBPassword)

	if normEngine != db.DBEngineSQLite {
		if dbUser == "" {
			safeProject := sanitizeIdentifier(req.ProjectID)
			if len(safeProject) > 10 {
				safeProject = safeProject[:10]
			}
			randomSuffix, _ := GenerateSessionSalt()
			if len(randomSuffix) > 6 {
				randomSuffix = randomSuffix[:6]
			}
			dbUser = fmt.Sprintf("u_%s_%s", safeProject, randomSuffix)
		}
		if dbPassword == "" {
			generatedPass, pErr := GenerateSecurePassword(24)
			if pErr != nil {
				return nil, fmt.Errorf("generate db password: %w", pErr)
			}
			dbPassword = generatedPass
		}

		if !validIdentifierRegex.MatchString(dbName) {
			return nil, fmt.Errorf("%w: db_name %q", ErrInvalidSQLIdentifier, dbName)
		}
		if !validIdentifierRegex.MatchString(dbUser) {
			return nil, fmt.Errorf("%w: db_user %q", ErrInvalidSQLIdentifier, dbUser)
		}
	}

	var (
		importedTables  []string
		totalStatements int
		schemaImported  bool
		execLogs        []string
	)

	// Execute engine-specific provisioning
	switch normEngine {
	case db.DBEngineSQLite:
		// Ensure isolated storage directory exists
		dataDir := filepath.Join(projectDir, "data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}

		sqliteFilePath := filepath.Join(dataDir, dbName+".sqlite")
		execLogs = append(execLogs, fmt.Sprintf("Provisioning SQLite database file at %s", sqliteFilePath))

		dbConn, err := sql.Open("sqlite", sqliteFilePath)
		if err != nil {
			return nil, fmt.Errorf("open sqlite database: %w", err)
		}
		defer dbConn.Close()

		if err := dbConn.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("ping sqlite database: %w", err)
		}

		// Import SQL schema if requested
		if req.ImportSchema {
			sqlContent, err := p.resolveSQLContent(projectDir, req.SQLFilePath, req.SQLContent)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(sqlContent) != "" {
				tables, count, err := p.executeSchemaAndVerify(ctx, dbConn, db.DBEngineSQLite, sqlContent)
				if err != nil {
					return nil, fmt.Errorf("import sqlite schema: %w", err)
				}
				importedTables = tables
				totalStatements = count
				schemaImported = len(tables) > 0
				execLogs = append(execLogs, fmt.Sprintf("Imported %d SQL statements, created %d tables", count, len(tables)))
			}
		}

	case db.DBEnginePostgres:
		execLogs = append(execLogs, fmt.Sprintf("Provisioning PostgreSQL database %s with isolated user %s", dbName, dbUser))
		if p.cfg.PostgresAdminURL != "" && !p.cfg.MockMode {
			adminConn, err := sql.Open("pgx", p.cfg.PostgresAdminURL)
			if err != nil {
				return nil, fmt.Errorf("connect to postgres admin: %w", err)
			}
			defer adminConn.Close()

			if err := adminConn.PingContext(ctx); err != nil {
				return nil, fmt.Errorf("ping postgres admin: %w", err)
			}

			// In Postgres, CREATE DATABASE cannot execute within a transaction block
			createDBQuery := fmt.Sprintf(`CREATE DATABASE "%s";`, dbName)
			if _, err := adminConn.ExecContext(ctx, createDBQuery); err != nil {
				return nil, fmt.Errorf("create postgres database: %w", err)
			}

			escapedPass := strings.ReplaceAll(dbPassword, "'", "''")
			createUserQuery := fmt.Sprintf(`CREATE USER "%s" WITH ENCRYPTED PASSWORD '%s';`, dbUser, escapedPass)
			if _, err := adminConn.ExecContext(ctx, createUserQuery); err != nil {
				// Cleanup created database on user creation failure
				_, _ = adminConn.ExecContext(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s";`, dbName))
				return nil, fmt.Errorf("create postgres user: %w", err)
			}

			grantQuery := fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s";`, dbName, dbUser)
			if _, err := adminConn.ExecContext(ctx, grantQuery); err != nil {
				return nil, fmt.Errorf("grant postgres permissions: %w", err)
			}

			if req.ImportSchema {
				sqlContent, err := p.resolveSQLContent(projectDir, req.SQLFilePath, req.SQLContent)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(sqlContent) != "" {
					targetDSN := replacePostgresDSNDatabase(p.cfg.PostgresAdminURL, dbName)
					targetConn, err := sql.Open("pgx", targetDSN)
					if err != nil {
						return nil, fmt.Errorf("connect to provisioned postgres db: %w", err)
					}
					defer targetConn.Close()

					tables, count, err := p.executeSchemaAndVerify(ctx, targetConn, db.DBEnginePostgres, sqlContent)
					if err != nil {
						return nil, fmt.Errorf("import postgres schema: %w", err)
					}
					importedTables = tables
					totalStatements = count
					schemaImported = len(tables) > 0
					execLogs = append(execLogs, fmt.Sprintf("Imported %d SQL statements, created %d tables", count, len(tables)))
				}
			}
		} else {
			// Mock / simulated environment: validate SQL schema syntax and verify parser
			execLogs = append(execLogs, "PostgreSQL provisioned in simulated offline mode")
			if req.ImportSchema {
				sqlContent, err := p.resolveSQLContent(projectDir, req.SQLFilePath, req.SQLContent)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(sqlContent) != "" {
					stmts := ParseSQLStatements(sqlContent)
					tables := extractCreatedTables(stmts)
					importedTables = tables
					totalStatements = len(stmts)
					schemaImported = len(tables) > 0
					execLogs = append(execLogs, fmt.Sprintf("Verified %d SQL statements for %d tables", totalStatements, len(tables)))
				}
			}
		}

	case db.DBEngineMySQL:
		execLogs = append(execLogs, fmt.Sprintf("Provisioning MySQL database %s with isolated user %s", dbName, dbUser))
		if p.cfg.MySQLAdminDSN != "" && !p.cfg.MockMode {
			adminConn, err := sql.Open("mysql", p.cfg.MySQLAdminDSN)
			if err != nil {
				return nil, fmt.Errorf("connect to mysql admin: %w", err)
			}
			defer adminConn.Close()

			if err := adminConn.PingContext(ctx); err != nil {
				return nil, fmt.Errorf("ping mysql admin: %w", err)
			}

			createDBQuery := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName)
			if _, err := adminConn.ExecContext(ctx, createDBQuery); err != nil {
				return nil, fmt.Errorf("create mysql database: %w", err)
			}

			escapedPass := strings.ReplaceAll(strings.ReplaceAll(dbPassword, "\\", "\\\\"), "'", "\\'")
			createUserQuery := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s';", dbUser, escapedPass)
			if _, err := adminConn.ExecContext(ctx, createUserQuery); err != nil {
				return nil, fmt.Errorf("create mysql user: %w", err)
			}

			grantQuery := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%';", dbName, dbUser)
			if _, err := adminConn.ExecContext(ctx, grantQuery); err != nil {
				return nil, fmt.Errorf("grant mysql permissions: %w", err)
			}

			_, _ = adminConn.ExecContext(ctx, "FLUSH PRIVILEGES;")

			if req.ImportSchema {
				sqlContent, err := p.resolveSQLContent(projectDir, req.SQLFilePath, req.SQLContent)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(sqlContent) != "" {
					targetDSN := replaceMySQLDSNDatabase(p.cfg.MySQLAdminDSN, dbName)
					targetConn, err := sql.Open("mysql", targetDSN)
					if err != nil {
						return nil, fmt.Errorf("connect to provisioned mysql db: %w", err)
					}
					defer targetConn.Close()

					tables, count, err := p.executeSchemaAndVerify(ctx, targetConn, db.DBEngineMySQL, sqlContent)
					if err != nil {
						return nil, fmt.Errorf("import mysql schema: %w", err)
					}
					importedTables = tables
					totalStatements = count
					schemaImported = len(tables) > 0
					execLogs = append(execLogs, fmt.Sprintf("Imported %d SQL statements, created %d tables", count, len(tables)))
				}
			}
		} else {
			// Mock / simulated environment: validate SQL schema syntax and verify parser
			execLogs = append(execLogs, "MySQL provisioned in simulated offline mode")
			if req.ImportSchema {
				sqlContent, err := p.resolveSQLContent(projectDir, req.SQLFilePath, req.SQLContent)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(sqlContent) != "" {
					stmts := ParseSQLStatements(sqlContent)
					tables := extractCreatedTables(stmts)
					importedTables = tables
					totalStatements = len(stmts)
					schemaImported = len(tables) > 0
					execLogs = append(execLogs, fmt.Sprintf("Verified %d SQL statements for %d tables", totalStatements, len(tables)))
				}
			}
		}
	}

	// Construct domain model and encrypt password via AES-256-GCM
	record := &db.ProvisionedDatabase{
		ID:             db.GenerateUUID(),
		ProjectID:      req.ProjectID,
		ServerID:       req.ServerID,
		Engine:         normEngine,
		DBName:         dbName,
		DBUser:         dbUser,
		Status:         db.DBStatusReady,
		SchemaImported: schemaImported,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if dbPassword != "" {
		if err := record.EncryptPassword(dbPassword, p.cfg.VaultSecret); err != nil {
			return nil, fmt.Errorf("encrypt db password: %w", err)
		}
	}

	if p.cfg.DBRepo != nil {
		if err := p.cfg.DBRepo.CreateDatabase(ctx, record); err != nil {
			return nil, fmt.Errorf("save provisioned database record: %w", err)
		}
	}

	return &ProvisionResult{
		Database:        record,
		PublicDTO:       record.ToDTO(),
		TablesCount:     len(importedTables),
		ImportedTables:  importedTables,
		TotalStatements: totalStatements,
		ExecutionLog:    execLogs,
	}, nil
}

// resolveSQLContent gets SQL text either from req.SQLContent or by reading req.SQLFilePath inside projectDir.
func (p *DatabaseProvisioner) resolveSQLContent(projectDir, sqlPath, inlineSQL string) (string, error) {
	if strings.TrimSpace(inlineSQL) != "" {
		return inlineSQL, nil
	}

	if strings.TrimSpace(sqlPath) != "" {
		fullPath := sqlPath
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(projectDir, sqlPath)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("read sql schema file '%s': %w", sqlPath, err)
		}
		return string(data), nil
	}

	// Auto-detect *.sql file in project directory if neither is specified
	entries, err := os.ReadDir(projectDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
				data, err := os.ReadFile(filepath.Join(projectDir, entry.Name()))
				if err == nil {
					return string(data), nil
				}
			}
		}
	}

	return "", nil
}

// executeSchemaAndVerify executes parsed SQL statements on dbConn and queries resulting table list.
func (p *DatabaseProvisioner) executeSchemaAndVerify(ctx context.Context, dbConn *sql.DB, dialect string, sqlContent string) ([]string, int, error) {
	stmts := ParseSQLStatements(sqlContent)
	if len(stmts) == 0 {
		return nil, 0, nil
	}

	// Use transaction for dialects that support transactional DDL (SQLite & Postgres)
	if dialect == db.DBEngineSQLite || dialect == db.DBEnginePostgres {
		tx, err := dbConn.BeginTx(ctx, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("begin schema tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		for idx, stmt := range stmts {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return nil, 0, fmt.Errorf("statement %d (%s): %w", idx+1, truncateSnippet(stmt, 50), err)
			}
		}

		if err := tx.Commit(); err != nil {
			return nil, 0, fmt.Errorf("commit schema tx: %w", err)
		}
	} else {
		// MySQL does not support transactional DDL; execute sequentially
		for idx, stmt := range stmts {
			if _, err := dbConn.ExecContext(ctx, stmt); err != nil {
				return nil, 0, fmt.Errorf("statement %d (%s): %w", idx+1, truncateSnippet(stmt, 50), err)
			}
		}
	}

	tables, err := queryTables(ctx, dbConn, dialect)
	if err != nil {
		return nil, len(stmts), fmt.Errorf("query tables after schema import: %w", err)
	}

	return tables, len(stmts), nil
}

// ParseSQLStatements splits SQL content into separate executable statements while properly
// preserving string literals, handling block/line comments, and ignoring empty statements.
func ParseSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	chars := []rune(sql)
	n := len(chars)

	for i := 0; i < n; i++ {
		ch := chars[i]

		// Handle active line comment
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				current.WriteRune(ch)
			}
			continue
		}

		// Handle active block comment
		if inBlockComment {
			if ch == '*' && i+1 < n && chars[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		// Handle quotes
		if inSingleQuote {
			current.WriteRune(ch)
			if ch == '\\' && i+1 < n {
				i++
				current.WriteRune(chars[i])
			} else if ch == '\'' {
				// Handle escaped quote in SQL ''
				if i+1 < n && chars[i+1] == '\'' {
					i++
					current.WriteRune(chars[i])
				} else {
					inSingleQuote = false
				}
			}
			continue
		}

		if inDoubleQuote {
			current.WriteRune(ch)
			if ch == '\\' && i+1 < n {
				i++
				current.WriteRune(chars[i])
			} else if ch == '"' {
				inDoubleQuote = false
			}
			continue
		}

		if inBacktick {
			current.WriteRune(ch)
			if ch == '`' {
				inBacktick = false
			}
			continue
		}

		// Check starting of line comments (-- or #)
		if (ch == '-' && i+1 < n && chars[i+1] == '-') || ch == '#' {
			inLineComment = true
			continue
		}

		// Check starting of block comments (/*)
		if ch == '/' && i+1 < n && chars[i+1] == '*' {
			inBlockComment = true
			i++
			continue
		}

		// Check start of string literals
		if ch == '\'' {
			inSingleQuote = true
			current.WriteRune(ch)
			continue
		}
		if ch == '"' {
			inDoubleQuote = true
			current.WriteRune(ch)
			continue
		}
		if ch == '`' {
			inBacktick = true
			current.WriteRune(ch)
			continue
		}

		// Statement terminator semicolon
		if ch == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteRune(ch)
	}

	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		statements = append(statements, remaining)
	}

	return statements
}

// queryTables queries the list of user tables in the given database connection.
func queryTables(ctx context.Context, dbConn *sql.DB, dialect string) ([]string, error) {
	var query string
	switch dialect {
	case db.DBEngineSQLite:
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;"
	case db.DBEnginePostgres:
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;"
	case db.DBEngineMySQL:
		query = "SHOW TABLES;"
	default:
		return nil, nil
	}

	rows, err := dbConn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Strings(tables)
	return tables, nil
}

// extractCreatedTables parses table names from CREATE TABLE statements for validation.
func extractCreatedTables(stmts []string) []string {
	tableMap := make(map[string]bool)
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+[` + "`" + `"'\[]?([a-zA-Z0-9_]+)`)
	for _, stmt := range stmts {
		matches := re.FindStringSubmatch(stmt)
		if len(matches) > 1 {
			tableMap[matches[1]] = true
		}
	}

	var tables []string
	for t := range tableMap {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	return tables
}

func sanitizeIdentifier(val string) string {
	val = strings.ToLower(val)
	var sb strings.Builder
	for _, ch := range val {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			sb.WriteRune(ch)
		} else {
			sb.WriteRune('_')
		}
	}
	res := strings.Trim(sb.String(), "_")
	if res == "" {
		return "db"
	}
	return res
}

func truncateSnippet(s string, maxLen int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func replacePostgresDSNDatabase(originalURL, newDBName string) string {
	idx := strings.LastIndex(originalURL, "/")
	if idx == -1 {
		return originalURL + "/" + newDBName
	}
	qIdx := strings.Index(originalURL[idx:], "?")
	if qIdx != -1 {
		return originalURL[:idx+1] + newDBName + originalURL[idx+qIdx:]
	}
	return originalURL[:idx+1] + newDBName
}

func replaceMySQLDSNDatabase(originalDSN, newDBName string) string {
	idx := strings.LastIndex(originalDSN, "/")
	if idx == -1 {
		return originalDSN + "/" + newDBName
	}
	qIdx := strings.Index(originalDSN[idx:], "?")
	if qIdx != -1 {
		return originalDSN[:idx+1] + newDBName + originalDSN[idx+qIdx:]
	}
	return originalDSN[:idx+1] + newDBName
}
