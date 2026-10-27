package server

import (
	"context"
	"strings"
	"time"
)

// PruneStaleAnonymousContacts removes empty anonymous visitor contacts and associated rooms
// that have no messages and were created more than maxAge ago.
func (s *Server) PruneStaleAnonymousContacts(ctx context.Context, maxAge time.Duration) (int64, error) {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	var deletedCount int64

	if s.db != nil {
		// 1. Delete stale crm_contacts without messages
		res, err := s.db.ExecContext(ctx, `
			DELETE FROM crm_contacts
			WHERE (name = '' OR name LIKE 'Visitor #%')
			  AND email = ''
			  AND created_at < $1
			  AND session_uuid NOT IN (SELECT room_id FROM project_messages)
		`, cutoff)
		if err != nil {
			return 0, err
		}
		deletedCount, _ = res.RowsAffected()

		// 2. Clean up orphaned empty project_rooms for visitors
		_, _ = s.db.ExecContext(ctx, `
			DELETE FROM project_rooms
			WHERE (title = '' OR title LIKE 'Visitor #%')
			  AND created_at < $1
			  AND id NOT IN (SELECT room_id FROM project_messages)
			  AND id NOT IN (SELECT session_uuid FROM crm_contacts)
		`, cutoff)
	}

	// In-memory cleanup
	s.crm.Lock()
	for key, c := range s.crm.contacts {
		if (c.Name == "" || strings.HasPrefix(c.Name, "Visitor #")) && c.Email == "" && c.CreatedAt.Before(cutoff) {
			if !s.hasMessages(c.SessionUUID) {
				delete(s.crm.contacts, key)
				if s.db == nil {
					deletedCount++
				}
			}
		}
	}
	s.crm.Unlock()

	// Clean stale transient sessions from globalChatManager
	globalChatManager.Lock()
	for id, sess := range globalChatManager.sessions {
		if sess != nil && sess.UpdatedAt.Before(cutoff) {
			delete(globalChatManager.sessions, id)
			delete(globalChatManager.sessionByGuest, sess.Product+"\x00"+sess.GuestUUID)
		}
	}
	globalChatManager.Unlock()

	return deletedCount, nil
}
