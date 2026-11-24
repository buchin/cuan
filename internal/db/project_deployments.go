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
)

// Constants for ProjectDeployment status
const (
	DeploymentStatusQueued   = "queued"
	DeploymentStatusBuilding = "building"
	DeploymentStatusSuccess  = "success"
	DeploymentStatusFailed   = "failed"
	DeploymentStatusCanceled = "canceled"
)

// Constants for Step status
const (
	StepStatusPending  = "pending"
	StepStatusBuilding = "building"
	StepStatusSuccess  = "success"
	StepStatusFailed   = "failed"
	StepStatusSkipped  = "skipped"
)

// Constants for trigger source
const (
	TriggerSourceManual     = "manual"
	TriggerSourceAutoDeploy = "auto_deploy"
	TriggerSourceGitPush    = "git_push"
	TriggerSourceCLI        = "cli"
	TriggerSourceWebhook    = "webhook"
)

var (
	ErrProjectDeploymentNotFound         = errors.New("project deployment not found")
	ErrEmptyProjectDeploymentProjectID   = errors.New("project deployment project id cannot be empty")
	ErrEmptyCommitSHA                    = errors.New("commit sha cannot be empty")
)

// ProjectDeploymentStep represents a granular phase inside a deployment run.
type ProjectDeploymentStep struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"` // 'success', 'building', 'failed', 'pending', 'skipped'
	Duration    string     `json:"duration"` // e.g. "4s", "28s"
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Summary     string     `json:"summary"`
	Log         string     `json:"log"`
}

