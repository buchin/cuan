package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/buchin/cuan/internal/diagnostics"
)

// Create an isolated, immutable snapshot per execution, not per AI profile.
// Concurrent rooms sharing a brain cannot replace or inherit each other's data.
func (s *Server) prepareInboxCRMSnapshot(ctx context.Context) (string, func(), error) {
	noop := func() {}
	trace, ok := ctx.Value(inboxRequestTraceKey{}).(inboxRequestTrace)
	owner, _ := ctx.Value(tasteMemberKey{}).(string)
	if !ok || owner == "" || trace.RoomID == "" || trace.RequestID == "" {
		return "", noop, nil
	}
	contacts, err := s.crmList(ctx)
	if err != nil {
		return "", noop, err
	}
	snapshot := diagnostics.InboxCRMSnapshot{OwnerID: owner, RoomID: trace.RoomID, RequestID: trace.RequestID, ObservedAt: time.Now().UTC(), Contacts: []diagnostics.ContactSummary{}, Memories: map[string]json.RawMessage{}, MemoryHasMore: map[string]bool{}}
	// The active contact must survive directory truncation.
	ordered := []CRMContact{}
	for _, c := range contacts {
		if c.SessionUUID == trace.RoomID {
			snapshot.CurrentContactID = c.ID
			ordered = append(ordered, c)
		}
	}
	for _, c := range contacts {
		if c.ID != snapshot.CurrentContactID {
			ordered = append(ordered, c)
		}
	}
	for i, c := range ordered {
		if i >= 200 {
			snapshot.Truncated = true
			break
		}
		snapshot.Contacts = append(snapshot.Contacts, diagnostics.ContactSummary{ID: c.ID, SessionUUID: c.SessionUUID, GuestUUID: c.GuestUUID, Name: c.Name, Email: c.Email, Product: c.Product, Tags: c.Tags, CreatedAt: c.CreatedAt.Format(time.RFC3339), UpdatedAt: c.UpdatedAt.Format(time.RFC3339)})
		if c.ID == snapshot.CurrentContactID {
			notes, e := s.memoryList(ctx, c.ID, "", 0)
			if e != nil {
				return "", noop, e
			}
			// The native 51st row is the existing pagination sentinel.
			snapshot.MemoryHasMore[c.ID] = len(notes) > 50
			if len(notes) > 50 {
				notes = notes[:50]
			}
			raw, e := json.Marshal(notes)
			if e != nil {
				return "", noop, e
			}
			snapshot.Memories[c.ID] = raw
		}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return "", noop, err
	}
	if len(raw) > 1<<20 {
		return "", noop, fmt.Errorf("CRM snapshot exceeds the bounded read-only context limit")
	}
	directory, err := os.MkdirTemp("", "cuan-inbox-crm-")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { os.RemoveAll(directory) }
	path := filepath.Join(directory, "snapshot.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		cleanup()
		return "", noop, err
	}
	return path, cleanup, nil
}
