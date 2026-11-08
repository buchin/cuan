package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/webfetch"
)

// Registry manages the lifecycle of Apps, Skills, Agent bindings, and MCP tools.
type Registry struct {
	mu          sync.RWMutex
	apps        map[string]*App
	skills      map[string]*Skill
	agentSkills map[string]map[string]bool // agentID -> skillID -> enabled
	tools       map[string]ToolDefinition
	handlers    map[string]ToolHandler
}

var (
	defaultRegistry     *Registry
	defaultRegistryOnce sync.Once
)

// GetDefaultRegistry returns the process-wide default MCP registry instance.
func GetDefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry()
	})
	return defaultRegistry
}

// NewRegistry initializes an MCP Registry pre-populated with built-in connectors and default agent bindings.
func NewRegistry() *Registry {
	r := &Registry{
		apps:        make(map[string]*App),
		skills:      make(map[string]*Skill),
		agentSkills: make(map[string]map[string]bool),
		tools:       make(map[string]ToolDefinition),
		handlers:    make(map[string]ToolHandler),
	}

	// 1. Register Built-in Apps
	sheetsApp := NewGoogleSheetsApp()
	r.apps[sheetsApp.ID] = sheetsApp

	// 2. Register Built-in Skills
	visionSkill := NewVisionReceiptExtractorSkill()
	r.skills[visionSkill.ID] = visionSkill

	now := time.Now()
	sheetsSkill := &Skill{
		ID:          SkillGoogleSheetsSyncID,
		AppID:       AppGoogleSheetsID,
		Name:        "Google Sheets Sync",
		Description: "Mencatat transaksi dan mutasi keuangan secara otomatis ke spreadsheet Google terpilih.",
		Icon:        "table",
		Category:    "finance",
		Enabled:     true,
		Tools: []string{
			"google_sheets_append_row",
			"google_sheets_read_sheet",
		},
		BoundAgents: []string{
			"agent_cuan",
			"agent_sm",
			"agent_dev",
			"agent_support",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.skills[sheetsSkill.ID] = sheetsSkill

	// 3. Register default bindings for known agents
	knownAgents := []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"}
	for _, a := range knownAgents {
		r.agentSkills[a] = map[string]bool{
			SkillVisionReceiptExtractorID: true,
			SkillGoogleSheetsSyncID:       true,
		}
	}

	// 4. Register Built-in Tool Implementations
	r.RegisterTool(
		ToolDefinition{
			Name:        "google_sheets_append_row",
			Description: "Append a row of transaction or record data into the connected Google Sheet.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"spreadsheet_id": map[string]any{"type": "string", "description": "Google Spreadsheet ID or URL"},
					"sheet_name":     map[string]any{"type": "string", "description": "Target worksheet title (default: Sheet1)"},
					"values":         map[string]any{"type": "array", "description": "Row values or transaction record to append"},
				},
				"required": []string{"values"},
			},
		},
		GoogleSheetsAppendRowHandler(r),
	)

	r.RegisterTool(
		ToolDefinition{
			Name:        "google_sheets_read_sheet",
			Description: "Read rows from a connected Google Sheet within a specified range.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"spreadsheet_id": map[string]any{"type": "string", "description": "Google Spreadsheet ID or URL"},
					"range":          map[string]any{"type": "string", "description": "Cell range (e.g. Sheet1!A1:Z50)"},
				},
			},
		},
		GoogleSheetsReadSheetHandler(r),
	)

	r.RegisterTool(
		ToolDefinition{
			Name:        "vision_extract_receipt",
			Description: "Extract structured financial JSON data (merchant, date, total amount, items, category) from receipt text, OCR output, or image input.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"receipt_text": map[string]any{"type": "string", "description": "Raw receipt text, OCR text lines, or message content"},
					"image_url":    map[string]any{"type": "string", "description": "Optional URL of receipt image"},
				},
			},
		},
		VisionExtractReceiptHandler(),
	)

	// 5. Register Internal Co-pilot Tools
	r.RegisterTool(
		ToolDefinition{
			Name:        "list_apps",
			Description: "List all connected and available external third-party Apps in Cuan ecosystem.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category": map[string]any{"type": "string", "description": "Filter by category (optional)"},
				},
			},
		},
		func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
			apps := r.ListApps()
			b, _ := json.Marshal(apps)
			return &ToolCallResponse{
				IsError: false,
				Content: []ToolContent{{Type: "text", Text: string(b)}},
			}, nil
		},
	)

	r.RegisterTool(
		ToolDefinition{
			Name:        "list_skills",
			Description: "List all modular intelligence skills in Cuan with their enabled status and bound agents.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category": map[string]any{"type": "string", "description": "Filter by category (optional)"},
				},
			},
		},
		func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
			skills := r.ListSkills()
			b, _ := json.Marshal(skills)
			return &ToolCallResponse{
				IsError: false,
				Content: []ToolContent{{Type: "text", Text: string(b)}},
			}, nil
		},
	)

	r.RegisterTool(
		ToolDefinition{
			Name:        "toggle_agent_skill",
			Description: "Enable or disable a specific modular skill for a designated agent persona.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agent_id": map[string]any{"type": "string", "description": "Agent ID (e.g. agent_sm, agent_dev, agent_support)"},
					"skill_id": map[string]any{"type": "string", "description": "Skill ID (e.g. vision_receipt_extractor, google_sheets_sync)"},
					"enabled":  map[string]any{"type": "boolean", "description": "True to activate, false to deactivate"},
				},
				"required": []string{"agent_id", "skill_id", "enabled"},
			},
		},
		func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
			agentID, _ := args["agent_id"].(string)
			skillID, _ := args["skill_id"].(string)
			enabled, _ := args["enabled"].(bool)

			if agentID == "" || skillID == "" {
				return nil, errors.New("agent_id and skill_id are required")
			}

			if err := r.ToggleAgentSkill(agentID, skillID, enabled); err != nil {
				return nil, err
			}

			statusStr := "enabled"
			if !enabled {
				statusStr = "disabled"
			}
			msg := fmt.Sprintf("Skill %s successfully %s for agent %s", skillID, statusStr, agentID)
			return &ToolCallResponse{
				IsError: false,
				Content: []ToolContent{{Type: "text", Text: msg}},
			}, nil
		},
	)

	r.RegisterTool(
		ToolDefinition{
			Name:        "web_fetch",
			Description: "Fetch and extract clean readable text content and title from a public web page URL (cleans ads, navigation, headers, scripts).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{"type": "string", "description": "Public HTTP or HTTPS URL of the web page or article to fetch"},
				},
				"required": []string{"url"},
			},
		},
		func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
			rawURL, _ := args["url"].(string)
			res, err := webfetch.Fetch(ctx, rawURL)
			if err != nil {
				return &ToolCallResponse{
					IsError: true,
					Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Error fetching web page: %v", err)}},
				}, nil
			}
			b, _ := json.Marshal(res)
			return &ToolCallResponse{
				IsError: false,
				Content: []ToolContent{{Type: "text", Text: string(b)}},
			}, nil
		},
	)

	return r
}

