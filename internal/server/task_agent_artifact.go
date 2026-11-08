package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/task"
)

var taskAttachmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)

func (s *Server) handleTaskAgentArtifact(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	digest := sha256.Sum256([]byte(raw))
	value, ok := s.taskAgentGrants.Load(digest)
	if !ok && raw != "" {
		if restored, err := s.loadTaskGrant(digest); err == nil {
			value, ok = restored, true
		}
	}

	if !ok || raw == "" {
		respondJSONError(w, http.StatusUnauthorized, "Task session access is unavailable")
		return
	}
	g := value.(taskAgentGrant)
	if time.Now().After(g.Expires) {
		s.forgetTaskGrant(digest)
		respondJSONError(w, http.StatusUnauthorized, "Task session access expired")
		return
	}
	if chi.URLParam(r, "id") != g.TaskID {
		respondJSONError(w, http.StatusForbidden, "This session can access only its bound task")
		return
	}
	latest, e := s.taskSvc.GetTaskDetail(r.Context(), g.TaskID)
	if e != nil || latest.LatestAttempt == nil || latest.LatestAttempt.ID != g.AttemptID || latest.LatestAttempt.Status != task.AttemptStatusRunning {
		respondJSONError(w, http.StatusConflict, "This task attempt is no longer active")
		return
	}

	attachmentID := chi.URLParam(r, "attachment_id")
	if !taskAttachmentIDPattern.MatchString(attachmentID) {
		respondJSONError(w, http.StatusBadRequest, "Invalid attachment ID")
		return
	}

	a, path, err := s.taskDeliveredAttachment(r.Context(), latest, attachmentID)
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Attachment not found or not delivered by this task")
		return
	}

	if r.URL.Query().Get("metadata") == "true" || r.URL.Query().Get("metadata") == "1" {
		respondJSON(w, http.StatusOK, map[string]any{
			"attachment_id": a.ID,
			"name":          a.Name,
			"media_type":    a.Type,
			"size":          a.Size,
		})
		return
	}

	w.Header().Set("Content-Type", a.Type)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(a.Name)}))
	http.ServeFile(w, r, path)
}

func (s *Server) taskDeliveredAttachment(ctx context.Context, d *task.TaskDetailResponse, attachmentID string) (conversationAttachment, string, error) {
	var empty conversationAttachment
	if d == nil || s.projectStudioRepo == nil || d.SourceRoomID == "" || d.SourceMessageID == "" {
		return empty, "", errors.New("task conversation is unavailable")
	}
	if !taskAttachmentIDPattern.MatchString(attachmentID) {
		return empty, "", errors.New("invalid attachment ID")
	}
	owner, err := s.taskArtifactOwner(ctx, d)
	if err != nil || owner == "" {
		return empty, "", errors.New("task conversation owner is unavailable")
	}

	attempts := append([]*task.TaskAttempt(nil), d.Attempts...)
	if d.LatestAttempt != nil {
		hasLatest := false
		for _, a := range attempts {
			if a != nil && a.ID == d.LatestAttempt.ID {
				hasLatest = true
				break
			}
		}
		if !hasLatest {
			attempts = append(attempts, d.LatestAttempt)
		}
	}

	var found bool
	for _, attempt := range attempts {
		if attempt == nil || attempt.TaskID != d.ID {
			continue
		}
		for _, prefix := range []string{"task-delivery:", "task-visual-delivery:"} {
			msgID := delegationMessageID(prefix + attempt.ID)
			message, err := s.projectStudioRepo.GetMessageByID(ctx, msgID)
			if err != nil || message == nil {
				continue
			}
			if message.RoomID != d.SourceRoomID || message.AuthorType != "agent" || message.ReplyToID == nil || *message.ReplyToID != d.SourceMessageID {
				continue
			}
			_, ids := decodeAttachmentMessage(message.Content)
			for _, id := range ids {
				if id == attachmentID {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return empty, "", errors.New("attachment not found or not delivered by this task")
	}

	a, path, err := loadAttachment(owner, attachmentID)
	if err != nil {
		return empty, "", err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > attachmentLimit {
		return empty, "", errors.New("attachment unavailable or exceeds size limit")
	}
	return a, path, nil
}
