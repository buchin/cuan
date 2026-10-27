package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
)

// Constants for ScriptDeployment build and health status
const (
	BuildStatusPending  = "pending"
	BuildStatusBuilding = "building"
	BuildStatusSuccess  = "success"
	BuildStatusFailed   = "failed"
	BuildStatusRunning  = "running"
	BuildStatusStopped  = "stopped"

	HealthStatusUnknown   = "unknown"
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusDegraded  = "degraded"
)

// Constants for ProvisionedDatabase engine and status
const (
	DBEngineSQLite   = "sqlite"
	DBEngineMySQL    = "mysql"
	DBEnginePostgres = "postgres"

	DBStatusProvisioning = "provisioning"
	DBStatusReady        = "ready"
	DBStatusFailed       = "failed"
	DBStatusDeleted      = "deleted"
)

var (
	ErrScriptDeploymentNotFound = errors.New("script deployment not found")
	ErrEmptyDeploymentProjectID = errors.New("deployment project id cannot be empty")
	ErrInvalidBuildStatus       = errors.New("invalid build status")
	ErrInvalidHealthStatus      = errors.New("invalid health status")

	ErrProvisionedDBNotFound = errors.New("provisioned database not found")
	ErrEmptyDBProjectID      = errors.New("provisioned database project id cannot be empty")
	ErrEmptyDBName           = errors.New("database name cannot be empty")
	ErrInvalidDBEngine       = errors.New("invalid database engine: must be sqlite, mysql, or postgres")
	ErrInvalidDBStatus       = errors.New("invalid database status: must be provisioning, ready, failed, or deleted")
)

// NormalizeBuildStatus validates and normalizes build status string.
func NormalizeBuildStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return BuildStatusPending, nil
	}
	switch norm {
	case BuildStatusPending, BuildStatusBuilding, BuildStatusSuccess, BuildStatusFailed, BuildStatusRunning, BuildStatusStopped:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidBuildStatus, status)
	}
}

// NormalizeHealthStatus validates and normalizes health status string.
func NormalizeHealthStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return HealthStatusUnknown, nil
	}
	switch norm {
	case HealthStatusUnknown, HealthStatusHealthy, HealthStatusUnhealthy, HealthStatusDegraded:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidHealthStatus, status)
	}
}

// NormalizeDBEngine validates and normalizes database engine string.
func NormalizeDBEngine(engine string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(engine))
	if norm == "" {
		return DBEngineSQLite, nil
	}
	switch norm {
	case DBEngineSQLite, DBEngineMySQL, DBEnginePostgres:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidDBEngine, engine)
	}
}

// NormalizeDBStatus validates and normalizes provisioned database status string.
func NormalizeDBStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return DBStatusProvisioning, nil
	}
	switch norm {
	case DBStatusProvisioning, DBStatusReady, DBStatusFailed, DBStatusDeleted:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidDBStatus, status)
	}
}

// ScriptDeployment represents a deployed script application record.
type ScriptDeployment struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	ServerID     *string   `json:"server_id,omitempty"`
	Port         int       `json:"port"`
	Domain       string    `json:"domain"`
	EnvEncrypted string    `json:"-"` // Never exposed in JSON
	BuildStatus  string    `json:"build_status"`
	Health       string    `json:"health"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ScriptDeploymentDTO is the public sanitized Data Transfer Object.
type ScriptDeploymentDTO struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	ServerID    *string   `json:"server_id,omitempty"`
	Port        int       `json:"port"`
	Domain      string    `json:"domain"`
	HasEnv      bool      `json:"has_env"`
	BuildStatus string    `json:"build_status"`
	Health      string    `json:"health"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToDTO converts ScriptDeployment to its public DTO with zero credential leakage.