// ListApps returns all registered third-party apps ordered by name.
func (r *Registry) ListApps() []*App {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*App, 0, len(r.apps))
	for _, a := range r.apps {
		copyApp := *a
		list = append(list, &copyApp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

// GetApp retrieves a single app by ID.
func (r *Registry) GetApp(id string) (*App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	app, ok := r.apps[id]
	if !ok {
		return nil, fmt.Errorf("app with id '%s' not found", id)
	}
	copyApp := *app
	return &copyApp, nil
}

// ConnectApp connects the designated app with 1-click user simplicity.
func (r *Registry) ConnectApp(id string, account string) (*App, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	app, ok := r.apps[id]
	if !ok {
		return nil, fmt.Errorf("app with id '%s' not found", id)
	}

	if account == "" {
		account = "toko.berkah@gmail.com"
	}
	now := time.Now()
	app.Status = "connected"
	app.Account = account
	app.ConnectedAt = &now
	app.Error = ""

	copyApp := *app
	return &copyApp, nil
}

// DisconnectApp disconnects the designated app.
func (r *Registry) DisconnectApp(id string) (*App, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	app, ok := r.apps[id]
	if !ok {
		return nil, fmt.Errorf("app with id '%s' not found", id)
	}

	app.Status = "disconnected"
	app.Account = ""
	app.ConnectedAt = nil
	app.Error = ""

	copyApp := *app
	return &copyApp, nil
}

// ListSkills returns all modular skills ordered by category and name.
func (r *Registry) ListSkills() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		copySkill := *s
		// Recompute bound agents dynamically
		var bound []string
		for agentID, bindings := range r.agentSkills {
			if bindings[s.ID] {
				bound = append(bound, agentID)
			}
		}
		sort.Strings(bound)
		copySkill.BoundAgents = bound
		list = append(list, &copySkill)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

// GetSkill retrieves a specific skill by ID.
func (r *Registry) GetSkill(id string) (*Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[id]
	if !ok {
		return nil, fmt.Errorf("skill with id '%s' not found", id)
	}
	copySkill := *skill
	var bound []string
	for agentID, bindings := range r.agentSkills {
		if bindings[skill.ID] {
			bound = append(bound, agentID)
		}
	}
	sort.Strings(bound)
	copySkill.BoundAgents = bound
	return &copySkill, nil
}

// GetAgentSkills returns all skills and their enablement state for a given agent.
func (r *Registry) GetAgentSkills(agentID string) ([]AgentSkillView, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agentBindings := r.agentSkills[agentID]
	var views []AgentSkillView

	for _, s := range r.skills {
		enabled := false
		if agentBindings != nil {
			enabled = agentBindings[s.ID]
		}
		views = append(views, AgentSkillView{
			SkillID:     s.ID,
			Name:        s.Name,
			Description: s.Description,
			Icon:        s.Icon,
			Category:    s.Category,
			AppID:       s.AppID,
			Enabled:     enabled,
			Tools:       s.Tools,
			UpdatedAt:   s.UpdatedAt,
		})
	}

	sort.Slice(views, func(i, j int) bool {
		return views[i].Name < views[j].Name
	})
	return views, nil
}

// UpdateAgentSkills replaces the skill enablement state for an agent.
func (r *Registry) UpdateAgentSkills(agentID string, inputs []AgentSkillInput) ([]AgentSkillView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.agentSkills[agentID] == nil {
		r.agentSkills[agentID] = make(map[string]bool)
	}

	for _, input := range inputs {
		if _, ok := r.skills[input.SkillID]; ok {
			r.agentSkills[agentID][input.SkillID] = input.Enabled
		}
	}

	var views []AgentSkillView
	for _, s := range r.skills {
		enabled := r.agentSkills[agentID][s.ID]
		views = append(views, AgentSkillView{
			SkillID:     s.ID,
			Name:        s.Name,
			Description: s.Description,
			Icon:        s.Icon,
			Category:    s.Category,
			AppID:       s.AppID,
			Enabled:     enabled,
			Tools:       s.Tools,
			UpdatedAt:   time.Now(),
		})
	}

	sort.Slice(views, func(i, j int) bool {
		return views[i].Name < views[j].Name
	})
	return views, nil
}

// ToggleAgentSkill toggles a single skill binding for an agent.
func (r *Registry) ToggleAgentSkill(agentID string, skillID string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[skillID]; !ok {
		return fmt.Errorf("skill '%s' does not exist", skillID)
	}

	if r.agentSkills[agentID] == nil {
		r.agentSkills[agentID] = make(map[string]bool)
	}
	r.agentSkills[agentID][skillID] = enabled
	return nil
}

// RegisterTool registers an in-process tool definition and its execution handler.
func (r *Registry) RegisterTool(def ToolDefinition, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[def.Name] = def
	r.handlers[def.Name] = handler
}

// DiscoverTools returns all registered tool definitions.
func (r *Registry) DiscoverTools(ctx context.Context) ([]ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolDefinition, 0, len(r.tools))
	for _, def := range r.tools {
		tools = append(tools, def)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools, nil
}

// ExecuteTool safely executes a registered tool with bounded timeout and panic safety.
func (r *Registry) ExecuteTool(ctx context.Context, name string, args map[string]any) (*ToolCallResponse, error) {
	r.mu.RLock()
	handler, ok := r.handlers[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool '%s' not registered in registry", name)
	}

	return SafeExecuteTool(ctx, handler, args, DefaultToolTimeout)
}
