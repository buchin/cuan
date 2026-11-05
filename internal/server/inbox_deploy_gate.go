package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/buchin/cuan/internal/db"
	"golang.org/x/sys/unix"
)

// The release activator holds the exclusive side while waiting for running
// requests and restarting. Keep the shared side only for the durable claim;
// active conversations may finish while new requests remain safely queued.
func (s *Server) claimInboxForDispatch(ctx context.Context, store db.InboxLifecycleRepository, room, request string) (db.InboxRequest, bool, error) {
	s.inboxAdmission.RLock()
	defer s.inboxAdmission.RUnlock()
	if !s.executesInbox() || s.inboxStopping.Load() {
		return db.InboxRequest{}, false, nil
	}
	if s.cfg == nil || s.cfg.WorkspacesDir == "" {
		return store.ClaimInboxRequest(ctx, room, request)
	}
	directory := filepath.Dir(filepath.Clean(s.cfg.WorkspacesDir))
	if err := os.MkdirAll(directory, 0700); err != nil {
		return db.InboxRequest{}, false, err
	}
	file, err := os.OpenFile(filepath.Join(directory, "inbox-admission.lock"), os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0660)
	if err != nil {
		return db.InboxRequest{}, false, err
	}
	defer file.Close()
	if err = unix.Flock(int(file.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		return db.InboxRequest{}, false, err
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return store.ClaimInboxRequest(ctx, room, request)
}
