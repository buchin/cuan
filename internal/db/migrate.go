package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/migrations"
)

type Migration struct {
	Version string
	Name    string
	UpSQL   string
	DownSQL string
}

// LoadMigrations parses and returns all embedded migrations ordered by version ascending.
func LoadMigrations() ([]Migration, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	upMap := make(map[string]struct {
		name string
		sql  string
	})
	downMap := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		parts := strings.Split(filename, ".")
		// Format: 000001_name.up.sql or 000001_name.down.sql
		if len(parts) < 3 {
			continue
		}
		direction := parts[len(parts)-2]
		base := strings.Join(parts[:len(parts)-2], ".")

		nameParts := strings.SplitN(base, "_", 2)
		version := nameParts[0]
		name := ""
		if len(nameParts) > 1 {
			name = nameParts[1]
		}

		content, err := migrations.FS.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read migration file %s: %w", filename, err)
		}

		if direction == "up" {
			upMap[version] = struct {
				name string
				sql  string
			}{name: name, sql: string(content)}
		} else if direction == "down" {
			downMap[version] = string(content)
		}
	}

	var versions []string
	for v := range upMap {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	var list []Migration
	for _, v := range versions {
		up := upMap[v]
		list = append(list, Migration{
			Version: v,
			Name:    up.name,
			UpSQL:   up.sql,
			DownSQL: downMap[v],
		})
	}

	return list, nil
}

// DryRun returns a formatted string of the planned migration statements without executing them.
func DryRun(direction string) (string, error) {
	migList, err := LoadMigrations()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- Cuan Migration Dry-Run Plan (Direction: %s)\n", strings.ToUpper(direction)))
	sb.WriteString(fmt.Sprintf("-- Total migrations found: %d\n\n", len(migList)))

	if strings.ToLower(direction) == "down" {
		for i := len(migList) - 1; i >= 0; i-- {
			m := migList[i]
			sb.WriteString(fmt.Sprintf("-- >>> [DOWN] Migration: %s (%s)\n", m.Version, m.Name))
			sb.WriteString(strings.TrimSpace(m.DownSQL))
			sb.WriteString("\n\n")
		}
	} else {
		for _, m := range migList {
			sb.WriteString(fmt.Sprintf("-- >>> [UP] Migration: %s (%s)\n", m.Version, m.Name))
			sb.WriteString(strings.TrimSpace(m.UpSQL))
			sb.WriteString("\n\n")
		}
	}

	return sb.String(), nil
}

// Run executes pending migrations against the given DB.
func Run(ctx context.Context, database *sql.DB, direction string) error {
	migList, err := LoadMigrations()
	if err != nil {
		return err
	}

	// Ensure migration table exists
	initTableSQL := `
	CREATE TABLE IF NOT EXISTS cuan_schema_migrations (
		version VARCHAR(64) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := database.ExecContext(ctx, initTableSQL); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	if strings.ToLower(direction) == "down" {
		for i := len(migList) - 1; i >= 0; i-- {
			m := migList[i]
			var applied bool
			err := database.QueryRowContext(ctx, "SELECT true FROM cuan_schema_migrations WHERE version = $1", m.Version).Scan(&applied)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("check migration %s: %w", m.Version, err)
			}
			if !applied {
				continue
			}

			tx, err := database.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("begin tx for %s: %w", m.Version, err)
			}

			if _, err := tx.ExecContext(ctx, m.DownSQL); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply down migration %s: %w", m.Version, err)
			}

			if _, err := tx.ExecContext(ctx, "DELETE FROM cuan_schema_migrations WHERE version = $1", m.Version); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("remove migration record %s: %w", m.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit down migration %s: %w", m.Version, err)
			}
		}
		return nil
	}

	// UP direction
	for _, m := range migList {
		var applied bool
		err := database.QueryRowContext(ctx, "SELECT true FROM cuan_schema_migrations WHERE version = $1", m.Version).Scan(&applied)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", m.Version, err)
		}
		if applied {
			continue
		}

		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx, m.UpSQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply up migration %s: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO cuan_schema_migrations (version, name, applied_at) VALUES ($1, $2, $3)", m.Version, m.Name, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit up migration %s: %w", m.Version, err)
		}
	}

	return nil
}
