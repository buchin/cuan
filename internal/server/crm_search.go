package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type CRMSearchNote struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
}

func crmMatchRank(c CRMContact, query string) int {
	if strings.Contains(strings.ToLower(c.Name+" "+c.Email), query) {
		return 0
	}
	if strings.Contains(strings.ToLower(c.Product+" "+strings.Join(c.Tags, " ")), query) {
		return 1
	}
	return 2
}
func memorySnippet(body, query string) string {
	// Walk runes so truncation never splits UTF-8, including emoji names/notes.
	runes := []rune(body)
	lower := strings.ToLower(body)
	index := strings.Index(lower, query)
	start := 0
	if index > 0 {
		start = utf8.RuneCountInString(lower[:index]) - 60
		if start < 0 {
			start = 0
		}
	}
	end := start + 180
	if end > len(runes) {
		end = len(runes)
	}
	text := string(runes[start:end])
	if start > 0 {
		text = "…" + text
	}
	if end < len(runes) {
		text += "…"
	}
	return text
}
func (s *Server) crmSearch(ctx context.Context, query, tag, source string, offset int) ([]CRMContact, error) {
	query = strings.ToLower(query)
	out := []CRMContact{}
	if s.db != nil {
		columns := "c." + strings.ReplaceAll(crmColumns, ",", ",c.")
		rows, err := s.db.QueryContext(ctx, `SELECT `+columns+`,COALESCE(m.id::text,''),COALESCE(m.snippet,'') FROM crm_contacts c
    LEFT JOIN LATERAL (
      SELECT id,
      CASE WHEN strpos(lower(body),$1)>60 THEN '…' ELSE '' END || substring(body FROM greatest(strpos(lower(body),$1)-60,1) FOR 180) || CASE WHEN length(body)>greatest(strpos(lower(body),$1)-60,1)+179 THEN '…' ELSE '' END AS snippet
      FROM contact_memory WHERE contact_id=c.id AND deleted_at IS NULL AND strpos(lower(body),$1)>0
      ORDER BY created_at DESC,id DESC LIMIT 1
    ) m ON true
    WHERE (strpos(lower(c.name||' '||c.email||' '||c.product||' '||c.tags::text),$1)>0 OR m.id IS NOT NULL)
    AND ($3='' OR c.tags @> to_jsonb(ARRAY[$3]::text[]))
    AND ($4='' OR COALESCE(NULLIF(btrim(c.product),''),'Live chat')=$4)
    ORDER BY CASE WHEN strpos(lower(c.name||' '||c.email),$1)>0 THEN 0 WHEN strpos(lower(c.product||' '||c.tags::text),$1)>0 THEN 1 ELSE 2 END,lower(c.name),c.id
    LIMIT 51 OFFSET $2`, query, offset, tag, source)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var c CRMContact
			var tags []byte
			var id, snippet string
			err = rows.Scan(&c.ID, &c.Product, &c.GuestUUID, &c.SessionUUID, &c.Name, &c.Email, &tags, &c.CreatedAt, &c.UpdatedAt, &id, &snippet)
			if err != nil {
				return nil, err
			}
			if err = json.Unmarshal(tags, &c.Tags); err != nil {
				return nil, err
			}
			c.IsManual = strings.HasPrefix(c.GuestUUID, "manual_")
			if id != "" {
				c.SearchNote = &CRMSearchNote{ID: id, Snippet: snippet}
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	for _, c := range s.crm.contacts {
		if source != "" {
			value := strings.TrimSpace(c.Product)
			if value == "" {
				value = "Live chat"
			}
			if value != source {
				continue
			}
		}
		if tag != "" {
			found := false
			for _, value := range c.Tags {
				if value == tag {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		var matched *ContactMemory
		for _, m := range s.crm.memory {
			if m.ContactID == c.ID && m.DeletedAt == nil && strings.Contains(strings.ToLower(m.Body), query) && (matched == nil || m.CreatedAt.After(matched.CreatedAt) || (m.CreatedAt.Equal(matched.CreatedAt) && m.ID > matched.ID)) {
				copy := m
				matched = &copy
			}
		}
		if crmMatchRank(c, query) == 2 && matched == nil {
			continue
		}
		if matched != nil {
			c.SearchNote = &CRMSearchNote{ID: matched.ID, Snippet: memorySnippet(matched.Body, query)}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := crmMatchRank(out[i], query), crmMatchRank(out[j], query)
		if a != b {
			return a < b
		}
		aName, bName := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if aName != bName {
			return aName < bName
		}
		return out[i].ID < out[j].ID
	})
	if offset >= len(out) {
		return []CRMContact{}, nil
	}
	out = out[offset:]
	if len(out) > 51 {
		out = out[:51]
	}
	return out, nil
}
func (s *Server) handleCRMSearch(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tag, source := r.URL.Query().Get("tag"), r.URL.Query().Get("source")
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if r.URL.Query().Get("offset") == "" {
		offset = 0
		err = nil
	}
	if err != nil || offset < 0 || offset > 100000 || q == "" || utf8.RuneCountInString(q) > 200 || utf8.RuneCountInString(tag) > 40 || len(source) > 300 {
		respondJSONError(w, 400, "Use a search of 1–200 characters")
		return
	}
	contacts, err := s.crmSearch(r.Context(), q, tag, source, offset)
	if err != nil {
		respondJSONError(w, 503, "Could not search contacts and notes. Try again.")
		return
	}
	more := len(contacts) > 50
	if more {
		contacts = contacts[:50]
	}
	respondJSON(w, 200, map[string]any{"contacts": contacts, "has_more": more})
}
