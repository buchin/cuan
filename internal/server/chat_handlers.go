package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	TaskDraft       *InboxTaskDraft  `json:"task_draft,omitempty"`
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

var inboxRoomResolutionMu sync.Mutex

var globalChatManager = &ChatSessionManager{
	sessions:       make(map[string]*LiveChatSession),
	sessionByGuest: make(map[string]*LiveChatSession),
}

const (
	defaultScrumMasterRoomID = "00000000-0000-4000-8000-000000000001"
	defaultAntigravityRoomID = "00000000-0000-4000-8000-000000000002"
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

// ensureDefaultAgentRooms retains the default project needed by registered agent conversations.
// It never creates example agents, agent rooms or welcome messages.
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

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	guestUUID := strings.TrimSpace(payload.GuestUUID)
	if guestUUID == "" {
		guestUUID = db.GenerateUUID()
	}

	email, err := normalizeCRMEmail(payload.Email)
	if err != nil || strings.HasPrefix(guestUUID, "manual_") || len(guestUUID) > 128 || len(payload.Product) > 100 || len(payload.Name) > 200 {
		respondJSONError(w, 400, "Identitas pengunjung tidak valid")
		return
	}
	product := strings.TrimSpace(payload.Product)
	name := strings.TrimSpace(payload.Name)
	contact, err := s.crmUpsert(r.Context(), guestUUID, product, name, email)
	if err != nil {
		respondJSONError(w, 503, "Sesi belum dapat disimpan")
		return
	}
	sessionUUID := contact.SessionUUID
	name = contact.Name
	if name == "" {
		name = "Visitor #" + contact.ID[:6]
	}
	email = contact.Email
	now := time.Now().UTC()
	liveSession := &LiveChatSession{
		UUID:        sessionUUID,
		GuestUUID:   guestUUID,
		Product:     strings.TrimSpace(payload.Product),
		Name:        name,
		Email:       email,
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
			if err := s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        sessionUUID,
				ProjectID: "default",
				Title:     name,
				Topic:     liveSession.Product,
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
				respondJSONError(w, 503, "Percakapan belum dapat disimpan")
				return
			}
		}

		// If initial message provided in session payload
		if text := strings.TrimSpace(payload.Message); text != "" {
			if err := s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
				ID:         db.GenerateUUID(),
				RoomID:     sessionUUID,
				AuthorType: "user",
				AuthorID:   guestUUID,
				AuthorName: name,
				Content:    text,
				CreatedAt:  now,
				UpdatedAt:  now,
			}); err != nil {
				respondJSONError(w, 503, "Pesan belum tersimpan")
				return
			}
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

	if _, err := s.crmSession(r.Context(), sessionUUID); err != nil {
		respondJSONError(w, 404, "Sesi tidak ditemukan")
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

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&payload); err != nil {
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

	sess, err := s.crmSession(r.Context(), sessionUUID)
	if err != nil {
		respondJSONError(w, 404, "Sesi tidak ditemukan; buka ulang live chat")
		return
	}
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
			if err := s.projectStudioRepo.CreateRoom(r.Context(), &db.ProjectRoom{
				ID:        sessionUUID,
				ProjectID: "default",
				Title:     authorName,
				Topic:     product,
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil {
				respondJSONError(w, 503, "Percakapan belum dapat disimpan")
				return
			}
		}

		if err := s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
			ID:         msgUUID,
			RoomID:     sessionUUID,
			AuthorType: "user",
			AuthorID:   sess.GuestUUID,
			AuthorName: authorName,
			Content:    text,
			CreatedAt:  now,
			UpdatedAt:  now,
		}); err != nil {
			respondJSONError(w, 503, "Pesan belum tersimpan")
			return
		}
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
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()
	s.ensureDefaultAgentRooms(r.Context())

	typeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	searchQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	contacts := make([]ChatContact, 0)

	if typeFilter != "customers" {
		// Include any agents from s.agentRepo that are active and not already included
		if s.agentRepo != nil {
			if dbAgents, err := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{Status: db.AgentStatusActive}); err == nil {
				for _, a := range dbAgents {
					if !a.Enabled {
						continue
					}
					roomID := ""
					lastMsg := "Belum ada percakapan."
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
						Status:         "idle",
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
		sessionList := []*LiveChatSession{}
		saved, err := s.crmList(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Percakapan pelanggan belum dapat dimuat")
			return
		}
		for _, c := range saved {
			if c.IsManual {
				continue
			}
			name := c.Name
			if name == "" {
				name = "Visitor #" + c.ID[:6]
			}
			sessionList = append(sessionList, &LiveChatSession{UUID: c.SessionUUID, GuestUUID: c.GuestUUID, Product: c.Product, Name: name, Email: c.Email, Status: "offline", CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt})
		}

		if s.projectStudioRepo != nil {
			known := map[string]bool{}
			for _, v := range sessionList {
				known[v.UUID] = true
			}
			if rooms, e := s.projectStudioRepo.ListAllRooms(r.Context()); e == nil {
				for _, rm := range rooms {
					if !known[rm.ID] && rm.ID != defaultScrumMasterRoomID && rm.ID != defaultAntigravityRoomID && !strings.Contains(strings.ToLower(rm.Title), "scrum") && !strings.HasPrefix(rm.Topic, "agent:") && !strings.Contains(rm.Topic, "Interactive Agent Chat") {
						sessionList = append(sessionList, &LiveChatSession{UUID: rm.ID, GuestUUID: rm.ID, Product: rm.Topic, Name: rm.Title, Status: "offline", CreatedAt: rm.CreatedAt, UpdatedAt: rm.UpdatedAt})
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
				Status:         "offline",
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
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()

	s.ensureDefaultAgentRooms(ctx)

	result := ResolvedConversation{
		RoomID:        strings.TrimSpace(convID),
		TargetHarness: "agy",
	}

	trimmedID := strings.TrimSpace(convID)

	// 3. Clean candidate agent ID from contact_ or agent_ prefix
	cleanAgentID := strings.TrimPrefix(trimmedID, "contact_")

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
			if strings.HasPrefix(rm.Topic, "agent:") && s.agentRepo != nil {
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
				if _, err := s.agentRepo.GetAgentByID(ctx, result.TargetAgentID); err == nil {
					return result
				}
				result.IsAgent = false
			}
		}
	}

	// 6. Check mention in message if any
	mention := extractMentionHandle(messageText)
	if mention != "" {
		if s.agentRepo != nil {
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
	if s.accountOwner(w, r) == "" {
		return
	}
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
	var requests []db.InboxRequest
	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		var err error
		requests, err = store.ListInboxRequests(r.Context(), resolved.RoomID)
		if err != nil {
			respondJSONError(w, 503, "Status Inbox belum dapat dibaca. Coba segarkan lagi.")
			return
		}
	}
	messages, err := s.getFormattedMessagesResult(r.Context(), resolved.RoomID, product)
	if err != nil {
		respondJSONError(w, 503, "Riwayat Inbox belum dapat dibaca. Coba lagi.")
		return
	}

	linked, err := s.inboxLinkedTasks(r.Context(), resolved.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Task percakapan belum dapat dibaca")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":           linked,
		"requests":        requests,
		"conversation_id": convID,
		"room_id":         resolved.RoomID,
		"contact":         contact,
		"messages":        messages,
	})
}

// POST /api/v1/chat/conversations/{id}/messages
func (s *Server) handleInternalSendConversationMessage(w http.ResponseWriter, r *http.Request) {
	ownerID := s.accountOwner(w, r)
	if ownerID == "" {
		return
	}
	var payload struct {
		AuthorType string `json:"author_type"`
		AuthorName string `json:"author_name"`
		Message    string `json:"message"`
		Content    string `json:"content"`
		RequestID  string `json:"request_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&payload) != nil {
		respondJSONError(w, 400, "Permintaan tidak valid")
		return
	}
	text := strings.TrimSpace(payload.Message)
	if text == "" {
		text = strings.TrimSpace(payload.Content)
	}
	if text == "" {
		respondJSONError(w, 400, "Pesan tidak boleh kosong")
		return
	}
	legacy := payload.RequestID == ""
	if legacy {
		payload.RequestID = db.GenerateUUID()
	}
	if !db.IsValidUUID(payload.RequestID) {
		respondJSONError(w, 400, "request_id harus UUID")
		return
	}
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		respondJSONError(w, 503, "Penyimpanan Inbox belum siap")
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), text)
	now := time.Now().UTC()
	q := db.InboxRequest{RoomID: resolved.RoomID, RequestID: payload.RequestID, PayloadHash: fmt.Sprintf("%x", sha256.Sum256([]byte(text))), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: ownerID, AgentID: resolved.TargetAgentID, State: "queued", CreatedAt: now, Deadline: now.Add(24 * time.Hour)}
	user := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", AuthorID: "operator_user", AuthorName: "Buchin", Content: text, CreatedAt: now, UpdatedAt: now}
	if !resolved.IsAgent {
		if kind, e := db.NormalizeAuthorType(payload.AuthorType); e == nil {
			user.AuthorType = kind
		}
		if name := strings.TrimSpace(payload.AuthorName); name != "" {
			user.AuthorName = name
		}
	}
	q, fresh, err := store.AcceptInboxRequest(r.Context(), q, user)
	if errors.Is(err, db.ErrInboxQueueFull) {
		respondJSONError(w, 429, "Antrean penuh (20 pesan). Tunggu atau batalkan pesan yang belum diproses.")
		return
	}
	if errors.Is(err, db.ErrInboxRequestConflict) {
		respondJSONError(w, 409, "ID permintaan sudah digunakan untuk pesan berbeda")
		return
	}
	if err != nil {
		respondJSONError(w, 503, "Pesan belum diterima. Penyimpanan Inbox belum siap; coba lagi dengan ID yang sama.")
		return
	}
	if fresh {
		s.publishInbox(q.RoomID)
		if legacy {
			s.drainInboxRoom(q.RoomID)
		} else {
			s.kickInboxRoom(q.RoomID)
		}
	}
	requests, err := store.ListInboxRequests(r.Context(), q.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Pesan telah diterima. Segarkan riwayat untuk memeriksa hasil; jangan buat ID baru.")
		return
	}
	for _, item := range requests {
		if item.RequestID == q.RequestID {
			q = item
		}
	}
	messages := s.getFormattedMessages(r.Context(), q.RoomID, "")
	response := map[string]any{"request": q}
	for _, m := range messages {
		if m.UUID == q.MessageID {
			response["message"] = m
		}
		if m.UUID == q.ReplyID {
			response["agent_reply"] = m
		}
	}
	status := http.StatusOK
	if q.State == "running" || q.State == "queued" {
		status = http.StatusAccepted
	}
	if legacy && q.State == "failed" {
		response["error"] = q.SafeError
		response["user_message_saved"] = true
		status = http.StatusBadGateway
	}
	respondJSON(w, status, response)
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

// executeAgentContextualResponse returns only actual CLI output; an empty result means provider failure.
// based on dynamic role (RoleScrumMaster, RoleDeveloper, RoleFriendly).
func (s *Server) executeAgentContextualResponse(ctx context.Context, agentName string, harness string, userPrompt string, role ChatContextRole, profileHomes ...string) string {
	out, _ := s.executeAgentInboxResponse(ctx, agentName, harness, userPrompt, role, profileHomes...)
	return out
}

func (s *Server) executeAgentInboxResponse(ctx context.Context, agentName string, harness string, userPrompt string, role ChatContextRole, profileHomes ...string) (string, string) {
	home, _ := os.UserHomeDir()
	if len(profileHomes) > 0 && profileHomes[0] != "" {
		home = profileHomes[0]
	}
	return s.executeInboxCLI(ctx, agentName, harness, userPrompt, home, "", nil)
}

func safeInboxProviderFailure(ctxErr error, raw string) string {
	lower := strings.ToLower(raw)
	if ctxErr != nil {
		return "Batas waktu provider tercapai. Pesan tersimpan; hasil provider belum pasti. Segarkan riwayat. Permintaan tidak dijalankan ulang otomatis."
	}
	if strings.Contains(lower, "quota") || strings.Contains(lower, "429") || strings.Contains(lower, "resource_exhausted") {
		return "Kuota provider sedang terbatas. Pesan tersimpan; tunggu kuota pulih. Tidak ada pengalihan akun otomatis."
	}
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "login") || strings.Contains(lower, "401") || strings.Contains(lower, "invalid_grant") {
		return "Login provider perlu diperiksa. Pesan tersimpan; buka status agen ini. Permintaan tidak dijalankan ulang otomatis."
	}
	return "Provider gagal memberikan balasan. Pesan tersimpan; periksa status agen ini lalu segarkan riwayat. Permintaan tidak dijalankan ulang otomatis."
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

	// Production widget must come from the same atomic release as the SPA.
	candidatePaths := []string{
		filepath.Join("web", "public", "chat", "widget.js"),
		filepath.Join("..", "..", "web", "public", "chat", "widget.js"),
		filepath.Join("web", "dist", "chat", "widget.js"),
	}

	if root := strings.TrimSpace(os.Getenv("CUAN_STATIC_DIR")); root != "" {
		candidatePaths = append([]string{filepath.Join(root, "chat", "widget.js")}, candidatePaths...)
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
	messages, _ := s.getFormattedMessagesResult(ctx, roomID, product)
	return messages
}
func (s *Server) getFormattedMessagesResult(ctx context.Context, roomID, product string) ([]ChatMessageDTO, error) {
	if s.projectStudioRepo == nil {
		return []ChatMessageDTO{}, nil
	}

	rawMsgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if len(rawMsgs) == 0 {
		return []ChatMessageDTO{}, nil
	}

	dtos := make([]ChatMessageDTO, len(rawMsgs))
	for i, m := range rawMsgs {
		senderType := "visitor"
		if m.AuthorType == "agent" || m.AuthorType == "system" {
			senderType = "operator"
		}

		var actionCard *AgentActionCard
		content, draft := splitInboxTaskDraft(m.AuthorType, m.Content)

		dtos[i] = ChatMessageDTO{
			ID:              int64(i + 1), // Monotonic numeric ID for Dojo widget
			UUID:            m.ID,
			ChatSessionUUID: roomID,
			RoomID:          roomID,
			SenderType:      senderType,
			AuthorType:      m.AuthorType,
			AuthorName:      m.AuthorName,
			Message:         content,
			Content:         content,
			Product:         product,
			ActionCard:      actionCard,
			TaskDraft:       draft,
			CreatedAt:       m.CreatedAt,
		}
	}

	return dtos, nil
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
