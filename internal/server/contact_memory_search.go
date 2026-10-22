package server

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

type ContactMemoryHit struct {
	ContactID string        `json:"contact_id"`
	Name      string        `json:"name"`
	Email     string        `json:"email"`
	URL       string        `json:"url"`
	Entry     ContactMemory `json:"entry"`
	Score     int           `json:"-"`
}

func memoryTerms(query string) []string {
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '@' && r != '.' && r != '-'
	})
	out := []string{}
	seen := map[string]bool{}
	for _, word := range words {
		if len([]rune(word)) >= 2 && !seen[word] {
			out = append(out, word)
			seen[word] = true
			if len(out) == 12 {
				break
			}
		}
	}
	return out
}
func (s *Server) searchContactMemory(ctx context.Context, contact, query string) ([]ContactMemoryHit, error) {
	terms := memoryTerms(query)
	out := []ContactMemoryHit{}
	if s.db != nil {
		encoded, _ := json.Marshal(terms)
		rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.name,c.email,m.id,m.contact_id,m.kind,m.body,m.recorded_by,m.source_room_id,m.source_message_id,m.created_at,m.updated_at,m.deleted_at,
    (SELECT count(*) FROM jsonb_array_elements_text($2::jsonb) t(word) WHERE strpos(lower(c.name||' '||c.email||' '||m.body),word)>0) AS score
    FROM contact_memory m JOIN crm_contacts c ON c.id=m.contact_id
    WHERE m.deleted_at IS NULL AND ($1='' OR c.id::text=$1)
    AND (jsonb_array_length($2::jsonb)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text($2::jsonb) t(word) WHERE strpos(lower(c.name||' '||c.email||' '||m.body),word)>0))
    ORDER BY score DESC,m.created_at DESC,m.id DESC LIMIT 41`, contact, string(encoded))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var h ContactMemoryHit
			m := &h.Entry
			err = rows.Scan(&h.ContactID, &h.Name, &h.Email, &m.ID, &m.ContactID, &m.Kind, &m.Body, &m.RecordedBy, &m.SourceRoomID, &m.SourceMessageID, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &h.Score)
			if err != nil {
				return nil, err
			}
			h.URL = "/contacts?contact=" + h.ContactID
			out = append(out, h)
		}
		return out, rows.Err()
	}
	s.crm.Lock()
	defer s.crm.Unlock()
	byID := map[string]CRMContact{}
	for _, c := range s.crm.contacts {
		byID[c.ID] = c
	}
	for _, m := range s.crm.memory {
		c, ok := byID[m.ContactID]
		if !ok || m.DeletedAt != nil || (contact != "" && contact != c.ID) {
			continue
		}
		haystack := strings.ToLower(c.Name + " " + c.Email + " " + m.Body)
		score := 0
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				score++
			}
		}
		if len(terms) > 0 && score == 0 {
			continue
		}
		out = append(out, ContactMemoryHit{ContactID: c.ID, Name: c.Name, Email: c.Email, URL: "/contacts?contact=" + c.ID, Entry: m, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Entry.CreatedAt.Equal(out[j].Entry.CreatedAt) {
			return out[i].Entry.ID > out[j].Entry.ID
		}
		return out[i].Entry.CreatedAt.After(out[j].Entry.CreatedAt)
	})
	if len(out) > 41 {
		out = out[:41]
	}
	return out, nil
}
