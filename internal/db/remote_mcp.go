package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrRemoteMCPNotFound     = errors.New("remote mcp connector not found")
	ErrRemoteMCPInvalidInput = errors.New("remote mcp connector id and base_url are required")
)

// RemoteMCPConnector represents a persisted external MCP server configuration.
type RemoteMCPConnector struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	BaseURL         string    `json:"base_url"`
	Transport       string    `json:"transport"`
	APIKeyEncrypted string    `json:"-"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RemoteMCPConnectorRepository defines persistence operations for Remote MCP connectors.
type RemoteMCPConnectorRepository interface {
	UpsertRemoteMCPConnector(ctx context.Context, c *RemoteMCPConnector) error
	ListRemoteMCPConnectors(ctx context.Context) ([]*RemoteMCPConnector, error)
	GetRemoteMCPConnector(ctx context.Context, id string) (*RemoteMCPConnector, error)
	DeleteRemoteMCPConnector(ctx context.Context, id string) error
}

// SQLRemoteMCPConnectorRepository implements RemoteMCPConnectorRepository using PostgreSQL.
type SQLRemoteMCPConnectorRepository struct {
	db *sql.DB
}

// NewSQLRemoteMCPConnectorRepository constructs a new SQLRemoteMCPConnectorRepository.
func NewSQLRemoteMCPConnectorRepository(db *sql.DB) *SQLRemoteMCPConnectorRepository {
	return &SQLRemoteMCPConnectorRepository{db: db}
}

func (r *SQLRemoteMCPConnectorRepository) UpsertRemoteMCPConnector(ctx context.Context, c *RemoteMCPConnector) error {
	if c == nil || strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.BaseURL) == "" {
		return ErrRemoteMCPInvalidInput
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if strings.TrimSpace(c.Transport) == "" {
		c.Transport = "http"
	}

	query := `
		INSERT INTO remote_mcp_connectors (
			id, name, description, base_url, transport, api_key_encrypted, enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			base_url = EXCLUDED.base_url,
			transport = EXCLUDED.transport,
			api_key_encrypted = EXCLUDED.api_key_encrypted,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		strings.TrimSpace(c.ID),
		strings.TrimSpace(c.Name),
		strings.TrimSpace(c.Description),
		strings.TrimSpace(c.BaseURL),
		strings.TrimSpace(c.Transport),
		c.APIKeyEncrypted,
		c.Enabled,
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert remote mcp connector: %w", err)
	}
	return nil
}

func (r *SQLRemoteMCPConnectorRepository) ListRemoteMCPConnectors(ctx context.Context) ([]*RemoteMCPConnector, error) {
	query := `
		SELECT id, name, description, base_url, transport, api_key_encrypted, enabled, created_at, updated_at
		FROM remote_mcp_connectors
		ORDER BY created_at ASC, id ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list remote mcp connectors: %w", err)
	}
	defer rows.Close()

	var list []*RemoteMCPConnector
	for rows.Next() {
		var c RemoteMCPConnector
		if err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.Description,
			&c.BaseURL,
			&c.Transport,
			&c.APIKeyEncrypted,
			&c.Enabled,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan remote mcp connector: %w", err)
		}
		list = append(list, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate remote mcp connectors: %w", err)
	}
	return list, nil
}

func (r *SQLRemoteMCPConnectorRepository) GetRemoteMCPConnector(ctx context.Context, id string) (*RemoteMCPConnector, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrRemoteMCPInvalidInput
	}
	query := `
		SELECT id, name, description, base_url, transport, api_key_encrypted, enabled, created_at, updated_at
		FROM remote_mcp_connectors
		WHERE id = $1
	`
	var c RemoteMCPConnector
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID,
		&c.Name,
		&c.Description,
		&c.BaseURL,
		&c.Transport,
		&c.APIKeyEncrypted,
		&c.Enabled,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRemoteMCPNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get remote mcp connector: %w", err)
	}
	return &c, nil
}

func (r *SQLRemoteMCPConnectorRepository) DeleteRemoteMCPConnector(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrRemoteMCPInvalidInput
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM remote_mcp_connectors WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete remote mcp connector: %w", err)
	}
	return nil
}

// MemoryRemoteMCPConnectorRepository implements RemoteMCPConnectorRepository in-memory for testing.
type MemoryRemoteMCPConnectorRepository struct {
	mu    sync.RWMutex
	items map[string]*RemoteMCPConnector
}

// NewMemoryRemoteMCPConnectorRepository constructs a new in-memory repository.
func NewMemoryRemoteMCPConnectorRepository() *MemoryRemoteMCPConnectorRepository {
	return &MemoryRemoteMCPConnectorRepository{
		items: make(map[string]*RemoteMCPConnector),
	}
}

func (r *MemoryRemoteMCPConnectorRepository) UpsertRemoteMCPConnector(_ context.Context, c *RemoteMCPConnector) error {
	if c == nil || strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.BaseURL) == "" {
		return ErrRemoteMCPInvalidInput
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	copyItem := *c
	copyItem.ID = strings.TrimSpace(copyItem.ID)
	copyItem.Name = strings.TrimSpace(copyItem.Name)
	copyItem.Description = strings.TrimSpace(copyItem.Description)
	copyItem.BaseURL = strings.TrimSpace(copyItem.BaseURL)
	copyItem.Transport = strings.TrimSpace(copyItem.Transport)
	if copyItem.Transport == "" {
		copyItem.Transport = "http"
	}
	if existing, ok := r.items[copyItem.ID]; ok {
		copyItem.CreatedAt = existing.CreatedAt
	} else if copyItem.CreatedAt.IsZero() {
		copyItem.CreatedAt = now
	}
	copyItem.UpdatedAt = now
	r.items[copyItem.ID] = &copyItem
	return nil
}

func (r *MemoryRemoteMCPConnectorRepository) ListRemoteMCPConnectors(_ context.Context) ([]*RemoteMCPConnector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*RemoteMCPConnector, 0, len(r.items))
	for _, item := range r.items {
		cp := *item
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].ID < list[j].ID
	})
	return list, nil
}

func (r *MemoryRemoteMCPConnectorRepository) GetRemoteMCPConnector(_ context.Context, id string) (*RemoteMCPConnector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[strings.TrimSpace(id)]
	if !ok {
		return nil, ErrRemoteMCPNotFound
	}
	cp := *item
	return &cp, nil
}

func (r *MemoryRemoteMCPConnectorRepository) DeleteRemoteMCPConnector(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.items, strings.TrimSpace(id))
	return nil
}
