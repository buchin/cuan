package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrServerNotFound  = errors.New("server not found")
	ErrDuplicateServer = errors.New("server with this id already exists")
)

// Server represents a remote server / VPS managed via BYOS (Bring Your Own Server).
type Server struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Host           string                 `json:"host"`
	Port           int                    `json:"port"`
	Username       string                 `json:"username"`
	AuthType       string                 `json:"auth_type"`
	EncryptedKey   string                 `json:"-"` // Explicitly ignored by JSON serialization (zero credential leakage)
	KeyFingerprint string                 `json:"key_fingerprint,omitempty"`
	Status         string                 `json:"status"` // 'pending', 'online', 'unreachable', 'error'
	LastPingAt     *time.Time             `json:"last_ping_at,omitempty"`
	OSInfo         map[string]interface{} `json:"os_info"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ServerDTO is a sanitized public Data Transfer Object for Server API responses.
type ServerDTO struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Host           string                 `json:"host"`
	Port           int                    `json:"port"`
	Username       string                 `json:"username"`
	AuthType       string                 `json:"auth_type"`
	KeyFingerprint string                 `json:"key_fingerprint,omitempty"`
	Status         string                 `json:"status"`
	LastPingAt     *time.Time             `json:"last_ping_at,omitempty"`
	OSInfo         map[string]interface{} `json:"os_info"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// ToDTO maps the internal Server entity to a safe public DTO.
