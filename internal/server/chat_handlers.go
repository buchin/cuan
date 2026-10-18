package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
)

// ChatContact represents a contact in the direct messenger hub (agent or customer).
type ChatContact struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"` // "agent" or "customer"
	Name           string    `json:"name"`
	Avatar         string    `json:"avatar"`
	Status         string    `json:"status"` // "online", "idle", "offline"
	Product        string    `json:"product,omitempty"`
	GuestUUID      string    `json:"guest_uuid,omitempty"`
	ConversationID string    `json:"conversation_id"`
	UnreadCount    int       `json:"unread_count"`
	LastMessage    string    `json:"last_message,omitempty"`
	LastMessageAt  time.Time `json:"last_message_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AgentActionCard represents an interactive 1-click execution card in an agent bubble.
type AgentActionCard struct {
	TaskTitle      string       `json:"task_title"`
	DoDChecklist   []db.DoDItem `json:"dod_checklist"`
	WorkerAgent    string       `json:"worker_agent"`
	Recommended    bool         `json:"recommended"`
	ApproveURL     string       `json:"approve_url,omitempty"`
	ExecutionState string       `json:"execution_state,omitempty"` // "ready", "running", "completed"
}

// ChatMessageDTO represents a message compatible with both Direct Messenger UI and Dojo Live Chat widget.
type ChatMessageDTO struct {
	ID              int64            `json:"id"`
	UUID            string           `json:"uuid,omitempty"`
	ChatSessionUUID string           `json:"chat_session_uuid,omitempty"`
	RoomID          string           `json:"room_id,omitempty"`
	SenderType      string           `json:"sender_type"` // "visitor" or "operator"
	AuthorType      string           `json:"author_type"` // "user", "agent", "system"
	AuthorName      string           `json:"author_name"`
	Message         string           `json:"message"`
	Content         string           `json:"content"`
	Product         string           `json:"product,omitempty"`
	AttachmentURL   string           `json:"attachment_url,omitempty"`
	AttachmentType  string           `json:"attachment_type,omitempty"`
	AttachmentName  string           `json:"attachment_name,omitempty"`
	AttachmentSize  int64            `json:"attachment_size,omitempty"`
	HasAttachment   bool             `json:"has_attachment,omitempty"`
	ActionCard      *AgentActionCard `json:"action_card,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

// LiveChatSession stores visitor session metadata for the Dojo live chat widget.
type LiveChatSession struct {
	UUID        string    `json:"uuid"`
	GuestUUID   string    `json:"guest_uuid"`
	Product     string    `json:"product"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	CurrentURL  string    `json:"current_url"`
	PageTitle   string    `json:"page_title"`
	ReferrerURL string    `json:"referrer_url"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ChatSessionManager tracks live chat sessions in a thread-safe registry.
type ChatSessionManager struct {
	sync.RWMutex
	sessions       map[string]*LiveChatSession
	sessionByGuest map[string]*LiveChatSession
}

var globalChatManager = &ChatSessionManager{
	sessions:       make(map[string]*LiveChatSession),
	sessionByGuest: make(map[string]*LiveChatSession),
}

const (
	defaultScrumMasterRoomID = "00000000-0000-4000-8000-000000000001"
	defaultAntigravityRoomID  = "00000000-0000-4000-8000-000000000002"
)

// chatCORSMiddleware ensures all chat endpoints and widget JS have open CORS headers.
func chatCORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// EnsureDefaultAgentRooms ensures default rooms exist for Scrum Master and Antigravity agents.
func (s *Server) ensureDefaultAgentRooms(ctx context.Context) {
	if s.projectStudioRepo == nil {
		return
	}

	// 1. Ensure Scrum Master room
	if _, err := s.projectStudioRepo.GetRoomByID(ctx, defaultScrumMasterRoomID); err != nil {
		now := time.Now().UTC()
		_ = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
			ID:        defaultScrumMasterRoomID,
			ProjectID: "default",
			Title:     "Scrum Master Direct",
			Topic:     "Agentic Scrum Management",
			CreatedAt: now,
			UpdatedAt: now,
		})
		_ = s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
			ID:         db.GenerateUUID(),
			RoomID:     defaultScrumMasterRoomID,
			AuthorType: "agent",
			AuthorID:   "agent_scrum_master",
			AuthorName: "Scrum Master",
			Content:    "Halo Buchin! Saya Scrum Master Anda. Sampaikan kebutuhan, bug, atau ide sprint Anda di sini.",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// 2. Ensure Antigravity Agent room
	if _, err := s.projectStudioRepo.GetRoomByID(ctx, defaultAntigravityRoomID); err != nil {
		now := time.Now().UTC()
		_ = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
			ID:        defaultAntigravityRoomID,
			ProjectID: "default",
			Title:     "Antigravity Flash",
			Topic:     "Autonomous Worker Agent",
			CreatedAt: now,
			UpdatedAt: now,
		})
		_ = s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
			ID:         db.GenerateUUID(),
			RoomID:     defaultAntigravityRoomID,
			AuthorType: "agent",
			AuthorID:   "antigravity-flash",
			AuthorName: "Antigravity Flash",
			Content:    "Antigravity Worker siap menjalankan eksekusi task berkecepatan tinggi.",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
}

