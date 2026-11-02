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
	ErrAgentNotFound           = errors.New("agent not found")
	ErrEmptyAgentName          = errors.New("agent name cannot be empty")
	ErrInvalidAgentStatus      = errors.New("invalid agent status: must be active, inactive, or archived")
	ErrDuplicateAgent          = errors.New("agent already exists")
	ErrAgentConnectionNotFound = errors.New("agent connection not found")
	ErrEmptyUserID             = errors.New("user id cannot be empty")
	ErrInvalidConnectionStatus = errors.New("invalid connection status: must be active, invalid, or disconnected")
	ErrDuplicateConnection     = errors.New("agent connection already exists")
)

const (
	AgentStatusActive    = "active"
	AgentStatusInactive  = "inactive"
	AgentStatusArchived  = "archived"
	AgentIDCuan          = "agent_cuan"
	DefaultAgentHandle   = "@cuan"
	DefaultAgentModelID  = "gemini-1.5-pro"
	DefaultAgentProvider = "google"
	DefaultAgentStatus   = "active"

	ConnectionStatusActive       = "active"
	ConnectionStatusInvalid      = "invalid"
	ConnectionStatusDisconnected = "disconnected"
	DefaultConnectionProvider    = "google"
	DefaultConnectionStatus      = "active"
)

// NormalizeAgentStatus validates and returns lowercase agent status.
func NormalizeAgentStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return AgentStatusActive, nil
	}
	switch norm {
	case AgentStatusActive, AgentStatusInactive, AgentStatusArchived:
		return norm, nil
	default:
		return "", ErrInvalidAgentStatus
	}
}

// NormalizeConnectionStatus validates and returns lowercase connection status.
func NormalizeConnectionStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return ConnectionStatusActive, nil
	}
	switch norm {
	case ConnectionStatusActive, ConnectionStatusInvalid, ConnectionStatusDisconnected:
		return norm, nil
	default:
		return "", ErrInvalidConnectionStatus
	}
}

// Agent represents an autonomous or interactive AI assistant persona in the system (standardized to Universal Agent format).
type Agent struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Harness         string    `json:"harness"`
	ConnectionID    string    `json:"connection_id"`
	Enabled         bool      `json:"enabled"`
	Avatar          string    `json:"avatar,omitempty"`
	Role            string    `json:"role,omitempty"`
	SystemPrompt    string    `json:"system_prompt,omitempty"`
	ModelID         string    `json:"model_id,omitempty"`
	FallbackModelID string    `json:"fallback_model_id,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	TokenBudget     *int      `json:"token_budget,omitempty"`
	Provider        string    `json:"provider,omitempty"`
	AuthType        string    `json:"auth_type,omitempty"`
	Status          string    `json:"status,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Clone returns a deep copy of the Agent.
