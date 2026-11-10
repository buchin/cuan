package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/go-chi/chi/v5"
)

// Existing agent aliases retain their legacy room; new topic UUIDs never resolve
// back to that room. Topic association uses existing durable room metadata.
func roomAgentID(topic string) string {
	if strings.HasPrefix(topic, "agent:") {
		return strings.TrimPrefix(topic, "agent:")
	}
	if strings.HasPrefix(topic, "agent-topic:") {
		return strings.TrimPrefix(topic, "agent-topic:")
	}
	return ""
}

func (s *Server) handleCreateAgentTopic(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	var req struct {
		AgentID   string `json:"agent_id"`
		RequestID string `json:"request_id"`
		ProjectID string `json:"project_id,omitempty"`
		Title     string `json:"title,omitempty"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || !db.IsValidUUID(req.RequestID) {
		respondJSONError(w, 400, "A valid conversation request ID is required")
		return
	}
	if s.agentRepo == nil || s.projectStudioRepo == nil {
		respondJSONError(w, 503, "Conversations are unavailable")
		return
	}
	// Canonicalize new conversation recipients while preserving existing rooms.
	if existing, _ := s.projectStudioRepo.GetRoomByID(r.Context(), req.RequestID); existing == nil {
		if cuan, e := s.agentRepo.GetAgentByID(r.Context(), db.AgentIDCuan); e == nil && cuan != nil {
			req.AgentID = cuan.ID
		}
	}
	a, err := s.agentRepo.GetAgentByID(r.Context(), req.AgentID)
	if err != nil || a == nil || !a.Enabled {
		respondJSONError(w, 404, "Agent is unavailable")
		return
	}
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()
	s.ensureDefaultAgentRooms(r.Context())
	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), req.RequestID)
	if err == nil && room != nil {
		if room.Topic != "agent-topic:"+a.ID {
			respondJSONError(w, 409, "Request ID belongs to another conversation")
			return
		}
	} else {
		now := time.Now().UTC()
		title := strings.TrimSpace(req.Title)
		if len([]rune(title)) > 200 {
			respondJSONError(w, 400, "Use a conversation title up to 200 characters")
			return
		}
		if title == "" {
			title = "New conversation"
		}
		projectID := "default"
		if s.projectSvc != nil {
			if req.ProjectID != "" {
				if p, err := s.projectSvc.Get(r.Context(), req.ProjectID); err == nil && p != nil && !p.IsArchived {
					projectID = p.ID
				}
			} else {
				// If no project specified, check if "default" is active.
				// If unavailable, use only a sole active project. Ambiguous
				// conversations remain unbound for Inbox discovery.
				if p, err := s.projectSvc.Get(r.Context(), "default"); err != nil || p == nil || p.IsArchived {
					if projects, err := s.projectSvc.List(r.Context()); err == nil {
						candidate := ""
						for _, p := range projects {
							if !p.IsArchived && !strings.EqualFold(p.ID, "default") {
								if candidate != "" {
									candidate = ""
									break
								}
								candidate = p.ID
							}
						}
						if candidate != "" {
							projectID = candidate
						}
					}
				}
			}
		}
		room = &db.ProjectRoom{ID: req.RequestID, ProjectID: projectID, Title: title, Topic: "agent-topic:" + a.ID, CreatedAt: now, UpdatedAt: now}
		if err = s.projectStudioRepo.CreateRoom(r.Context(), room); err != nil {
			respondJSONError(w, 503, "Could not save the conversation. Retry using the same request ID.")
			return
		}
		if strings.TrimSpace(req.Title) != "" && title != "New conversation" {
			s.markRoomTitleManual(owner, room.ID, title)
		}
	}
	respondJSON(w, 200, ChatContact{ID: "topic_" + room.ID, AgentID: a.ID, AgentName: a.Name, Type: "agent", Name: room.Title, Avatar: a.Avatar, Status: "idle", ConversationID: room.ID, UpdatedAt: room.UpdatedAt})
}

func (s *Server) nameAgentTopic(ctx context.Context, roomID, text string) {
	text, ids := decodeAttachmentMessage(text)
	if text == "" && len(ids) > 0 {
		text = "Shared files"
	}
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()
	room, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if err != nil || room == nil || !strings.HasPrefix(room.Topic, "agent-topic:") || room.Title != "New conversation" {
		return
	}
	title := []rune(strings.Join(strings.Fields(text), " "))
	if len(title) > 80 {
		title = append(title[:79], '…')
	}
	if len(title) == 0 {
		return
	}
	room.Title = string(title)
	room.UpdatedAt = time.Now().UTC()
	_ = s.projectStudioRepo.UpdateRoom(ctx, room)
}

func (s *Server) handleUpdateConversation(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	if s.projectStudioRepo == nil {
		respondJSONError(w, 503, "Storage is unavailable")
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	var req struct {
		Title string `json:"title"`
		Name  string `json:"name"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil {
		respondJSONError(w, 400, "Invalid title update request")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(req.Name)
	}
	if title == "" || len([]rune(title)) > 200 {
		respondJSONError(w, 400, "Enter a conversation title of up to 200 characters")
		return
	}
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()
	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), resolved.RoomID)
	if err != nil || room == nil {
		respondJSONError(w, 404, "Conversation not found")
		return
	}
	room.Title = title
	room.UpdatedAt = time.Now().UTC()
	if err := s.projectStudioRepo.UpdateRoom(r.Context(), room); err != nil {
		respondJSONError(w, 500, "Could not update conversation title: "+err.Error())
		return
	}
	s.markRoomTitleManual(owner, room.ID, title)
	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(room.ID, "room.updated", map[string]any{
			"id":    room.ID,
			"title": title,
			"topic": room.Topic,
		})
	}
	s.publishInbox(room.ID)
	respondJSON(w, 200, map[string]any{"id": room.ID, "title": title})
}
