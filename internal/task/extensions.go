package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	sdk "github.com/buchin/cuan/pkg/extensions"
	"log"
	"time"
)

// Bootstrap-only; listeners run after persistence and after mutation locks are
// released. Enqueue substantial follow-up work; avoid recursive event loops.
func (s *Service) SetExtensionRegistry(registry *sdk.Registry) { s.extensions = registry }
func (s *Service) extensionEvent(ctx context.Context, kind string, t *Task) {
	if s.extensions == nil || t == nil {
		return
	}
	sum := sha256.Sum256([]byte(kind + ":" + t.ID + ":" + t.UpdatedAt.Format(time.RFC3339Nano)))
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 500*time.Millisecond)
	defer cancel()
	event := sdk.Event{ID: hex.EncodeToString(sum[:]), Type: kind, Version: 1, Scope: sdk.Scope{ProjectID: t.ProjectID, ResourceID: t.ID}, At: t.UpdatedAt}
	if failures := s.extensions.Publish(bounded, event); len(failures) > 0 {
		log.Printf("extension task event delivery incomplete type=%s failures=%d", kind, len(failures))
	}
}
func (s *Service) extensionTransition(ctx context.Context, old string, t *Task) {
	if s.extensions == nil || t == nil || old == t.Status {
		return
	}
	s.extensionEvent(ctx, "task.status_changed", t)
	if t.Status == TaskStatusDone {
		s.extensionEvent(ctx, "task.completed", t)
	} else if t.Status == TaskStatusBlocked {
		s.extensionEvent(ctx, "task.blocked", t)
	}
}