func (a *Agent) Clone() *Agent {
	if a == nil {
		return nil
	}
	connID := a.ConnectionID
	modelID := a.ModelID
	if strings.HasPrefix(a.ModelID, "conn_") {
		if idx := strings.Index(a.ModelID, "#"); idx != -1 {
			if connID == "" {
				connID = a.ModelID[:idx]
			}
			modelID = a.ModelID[idx+1:]
		} else {
			if connID == "" {
				connID = a.ModelID
			}
		}
	}
	var temp *float64
	if a.Temperature != nil {
		tVal := *a.Temperature
		temp = &tVal
	}
	var budget *int
	if a.TokenBudget != nil {
		bVal := *a.TokenBudget
		budget = &bVal
	}
	return &Agent{
		ID:              a.ID,
		Name:            a.Name,
		Harness:         a.Harness,
		ConnectionID:    connID,
		Enabled:         a.Enabled,
		Avatar:          a.Avatar,
		Role:            a.Role,
		SystemPrompt:    a.SystemPrompt,
		ModelID:         modelID,
		FallbackModelID: a.FallbackModelID,
		Temperature:     temp,
		TokenBudget:     budget,
		Provider:        a.Provider,
		AuthType:        a.AuthType,
		Status:          a.Status,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

// AgentDTO represents the public Data Transfer Object for an Agent in Universal Agent format.
type AgentDTO struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Harness         string    `json:"harness"`
	ConnectionID    string    `json:"connection_id"`
	Enabled         bool      `json:"enabled"`
	Avatar          string    `json:"avatar,omitempty"`
	Role            string    `json:"role,omitempty"`
	SystemPrompt    string    `json:"system_prompt,omitempty"`
	ModelID         string    `json:"model_id,omitempty"`
	FallbackModelID string    `json:"fallback_model_id,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	TokenBudget     *int      `json:"token_budget,omitempty"`
	Provider        string    `json:"provider,omitempty"`
	AuthType        string    `json:"auth_type,omitempty"`
	Status          string    `json:"status,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ToDTO converts an Agent entity to its public AgentDTO representation.
func (a *Agent) ToDTO() AgentDTO {
	harness := strings.TrimSpace(a.Harness)
	if harness == "" {
		harness = strings.TrimSpace(a.Provider)
	}
	if harness == "" {
		harness = "agy"
	}
	connID := strings.TrimSpace(a.ConnectionID)
	modelID := a.ModelID
	if strings.HasPrefix(a.ModelID, "conn_") {
		if idx := strings.Index(a.ModelID, "#"); idx != -1 {
			if connID == "" {
				connID = a.ModelID[:idx]
			}
			modelID = a.ModelID[idx+1:]
		} else {
			if connID == "" {
				connID = a.ModelID
			}
		}
	}
	enabled := a.Enabled || a.Status == AgentStatusActive || a.Status == ""

	status := a.Status
	if status == "" {
		if enabled {
			status = AgentStatusActive
		} else {
			status = AgentStatusInactive
		}
	}

	var temp *float64
	if a.Temperature != nil {
		tVal := *a.Temperature
		temp = &tVal
	}
	var budget *int
	if a.TokenBudget != nil {
		bVal := *a.TokenBudget
		budget = &bVal
	}

	return AgentDTO{
		ID:              a.ID,
		Name:            a.Name,
		Harness:         harness,
		ConnectionID:    connID,
		Enabled:         enabled,
		Avatar:          a.Avatar,
		Role:            a.Role,
		SystemPrompt:    a.SystemPrompt,
		ModelID:         modelID,
		FallbackModelID: a.FallbackModelID,
		Temperature:     temp,
		TokenBudget:     budget,
		Provider:        a.Provider,
		AuthType:        a.AuthType,
		Status:          status,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

// CreateAgentInput holds parameters for registering a new agent.
type CreateAgentInput struct {
	ID              string   `json:"id,omitempty"`
	Name            string   `json:"name"`
	Harness         string   `json:"harness,omitempty"`
	ConnectionID    string   `json:"connection_id,omitempty"`
	Enabled         *bool    `json:"enabled,omitempty"`
	Avatar          string   `json:"avatar,omitempty"`
	Role            string   `json:"role,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	ModelID         string   `json:"model_id,omitempty"`
	FallbackModelID string   `json:"fallback_model_id,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TokenBudget     *int     `json:"token_budget,omitempty"`
	Provider        string   `json:"provider,omitempty"`
	Status          string   `json:"status,omitempty"`
	ApiKey          string   `json:"api_key,omitempty"`
	Endpoint        string   `json:"endpoint,omitempty"`
	HostSession     bool     `json:"host_session,omitempty"`
}

// Validate checks constraints on CreateAgentInput.
func (in *CreateAgentInput) Validate() error {
	if strings.TrimSpace(in.Name) == "" {
		return ErrEmptyAgentName
	}
	if in.Status != "" {
		if _, err := NormalizeAgentStatus(in.Status); err != nil {
			return err
		}
	}
	if in.Temperature != nil && (*in.Temperature < 0.0 || *in.Temperature > 2.0) {
		return errors.New("temperature must be between 0.0 and 2.0")
	}
	if in.TokenBudget != nil && *in.TokenBudget < 0 {
		return errors.New("token budget cannot be negative")
	}
	return nil
}

// ToEntity transforms CreateAgentInput into an Agent entity with sensible defaults.
func (in *CreateAgentInput) ToEntity() (*Agent, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	harness := strings.TrimSpace(in.Harness)
	provider := strings.TrimSpace(in.Provider)
	if harness != "" && provider == "" {
		provider = harness
	}
	if provider != "" && harness == "" {
		harness = provider
	}
	if provider == "" {
		provider = DefaultAgentProvider
	}
	if harness == "" {
		harness = "agy"
	}

	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	} else if in.Status != "" {
		norm, _ := NormalizeAgentStatus(in.Status)
		enabled = (norm == AgentStatusActive)
	}

	status := AgentStatusActive
	if !enabled {
		status = AgentStatusInactive
	}
	if in.Status != "" {
		norm, err := NormalizeAgentStatus(in.Status)
		if err == nil {
			status = norm
		}
	}

	connectionID := strings.TrimSpace(in.ConnectionID)
	modelID := strings.TrimSpace(in.ModelID)
	if modelID == "" {
		if connectionID != "" {
			modelID = connectionID
		} else {
			modelID = DefaultAgentModelID
		}
	}

	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = GenerateUUID()
	}

	var temp *float64
	if in.Temperature != nil {
		tVal := *in.Temperature
		temp = &tVal
	}
	var budget *int
	if in.TokenBudget != nil {
		bVal := *in.TokenBudget
		budget = &bVal
	}

	return &Agent{
		ID:              id,
		Name:            strings.TrimSpace(in.Name),
		Harness:         harness,
		ConnectionID:    connectionID,
		Enabled:         enabled,
		Avatar:          strings.TrimSpace(in.Avatar),
		Role:            strings.TrimSpace(in.Role),
		SystemPrompt:    strings.TrimSpace(in.SystemPrompt),
		ModelID:         modelID,
		FallbackModelID: strings.TrimSpace(in.FallbackModelID),
		Temperature:     temp,
		TokenBudget:     budget,
		Provider:        provider,
		Status:          status,
	}, nil
}

// UpdateAgentInput specifies mutable fields on an existing Agent.
type UpdateAgentInput struct {
	Name            *string  `json:"name,omitempty"`
	Harness         *string  `json:"harness,omitempty"`
	ConnectionID    *string  `json:"connection_id,omitempty"`
	Enabled         *bool    `json:"enabled,omitempty"`
	Avatar          *string  `json:"avatar,omitempty"`
	Role            *string  `json:"role,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	ModelID         *string  `json:"model_id,omitempty"`
	FallbackModelID *string  `json:"fallback_model_id,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TokenBudget     *int     `json:"token_budget,omitempty"`
	Provider        *string  `json:"provider,omitempty"`
	Status          *string  `json:"status,omitempty"`
}

// PatchAgentInput is an alias for UpdateAgentInput.
type PatchAgentInput = UpdateAgentInput

// Validate verifies non-nil fields in UpdateAgentInput.
func (in *UpdateAgentInput) Validate() error {
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return ErrEmptyAgentName
	}
	if in.Status != nil {
		if _, err := NormalizeAgentStatus(*in.Status); err != nil {
			return err
		}
	}
	if in.Temperature != nil && (*in.Temperature < 0.0 || *in.Temperature > 2.0) {
		return errors.New("temperature must be between 0.0 and 2.0")
	}
	if in.TokenBudget != nil && *in.TokenBudget < 0 {
		return errors.New("token budget cannot be negative")
	}
	return nil
}

// ApplyTo modifies the provided Agent in-place.
func (in *UpdateAgentInput) ApplyTo(agent *Agent) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if in.Name != nil {
		agent.Name = strings.TrimSpace(*in.Name)
	}
	if in.Harness != nil {
		agent.Harness = strings.TrimSpace(*in.Harness)
		agent.Provider = agent.Harness
	}
	if in.ConnectionID != nil {
		agent.ConnectionID = strings.TrimSpace(*in.ConnectionID)
		if strings.HasPrefix(agent.ConnectionID, "conn_") {
			agent.ModelID = agent.ConnectionID
		}
	}
	if in.Enabled != nil {
		agent.Enabled = *in.Enabled
		if agent.Enabled {
			agent.Status = AgentStatusActive
		} else {
			agent.Status = AgentStatusInactive
		}
	}
	if in.Avatar != nil {
		agent.Avatar = strings.TrimSpace(*in.Avatar)
	}
	if in.Role != nil {
		agent.Role = strings.TrimSpace(*in.Role)
	}
	if in.SystemPrompt != nil {
		agent.SystemPrompt = strings.TrimSpace(*in.SystemPrompt)
	}
	if in.ModelID != nil && strings.TrimSpace(*in.ModelID) != "" {
		agent.ModelID = strings.TrimSpace(*in.ModelID)
	}
	if in.FallbackModelID != nil {
		agent.FallbackModelID = strings.TrimSpace(*in.FallbackModelID)
	}
	if in.Temperature != nil {
		tVal := *in.Temperature
		agent.Temperature = &tVal
	}
	if in.TokenBudget != nil {
		bVal := *in.TokenBudget
		agent.TokenBudget = &bVal
	}
	if in.Provider != nil && strings.TrimSpace(*in.Provider) != "" {
		agent.Provider = strings.TrimSpace(*in.Provider)
		if agent.Harness == "" {
			agent.Harness = agent.Provider
		}
	}
	if in.Status != nil {
		norm, _ := NormalizeAgentStatus(*in.Status)
		agent.Status = norm
		agent.Enabled = (norm == AgentStatusActive)
	}
	return nil
}

