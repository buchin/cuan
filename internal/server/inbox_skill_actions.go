package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/buchin/cuan/internal/db"
)

type inboxSkillAction struct {
	Action  string `json:"action"` // "toggle"
	AgentID string `json:"agent_id"`
	SkillID string `json:"skill_id"`
	Enabled bool   `json:"enabled"`
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
	if strings.TrimSpace(a.AgentID) == "" {
		return "", nil, errors.New("Agent ID is required.")
	}
	if strings.TrimSpace(a.SkillID) == "" {
		return "", nil, errors.New("Skill ID is required.")
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
	case "toggle":
		if err := s.mcpRegistry.ToggleAgentSkill(action.AgentID, action.SkillID, action.Enabled); err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Failed to update skill %s: %v", action.SkillID, err)
		}
		status := "diaktifkan"
		if !action.Enabled {
			status = "dinonaktifkan"
		}
		return cleanText + fmt.Sprintf("\n\n✓ **Skill Diperbarui:** Skill `%s` telah **%s** untuk agent `%s`.", action.SkillID, status, action.AgentID)
	default:
		return cleanText
	}
}