// ============================================================================
// VISITOR API (Dojo Live Chat Widget Compatible)
// ============================================================================

// POST /api/chat/session
func (s *Server) handleVisitorChatSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		GuestUUID   string `json:"guest_uuid"`
		SessionUUID string `json:"session_uuid"`
		CurrentURL  string `json:"current_url"`
		PageTitle   string `json:"page_title"`
		ReferrerURL string `json:"referrer_url"`
		Product     string `json:"product"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		Message     string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	guestUUID := strings.TrimSpace(payload.GuestUUID)
	if guestUUID == "" {
		guestUUID = db.GenerateUUID()
	}

	sessionUUID := strings.TrimSpace(payload.SessionUUID)
	if sessionUUID == "" {
		globalChatManager.RLock()
		if existing, ok := globalChatManager.sessionByGuest[guestUUID]; ok && existing != nil {
			sessionUUID = existing.UUID
		}
		globalChatManager.RUnlock()
	}
	if sessionUUID == "" {
		sessionUUID = db.GenerateUUID()
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		if payload.Product != "" {
			name = fmt.Sprintf("Customer (%s)", payload.Product)
		} else {
			name = fmt.Sprintf("Visitor #%s", guestUUID[:6])
		}
	}

	now := time.Now().UTC()
	liveSession := &LiveChatSession{
		UUID:        sessionUUID,
		GuestUUID:   guestUUID,
		Product:     strings.TrimSpace(payload.Product),
		Name:        name,
		Email:       strings.TrimSpace(payload.Email),
		CurrentURL:  payload.CurrentURL,
		PageTitle:   payload.PageTitle,
		ReferrerURL: payload.ReferrerURL,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	globalChatManager.Lock()
	globalChatManager.sessions[sessionUUID] = liveSession
	globalChatManager.sessionByGuest[guestUUID] = liveSession
	globalChatManager.Unlock()

	// Ensure backing database room
	if s.projectStudioRepo != nil {
		if _, err := s.projectStudioRepo.GetRoomByID(r.Context(), sessionUUID); err != nil {
			_ = s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        sessionUUID,
				ProjectID: "default",
				Title:     name,
				Topic:     liveSession.Product,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		// If initial message provided in session payload
		if text := strings.TrimSpace(payload.Message); text != "" {
			_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
				ID:         db.GenerateUUID(),
				RoomID:     sessionUUID,
				AuthorType: "user",
				AuthorID:   guestUUID,
				AuthorName: name,
				Content:    text,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}

	// Fetch existing messages for session
	messages := s.getFormattedMessages(r.Context(), sessionUUID, liveSession.Product)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"session_uuid": liveSession.UUID,
		"session": map[string]interface{}{
			"uuid":       liveSession.UUID,
			"guest_uuid": liveSession.GuestUUID,
			"product":    liveSession.Product,
			"name":       liveSession.Name,
			"email":      liveSession.Email,
			"status":     liveSession.Status,
		},
		"messages": messages,
	})
}

// GET /api/chat/messages
func (s *Server) handleVisitorGetMessages(w http.ResponseWriter, r *http.Request) {
	sessionUUID := strings.TrimSpace(r.URL.Query().Get("session_uuid"))
	if sessionUUID == "" {
		respondJSONError(w, http.StatusBadRequest, "session_uuid is required")
		return
	}

	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after_id"), 10, 64)

	product := ""
	globalChatManager.RLock()
	if sess, ok := globalChatManager.sessions[sessionUUID]; ok && sess != nil {
		product = sess.Product
	}
	globalChatManager.RUnlock()

	allMessages := s.getFormattedMessages(r.Context(), sessionUUID, product)
	filtered := make([]ChatMessageDTO, 0)
	for _, m := range allMessages {
		if m.ID > afterID {
			filtered = append(filtered, m)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"messages": filtered,
		"receipts": []interface{}{},
	})
}

// POST /api/chat/messages
func (s *Server) handleVisitorSendMessage(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionUUID    string `json:"session_uuid"`
		Message        string `json:"message"`
		Text           string `json:"text"`
		Content        string `json:"content"`
		GuestUUID      string `json:"guest_uuid"`
		AttachmentURL  string `json:"attachment_url"`
		AttachmentType string `json:"attachment_type"`
		AttachmentName string `json:"attachment_name"`
		AttachmentSize int64  `json:"attachment_size"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	sessionUUID := strings.TrimSpace(payload.SessionUUID)
	if sessionUUID == "" {
		respondJSONError(w, http.StatusBadRequest, "session_uuid is required")
		return
	}

	text := strings.TrimSpace(payload.Message)
	if text == "" {
		text = strings.TrimSpace(payload.Text)
	}
	if text == "" {
		text = strings.TrimSpace(payload.Content)
	}
	if text == "" && payload.AttachmentURL == "" {
		respondJSONError(w, http.StatusBadRequest, "message cannot be empty")
		return
	}

	globalChatManager.RLock()
	sess := globalChatManager.sessions[sessionUUID]
	globalChatManager.RUnlock()

	authorName := "Customer"
	product := ""
	if sess != nil {
		authorName = sess.Name
		product = sess.Product
	}

	now := time.Now().UTC()
	msgUUID := db.GenerateUUID()

	if s.projectStudioRepo != nil {
		// Ensure room exists
		if _, err := s.projectStudioRepo.GetRoomByID(r.Context(), sessionUUID); err != nil {
			_ = s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        sessionUUID,
				ProjectID: "default",
				Title:     authorName,
				Topic:     product,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
			ID:         msgUUID,
			RoomID:     sessionUUID,
			AuthorType: "user",
			AuthorID:   payload.GuestUUID,
			AuthorName: authorName,
			Content:    text,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// Determine new message index
	messages := s.getFormattedMessages(r.Context(), sessionUUID, product)
	var newMsgDTO ChatMessageDTO
	for _, m := range messages {
		if m.UUID == msgUUID {
			newMsgDTO = m
			break
		}
	}
	if newMsgDTO.ID == 0 {
		newMsgDTO = ChatMessageDTO{
			ID:              int64(len(messages)),
			UUID:            msgUUID,
			ChatSessionUUID: sessionUUID,
			RoomID:          sessionUUID,
			SenderType:      "visitor",
			AuthorType:      "user",
			AuthorName:      authorName,
			Message:         text,
			Content:         text,
			Product:         product,
			AttachmentURL:   payload.AttachmentURL,
			AttachmentType:  payload.AttachmentType,
			AttachmentName:  payload.AttachmentName,
			AttachmentSize:  payload.AttachmentSize,
			HasAttachment:   payload.AttachmentURL != "",
			CreatedAt:       now,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": newMsgDTO,
	})
}

// POST /api/chat/sessions/{uuid}/page
func (s *Server) handleVisitorPageTelemetry(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if uuid == "" {
		uuid = chi.URLParam(r, "id")
	}

	var payload struct {
		CurrentURL  string `json:"current_url"`
		PageTitle   string `json:"page_title"`
		ReferrerURL string `json:"referrer_url"`
		Product     string `json:"product"`
	}

	_ = json.NewDecoder(r.Body).Decode(&payload)

	globalChatManager.Lock()
	if sess, ok := globalChatManager.sessions[uuid]; ok && sess != nil {
		if payload.CurrentURL != "" {
			sess.CurrentURL = payload.CurrentURL
		}
		if payload.PageTitle != "" {
			sess.PageTitle = payload.PageTitle
		}
		if payload.Product != "" {
			sess.Product = payload.Product
		}
		sess.UpdatedAt = time.Now().UTC()
	}
	globalChatManager.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
	})
}