// AgentFilter configures filtering and pagination for listing agents.
type AgentFilter struct {
	Provider string `json:"provider,omitempty"`
	Status   string `json:"status,omitempty"`
	Role     string `json:"role,omitempty"`
	Search   string `json:"search,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// AgentConnection stores third-party LLM provider credentials and configurations for a user.
type AgentConnection struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Provider             string    `json:"provider"`
	CredentialsEncrypted string    `json:"-"` // Omitted from public JSON to prevent credential leakage
	PlanHint             string    `json:"plan_hint"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Clone returns a deep copy of the AgentConnection.
func (c *AgentConnection) Clone() *AgentConnection {
	if c == nil {
		return nil
	}
	return &AgentConnection{
		ID:                   c.ID,
		UserID:               c.UserID,
		Provider:             c.Provider,
		CredentialsEncrypted: c.CredentialsEncrypted,
		PlanHint:             c.PlanHint,
		Status:               c.Status,
		CreatedAt:            c.CreatedAt,
		UpdatedAt:            c.UpdatedAt,
	}
}

// AgentConnectionDTO is a sanitized Data Transfer Object for public API responses.
type AgentConnectionDTO struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	PlanHint       string    `json:"plan_hint"`
	Status         string    `json:"status"`
	HasCredentials bool      `json:"has_credentials"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ToDTO converts an AgentConnection into its sanitized public DTO.
func (c *AgentConnection) ToDTO() AgentConnectionDTO {
	return AgentConnectionDTO{
		ID:             c.ID,
		UserID:         c.UserID,
		Provider:       c.Provider,
		PlanHint:       c.PlanHint,
		Status:         c.Status,
		HasCredentials: strings.TrimSpace(c.CredentialsEncrypted) != "",
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

// CreateAgentConnectionInput contains attributes for creating an AgentConnection.
type CreateAgentConnectionInput struct {
	ID                   string `json:"id,omitempty"`
	UserID               string `json:"user_id"`
	Provider             string `json:"provider,omitempty"`
	CredentialsEncrypted string `json:"credentials_encrypted,omitempty"`
	PlanHint             string `json:"plan_hint,omitempty"`
	Status               string `json:"status,omitempty"`
}

// Validate checks validity of CreateAgentConnectionInput.
func (in *CreateAgentConnectionInput) Validate() error {
	if strings.TrimSpace(in.UserID) == "" {
		return ErrEmptyUserID
	}
	if in.Status != "" {
		if _, err := NormalizeConnectionStatus(in.Status); err != nil {
			return err
		}
	}
	return nil
}

// ToEntity transforms input into an AgentConnection entity.
func (in *CreateAgentConnectionInput) ToEntity() (*AgentConnection, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	status := DefaultConnectionStatus
	if in.Status != "" {
		norm, err := NormalizeConnectionStatus(in.Status)
		if err != nil {
			return nil, err
		}
		status = norm
	}
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		provider = DefaultConnectionProvider
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = GenerateUUID()
	}

	return &AgentConnection{
		ID:                   id,
		UserID:               strings.TrimSpace(in.UserID),
		Provider:             provider,
		CredentialsEncrypted: strings.TrimSpace(in.CredentialsEncrypted),
		PlanHint:             strings.TrimSpace(in.PlanHint),
		Status:               status,
	}, nil
}

// UpdateAgentConnectionInput specifies mutable fields for an AgentConnection.
type UpdateAgentConnectionInput struct {
	Provider             *string `json:"provider,omitempty"`
	CredentialsEncrypted *string `json:"credentials_encrypted,omitempty"`
	PlanHint             *string `json:"plan_hint,omitempty"`
	Status               *string `json:"status,omitempty"`
}

// Validate validates non-nil fields of UpdateAgentConnectionInput.
func (in *UpdateAgentConnectionInput) Validate() error {
	if in.Status != nil {
		if _, err := NormalizeConnectionStatus(*in.Status); err != nil {
			return err
		}
	}
	return nil
}

// ApplyTo modifies the target AgentConnection in-place.
func (in *UpdateAgentConnectionInput) ApplyTo(conn *AgentConnection) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if in.Provider != nil && strings.TrimSpace(*in.Provider) != "" {
		conn.Provider = strings.TrimSpace(*in.Provider)
	}
	if in.CredentialsEncrypted != nil {
		conn.CredentialsEncrypted = strings.TrimSpace(*in.CredentialsEncrypted)
	}
	if in.PlanHint != nil {
		conn.PlanHint = strings.TrimSpace(*in.PlanHint)
	}
	if in.Status != nil {
		norm, _ := NormalizeConnectionStatus(*in.Status)
		conn.Status = norm
	}
	return nil
}

// AgentConnectionFilter configures criteria for querying connections.
type AgentConnectionFilter struct {
	UserID   string `json:"user_id,omitempty"`
	Provider string `json:"provider,omitempty"`
	Status   string `json:"status,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// AgentRepository defines storage operations for Agents and AgentConnections.
type AgentRepository interface {
	// Agent methods
	CreateAgent(ctx context.Context, agent *Agent) error
	GetAgentByID(ctx context.Context, id string) (*Agent, error)
	ListAgents(ctx context.Context, filter AgentFilter) ([]Agent, error)
	UpdateAgent(ctx context.Context, agent *Agent) error
	DeleteAgent(ctx context.Context, id string) error
	CountAgents(ctx context.Context, filter AgentFilter) (int, error)

	// AgentConnection methods
	CreateConnection(ctx context.Context, conn *AgentConnection) error
	GetConnectionByID(ctx context.Context, id string) (*AgentConnection, error)
	GetConnectionByUserAndProvider(ctx context.Context, userID, provider string) (*AgentConnection, error)
	ListConnections(ctx context.Context, filter AgentConnectionFilter) ([]AgentConnection, error)
	UpdateConnection(ctx context.Context, conn *AgentConnection) error
	DeleteConnection(ctx context.Context, id string) error
	CountConnections(ctx context.Context, filter AgentConnectionFilter) (int, error)
}

// ============================================================================
// SQL Implementation (PostgreSQL)
// ============================================================================

type SQLAgentRepository struct {
	db *sql.DB
}

// NewSQLAgentRepository constructs an AgentRepository backed by PostgreSQL.
func NewSQLAgentRepository(db *sql.DB) *SQLAgentRepository {
	return &SQLAgentRepository{db: db}
}

func (r *SQLAgentRepository) CreateAgent(ctx context.Context, agent *Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}
	if strings.TrimSpace(agent.Name) == "" {
		return ErrEmptyAgentName
	}
	if agent.Status == "" {
		if agent.Enabled {
			agent.Status = AgentStatusActive
		} else {
			agent.Status = AgentStatusInactive
		}
	}
	normStatus, err := NormalizeAgentStatus(agent.Status)
	if err != nil {
		return err
	}
	agent.Status = normStatus
	agent.Enabled = (agent.Status == AgentStatusActive)

	if strings.TrimSpace(agent.ID) == "" {
		agent.ID = GenerateUUID()
	}
	if strings.TrimSpace(agent.Provider) == "" {
		if strings.TrimSpace(agent.Harness) != "" {
			agent.Provider = agent.Harness
		} else {
			agent.Provider = DefaultAgentProvider
		}
	}
	if strings.TrimSpace(agent.Harness) == "" {
		if agent.Provider != DefaultAgentProvider {
			agent.Harness = agent.Provider
		} else {
			agent.Harness = "agy"
		}
	}
	if strings.TrimSpace(agent.ConnectionID) != "" {
		if agent.ModelID != "" && agent.ModelID != agent.ConnectionID && !strings.HasPrefix(agent.ModelID, "conn_") {
			agent.ModelID = agent.ConnectionID + "#" + agent.ModelID
		} else if agent.ModelID == "" || agent.ModelID == DefaultAgentModelID {
			agent.ModelID = agent.ConnectionID
		}
	}
	if strings.TrimSpace(agent.ModelID) == "" {
		agent.ModelID = DefaultAgentModelID
	}
	if strings.TrimSpace(agent.Provider) == "" {
		agent.Provider = DefaultAgentProvider
	}

	now := time.Now().UTC()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	if agent.UpdatedAt.IsZero() {
		agent.UpdatedAt = now
	}

	query := `
		INSERT INTO agents (
			id, name, avatar, role, system_prompt, model_id, fallback_model_id, temperature, token_budget, provider, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err = r.db.ExecContext(
		ctx,
		query,
		agent.ID,
		agent.Name,
		agent.Avatar,
		agent.Role,
		agent.SystemPrompt,
		agent.ModelID,
		agent.FallbackModelID,
		agent.Temperature,
		agent.TokenBudget,
		agent.Provider,
		agent.Status,
		agent.CreatedAt,
		agent.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrDuplicateAgent
		}
		return fmt.Errorf("create agent: %w", err)
	}
	return nil
}

func (r *SQLAgentRepository) GetAgentByID(ctx context.Context, id string) (*Agent, error) {
	query := `
		SELECT id, name, avatar, role, system_prompt, model_id, fallback_model_id, temperature, token_budget, provider, status, created_at, updated_at
		FROM agents
		WHERE id = $1
	`
	var a Agent
	var fallbackModel sql.NullString
	var temperature sql.NullFloat64
	var tokenBudget sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID,
		&a.Name,
		&a.Avatar,
		&a.Role,
		&a.SystemPrompt,
		&a.ModelID,
		&fallbackModel,
		&temperature,
		&tokenBudget,
		&a.Provider,
		&a.Status,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if id == "agent_sm" || id == "agent_dev" || id == "agent_support" {
				return r.GetAgentByID(ctx, AgentIDCuan)
			}
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("get agent by id: %w", err)
	}
	if fallbackModel.Valid {
		a.FallbackModelID = fallbackModel.String
	}
	if temperature.Valid {
		temp := temperature.Float64
		a.Temperature = &temp
	}
	if tokenBudget.Valid {
		budget := int(tokenBudget.Int64)
		a.TokenBudget = &budget
	}
	if a.Harness == "" {
		a.Harness = a.Provider
	}
	if strings.HasPrefix(a.ModelID, "conn_") {
		if idx := strings.Index(a.ModelID, "#"); idx != -1 {
			if a.ConnectionID == "" {
				a.ConnectionID = a.ModelID[:idx]
			}
			a.ModelID = a.ModelID[idx+1:]
		} else {
			if a.ConnectionID == "" {
				a.ConnectionID = a.ModelID
			}
		}
	}
	a.Enabled = (a.Status == AgentStatusActive || a.Status == "")
	return &a, nil
}

func (r *SQLAgentRepository) ListAgents(ctx context.Context, filter AgentFilter) ([]Agent, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(filter.Provider) != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Provider))
		idx++
	}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Status))
		idx++
	}
	if strings.TrimSpace(filter.Role) != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Role))
		idx++
	}
	if strings.TrimSpace(filter.Search) != "" {
		pattern := "%" + strings.TrimSpace(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR role ILIKE $%d OR system_prompt ILIKE $%d)", idx, idx, idx))
		args = append(args, pattern)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, name, avatar, role, system_prompt, model_id, fallback_model_id, temperature, token_budget, provider, status, created_at, updated_at
		FROM agents
		%s
		ORDER BY created_at DESC
	`, whereClause)

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, filter.Limit)
		idx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", idx)
		args = append(args, filter.Offset)
		idx++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var fallbackModel sql.NullString
		var temperature sql.NullFloat64
		var tokenBudget sql.NullInt64
		if err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.Avatar,
			&a.Role,
			&a.SystemPrompt,
			&a.ModelID,
			&fallbackModel,
			&temperature,
			&tokenBudget,
			&a.Provider,
			&a.Status,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if fallbackModel.Valid {
			a.FallbackModelID = fallbackModel.String
		}
		if temperature.Valid {
			temp := temperature.Float64
			a.Temperature = &temp
		}
		if tokenBudget.Valid {
			budget := int(tokenBudget.Int64)
			a.TokenBudget = &budget
		}
		if a.Harness == "" {
			a.Harness = a.Provider
		}
		if strings.HasPrefix(a.ModelID, "conn_") {
			if idx := strings.Index(a.ModelID, "#"); idx != -1 {
				if a.ConnectionID == "" {
					a.ConnectionID = a.ModelID[:idx]
				}
				a.ModelID = a.ModelID[idx+1:]
			} else {
				if a.ConnectionID == "" {
					a.ConnectionID = a.ModelID
				}
			}
		}
		a.Enabled = (a.Status == AgentStatusActive || a.Status == "")
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return agents, nil
}

