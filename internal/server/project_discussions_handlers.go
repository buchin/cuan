package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
)

// RoomEvent represents an event pushed over Server-Sent Events (SSE).
type RoomEvent struct {
	RoomID string      `json:"room_id"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
}

// RoomEventBroker manages SSE client subscriptions and event fan-out per project room.
type RoomEventBroker struct {
	mu                sync.RWMutex
	subscribers       map[string]map[chan RoomEvent]struct{}
	heartbeatInterval time.Duration
}

// NewRoomEventBroker initializes a new thread-safe RoomEventBroker.
func NewRoomEventBroker() *RoomEventBroker {
	return &RoomEventBroker{
		subscribers:       make(map[string]map[chan RoomEvent]struct{}),
		heartbeatInterval: 15 * time.Second,
	}
}

// SetHeartbeatInterval configures the heartbeat interval for SSE streams.
func (b *RoomEventBroker) SetHeartbeatInterval(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.heartbeatInterval = d
}

// HeartbeatInterval returns the configured heartbeat interval.
func (b *RoomEventBroker) HeartbeatInterval() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.heartbeatInterval <= 0 {
		return 15 * time.Second
	}
	return b.heartbeatInterval
}

// Subscribe registers a new subscriber channel for a specific room ID.
func (b *RoomEventBroker) Subscribe(roomID string) chan RoomEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan RoomEvent, 64)
	if b.subscribers[roomID] == nil {
		b.subscribers[roomID] = make(map[chan RoomEvent]struct{})
	}
	b.subscribers[roomID][ch] = struct{}{}
	return ch
}

// Unsubscribe removes and closes a subscriber channel for a specific room ID.
func (b *RoomEventBroker) Unsubscribe(roomID string, ch chan RoomEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subscribers[roomID]; ok {
		if _, exists := subs[ch]; exists {
			delete(subs, ch)
			close(ch)
		}
		if len(subs) == 0 {
			delete(b.subscribers, roomID)
		}
	}
}

// Broadcast dispatches an event to all active subscribers for the given room ID.
func (b *RoomEventBroker) Broadcast(roomID string, eventType string, data interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs, ok := b.subscribers[roomID]
	if !ok || len(subs) == 0 {
		return
	}

	event := RoomEvent{
		RoomID: roomID,
		Type:   eventType,
		Data:   data,
	}

	for ch := range subs {
		select {
		case ch <- event:
		default:
			// Non-blocking drop if channel buffer is full to prevent lagging clients from stalling the broker.
		}
	}
}

// SubscriberCount returns the current count of active SSE subscribers for a room ID.
func (b *RoomEventBroker) SubscriberCount(roomID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, ok := b.subscribers[roomID]; ok {
		return len(subs)
	}
	return 0
}

// CreateRoomPayload defines the JSON body for creating a discussion room.
type CreateRoomPayload struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	Topic string `json:"topic"`
}

// UpdateRoomPayload defines the JSON body for updating a discussion room.
type UpdateRoomPayload struct {
	Title string `json:"title"`
	Topic string `json:"topic"`
}

// CreateMessagePayload defines the JSON body for posting a message in a discussion room.
type CreateMessagePayload struct {
	ID         string  `json:"id,omitempty"`
	AuthorType string  `json:"author_type"` // 'user', 'agent', 'system'
	AuthorID   string  `json:"author_id"`
	AuthorName string  `json:"author_name"`
	Content    string  `json:"content"`
	ReplyToID  *string `json:"reply_to_id,omitempty"`
}

// DistillTaskPayload defines the JSON body for extracting tasks from room messages.
type DistillTaskPayload struct {
	MessageIDs []string `json:"message_ids,omitempty"`
	Title      string   `json:"title,omitempty"`
	BatchID    *string  `json:"batch_id,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// handleCreateProjectRoom creates a new discussion room bound to a project.
// POST /api/projects/{id}/rooms
func (s *Server) handleCreateProjectRoom(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var payload CreateRoomPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		respondJSONError(w, http.StatusBadRequest, "room title cannot be empty")
		return
	}

	room := db.ProjectRoom{
		ID:        payload.ID,
		ProjectID: projectID,
		Title:     title,
		Topic:     strings.TrimSpace(payload.Topic),
	}

	if err := s.projectStudioRepo.CreateRoom(r.Context(), &room); err != nil {
		if errors.Is(err, db.ErrDuplicateRoom) {
			respondJSONError(w, http.StatusConflict, "project room already exists")
			return
		}
		if errors.Is(err, db.ErrEmptyRoomTitle) || errors.Is(err, db.ErrEmptyProjectID) || errors.Is(err, db.ErrInvalidUUID) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create project room: "+err.Error())
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(room.ID, "room.created", room.ToDTO())
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"room": room.ToDTO(),
	})
}