func (s *ScriptDeployment) ToDTO() ScriptDeploymentDTO {
	buildStatus := s.BuildStatus
	if buildStatus == "" {
		buildStatus = BuildStatusPending
	}
	health := s.Health
	if health == "" {
		health = HealthStatusUnknown
	}

	return ScriptDeploymentDTO{
		ID:          s.ID,
		ProjectID:   s.ProjectID,
		ServerID:    s.ServerID,
		Port:        s.Port,
		Domain:      s.Domain,
		HasEnv:      s.EnvEncrypted != "",
		BuildStatus: buildStatus,
		Health:      health,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// EncryptEnvMap serializes and encrypts an environment variable map via AES-256-GCM.
func (s *ScriptDeployment) EncryptEnvMap(envMap map[string]string, secret string) error {
	if len(envMap) == 0 {
		s.EnvEncrypted = ""
		return nil
	}
	bytes, err := json.Marshal(envMap)
	if err != nil {
		return fmt.Errorf("marshal env map: %w", err)
	}
	enc, err := crypto.EncryptStringAES256GCM(string(bytes), secret)
	if err != nil {
		return fmt.Errorf("encrypt env: %w", err)
	}
	s.EnvEncrypted = enc
	return nil
}

// DecryptEnvMap decrypts and parses an environment variable map via AES-256-GCM.
func (s *ScriptDeployment) DecryptEnvMap(secret string) (map[string]string, error) {
	if s.EnvEncrypted == "" {
		return make(map[string]string), nil
	}
	plain, err := crypto.DecryptStringAES256GCM(s.EnvEncrypted, secret)
	if err != nil {
		return nil, fmt.Errorf("decrypt env: %w", err)
	}
	var envMap map[string]string
	if err := json.Unmarshal([]byte(plain), &envMap); err != nil {
		return nil, fmt.Errorf("unmarshal env map: %w", err)
	}
	return envMap, nil
}

// EncryptRawEnv encrypts a raw multi-line environment string via AES-256-GCM.
func (s *ScriptDeployment) EncryptRawEnv(rawEnv, secret string) error {
	if strings.TrimSpace(rawEnv) == "" {
		s.EnvEncrypted = ""
		return nil
	}
	enc, err := crypto.EncryptStringAES256GCM(rawEnv, secret)
	if err != nil {
		return fmt.Errorf("encrypt raw env: %w", err)
	}
	s.EnvEncrypted = enc
	return nil
}

// DecryptRawEnv decrypts a raw multi-line environment string via AES-256-GCM.
func (s *ScriptDeployment) DecryptRawEnv(secret string) (string, error) {
	if s.EnvEncrypted == "" {
		return "", nil
	}
	return crypto.DecryptStringAES256GCM(s.EnvEncrypted, secret)
}

// ProvisionedDatabase represents an isolated database instance created for a project.
type ProvisionedDatabase struct {
	ID                  string    `json:"id"`
	ProjectID           string    `json:"project_id"`
	ServerID            *string   `json:"server_id,omitempty"`
	Engine              string    `json:"engine"`
	DBName              string    `json:"db_name"`
	DBUser              string    `json:"db_user"`
	DBPasswordEncrypted string    `json:"-"` // Never exposed in JSON
	Status              string    `json:"status"`
	SchemaImported      bool      `json:"schema_imported"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ProvisionedDatabaseDTO is the public sanitized Data Transfer Object.
type ProvisionedDatabaseDTO struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	ServerID       *string   `json:"server_id,omitempty"`
	Engine         string    `json:"engine"`
	DBName         string    `json:"db_name"`
	DBUser         string    `json:"db_user"`
	HasPassword    bool      `json:"has_password"`
	Status         string    `json:"status"`
	SchemaImported bool      `json:"schema_imported"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ToDTO converts ProvisionedDatabase to its public DTO with zero password leakage.
func (p *ProvisionedDatabase) ToDTO() ProvisionedDatabaseDTO {
	engine := p.Engine
	if engine == "" {
		engine = DBEngineSQLite
	}
	status := p.Status
	if status == "" {
		status = DBStatusProvisioning
	}

	return ProvisionedDatabaseDTO{
		ID:             p.ID,
		ProjectID:      p.ProjectID,
		ServerID:       p.ServerID,
		Engine:         engine,
		DBName:         p.DBName,
		DBUser:         p.DBUser,
		HasPassword:    p.DBPasswordEncrypted != "",
		Status:         status,
		SchemaImported: p.SchemaImported,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// EncryptPassword encrypts plain database password using AES-256-GCM.
func (p *ProvisionedDatabase) EncryptPassword(plainPassword, secret string) error {
	if plainPassword == "" {
		p.DBPasswordEncrypted = ""
		return nil
	}
	enc, err := crypto.EncryptStringAES256GCM(plainPassword, secret)
	if err != nil {
		return fmt.Errorf("encrypt db password: %w", err)
	}
	p.DBPasswordEncrypted = enc
	return nil
}

// DecryptPassword decrypts database password using AES-256-GCM.
func (p *ProvisionedDatabase) DecryptPassword(secret string) (string, error) {
	if p.DBPasswordEncrypted == "" {
		return "", nil
	}
	return crypto.DecryptStringAES256GCM(p.DBPasswordEncrypted, secret)
}

// ScriptDeploymentRepository defines data layer operations for ScriptDeployments.
type ScriptDeploymentRepository interface {
	CreateDeployment(ctx context.Context, dep *ScriptDeployment) error
	GetDeploymentByID(ctx context.Context, id string) (*ScriptDeployment, error)
	GetDeploymentByProjectID(ctx context.Context, projectID string) (*ScriptDeployment, error)
	ListDeploymentsByProjectID(ctx context.Context, projectID string) ([]*ScriptDeployment, error)
	UpdateDeployment(ctx context.Context, dep *ScriptDeployment) error
	DeleteDeployment(ctx context.Context, id string) error
}

// ProvisionedDatabaseRepository defines data layer operations for ProvisionedDatabases.
type ProvisionedDatabaseRepository interface {
	CreateDatabase(ctx context.Context, pdb *ProvisionedDatabase) error
	GetDatabaseByID(ctx context.Context, id string) (*ProvisionedDatabase, error)
	GetDatabaseByProjectID(ctx context.Context, projectID string) (*ProvisionedDatabase, error)
	ListDatabasesByProjectID(ctx context.Context, projectID string) ([]*ProvisionedDatabase, error)
	UpdateDatabase(ctx context.Context, pdb *ProvisionedDatabase) error
	DeleteDatabase(ctx context.Context, id string) error
}

// -----------------------------------------------------------------------------
// PostgreSQL Implementation
// -----------------------------------------------------------------------------

// SQLScriptDeploymentRepository implements ScriptDeploymentRepository using PostgreSQL.
type SQLScriptDeploymentRepository struct {
	db *sql.DB
}

// NewSQLScriptDeploymentRepository constructs a new SQLScriptDeploymentRepository.
func NewSQLScriptDeploymentRepository(db *sql.DB) *SQLScriptDeploymentRepository {
	return &SQLScriptDeploymentRepository{db: db}
}

func (r *SQLScriptDeploymentRepository) CreateDeployment(ctx context.Context, dep *ScriptDeployment) error {
	if dep == nil {
		return errors.New("script deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ProjectID) == "" {
		return ErrEmptyDeploymentProjectID
	}
	if strings.TrimSpace(dep.ID) == "" {
		dep.ID = GenerateUUID()
	}

	normBuildStatus, err := NormalizeBuildStatus(dep.BuildStatus)
	if err != nil {
		return err
	}
	dep.BuildStatus = normBuildStatus

	normHealth, err := NormalizeHealthStatus(dep.Health)
	if err != nil {
		return err
	}
	dep.Health = normHealth

	now := time.Now().UTC()
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now

	var serverIDVal *string
	if dep.ServerID != nil && strings.TrimSpace(*dep.ServerID) != "" {
		val := strings.TrimSpace(*dep.ServerID)
		serverIDVal = &val
	}

	query := `
		INSERT INTO script_deployments (
			id, project_id, server_id, port, domain, env_encrypted, build_status, health, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.db.ExecContext(ctx, query,
		dep.ID,
		dep.ProjectID,
		serverIDVal,
		dep.Port,
		dep.Domain,
		dep.EnvEncrypted,
		dep.BuildStatus,
		dep.Health,
		dep.CreatedAt,
		dep.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create script deployment: %w", err)
	}
	return nil
}

func (r *SQLScriptDeploymentRepository) GetDeploymentByID(ctx context.Context, id string) (*ScriptDeployment, error) {
	query := `
		SELECT id, project_id, server_id, port, COALESCE(domain, ''), COALESCE(env_encrypted, ''),
		       COALESCE(build_status, 'pending'), COALESCE(health, 'unknown'), created_at, updated_at
		FROM script_deployments
		WHERE id = $1
	`
	var dep ScriptDeployment
	var serverID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&dep.ID,
		&dep.ProjectID,
		&serverID,
		&dep.Port,
		&dep.Domain,
		&dep.EnvEncrypted,
		&dep.BuildStatus,
		&dep.Health,
		&dep.CreatedAt,
		&dep.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScriptDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get script deployment by id: %w", err)
	}
	if serverID.Valid && serverID.String != "" {
		val := serverID.String
		dep.ServerID = &val
	}
	return &dep, nil
}

func (r *SQLScriptDeploymentRepository) GetDeploymentByProjectID(ctx context.Context, projectID string) (*ScriptDeployment, error) {
	query := `
		SELECT id, project_id, server_id, port, COALESCE(domain, ''), COALESCE(env_encrypted, ''),
		       COALESCE(build_status, 'pending'), COALESCE(health, 'unknown'), created_at, updated_at
		FROM script_deployments
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var dep ScriptDeployment
	var serverID sql.NullString
	err := r.db.QueryRowContext(ctx, query, projectID).Scan(
		&dep.ID,
		&dep.ProjectID,
		&serverID,
		&dep.Port,
		&dep.Domain,
		&dep.EnvEncrypted,
		&dep.BuildStatus,
		&dep.Health,
		&dep.CreatedAt,
		&dep.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScriptDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get script deployment by project id: %w", err)
	}
	if serverID.Valid && serverID.String != "" {
		val := serverID.String
		dep.ServerID = &val
	}
	return &dep, nil
}

func (r *SQLScriptDeploymentRepository) ListDeploymentsByProjectID(ctx context.Context, projectID string) ([]*ScriptDeployment, error) {
	query := `
		SELECT id, project_id, server_id, port, COALESCE(domain, ''), COALESCE(env_encrypted, ''),
		       COALESCE(build_status, 'pending'), COALESCE(health, 'unknown'), created_at, updated_at
		FROM script_deployments
		WHERE project_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list script deployments: %w", err)
	}
	defer rows.Close()

	var list []*ScriptDeployment
	for rows.Next() {
		var dep ScriptDeployment
		var serverID sql.NullString
		if err := rows.Scan(
			&dep.ID,
			&dep.ProjectID,
			&serverID,
			&dep.Port,
			&dep.Domain,
			&dep.EnvEncrypted,
			&dep.BuildStatus,
			&dep.Health,
			&dep.CreatedAt,
			&dep.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan script deployment: %w", err)
		}
		if serverID.Valid && serverID.String != "" {
			val := serverID.String
			dep.ServerID = &val
		}
		list = append(list, &dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate script deployments: %w", err)
	}
	return list, nil
}

func (r *SQLScriptDeploymentRepository) UpdateDeployment(ctx context.Context, dep *ScriptDeployment) error {
	if dep == nil {
		return errors.New("script deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ID) == "" {
		return ErrScriptDeploymentNotFound
	}

	normBuildStatus, err := NormalizeBuildStatus(dep.BuildStatus)
	if err != nil {
		return err
	}
	dep.BuildStatus = normBuildStatus

	normHealth, err := NormalizeHealthStatus(dep.Health)
	if err != nil {
		return err
	}
	dep.Health = normHealth
	dep.UpdatedAt = time.Now().UTC()

	var serverIDVal *string
	if dep.ServerID != nil && strings.TrimSpace(*dep.ServerID) != "" {
		val := strings.TrimSpace(*dep.ServerID)
		serverIDVal = &val
	}

	query := `
		UPDATE script_deployments
		SET server_id = $1, port = $2, domain = $3, env_encrypted = $4,
		    build_status = $5, health = $6, updated_at = $7
		WHERE id = $8
	`
	res, err := r.db.ExecContext(ctx, query,
		serverIDVal,
		dep.Port,
		dep.Domain,
		dep.EnvEncrypted,
		dep.BuildStatus,
		dep.Health,
		dep.UpdatedAt,
		dep.ID,
	)
	if err != nil {
		return fmt.Errorf("update script deployment: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return ErrScriptDeploymentNotFound
	}
	return nil
}

func (r *SQLScriptDeploymentRepository) DeleteDeployment(ctx context.Context, id string) error {
	query := `DELETE FROM script_deployments WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete script deployment: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return ErrScriptDeploymentNotFound
	}
	return nil
}

// SQLProvisionedDatabaseRepository implements ProvisionedDatabaseRepository using PostgreSQL.
type SQLProvisionedDatabaseRepository struct {
	db *sql.DB
}

// NewSQLProvisionedDatabaseRepository constructs a new SQLProvisionedDatabaseRepository.
func NewSQLProvisionedDatabaseRepository(db *sql.DB) *SQLProvisionedDatabaseRepository {
	return &SQLProvisionedDatabaseRepository{db: db}
}

func (r *SQLProvisionedDatabaseRepository) CreateDatabase(ctx context.Context, pdb *ProvisionedDatabase) error {
	if pdb == nil {
		return errors.New("provisioned database cannot be nil")
	}
	if strings.TrimSpace(pdb.ProjectID) == "" {
		return ErrEmptyDBProjectID
	}
	if strings.TrimSpace(pdb.DBName) == "" {
		return ErrEmptyDBName
	}
	if strings.TrimSpace(pdb.ID) == "" {
		pdb.ID = GenerateUUID()
	}

	normEngine, err := NormalizeDBEngine(pdb.Engine)
	if err != nil {
		return err
	}
	pdb.Engine = normEngine

	normStatus, err := NormalizeDBStatus(pdb.Status)
	if err != nil {
		return err
	}
	pdb.Status = normStatus

	now := time.Now().UTC()
	if pdb.CreatedAt.IsZero() {
		pdb.CreatedAt = now
	}
	pdb.UpdatedAt = now

	var serverIDVal *string
	if pdb.ServerID != nil && strings.TrimSpace(*pdb.ServerID) != "" {
		val := strings.TrimSpace(*pdb.ServerID)
		serverIDVal = &val
	}

	query := `
		INSERT INTO provisioned_databases (
			id, project_id, server_id, engine, db_name, db_user, db_password_encrypted,
			status, schema_imported, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.db.ExecContext(ctx, query,
		pdb.ID,
		pdb.ProjectID,
		serverIDVal,
		pdb.Engine,
		pdb.DBName,
		pdb.DBUser,
		pdb.DBPasswordEncrypted,
		pdb.Status,
		pdb.SchemaImported,
		pdb.CreatedAt,
		pdb.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create provisioned database: %w", err)
	}
	return nil
}

func (r *SQLProvisionedDatabaseRepository) GetDatabaseByID(ctx context.Context, id string) (*ProvisionedDatabase, error) {
	query := `
		SELECT id, project_id, server_id, engine, db_name, COALESCE(db_user, ''),
		       COALESCE(db_password_encrypted, ''), COALESCE(status, 'provisioning'),
		       schema_imported, created_at, updated_at
		FROM provisioned_databases
		WHERE id = $1
	`
	var pdb ProvisionedDatabase
	var serverID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&pdb.ID,
		&pdb.ProjectID,
		&serverID,
		&pdb.Engine,
		&pdb.DBName,
		&pdb.DBUser,
		&pdb.DBPasswordEncrypted,
		&pdb.Status,
		&pdb.SchemaImported,
		&pdb.CreatedAt,
		&pdb.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProvisionedDBNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provisioned database by id: %w", err)
	}
	if serverID.Valid && serverID.String != "" {
		val := serverID.String
		pdb.ServerID = &val
	}
	return &pdb, nil
}

func (r *SQLProvisionedDatabaseRepository) GetDatabaseByProjectID(ctx context.Context, projectID string) (*ProvisionedDatabase, error) {
	query := `
		SELECT id, project_id, server_id, engine, db_name, COALESCE(db_user, ''),
		       COALESCE(db_password_encrypted, ''), COALESCE(status, 'provisioning'),
		       schema_imported, created_at, updated_at
		FROM provisioned_databases
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var pdb ProvisionedDatabase
	var serverID sql.NullString
	err := r.db.QueryRowContext(ctx, query, projectID).Scan(
		&pdb.ID,
		&pdb.ProjectID,
		&serverID,
		&pdb.Engine,
		&pdb.DBName,
		&pdb.DBUser,
		&pdb.DBPasswordEncrypted,
		&pdb.Status,
		&pdb.SchemaImported,
		&pdb.CreatedAt,
		&pdb.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProvisionedDBNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get provisioned database by project id: %w", err)
	}
	if serverID.Valid && serverID.String != "" {
		val := serverID.String
		pdb.ServerID = &val
	}
	return &pdb, nil
}

func (r *SQLProvisionedDatabaseRepository) ListDatabasesByProjectID(ctx context.Context, projectID string) ([]*ProvisionedDatabase, error) {
	query := `
		SELECT id, project_id, server_id, engine, db_name, COALESCE(db_user, ''),
		       COALESCE(db_password_encrypted, ''), COALESCE(status, 'provisioning'),
		       schema_imported, created_at, updated_at
		FROM provisioned_databases
		WHERE project_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list provisioned databases: %w", err)
	}
	defer rows.Close()

	var list []*ProvisionedDatabase
	for rows.Next() {
		var pdb ProvisionedDatabase
		var serverID sql.NullString
		if err := rows.Scan(
			&pdb.ID,
			&pdb.ProjectID,
			&serverID,
			&pdb.Engine,
			&pdb.DBName,
			&pdb.DBUser,
			&pdb.DBPasswordEncrypted,
			&pdb.Status,
			&pdb.SchemaImported,
			&pdb.CreatedAt,
			&pdb.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan provisioned database: %w", err)
		}
		if serverID.Valid && serverID.String != "" {
			val := serverID.String
			pdb.ServerID = &val
		}
		list = append(list, &pdb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provisioned databases: %w", err)
	}
	return list, nil
}

func (r *SQLProvisionedDatabaseRepository) UpdateDatabase(ctx context.Context, pdb *ProvisionedDatabase) error {
	if pdb == nil {
		return errors.New("provisioned database cannot be nil")
	}
	if strings.TrimSpace(pdb.ID) == "" {
		return ErrProvisionedDBNotFound
	}

	normEngine, err := NormalizeDBEngine(pdb.Engine)
	if err != nil {
		return err
	}
	pdb.Engine = normEngine

	normStatus, err := NormalizeDBStatus(pdb.Status)
	if err != nil {
		return err
	}
	pdb.Status = normStatus
	pdb.UpdatedAt = time.Now().UTC()

	var serverIDVal *string
	if pdb.ServerID != nil && strings.TrimSpace(*pdb.ServerID) != "" {
		val := strings.TrimSpace(*pdb.ServerID)
		serverIDVal = &val
	}

	query := `
		UPDATE provisioned_databases
		SET server_id = $1, engine = $2, db_name = $3, db_user = $4,
		    db_password_encrypted = $5, status = $6, schema_imported = $7, updated_at = $8
		WHERE id = $9
	`
	res, err := r.db.ExecContext(ctx, query,
		serverIDVal,
		pdb.Engine,
		pdb.DBName,
		pdb.DBUser,
		pdb.DBPasswordEncrypted,
		pdb.Status,
		pdb.SchemaImported,
		pdb.UpdatedAt,
		pdb.ID,
	)
	if err != nil {
		return fmt.Errorf("update provisioned database: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return ErrProvisionedDBNotFound
	}
	return nil
}

func (r *SQLProvisionedDatabaseRepository) DeleteDatabase(ctx context.Context, id string) error {
	query := `DELETE FROM provisioned_databases WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete provisioned database: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected check: %w", err)
	}
	if rowsAffected == 0 {
		return ErrProvisionedDBNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// In-Memory Thread-Safe Implementation
// -----------------------------------------------------------------------------

// MemoryScriptDeploymentRepository provides thread-safe in-memory ScriptDeployment operations.
type MemoryScriptDeploymentRepository struct {
	mu          sync.RWMutex
	deployments map[string]*ScriptDeployment
}

// NewMemoryScriptDeploymentRepository creates a new in-memory ScriptDeployment repository.
func NewMemoryScriptDeploymentRepository() *MemoryScriptDeploymentRepository {
	return &MemoryScriptDeploymentRepository{
		deployments: make(map[string]*ScriptDeployment),
	}
}

func (m *MemoryScriptDeploymentRepository) CreateDeployment(ctx context.Context, dep *ScriptDeployment) error {
	if dep == nil {
		return errors.New("script deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ProjectID) == "" {
		return ErrEmptyDeploymentProjectID
	}

	normBuildStatus, err := NormalizeBuildStatus(dep.BuildStatus)
	if err != nil {
		return err
	}
	dep.BuildStatus = normBuildStatus

	normHealth, err := NormalizeHealthStatus(dep.Health)
	if err != nil {
		return err
	}
	dep.Health = normHealth

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(dep.ID) == "" {
		dep.ID = GenerateUUID()
	}

	now := time.Now().UTC()
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now

	cp := *dep
	m.deployments[dep.ID] = &cp
	return nil
}

func (m *MemoryScriptDeploymentRepository) GetDeploymentByID(ctx context.Context, id string) (*ScriptDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dep, exists := m.deployments[id]
	if !exists {
		return nil, ErrScriptDeploymentNotFound
	}
	cp := *dep
	return &cp, nil
}

func (m *MemoryScriptDeploymentRepository) GetDeploymentByProjectID(ctx context.Context, projectID string) (*ScriptDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *ScriptDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID {
			if latest == nil || dep.CreatedAt.After(latest.CreatedAt) {
				latest = dep
			}
		}
	}
	if latest == nil {
		return nil, ErrScriptDeploymentNotFound
	}
	cp := *latest
	return &cp, nil
}

func (m *MemoryScriptDeploymentRepository) ListDeploymentsByProjectID(ctx context.Context, projectID string) ([]*ScriptDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*ScriptDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID {
			cp := *dep
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (m *MemoryScriptDeploymentRepository) UpdateDeployment(ctx context.Context, dep *ScriptDeployment) error {
	if dep == nil {
		return errors.New("script deployment cannot be nil")
	}
	normBuildStatus, err := NormalizeBuildStatus(dep.BuildStatus)
	if err != nil {
		return err
	}
	dep.BuildStatus = normBuildStatus

	normHealth, err := NormalizeHealthStatus(dep.Health)
	if err != nil {
		return err
	}
	dep.Health = normHealth

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.deployments[dep.ID]
	if !exists {
		return ErrScriptDeploymentNotFound
	}

	dep.UpdatedAt = time.Now().UTC()
	dep.CreatedAt = existing.CreatedAt
	cp := *dep
	m.deployments[dep.ID] = &cp
	return nil
}

func (m *MemoryScriptDeploymentRepository) DeleteDeployment(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.deployments[id]; !exists {
		return ErrScriptDeploymentNotFound
	}
	delete(m.deployments, id)
	return nil
}

// MemoryProvisionedDatabaseRepository provides thread-safe in-memory ProvisionedDatabase operations.
type MemoryProvisionedDatabaseRepository struct {
	mu        sync.RWMutex
	databases map[string]*ProvisionedDatabase
}

// NewMemoryProvisionedDatabaseRepository creates a new in-memory ProvisionedDatabase repository.
func NewMemoryProvisionedDatabaseRepository() *MemoryProvisionedDatabaseRepository {
	return &MemoryProvisionedDatabaseRepository{
		databases: make(map[string]*ProvisionedDatabase),
	}
}

func (m *MemoryProvisionedDatabaseRepository) CreateDatabase(ctx context.Context, pdb *ProvisionedDatabase) error {
	if pdb == nil {
		return errors.New("provisioned database cannot be nil")
	}
	if strings.TrimSpace(pdb.ProjectID) == "" {
		return ErrEmptyDBProjectID
	}
	if strings.TrimSpace(pdb.DBName) == "" {
		return ErrEmptyDBName
	}

	normEngine, err := NormalizeDBEngine(pdb.Engine)
	if err != nil {
		return err
	}
	pdb.Engine = normEngine

	normStatus, err := NormalizeDBStatus(pdb.Status)
	if err != nil {
		return err
	}
	pdb.Status = normStatus

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(pdb.ID) == "" {
		pdb.ID = GenerateUUID()
	}

	now := time.Now().UTC()
	if pdb.CreatedAt.IsZero() {
		pdb.CreatedAt = now
	}
	pdb.UpdatedAt = now

	cp := *pdb
	m.databases[pdb.ID] = &cp
	return nil
}

func (m *MemoryProvisionedDatabaseRepository) GetDatabaseByID(ctx context.Context, id string) (*ProvisionedDatabase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pdb, exists := m.databases[id]
	if !exists {
		return nil, ErrProvisionedDBNotFound
	}
	cp := *pdb
	return &cp, nil
}

func (m *MemoryProvisionedDatabaseRepository) GetDatabaseByProjectID(ctx context.Context, projectID string) (*ProvisionedDatabase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *ProvisionedDatabase
	for _, pdb := range m.databases {
		if pdb.ProjectID == projectID {
			if latest == nil || pdb.CreatedAt.After(latest.CreatedAt) {
				latest = pdb
			}
		}
	}
	if latest == nil {
		return nil, ErrProvisionedDBNotFound
	}
	cp := *latest
	return &cp, nil
}

func (m *MemoryProvisionedDatabaseRepository) ListDatabasesByProjectID(ctx context.Context, projectID string) ([]*ProvisionedDatabase, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*ProvisionedDatabase
	for _, pdb := range m.databases {
		if pdb.ProjectID == projectID {
			cp := *pdb
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (m *MemoryProvisionedDatabaseRepository) UpdateDatabase(ctx context.Context, pdb *ProvisionedDatabase) error {
	if pdb == nil {
		return errors.New("provisioned database cannot be nil")
	}
	normEngine, err := NormalizeDBEngine(pdb.Engine)
	if err != nil {
		return err
	}
	pdb.Engine = normEngine

	normStatus, err := NormalizeDBStatus(pdb.Status)
	if err != nil {
		return err
	}
	pdb.Status = normStatus

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.databases[pdb.ID]
	if !exists {
		return ErrProvisionedDBNotFound
	}

	pdb.UpdatedAt = time.Now().UTC()
	pdb.CreatedAt = existing.CreatedAt
	cp := *pdb
	m.databases[pdb.ID] = &cp
	return nil
}

func (m *MemoryProvisionedDatabaseRepository) DeleteDatabase(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.databases[id]; !exists {
		return ErrProvisionedDBNotFound
	}
	delete(m.databases, id)
	return nil
}
