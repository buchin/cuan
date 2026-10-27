package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
)

var (
	ErrIntegrationNotFound         = errors.New("project integration not found")
	ErrIntegrationAlreadyExists    = errors.New("project integration already exists for this type")
	ErrEmptyIntegrationProjectID   = errors.New("project id cannot be empty")
	ErrEmptyIntegrationType        = errors.New("integration type cannot be empty")
	ErrInvalidIntegrationStatus    = errors.New("invalid integration status: must be active, inactive, or error")
)

// NormalizeIntegrationType normalizes integration type string to lowercase.
func NormalizeIntegrationType(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

// NormalizeIntegrationStatus validates and returns normalized status.
func NormalizeIntegrationStatus(s string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	if norm == "" {
		return "active", nil
	}
	switch norm {
	case "active", "inactive", "error":
		return norm, nil
	default:
		return "", ErrInvalidIntegrationStatus
	}
}

// ProjectIntegration represents an external system integration bound to a project (e.g. WordPress).
type ProjectIntegration struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	IntegrationType string    `json:"integration_type"`
	ConfigEncrypted string    `json:"-"` // Never serialized in JSON
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ProjectIntegrationDTO is the public Data Transfer Object for ProjectIntegration.
type ProjectIntegrationDTO struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	IntegrationType string    `json:"integration_type"`
	HasConfig       bool      `json:"has_config"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ToDTO converts ProjectIntegration to its public DTO with zero credential leakage.
func (in *ProjectIntegration) ToDTO() ProjectIntegrationDTO {
	status := in.Status
	if status == "" {
		status = "active"
	}
	return ProjectIntegrationDTO{
		ID:              in.ID,
		ProjectID:       in.ProjectID,
		IntegrationType: NormalizeIntegrationType(in.IntegrationType),
		HasConfig:       in.ConfigEncrypted != "",
		Status:          status,
		CreatedAt:       in.CreatedAt,
		UpdatedAt:       in.UpdatedAt,
	}
}

// ProjectIntegrationRepository defines data operations for ProjectIntegrations.
type ProjectIntegrationRepository interface {
	CreateIntegration(ctx context.Context, in *ProjectIntegration) error
	GetIntegrationByID(ctx context.Context, id string) (*ProjectIntegration, error)
	GetIntegrationByProjectAndType(ctx context.Context, projectID, integrationType string) (*ProjectIntegration, error)
	ListIntegrationsByProjectID(ctx context.Context, projectID string) ([]*ProjectIntegration, error)
	UpdateIntegration(ctx context.Context, in *ProjectIntegration) error
	DeleteIntegration(ctx context.Context, id string) error
}

// SQLProjectIntegrationRepository implements ProjectIntegrationRepository using PostgreSQL.
type SQLProjectIntegrationRepository struct {
	db *sql.DB
}

// NewSQLProjectIntegrationRepository creates a new SQLProjectIntegrationRepository.
func NewSQLProjectIntegrationRepository(db *sql.DB) *SQLProjectIntegrationRepository {
	return &SQLProjectIntegrationRepository{db: db}
}

func (r *SQLProjectIntegrationRepository) CreateIntegration(ctx context.Context, in *ProjectIntegration) error {
	if in == nil {
		return errors.New("project integration cannot be nil")
	}
	if strings.TrimSpace(in.ProjectID) == "" {
		return ErrEmptyIntegrationProjectID
	}
	if strings.TrimSpace(in.IntegrationType) == "" {
		return ErrEmptyIntegrationType
	}
	if strings.TrimSpace(in.ID) == "" {
		in.ID = GenerateUUID()
	}

	in.IntegrationType = NormalizeIntegrationType(in.IntegrationType)
	normStatus, err := NormalizeIntegrationStatus(in.Status)
	if err != nil {
		return err
	}
	in.Status = normStatus

	now := time.Now().UTC()
	if in.CreatedAt.IsZero() {
		in.CreatedAt = now
	}
	in.UpdatedAt = now

	query := `
		INSERT INTO project_integrations (
			id, project_id, integration_type, config_encrypted, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.db.ExecContext(ctx, query,
		in.ID,
		in.ProjectID,
		in.IntegrationType,
		in.ConfigEncrypted,
		in.Status,
		in.CreatedAt,
		in.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrIntegrationAlreadyExists
		}
		return fmt.Errorf("create project integration: %w", err)
	}
	return nil
}