// POST /api/chat/receipts
func (s *Server) handleVisitorReceipts(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"receipts": []interface{}{},
	})
}

// ============================================================================
// INTERNAL DIRECT MESSENGER API
// ============================================================================

// GET /api/v1/chat/contacts
func (s *Server) handleInternalListContacts(w http.ResponseWriter, r *http.Request) {
	s.ensureDefaultAgentRooms(r.Context())

	typeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	searchQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	contacts := make([]ChatContact, 0)

	// 1. Scrum Master Agent Contact
	smLastMsg := "Siap membantu merumuskan DoD dan mengoordinasikan sprint."
	smLastAt := time.Now().UTC()
	smUnread := 0
	smRoomID := defaultScrumMasterRoomID

	// S17 backward compatibility: check most recent room in projectStudioRepo
	if s.projectStudioRepo != nil {
		if allRooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil && len(allRooms) > 0 {
			// Find most active room or use default
			for _, rm := range allRooms {
				if rm.ID == defaultScrumMasterRoomID || strings.Contains(strings.ToLower(rm.Title), "scrum") {
					smRoomID = rm.ID
					if msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), rm.ID); len(msgs) > 0 {
						smLastMsg = msgs[len(msgs)-1].Content
						smLastAt = msgs[len(msgs)-1].CreatedAt
					}
					break
				}
			}
		}
	}

	smContact := ChatContact{
		ID:             "contact_scrum_master",
		Type:           "agent",
		Name:           "Scrum Master",
		Avatar:         "👔",
		Status:         "online",
		ConversationID: smRoomID,
		UnreadCount:    smUnread,
		LastMessage:    smLastMsg,
		LastMessageAt:  smLastAt,
		UpdatedAt:      smLastAt,
	}

	// 2. Antigravity Agent Contact
	agContact := ChatContact{
		ID:             "contact_antigravity",
		Type:           "agent",
		Name:           "Antigravity Flash",
		Avatar:         "🤖",
		Status:         "online",
		ConversationID: defaultAntigravityRoomID,
		UnreadCount:    0,
		LastMessage:    "Antigravity Worker siap mengeksekusi task.",
		LastMessageAt:  time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if typeFilter != "customers" {
		contacts = append(contacts, smContact, agContact)
	}

	// 3. Customer Live Chat Sessions (from Dojo Live Chat visitors)
	if typeFilter != "agents" {
		globalChatManager.RLock()
		sessionList := make([]*LiveChatSession, 0, len(globalChatManager.sessions))
		for _, sess := range globalChatManager.sessions {
			sessionList = append(sessionList, sess)
		}
		globalChatManager.RUnlock()

		// Also read any rooms from DB that belong to visitors
		if s.projectStudioRepo != nil {
			if allRooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil {
				for _, rm := range allRooms {
					if rm.ID != defaultScrumMasterRoomID && rm.ID != defaultAntigravityRoomID && !strings.Contains(strings.ToLower(rm.Title), "scrum") {
						globalChatManager.Lock()
						if _, exists := globalChatManager.sessions[rm.ID]; !exists {
							sess := &LiveChatSession{
								UUID:        rm.ID,
								GuestUUID:   rm.ID,
								Product:     rm.Topic,
								Name:        rm.Title,
								Status:      "online",
								CreatedAt:   rm.CreatedAt,
								UpdatedAt:   rm.UpdatedAt,
							}
							globalChatManager.sessions[rm.ID] = sess
							sessionList = append(sessionList, sess)
						}
						globalChatManager.Unlock()
					}
				}
			}
		}

		sort.Slice(sessionList, func(i, j int) bool {
			return sessionList[i].UpdatedAt.After(sessionList[j].UpdatedAt)
		})

		for _, sess := range sessionList {
			lastMsg := "Sesi percakapan baru dimulai."
			lastAt := sess.UpdatedAt
			unread := 0

			if s.projectStudioRepo != nil {
				if msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), sess.UUID); len(msgs) > 0 {
					lastMsg = msgs[len(msgs)-1].Content
					lastAt = msgs[len(msgs)-1].CreatedAt
					for _, m := range msgs {
						if m.AuthorType == "user" {
							unread++
						}
					}
				}
			}

			custContact := ChatContact{
				ID:             "cust_" + sess.UUID,
				Type:           "customer",
				Name:           sess.Name,
				Avatar:         "👤",
				Status:         "online",
				Product:        sess.Product,
				GuestUUID:      sess.GuestUUID,
				ConversationID: sess.UUID,
				UnreadCount:    unread,
				LastMessage:    lastMsg,
				LastMessageAt:  lastAt,
				UpdatedAt:      lastAt,
			}
			contacts = append(contacts, custContact)
		}
	}

	// Filter by search query if specified
	if searchQuery != "" {
		filtered := make([]ChatContact, 0)
		for _, c := range contacts {
			if strings.Contains(strings.ToLower(c.Name), searchQuery) ||
				strings.Contains(strings.ToLower(c.LastMessage), searchQuery) ||
				strings.Contains(strings.ToLower(c.Product), searchQuery) {
				filtered = append(filtered, c)
			}
		}
		contacts = filtered
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"contacts": contacts,
		"total":    len(contacts),
	})
}