// handleListProjectRooms retrieves all discussion rooms belonging to a project.
// GET /api/projects/{id}/rooms
func (s *Server) handleListProjectRooms(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	rooms, err := s.projectStudioRepo.ListRoomsByProjectID(r.Context(), projectID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list project rooms: "+err.Error())
		return
	}

	dtos := make([]db.ProjectRoomDTO, len(rooms))
	for i, rm := range rooms {
		dtos[i] = rm.ToDTO()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"rooms": dtos,
		"count": len(dtos),
	})
}

// handleGetProjectRoom retrieves a discussion room by its ID.
// GET /api/rooms/{id} or GET /api/projects/rooms/{id}
func (s *Server) handleGetProjectRoom(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project room: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"room": room.ToDTO(),
	})
}

// handleUpdateProjectRoom updates the title and topic of a discussion room.
// PUT /api/rooms/{id} or PUT /api/projects/rooms/{id}
func (s *Server) handleUpdateProjectRoom(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	var payload UpdateRoomPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		respondJSONError(w, http.StatusBadRequest, "room title cannot be empty")
		return
	}

	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project room: "+err.Error())
		return
	}

	room.Title = title
	room.Topic = strings.TrimSpace(payload.Topic)

	if err := s.projectStudioRepo.UpdateRoom(r.Context(), room); err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to update project room: "+err.Error())
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(room.ID, "room.updated", room.ToDTO())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"room": room.ToDTO(),
	})
}

// handleDeleteProjectRoom removes a discussion room by its ID.
// DELETE /api/rooms/{id} or DELETE /api/projects/rooms/{id}
func (s *Server) handleDeleteProjectRoom(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	if err := s.projectStudioRepo.DeleteRoom(r.Context(), roomID); err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete project room: "+err.Error())
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "room.deleted", map[string]string{
			"id": roomID,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
		"id":     roomID,
	})
}

// handleCreateProjectMessage posts a new message to a discussion room and broadcasts via SSE.
// POST /api/rooms/{id}/messages or POST /api/projects/rooms/{id}/messages
func (s *Server) handleCreateProjectMessage(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	// Verify room existence
	if _, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID); err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to verify project room: "+err.Error())
		return
	}

	var payload CreateMessagePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	content := strings.TrimSpace(payload.Content)
	if content == "" {
		respondJSONError(w, http.StatusBadRequest, "message content cannot be empty")
		return
	}

	authorType := strings.TrimSpace(payload.AuthorType)
	if authorType == "" {
		authorType = "user"
	}
	normAuthor, err := db.NormalizeAuthorType(authorType)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if payload.ReplyToID != nil && strings.TrimSpace(*payload.ReplyToID) != "" {
		trimmedReply := strings.TrimSpace(*payload.ReplyToID)
		if !db.IsValidUUID(trimmedReply) {
			respondJSONError(w, http.StatusBadRequest, "invalid reply_to_id uuid format")
			return
		}
		payload.ReplyToID = &trimmedReply
	} else {
		payload.ReplyToID = nil
	}

	msg := db.ProjectMessage{
		ID:         payload.ID,
		RoomID:     roomID,
		AuthorType: normAuthor,
		AuthorID:   strings.TrimSpace(payload.AuthorID),
		AuthorName: strings.TrimSpace(payload.AuthorName),
		Content:    content,
		ReplyToID:  payload.ReplyToID,
	}

	if err := s.projectStudioRepo.CreateMessage(r.Context(), &msg); err != nil {
		if errors.Is(err, db.ErrDuplicateMessage) {
			respondJSONError(w, http.StatusConflict, "project message already exists")
			return
		}
		if errors.Is(err, db.ErrEmptyMessageContent) || errors.Is(err, db.ErrInvalidUUID) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create project message: "+err.Error())
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "message.created", msg.ToDTO())
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message": msg.ToDTO(),
	})
}