func (s *Server) ToDTO() ServerDTO {
	osCopy := make(map[string]interface{})
	for k, v := range s.OSInfo {
		osCopy[k] = v
	}
	return ServerDTO{
		ID:             s.ID,
		Name:           s.Name,
		Host:           s.Host,
		Port:           s.Port,
		Username:       s.Username,
		AuthType:       s.AuthType,
		KeyFingerprint: s.KeyFingerprint,
		Status:         s.Status,
		LastPingAt:     s.LastPingAt,
		OSInfo:         osCopy,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// ServerRepository defines the persistence contract for BYOS server entities.
type ServerRepository interface {
	Create(ctx context.Context, server *Server) error
	GetByID(ctx context.Context, id string) (*Server, error)
	List(ctx context.Context) ([]Server, error)
	Update(ctx context.Context, server *Server) error
	UpdateStatus(ctx context.Context, id, status string, lastPingAt *time.Time, osInfo map[string]interface{}) error
	Delete(ctx context.Context, id string) error
}

// SQLServerRepository implements ServerRepository using PostgreSQL.
type SQLServerRepository struct {
	db *sql.DB
}

// NewSQLServerRepository creates a new SQLServerRepository backed by a SQL database.
func NewSQLServerRepository(db *sql.DB) *SQLServerRepository {
	return &SQLServerRepository{db: db}
}

func (r *SQLServerRepository) Create(ctx context.Context, server *Server) error {
	now := time.Now().UTC()
	if server.CreatedAt.IsZero() {
		server.CreatedAt = now
	}
	server.UpdatedAt = now

	if server.Port == 0 {
		server.Port = 22
	}
	if server.Username == "" {
		server.Username = "ubuntu"
	}
	if server.AuthType == "" {
		server.AuthType = "key"
	}
	if server.Status == "" {
		server.Status = "pending"
	}

	osJSON, err := json.Marshal(server.OSInfo)
	if err != nil || server.OSInfo == nil {
		osJSON = []byte("{}")
	}

	query := `
		INSERT INTO servers (
			id, name, host, port, username, auth_type, encrypted_key,
			key_fingerprint, status, last_ping_at, os_info, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err = r.db.ExecContext(ctx, query,
		server.ID,
		server.Name,
		server.Host,
		server.Port,
		server.Username,
		server.AuthType,
		server.EncryptedKey,
		server.KeyFingerprint,
		server.Status,
		server.LastPingAt,
		osJSON,
		server.CreatedAt,
		server.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return nil
}

func (r *SQLServerRepository) GetByID(ctx context.Context, id string) (*Server, error) {
	query := `
		SELECT id, name, host, port, username, auth_type, encrypted_key,
		       COALESCE(key_fingerprint, ''), status, last_ping_at, os_info, created_at, updated_at
		FROM servers
		WHERE id = $1
	`
	var s Server
	var osRaw []byte
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&s.ID,
		&s.Name,
		&s.Host,
		&s.Port,
		&s.Username,
		&s.AuthType,
		&s.EncryptedKey,
		&s.KeyFingerprint,
		&s.Status,
		&s.LastPingAt,
		&osRaw,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrServerNotFound
		}
		return nil, fmt.Errorf("get server by id: %w", err)
	}

	if len(osRaw) > 0 {
		_ = json.Unmarshal(osRaw, &s.OSInfo)
	}
	if s.OSInfo == nil {
		s.OSInfo = make(map[string]interface{})
	}

	return &s, nil
}

func (r *SQLServerRepository) List(ctx context.Context) ([]Server, error) {
	query := `
		SELECT id, name, host, port, username, auth_type, encrypted_key,
		       COALESCE(key_fingerprint, ''), status, last_ping_at, os_info, created_at, updated_at
		FROM servers
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer rows.Close()

	var list []Server
	for rows.Next() {
		var s Server
		var osRaw []byte
		if err := rows.Scan(
			&s.ID,
			&s.Name,
			&s.Host,
			&s.Port,
			&s.Username,
			&s.AuthType,
			&s.EncryptedKey,
			&s.KeyFingerprint,
			&s.Status,
			&s.LastPingAt,
			&osRaw,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan server row: %w", err)
		}

		if len(osRaw) > 0 {
			_ = json.Unmarshal(osRaw, &s.OSInfo)
		}
		if s.OSInfo == nil {
			s.OSInfo = make(map[string]interface{})
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}

	return list, nil
}

func (r *SQLServerRepository) Update(ctx context.Context, server *Server) error {
	server.UpdatedAt = time.Now().UTC()
	osJSON, err := json.Marshal(server.OSInfo)
	if err != nil || server.OSInfo == nil {
		osJSON = []byte("{}")
	}

	query := `
		UPDATE servers
		SET name = $2, host = $3, port = $4, username = $5, auth_type = $6,
		    encrypted_key = $7, key_fingerprint = $8, status = $9,
		    last_ping_at = $10, os_info = $11, updated_at = $12
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		server.ID,
		server.Name,
		server.Host,
		server.Port,
		server.Username,
		server.AuthType,
		server.EncryptedKey,
		server.KeyFingerprint,
		server.Status,
		server.LastPingAt,
		osJSON,
		server.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update server rows affected: %w", err)
	}
	if rows == 0 {
		return ErrServerNotFound
	}
	return nil
}

func (r *SQLServerRepository) UpdateStatus(ctx context.Context, id, status string, lastPingAt *time.Time, osInfo map[string]interface{}) error {
	now := time.Now().UTC()
	osJSON, err := json.Marshal(osInfo)
	if err != nil || osInfo == nil {
		osJSON = []byte("{}")
	}

	query := `
		UPDATE servers
		SET status = $2, last_ping_at = $3, os_info = $4, updated_at = $5
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id, status, lastPingAt, osJSON, now)
	if err != nil {
		return fmt.Errorf("update server status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update server status rows: %w", err)
	}
	if rows == 0 {
		return ErrServerNotFound
	}
	return nil
}

func (r *SQLServerRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM servers WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete server rows: %w", err)
	}
	if rows == 0 {
		return ErrServerNotFound
	}
	return nil
}

// MemoryServerRepository provides a thread-safe in-memory implementation of ServerRepository
// for unit testing and mock environments.
type MemoryServerRepository struct {
	mu      sync.RWMutex
	servers map[string]*Server
}

// NewMemoryServerRepository creates an in-memory server repository.
func NewMemoryServerRepository() *MemoryServerRepository {
	return &MemoryServerRepository{
		servers: make(map[string]*Server),
	}
}

func (m *MemoryServerRepository) Create(ctx context.Context, server *Server) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[server.ID]; exists {
		return ErrDuplicateServer
	}

	now := time.Now().UTC()
	if server.CreatedAt.IsZero() {
		server.CreatedAt = now
	}
	server.UpdatedAt = now

	copied := *server
	m.servers[server.ID] = &copied
	return nil
}

func (m *MemoryServerRepository) GetByID(ctx context.Context, id string) (*Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	srv, exists := m.servers[id]
	if !exists {
		return nil, ErrServerNotFound
	}
	copied := *srv
	return &copied, nil
}

func (m *MemoryServerRepository) List(ctx context.Context) ([]Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Server, 0, len(m.servers))
	for _, srv := range m.servers {
		list = append(list, *srv)
	}
	return list, nil
}

func (m *MemoryServerRepository) Update(ctx context.Context, server *Server) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[server.ID]; !exists {
		return ErrServerNotFound
	}
	server.UpdatedAt = time.Now().UTC()
	copied := *server
	m.servers[server.ID] = &copied
	return nil
}

func (m *MemoryServerRepository) UpdateStatus(ctx context.Context, id, status string, lastPingAt *time.Time, osInfo map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srv, exists := m.servers[id]
	if !exists {
		return ErrServerNotFound
	}
	srv.Status = status
	srv.LastPingAt = lastPingAt
	if osInfo != nil {
		srv.OSInfo = osInfo
	}
	srv.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryServerRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[id]; !exists {
		return ErrServerNotFound
	}
	delete(m.servers, id)
	return nil
}