func (r *SQLProjectIntegrationRepository) GetIntegrationByID(ctx context.Context, id string) (*ProjectIntegration, error) {
	query := `
		SELECT id, project_id, integration_type, COALESCE(config_encrypted, ''), COALESCE(status, 'active'), created_at, updated_at
		FROM project_integrations
		WHERE id = $1
	`
	var in ProjectIntegration
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&in.ID,
		&in.ProjectID,
		&in.IntegrationType,
		&in.ConfigEncrypted,
		&in.Status,
		&in.CreatedAt,
		&in.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIntegrationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project integration by id: %w", err)
	}
	return &in, nil
}

func (r *SQLProjectIntegrationRepository) GetIntegrationByProjectAndType(ctx context.Context, projectID, integrationType string) (*ProjectIntegration, error) {
	normType := NormalizeIntegrationType(integrationType)
	query := `
		SELECT id, project_id, integration_type, COALESCE(config_encrypted, ''), COALESCE(status, 'active'), created_at, updated_at
		FROM project_integrations
		WHERE project_id = $1 AND LOWER(integration_type) = LOWER($2)
		LIMIT 1
	`
	var in ProjectIntegration
	err := r.db.QueryRowContext(ctx, query, projectID, normType).Scan(
		&in.ID,
		&in.ProjectID,
		&in.IntegrationType,
		&in.ConfigEncrypted,
		&in.Status,
		&in.CreatedAt,
		&in.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrIntegrationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project integration by type: %w", err)
	}
	return &in, nil
}

func (r *SQLProjectIntegrationRepository) ListIntegrationsByProjectID(ctx context.Context, projectID string) ([]*ProjectIntegration, error) {
	query := `
		SELECT id, project_id, integration_type, COALESCE(config_encrypted, ''), COALESCE(status, 'active'), created_at, updated_at
		FROM project_integrations
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project integrations: %w", err)
	}
	defer rows.Close()

	var list []*ProjectIntegration
	for rows.Next() {
		var in ProjectIntegration
		if err := rows.Scan(
			&in.ID,
			&in.ProjectID,
			&in.IntegrationType,
			&in.ConfigEncrypted,
			&in.Status,
			&in.CreatedAt,
			&in.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project integration: %w", err)
		}
		list = append(list, &in)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project integrations: %w", err)
	}
	return list, nil
}

func (r *SQLProjectIntegrationRepository) UpdateIntegration(ctx context.Context, in *ProjectIntegration) error {
	if in == nil {
		return errors.New("project integration cannot be nil")
	}
	normStatus, err := NormalizeIntegrationStatus(in.Status)
	if err != nil {
		return err
	}
	in.Status = normStatus
	in.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE project_integrations
		SET config_encrypted = $2, status = $3, updated_at = $4
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		in.ID,
		in.ConfigEncrypted,
		in.Status,
		in.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update project integration: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project integration rows affected: %w", err)
	}
	if rows == 0 {
		return ErrIntegrationNotFound
	}
	return nil
}

func (r *SQLProjectIntegrationRepository) DeleteIntegration(ctx context.Context, id string) error {
	query := `DELETE FROM project_integrations WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project integration: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project integration rows affected: %w", err)
	}
	if rows == 0 {
		return ErrIntegrationNotFound
	}
	return nil
}

// MemoryProjectIntegrationRepository implements ProjectIntegrationRepository in-memory for testing.
type MemoryProjectIntegrationRepository struct {
	mu           sync.RWMutex
	integrations map[string]*ProjectIntegration
}