func (r *SQLAgentRepository) UpdateAgent(ctx context.Context, agent *Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}
	if strings.TrimSpace(agent.Name) == "" {
		return ErrEmptyAgentName
	}
	if strings.TrimSpace(agent.Harness) != "" {
		agent.Provider = agent.Harness
	}
	if strings.TrimSpace(agent.ConnectionID) != "" {
		if agent.ModelID != "" && agent.ModelID != agent.ConnectionID && !strings.HasPrefix(agent.ModelID, "conn_") {
			agent.ModelID = agent.ConnectionID + "#" + agent.ModelID
		} else {
			agent.ModelID = agent.ConnectionID
		}
	}
	if agent.Status == "" {
		if agent.Enabled {
			agent.Status = AgentStatusActive
		} else {
			agent.Status = AgentStatusInactive
		}
	}
	normStatus, err := NormalizeAgentStatus(agent.Status)
	if err != nil {
		return err
	}
	agent.Status = normStatus
	agent.Enabled = (agent.Status == AgentStatusActive)
	agent.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE agents
		SET name = $1, avatar = $2, role = $3, system_prompt = $4, model_id = $5, fallback_model_id = $6, temperature = $7, token_budget = $8, provider = $9, status = $10, updated_at = $11
		WHERE id = $12
	`
	res, err := r.db.ExecContext(
		ctx,
		query,
		agent.Name,
		agent.Avatar,
		agent.Role,
		agent.SystemPrompt,
		agent.ModelID,
		agent.FallbackModelID,
		agent.Temperature,
		agent.TokenBudget,
		agent.Provider,
		agent.Status,
		agent.UpdatedAt,
		agent.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

func (r *SQLAgentRepository) DeleteAgent(ctx context.Context, id string) error {
	query := `DELETE FROM agents WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrAgentNotFound
	}
	return nil
}

