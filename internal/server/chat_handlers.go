package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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

	// 0. Ensure default project exists first so foreign key constraints on project_rooms pass
	if s.db != nil {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO projects (id, name, repo_url, default_branch, clone_status, project_type, created_at, updated_at)
			VALUES ('default', 'cuan', 'https://github.com/buchin/cuan', 'main', 'connected', 'custom', NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET name = 'cuan' WHERE projects.name != 'cuan';
		`)
	}
	if s.projectRepo != nil {
		if proj, err := s.projectRepo.GetProjectByID(ctx, "default"); err != nil || proj == nil {
			_ = s.projectRepo.CreateProject(ctx, &db.Project{
				ID:            "default",
				Name:          "cuan",
				RepoURL:       "https://github.com/buchin/cuan",
				DefaultBranch: "main",
				CloneStatus:   "connected",
				ProjectType:   "custom",
			})
		}
	}

	// 1. Ensure Scrum Master room
	if _, err := s.projectStudioRepo.GetRoomByID(ctx, defaultScrumMasterRoomID); err != nil {
		now := time.Now().UTC()
		if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
			ID:        defaultScrumMasterRoomID,
			ProjectID: "default",
			Title:     "Scrum Master Direct",
			Topic:     "Agentic Scrum Management",
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			log.Printf("[ensureDefaultAgentRooms] failed to create Scrum Master room: %v", err)
		} else {
			if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
				ID:         db.GenerateUUID(),
				RoomID:     defaultScrumMasterRoomID,
				AuthorType: "agent",
				AuthorID:   "agent_scrum_master",
				AuthorName: "Scrum Master",
				Content:    "Halo Buchin! Saya Scrum Master Anda. Sampaikan kebutuhan, bug, atau ide sprint Anda di sini.",
				CreatedAt:  now,
				UpdatedAt:  now,
			}); err != nil {
				log.Printf("[ensureDefaultAgentRooms] failed to create initial Scrum Master message: %v", err)
			}
		}
	}

	// 2. Ensure Antigravity Agent room
	if _, err := s.projectStudioRepo.GetRoomByID(ctx, defaultAntigravityRoomID); err != nil {
		now := time.Now().UTC()
		if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
			ID:        defaultAntigravityRoomID,
			ProjectID: "default",
			Title:     "Antigravity Flash",
			Topic:     "Autonomous Worker Agent",
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			log.Printf("[ensureDefaultAgentRooms] failed to create Antigravity room: %v", err)
		} else {
			if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
				ID:         db.GenerateUUID(),
				RoomID:     defaultAntigravityRoomID,
				AuthorType: "agent",
				AuthorID:   "antigravity-flash",
				AuthorName: "Antigravity Flash",
				Content:    "Antigravity Worker siap menjalankan eksekusi task berkecepatan tinggi.",
				CreatedAt:  now,
				UpdatedAt:  now,
			}); err != nil {
				log.Printf("[ensureDefaultAgentRooms] failed to create initial Antigravity message: %v", err)
			}
		}
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

		// Include any agents from s.agentRepo that are active and not already included
		if s.agentRepo != nil {
			if dbAgents, err := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{Status: db.AgentStatusActive}); err == nil {
				for _, a := range dbAgents {
					if a.ID == "agent_scrum_master" || a.ID == "agent_antigravity" || a.ID == defaultAntigravityRoomID {
						continue
					}
					roomID := ""
					lastMsg := "Sesi CLI terhubung tanpa API Key. Siap menerima tugas."
					lastAt := a.UpdatedAt
					if s.projectStudioRepo != nil {
						if allRooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil {
							for _, rm := range allRooms {
								if rm.Topic == "agent:"+a.ID || rm.Title == a.Name {
									roomID = rm.ID
									break
								}
							}
						}
						if roomID != "" {
							if msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), roomID); len(msgs) > 0 {
								lastMsg = msgs[len(msgs)-1].Content
								lastAt = msgs[len(msgs)-1].CreatedAt
							}
						}
					}
					if roomID == "" {
						roomID = db.GenerateUUID()
						if s.projectStudioRepo != nil {
							_ = s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
								ID:        roomID,
								ProjectID: "default",
								Title:     a.Name,
								Topic:     "agent:" + a.ID,
								CreatedAt: a.CreatedAt,
								UpdatedAt: a.UpdatedAt,
							})
						}
					}
					c := ChatContact{
						ID:             "contact_" + a.ID,
						Type:           "agent",
						Name:           a.Name,
						Avatar:         a.Avatar,
						Status:         "online",
						ConversationID: roomID,
						UnreadCount:    0,
						LastMessage:    lastMsg,
						LastMessageAt:  lastAt,
						UpdatedAt:      lastAt,
					}
					contacts = append(contacts, c)
				}
			}
		}
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
					if rm.ID != defaultScrumMasterRoomID && rm.ID != defaultAntigravityRoomID && !strings.Contains(strings.ToLower(rm.Title), "scrum") && !strings.HasPrefix(rm.Topic, "agent:") && !strings.Contains(rm.Topic, "Interactive Agent Chat") {
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

// ResolvedConversation holds the resolved room and target agent metadata.
type ResolvedConversation struct {
	RoomID          string
	TargetAgentID   string
	TargetAgentName string
	TargetHarness   string
	IsAgent         bool
}

// resolveConversationContext resolves any conversation identifier (room UUID, agent ID, contact alias, handle)
// into a valid room UUID in project_rooms and the target agent persona.
func (s *Server) resolveConversationContext(ctx context.Context, convID string, messageText string) ResolvedConversation {
	s.ensureDefaultAgentRooms(ctx)

	result := ResolvedConversation{
		RoomID:        strings.TrimSpace(convID),
		TargetHarness: "agy",
	}

	trimmedID := strings.TrimSpace(convID)

	// 1. Check Scrum Master shortcuts
	if trimmedID == defaultScrumMasterRoomID ||
		trimmedID == "contact_scrum_master" ||
		trimmedID == "scrum" ||
		trimmedID == "agent_scrum_master" ||
		trimmedID == "@sm" ||
		trimmedID == "sm" {
		result.RoomID = defaultScrumMasterRoomID
		result.TargetAgentID = "agent_scrum_master"
		result.TargetAgentName = "Scrum Master"
		result.TargetHarness = "agy"
		result.IsAgent = true
		return result
	}

	// 2. Check Antigravity Flash shortcuts
	if trimmedID == defaultAntigravityRoomID ||
		trimmedID == "contact_antigravity" ||
		trimmedID == "antigravity" ||
		trimmedID == "agent_antigravity" ||
		trimmedID == "@antigravity" {
		result.RoomID = defaultAntigravityRoomID
		result.TargetAgentID = "antigravity_flash"
		result.TargetAgentName = "Antigravity Flash"
		result.TargetHarness = "agy"
		result.IsAgent = true
		return result
	}

	// 3. Clean candidate agent ID from contact_ or agent_ prefix
	cleanAgentID := strings.TrimPrefix(trimmedID, "contact_")
	cleanAgentID = strings.TrimPrefix(cleanAgentID, "agent_")

	// 4. Check if convID matches an agent in agentRepo
	if s.agentRepo != nil {
		if ag, err := s.agentRepo.GetAgentByID(ctx, cleanAgentID); err == nil && ag != nil {
			result.TargetAgentID = ag.ID
			result.TargetAgentName = ag.Name
			result.TargetHarness = ag.Harness
			result.IsAgent = true
		} else if ags, err := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); err == nil {
			for _, a := range ags {
				handleClean := strings.ToLower(strings.TrimPrefix(a.Name, "@"))
				reqClean := strings.ToLower(strings.TrimPrefix(cleanAgentID, "@"))
				if a.ID == cleanAgentID || a.ID == trimmedID || handleClean == reqClean {
					result.TargetAgentID = a.ID
					result.TargetAgentName = a.Name
					result.TargetHarness = a.Harness
					result.IsAgent = true
					break
				}
			}
		}
	}

	// If resolved to an agent directly, find or create room for this agent
	if result.IsAgent {
		if s.projectStudioRepo != nil {
			var foundRoomID string
			if allRooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil {
				for _, rm := range allRooms {
					if rm.Topic == "agent:"+result.TargetAgentID || rm.Title == result.TargetAgentName {
						foundRoomID = rm.ID
						break
					}
				}
			}
			if foundRoomID == "" {
				foundRoomID = db.GenerateUUID()
				now := time.Now().UTC()
				_ = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
					ID:        foundRoomID,
					ProjectID: "default",
					Title:     result.TargetAgentName,
					Topic:     "agent:" + result.TargetAgentID,
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
			result.RoomID = foundRoomID
		}
		return result
	}

	// 5. If convID is a room UUID in projectStudioRepo
	if s.projectStudioRepo != nil {
		if rm, err := s.projectStudioRepo.GetRoomByID(ctx, trimmedID); err == nil && rm != nil {
			result.RoomID = rm.ID
			if strings.HasPrefix(rm.Topic, "agent:") {
				result.IsAgent = true
				result.TargetAgentID = strings.TrimPrefix(rm.Topic, "agent:")
				result.TargetAgentName = rm.Title
				if s.agentRepo != nil {
					if ag, err := s.agentRepo.GetAgentByID(ctx, result.TargetAgentID); err == nil && ag != nil {
						result.TargetHarness = ag.Harness
						if ag.Name != "" {
							result.TargetAgentName = ag.Name
						}
					}
				}
				return result
			} else if strings.Contains(rm.Topic, "Interactive Agent Chat") || strings.HasPrefix(rm.Title, "@") {
				result.IsAgent = true
				result.TargetAgentName = rm.Title
				result.TargetAgentID = "agent_" + strings.TrimPrefix(rm.Title, "@")
				return result
			}
		}
	}

	// 6. Check mention in message if any
	mention := extractMentionHandle(messageText)
	if mention != "" {
		if mention == "@sm" || mention == "@scrum" || strings.EqualFold(mention, "@scrum_master") {
			result.TargetAgentID = "agent_scrum_master"
			result.TargetAgentName = "Scrum Master"
			result.TargetHarness = "agy"
			result.IsAgent = true
		} else if s.agentRepo != nil {
			if ags, err := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); err == nil {
				for _, a := range ags {
					if strings.EqualFold(a.Name, mention) || strings.EqualFold("@"+strings.TrimPrefix(a.Name, "@"), mention) {
						result.TargetAgentID = a.ID
						result.TargetAgentName = a.Name
						result.TargetHarness = a.Harness
						result.IsAgent = true
						break
					}
				}
			}
		}
		if !result.IsAgent && strings.HasPrefix(mention, "@") {
			result.TargetAgentName = mention
			result.TargetAgentID = "agent_" + strings.TrimPrefix(mention, "@")
			result.TargetHarness = "agy"
			result.IsAgent = true
		}
	}

	// 7. Ensure RoomID is a valid UUID so SQL insert into project_messages never fails
	if !db.IsValidUUID(result.RoomID) {
		if s.projectStudioRepo != nil {
			var matchedRoomID string
			if allRooms, err := s.projectStudioRepo.ListAllRooms(ctx); err == nil {
				for _, rm := range allRooms {
					if rm.Title == trimmedID || rm.Topic == trimmedID {
						matchedRoomID = rm.ID
						break
					}
				}
			}
			if matchedRoomID == "" {
				matchedRoomID = db.GenerateUUID()
				now := time.Now().UTC()
				_ = s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
					ID:        matchedRoomID,
					ProjectID: "default",
					Title:     trimmedID,
					Topic:     "general",
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
			result.RoomID = matchedRoomID
		} else {
			result.RoomID = db.GenerateUUID()
		}
	}

	return result
}

// GET /api/v1/chat/conversations/{id}/messages
func (s *Server) handleInternalGetConversationMessages(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if convID == "" {
		respondJSONError(w, http.StatusBadRequest, "conversation id is required")
		return
	}

	resolved := s.resolveConversationContext(r.Context(), convID, "")

	var contact ChatContact
	if resolved.IsAgent {
		avatar := "🤖"
		if resolved.TargetAgentName == "Scrum Master" {
			avatar = "👔"
		} else if s.agentRepo != nil {
			if ag, err := s.agentRepo.GetAgentByID(r.Context(), resolved.TargetAgentID); err == nil && ag != nil && ag.Avatar != "" {
				avatar = ag.Avatar
			}
		}
		contact = ChatContact{
			ID:             "contact_" + resolved.TargetAgentID,
			Type:           "agent",
			Name:           resolved.TargetAgentName,
			Avatar:         avatar,
			Status:         "online",
			ConversationID: resolved.RoomID,
		}
	} else {
		// Customer contact
		globalChatManager.RLock()
		sess := globalChatManager.sessions[resolved.RoomID]
		globalChatManager.RUnlock()

		name := "Customer"
		product := ""
		guestUUID := ""
		if sess != nil {
			name = sess.Name
			product = sess.Product
			guestUUID = sess.GuestUUID
		} else if s.projectStudioRepo != nil {
			if rm, err := s.projectStudioRepo.GetRoomByID(r.Context(), resolved.RoomID); err == nil && rm != nil {
				name = rm.Title
				product = rm.Topic
			}
		}

		contact = ChatContact{
			ID:             "cust_" + resolved.RoomID,
			Type:           "customer",
			Name:           name,
			Avatar:         "👤",
			Status:         "online",
			Product:        product,
			GuestUUID:      guestUUID,
			ConversationID: resolved.RoomID,
		}
	}

	product := contact.Product
	messages := s.getFormattedMessages(r.Context(), resolved.RoomID, product)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"conversation_id": convID,
		"room_id":         resolved.RoomID,
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

	resolved := s.resolveConversationContext(r.Context(), convID, text)

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

	// Save the user message to the resolved room
	if s.projectStudioRepo != nil {
		if err := s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
			ID:         msgUUID,
			RoomID:     resolved.RoomID,
			AuthorType: authorType,
			AuthorID:   "operator_user",
			AuthorName: authorName,
			Content:    text,
			CreatedAt:  now,
			UpdatedAt:  now,
		}); err != nil {
			log.Printf("[handleInternalSendConversationMessage] failed to save user message: %v", err)
		}
	}

	var agentMsgDTO *ChatMessageDTO

	if resolved.IsAgent {
		role := ClassifyMessageContext(text)

		// Backward-compatible triage card for Scrum Master on technical task instructions
		if (resolved.RoomID == defaultScrumMasterRoomID || resolved.TargetAgentName == "Scrum Master") && isTechnicalTaskInstruction(text) {
			triage := generateSMTriageResponse(text)
			agentUUID := db.GenerateUUID()
			agentNow := now.Add(100 * time.Millisecond)

			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
					ID:         agentUUID,
					RoomID:     resolved.RoomID,
					AuthorType: "agent",
					AuthorID:   resolved.TargetAgentID,
					AuthorName: resolved.TargetAgentName,
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
				ApproveURL:     fmt.Sprintf("/api/v1/inbox/%s/approve", resolved.RoomID),
				ExecutionState: "ready",
			}

			agentMsgDTO = &ChatMessageDTO{
				ID:         2,
				UUID:       agentUUID,
				RoomID:     resolved.RoomID,
				SenderType: "operator",
				AuthorType: "agent",
				AuthorName: resolved.TargetAgentName,
				Message:    triage.Text,
				Content:    triage.Text,
				ActionCard: card,
				CreatedAt:  agentNow,
			}
		} else if (resolved.RoomID == defaultScrumMasterRoomID || resolved.TargetAgentName == "Scrum Master") && role == RoleFriendly {
			convReplyText := s.generateSMConversationalResponse(r.Context(), text)
			agentUUID := db.GenerateUUID()
			agentNow := now.Add(100 * time.Millisecond)

			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
					ID:         agentUUID,
					RoomID:     resolved.RoomID,
					AuthorType: "agent",
					AuthorID:   resolved.TargetAgentID,
					AuthorName: resolved.TargetAgentName,
					Content:    convReplyText,
					CreatedAt:  agentNow,
					UpdatedAt:  agentNow,
				})
			}

			agentMsgDTO = &ChatMessageDTO{
				ID:         2,
				UUID:       agentUUID,
				RoomID:     resolved.RoomID,
				SenderType: "operator",
				AuthorType: "agent",
				AuthorName: resolved.TargetAgentName,
				Message:    convReplyText,
				Content:    convReplyText,
				ActionCard: nil,
				CreatedAt:  agentNow,
			}
		} else {
			// Contextual response based on dynamic persona (Scrum Master, Developer, or Friendly)
			agentReplyText := s.executeAgentContextualResponse(r.Context(), resolved.TargetAgentName, resolved.TargetHarness, text, role)
			agentUUID := db.GenerateUUID()
			agentNow := now.Add(100 * time.Millisecond)

			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
					ID:         agentUUID,
					RoomID:     resolved.RoomID,
					AuthorType: "agent",
					AuthorID:   resolved.TargetAgentID,
					AuthorName: resolved.TargetAgentName,
					Content:    agentReplyText,
					CreatedAt:  agentNow,
					UpdatedAt:  agentNow,
				})
			}

			agentMsgDTO = &ChatMessageDTO{
				ID:         2,
				UUID:       agentUUID,
				RoomID:     resolved.RoomID,
				SenderType: "operator",
				AuthorType: "agent",
				AuthorName: resolved.TargetAgentName,
				Message:    agentReplyText,
				Content:    agentReplyText,
				CreatedAt:  agentNow,
			}
		}
	}

	userDTO := ChatMessageDTO{
		ID:         1,
		UUID:       msgUUID,
		RoomID:     resolved.RoomID,
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

// ChatContextRole defines the dynamic contextual persona an agent adopts in chat.
type ChatContextRole string

const (
	RoleScrumMaster ChatContextRole = "scrum_master"
	RoleDeveloper   ChatContextRole = "developer"
	RoleFriendly    ChatContextRole = "friendly"
)

// ClassifyMessageContext analyzes message text and returns the dynamic persona role:
// - RoleScrumMaster: sprint planning, review, evaluation, DoD, backlog, standup, coordination
// - RoleDeveloper: technical instructions, coding, bug fixing, endpoints, refactoring, unit tests
// - RoleFriendly: greetings, status inquiries, casual conversational phrases
func ClassifyMessageContext(msg string) ChatContextRole {
	raw := strings.ToLower(strings.TrimSpace(msg))
	cleaned := strings.TrimRight(raw, "?!.,;: \t\n\r")
	if cleaned == "" {
		return RoleFriendly
	}

	// 1. Check for Sprint / Scrum / Planning / Review / Evaluation / DoD keywords
	scrumKeywords := []string{
		"sprint", "planning", "review", "evaluasi", "eval", "retrospektif", "retro",
		"backlog", "dod", "definition of done", "standup", "daily standup",
		"scrum master", "scrum", "burndown", "velocity", "scope",
		"koordinasi tim", "koordinasi sprint", "prioritas backlog",
		"user story", "acceptance criteria",
	}
	for _, kw := range scrumKeywords {
		if strings.Contains(cleaned, kw) {
			return RoleScrumMaster
		}
	}

	// 2. Check for Technical / Developer / Coding / Fix keywords
	devKeywords := []string{
		"ngoding", "coding", "code", "koding", "fix", "perbaiki", "repair", "patch",
		"bug", "error", "issue", "crash", "hotfix", "atasi",
		"endpoint", "api", "route", "handler",
		"database", "sql", "migration", "migrasi", "tabel", "query", "foreign key",
		"refactor", "unit test", "test suite", "testing",
		"build", "compile", "deploy", "deployment", "ship", "rilis",
		"pty", "terminal", "git", "commit", "push",
		"buatkan fungsi", "buat fungsi", "bikin script", "script",
		"backend", "frontend", "react", "golang",
		"buatkan fitur", "buat fitur", "bikin fitur", "tambah fitur", "implementasi",
	}
	for _, kw := range devKeywords {
		if strings.Contains(cleaned, kw) {
			return RoleDeveloper
		}
	}

	// 3. Fallback / Casual Greeting & General Inquiry
	return RoleFriendly
}

// extractMentionHandle finds the first @handle mention in the message, e.g. "@dev" -> "@dev".
func extractMentionHandle(msg string) string {
	parts := strings.Fields(msg)
	for _, p := range parts {
		clean := strings.Trim(p, ",!?:;\"'()[]{}")
		if strings.HasPrefix(clean, "@") && len(clean) > 1 {
			return clean
		}
	}
	return ""
}

// executeAgentContextualResponse runs CLI or intelligent fallback to generate contextual response
// based on dynamic role (RoleScrumMaster, RoleDeveloper, RoleFriendly).
func (s *Server) executeAgentContextualResponse(ctx context.Context, agentName string, harness string, userPrompt string, role ChatContextRole) string {
	// Attempt CLI execution if binary is present and not testing/staging
	if s.cfg == nil || (s.cfg.Environment != "testing" && s.cfg.Environment != "staging") {
		binName := "agy"
		if harness == "claude" {
			binName = "claude"
		} else if harness == "codex" {
			binName = "codex"
		}

		binPath, err := exec.LookPath(binName)
		if err != nil && binName == "agy" {
			candidate := "/home/dev/.local/bin/agy"
			if _, statErr := os.Stat(candidate); statErr == nil {
				binPath = candidate
				err = nil
			}
		}

		if err == nil && binPath != "" {
			execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			var systemContext string
			switch role {
			case RoleScrumMaster:
				systemContext = fmt.Sprintf("Anda adalah %s di sistem Cuan. Dalam konteks ini, bertindaklah sebagai Scrum Master yang memfasilitasi koordinasi sprint, evaluasi backlog, Definition of Done (DoD), dan alur kerja tim. Pengguna mengatakan: \"%s\". Berikan respon profesional terarah dalam Bahasa Indonesia.", agentName, userPrompt)
			case RoleDeveloper:
				systemContext = fmt.Sprintf("Anda adalah %s di sistem Cuan. Dalam konteks ini, bertindaklah sebagai Developer handal yang berfokus pada implementasi teknis, coding, arsitektur, debugging, dan unit testing. Pengguna mengatakan: \"%s\". Berikan respon teknis yang solutif dalam Bahasa Indonesia.", agentName, userPrompt)
			default:
				systemContext = fmt.Sprintf("Anda adalah %s di sistem Cuan. Dalam konteks ini, bertindaklah ramah dan natural sebagai rekan kerja tim. Pengguna mengatakan: \"%s\". Berikan respon sapaan hangat, natural, dan ringkas (1-2 kalimat) dalam Bahasa Indonesia tanpa template kaku.", agentName, userPrompt)
			}

			cmd := exec.CommandContext(execCtx, binPath, "-p", systemContext, "--print-timeout", "12s")
			var stdoutBuf, stderrBuf bytes.Buffer
			cmd.Stdout = &stdoutBuf
			cmd.Stderr = &stderrBuf

			if cmdErr := cmd.Run(); cmdErr == nil {
				out := strings.TrimSpace(stdoutBuf.String())
				if out != "" {
					if !strings.Contains(out, agentName) && !strings.Contains(out, "Antigravity") {
						out = fmt.Sprintf("[%s] %s", agentName, out)
					}
					return out
				}
			}
		}
	}

	// Fallback intelligent response based on dynamic contextual role
	return getFallbackAgentContextualResponse(agentName, userPrompt, role)
}

func getFallbackAgentContextualResponse(agentName string, userPrompt string, role ChatContextRole) string {
	cleaned := strings.ToLower(strings.TrimSpace(userPrompt))
	switch role {
	case RoleScrumMaster:
		if strings.Contains(cleaned, "evaluasi") || strings.Contains(cleaned, "review") {
			return fmt.Sprintf("Halo Buchin! Saya %s bertindak sebagai Scrum Master. Terkait evaluasi dan review sprint: mari kita tinjau pencapaian Definition of Done (DoD), velocity tim, serta backlog yang telah terselesaikan untuk memastikan kualitas rilis.", agentName)
		}
		if strings.Contains(cleaned, "planning") || strings.Contains(cleaned, "perencanaan") {
			return fmt.Sprintf("Halo Buchin! Saya %s bertindak sebagai Scrum Master. Terkait sprint planning: mari kita prioritaskan backlog item, tentukan kapasitas tim, dan tetapkan kriteria Definition of Done (DoD) yang jelas.", agentName)
		}
		if strings.Contains(cleaned, "dod") || strings.Contains(cleaned, "definition of done") {
			return fmt.Sprintf("Halo Buchin! Saya %s sebagai Scrum Master. Definition of Done (DoD) mencakup: kode teruji (unit test pass 100%%), zero regression, clean code, dan dokumentasi audit yang lengkap.", agentName)
		}
		return fmt.Sprintf("Halo Buchin! Saya %s bertindak sebagai Scrum Master. Saya siap memfasilitasi koordinasi sprint, mengawal alur backlog, dan memastikan ritme kerja tim berjalan optimal.", agentName)

	case RoleDeveloper:
		if strings.Contains(cleaned, "bug") || strings.Contains(cleaned, "fix") || strings.Contains(cleaned, "error") || strings.Contains(cleaned, "crash") || strings.Contains(cleaned, "perbaiki") {
			return fmt.Sprintf("Halo Buchin! Saya %s bertindak sebagai Developer. Saya siap menganalisis root cause bug tersebut, memperbaiki kode pada komponen terkait, dan memastikan unit test lulus 100%%.", agentName)
		}
		if strings.Contains(cleaned, "ngoding") || strings.Contains(cleaned, "coding") || strings.Contains(cleaned, "code") || strings.Contains(cleaned, "fitur") || strings.Contains(cleaned, "endpoint") || strings.Contains(cleaned, "api") || strings.Contains(cleaned, "unit test") {
			return fmt.Sprintf("Halo Buchin! Saya %s bertindak sebagai Developer. Saya siap mengimplementasikan instruksi teknis ini, menyusun struktur kode yang bersih, dan menyiapkan endpoint/fungsi yang dibutuhkan.", agentName)
		}
		return fmt.Sprintf("Halo Buchin! Saya %s bertindak sebagai Developer. Instruksi teknis Anda telah saya catat dan siap dieksekusi dengan standar clean code dan automated tests.", agentName)

	default: // RoleFriendly
		if strings.Contains(cleaned, "kabar") {
			return fmt.Sprintf("Kabar baik dan luar biasa, Buchin! Saya %s, siap mendampingi Anda berkolaborasi hari ini. Ada yang bisa saya bantu?", agentName)
		}
		if strings.Contains(cleaned, "halo") || strings.Contains(cleaned, "hai") || strings.Contains(cleaned, "pagi") || strings.Contains(cleaned, "siang") || strings.Contains(cleaned, "sore") || strings.Contains(cleaned, "malam") {
			return fmt.Sprintf("Halo Buchin! Selamat beraktivitas. Saya %s di sini siap berkolaborasi bersama Anda.", agentName)
		}
		if strings.Contains(cleaned, "siapa kamu") || strings.Contains(cleaned, "kamu siapa") {
			return fmt.Sprintf("Saya %s, agen AI otonom di Cuan yang siap beradaptasi mendukung kebutuhan sprint maupun teknis Anda.", agentName)
		}
		if strings.Contains(cleaned, "terima kasih") || strings.Contains(cleaned, "makasih") || strings.Contains(cleaned, "thanks") {
			return fmt.Sprintf("Sama-sama, Buchin! Saya %s, senang bisa membantu Anda. Kabari saya jika butuh bantuan lagi.", agentName)
		}
		if strings.Contains(cleaned, "oke") || strings.Contains(cleaned, "ok") || strings.Contains(cleaned, "siap") || strings.Contains(cleaned, "mantap") {
			return fmt.Sprintf("Siap, dipahami Buchin! Saya %s selalu stand by.", agentName)
		}
		return fmt.Sprintf("Halo Buchin! Senang mengobrol dengan Anda. Saya %s, siap membantu kapan pun dibutuhkan.", agentName)
	}
}

// executeAgentCLIResponse runs the agent CLI runner (agy) or subprocess adapter and returns the agent text output.
func (s *Server) executeAgentCLIResponse(ctx context.Context, agentName string, userPrompt string) string {
	role := ClassifyMessageContext(userPrompt)
	return s.executeAgentContextualResponse(ctx, agentName, "agy", userPrompt, role)
}

// isTechnicalTaskInstruction determines whether a message contains explicit task/technical work instructions
// versus general greeting, inquiry, status question, or casual chat.
func isTechnicalTaskInstruction(msg string) bool {
	raw := strings.ToLower(strings.TrimSpace(msg))
	cleaned := strings.TrimRight(raw, "?!.,;: \t\n\r")
	if cleaned == "" {
		return false
	}

	// 1. Explicit casual conversation, greeting, status inquiry, social phrases
	casualPrefixes := []string{
		"apa kabar", "gimana kabar", "kabar baik", "bagaimana kabar", "bagaimana kabarmu", "kabar",
		"halo", "hai", "hello", "hi", "hey",
		"selamat pagi", "pagi", "selamat siang", "siang", "selamat sore", "sore", "selamat malam", "malam",
		"siapa kamu", "kamu siapa", "siapa anda", "anda siapa", "who are you",
		"terima kasih", "makasih", "thanks", "thank you",
		"oke", "ok", "siap", "mantap", "sip", "keren", "good", "nice",
		"bisa bantu apa", "apa yang bisa kamu lakukan", "apa fungsi kamu",
		"bagaimana sprint", "gimana sprint", "status sprint", "update sprint", "progress sprint", "progres sprint",
		"lagi apa", "sedang apa", "lagi ngapain",
	}

	for _, cp := range casualPrefixes {
		if cleaned == cp {
			return false
		}
		// If message is a pure greeting or short casual question
		if (strings.HasPrefix(cleaned, cp+" ") || strings.HasPrefix(cleaned, cp+"?")) && len(cleaned) < 35 {
			return false
		}
	}

	// 2. Clear technical action keywords
	taskKeywords := []string{
		"buat", "bikin", "ciptakan", "tambahkan", "tambah", "create", "build", "add",
		"perbaiki", "fix", "repair", "patch", "hotfix", "atasi",
		"bug", "error", "issue", "crash", "problem statement",
		"deploy", "deployment", "rilis", "release", "ship",
		"sprint backlog", "backlog", "task", "tugas",
		"fitur", "feature",
		"refactor", "migrasi", "migration", "database", "tabel", "foreign key",
		"endpoint", "api", "route", "handler",
		"implementasi", "implementasikan", "kerjakan", "eksekusi",
		"setup", "konfigurasi", "config", "test suite", "testing",
	}

	for _, kw := range taskKeywords {
		if strings.Contains(cleaned, kw) {
			return true
		}
	}

	// If longer instruction (over 50 chars) and not a casual question, treat as task
	if len(cleaned) > 50 && !strings.Contains(cleaned, "kabar") && !strings.Contains(cleaned, "siapa") {
		return true
	}

	return false
}

// generateSMConversationalResponse generates natural, friendly, and contextual responses for Scrum Master
// without action cards, rigid problem statements, or DoD templates.
func (s *Server) generateSMConversationalResponse(ctx context.Context, userPrompt string) string {
	if s.cfg != nil && (s.cfg.Environment == "testing" || s.cfg.Environment == "staging") {
		return getFallbackSMConversationResponse(userPrompt)
	}

	// Attempt agy execution with bounded timeout if CLI is present
	binPath, err := exec.LookPath("agy")
	if err != nil {
		candidate := "/home/dev/.local/bin/agy"
		if _, statErr := os.Stat(candidate); statErr == nil {
			binPath = candidate
			err = nil
		}
	}

	if err == nil && binPath != "" {
		execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		systemContext := fmt.Sprintf("Anda adalah Scrum Master cerdas, ramah, dan solutif di sistem Cuan. Pengguna (Product Owner: Buchin) mengatakan: \"%s\". Berikan balasan percakapan natural, hangat, dan ringkas dalam Bahasa Indonesia (2-3 kalimat saja) sebagai rekan kerja yang responsif. Jangan gunakan format Action Card, jangan gunakan template Problem Statement/DoD, dan jangan ada tombol.", userPrompt)
		cmd := exec.CommandContext(execCtx, binPath, "-p", systemContext, "--print-timeout", "4s")
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		if cmdErr := cmd.Run(); cmdErr == nil {
			out := strings.TrimSpace(stdoutBuf.String())
			if out != "" && !strings.Contains(out, "Problem Statement:") {
				out = strings.TrimPrefix(out, "[Scrum Master] ")
				return out
			}
		}
	}

	return getFallbackSMConversationResponse(userPrompt)
}

func getFallbackSMConversationResponse(userPrompt string) string {
	cleaned := strings.ToLower(strings.TrimSpace(userPrompt))
	cleaned = strings.TrimRight(cleaned, "?!.,;: \t\n\r")

	if strings.Contains(cleaned, "kabar") {
		return "Kabar baik dan luar biasa, Buchin! Saya Scrum Master Cuan, siap mendampingi Anda mengawal ritme sprint dan koordinasi tim agen. Ada yang ingin kita diskusikan atau rencanakan hari ini?"
	}
	if strings.Contains(cleaned, "halo") || strings.Contains(cleaned, "hai") || strings.Contains(cleaned, "hello") || strings.Contains(cleaned, "pagi") || strings.Contains(cleaned, "siang") || strings.Contains(cleaned, "sore") || strings.Contains(cleaned, "malam") {
		return "Halo Buchin! Selamat beraktivitas. Scrum Master di sini siap membantu mengelola backlog, memantau sprint, atau berkoordinasi dengan agen pekerja. Apa yang bisa saya bantu hari ini?"
	}
	if strings.Contains(cleaned, "siapa kamu") || strings.Contains(cleaned, "kamu siapa") || strings.Contains(cleaned, "siapa anda") {
		return "Saya Scrum Master di Cuan, asisten cerdas Anda yang bertugas memfasilitasi Agentic Scrum, memecah ide Anda menjadi Definition of Done (DoD) yang terukur, dan mengorkestrasi agen pekerja seperti Antigravity Flash untuk menyelesaikan sprint tepat waktu."
	}
	if strings.Contains(cleaned, "bagaimana sprint") || strings.Contains(cleaned, "gimana sprint") || strings.Contains(cleaned, "status sprint") || strings.Contains(cleaned, "update sprint") || strings.Contains(cleaned, "progress sprint") || strings.Contains(cleaned, "progres sprint") {
		return "Sprint saat ini berjalan aktif dengan tim yang terkoordinasi dengan baik! Seluruh backlog prioritas terpantau di Global Task Board (tab Tasks). Jika ada penyesuaian scope atau instruksi baru untuk tim pengembang, silakan beri tahu saya!"
	}
	if strings.Contains(cleaned, "terima kasih") || strings.Contains(cleaned, "makasih") || strings.Contains(cleaned, "thanks") {
		return "Sama-sama, Buchin! Senang bisa membantu kelancaran sprint Anda. Jangan ragu hubungi saya kembali jika butuh bantuan."
	}
	if strings.Contains(cleaned, "oke") || strings.Contains(cleaned, "ok") || strings.Contains(cleaned, "siap") || strings.Contains(cleaned, "mantap") || strings.Contains(cleaned, "sip") || strings.Contains(cleaned, "keren") {
		return "Siap, dipahami Buchin! Saya selalu stand by memantau progres tim. Beri tahu saya jika Anda ingin memulai task atau ada instruksi teknis baru."
	}
	if strings.Contains(cleaned, "bisa bantu apa") || strings.Contains(cleaned, "apa yang bisa kamu lakukan") || strings.Contains(cleaned, "apa fungsi kamu") {
		return "Sebagai Scrum Master cerdas di Cuan, saya dapat: 1) Berdiskusi santai mengenai arah produk, 2) Membedah kebutuhan fitur atau bug menjadi Problem Statement & DoD terukur, 3) Menugaskan Worker Agent untuk eksekusi otomatis. Cukup sampaikan ide atau instruksi Anda!"
	}

	return "Halo Buchin! Senang mengobrol dengan Anda. Sebagai Scrum Master, saya siap membantu kapan pun Anda membutuhkan arahan sprint, pembuatan backlog fitur, atau investigasi bug. Silakan beri tahu apa yang ingin kita capai hari ini!"
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
	raw := strings.TrimSpace(userMessage)
	title := raw
	lower := strings.ToLower(raw)
	for _, prefix := range []string{
		"halo scrum master, tolong siapkan sprint backlog untuk ",
		"halo scrum master, tolong buatkan ",
		"halo scrum master, tolong perbaiki ",
		"halo scrum master, ",
		"scrum master, tolong ",
		"tolong siapkan sprint backlog untuk ",
		"tolong buatkan ",
		"tolong perbaiki ",
		"tolong ",
		"buatkan ",
		"perbaiki ",
	} {
		if strings.HasPrefix(lower, prefix) {
			title = strings.TrimSpace(raw[len(prefix):])
			break
		}
	}

	if len(title) > 60 {
		title = title[:57] + "..."
	}
	if title == "" {
		title = raw
		if len(title) > 60 {
			title = title[:57] + "..."
		}
	}

	worker := "antigravity-flash"

	// Dynamic DoD based on user prompt topic
	lowerPrompt := strings.ToLower(raw)
	var dodList []db.DoDItem

	if strings.Contains(lowerPrompt, "bug") || strings.Contains(lowerPrompt, "error") || strings.Contains(lowerPrompt, "perbaiki") || strings.Contains(lowerPrompt, "fix") {
		dodList = []db.DoDItem{
			{Text: "Investigasi root cause & isolasi bug pada modul terkait", Completed: false},
			{Text: "Implementasikan perbaikan bug & patch di repositori", Completed: false},
			{Text: "Targeted automated test PASS & zero regression verifikasi", Completed: false},
		}
	} else if strings.Contains(lowerPrompt, "deploy") || strings.Contains(lowerPrompt, "vps") || strings.Contains(lowerPrompt, "rilis") || strings.Contains(lowerPrompt, "release") {
		dodList = []db.DoDItem{
			{Text: "Build frontend bundle & kompilasi binary produksi", Completed: false},
			{Text: "Sinkronisasi ke /opt/cuan di VPS target & restart service", Completed: false},
			{Text: "Public smoke test HTTP 200 OK & verifikasi live pass", Completed: false},
		}
	} else {
		dodList = []db.DoDItem{
			{Text: "Rancang spesifikasi teknis & breakdown task di repositori", Completed: false},
			{Text: "Implementasikan solusi fitur sesuai standar arsitektur", Completed: false},
			{Text: "Targeted automated unit test PASS & audit evidence", Completed: false},
		}
	}

	respText := fmt.Sprintf(`Halo Buchin! Saya telah menganalisis kebutuhan Anda:

**Problem Statement:**
%s

**Rencana Eksekusi & Definition of Done (DoD):**
1. %s
2. %s
3. %s

**Rekomendasi Worker Agent:** %s (Limit kuota mencukupi).
Silakan klik tombol **Mulai Kerjakan** di bawah untuk memulai eksekusi sprint.`, title, dodList[0].Text, dodList[1].Text, dodList[2].Text, worker)

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
