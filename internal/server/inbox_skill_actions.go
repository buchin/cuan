package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
)

type inboxSkillAction struct {
	Action      string   `json:"action"` // "toggle", "install", "uninstall", "connect_mcp"
	AgentID     string   `json:"agent_id,omitempty"`
	SkillID     string   `json:"skill_id,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	URL         string   `json:"url,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category,omitempty"`
	Content     string   `json:"content,omitempty"`
	APIKey      string   `json:"api_key,omitempty"`
	Transport   string   `json:"transport,omitempty"`
	BoundAgents []string `json:"bound_agents,omitempty"`
}

func parseInboxSkill(content string) (string, *inboxSkillAction, error) {
	const marker = "```cuan-skill\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid skill action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxSkillAction
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&a) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	a.Action = strings.ToLower(strings.TrimSpace(a.Action))
	if a.Action == "" {
		a.Action = "toggle"
	}

	switch a.Action {
	case "install":
		if strings.TrimSpace(a.URL) == "" && strings.TrimSpace(a.Content) == "" && strings.TrimSpace(a.Name) == "" {
			return "", nil, errors.New("Skill URL, name, or content is required for installation.")
		}
	case "uninstall":
		if strings.TrimSpace(a.SkillID) == "" {
			return "", nil, errors.New("Skill ID is required for uninstall.")
		}
	case "connect_mcp":
		if strings.TrimSpace(a.URL) == "" {
			return "", nil, errors.New("MCP Server URL is required.")
		}
	case "toggle":
		if strings.TrimSpace(a.AgentID) == "" {
			return "", nil, errors.New("Agent ID is required.")
		}
		if strings.TrimSpace(a.SkillID) == "" {
			return "", nil, errors.New("Skill ID is required.")
		}
	default:
		return "", nil, errors.New("Unsupported skill action.")
	}

	return strings.TrimSpace(normalized[:start]), &a, nil
}

func (s *Server) applyInboxSkill(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxSkill(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}
	if s.mcpRegistry == nil {
		return cleanText + "\n\n⚠️ MCP registry is unavailable: skill manager is not configured."
	}

	switch action.Action {
	case "install":
		var skill *mcp.Skill
		var installErr error
		if strings.TrimSpace(action.URL) != "" {
			skill, installErr = s.mcpRegistry.InstallSkill(ctx, strings.TrimSpace(action.URL), action.BoundAgents)
		} else {
			skill, installErr = s.mcpRegistry.InstallSkillFromContent(
				ctx,
				strings.TrimSpace(action.SkillID),
				strings.TrimSpace(action.Name),
				strings.TrimSpace(action.Description),
				strings.TrimSpace(action.Category),
				strings.TrimSpace(action.Content),
				action.BoundAgents,
			)
		}
		if installErr != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memasang skill: %v", installErr)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Skill Baru Berhasil Dipasang:** **%s** (`%s`). Tersedia untuk seluruh agen.", skill.Name, skill.ID)

	case "uninstall":
		if err := s.mcpRegistry.UninstallSkill(ctx, action.SkillID); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal menghapus skill %s: %v", action.SkillID, err)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Skill Berhasil Dihapus:** `%s`.", action.SkillID)

	case "connect_mcp":
		name := strings.TrimSpace(action.Name)
		if name == "" {
			name = "Custom MCP Service"
		}
		cfg := mcp.ServerConfig{
			Name:        name,
			BaseURL:     strings.TrimSpace(action.URL),
			Transport:   mcp.TransportType(strings.TrimSpace(action.Transport)),
			APIKey:      strings.TrimSpace(action.APIKey),
			Description: strings.TrimSpace(action.Description),
			Enabled:     true,
		}
		app, tools, connectErr := s.mcpRegistry.RegisterRemoteMCP(ctx, cfg)
		if connectErr != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal menghubungkan MCP server %s: %v", action.URL, connectErr)
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Konektor MCP Terhubung:** **%s** (`%s`) dengan %d tools aktif siap digunakan.", app.Name, app.ID, len(tools))

	case "toggle":
		enabled := true
		if action.Enabled != nil {
			enabled = *action.Enabled
		}
		if err := s.mcpRegistry.ToggleAgentSkill(action.AgentID, action.SkillID, enabled); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Failed to update skill %s: %v", action.SkillID, err)
		}
		status := "diaktifkan"
		if !enabled {
			status = "dinonaktifkan"
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Skill Diperbarui:** Skill `%s` telah **%s** untuk agent `%s`.", action.SkillID, status, action.AgentID)

	default:
		return cleanText
	}
}
