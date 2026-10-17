package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrTaskProjectInvalidInput = errors.New("task_id and project_id are required")
)

// TaskProject represents a many-to-many relationship linking tasks and projects.
type TaskProject struct {
	TaskID    string    `json:"task_id"`
	ProjectID string    `json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskProjectRepository defines data operations for multi-project task linking.
type TaskProjectRepository interface {
	LinkTaskProject(ctx context.Context, taskID, projectID string) error
	UnlinkTaskProject(ctx context.Context, taskID, projectID string) error
	GetProjectsByTask(ctx context.Context, taskID string) ([]string, error)
	GetTasksByProject(ctx context.Context, projectID string) ([]string, error)
	SetTaskProjects(ctx context.Context, taskID string, projectIDs []string) error
	CountTasksByProject(ctx context.Context, projectID string) (int, error)
}

// SQLTaskProjectRepository implements TaskProjectRepository using PostgreSQL.
type SQLTaskProjectRepository struct {
	db *sql.DB
}

// NewSQLTaskProjectRepository constructs a new SQLTaskProjectRepository.
func NewSQLTaskProjectRepository(db *sql.DB) *SQLTaskProjectRepository {
	return &SQLTaskProjectRepository{db: db}
}

func (r *SQLTaskProjectRepository) LinkTaskProject(ctx context.Context, taskID, projectID string) error {
	taskID = strings.TrimSpace(taskID)
	projectID = strings.TrimSpace(projectID)
	if taskID == "" || projectID == "" {
		return ErrTaskProjectInvalidInput
	}

	query := `
		INSERT INTO task_projects (task_id, project_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (task_id, project_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, taskID, projectID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("link task to project: %w", err)
	}
	return nil
}

func (r *SQLTaskProjectRepository) UnlinkTaskProject(ctx context.Context, taskID, projectID string) error {
	taskID = strings.TrimSpace(taskID)
	projectID = strings.TrimSpace(projectID)
	if taskID == "" || projectID == "" {
		return ErrTaskProjectInvalidInput
	}

	query := `DELETE FROM task_projects WHERE task_id = $1 AND project_id = $2`
	_, err := r.db.ExecContext(ctx, query, taskID, projectID)
	if err != nil {
		return fmt.Errorf("unlink task from project: %w", err)
	}
	return nil
}

func (r *SQLTaskProjectRepository) GetProjectsByTask(ctx context.Context, taskID string) ([]string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrTaskProjectInvalidInput
	}

	query := `SELECT project_id FROM task_projects WHERE task_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("get projects by task: %w", err)
	}
	defer rows.Close()

	var projectIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, fmt.Errorf("scan project_id: %w", err)
		}
		projectIDs = append(projectIDs, pid)
	}
	return projectIDs, nil
}

func (r *SQLTaskProjectRepository) GetTasksByProject(ctx context.Context, projectID string) ([]string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrTaskProjectInvalidInput
	}

	query := `SELECT task_id FROM task_projects WHERE project_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("get tasks by project: %w", err)
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			return nil, fmt.Errorf("scan task_id: %w", err)
		}
		taskIDs = append(taskIDs, tid)
	}
	return taskIDs, nil
}

func (r *SQLTaskProjectRepository) SetTaskProjects(ctx context.Context, taskID string, projectIDs []string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrTaskProjectInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM task_projects WHERE task_id = $1`, taskID); err != nil {
		return fmt.Errorf("delete old task_projects: %w", err)
	}

	now := time.Now().UTC()
	for _, pid := range projectIDs {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO task_projects (task_id, project_id, created_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (task_id, project_id) DO NOTHING
		`, taskID, pid, now)
		if err != nil {
			return fmt.Errorf("insert task_project (%s, %s): %w", taskID, pid, err)
		}
	}

	return tx.Commit()
}

func (r *SQLTaskProjectRepository) CountTasksByProject(ctx context.Context, projectID string) (int, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return 0, ErrTaskProjectInvalidInput
	}

	var count int
	query := `SELECT COUNT(*) FROM task_projects WHERE project_id = $1`
	err := r.db.QueryRowContext(ctx, query, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tasks by project: %w", err)
	}
	return count, nil
}

// MemoryTaskProjectRepository implements TaskProjectRepository in-memory for tests.
type MemoryTaskProjectRepository struct {
	mu           sync.RWMutex
	taskProjects map[string]map[string]time.Time // taskID -> set of projectID -> createdAt
}

// NewMemoryTaskProjectRepository constructs a thread-safe MemoryTaskProjectRepository.
func NewMemoryTaskProjectRepository() *MemoryTaskProjectRepository {
	return &MemoryTaskProjectRepository{
		taskProjects: make(map[string]map[string]time.Time),
	}
}

func (m *MemoryTaskProjectRepository) LinkTaskProject(ctx context.Context, taskID, projectID string) error {
	taskID = strings.TrimSpace(taskID)
	projectID = strings.TrimSpace(projectID)
	if taskID == "" || projectID == "" {
		return ErrTaskProjectInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.taskProjects[taskID] == nil {
		m.taskProjects[taskID] = make(map[string]time.Time)
	}
	if _, exists := m.taskProjects[taskID][projectID]; !exists {
		m.taskProjects[taskID][projectID] = time.Now().UTC()
	}
	return nil
}

func (m *MemoryTaskProjectRepository) UnlinkTaskProject(ctx context.Context, taskID, projectID string) error {
	taskID = strings.TrimSpace(taskID)
	projectID = strings.TrimSpace(projectID)
	if taskID == "" || projectID == "" {
		return ErrTaskProjectInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.taskProjects[taskID] != nil {
		delete(m.taskProjects[taskID], projectID)
	}
	return nil
}

func (m *MemoryTaskProjectRepository) GetProjectsByTask(ctx context.Context, taskID string) ([]string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrTaskProjectInvalidInput
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []string
	if projects, ok := m.taskProjects[taskID]; ok {
		for pid := range projects {
			result = append(result, pid)
		}
	}
	return result, nil
}

func (m *MemoryTaskProjectRepository) GetTasksByProject(ctx context.Context, projectID string) ([]string, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrTaskProjectInvalidInput
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []string
	for tid, projects := range m.taskProjects {
		if _, exists := projects[projectID]; exists {
			result = append(result, tid)
		}
	}
	return result, nil
}

func (m *MemoryTaskProjectRepository) SetTaskProjects(ctx context.Context, taskID string, projectIDs []string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrTaskProjectInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.taskProjects[taskID] = make(map[string]time.Time)
	now := time.Now().UTC()
	for _, pid := range projectIDs {
		pid = strings.TrimSpace(pid)
		if pid != "" {
			m.taskProjects[taskID][pid] = now
		}
	}
	return nil
}

func (m *MemoryTaskProjectRepository) CountTasksByProject(ctx context.Context, projectID string) (int, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return 0, ErrTaskProjectInvalidInput
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, projects := range m.taskProjects {
		if _, exists := projects[projectID]; exists {
			count++
		}
	}
	return count, nil
}
