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

// Constants for WordPressDeployment status and health status
const (
	WPStatusPending    = "pending"
	WPStatusInstalling = "installing"
	WPStatusRunning    = "running"
	WPStatusStopped    = "stopped"
	WPStatusFailed     = "failed"

	WPHealthStatusUnknown   = "unknown"
	WPHealthStatusHealthy   = "healthy"
	WPHealthStatusUnhealthy = "unhealthy"
)

// Constants for WordPressPluginTheme kind
const (
	PluginThemeKindPlugin = "plugin"
	PluginThemeKindTheme  = "theme"
)

// Constants for WordPressWebmasterAction action_type
const (
	WebmasterActionInjectScript = "inject_script"
	WebmasterActionConfigTweak  = "config_tweak"
	WebmasterActionInstallAsset = "install_asset"
)

// Errors for WordPress Data Layer
var (
	ErrWordPressDeploymentNotFound      = errors.New("wordpress deployment not found")
	ErrWordPressPluginThemeNotFound     = errors.New("wordpress plugin/theme not found")
	ErrWordPressWebmasterActionNotFound = errors.New("wordpress webmaster action not found")

	ErrEmptyWordPressProjectID      = errors.New("wordpress project id cannot be empty")
	ErrEmptyWordPressDeploymentID   = errors.New("wordpress deployment id cannot be empty")
	ErrEmptyWordPressSiteTitle      = errors.New("wordpress site title cannot be empty")
	ErrEmptyWordPressAdminEmail     = errors.New("wordpress admin email cannot be empty")
	ErrEmptyWordPressAdminUsername  = errors.New("wordpress admin username cannot be empty")
	ErrEmptyWordPressDomain         = errors.New("wordpress domain cannot be empty")
	ErrEmptyWordPressPluginSlug     = errors.New("wordpress plugin/theme slug cannot be empty")
	ErrEmptyWordPressPluginName     = errors.New("wordpress plugin/theme name cannot be empty")

	ErrInvalidWPStatus            = errors.New("invalid wordpress status: must be pending, installing, running, stopped, or failed")
	ErrInvalidWPHealthStatus      = errors.New("invalid wordpress health status: must be unknown, healthy, or unhealthy")
	ErrInvalidPluginThemeKind     = errors.New("invalid plugin/theme kind: must be plugin or theme")
	ErrInvalidWebmasterActionType = errors.New("invalid webmaster action type: must be inject_script, config_tweak, or install_asset")
)

// NormalizeWPStatus validates and normalizes the WordPress deployment status string.
func NormalizeWPStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return WPStatusPending, nil
	}
	switch norm {
	case WPStatusPending, WPStatusInstalling, WPStatusRunning, WPStatusStopped, WPStatusFailed:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidWPStatus, status)
	}
}

// NormalizeWPHealthStatus validates and normalizes the WordPress deployment health status string.
func NormalizeWPHealthStatus(health string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(health))
	if norm == "" {
		return WPHealthStatusUnknown, nil
	}
	switch norm {
	case WPHealthStatusUnknown, WPHealthStatusHealthy, WPHealthStatusUnhealthy:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidWPHealthStatus, health)
	}
}

// NormalizePluginThemeKind validates and normalizes plugin/theme kind.
func NormalizePluginThemeKind(kind string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(kind))
	switch norm {
	case PluginThemeKindPlugin, PluginThemeKindTheme:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidPluginThemeKind, kind)
	}
}

// NormalizeWebmasterActionType validates and normalizes webmaster action type.
func NormalizeWebmasterActionType(actionType string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(actionType))
	switch norm {
	case WebmasterActionInjectScript, WebmasterActionConfigTweak, WebmasterActionInstallAsset:
		return norm, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrInvalidWebmasterActionType, actionType)
	}
}