func (r *SQLAgentRepository) CountAgents(ctx context.Context, filter AgentFilter) (int, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(filter.Provider) != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Provider))
		idx++
	}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Status))
		idx++
	}
	if strings.TrimSpace(filter.Role) != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Role))
		idx++
	}
	if strings.TrimSpace(filter.Search) != "" {
		pattern := "%" + strings.TrimSpace(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR role ILIKE $%d OR system_prompt ILIKE $%d)", idx, idx, idx))
		args = append(args, pattern)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM agents %s`, whereClause)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count agents: %w", err)
	}
	return count, nil
}

func (r *SQLAgentRepository) CreateConnection(ctx context.Context, conn *AgentConnection) error {
	if conn == nil {
		return errors.New("connection cannot be nil")
	}
	if strings.TrimSpace(conn.UserID) == "" {
		return ErrEmptyUserID
	}
	normStatus, err := NormalizeConnectionStatus(conn.Status)
	if err != nil {
		return err
	}
	conn.Status = normStatus

	if strings.TrimSpace(conn.ID) == "" {
		conn.ID = GenerateUUID()
	}
	if strings.TrimSpace(conn.Provider) == "" {
		conn.Provider = DefaultConnectionProvider
	}

	now := time.Now().UTC()
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = now
	}
	if conn.UpdatedAt.IsZero() {
		conn.UpdatedAt = now
	}

	query := `
		INSERT INTO agent_connections (
			id, user_id, provider, credentials_encrypted, plan_hint, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = r.db.ExecContext(
		ctx,
		query,
		conn.ID,
		conn.UserID,
		conn.Provider,
		conn.CredentialsEncrypted,
		conn.PlanHint,
		conn.Status,
		conn.CreatedAt,
		conn.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrDuplicateConnection
		}
		return fmt.Errorf("create agent connection: %w", err)
	}
	return nil
}

