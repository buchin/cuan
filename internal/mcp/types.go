package mcp

import "time"

// App represents an integrated third-party application or service (e.g. Google Sheets, remote MCP).
type App struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Icon         string     `json:"icon"`
	Category     string     `json:"category"` // "productivity", "finance", "messaging", "connector", etc.
	Status       string     `json:"status"`   // "connected", "disconnected", "error"
	Account      string     `json:"account,omitempty"`
	BaseURL      string     `json:"base_url,omitempty"`
	Transport    string     `json:"transport,omitempty"` // "http", "sse"
	IsCustom     bool       `json:"is_custom,omitempty"`
	ConnectedAt  *time.Time `json:"connected_at,omitempty"`
	Capabilities []string   `json:"capabilities"`
	Error        string     `json:"error,omitempty"`
}

// Skill represents a modular intelligent capability that can be assigned to agents.
type Skill struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id,omitempty"` // Parent App ID if backed by an external app
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Category    string    `json:"category"` // "vision", "finance", "automation", "productivity", "custom"
	Enabled     bool      `json:"enabled"`
	Tools       []string  `json:"tools"`
	BoundAgents []string  `json:"bound_agents"` // IDs of agents currently bound to this skill
	SourceURL   string    `json:"source_url,omitempty"`
	Content     string    `json:"content,omitempty"`
	IsCustom    bool      `json:"is_custom,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AgentSkillBinding represents the binding of a skill to an agent persona.
type AgentSkillBinding struct {
	AgentID   string    `json:"agent_id"`
	SkillID   string    `json:"skill_id"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AgentSkillView is returned when querying an agent's skills configuration.
type AgentSkillView struct {
	SkillID     string    `json:"skill_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Category    string    `json:"category"`
	AppID       string    `json:"app_id,omitempty"`
	Enabled     bool      `json:"enabled"`
	Tools       []string  `json:"tools,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// AgentSkillInput is used for updating an agent's skill configuration.
type AgentSkillInput struct {
	SkillID string `json:"skill_id"`
	Enabled bool   `json:"enabled"`
}

// ReceiptItem represents a single item line extracted from a receipt.
type ReceiptItem struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
	Total    float64 `json:"total"`
}

// ReceiptExtraction represents structured financial data extracted from a receipt or bill.
type ReceiptExtraction struct {
	Merchant    string        `json:"merchant"`
	Date        string        `json:"date"`
	TotalAmount float64       `json:"total_amount"`
	Currency    string        `json:"currency"`
	Items       []ReceiptItem `json:"items"`
	Category    string        `json:"category"`
	Confidence  float64       `json:"confidence"`
	RawText     string        `json:"raw_text,omitempty"`
}
