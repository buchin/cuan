package deployer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DBStore implements Store using PostgreSQL.
type DBStore struct {
	db *sql.DB
}

// NewDBStore creates a PostgreSQL-backed deployer store.
func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

func (s *DBStore) CreateDeployment(ctx context.Context, dep *Deployment) error {
	query := `
		INSERT INTO deployments (
			id, server_id, project_name, slug, source_type, source_ref, runtime,
			status, internal_port, domain, env_vars, process_pid,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	now := time.Now().UTC()
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now

	envJSON, err := json.Marshal(dep.EnvVars)
	if err != nil {
		envJSON = []byte("{}")
	}

	var serverIDVal *string
	if dep.ServerID != "" {
		serverIDVal = &dep.ServerID
	}

	_, err = s.db.ExecContext(ctx, query,
		dep.ID,
		serverIDVal,
		dep.ProjectName,
		dep.Slug,
		dep.SourceType,
		dep.SourceRef,
		dep.Runtime,
		dep.Status,
		dep.InternalPort,
		dep.Domain,
		envJSON,
		dep.ProcessPID,
		dep.CreatedAt,
		dep.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrSlugConflict
		}
		return fmt.Errorf("create deployment: %w", err)
	}
	return nil
}

func (s *DBStore) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	query := `
		SELECT id, COALESCE(server_id, ''), project_name, slug, source_type, COALESCE(source_ref, ''),
		       runtime, status, internal_port, domain, env_vars, process_pid,
		       created_at, updated_at
		FROM deployments
		WHERE id = $1
	`
	return s.scanDeployment(s.db.QueryRowContext(ctx, query, id))
}

func (s *DBStore) GetDeploymentBySlug(ctx context.Context, slug string) (*Deployment, error) {
	query := `
		SELECT id, COALESCE(server_id, ''), project_name, slug, source_type, COALESCE(source_ref, ''),
		       runtime, status, internal_port, domain, env_vars, process_pid,
		       created_at, updated_at
		FROM deployments
		WHERE slug = $1
	`
	return s.scanDeployment(s.db.QueryRowContext(ctx, query, slug))
}

func (s *DBStore) UpdateDeployment(ctx context.Context, dep *Deployment) error {
	query := `
		UPDATE deployments
		SET server_id = $2, project_name = $3, slug = $4, source_type = $5, source_ref = $6,
		    runtime = $7, status = $8, internal_port = $9, domain = $10,
		    env_vars = $11, process_pid = $12, updated_at = $13
		WHERE id = $1
	`
	dep.UpdatedAt = time.Now().UTC()
	envJSON, err := json.Marshal(dep.EnvVars)
	if err != nil {
		envJSON = []byte("{}")
	}

	var serverIDVal *string
	if dep.ServerID != "" {
		serverIDVal = &dep.ServerID
	}

	res, err := s.db.ExecContext(ctx, query,
		dep.ID,
		serverIDVal,
		dep.ProjectName,
		dep.Slug,
		dep.SourceType,
		dep.SourceRef,
		dep.Runtime,
		dep.Status,
		dep.InternalPort,
		dep.Domain,
		envJSON,
		dep.ProcessPID,
		dep.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrSlugConflict
		}
		return fmt.Errorf("update deployment: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update deployment rows: %w", err)
	}
	if rows == 0 {
		return ErrDeploymentNotFound
	}
	return nil
}

func (s *DBStore) UpdateDeploymentStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE deployments
		SET status = $2, updated_at = $3
		WHERE id = $1
	`
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, query, id, status, now)
	if err != nil {
		return fmt.Errorf("update deployment status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update deployment status rows: %w", err)
	}
	if rows == 0 {
		return ErrDeploymentNotFound
	}
	return nil
}

func (s *DBStore) ListDeployments(ctx context.Context) ([]Deployment, error) {
	query := `
		SELECT id, COALESCE(server_id, ''), project_name, slug, source_type, COALESCE(source_ref, ''),
		       runtime, status, internal_port, domain, env_vars, process_pid,
		       created_at, updated_at
		FROM deployments
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()

	var list []Deployment
	for rows.Next() {
		var dep Deployment
		var envRaw []byte
		if err := rows.Scan(
			&dep.ID,
			&dep.ServerID,
			&dep.ProjectName,
			&dep.Slug,
			&dep.SourceType,
			&dep.SourceRef,
			&dep.Runtime,
			&dep.Status,
			&dep.InternalPort,
			&dep.Domain,
			&envRaw,
			&dep.ProcessPID,
			&dep.CreatedAt,
			&dep.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan deployment row: %w", err)
		}
		if len(envRaw) > 0 {
			_ = json.Unmarshal(envRaw, &dep.EnvVars)
		}
		list = append(list, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deployments: %w", err)
	}

	return list, nil
}

func (s *DBStore) DeleteDeployment(ctx context.Context, id string) error {
	query := `DELETE FROM deployments WHERE id = $1`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete deployment: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete deployment rows: %w", err)
	}
	if rows == 0 {
		return ErrDeploymentNotFound
	}
	return nil
}

func (s *DBStore) AppendLog(ctx context.Context, log *LogEntry) error {
	query := `
		INSERT INTO deployment_logs (deployment_id, stream, message, timestamp)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	err := s.db.QueryRowContext(ctx, query,
		log.DeploymentID,
		log.Stream,
		log.Message,
		log.Timestamp,
	).Scan(&log.ID)
	if err != nil {
		return fmt.Errorf("append deployment log: %w", err)
	}
	return nil
}

func (s *DBStore) GetLogs(ctx context.Context, deploymentID string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT id, deployment_id, stream, message, timestamp
		FROM (
			SELECT id, deployment_id, stream, message, timestamp
			FROM deployment_logs
			WHERE deployment_id = $1
			ORDER BY timestamp DESC, id DESC
			LIMIT $2
		) sub
		ORDER BY timestamp ASC, id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, deploymentID, limit)
	if err != nil {
		return nil, fmt.Errorf("get deployment logs: %w", err)
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var l LogEntry
		if err := rows.Scan(&l.ID, &l.DeploymentID, &l.Stream, &l.Message, &l.Timestamp); err != nil {
			return nil, fmt.Errorf("scan log row: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate logs: %w", err)
	}

	return logs, nil
}

func (s *DBStore) scanDeployment(row *sql.Row) (*Deployment, error) {
	var dep Deployment
	var envRaw []byte
	err := row.Scan(
		&dep.ID,
		&dep.ServerID,
		&dep.ProjectName,
		&dep.Slug,
		&dep.SourceType,
		&dep.SourceRef,
		&dep.Runtime,
		&dep.Status,
		&dep.InternalPort,
		&dep.Domain,
		&envRaw,
		&dep.ProcessPID,
		&dep.CreatedAt,
		&dep.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan deployment: %w", err)
	}
	if len(envRaw) > 0 {
		_ = json.Unmarshal(envRaw, &dep.EnvVars)
	}
	return &dep, nil
}