func (r *SQLAgentRepository) GetConnectionByID(ctx context.Context, id string) (*AgentConnection, error) {
	query := `
		SELECT id, user_id, provider, credentials_encrypted, plan_hint, status, created_at, updated_at
		FROM agent_connections
		WHERE id = $1
	`
	var c AgentConnection
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID,
		&c.UserID,
		&c.Provider,
		&c.CredentialsEncrypted,
		&c.PlanHint,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentConnectionNotFound
		}
		return nil, fmt.Errorf("get agent connection by id: %w", err)
	}
	return &c, nil
}

func (r *SQLAgentRepository) GetConnectionByUserAndProvider(ctx context.Context, userID, provider string) (*AgentConnection, error) {
	query := `
		SELECT id, user_id, provider, credentials_encrypted, plan_hint, status, created_at, updated_at
		FROM agent_connections
		WHERE user_id = $1 AND provider = $2
		ORDER BY created_at DESC
		LIMIT 1
	`
	var c AgentConnection
	err := r.db.QueryRowContext(ctx, query, userID, provider).Scan(
		&c.ID,
		&c.UserID,
		&c.Provider,
		&c.CredentialsEncrypted,
		&c.PlanHint,
		&c.Status,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAgentConnectionNotFound
		}
		return nil, fmt.Errorf("get agent connection by user and provider: %w", err)
	}
	return &c, nil
}

func (r *SQLAgentRepository) ListConnections(ctx context.Context, filter AgentConnectionFilter) ([]AgentConnection, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(filter.UserID) != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, strings.TrimSpace(filter.UserID))
		idx++
	}
	if strings.TrimSpace(filter.Provider) != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Provider))
		idx++
	}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Status))
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, provider, credentials_encrypted, plan_hint, status, created_at, updated_at
		FROM agent_connections
		%s
		ORDER BY created_at DESC
	`, whereClause)

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", idx)
		args = append(args, filter.Limit)
		idx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", idx)
		args = append(args, filter.Offset)
		idx++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	var list []AgentConnection
	for rows.Next() {
		var c AgentConnection
		if err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Provider,
			&c.CredentialsEncrypted,
			&c.PlanHint,
			&c.Status,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return list, nil
}

func (r *SQLAgentRepository) UpdateConnection(ctx context.Context, conn *AgentConnection) error {
	if conn == nil {
		return errors.New("connection cannot be nil")
	}
	normStatus, err := NormalizeConnectionStatus(conn.Status)
	if err != nil {
		return err
	}
	conn.Status = normStatus
	conn.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE agent_connections
		SET provider = $1, credentials_encrypted = $2, plan_hint = $3, status = $4, updated_at = $5
		WHERE id = $6
	`
	res, err := r.db.ExecContext(
		ctx,
		query,
		conn.Provider,
		conn.CredentialsEncrypted,
		conn.PlanHint,
		conn.Status,
		conn.UpdatedAt,
		conn.ID,
	)
	if err != nil {
		return fmt.Errorf("update connection: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrAgentConnectionNotFound
	}
	return nil
}

func (r *SQLAgentRepository) DeleteConnection(ctx context.Context, id string) error {
	query := `DELETE FROM agent_connections WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrAgentConnectionNotFound
	}
	return nil
}

func (r *SQLAgentRepository) CountConnections(ctx context.Context, filter AgentConnectionFilter) (int, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if strings.TrimSpace(filter.UserID) != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, strings.TrimSpace(filter.UserID))
		idx++
	}
	if strings.TrimSpace(filter.Provider) != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Provider))
		idx++
	}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, strings.TrimSpace(filter.Status))
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM agent_connections %s`, whereClause)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count connections: %w", err)
	}
	return count, nil
}

// ============================================================================
// Thread-Safe In-Memory Store
// ============================================================================

type MemoryAgentRepository struct {
	mu          sync.RWMutex
	agents      map[string]*Agent
	connections map[string]*AgentConnection
}

// NewMemoryAgentRepository creates a new thread-safe in-memory AgentRepository.
func NewMemoryAgentRepository() *MemoryAgentRepository {
	return &MemoryAgentRepository{
		agents:      make(map[string]*Agent),
		connections: make(map[string]*AgentConnection),
	}
}

func (m *MemoryAgentRepository) CreateAgent(_ context.Context, agent *Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}
	if strings.TrimSpace(agent.Name) == "" {
		return ErrEmptyAgentName
	}
	if strings.TrimSpace(agent.Provider) == "" {
		if strings.TrimSpace(agent.Harness) != "" {
			agent.Provider = agent.Harness
		} else {
			agent.Provider = DefaultAgentProvider
		}
	}
	if strings.TrimSpace(agent.Harness) == "" {
		if agent.Provider != DefaultAgentProvider {
			agent.Harness = agent.Provider
		} else {
			agent.Harness = "agy"
		}
	}
	if strings.TrimSpace(agent.ConnectionID) != "" {
		if agent.ModelID != "" && agent.ModelID != agent.ConnectionID && !strings.HasPrefix(agent.ModelID, "conn_") {
			agent.ModelID = agent.ConnectionID + "#" + agent.ModelID
		} else if agent.ModelID == "" || agent.ModelID == DefaultAgentModelID {
			agent.ModelID = agent.ConnectionID
		}
	}
	if agent.Status == "" {
		if agent.Enabled {
			agent.Status = AgentStatusActive
		} else {
			agent.Status = AgentStatusInactive
		}
	}
	normStatus, err := NormalizeAgentStatus(agent.Status)
	if err != nil {
		return err
	}
	agent.Status = normStatus
	agent.Enabled = (normStatus == AgentStatusActive)

	if strings.TrimSpace(agent.ID) == "" {
		agent.ID = GenerateUUID()
	}
	if strings.TrimSpace(agent.ModelID) == "" {
		agent.ModelID = DefaultAgentModelID
	}
	if strings.TrimSpace(agent.Provider) == "" {
		agent.Provider = DefaultAgentProvider
	}

	now := time.Now().UTC()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	if agent.UpdatedAt.IsZero() {
		agent.UpdatedAt = now
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[agent.ID]; exists {
		return ErrDuplicateAgent
	}
	m.agents[agent.ID] = agent.Clone()
	return nil
}

