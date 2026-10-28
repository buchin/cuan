package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
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
	ActiveTasks  int                   `json:"active_tasks"`
	QueuedTasks  int                   `json:"queued_tasks"`
	DoneTasks    int                   `json:"done_tasks"`
	FailedTasks  int                   `json:"failed_tasks"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// CreateInboxThreadRequest defines the payload for creating a new conversation thread.
type CreateInboxThreadRequest struct {
	Title     string `json:"title"`
	Message   string `json:"message"`
	ProjectID string `json:"project_id,omitempty"`
}

// CreateInboxMessageRequest defines the payload for posting a reply message in a thread.
type CreateInboxMessageRequest struct {
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

	for _, rm := range rooms {
		// Fetch messages for message count and last message
		messages, _ := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), rm.ID)
		messageCount := len(messages)
		var lastMsgDTO *db.ProjectMessageDTO
		if messageCount > 0 {
			dto := messages[messageCount-1].ToDTO()
			lastMsgDTO = &dto
		}

		// Fetch linked tasks to derive status
		tasks, _ := s.projectStudioRepo.ListTasksByRoomID(r.Context(), rm.ID)
		taskCount := len(tasks)
		activeTasks := 0
		queuedTasks := 0
		doneTasks := 0
		failedTasks := 0
		hasDone := false
		hasDoing := false

		seenTaskIDs := make(map[string]bool)
		for _, t := range tasks {
			seenTaskIDs[t.ID] = true
			st := strings.ToLower(strings.TrimSpace(t.Status))
			if st == "done" {
				doneTasks++
				hasDone = true
			} else if st == "blocked" || st == "failed" {
				failedTasks++
			} else if st == "queued" || st == "pending" {
				queuedTasks++
			} else {
				activeTasks++
			}
			if st == "doing" || st == "review" {
				hasDoing = true
			}
		}

		if s.taskSvc != nil {
			if svcTasks, err := s.taskSvc.ListTasks(r.Context(), task.TaskFilter{SourceRoomID: rm.ID}); err == nil && len(svcTasks) > 0 {
				for _, st := range svcTasks {
					if !seenTaskIDs[st.ID] {
						taskCount++
						stStatus := strings.ToLower(strings.TrimSpace(st.Status))
						if stStatus == "done" {
							doneTasks++
							hasDone = true
						} else if stStatus == "blocked" || stStatus == "failed" {
							failedTasks++
						} else if stStatus == "queued" || stStatus == "pending" {
							queuedTasks++
						} else {
							activeTasks++
						}
						if stStatus == "doing" || stStatus == "review" {
							hasDoing = true
						}
					}
				}
			}
		}

		derivedStatus := "unread"
		if taskCount > 0 {
			if hasDoing {
				derivedStatus = "in_progress"
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

// handleCreateInboxThread creates a new conversation thread with initial user message and SM triage response.
// POST /api/v1/inbox or POST /api/inbox
func (s *Server) handleCreateInboxThread(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	var req CreateInboxThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	messageText := strings.TrimSpace(req.Message)
	if title == "" {
		if len(messageText) > 40 {
			title = messageText[:37] + "..."
		} else if messageText != "" {
			title = messageText
		} else {
			title = "Direct Message"
		}
	}

	if messageText == "" {
		respondJSONError(w, http.StatusBadRequest, "message cannot be empty")
		return
	}

	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" && s.projectSvc != nil {
		// Default to first active project
		if projects, err := s.projectSvc.List(r.Context()); err == nil && len(projects) > 0 {
			projectID = projects[0].ID
		}
	}
	if projectID == "" {
		projectID = "default"
	}

	roomID := db.GenerateUUID()
	now := time.Now().UTC()

	room := &db.ProjectRoom{
		ID:        roomID,
		ProjectID: projectID,
		Title:     title,
		Topic:     messageText,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.projectStudioRepo.CreateRoom(r.Context(), room); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create inbox thread: "+err.Error())
		return
	}

	// 1. Post initial user message
	userMsg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "user",
		AuthorName: "Product Owner",
		Content:    messageText,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_ = s.projectStudioRepo.CreateMessage(r.Context(), userMsg)

	// 2. Autonomous Scrum Master Triage & DoD formulation (S17.3)
	workerAgent := "antigravity-flash"
	smResponseText := fmt.Sprintf(`Here is a proposed plan for your request:

### Problem statement & scope
%s

### Definition of done
- [ ] Analyze the technical requirements and write specifications
- [ ] Implement modular, maintainable code changes
- [ ] Run targeted unit tests and check for regressions
- [ ] Verify production readiness and commit with the configured author

### Recommended worker agent
- **Worker:** %s
- **Status:** Check the agent connection and quota before starting

Select **Approve and start task** to register the task and begin execution.`,
		messageText, workerAgent)

	smMsg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorName: "Scrum Master",
		Content:    smResponseText,
		CreatedAt:  now.Add(100 * time.Millisecond),
		UpdatedAt:  now.Add(100 * time.Millisecond),
	}
	_ = s.projectStudioRepo.CreateMessage(r.Context(), smMsg)

	// Broadcast events if broker active
	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(roomID, "room.created", room.ToDTO())
		s.roomEventBroker.Broadcast(roomID, "message.created", userMsg.ToDTO())
		s.roomEventBroker.Broadcast(roomID, "message.created", smMsg.ToDTO())
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"thread": room.ToDTO(),
		"messages": []db.ProjectMessageDTO{
			userMsg.ToDTO(),
			smMsg.ToDTO(),
		},
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
		for _, t := range tasks {
			st := strings.ToLower(t.Status)
			if st == "doing" || st == "review" {
				hasDoing = true
			} else if st == "done" {
				hasDone = true
			}
		}
		if hasDoing {
			derivedStatus = "in_progress"
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