// ProjectDeployment represents a recorded build and release iteration for a project.
type ProjectDeployment struct {
	ID              string     `json:"id"`
	ProjectID       string     `json:"project_id"`
	CommitSHA       string     `json:"commit_sha"`
	ShortSHA        string     `json:"short_sha"`
	CommitMessage   string     `json:"commit_message"`
	Branch          string     `json:"branch"`
	Status          string     `json:"status"` // 'building', 'success', 'failed', 'canceled'
	TriggerSource   string     `json:"trigger_source"` // 'manual', 'auto_deploy', 'git_push', 'cli'
	TriggerAuthor   string     `json:"trigger_author"`
	DurationSeconds int        `json:"duration_seconds"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	StepsJSON       string     `json:"-"` // Stored in DB
	Steps           []ProjectDeploymentStep `json:"steps"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// FormatDuration returns a clean human-readable duration like "42s" or "1m 15s".
func (d *ProjectDeployment) FormatDuration() string {
	sec := d.DurationSeconds
	if sec <= 0 && !d.StartedAt.IsZero() {
		if d.CompletedAt != nil {
			sec = int(d.CompletedAt.Sub(d.StartedAt).Seconds())
		} else if d.Status == DeploymentStatusBuilding {
			sec = int(time.Since(d.StartedAt).Seconds())
		}
	}
	if sec < 0 {
		sec = 0
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	mins := sec / 60
	remSec := sec % 60
	if remSec == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm %ds", mins, remSec)
}

// ProjectDeploymentRepository defines data layer operations for project deployments.
type ProjectDeploymentRepository interface {
	Create(ctx context.Context, dep *ProjectDeployment) error
	GetByID(ctx context.Context, id string) (*ProjectDeployment, error)
	ListByProjectID(ctx context.Context, projectID string, limit int) ([]*ProjectDeployment, error)
	GetLatestSuccess(ctx context.Context, projectID string) (*ProjectDeployment, error)
	GetActiveBuilding(ctx context.Context, projectID string) (*ProjectDeployment, error)
	GetNextQueued(ctx context.Context, projectID string) (*ProjectDeployment, error)
	CancelQueuedForProject(ctx context.Context, projectID string, exceptID string) error
	Update(ctx context.Context, dep *ProjectDeployment) error
	Delete(ctx context.Context, id string) error
}

// -----------------------------------------------------------------------------
// PostgreSQL / SQLite SQL Implementation
// -----------------------------------------------------------------------------

type SQLProjectDeploymentRepository struct {
	db *sql.DB
}

func NewSQLProjectDeploymentRepository(db *sql.DB) *SQLProjectDeploymentRepository {
	return &SQLProjectDeploymentRepository{db: db}
}

func (r *SQLProjectDeploymentRepository) Create(ctx context.Context, dep *ProjectDeployment) error {
	if dep == nil {
		return errors.New("deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ProjectID) == "" {
		return ErrEmptyProjectDeploymentProjectID
	}
	if strings.TrimSpace(dep.CommitSHA) == "" {
		return ErrEmptyCommitSHA
	}
	if strings.TrimSpace(dep.ID) == "" {
		dep.ID = GenerateUUID()
	}
	if dep.ShortSHA == "" {
		if len(dep.CommitSHA) >= 7 {
			dep.ShortSHA = dep.CommitSHA[:7]
		} else {
			dep.ShortSHA = dep.CommitSHA
		}
	}
	if dep.Status == "" {
		dep.Status = DeploymentStatusBuilding
	}
	if dep.Branch == "" {
		dep.Branch = "main"
	}
	if dep.TriggerSource == "" {
		dep.TriggerSource = TriggerSourceManual
	}
	now := time.Now().UTC()
	if dep.StartedAt.IsZero() {
		dep.StartedAt = now
	}
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now

	if len(dep.Steps) > 0 && (dep.StepsJSON == "" || dep.StepsJSON == "[]") {
		b, err := json.Marshal(dep.Steps)
		if err == nil {
			dep.StepsJSON = string(b)
		}
	}
	if dep.StepsJSON == "" {
		dep.StepsJSON = "[]"
	}

	query := `
		INSERT INTO project_deployments (
			id, project_id, commit_sha, short_sha, commit_message,
			branch, status, trigger_source, trigger_author,
			duration_seconds, started_at, completed_at,
			steps_json, error_message, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := r.db.ExecContext(ctx, query,
		dep.ID,
		dep.ProjectID,
		dep.CommitSHA,
		dep.ShortSHA,
		dep.CommitMessage,
		dep.Branch,
		dep.Status,
		dep.TriggerSource,
		dep.TriggerAuthor,
		dep.DurationSeconds,
		dep.StartedAt,
		dep.CompletedAt,
		dep.StepsJSON,
		dep.ErrorMessage,
		dep.CreatedAt,
		dep.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create project deployment: %w", err)
	}
	return nil
}

func (r *SQLProjectDeploymentRepository) scanOne(row *sql.Row) (*ProjectDeployment, error) {
	dep := &ProjectDeployment{}
	var stepsRaw string
	var completedAt sql.NullTime

	err := row.Scan(
		&dep.ID,
		&dep.ProjectID,
		&dep.CommitSHA,
		&dep.ShortSHA,
		&dep.CommitMessage,
		&dep.Branch,
		&dep.Status,
		&dep.TriggerSource,
		&dep.TriggerAuthor,
		&dep.DurationSeconds,
		&dep.StartedAt,
		&completedAt,
		&stepsRaw,
		&dep.ErrorMessage,
		&dep.CreatedAt,
		&dep.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrProjectDeploymentNotFound
		}
		return nil, fmt.Errorf("scan project deployment: %w", err)
	}
	if completedAt.Valid {
		dep.CompletedAt = &completedAt.Time
	}
	dep.StepsJSON = stepsRaw
	if strings.TrimSpace(stepsRaw) != "" && stepsRaw != "[]" {
		_ = json.Unmarshal([]byte(stepsRaw), &dep.Steps)
	}
	if dep.Steps == nil {
		dep.Steps = []ProjectDeploymentStep{}
	}
	return dep, nil
}

func (r *SQLProjectDeploymentRepository) GetByID(ctx context.Context, id string) (*ProjectDeployment, error) {
	query := `
		SELECT id, project_id, commit_sha, short_sha, commit_message,
		       branch, status, trigger_source, trigger_author,
		       duration_seconds, started_at, completed_at,
		       steps_json, error_message, created_at, updated_at
		FROM project_deployments
		WHERE id = $1
	`
	return r.scanOne(r.db.QueryRowContext(ctx, query, id))
}

func (r *SQLProjectDeploymentRepository) ListByProjectID(ctx context.Context, projectID string, limit int) ([]*ProjectDeployment, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT id, project_id, commit_sha, short_sha, commit_message,
		       branch, status, trigger_source, trigger_author,
		       duration_seconds, started_at, completed_at,
		       steps_json, error_message, created_at, updated_at
		FROM project_deployments
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list project deployments: %w", err)
	}
	defer rows.Close()

	var list []*ProjectDeployment
	for rows.Next() {
		dep := &ProjectDeployment{}
		var stepsRaw string
		var completedAt sql.NullTime

		err := rows.Scan(
			&dep.ID,
			&dep.ProjectID,
			&dep.CommitSHA,
			&dep.ShortSHA,
			&dep.CommitMessage,
			&dep.Branch,
			&dep.Status,
			&dep.TriggerSource,
			&dep.TriggerAuthor,
			&dep.DurationSeconds,
			&dep.StartedAt,
			&completedAt,
			&stepsRaw,
			&dep.ErrorMessage,
			&dep.CreatedAt,
			&dep.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan deployment row: %w", err)
		}
		if completedAt.Valid {
			dep.CompletedAt = &completedAt.Time
		}
		dep.StepsJSON = stepsRaw
		if strings.TrimSpace(stepsRaw) != "" && stepsRaw != "[]" {
			_ = json.Unmarshal([]byte(stepsRaw), &dep.Steps)
		}
		if dep.Steps == nil {
			dep.Steps = []ProjectDeploymentStep{}
		}
		list = append(list, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *SQLProjectDeploymentRepository) GetLatestSuccess(ctx context.Context, projectID string) (*ProjectDeployment, error) {
	query := `
		SELECT id, project_id, commit_sha, short_sha, commit_message,
		       branch, status, trigger_source, trigger_author,
		       duration_seconds, started_at, completed_at,
		       steps_json, error_message, created_at, updated_at
		FROM project_deployments
		WHERE project_id = $1 AND status = 'success'
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanOne(r.db.QueryRowContext(ctx, query, projectID))
}

func (r *SQLProjectDeploymentRepository) GetActiveBuilding(ctx context.Context, projectID string) (*ProjectDeployment, error) {
	query := `
		SELECT id, project_id, commit_sha, short_sha, commit_message,
		       branch, status, trigger_source, trigger_author,
		       duration_seconds, started_at, completed_at,
		       steps_json, error_message, created_at, updated_at
		FROM project_deployments
		WHERE project_id = $1 AND status = 'building'
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanOne(r.db.QueryRowContext(ctx, query, projectID))
}

func (r *SQLProjectDeploymentRepository) GetNextQueued(ctx context.Context, projectID string) (*ProjectDeployment, error) {
	query := `
		SELECT id, project_id, commit_sha, short_sha, commit_message,
		       branch, status, trigger_source, trigger_author,
		       duration_seconds, started_at, completed_at,
		       steps_json, error_message, created_at, updated_at
		FROM project_deployments
		WHERE project_id = $1 AND status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1
	`
	return r.scanOne(r.db.QueryRowContext(ctx, query, projectID))
}

func (r *SQLProjectDeploymentRepository) CancelQueuedForProject(ctx context.Context, projectID string, exceptID string) error {
	query := `
		UPDATE project_deployments
		SET status = 'canceled', error_message = 'Superseded by newer deployment', completed_at = NOW(), updated_at = NOW()
		WHERE project_id = $1 AND status = 'queued' AND id != $2
	`
	_, err := r.db.ExecContext(ctx, query, projectID, exceptID)
	return err
}

func (r *SQLProjectDeploymentRepository) Update(ctx context.Context, dep *ProjectDeployment) error {
	if dep == nil {
		return errors.New("deployment cannot be nil")
	}
	dep.UpdatedAt = time.Now().UTC()
	if len(dep.Steps) > 0 {
		b, err := json.Marshal(dep.Steps)
		if err == nil {
			dep.StepsJSON = string(b)
		}
	}
	if dep.StepsJSON == "" {
		dep.StepsJSON = "[]"
	}

	query := `
		UPDATE project_deployments
		SET commit_sha = $2, short_sha = $3, commit_message = $4,
		    branch = $5, status = $6, trigger_source = $7, trigger_author = $8,
		    duration_seconds = $9, started_at = $10, completed_at = $11,
		    steps_json = $12, error_message = $13, updated_at = $14
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		dep.ID,
		dep.CommitSHA,
		dep.ShortSHA,
		dep.CommitMessage,
		dep.Branch,
		dep.Status,
		dep.TriggerSource,
		dep.TriggerAuthor,
		dep.DurationSeconds,
		dep.StartedAt,
		dep.CompletedAt,
		dep.StepsJSON,
		dep.ErrorMessage,
		dep.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update project deployment: %w", err)
	}
	rowsAff, err := res.RowsAffected()
	if err == nil && rowsAff == 0 {
		return ErrProjectDeploymentNotFound
	}
	return nil
}

func (r *SQLProjectDeploymentRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM project_deployments WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project deployment: %w", err)
	}
	rowsAff, err := res.RowsAffected()
	if err == nil && rowsAff == 0 {
		return ErrProjectDeploymentNotFound
	}
	return nil
}

// -----------------------------------------------------------------------------
// In-Memory Implementation (For Testing and Environments without DB)
// -----------------------------------------------------------------------------

type MemoryProjectDeploymentRepository struct {
	mu          sync.RWMutex
	deployments map[string]*ProjectDeployment
}

func NewMemoryProjectDeploymentRepository() *MemoryProjectDeploymentRepository {
	return &MemoryProjectDeploymentRepository{
		deployments: make(map[string]*ProjectDeployment),
	}
}

func (m *MemoryProjectDeploymentRepository) cloneDeployment(dep *ProjectDeployment) *ProjectDeployment {
	cp := *dep
	if dep.CompletedAt != nil {
		t := *dep.CompletedAt
		cp.CompletedAt = &t
	}
	if len(dep.Steps) > 0 {
		cp.Steps = make([]ProjectDeploymentStep, len(dep.Steps))
		copy(cp.Steps, dep.Steps)
	} else {
		cp.Steps = []ProjectDeploymentStep{}
	}
	return &cp
}

func (m *MemoryProjectDeploymentRepository) Create(ctx context.Context, dep *ProjectDeployment) error {
	if dep == nil {
		return errors.New("deployment cannot be nil")
	}
	if strings.TrimSpace(dep.ProjectID) == "" {
		return ErrEmptyProjectDeploymentProjectID
	}
	if strings.TrimSpace(dep.CommitSHA) == "" {
		return ErrEmptyCommitSHA
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if dep.ID == "" {
		dep.ID = GenerateUUID()
	}
	if dep.ShortSHA == "" {
		if len(dep.CommitSHA) >= 7 {
			dep.ShortSHA = dep.CommitSHA[:7]
		} else {
			dep.ShortSHA = dep.CommitSHA
		}
	}
	if dep.Status == "" {
		dep.Status = DeploymentStatusBuilding
	}
	if dep.Branch == "" {
		dep.Branch = "main"
	}
	if dep.TriggerSource == "" {
		dep.TriggerSource = TriggerSourceManual
	}
	now := time.Now().UTC()
	if dep.StartedAt.IsZero() {
		dep.StartedAt = now
	}
	if dep.CreatedAt.IsZero() {
		dep.CreatedAt = now
	}
	dep.UpdatedAt = now
	if dep.Steps == nil {
		dep.Steps = []ProjectDeploymentStep{}
	}

	m.deployments[dep.ID] = m.cloneDeployment(dep)
	return nil
}

func (m *MemoryProjectDeploymentRepository) GetByID(ctx context.Context, id string) (*ProjectDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dep, ok := m.deployments[id]
	if !ok {
		return nil, ErrProjectDeploymentNotFound
	}
	return m.cloneDeployment(dep), nil
}

func (m *MemoryProjectDeploymentRepository) ListByProjectID(ctx context.Context, projectID string, limit int) ([]*ProjectDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var matched []*ProjectDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID {
			matched = append(matched, m.cloneDeployment(dep))
		}
	}

	// Sort created_at DESC
	for i := 0; i < len(matched)-1; i++ {
		for j := i + 1; j < len(matched); j++ {
			if matched[i].CreatedAt.Before(matched[j].CreatedAt) {
				matched[i], matched[j] = matched[j], matched[i]
			}
		}
	}

	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

func (m *MemoryProjectDeploymentRepository) GetLatestSuccess(ctx context.Context, projectID string) (*ProjectDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *ProjectDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID && dep.Status == DeploymentStatusSuccess {
			if latest == nil || dep.CreatedAt.After(latest.CreatedAt) {
				latest = dep
			}
		}
	}
	if latest == nil {
		return nil, ErrProjectDeploymentNotFound
	}
	return m.cloneDeployment(latest), nil
}

func (m *MemoryProjectDeploymentRepository) GetActiveBuilding(ctx context.Context, projectID string) (*ProjectDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, dep := range m.deployments {
		if dep.ProjectID == projectID && dep.Status == DeploymentStatusBuilding {
			return m.cloneDeployment(dep), nil
		}
	}
	return nil, ErrProjectDeploymentNotFound
}

func (m *MemoryProjectDeploymentRepository) GetNextQueued(ctx context.Context, projectID string) (*ProjectDeployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var oldest *ProjectDeployment
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID && dep.Status == DeploymentStatusQueued {
			if oldest == nil || dep.CreatedAt.Before(oldest.CreatedAt) {
				oldest = dep
			}
		}
	}
	if oldest == nil {
		return nil, ErrProjectDeploymentNotFound
	}
	return m.cloneDeployment(oldest), nil
}

func (m *MemoryProjectDeploymentRepository) CancelQueuedForProject(ctx context.Context, projectID string, exceptID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	for _, dep := range m.deployments {
		if dep.ProjectID == projectID && dep.Status == DeploymentStatusQueued && dep.ID != exceptID {
			dep.Status = DeploymentStatusCanceled
			dep.ErrorMessage = "Superseded by newer deployment"
			dep.CompletedAt = &now
			dep.UpdatedAt = now
		}
	}
	return nil
}

func (m *MemoryProjectDeploymentRepository) Update(ctx context.Context, dep *ProjectDeployment) error {
	if dep == nil {
		return errors.New("deployment cannot be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.deployments[dep.ID]
	if !ok {
		return ErrProjectDeploymentNotFound
	}
	dep.UpdatedAt = time.Now().UTC()
	m.deployments[dep.ID] = m.cloneDeployment(dep)
	return nil
}

func (m *MemoryProjectDeploymentRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.deployments[id]; !ok {
		return ErrProjectDeploymentNotFound
	}
	delete(m.deployments, id)
	return nil
}