func (m *MemoryAgentRepository) GetAgentByID(_ context.Context, id string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, exists := m.agents[id]
	if !exists {
		if id == "agent_sm" || id == "agent_dev" || id == "agent_support" {
			if cuan, existsCuan := m.agents[AgentIDCuan]; existsCuan {
				return cuan.Clone(), nil
			}
		}
		return nil, ErrAgentNotFound
	}
	return a.Clone(), nil
}

func (m *MemoryAgentRepository) ListAgents(_ context.Context, filter AgentFilter) ([]Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []Agent
	search := strings.ToLower(strings.TrimSpace(filter.Search))

	for _, a := range m.agents {
		if filter.Provider != "" && !strings.EqualFold(a.Provider, filter.Provider) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(a.Status, filter.Status) {
			continue
		}
		if filter.Role != "" && !strings.EqualFold(a.Role, filter.Role) {
			continue
		}
		if search != "" {
			nameMatch := strings.Contains(strings.ToLower(a.Name), search)
			roleMatch := strings.Contains(strings.ToLower(a.Role), search)
			promptMatch := strings.Contains(strings.ToLower(a.SystemPrompt), search)
			if !nameMatch && !roleMatch && !promptMatch {
				continue
			}
		}
		list = append(list, *a.Clone())
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	if filter.Offset > 0 {
		if filter.Offset >= len(list) {
			return []Agent{}, nil
		}
		list = list[filter.Offset:]
	}
	if filter.Limit > 0 && len(list) > filter.Limit {
		list = list[:filter.Limit]
	}

	return list, nil
}

func (m *MemoryAgentRepository) UpdateAgent(_ context.Context, agent *Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}
	if strings.TrimSpace(agent.Name) == "" {
		return ErrEmptyAgentName
	}
	if agent.Status == "" {
		if agent.Enabled {
			agent.Status = AgentStatusActive
		} else {
			agent.Status = AgentStatusInactive
		}
	}
	normStatus, err := NormalizeAgentStatus(agent.Status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.agents[agent.ID]
	if !exists {
		return ErrAgentNotFound
	}

	existing.Name = agent.Name
	if strings.TrimSpace(agent.Harness) != "" {
		existing.Harness = agent.Harness
		existing.Provider = agent.Harness
	}
	if strings.TrimSpace(agent.ConnectionID) != "" {
		existing.ConnectionID = agent.ConnectionID
	}
	existing.Enabled = (normStatus == AgentStatusActive)
	existing.Avatar = agent.Avatar
	existing.Role = agent.Role
	existing.SystemPrompt = agent.SystemPrompt
	if strings.TrimSpace(agent.ModelID) != "" {
		existing.ModelID = agent.ModelID
	}
	existing.FallbackModelID = agent.FallbackModelID
	if agent.Temperature != nil {
		tVal := *agent.Temperature
		existing.Temperature = &tVal
	} else {
		existing.Temperature = nil
	}
	if agent.TokenBudget != nil {
		bVal := *agent.TokenBudget
		existing.TokenBudget = &bVal
	} else {
		existing.TokenBudget = nil
	}
	if strings.TrimSpace(agent.Provider) != "" {
		existing.Provider = agent.Provider
	}
	existing.Status = normStatus
	existing.UpdatedAt = time.Now().UTC()

	return nil
}

func (m *MemoryAgentRepository) DeleteAgent(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[id]; !exists {
		return ErrAgentNotFound
	}
	delete(m.agents, id)
	return nil
}

func (m *MemoryAgentRepository) CountAgents(_ context.Context, filter AgentFilter) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	search := strings.ToLower(strings.TrimSpace(filter.Search))

	for _, a := range m.agents {
		if filter.Provider != "" && !strings.EqualFold(a.Provider, filter.Provider) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(a.Status, filter.Status) {
			continue
		}
		if filter.Role != "" && !strings.EqualFold(a.Role, filter.Role) {
			continue
		}
		if search != "" {
			nameMatch := strings.Contains(strings.ToLower(a.Name), search)
			roleMatch := strings.Contains(strings.ToLower(a.Role), search)
			promptMatch := strings.Contains(strings.ToLower(a.SystemPrompt), search)
			if !nameMatch && !roleMatch && !promptMatch {
				continue
			}
		}
		count++
	}
	return count, nil
}

func (m *MemoryAgentRepository) CreateConnection(_ context.Context, conn *AgentConnection) error {
	if conn == nil {
		return errors.New("connection cannot be nil")
	}
	if strings.TrimSpace(conn.UserID) == "" {
		return ErrEmptyUserID
	}
	normStatus, err := NormalizeConnectionStatus(conn.Status)
	if err != nil {
		return err
	}
	conn.Status = normStatus

	if strings.TrimSpace(conn.ID) == "" {
		conn.ID = GenerateUUID()
	}
	if strings.TrimSpace(conn.Provider) == "" {
		conn.Provider = DefaultConnectionProvider
	}

	now := time.Now().UTC()
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = now
	}
	if conn.UpdatedAt.IsZero() {
		conn.UpdatedAt = now
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[conn.ID]; exists {
		return ErrDuplicateConnection
	}
	m.connections[conn.ID] = conn.Clone()
	return nil
}

func (m *MemoryAgentRepository) GetConnectionByID(_ context.Context, id string) (*AgentConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, exists := m.connections[id]
	if !exists {
		return nil, ErrAgentConnectionNotFound
	}
	return c.Clone(), nil
}

