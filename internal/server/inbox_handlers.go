package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
)

// InboxThreadItem represents a summarized thread item for the inbox sub-panel list.
type InboxThreadItem struct {
	ID           string                `json:"id"`
	ProjectID    string                `json:"project_id"`
	ProjectName  string                `json:"project_name"`
	Title        string                `json:"title"`
	Topic        string                `json:"topic"`
	Status       string                `json:"status"` // 'unread', 'in_progress', 'done'
	LastMessage  *db.ProjectMessageDTO `json:"last_message,omitempty"`
	MessageCount int                   `json:"message_count"`
	TaskCount    int                   `json:"task_count"`
	ReviewTasks  int                   `json:"review_tasks"`
	ActiveTasks  int                   `json:"active_tasks"`
	QueuedTasks  int                   `json:"queued_tasks"`
	DoneTasks    int                   `json:"done_tasks"`
	FailedTasks  int                   `json:"failed_tasks"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// CreateInboxThreadRequest defines the payload for creating a new conversation thread.
type CreateInboxThreadRequest struct {
	RequestID string `json:"request_id,omitempty"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	ProjectID string `json:"project_id,omitempty"`
}

// CreateInboxMessageRequest defines the payload for posting a reply message in a thread.
type CreateInboxMessageRequest struct {
	RequestID  string `json:"request_id,omitempty"`
	Content    string `json:"content"`
	AuthorType string `json:"author_type,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
}

// ApproveInboxThreadRequest defines the payload for 1-click approval to spawn tasks.
type ApproveInboxThreadRequest struct {
	ProjectID    string       `json:"project_id,omitempty"`
	Title        string       `json:"title,omitempty"`
	DoDChecklist []db.DoDItem `json:"dod_checklist,omitempty"`
	Assignee     string       `json:"assignee,omitempty"`
}

// handleListInboxThreads retrieves all conversational inbox threads.
// GET /api/v1/inbox or GET /api/inbox
func (s *Server) handleListInboxThreads(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	statusFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	projectIDFilter := strings.TrimSpace(r.URL.Query().Get("project_id"))

	var rooms []db.ProjectRoom
	var err error

	if projectIDFilter != "" {
		rooms, err = s.projectStudioRepo.ListRoomsByProjectID(r.Context(), projectIDFilter)
	} else {
		rooms, err = s.projectStudioRepo.ListAllRooms(r.Context())
	}

	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list inbox threads: "+err.Error())
		return
	}

	projectNameCache := make(map[string]string)
	var threadItems []InboxThreadItem

	var allRecentMessages map[string][]db.ProjectMessage
	if s.projectStudioRepo != nil {
		allRecentMessages, _ = s.projectStudioRepo.ListRecentMessages(r.Context(), 50)
	}
	counts, _ := s.readTaskCounts(r.Context())

	for _, rm := range rooms {
		// Fetch messages for message count and last message from batch loaded recent messages
		messages := visibleInboxMessages(allRecentMessages[rm.ID])
		messageCount := len(messages)
		var lastMsgDTO *db.ProjectMessageDTO
		if messageCount > 0 {
			dto := messages[messageCount-1].ToDTO()
			lastMsgDTO = &dto
		}

		c := counts.Rooms[rm.ID]
		taskCount := c.Total
		reviewTasks := c.Review
		activeTasks := c.Active
		queuedTasks := c.Queued
		doneTasks := c.Done
		failedTasks := c.Failed
		hasDone := doneTasks > 0
		hasDoing := activeTasks > 0

		derivedStatus := "unread"
		if taskCount > 0 {
			if hasDoing {
				derivedStatus = "in_progress"
			} else if reviewTasks > 0 {
				derivedStatus = "review"
			} else if hasDone {
				derivedStatus = "done"
			} else {
				derivedStatus = "unread"
			}
		}

		// Apply status filter if set
		if statusFilter != "" && statusFilter != "all" {
			if derivedStatus != statusFilter {
				continue
			}
		}

		// Resolve project name
		projName := projectNameCache[rm.ProjectID]
		if projName == "" && s.projectSvc != nil {
			if p, err := s.projectSvc.Get(r.Context(), rm.ProjectID); err == nil && p != nil {
				projName = p.Name
				projectNameCache[rm.ProjectID] = projName
			}
		}
		if projName == "" {
			projName = rm.ProjectID
		}

		threadItems = append(threadItems, InboxThreadItem{
			ID:           rm.ID,
			ProjectID:    rm.ProjectID,
			ProjectName:  projName,
			Title:        rm.Title,
			Topic:        rm.Topic,
			Status:       derivedStatus,
			LastMessage:  lastMsgDTO,
			MessageCount: messageCount,
			TaskCount:    taskCount,
			ActiveTasks:  activeTasks,
			ReviewTasks:  reviewTasks,
			QueuedTasks:  queuedTasks,
			DoneTasks:    doneTasks,
			FailedTasks:  failedTasks,
			CreatedAt:    rm.CreatedAt,
			UpdatedAt:    rm.UpdatedAt,
		})
	}

	if threadItems == nil {
		threadItems = []InboxThreadItem{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"threads": threadItems,
		"total":   len(threadItems),
	})
}

// handleCreateInboxThread preserves the legacy envelope over durable Studio intake.
// POST /api/v1/inbox or POST /api/inbox
func (s *Server) handleCreateInboxThread(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	var req CreateInboxThreadRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32000)).Decode(&req) != nil {
		respondJSONError(w, 400, "Invalid conversation request")
		return
	}
	if req.RequestID == "" {
		req.RequestID = db.GenerateUUID()
	}
	payload, _ := json.Marshal(map[string]string{
		"goal": req.Message, "title": req.Title,
		"project_id": req.ProjectID, "request_id": req.RequestID,
	})
	copy := r.Clone(r.Context())
	copy.Body = io.NopCloser(bytes.NewReader(payload))
	copy.ContentLength = int64(len(payload))
	accepted := &delegationResponse{header: make(http.Header)}
	s.handleStudioGoal(accepted, copy)
	if accepted.code != http.StatusAccepted {
		w.WriteHeader(accepted.code)
		_, _ = w.Write(accepted.body.Bytes())
		return
	}
	var result struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(accepted.body.Bytes(), &result); err != nil {
		respondJSONError(w, 503, "Conversation accepted; reconcile using the same request ID")
		return
	}
	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), result.ConversationID)
	if err != nil || room == nil {
		respondJSONError(w, 503, "Conversation accepted; reconcile using the same request ID")
		return
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), room.ID)
	if err != nil {
		respondJSONError(w, 503, "Conversation accepted; reconcile using the same request ID")
		return
	}
	dtos := make([]db.ProjectMessageDTO, 0, len(messages))
	for _, m := range messages {
		dtos = append(dtos, m.ToDTO())
	}
	// Preserve the legacy thread envelope, but replies come from the durable PM.
	respondJSON(w, http.StatusCreated, map[string]any{
		"thread": room.ToDTO(), "messages": dtos,
		"status": "accepted", "request_id": req.RequestID,
	})
}

// handleGetInboxThread retrieves thread details, messages, and associated tasks.
// GET /api/v1/inbox/{id} or GET /api/inbox/{id}
func (s *Server) handleGetInboxThread(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "thread id is required")
		return
	}

	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "thread not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get thread: "+err.Error())
		return
	}

	messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), roomID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list messages: "+err.Error())
		return
	}

	msgDTOs := make([]db.ProjectMessageDTO, len(messages))
	for i, m := range messages {
		msgDTOs[i] = m.ToDTO()
	}

	tasks, err := s.projectStudioRepo.ListTasksByRoomID(r.Context(), roomID)
	if err != nil {
		tasks = []db.ProjectTask{}
	}

	taskDTOs := make([]db.ProjectTaskDTO, len(tasks))
	for i, t := range tasks {
		taskDTOs[i] = t.ToDTO()
	}

	// Resolve project name
	projName := room.ProjectID
	if s.projectSvc != nil {
		if p, err := s.projectSvc.Get(r.Context(), room.ProjectID); err == nil && p != nil {
			projName = p.Name
		}
	}

	derivedStatus := "unread"
	if len(tasks) > 0 {
		hasDone := false
		hasDoing := false
		reviewTasks := 0
		for _, t := range tasks {
			st := strings.ToLower(t.Status)
			if st == "review" {
				reviewTasks++
			} else if inboxTaskIsWorking(st) {
				hasDoing = true
			} else if st == "done" {
				hasDone = true
			}
		}
		if hasDoing {
			derivedStatus = "in_progress"
		} else if reviewTasks > 0 {
			derivedStatus = "review"
		} else if hasDone {
			derivedStatus = "done"
		}
	}

	threadItem := InboxThreadItem{
		ID:           room.ID,
		ProjectID:    room.ProjectID,
		ProjectName:  projName,
		Title:        room.Title,
		Topic:        room.Topic,
		Status:       derivedStatus,
		MessageCount: len(msgDTOs),
		TaskCount:    len(taskDTOs),
		CreatedAt:    room.CreatedAt,
		UpdatedAt:    room.UpdatedAt,
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"thread":   threadItem,
		"messages": msgDTOs,
		"tasks":    taskDTOs,
	})
}

// handleCreateInboxMessage posts a reply to an inbox thread.
// POST /api/v1/inbox/{id}/messages or POST /api/inbox/{id}/messages
func (s *Server) handleCreateInboxMessage(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	if roomID == "" {
		respondJSONError(w, http.StatusBadRequest, "thread id is required")
		return
	}

	room, err := s.projectStudioRepo.GetRoomByID(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			respondJSONError(w, http.StatusNotFound, "thread not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to verify thread: "+err.Error())
		return
	}

	var req CreateInboxMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondJSONError(w, http.StatusBadRequest, "message content cannot be empty")
		return
	}
	if roomAgentID(room.Topic) != "" {
		if req.AuthorType != "" && req.AuthorType != "user" {
			respondJSONError(w, 400, "Send an operator message to the assistant")
			return
		}
		if req.RequestID == "" {
			req.RequestID = db.GenerateUUID()
		}
		payload, _ := json.Marshal(map[string]string{"message": content, "request_id": req.RequestID})
		copy := r.Clone(r.Context())
		copy.Body = io.NopCloser(bytes.NewReader(payload))
		copy.ContentLength = int64(len(payload))
		accepted := &delegationResponse{header: make(http.Header)}
		s.handleInternalSendConversationMessage(accepted, copy)
		if accepted.code != http.StatusAccepted {
			w.WriteHeader(accepted.code)
			_, _ = w.Write(accepted.body.Bytes())
			return
		}
		var result struct {
			Request db.InboxRequest `json:"request"`
		}
		if err := json.Unmarshal(accepted.body.Bytes(), &result); err != nil {
			respondJSONError(w, 503, "Message accepted; reconcile using the same request ID")
			return
		}
		message, err := s.projectStudioRepo.GetMessageByID(r.Context(), result.Request.MessageID)
		if err != nil || message == nil {
			respondJSONError(w, 503, "Message accepted; reconcile using the same request ID")
			return
		}
		respondJSON(w, http.StatusCreated, map[string]any{"message": message.ToDTO(), "request": result.Request})
		return
	}

	authorType := "user"
	if req.AuthorType != "" {
		if norm, err := db.NormalizeAuthorType(req.AuthorType); err == nil {
			authorType = norm
		}
	}

	authorName := strings.TrimSpace(req.AuthorName)
	if authorName == "" {
		if authorType == "agent" {
			authorName = "Scrum Master"
		} else {
			authorName = "Product Owner"
		}
	}

	now := time.Now().UTC()
	msg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: authorType,
		AuthorID:   "operator_user",
		AuthorName: authorName,
		Content:    content,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.projectStudioRepo.CreateMessage(r.Context(), msg); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create message: "+err.Error())
		return
	}

	// Update room timestamp
	room.UpdatedAt = now
	_ = s.projectStudioRepo.UpdateRoom(r.Context(), room)

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "message.created", msg.ToDTO())
	}
	s.OnOperatorChatMessageSent(roomID)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message": msg.ToDTO(),
	})
}

// handleApproveInboxThread handles 1-Click approval to spawn sprint tasks from thread (S17.3).
// POST /api/v1/inbox/{id}/approve or POST /api/inbox/{id}/approve
// Legacy thread approval delegates to canonical source-message capture. It no
// longer starts an unrelated Gemini job or fabricates completed DoD items.
func (s *Server) handleApproveInboxThread(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	if s.projectStudioRepo == nil {
		respondJSONError(w, 503, "Storage is unavailable")
		return
	}
	roomID := strings.TrimSpace(chi.URLParam(r, "id"))
	room, e := s.projectStudioRepo.GetRoomByID(r.Context(), roomID)
	if e != nil || room == nil {
		respondJSONError(w, 404, "Conversation not found")
		return
	}
	var req ApproveInboxThreadRequest
	if r.Body != nil && json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req) != nil {
		respondJSONError(w, 400, "Invalid task request")
		return
	}
	messages, e := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), roomID)
	if e != nil {
		respondJSONError(w, 503, "Could not read the message")
		return
	}
	var source *db.ProjectMessage
	for i := range messages {
		if messages[i].AuthorType == "user" && (source == nil || messages[i].CreatedAt.After(source.CreatedAt)) {
			source = &messages[i]
		}
	}
	if source == nil {
		respondJSONError(w, 400, "Select a source message before creating a task")
		return
	}
	if req.ProjectID == "" {
		req.ProjectID = room.ProjectID
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = room.Title
	}
	payload, _ := json.Marshal(req)
	copy := r.Clone(r.Context())
	copy.Body = io.NopCloser(bytes.NewReader(payload))
	params := chi.NewRouteContext()
	params.URLParams.Add("id", roomID)
	params.URLParams.Add("messageID", source.ID)
	copy = copy.WithContext(context.WithValue(copy.Context(), chi.RouteCtxKey, params))
	s.handleInboxMessageTask(w, copy)
}