// WordPressDeployment represents a deployed WordPress site instance record.
type WordPressDeployment struct {
	ID                     string    `json:"id"`
	ProjectID              string    `json:"project_id"`
	SiteTitle              string    `json:"site_title"`
	AdminEmail             string    `json:"admin_email"`
	AdminUsername          string    `json:"admin_username"`
	AdminPasswordEncrypted string    `json:"-"` // Never exposed in JSON
	DBName                 string    `json:"db_name"`
	DBUser                 string    `json:"db_user"`
	DBPasswordEncrypted    string    `json:"-"` // Never exposed in JSON
	DBHost                 string    `json:"db_host"`
	DBPort                 int       `json:"db_port"`
	Domain                 string    `json:"domain"`
	Port                   int       `json:"port"`
	TablePrefix            string    `json:"table_prefix"`
	WPVersion              string    `json:"wp_version"`
	Status                 string    `json:"status"`
	HealthStatus           string    `json:"health_status"`
	PermalinkStructure     string    `json:"permalink_structure"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// WordPressDeploymentDTO is the public sanitized Data Transfer Object with zero credential leakage.
type WordPressDeploymentDTO struct {
	ID                 string    `json:"id"`
	ProjectID          string    `json:"project_id"`
	SiteTitle          string    `json:"site_title"`
	AdminEmail         string    `json:"admin_email"`
	AdminUsername      string    `json:"admin_username"`
	HasAdminPassword   bool      `json:"has_admin_password"`
	DBName             string    `json:"db_name"`
	DBUser             string    `json:"db_user"`
	HasDBPassword      bool      `json:"has_db_password"`
	DBHost             string    `json:"db_host"`
	DBPort             int       `json:"db_port"`
	Domain             string    `json:"domain"`
	Port               int       `json:"port"`
	TablePrefix        string    `json:"table_prefix"`
	WPVersion          string    `json:"wp_version"`
	Status             string    `json:"status"`
	HealthStatus       string    `json:"health_status"`
	PermalinkStructure string    `json:"permalink_structure"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ToDTO converts WordPressDeployment to its public DTO with zero password leakage.
func (d *WordPressDeployment) ToDTO() WordPressDeploymentDTO {
	status := d.Status
	if status == "" {
		status = WPStatusPending
	}
	health := d.HealthStatus
	if health == "" {
		health = WPHealthStatusUnknown
	}
	dbHost := d.DBHost
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := d.DBPort
	if dbPort <= 0 {
		dbPort = 3306
	}
	tablePrefix := d.TablePrefix
	if tablePrefix == "" {
		tablePrefix = "wp_"
	}
	wpVersion := d.WPVersion
	if wpVersion == "" {
		wpVersion = "latest"
	}
	permalink := d.PermalinkStructure
	if permalink == "" {
		permalink = "/%postname%/"
	}

	return WordPressDeploymentDTO{
		ID:                 d.ID,
		ProjectID:          d.ProjectID,
		SiteTitle:          d.SiteTitle,
		AdminEmail:         d.AdminEmail,
		AdminUsername:      d.AdminUsername,
		HasAdminPassword:   d.AdminPasswordEncrypted != "",
		DBName:             d.DBName,
		DBUser:             d.DBUser,
		HasDBPassword:      d.DBPasswordEncrypted != "",
		DBHost:             dbHost,
		DBPort:             dbPort,
		Domain:             d.Domain,
		Port:               d.Port,
		TablePrefix:        tablePrefix,
		WPVersion:          wpVersion,
		Status:             status,
		HealthStatus:       health,
		PermalinkStructure: permalink,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

// EncryptAdminPassword encrypts plaintext WordPress admin password using AES-256-GCM.
func (d *WordPressDeployment) EncryptAdminPassword(plainPassword, secret string) error {
	if plainPassword == "" {
		d.AdminPasswordEncrypted = ""
		return nil
	}
	enc, err := crypto.EncryptStringAES256GCM(plainPassword, secret)
	if err != nil {
		return fmt.Errorf("encrypt admin password: %w", err)
	}
	d.AdminPasswordEncrypted = enc
	return nil
}

// DecryptAdminPassword decrypts WordPress admin password using AES-256-GCM.
func (d *WordPressDeployment) DecryptAdminPassword(secret string) (string, error) {
	if d.AdminPasswordEncrypted == "" {
		return "", nil
	}
	return crypto.DecryptStringAES256GCM(d.AdminPasswordEncrypted, secret)
}

// EncryptDBPassword encrypts plaintext database password using AES-256-GCM.
func (d *WordPressDeployment) EncryptDBPassword(plainPassword, secret string) error {
	if plainPassword == "" {
		d.DBPasswordEncrypted = ""
		return nil
	}
	enc, err := crypto.EncryptStringAES256GCM(plainPassword, secret)
	if err != nil {
		return fmt.Errorf("encrypt db password: %w", err)
	}
	d.DBPasswordEncrypted = enc
	return nil
}

// DecryptDBPassword decrypts database password using AES-256-GCM.
func (d *WordPressDeployment) DecryptDBPassword(secret string) (string, error) {
	if d.DBPasswordEncrypted == "" {
		return "", nil
	}
	return crypto.DecryptStringAES256GCM(d.DBPasswordEncrypted, secret)
}

// WordPressPluginTheme represents an installed plugin or theme on a WordPress deployment.
type WordPressPluginTheme struct {
	ID           string    `json:"id"`
	DeploymentID string    `json:"deployment_id"`
	Kind         string    `json:"kind"` // plugin, theme
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Version      string    `json:"version,omitempty"`
	IsActive     bool      `json:"is_active"`
	SourceZip    string    `json:"source_zip,omitempty"`
	InstalledAt  time.Time `json:"installed_at"`
}

// WordPressWebmasterAction represents an audit log entry for actions executed by the AI Webmaster.
type WordPressWebmasterAction struct {
	ID           string          `json:"id"`
	DeploymentID string          `json:"deployment_id"`
	ActionType   string          `json:"action_type"` // inject_script, config_tweak, install_asset
	Payload      json.RawMessage `json:"payload"`
	AppliedAt    time.Time       `json:"applied_at"`
}

// WordPressRepository defines complete CRUD operations for WordPress turnkey deployment, plugins/themes, and webmaster actions.
type WordPressRepository interface {
	// Deployments
	CreateDeployment(ctx context.Context, dep *WordPressDeployment) error
	GetDeploymentByID(ctx context.Context, id string) (*WordPressDeployment, error)
	GetDeploymentByProjectID(ctx context.Context, projectID string) (*WordPressDeployment, error)
	ListDeployments(ctx context.Context) ([]*WordPressDeployment, error)
	ListDeploymentsByProjectID(ctx context.Context, projectID string) ([]*WordPressDeployment, error)
	UpdateDeployment(ctx context.Context, dep *WordPressDeployment) error
	UpdateDeploymentStatus(ctx context.Context, id, status string) error
	UpdateDeploymentHealth(ctx context.Context, id, healthStatus string) error
	DeleteDeployment(ctx context.Context, id string) error

	// Plugins and Themes
	CreatePluginTheme(ctx context.Context, pt *WordPressPluginTheme) error
	GetPluginThemeByID(ctx context.Context, id string) (*WordPressPluginTheme, error)
	ListPluginsThemesByDeploymentID(ctx context.Context, deploymentID string, kind string) ([]*WordPressPluginTheme, error)
	UpdatePluginThemeStatus(ctx context.Context, id string, isActive bool) error
	DeletePluginTheme(ctx context.Context, id string) error

	// Webmaster Actions
	RecordWebmasterAction(ctx context.Context, act *WordPressWebmasterAction) error
	GetWebmasterActionByID(ctx context.Context, id string) (*WordPressWebmasterAction, error)
	ListWebmasterActionsByDeploymentID(ctx context.Context, deploymentID string) ([]*WordPressWebmasterAction, error)
}

// -----------------------------------------------------------------------------
// PostgreSQL Implementation
// -----------------------------------------------------------------------------

// SQLWordPressRepository implements WordPressRepository using PostgreSQL.
type SQLWordPressRepository struct {
	db *sql.DB
}

// NewSQLWordPressRepository constructs a new SQLWordPressRepository.
func NewSQLWordPressRepository(db *sql.DB) *SQLWordPressRepository {
	return &SQLWordPressRepository{db: db}
}

func (r *SQLWordPressRepository) CreateDeployment(ctx context.Context, dep *WordPressDeployment) error {
	if dep == nil {
		return errors.New("wordpress deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ProjectID) == "" {
		return ErrEmptyWordPressProjectID
	}
	if strings.TrimSpace(dep.SiteTitle) == "" {
		return ErrEmptyWordPressSiteTitle
	}
	if strings.TrimSpace(dep.AdminEmail) == "" {
		return ErrEmptyWordPressAdminEmail
	}
	if strings.TrimSpace(dep.AdminUsername) == "" {
		return ErrEmptyWordPressAdminUsername
	}
	if strings.TrimSpace(dep.Domain) == "" {
		return ErrEmptyWordPressDomain
	}

	normStatus, err := NormalizeWPStatus(dep.Status)
	if err != nil {
		return err
	}
	dep.Status = normStatus

	normHealth, err := NormalizeWPHealthStatus(dep.HealthStatus)
	if err != nil {
		return err
	}
	dep.HealthStatus = normHealth

	if strings.TrimSpace(dep.ID) == "" {
		dep.ID = GenerateUUID()
	}
	if strings.TrimSpace(dep.DBHost) == "" {
		dep.DBHost = "127.0.0.1"
	}
	if dep.DBPort <= 0 {
		dep.DBPort = 3306
	}
	if strings.TrimSpace(dep.TablePrefix) == "" {
		dep.TablePrefix = "wp_"
	}
	if strings.TrimSpace(dep.WPVersion) == "" {
		dep.WPVersion = "latest"
	}
	if strings.TrimSpace(dep.PermalinkStructure) == "" {
		dep.PermalinkStructure = "/%postname%/"
	}

	now := time.Now().UTC()
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now

	query := `
		INSERT INTO wordpress_deployments (
			id, project_id, site_title, admin_email, admin_username, admin_password_encrypted,
			db_name, db_user, db_password_encrypted, db_host, db_port,
			domain, port, table_prefix, wp_version, status, health_status, permalink_structure,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
	`
	_, err = r.db.ExecContext(ctx, query,
		dep.ID,
		dep.ProjectID,
		dep.SiteTitle,
		dep.AdminEmail,
		dep.AdminUsername,
		dep.AdminPasswordEncrypted,
		dep.DBName,
		dep.DBUser,
		dep.DBPasswordEncrypted,
		dep.DBHost,
		dep.DBPort,
		dep.Domain,
		dep.Port,
		dep.TablePrefix,
		dep.WPVersion,
		dep.Status,
		dep.HealthStatus,
		dep.PermalinkStructure,
		dep.CreatedAt,
		dep.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create wordpress deployment: %w", err)
	}
	return nil
}

func (r *SQLWordPressRepository) GetDeploymentByID(ctx context.Context, id string) (*WordPressDeployment, error) {
	query := `
		SELECT id, project_id, site_title, admin_email, admin_username, admin_password_encrypted,
		       db_name, db_user, db_password_encrypted, db_host, db_port,
		       domain, port, table_prefix, wp_version, status, health_status, permalink_structure,
		       created_at, updated_at
		FROM wordpress_deployments
		WHERE id = $1
	`
	var dep WordPressDeployment
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&dep.ID,
		&dep.ProjectID,
		&dep.SiteTitle,
		&dep.AdminEmail,
		&dep.AdminUsername,
		&dep.AdminPasswordEncrypted,
		&dep.DBName,
		&dep.DBUser,
		&dep.DBPasswordEncrypted,
		&dep.DBHost,
		&dep.DBPort,
		&dep.Domain,
		&dep.Port,
		&dep.TablePrefix,
		&dep.WPVersion,
		&dep.Status,
		&dep.HealthStatus,
		&dep.PermalinkStructure,
		&dep.CreatedAt,
		&dep.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWordPressDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wordpress deployment by id: %w", err)
	}
	return &dep, nil
}

func (r *SQLWordPressRepository) GetDeploymentByProjectID(ctx context.Context, projectID string) (*WordPressDeployment, error) {
	query := `
		SELECT id, project_id, site_title, admin_email, admin_username, admin_password_encrypted,
		       db_name, db_user, db_password_encrypted, db_host, db_port,
		       domain, port, table_prefix, wp_version, status, health_status, permalink_structure,
		       created_at, updated_at
		FROM wordpress_deployments
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var dep WordPressDeployment
	err := r.db.QueryRowContext(ctx, query, projectID).Scan(
		&dep.ID,
		&dep.ProjectID,
		&dep.SiteTitle,
		&dep.AdminEmail,
		&dep.AdminUsername,
		&dep.AdminPasswordEncrypted,
		&dep.DBName,
		&dep.DBUser,
		&dep.DBPasswordEncrypted,
		&dep.DBHost,
		&dep.DBPort,
		&dep.Domain,
		&dep.Port,
		&dep.TablePrefix,
		&dep.WPVersion,
		&dep.Status,
		&dep.HealthStatus,
		&dep.PermalinkStructure,
		&dep.CreatedAt,
		&dep.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWordPressDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wordpress deployment by project id: %w", err)
	}
	return &dep, nil
}

func (r *SQLWordPressRepository) ListDeployments(ctx context.Context) ([]*WordPressDeployment, error) {
	query := `
		SELECT id, project_id, site_title, admin_email, admin_username, admin_password_encrypted,
		       db_name, db_user, db_password_encrypted, db_host, db_port,
		       domain, port, table_prefix, wp_version, status, health_status, permalink_structure,
		       created_at, updated_at
		FROM wordpress_deployments
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list wordpress deployments: %w", err)
	}
	defer rows.Close()

	var list []*WordPressDeployment
	for rows.Next() {
		var dep WordPressDeployment
		if err := rows.Scan(
			&dep.ID,
			&dep.ProjectID,
			&dep.SiteTitle,
			&dep.AdminEmail,
			&dep.AdminUsername,
			&dep.AdminPasswordEncrypted,
			&dep.DBName,
			&dep.DBUser,
			&dep.DBPasswordEncrypted,
			&dep.DBHost,
			&dep.DBPort,
			&dep.Domain,
			&dep.Port,
			&dep.TablePrefix,
			&dep.WPVersion,
			&dep.Status,
			&dep.HealthStatus,
			&dep.PermalinkStructure,
			&dep.CreatedAt,
			&dep.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan wordpress deployment: %w", err)
		}
		list = append(list, &dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iterate wordpress deployments: %w", err)
	}
	return list, nil
}

func (r *SQLWordPressRepository) ListDeploymentsByProjectID(ctx context.Context, projectID string) ([]*WordPressDeployment, error) {
	query := `
		SELECT id, project_id, site_title, admin_email, admin_username, admin_password_encrypted,
		       db_name, db_user, db_password_encrypted, db_host, db_port,
		       domain, port, table_prefix, wp_version, status, health_status, permalink_structure,
		       created_at, updated_at
		FROM wordpress_deployments
		WHERE project_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list wordpress deployments by project id: %w", err)
	}
	defer rows.Close()

	var list []*WordPressDeployment
	for rows.Next() {
		var dep WordPressDeployment
		if err := rows.Scan(
			&dep.ID,
			&dep.ProjectID,
			&dep.SiteTitle,
			&dep.AdminEmail,
			&dep.AdminUsername,
			&dep.AdminPasswordEncrypted,
			&dep.DBName,
			&dep.DBUser,
			&dep.DBPasswordEncrypted,
			&dep.DBHost,
			&dep.DBPort,
			&dep.Domain,
			&dep.Port,
			&dep.TablePrefix,
			&dep.WPVersion,
			&dep.Status,
			&dep.HealthStatus,
			&dep.PermalinkStructure,
			&dep.CreatedAt,
			&dep.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan wordpress deployment: %w", err)
		}
		list = append(list, &dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iterate wordpress deployments: %w", err)
	}
	return list, nil
}

func (r *SQLWordPressRepository) UpdateDeployment(ctx context.Context, dep *WordPressDeployment) error {
	if dep == nil {
		return errors.New("wordpress deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ID) == "" {
		return ErrEmptyWordPressDeploymentID
	}

	normStatus, err := NormalizeWPStatus(dep.Status)
	if err != nil {
		return err
	}
	dep.Status = normStatus

	normHealth, err := NormalizeWPHealthStatus(dep.HealthStatus)
	if err != nil {
		return err
	}
	dep.HealthStatus = normHealth

	dep.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE wordpress_deployments
		SET site_title = $1, admin_email = $2, admin_username = $3, admin_password_encrypted = $4,
		    db_name = $5, db_user = $6, db_password_encrypted = $7, db_host = $8, db_port = $9,
		    domain = $10, port = $11, table_prefix = $12, wp_version = $13,
		    status = $14, health_status = $15, permalink_structure = $16, updated_at = $17
		WHERE id = $18
	`
	res, err := r.db.ExecContext(ctx, query,
		dep.SiteTitle,
		dep.AdminEmail,
		dep.AdminUsername,
		dep.AdminPasswordEncrypted,
		dep.DBName,
		dep.DBUser,
		dep.DBPasswordEncrypted,
		dep.DBHost,
		dep.DBPort,
		dep.Domain,
		dep.Port,
		dep.TablePrefix,
		dep.WPVersion,
		dep.Status,
		dep.HealthStatus,
		dep.PermalinkStructure,
		dep.UpdatedAt,
		dep.ID,
	)
	if err != nil {
		return fmt.Errorf("update wordpress deployment: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected on update: %w", err)
	}
	if affected == 0 {
		return ErrWordPressDeploymentNotFound
	}
	return nil
}

func (r *SQLWordPressRepository) UpdateDeploymentStatus(ctx context.Context, id, status string) error {
	normStatus, err := NormalizeWPStatus(status)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	query := `UPDATE wordpress_deployments SET status = $1, updated_at = $2 WHERE id = $3`
	res, err := r.db.ExecContext(ctx, query, normStatus, now, id)
	if err != nil {
		return fmt.Errorf("update wordpress deployment status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return ErrWordPressDeploymentNotFound
	}
	return nil
}

func (r *SQLWordPressRepository) UpdateDeploymentHealth(ctx context.Context, id, healthStatus string) error {
	normHealth, err := NormalizeWPHealthStatus(healthStatus)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	query := `UPDATE wordpress_deployments SET health_status = $1, updated_at = $2 WHERE id = $3`
	res, err := r.db.ExecContext(ctx, query, normHealth, now, id)
	if err != nil {
		return fmt.Errorf("update wordpress deployment health: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return ErrWordPressDeploymentNotFound
	}
	return nil
}

func (r *SQLWordPressRepository) DeleteDeployment(ctx context.Context, id string) error {
	query := `DELETE FROM wordpress_deployments WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete wordpress deployment: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return ErrWordPressDeploymentNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// Plugins & Themes SQL Operations
// -----------------------------------------------------------------------------

func (r *SQLWordPressRepository) CreatePluginTheme(ctx context.Context, pt *WordPressPluginTheme) error {
	if pt == nil {
		return errors.New("plugin/theme item cannot be nil")
	}
	if strings.TrimSpace(pt.DeploymentID) == "" {
		return ErrEmptyWordPressDeploymentID
	}
	if strings.TrimSpace(pt.Slug) == "" {
		return ErrEmptyWordPressPluginSlug
	}
	if strings.TrimSpace(pt.Name) == "" {
		return ErrEmptyWordPressPluginName
	}
	normKind, err := NormalizePluginThemeKind(pt.Kind)
	if err != nil {
		return err
	}
	pt.Kind = normKind

	if strings.TrimSpace(pt.ID) == "" {
		pt.ID = GenerateUUID()
	}
	if pt.InstalledAt.IsZero() {
		pt.InstalledAt = time.Now().UTC()
	}

	query := `
		INSERT INTO wordpress_plugins_themes (
			id, deployment_id, kind, slug, name, version, is_active, source_zip, installed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.db.ExecContext(ctx, query,
		pt.ID,
		pt.DeploymentID,
		pt.Kind,
		pt.Slug,
		pt.Name,
		pt.Version,
		pt.IsActive,
		pt.SourceZip,
		pt.InstalledAt,
	)
	if err != nil {
		return fmt.Errorf("create wordpress plugin/theme: %w", err)
	}
	return nil
}

func (r *SQLWordPressRepository) GetPluginThemeByID(ctx context.Context, id string) (*WordPressPluginTheme, error) {
	query := `
		SELECT id, deployment_id, kind, slug, name, COALESCE(version, ''), is_active, COALESCE(source_zip, ''), installed_at
		FROM wordpress_plugins_themes
		WHERE id = $1
	`
	var pt WordPressPluginTheme
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&pt.ID,
		&pt.DeploymentID,
		&pt.Kind,
		&pt.Slug,
		&pt.Name,
		&pt.Version,
		&pt.IsActive,
		&pt.SourceZip,
		&pt.InstalledAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWordPressPluginThemeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wordpress plugin/theme by id: %w", err)
	}
	return &pt, nil
}

func (r *SQLWordPressRepository) ListPluginsThemesByDeploymentID(ctx context.Context, deploymentID string, kind string) ([]*WordPressPluginTheme, error) {
	var query string
	var args []interface{}
	args = append(args, deploymentID)

	if strings.TrimSpace(kind) != "" {
		normKind, err := NormalizePluginThemeKind(kind)
		if err != nil {
			return nil, err
		}
		query = `
			SELECT id, deployment_id, kind, slug, name, COALESCE(version, ''), is_active, COALESCE(source_zip, ''), installed_at
			FROM wordpress_plugins_themes
			WHERE deployment_id = $1 AND kind = $2
			ORDER BY installed_at DESC
		`
		args = append(args, normKind)
	} else {
		query = `
			SELECT id, deployment_id, kind, slug, name, COALESCE(version, ''), is_active, COALESCE(source_zip, ''), installed_at
			FROM wordpress_plugins_themes
			WHERE deployment_id = $1
			ORDER BY installed_at DESC
		`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list wordpress plugins/themes: %w", err)
	}
	defer rows.Close()

	var list []*WordPressPluginTheme
	for rows.Next() {
		var pt WordPressPluginTheme
		if err := rows.Scan(
			&pt.ID,
			&pt.DeploymentID,
			&pt.Kind,
			&pt.Slug,
			&pt.Name,
			&pt.Version,
			&pt.IsActive,
			&pt.SourceZip,
			&pt.InstalledAt,
		); err != nil {
			return nil, fmt.Errorf("scan wordpress plugin/theme: %w", err)
		}
		list = append(list, &pt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iterate wordpress plugins/themes: %w", err)
	}
	return list, nil
}

func (r *SQLWordPressRepository) UpdatePluginThemeStatus(ctx context.Context, id string, isActive bool) error {
	query := `UPDATE wordpress_plugins_themes SET is_active = $1 WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, isActive, id)
	if err != nil {
		return fmt.Errorf("update wordpress plugin/theme status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return ErrWordPressPluginThemeNotFound
	}
	return nil
}

func (r *SQLWordPressRepository) DeletePluginTheme(ctx context.Context, id string) error {
	query := `DELETE FROM wordpress_plugins_themes WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete wordpress plugin/theme: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if affected == 0 {
		return ErrWordPressPluginThemeNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// Webmaster Actions SQL Operations
// -----------------------------------------------------------------------------

func (r *SQLWordPressRepository) RecordWebmasterAction(ctx context.Context, act *WordPressWebmasterAction) error {
	if act == nil {
		return errors.New("webmaster action cannot be nil")
	}
	if strings.TrimSpace(act.DeploymentID) == "" {
		return ErrEmptyWordPressDeploymentID
	}
	normActionType, err := NormalizeWebmasterActionType(act.ActionType)
	if err != nil {
		return err
	}
	act.ActionType = normActionType

	if strings.TrimSpace(act.ID) == "" {
		act.ID = GenerateUUID()
	}
	if act.AppliedAt.IsZero() {
		act.AppliedAt = time.Now().UTC()
	}
	if len(act.Payload) == 0 {
		act.Payload = json.RawMessage("{}")
	}

	query := `
		INSERT INTO wordpress_webmaster_actions (
			id, deployment_id, action_type, payload, applied_at
		) VALUES ($1, $2, $3, $4, $5)
	`
	_, err = r.db.ExecContext(ctx, query,
		act.ID,
		act.DeploymentID,
		act.ActionType,
		act.Payload,
		act.AppliedAt,
	)
	if err != nil {
		return fmt.Errorf("record webmaster action: %w", err)
	}
	return nil
}

func (r *SQLWordPressRepository) GetWebmasterActionByID(ctx context.Context, id string) (*WordPressWebmasterAction, error) {
	query := `
		SELECT id, deployment_id, action_type, payload, applied_at
		FROM wordpress_webmaster_actions
		WHERE id = $1
	`
	var act WordPressWebmasterAction
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&act.ID,
		&act.DeploymentID,
		&act.ActionType,
		&act.Payload,
		&act.AppliedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWordPressWebmasterActionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get webmaster action by id: %w", err)
	}
	return &act, nil
}

func (r *SQLWordPressRepository) ListWebmasterActionsByDeploymentID(ctx context.Context, deploymentID string) ([]*WordPressWebmasterAction, error) {
	query := `
		SELECT id, deployment_id, action_type, payload, applied_at
		FROM wordpress_webmaster_actions
		WHERE deployment_id = $1
		ORDER BY applied_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("list webmaster actions: %w", err)
	}
	defer rows.Close()

	var list []*WordPressWebmasterAction
	for rows.Next() {
		var act WordPressWebmasterAction
		if err := rows.Scan(
			&act.ID,
			&act.DeploymentID,
			&act.ActionType,
			&act.Payload,
			&act.AppliedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webmaster action: %w", err)
		}
		list = append(list, &act)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iterate webmaster actions: %w", err)
	}
	return list, nil
}

// -----------------------------------------------------------------------------
// In-Memory Implementation (Thread-Safe)
// -----------------------------------------------------------------------------

// MemoryWordPressRepository implements WordPressRepository with an in-memory thread-safe map.
type MemoryWordPressRepository struct {
	mu           sync.RWMutex
	deployments  map[string]*WordPressDeployment
	pluginThemes map[string]*WordPressPluginTheme
	actions      map[string]*WordPressWebmasterAction
}

// NewMemoryWordPressRepository constructs a new MemoryWordPressRepository.
func NewMemoryWordPressRepository() *MemoryWordPressRepository {
	return &MemoryWordPressRepository{
		deployments:  make(map[string]*WordPressDeployment),
		pluginThemes: make(map[string]*WordPressPluginTheme),
		actions:      make(map[string]*WordPressWebmasterAction),
	}
}

func (m *MemoryWordPressRepository) CreateDeployment(ctx context.Context, dep *WordPressDeployment) error {
	if dep == nil {
		return errors.New("wordpress deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ProjectID) == "" {
		return ErrEmptyWordPressProjectID
	}
	if strings.TrimSpace(dep.SiteTitle) == "" {
		return ErrEmptyWordPressSiteTitle
	}
	if strings.TrimSpace(dep.AdminEmail) == "" {
		return ErrEmptyWordPressAdminEmail
	}
	if strings.TrimSpace(dep.AdminUsername) == "" {
		return ErrEmptyWordPressAdminUsername
	}
	if strings.TrimSpace(dep.Domain) == "" {
		return ErrEmptyWordPressDomain
	}

	normStatus, err := NormalizeWPStatus(dep.Status)
	if err != nil {
		return err
	}
	dep.Status = normStatus

	normHealth, err := NormalizeWPHealthStatus(dep.HealthStatus)
	if err != nil {
		return err
	}
	dep.HealthStatus = normHealth

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(dep.ID) == "" {
		dep.ID = GenerateUUID()
	}
	if strings.TrimSpace(dep.DBHost) == "" {
		dep.DBHost = "127.0.0.1"
	}
	if dep.DBPort <= 0 {
		dep.DBPort = 3306
	}
	if strings.TrimSpace(dep.TablePrefix) == "" {
		dep.TablePrefix = "wp_"
	}
	if strings.TrimSpace(dep.WPVersion) == "" {
		dep.WPVersion = "latest"
	}
	if strings.TrimSpace(dep.PermalinkStructure) == "" {
		dep.PermalinkStructure = "/%postname%/"
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

func (m *MemoryWordPressRepository) GetDeploymentByID(ctx context.Context, id string) (*WordPressDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dep, exists := m.deployments[id]
	if !exists {
		return nil, ErrWordPressDeploymentNotFound
	}
	cp := *dep
	return &cp, nil
}

func (m *MemoryWordPressRepository) GetDeploymentByProjectID(ctx context.Context, projectID string) (*WordPressDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *WordPressDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID {
			if latest == nil || dep.CreatedAt.After(latest.CreatedAt) {
				latest = dep
			}
		}
	}
	if latest == nil {
		return nil, ErrWordPressDeploymentNotFound
	}
	cp := *latest
	return &cp, nil
}

func (m *MemoryWordPressRepository) ListDeployments(ctx context.Context) ([]*WordPressDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*WordPressDeployment
	for _, dep := range m.deployments {
		cp := *dep
		list = append(list, &cp)
	}
	return list, nil
}

func (m *MemoryWordPressRepository) ListDeploymentsByProjectID(ctx context.Context, projectID string) ([]*WordPressDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*WordPressDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID {
			cp := *dep
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (m *MemoryWordPressRepository) UpdateDeployment(ctx context.Context, dep *WordPressDeployment) error {
	if dep == nil {
		return errors.New("wordpress deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ID) == "" {
		return ErrEmptyWordPressDeploymentID
	}
	normStatus, err := NormalizeWPStatus(dep.Status)
	if err != nil {
		return err
	}
	dep.Status = normStatus

	normHealth, err := NormalizeWPHealthStatus(dep.HealthStatus)
	if err != nil {
		return err
	}
	dep.HealthStatus = normHealth

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.deployments[dep.ID]
	if !exists {
		return ErrWordPressDeploymentNotFound
	}

	dep.CreatedAt = existing.CreatedAt
	dep.UpdatedAt = time.Now().UTC()

	cp := *dep
	m.deployments[dep.ID] = &cp
	return nil
}

func (m *MemoryWordPressRepository) UpdateDeploymentStatus(ctx context.Context, id, status string) error {
	normStatus, err := NormalizeWPStatus(status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.deployments[id]
	if !exists {
		return ErrWordPressDeploymentNotFound
	}
	existing.Status = normStatus
	existing.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryWordPressRepository) UpdateDeploymentHealth(ctx context.Context, id, healthStatus string) error {
	normHealth, err := NormalizeWPHealthStatus(healthStatus)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.deployments[id]
	if !exists {
		return ErrWordPressDeploymentNotFound
	}
	existing.HealthStatus = normHealth
	existing.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryWordPressRepository) DeleteDeployment(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.deployments[id]; !exists {
		return ErrWordPressDeploymentNotFound
	}
	delete(m.deployments, id)

	// Cascade delete plugins/themes and actions for this deployment
	for ptID, pt := range m.pluginThemes {
		if pt.DeploymentID == id {
			delete(m.pluginThemes, ptID)
		}
	}
	for actID, act := range m.actions {
		if act.DeploymentID == id {
			delete(m.actions, actID)
		}
	}
	return nil
}

func (m *MemoryWordPressRepository) CreatePluginTheme(ctx context.Context, pt *WordPressPluginTheme) error {
	if pt == nil {
		return errors.New("plugin/theme item cannot be nil")
	}
	if strings.TrimSpace(pt.DeploymentID) == "" {
		return ErrEmptyWordPressDeploymentID
	}
	if strings.TrimSpace(pt.Slug) == "" {
		return ErrEmptyWordPressPluginSlug
	}
	if strings.TrimSpace(pt.Name) == "" {
		return ErrEmptyWordPressPluginName
	}
	normKind, err := NormalizePluginThemeKind(pt.Kind)
	if err != nil {
		return err
	}
	pt.Kind = normKind

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(pt.ID) == "" {
		pt.ID = GenerateUUID()
	}
	if pt.InstalledAt.IsZero() {
		pt.InstalledAt = time.Now().UTC()
	}

	cp := *pt
	m.pluginThemes[pt.ID] = &cp
	return nil
}

func (m *MemoryWordPressRepository) GetPluginThemeByID(ctx context.Context, id string) (*WordPressPluginTheme, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pt, exists := m.pluginThemes[id]
	if !exists {
		return nil, ErrWordPressPluginThemeNotFound
	}
	cp := *pt
	return &cp, nil
}

func (m *MemoryWordPressRepository) ListPluginsThemesByDeploymentID(ctx context.Context, deploymentID string, kind string) ([]*WordPressPluginTheme, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var normKind string
	if strings.TrimSpace(kind) != "" {
		k, err := NormalizePluginThemeKind(kind)
		if err != nil {
			return nil, err
		}
		normKind = k
	}

	var list []*WordPressPluginTheme
	for _, pt := range m.pluginThemes {
		if pt.DeploymentID == deploymentID {
			if normKind == "" || pt.Kind == normKind {
				cp := *pt
				list = append(list, &cp)
			}
		}
	}
	return list, nil
}

func (m *MemoryWordPressRepository) UpdatePluginThemeStatus(ctx context.Context, id string, isActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.pluginThemes[id]
	if !exists {
		return ErrWordPressPluginThemeNotFound
	}
	existing.IsActive = isActive
	return nil
}

func (m *MemoryWordPressRepository) DeletePluginTheme(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pluginThemes[id]; !exists {
		return ErrWordPressPluginThemeNotFound
	}
	delete(m.pluginThemes, id)
	return nil
}

func (m *MemoryWordPressRepository) RecordWebmasterAction(ctx context.Context, act *WordPressWebmasterAction) error {
	if act == nil {
		return errors.New("webmaster action cannot be nil")
	}
	if strings.TrimSpace(act.DeploymentID) == "" {
		return ErrEmptyWordPressDeploymentID
	}
	normActionType, err := NormalizeWebmasterActionType(act.ActionType)
	if err != nil {
		return err
	}
	act.ActionType = normActionType

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(act.ID) == "" {
		act.ID = GenerateUUID()
	}
	if act.AppliedAt.IsZero() {
		act.AppliedAt = time.Now().UTC()
	}
	if len(act.Payload) == 0 {
		act.Payload = json.RawMessage("{}")
	}

	cp := *act
	m.actions[act.ID] = &cp
	return nil
}

func (m *MemoryWordPressRepository) GetWebmasterActionByID(ctx context.Context, id string) (*WordPressWebmasterAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	act, exists := m.actions[id]
	if !exists {
		return nil, ErrWordPressWebmasterActionNotFound
	}
	cp := *act
	return &cp, nil
}

func (m *MemoryWordPressRepository) ListWebmasterActionsByDeploymentID(ctx context.Context, deploymentID string) ([]*WordPressWebmasterAction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*WordPressWebmasterAction
	for _, act := range m.actions {
		if act.DeploymentID == deploymentID {
			cp := *act
			list = append(list, &cp)
		}
	}
	return list, nil
}