// GET /api/v1/chat/conversations/{id}/messages
func (s *Server) handleInternalGetConversationMessages(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if convID == "" {
		respondJSONError(w, http.StatusBadRequest, "conversation id is required")
		return
	}

	s.ensureDefaultAgentRooms(r.Context())

	// Resolve contact
	var contact ChatContact
	if convID == defaultScrumMasterRoomID || convID == "contact_scrum_master" {
		convID = defaultScrumMasterRoomID
		contact = ChatContact{
			ID:             "contact_scrum_master",
			Type:           "agent",
			Name:           "Scrum Master",
			Avatar:         "👔",
			Status:         "online",
			ConversationID: defaultScrumMasterRoomID,
		}
	} else if convID == defaultAntigravityRoomID || convID == "contact_antigravity" {
		convID = defaultAntigravityRoomID
		contact = ChatContact{
			ID:             "contact_antigravity",
			Type:           "agent",
			Name:           "Antigravity Flash",
			Avatar:         "🤖",
			Status:         "online",
			ConversationID: defaultAntigravityRoomID,
		}
	} else {
		// Customer contact
		globalChatManager.RLock()
		sess := globalChatManager.sessions[convID]
		globalChatManager.RUnlock()

		name := "Customer"
		product := ""
		guestUUID := ""
		if sess != nil {
			name = sess.Name
			product = sess.Product
			guestUUID = sess.GuestUUID
		} else if s.projectStudioRepo != nil {
			if rm, err := s.projectStudioRepo.GetRoomByID(r.Context(), convID); err == nil && rm != nil {
				name = rm.Title
				product = rm.Topic
			}
		}

		contact = ChatContact{
			ID:             "cust_" + convID,
			Type:           "customer",
			Name:           name,
			Avatar:         "👤",
			Status:         "online",
			Product:        product,
			GuestUUID:      guestUUID,
			ConversationID: convID,
		}
	}

	product := contact.Product
	messages := s.getFormattedMessages(r.Context(), convID, product)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversation_id": convID,
		"contact":         contact,
		"messages":        messages,
	})
}

