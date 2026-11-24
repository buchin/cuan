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

type inboxBrandMemoryAction struct {
	Action   string `json:"action"` // "list", "add", "update", "delete", "reset"
	Category string `json:"category,omitempty"` // "brand" or "technical"
	Topic    string `json:"topic,omitempty"`
	Content  string `json:"content,omitempty"`
	ID       string `json:"id,omitempty"`
}

func parseInboxBrandMemory(content string) (string, *inboxBrandMemoryAction, error) {
	const marker = "```cuan-brand-memory\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid brand memory action.")
	if strings.Count(normalized, marker) != 1 || !strings.HasSuffix(normalized, "```") {
		return "", nil, bad
	}
	raw := normalized[start+len(marker) : len(normalized)-3]
	var a inboxBrandMemoryAction
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
		a.Action = "list"
	}
	switch a.Action {
	case "add":
		if strings.TrimSpace(a.Content) == "" {
			return "", nil, errors.New("Content is required to add brand memory item.")
		}
	case "update":
		if strings.TrimSpace(a.ID) == "" {
			return "", nil, errors.New("ID is required to update brand memory item.")
		}
		if strings.TrimSpace(a.Content) == "" && strings.TrimSpace(a.Topic) == "" && strings.TrimSpace(a.Category) == "" {
			return "", nil, errors.New("At least one field (topic, content, category) is required to update.")
		}
	case "delete":
		if strings.TrimSpace(a.ID) == "" {
			return "", nil, errors.New("ID is required to delete brand memory item.")
		}
	case "list", "reset":
		// No specific fields required
	default:
		return "", nil, errors.New("Unsupported brand memory action.")
	}
	return strings.TrimSpace(normalized[:start]), &a, nil
}

func (s *Server) applyInboxBrandMemory(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, action, err := parseInboxBrandMemory(content)
	if err != nil {
		return err.Error()
	}
	if action == nil {
		return content
	}

	owner := strings.TrimSpace(q.OwnerID)
	if owner == "" {
		owner = "default"
	}

	switch action.Action {
	case "list":
		profile, err := s.getTasteProfile(owner)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memuat profil brand memory: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString("### Brand Guidelines & Tone of Voice\n")
		if len(profile.BrandItems) == 0 {
			sb.WriteString("_Belum ada pedoman brand atau gaya bahasa yang dicatat._\n")
		} else {
			for _, it := range profile.BrandItems {
				topic := it.Topic
				if topic == "" {
					topic = "Pedoman Brand"
				}
				sb.WriteString(fmt.Sprintf("• **%s** (`%s`):\n  %s\n", topic, it.ID, it.Content))
			}
		}

		sb.WriteString("\n### Standar Teknis & Batasan Operasional\n")
		if len(profile.TechnicalItems) == 0 {
			sb.WriteString("_Belum ada standar teknis khusus yang dicatat._\n")
		} else {
			for _, it := range profile.TechnicalItems {
				topic := it.Topic
				if topic == "" {
					topic = "Standar Teknis"
				}
				sb.WriteString(fmt.Sprintf("• **%s** (`%s`):\n  %s\n", topic, it.ID, it.Content))
			}
		}

		return strings.TrimSpace(sb.String())

	case "add":
		category := strings.ToLower(strings.TrimSpace(action.Category))
		if category == "" || (category != "technical" && category != "brand") {
			category = "brand"
		}
		topic := strings.TrimSpace(action.Topic)
		if topic == "" {
			if category == "brand" {
				topic = "Brand Guideline"
			} else {
				topic = "Technical Standard"
			}
		}
		itemContent := strings.TrimSpace(action.Content)

		created, err := s.createTasteItem(owner, category, topic, itemContent)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal menambahkan memori brand: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✓ Pedoman berhasil disimpan ke memori **%s**.\n", strings.Title(category)))
		sb.WriteString(fmt.Sprintf("• **Topik:** %s (`%s`)\n", created.Topic, created.ID))
		sb.WriteString(fmt.Sprintf("• **Pedoman:** %s", created.Content))
		return strings.TrimSpace(sb.String())

	case "update":
		var catPtr, topicPtr, contentPtr *string
		if action.Category != "" {
			normCat, err := normalizeCategory(action.Category)
			if err == nil {
				catPtr = &normCat
			}
		}
		if action.Topic != "" {
			t := strings.TrimSpace(action.Topic)
			topicPtr = &t
		}
		if action.Content != "" {
			c := strings.TrimSpace(action.Content)
			contentPtr = &c
		}

		updated, err := s.updateTasteItem(owner, strings.TrimSpace(action.ID), catPtr, topicPtr, contentPtr)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal memperbarui memori: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✓ Pedoman (`%s`) berhasil diperbarui:\n", updated.ID))
		sb.WriteString(fmt.Sprintf("• **Topik:** %s\n", updated.Topic))
		sb.WriteString(fmt.Sprintf("• **Isi:** %s", updated.Content))
		return strings.TrimSpace(sb.String())

	case "delete":
		err := s.deleteTasteItem(owner, strings.TrimSpace(action.ID))
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal menghapus pedoman: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("✓ Pedoman (`%s`) berhasil dihapus dari memori.", action.ID))
		return strings.TrimSpace(sb.String())

	case "reset":
		_, err := s.resetTasteProfile(owner)
		if err != nil {
			return cleanText + fmt.Sprintf("\n\n⚠️ Gagal mereset memori brand: %v", err)
		}

		var sb strings.Builder
		if cleanText != "" {
			sb.WriteString(cleanText)
			sb.WriteString("\n\n")
		}
		sb.WriteString("✓ Profil memori brand telah direset ke standar bawaan Cuan.")
		return strings.TrimSpace(sb.String())

	default:
		return cleanText + fmt.Sprintf("\n\n⚠️ Aksi brand memory '%s' tidak didukung.", action.Action)
	}
}