// NewMemoryProjectIntegrationRepository creates a new thread-safe MemoryProjectIntegrationRepository.
func NewMemoryProjectIntegrationRepository() *MemoryProjectIntegrationRepository {
	return &MemoryProjectIntegrationRepository{
		integrations: make(map[string]*ProjectIntegration),
	}
}

func cloneIntegration(in *ProjectIntegration) *ProjectIntegration {
	if in == nil {
		return nil
	}
	cp := *in
	return &cp
}

func (m *MemoryProjectIntegrationRepository) CreateIntegration(ctx context.Context, in *ProjectIntegration) error {
	if in == nil {
		return errors.New("project integration cannot be nil")
	}
	if strings.TrimSpace(in.ProjectID) == "" {
		return ErrEmptyIntegrationProjectID
	}
	if strings.TrimSpace(in.IntegrationType) == "" {
		return ErrEmptyIntegrationType
	}

	normType := NormalizeIntegrationType(in.IntegrationType)
	normStatus, err := NormalizeIntegrationStatus(in.Status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.integrations {
		if existing.ProjectID == in.ProjectID && existing.IntegrationType == normType {
			return ErrIntegrationAlreadyExists
		}
	}

	if strings.TrimSpace(in.ID) == "" {
		in.ID = GenerateUUID()
	}
	in.IntegrationType = normType
	in.Status = normStatus

	now := time.Now().UTC()
	if in.CreatedAt.IsZero() {
		in.CreatedAt = now
	}
	in.UpdatedAt = now

	m.integrations[in.ID] = cloneIntegration(in)
	return nil
}

func (m *MemoryProjectIntegrationRepository) GetIntegrationByID(ctx context.Context, id string) (*ProjectIntegration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	in, exists := m.integrations[id]
	if !exists {
		return nil, ErrIntegrationNotFound
	}
	return cloneIntegration(in), nil
}

func (m *MemoryProjectIntegrationRepository) GetIntegrationByProjectAndType(ctx context.Context, projectID, integrationType string) (*ProjectIntegration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normType := NormalizeIntegrationType(integrationType)
	for _, in := range m.integrations {
		if in.ProjectID == projectID && in.IntegrationType == normType {
			return cloneIntegration(in), nil
		}
	}
	return nil, ErrIntegrationNotFound
}

func (m *MemoryProjectIntegrationRepository) ListIntegrationsByProjectID(ctx context.Context, projectID string) ([]*ProjectIntegration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*ProjectIntegration
	for _, in := range m.integrations {
		if in.ProjectID == projectID {
			list = append(list, cloneIntegration(in))
		}
	}
	return list, nil
}

func (m *MemoryProjectIntegrationRepository) UpdateIntegration(ctx context.Context, in *ProjectIntegration) error {
	if in == nil {
		return errors.New("project integration cannot be nil")
	}
	normStatus, err := NormalizeIntegrationStatus(in.Status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.integrations[in.ID]
	if !exists {
		return ErrIntegrationNotFound
	}

	in.Status = normStatus
	in.UpdatedAt = time.Now().UTC()
	in.CreatedAt = existing.CreatedAt
	in.ProjectID = existing.ProjectID
	in.IntegrationType = existing.IntegrationType

	m.integrations[in.ID] = cloneIntegration(in)
	return nil
}

func (m *MemoryProjectIntegrationRepository) DeleteIntegration(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.integrations[id]; !exists {
		return ErrIntegrationNotFound
	}
	delete(m.integrations, id)
	return nil
}

// EncryptApplicationPassword encrypts an application password (e.g. for WordPress) via AES-256-GCM.
func EncryptApplicationPassword(appPassword, secret string) (string, error) {
	return crypto.EncryptStringAES256GCM(appPassword, secret)
}

// DecryptApplicationPassword decrypts an application password via AES-256-GCM.
func DecryptApplicationPassword(encryptedB64, secret string) (string, error) {
	return crypto.DecryptStringAES256GCM(encryptedB64, secret)
}