// handleListProjectMessages lists all messages in a discussion room in chronological order.
// GET /api/rooms/{id}/messages or GET /api/projects/rooms/{id}/messages
func (s *Server) handleListProjectMessages(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	// Verify room existence
	if _, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID); err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to verify project room: "+err.Error())
		return
	}

	messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), roomID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list project messages: "+err.Error())
		return
	}

	dtos := make([]db.ProjectMessageDTO, len(messages))
	for i, m := range messages {
		dtos[i] = m.ToDTO()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"messages": dtos,
		"count":    len(dtos),
	})
}

// handleDeleteProjectMessage deletes a message by its ID and broadcasts message.deleted via SSE.
// DELETE /api/messages/{id}
func (s *Server) handleDeleteProjectMessage(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	msgID := strings.TrimSpace(chi.URLParam(r, "id"))
	if msgID == "" {
		respondJSONError(w, http.StatusBadRequest, "message id is required")
		return
	}
	if !db.IsValidUUID(msgID) {
		respondJSONError(w, http.StatusBadRequest, "invalid message uuid format")
		return
	}

	msg, err := s.projectStudioRepo.GetMessageByID(r.Context(), msgID)
	if err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			respondJSONError(w, http.StatusNotFound, "project message not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project message: "+err.Error())
		return
	}

	if err := s.projectStudioRepo.DeleteMessage(r.Context(), msgID); err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			respondJSONError(w, http.StatusNotFound, "project message not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete project message: "+err.Error())
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(msg.RoomID, "message.deleted", map[string]string{
			"id":      msgID,
			"room_id": msg.RoomID,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
		"id":     msgID,
	})
}

// handleStreamRoomEvents handles real-time Server-Sent Events (SSE) for discussion rooms.
// Features a 15-second heartbeat, real-time event broadcasting, and graceful connection cleanup on disconnect.
// GET /api/rooms/{id}/events or GET /api/projects/rooms/{id}/events
func (s *Server) handleStreamRoomEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "streaming unsupported: flusher unavailable")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	// Verify room exists in repository
	if s.projectStudioRepo != nil {
		if _, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID); err != nil {
			if errors.Is(err, db.ErrRoomNotFound) {
				respondJSONError(w, http.StatusNotFound, "project room not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get project room: "+err.Error())
			return
		}
	}

	// Set standard Server-Sent Events headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Emit initial connection handshake event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\",\"room_id\":\"%s\"}\n\n", roomID)
	flusher.Flush()

	// Subscribe client channel
	if s.roomEventBroker == nil {
		s.roomEventBroker = NewRoomEventBroker()
	}
	clientChan := s.roomEventBroker.Subscribe(roomID)
	defer s.roomEventBroker.Unsubscribe(roomID, clientChan)

	// Heartbeat ticker (15s default)
	heartbeatInterval := s.roomEventBroker.HeartbeatInterval()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected gracefully; deferred unsubscribe will clean up channel
			return

		case <-ticker.C:
			// Send heartbeat comment and ping event
			fmt.Fprintf(w, ": heartbeat\n\nevent: ping\ndata: {\"timestamp\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
			flusher.Flush()

		case ev, ok := <-clientChan:
			if !ok {
				return
			}
			dataBytes, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(dataBytes))
			flusher.Flush()
		}
	}
}

