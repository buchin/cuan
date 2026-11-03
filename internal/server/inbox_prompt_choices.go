package server

import (
	"encoding/json"
	"fmt"
	"strings"
)

// InboxPromptOption defines an individual choice button in a prompt card.
type InboxPromptOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Variant string `json:"variant,omitempty"` // "primary" | "outline" | "ghost"
}

// InboxPromptChoices represents interactive discrete options presented to the user.
type InboxPromptChoices struct {
	Question   string              `json:"question,omitempty"`
	Options    []InboxPromptOption `json:"options"`
	AllowOther bool                `json:"allow_other,omitempty"`
}

func splitInboxPromptChoices(author, content string) (string, *InboxPromptChoices) {
	if author != "agent" {
		return content, nil
	}
	const marker = "```cuan-prompt\n"
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.LastIndex(trimmed, marker)
	if start < 0 || !strings.HasSuffix(trimmed, "```") || strings.Count(trimmed, marker) != 1 {
		return content, nil
	}
	raw := strings.TrimSpace(trimmed[start+len(marker) : len(trimmed)-3])
	if len(raw) > 32000 {
		return content, nil
	}
	var p InboxPromptChoices
	if json.Unmarshal([]byte(raw), &p) != nil {
		return content, nil
	}
	if len(p.Options) == 0 || len(p.Options) > 8 {
		return content, nil
	}
	for i := range p.Options {
		p.Options[i].Label = strings.TrimSpace(p.Options[i].Label)
		if p.Options[i].Label == "" || len([]rune(p.Options[i].Label)) > 80 {
			return content, nil
		}
		if p.Options[i].ID == "" {
			p.Options[i].ID = fmt.Sprintf("opt_%d", i+1)
		}
	}
	return strings.TrimSpace(trimmed[:start]), &p
}