func (m *MemoryAgentRepository) GetConnectionByUserAndProvider(_ context.Context, userID, provider string) (*AgentConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*AgentConnection
	for _, c := range m.connections {
		if c.UserID == userID && strings.EqualFold(c.Provider, provider) {
			matches = append(matches, c.Clone())
		}
	}
	if len(matches) == 0 {
		return nil, ErrAgentConnectionNotFound
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	return matches[0], nil
}

func (m *MemoryAgentRepository) ListConnections(_ context.Context, filter AgentConnectionFilter) ([]AgentConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []AgentConnection
	for _, c := range m.connections {
		if filter.UserID != "" && c.UserID != filter.UserID {
			continue
		}
		if filter.Provider != "" && !strings.EqualFold(c.Provider, filter.Provider) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(c.Status, filter.Status) {
			continue
		}
		list = append(list, *c.Clone())
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID > list[j].ID
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	if filter.Offset > 0 {
		if filter.Offset >= len(list) {
			return []AgentConnection{}, nil
		}
		list = list[filter.Offset:]
	}
	if filter.Limit > 0 && len(list) > filter.Limit {
		list = list[:filter.Limit]
	}

	return list, nil
}

func (m *MemoryAgentRepository) UpdateConnection(_ context.Context, conn *AgentConnection) error {
	if conn == nil {
		return errors.New("connection cannot be nil")
	}
	normStatus, err := NormalizeConnectionStatus(conn.Status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.connections[conn.ID]
	if !exists {
		return ErrAgentConnectionNotFound
	}

	if strings.TrimSpace(conn.Provider) != "" {
		existing.Provider = conn.Provider
	}
	existing.CredentialsEncrypted = conn.CredentialsEncrypted
	existing.PlanHint = conn.PlanHint
	existing.Status = normStatus
	existing.UpdatedAt = time.Now().UTC()

	return nil
}

func (m *MemoryAgentRepository) DeleteConnection(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.connections[id]; !exists {
		return ErrAgentConnectionNotFound
	}
	delete(m.connections, id)
	return nil
}

func (m *MemoryAgentRepository) CountConnections(_ context.Context, filter AgentConnectionFilter) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, c := range m.connections {
		if filter.UserID != "" && c.UserID != filter.UserID {
			continue
		}
		if filter.Provider != "" && !strings.EqualFold(c.Provider, filter.Provider) {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(c.Status, filter.Status) {
			continue
		}
		count++
	}
	return count, nil
}

// EnsureBuiltinAgents checks and seeds the default builtin agents (@cuan, @sm, @dev, @support)
// with sensible model configurations if they do not yet exist in the repository.
// @cuan acts as the primary Single Agent Facade.
func EnsureBuiltinAgents(ctx context.Context, repo AgentRepository) error {
	if repo == nil {
		return nil
	}
	defaultCuanTemp := 0.7
	defaultCuanBudget := 8192
	defaultSMTemp := 0.7
	defaultSMBudget := 8192
	defaultDevTemp := 0.2
	defaultDevBudget := 8192
	defaultSupportTemp := 0.5
	defaultSupportBudget := 4096

	builtins := []*Agent{
		{
			ID:              AgentIDCuan,
			Name:            DefaultAgentHandle,
			Role:            "Cuan Universal Assistant",
			SystemPrompt:    "You are @cuan, the universal AI assistant for Cuan. You assist users with operations, customer support, task management, code implementation, and business automation. You communicate warmly, concisely, and effectively without technical jargon. You have full access to platform tools and orchestrate execution seamlessly across available engines.",
			ModelID:         "gemini-1.5-pro",
			FallbackModelID: "gemini-1.5-flash",
			Temperature:     &defaultCuanTemp,
			TokenBudget:     &defaultCuanBudget,
			Harness:         "agy",
			Provider:        "google",
			Status:          AgentStatusActive,
			Enabled:         true,
		},
		{
			ID:              "agent_sm",
			Name:            "@sm",
			Role:            "Scrum Master (Orchestrator)",
			SystemPrompt:    "You coordinate Cuan conversations as Scrum Master. All agents in Inbox act as Scrum Master / Technical Orchestrator with Full MCP access across all projects and platform modules.",
			ModelID:         "gemini-1.5-pro",
			FallbackModelID: "gemini-1.5-flash",
			Temperature:     &defaultSMTemp,
			TokenBudget:     &defaultSMBudget,
			Harness:         "agy",
			Provider:        "google",
			Status:          AgentStatusActive,
			Enabled:         true,
		},
		{
			ID:              "agent_dev",
			Name:            "@dev",
			Role:            "Development Team (Worker)",
			SystemPrompt:    "Autonomous development agent specialized in code implementation, test verification, bug fixing, and task execution.",
			ModelID:         "gemini-1.5-pro",
			FallbackModelID: "gemini-1.5-flash",
			Temperature:     &defaultDevTemp,
			TokenBudget:     &defaultDevBudget,
			Harness:         "agy",
			Provider:        "google",
			Status:          AgentStatusActive,
			Enabled:         true,
		},
		{
			ID:              "agent_support",
			Name:            "@support",
			Role:            "Customer Support",
			SystemPrompt:    "Customer support specialist dedicated to answering customer questions, triaging issues, and maintaining helpful and friendly communication.",
			ModelID:         "gemini-1.5-flash",
			FallbackModelID: "gemini-1.5-pro",
			Temperature:     &defaultSupportTemp,
			TokenBudget:     &defaultSupportBudget,
			Harness:         "agy",
			Provider:        "google",
			Status:          AgentStatusActive,
			Enabled:         true,
		},
	}

	for _, b := range builtins {
		existing, err := repo.GetAgentByID(ctx, b.ID)
		if err != nil || (existing != nil && existing.ID != b.ID) {
			if errors.Is(err, ErrAgentNotFound) || (existing != nil && existing.ID != b.ID) {
				_ = repo.CreateAgent(ctx, b)
			}
			continue
		}
		updated := false
		if existing.FallbackModelID == "" && b.FallbackModelID != "" {
			existing.FallbackModelID = b.FallbackModelID
			updated = true
		}
		if existing.Temperature == nil && b.Temperature != nil {
			existing.Temperature = b.Temperature
			updated = true
		}
		if existing.TokenBudget == nil && b.TokenBudget != nil {
			existing.TokenBudget = b.TokenBudget
			updated = true
		}
		if updated {
			_ = repo.UpdateAgent(ctx, existing)
		}
	}
	return nil
}