// handleDistillTaskFromRoom analyzes room message history and distills a structured task draft
// with extracted Definition of Done (DoD) checklist items.
// POST /api/rooms/{id}/distill or POST /api/projects/rooms/{id}/distill
func (s *Server) handleDistillTaskFromRoom(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "room id is required")
		return
	}
	if !db.IsValidUUID(roomID) {
		respondJSONError(w, http.StatusBadRequest, "invalid room uuid format")
		return
	}

	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "project room not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project room: "+err.Error())
		return
	}

	var payload DistillTaskPayload
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), roomID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list messages for distiller: "+err.Error())
		return
	}

	task := DistillTaskFromMessages(room, messages, payload)

	if err := s.projectStudioRepo.CreateTask(r.Context(), task); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to persist distilled task: "+err.Error())
		return
	}

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "task.distilled", task.ToDTO())
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"task":                     task.ToDTO(),
		"distilled_from_messages": len(messages),
	})
}

// DistillTaskFromMessages extracts structured task title, description, and DoD checklist items
// from discussion room context and chat messages.
func DistillTaskFromMessages(room *db.ProjectRoom, messages []db.ProjectMessage, payload DistillTaskPayload) *db.ProjectTask {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		if room.Title != "" {
			title = "Task: " + room.Title
		} else {
			title = "Distilled Task from Discussion"
		}
	}

	var descLines []string
	if room.Topic != "" {
		descLines = append(descLines, "Room Topic: "+room.Topic)
	}

	var dodItems []db.DoDItem
	dodSeen := make(map[string]bool)

	selectedIDs := make(map[string]bool)
	for _, id := range payload.MessageIDs {
		selectedIDs[id] = true
	}

	for _, msg := range messages {
		if len(selectedIDs) > 0 && !selectedIDs[msg.ID] {
			continue
		}

		lines := strings.Split(msg.Content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			// Extract markdown checklist items e.g. - [ ] Item or - [x] Item
			if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "* [ ]") {
				itemText := strings.TrimSpace(trimmed[5:])
				if itemText != "" && !dodSeen[itemText] {
					dodItems = append(dodItems, db.DoDItem{
						ID:        db.GenerateUUID(),
						Text:      itemText,
						Completed: false,
					})
					dodSeen[itemText] = true
				}
			} else if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]") || strings.HasPrefix(trimmed, "* [x]") {
				itemText := strings.TrimSpace(trimmed[5:])
				if itemText != "" && !dodSeen[itemText] {
					dodItems = append(dodItems, db.DoDItem{
						ID:        db.GenerateUUID(),
						Text:      itemText,
						Completed: true,
					})
					dodSeen[itemText] = true
				}
			} else if strings.HasPrefix(strings.ToLower(trimmed), "dod:") {
				itemText := strings.TrimSpace(trimmed[4:])
				if itemText != "" && !dodSeen[itemText] {
					dodItems = append(dodItems, db.DoDItem{
						ID:        db.GenerateUUID(),
						Text:      itemText,
						Completed: false,
					})
					dodSeen[itemText] = true
				}
			}
		}

		descLines = append(descLines, fmt.Sprintf("[%s - %s]: %s", msg.AuthorType, msg.AuthorName, msg.Content))
	}

	// Fallback DoD item if none extracted
	if len(dodItems) == 0 {
		dodItems = append(dodItems, db.DoDItem{
			ID:        db.GenerateUUID(),
			Text:      "Complete implementation based on room discussion",
			Completed: false,
		})
	}

	tags := payload.Tags
	if tags == nil {
		tags = []string{"studio", "distilled"}
	}

	task := &db.ProjectTask{
		ProjectID:    room.ProjectID,
		RoomID:       &room.ID,
		Title:        title,
		Description:  strings.Join(descLines, "\n\n"),
		Status:       "backlog",
		Priority:     "normal",
		DoDChecklist: dodItems,
		BatchID:      payload.BatchID,
		Tags:         tags,
	}

	return task
}
