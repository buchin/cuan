package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"sort"
	"strings"
	"unicode/utf8"
)

// BroadcastRecipient represents a single resolved recipient for email broadcasting.
type BroadcastRecipient struct {
	Email     string   `json:"email"`
	Name      string   `json:"name,omitempty"`
	Source    string   `json:"source"` // "crm", "custom", or "both"
	ContactID string   `json:"contact_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// BroadcastAudienceRequest is the payload sent to resolve the broadcast recipient audience.
type BroadcastAudienceRequest struct {
	Tags         []string `json:"tags"`
	CustomEmails any      `json:"custom_emails"`
}

// BroadcastAudienceResponse is the result of resolving the broadcast audience.
type BroadcastAudienceResponse struct {
	TotalUnique int                  `json:"total_unique"`
	CRMCount    int                  `json:"crm_count"`
	CustomCount int                  `json:"custom_count"`
	Recipients  []BroadcastRecipient `json:"recipients"`
}

// BroadcastTagSummary represents a CRM tag and the number of email-ready contacts having that tag.
type BroadcastTagSummary struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func normalizeTagKey(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	return strings.ToLower(tag)
}

func parseAndValidateEmail(raw string) (email string, name string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("Email address cannot be empty")
	}
	if len(raw) > 254 {
		return "", "", errors.New("Email address exceeds maximum length of 254 characters")
	}

	addr, parseErr := mail.ParseAddress(raw)
	if parseErr != nil {
		return "", "", fmt.Errorf("Invalid email address: %s", raw)
	}

	email = strings.ToLower(strings.TrimSpace(addr.Address))
	name = strings.TrimSpace(addr.Name)

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("Invalid email address: %s", raw)
	}
	if !strings.Contains(parts[1], ".") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".") {
		return "", "", fmt.Errorf("Invalid email domain: %s", raw)
	}
	return email, name, nil
}

func extractCustomEmailStrings(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		fields := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r'
		})
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			s := strings.TrimSpace(f)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			fields := strings.FieldsFunc(item, func(r rune) bool {
				return r == ',' || r == ';' || r == '\n' || r == '\r'
			})
			for _, f := range fields {
				s := strings.TrimSpace(f)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				fields := strings.FieldsFunc(str, func(r rune) bool {
					return r == ',' || r == ';' || r == '\n' || r == '\r'
				})
				for _, f := range fields {
					s := strings.TrimSpace(f)
					if s != "" {
						out = append(out, s)
					}
				}
			} else {
				return nil, errors.New("Each custom email must be a string")
			}
		}
		return out, nil
	default:
		return nil, errors.New("Invalid format for custom_emails: must be a string or array of strings")
	}
}

// handleBroadcastTags returns all tags in CRM with the count of contacts that have a valid email.
func (s *Server) handleBroadcastTags(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	contacts, err := s.crmList(r.Context())
	if err != nil {
		respondJSONError(w, 503, "Could not load contacts")
		return
	}

	tagCounts := make(map[string]int)
	for _, c := range contacts {
		if strings.TrimSpace(c.Email) == "" {
			continue
		}
		seenInContact := make(map[string]bool)
		for _, t := range c.Tags {
			clean := strings.TrimSpace(t)
			clean = strings.TrimPrefix(clean, "#")
			k := strings.ToLower(clean)
			if k != "" && !seenInContact[k] {
				seenInContact[k] = true
				tagCounts[clean]++
			}
		}
	}

	var tags []BroadcastTagSummary
	for tag, count := range tagCounts {
		tags = append(tags, BroadcastTagSummary{Tag: tag, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return strings.ToLower(tags[i].Tag) < strings.ToLower(tags[j].Tag)
	})

	respondJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// handleBroadcastAudience resolves unique email recipients from CRM tags and custom emails with strict deduplication.
func (s *Server) handleBroadcastAudience(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}

	var req BroadcastAudienceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, 400, "Invalid JSON payload for audience resolution")
		return
	}

	if len(req.Tags) > 50 {
		respondJSONError(w, 400, "Up to 50 tags are allowed")
		return
	}

	selectedTagMap := make(map[string]bool)
	for _, t := range req.Tags {
		t = strings.TrimSpace(t)
		if len(t) == 0 || utf8.RuneCountInString(t) > 40 {
			respondJSONError(w, 400, "Tags must contain 1–40 characters")
			return
		}
		k := normalizeTagKey(t)
		if k != "" {
			selectedTagMap[k] = true
		}
	}

	customList, err := extractCustomEmailStrings(req.CustomEmails)
	if err != nil {
		respondJSONError(w, 400, err.Error())
		return
	}
	if len(customList) > 500 {
		respondJSONError(w, 400, "Up to 500 custom emails are allowed")
		return
	}

	recipientMap := make(map[string]*BroadcastRecipient)
	var order []string

	// 1. Resolve CRM contacts matching selected tags
	if len(selectedTagMap) > 0 {
		contacts, err := s.crmList(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Could not load contacts")
			return
		}

		for _, c := range contacts {
			if strings.TrimSpace(c.Email) == "" {
				continue
			}
			cEmail, _, parseErr := parseAndValidateEmail(c.Email)
			if parseErr != nil {
				continue
			}

			var matchingTags []string
			for _, ct := range c.Tags {
				if selectedTagMap[normalizeTagKey(ct)] {
					matchingTags = append(matchingTags, ct)
				}
			}
			if len(matchingTags) == 0 {
				continue
			}

			if existing, ok := recipientMap[cEmail]; ok {
				// Contact already present (e.g. duplicate email in CRM); merge tags
				for _, mt := range matchingTags {
					found := false
					for _, et := range existing.Tags {
						if strings.EqualFold(mt, et) {
							found = true
							break
						}
					}
					if !found {
						existing.Tags = append(existing.Tags, mt)
					}
				}
				if existing.ContactID == "" {
					existing.ContactID = c.ID
				}
			} else {
				displayName := strings.TrimSpace(c.Name)
				if displayName == "" {
					displayName = cEmail
				}
				recipientMap[cEmail] = &BroadcastRecipient{
					Email:     cEmail,
					Name:      displayName,
					Source:    "crm",
					ContactID: c.ID,
					Tags:      matchingTags,
				}
				order = append(order, cEmail)
			}
		}
	}

	// 2. Resolve custom emails with validation & deduplication
	for _, rawEmail := range customList {
		email, name, valErr := parseAndValidateEmail(rawEmail)
		if valErr != nil {
			respondJSONError(w, 400, valErr.Error())
			return
		}

		if existing, ok := recipientMap[email]; ok {
			// Deduplicated with existing entry!
			if existing.Source == "crm" {
				existing.Source = "both"
			}
			if (existing.Name == "" || existing.Name == email) && name != "" {
				existing.Name = name
			}
		} else {
			displayName := name
			if displayName == "" {
				displayName = email
			}
			recipientMap[email] = &BroadcastRecipient{
				Email:  email,
				Name:   displayName,
				Source: "custom",
			}
			order = append(order, email)
		}
	}

	recipients := make([]BroadcastRecipient, 0, len(order))
	crmCount := 0
	customCount := 0

	for _, emailKey := range order {
		rec := *recipientMap[emailKey]
		if rec.Source == "crm" || rec.Source == "both" {
			crmCount++
		}
		if rec.Source == "custom" || rec.Source == "both" {
			customCount++
		}
		recipients = append(recipients, rec)
	}

	// Sort alphabetically by name then email for a serene, predictable list
	sort.Slice(recipients, func(i, j int) bool {
		nameI := strings.ToLower(recipients[i].Name)
		nameJ := strings.ToLower(recipients[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		return recipients[i].Email < recipients[j].Email
	})

	resp := BroadcastAudienceResponse{
		TotalUnique: len(recipients),
		CRMCount:    crmCount,
		CustomCount: customCount,
		Recipients:  recipients,
	}

	respondJSON(w, http.StatusOK, resp)
}