// POST /api/v1/chat/conversations/{id}/messages
func (s *Server) handleInternalSendConversationMessage(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if convID == "" {
		respondJSONError(w, http.StatusBadRequest, "conversation id is required")
		return
	}

	var payload struct {
		Message    string `json:"message"`
		Content    string `json:"content"`
		AuthorType string `json:"author_type"`
		AuthorName string `json:"author_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	text := strings.TrimSpace(payload.Message)
	if text == "" {
		text = strings.TrimSpace(payload.Content)
	}
	if text == "" {
		respondJSONError(w, http.StatusBadRequest, "message cannot be empty")
		return
	}

	now := time.Now().UTC()
	msgUUID := db.GenerateUUID()

	authorType := strings.TrimSpace(payload.AuthorType)
	if authorType == "" {
		authorType = "user"
	}
	authorName := strings.TrimSpace(payload.AuthorName)
	if authorName == "" {
		authorName = "Buchin"
	}

	// Save the operator/user message
	if s.projectStudioRepo != nil {
		_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
			ID:         msgUUID,
			RoomID:     convID,
			AuthorType: authorType,
			AuthorID:   "operator_user",
			AuthorName: authorName,
			Content:    text,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// If conversation is with Scrum Master agent, trigger automated triage response
	var agentMsgDTO *ChatMessageDTO
	if convID == defaultScrumMasterRoomID || strings.Contains(convID, "scrum") {
		triage := generateSMTriageResponse(text)
		agentUUID := db.GenerateUUID()
		agentNow := now.Add(200 * time.Millisecond)

		if s.projectStudioRepo != nil {
			_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
				ID:         agentUUID,
				RoomID:     convID,
				AuthorType: "agent",
				AuthorID:   "agent_scrum_master",
				AuthorName: "Scrum Master",
				Content:    triage.Text,
				CreatedAt:  agentNow,
				UpdatedAt:  agentNow,
			})
		}

		card := &AgentActionCard{
			TaskTitle:      triage.ProblemTitle,
			DoDChecklist:   triage.DoDItems,
			WorkerAgent:    triage.WorkerAgent,
			Recommended:    true,
			ApproveURL:     fmt.Sprintf("/api/v1/inbox/%s/approve", convID),
			ExecutionState: "ready",
		}

		agentMsgDTO = &ChatMessageDTO{
			ID:         2,
			UUID:       agentUUID,
			RoomID:     convID,
			SenderType: "operator",
			AuthorType: "agent",
			AuthorName: "Scrum Master",
			Message:    triage.Text,
			Content:    triage.Text,
			ActionCard: card,
			CreatedAt:  agentNow,
		}
	}

	userDTO := ChatMessageDTO{
		ID:         1,
		UUID:       msgUUID,
		RoomID:     convID,
		SenderType: "visitor",
		AuthorType: authorType,
		AuthorName: authorName,
		Message:    text,
		Content:    text,
		CreatedAt:  now,
	}

	resp := map[string]interface{}{
		"message": userDTO,
	}
	if agentMsgDTO != nil {
		resp["agent_reply"] = agentMsgDTO
	}

	respondJSON(w, http.StatusOK, resp)
}

// ============================================================================
// DOJO DROP-IN WIDGET JS HANDLER (/chat/widget.js)
// ============================================================================

// handleChatWidgetJS serves the standalone Vanilla JS widget with open CORS.
func (s *Server) handleChatWidgetJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Cache-Control", "public, max-age=300")

	// Try reading from web/public/chat/widget.js
	candidatePaths := []string{
		filepath.Join("web", "public", "chat", "widget.js"),
		filepath.Join("..", "..", "web", "public", "chat", "widget.js"),
		"/opt/cuan/web/dist/chat/widget.js",
	}

	for _, p := range candidatePaths {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			_, _ = w.Write(data)
			return
		}
	}

	// Fallback to embedded script if file not found
	_, _ = io.WriteString(w, embeddedWidgetJSFallback)
}

// ============================================================================
// HELPER METHODS
// ============================================================================

func (s *Server) getFormattedMessages(ctx context.Context, roomID string, product string) []ChatMessageDTO {
	if s.projectStudioRepo == nil {
		return []ChatMessageDTO{}
	}

	rawMsgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil || len(rawMsgs) == 0 {
		return []ChatMessageDTO{}
	}

	dtos := make([]ChatMessageDTO, len(rawMsgs))
	for i, m := range rawMsgs {
		senderType := "visitor"
		if m.AuthorType == "agent" || m.AuthorType == "system" {
			senderType = "operator"
		}

		var actionCard *AgentActionCard
		if m.AuthorType == "agent" && strings.Contains(m.Content, "Definition of Done (DoD)") {
			// Extract DoD items
			cardDoD := []db.DoDItem{
				{Text: "Selesaikan instruksi spesifik", Completed: false},
				{Text: "Lulus automated test & audit evidence", Completed: false},
			}
			actionCard = &AgentActionCard{
				TaskTitle:      "Sprint Task: " + m.AuthorName,
				DoDChecklist:   cardDoD,
				WorkerAgent:    "antigravity-flash",
				Recommended:    true,
				ApproveURL:     fmt.Sprintf("/api/v1/inbox/%s/approve", roomID),
				ExecutionState: "ready",
			}
		}

		dtos[i] = ChatMessageDTO{
			ID:              int64(i + 1), // Monotonic numeric ID for Dojo widget
			UUID:            m.ID,
			ChatSessionUUID: roomID,
			RoomID:          roomID,
			SenderType:      senderType,
			AuthorType:      m.AuthorType,
			AuthorName:      m.AuthorName,
			Message:         m.Content,
			Content:         m.Content,
			Product:         product,
			ActionCard:      actionCard,
			CreatedAt:       m.CreatedAt,
		}
	}

	return dtos
}

// SMTriageResult holds distilled problem analysis and DoD.
type SMTriageResult struct {
	ProblemTitle string
	WorkerAgent  string
	DoDItems     []db.DoDItem
	Text         string
}

func generateSMTriageResponse(userMessage string) SMTriageResult {
	title := strings.TrimSpace(userMessage)
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	if title == "" {
		title = "Sprint Task"
	}

	worker := "antigravity-flash"

	dodList := []db.DoDItem{
		{Text: "Implementasikan solusi spesifik di repositori", Completed: false},
		{Text: "Targeted automated unit test PASS", Completed: false},
		{Text: "Zero regression verifikasi DB & build bundle", Completed: false},
	}

	respText := fmt.Sprintf(`Halo Buchin! Saya telah menganalisis kebutuhan Anda:

**Problem Statement:**
%s

**Rencana Eksekusi & Definition of Done (DoD):**
1. Implementasikan solusi spesifik di repositori
2. Targeted automated unit test PASS
3. Zero regression verifikasi DB & build bundle

**Rekomendasi Worker Agent:** %s (Limit kuota mencukupi).
Silakan klik tombol **Mulai Kerjakan** di bawah untuk memulai eksekusi sprint.`, title, worker)

	return SMTriageResult{
		ProblemTitle: title,
		WorkerAgent:  worker,
		DoDItems:     dodList,
		Text:         respText,
	}
}

const embeddedWidgetJSFallback = `/* Dojo Live Chat Widget (Vanilla JS Shadow DOM) */
(function() {
  if (window.__dojo_livechat_installed) return;
  window.__dojo_livechat_installed = true;

  window.$crisp = window.$crisp || [];
  window.$dojoChat = window.$dojoChat || {
    open: function() { console.log('[DojoChat] open'); },
    close: function() { console.log('[DojoChat] close'); },
    toggle: function() { console.log('[DojoChat] toggle'); },
    setMessage: function(t) { console.log('[DojoChat] setMessage', t); },
    startTopic: function(t) { console.log('[DojoChat] startTopic', t); }
  };
})();
`
