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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

// ChatContact represents a contact in the direct messenger hub (agent or customer).
type ChatContact struct {
	ResolutionObservedAt time.Time  `json:"resolution_observed_at"`
	Resolved             bool       `json:"resolved"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	ContactID            string     `json:"contact_id,omitempty"`
	AgentID              string     `json:"agent_id,omitempty"`
	AgentName            string     `json:"agent_name,omitempty"`
	ID                   string     `json:"id"`
	Type                 string     `json:"type"` // "agent" or "customer"
	Name                 string     `json:"name"`
	Email                string     `json:"email,omitempty"`
	Avatar               string     `json:"avatar"`
	Status               string     `json:"status"` // "online", "idle", "offline"
	Product              string     `json:"product,omitempty"`
	GuestUUID            string     `json:"guest_uuid,omitempty"`
	ConversationID       string     `json:"conversation_id"`
	UnreadCount          int        `json:"unread_count"`
	LastMessage          string     `json:"last_message,omitempty"`
	LastMessageAt        time.Time  `json:"last_message_at,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at"`
	HasMessages          bool       `json:"has_messages"`
	MessageCount         int        `json:"message_count"`
	IsOnline             bool       `json:"is_online"`
	CurrentURL           string     `json:"current_url,omitempty"`
	PageTitle            string     `json:"page_title,omitempty"`
	TaskCount            int        `json:"task_count"`
	ActiveTasks          int        `json:"active_tasks"`
	ReviewTasks          int        `json:"review_tasks"`
	QueuedTasks          int        `json:"queued_tasks"`
	DoneTasks            int        `json:"done_tasks"`
	FailedTasks          int        `json:"failed_tasks"`
	CanceledTasks        int        `json:"canceled_tasks"`
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
	RegenerateReplyID string           `json:"regenerate_reply_id,omitempty"`
	WorkUpdate        *inboxWorkUpdate `json:"work_update,omitempty"`
	db.MessageReceipt
	EmailDelivery      *ChatEmailDelivery       `json:"email_delivery,omitempty"`
	PlanDeferred       bool                     `json:"plan_deferred,omitempty"`
	PlanDeferRequestID string                   `json:"plan_defer_request_id,omitempty"`
	ApprovalRequestID  string                   `json:"approval_request_id,omitempty"`
	ApprovalAccepted   bool                     `json:"approval_accepted,omitempty"`
	ApprovalPlan       *inboxApprovalPlan       `json:"approval_plan,omitempty"`
	ApprovalTitle      string                   `json:"approval_title,omitempty"`
	PromptChoices      *InboxPromptChoices      `json:"prompt_choices,omitempty"`
	Attachments        []conversationAttachment `json:"attachments,omitempty"`
	ID                 int64                    `json:"id"`
	UUID               string                   `json:"uuid,omitempty"`
	ChatSessionUUID    string                   `json:"chat_session_uuid,omitempty"`
	RoomID             string                   `json:"room_id,omitempty"`
	SenderType         string                   `json:"sender_type"` // "visitor" or "operator"
	AuthorType         string                   `json:"author_type"` // "user", "agent", "system"
	AuthorID           string                   `json:"author_id,omitempty"`
	AuthorName         string                   `json:"author_name"`
	Message            string                   `json:"message"`
	Content            string                   `json:"content"`
	Product            string                   `json:"product,omitempty"`
	AttachmentURL      string                   `json:"attachment_url,omitempty"`
	AttachmentType     string                   `json:"attachment_type,omitempty"`
	AttachmentName     string                   `json:"attachment_name,omitempty"`
	AttachmentSize     int64                    `json:"attachment_size,omitempty"`
	HasAttachment      bool                     `json:"has_attachment,omitempty"`
	ActionCard         *AgentActionCard         `json:"action_card,omitempty"`
	TaskDraft          *InboxTaskDraft          `json:"task_draft,omitempty"`
	IsInternal         bool                     `json:"is_internal"`
	MentionHandle      string                   `json:"mention_handle,omitempty"`
	ReplyDraft         string                   `json:"reply_draft,omitempty"`
	CreatedAt          time.Time                `json:"created_at"`
	GroundingTastes    []GroundingTaste         `json:"grounding_tastes,omitempty"`
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
			VALUES ('default', 'Default', '', 'main', 'pending', 'custom', NOW(), NOW())
			ON CONFLICT (id) DO NOTHING;
		`)
	}
	if s.projectRepo != nil {
		if proj, err := s.projectRepo.GetProjectByID(ctx, "default"); err != nil || proj == nil {
			_ = s.projectRepo.CreateProject(ctx, &db.Project{
				ID:            "default",
				Name:          "Default",
				RepoURL:       "",
				DefaultBranch: "main",
				CloneStatus:   "pending",
				ProjectType:   "custom",
			})
		}
	}

	if s.db != nil && s.agentRepo != nil {
		_ = db.EnsureBuiltinAgents(ctx, s.agentRepo)
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
		respondJSONError(w, 400, "Invalid visitor identity")
		return
	}
	product := strings.TrimSpace(payload.Product)
	name := strings.TrimSpace(payload.Name)
	msgText := strings.TrimSpace(payload.Message)

	// Check if this visitor is already an existing known contact
	existingContact, findErr := s.crmFindByGuest(r.Context(), product, guestUUID)
	isKnownContact := findErr == nil && existingContact.ID != ""

	guestKey := product + "\x00" + guestUUID
	globalChatManager.RLock()
	existingLive := globalChatManager.sessionByGuest[guestKey]
	globalChatManager.RUnlock()

	var sessionUUID string
	var liveSession *LiveChatSession
	now := time.Now().UTC()

	// If visitor provides identity (name or email) OR an initial message OR is already known in CRM:
	if name != "" || email != "" || msgText != "" || isKnownContact {
		existingSessionUUID := ""
		if existingLive != nil && existingLive.Product == product {
			existingSessionUUID = existingLive.UUID
		}
		contact, err := s.crmUpsertWithSession(r.Context(), guestUUID, product, name, email, existingSessionUUID)
		if err != nil {
			respondJSONError(w, 503, "Could not save the session")
			return
		}
		sessionUUID = contact.SessionUUID
		name = contact.Name
		if name == "" {
			name = "Visitor #" + contact.ID[:6]
		}
		email = contact.Email
		liveSession = &LiveChatSession{
			UUID:        sessionUUID,
			GuestUUID:   guestUUID,
			Product:     product,
			Name:        name,
			Email:       email,
			CurrentURL:  payload.CurrentURL,
			PageTitle:   payload.PageTitle,
			ReferrerURL: payload.ReferrerURL,
			Status:      "active",
			CreatedAt:   contact.CreatedAt,
			UpdatedAt:   now,
		}

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
					respondJSONError(w, 503, "Could not save the conversation")
					return
				}
			}

			// If initial message provided in session payload
			if msgText != "" {
				initMsgID := db.GenerateUUID()
				if err := s.projectStudioRepo.CreateMessage(r.Context(), &db.ProjectMessage{
					ID:         initMsgID,
					RoomID:     sessionUUID,
					AuthorType: "user",
					AuthorID:   guestUUID,
					AuthorName: name,
					Content:    msgText,
					CreatedAt:  now,
					UpdatedAt:  now,
				}); err != nil {
					respondJSONError(w, 503, "Message could not be saved")
					return
				}
				s.DispatchCustomerMessagePush(r.Context(), sessionUUID, initMsgID, name, msgText)
			}
		}
	} else {
		// Transient anonymous visitor session: do not write to crm_contacts or project_rooms yet.
		if existingLive != nil && existingLive.Product == product {
			sessionUUID = existingLive.UUID
			name = existingLive.Name
			liveSession = existingLive
			liveSession.CurrentURL = payload.CurrentURL
			liveSession.PageTitle = payload.PageTitle
			liveSession.ReferrerURL = payload.ReferrerURL
			liveSession.UpdatedAt = now
			liveSession.Status = "active"
		} else {
			sessionUUID = db.GenerateUUID()
			name = "Visitor #" + sessionUUID[:6]
			liveSession = &LiveChatSession{
				UUID:        sessionUUID,
				GuestUUID:   guestUUID,
				Product:     product,
				Name:        name,
				Email:       "",
				CurrentURL:  payload.CurrentURL,
				PageTitle:   payload.PageTitle,
				ReferrerURL: payload.ReferrerURL,
				Status:      "active",
				CreatedAt:   now,
				UpdatedAt:   now,
			}
		}
	}

	globalChatManager.Lock()
	globalChatManager.sessions[sessionUUID] = liveSession
	globalChatManager.sessionByGuest[guestKey] = liveSession
	globalChatManager.Unlock()

	// Fetch existing messages for session (filtering out internal whisper/bot messages)
	rawMessages := s.getFormattedMessages(r.Context(), sessionUUID, liveSession.Product)
	messages := make([]ChatMessageDTO, 0, len(rawMessages))
	for _, m := range rawMessages {
		if !m.IsInternal {
			messages = append(messages, m)
		}
	}

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
		respondJSONError(w, 404, "Session not found")
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
	receipts := make([]map[string]any, 0)
	for _, m := range allMessages {
		if m.ID > afterID && !m.IsInternal {
			filtered = append(filtered, m)
		}
		if !m.IsInternal && m.SenderType == "visitor" && (m.DeliveredAt != nil || m.ReadConfirmedAt != nil) {
			receipts = append(receipts, map[string]any{"uuid": m.UUID, "id": m.ID, "delivered_at": m.DeliveredAt, "read_confirmed_at": m.ReadConfirmedAt})
		}
	}
	if len(receipts) > 500 {
		receipts = receipts[len(receipts)-500:]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"messages": filtered,
		"receipts": receipts,
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

	if payload.AttachmentURL != "" {
		u := strings.TrimRight(payload.AttachmentURL, "/")
		parts := strings.Split(u, "/")
		lastPart := parts[len(parts)-1]
		if db.IsValidUUID(lastPart) {
			text = encodeAttachmentMessage(text, []string{lastPart})
		}
	}

	sess, err := s.crmSession(r.Context(), sessionUUID)
	if err != nil {
		respondJSONError(w, 404, "Session not found. Reopen live chat")
		return
	}
	authorName := "Customer"
	product := ""
	if sess != nil {
		authorName = sess.Name
		product = sess.Product
	}

	// Promote transient session to persistent CRM contact on first message
	if sess != nil {
		if _, findErr := s.crmFindByGuest(r.Context(), product, sess.GuestUUID); findErr != nil {
			contact, upsertErr := s.crmUpsertWithSession(r.Context(), sess.GuestUUID, product, sess.Name, sess.Email, sess.UUID)
			if upsertErr == nil {
				if sess.Name == "" || strings.HasPrefix(sess.Name, "Visitor #") {
					authorName = "Visitor #" + contact.ID[:6]
					sess.Name = authorName
				}
			}
		}
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
				respondJSONError(w, 503, "Could not save the conversation")
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
			respondJSONError(w, 503, "Message could not be saved")
			return
		}

		s.DispatchCustomerMessagePush(r.Context(), sessionUUID, msgUUID, authorName, text)
		s.OnCustomerChatMessageReceived(sessionUUID, msgUUID)
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
			AuthorID:        sess.GuestUUID,
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
	s.handleChatReceipts(w, r, false)
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

	preferences := itemPreferences{}
	if s.authSvc != nil {
		if owner, e := s.authSvc.ValidateToken(r.Context(), auth.ExtractToken(r)); e == nil {
			itemPreferencesMu.Lock()
			saved, readError := readItemPreferences(owner.ID)
			itemPreferencesMu.Unlock()
			if readError != nil {
				respondJSONError(w, 503, "Could not load conversation state. Try again.")
				return
			}
			preferences = saved
		}
	}
	countUnread := func(id string, messages []db.ProjectMessage, isAgent bool) int {
		state := preferences["conversation:"+id]
		cutoff, _ := time.Parse(time.RFC3339Nano, state.ReadAt)
		count := 0
		for _, message := range messages {
			if internalInboxReviewInput(message) || !message.CreatedAt.After(cutoff) {
				continue
			}
			if isAgent {
				if message.AuthorType == "agent" || message.AuthorType == "system" {
					count++
				} else if message.AuthorType == "user" {
					count = 0
				}
			} else {
				if message.AuthorType == "user" {
					count++
				} else if message.AuthorType == "agent" || message.AuthorType == "system" {
					count = 0
				}
			}
		}
		if state.Unread && count == 0 {
			return 1
		}
		return count
	}
	contacts := make([]ChatContact, 0)

	// Count active tasks and total tasks per conversation room
	counts, err := s.readTaskCounts(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Could not load conversation task progress. Try again.")
		return
	}
	resolutionObserved := map[string]time.Time{}
	observeResolution := func(id string, messages []db.ProjectMessage) {
		for _, message := range messages {
			if message.CreatedAt.After(resolutionObserved[id]) {
				resolutionObserved[id] = message.CreatedAt
			}
		}
	}
	getRoomTaskCounts := func(ids ...string) (int, int, int, int, int, int, int) {
		for _, id := range ids {
			if c, ok := counts.Rooms[id]; ok {
				return c.Total, c.Active, c.Queued, c.Done, c.Failed, c.Review, c.Canceled
			}
		}
		return 0, 0, 0, 0, 0, 0, 0
	}

	if typeFilter != "customers" {
		// Include any agents from s.agentRepo that are active and not already included
		if s.agentRepo != nil {
			if dbAgents, err := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{Status: db.AgentStatusActive}); err == nil {
				hasCuanAgent := false
				for _, a := range dbAgents {
					if a.Enabled && (a.ID == db.AgentIDCuan || a.Name == db.DefaultAgentHandle) {
						hasCuanAgent = true
						break
					}
				}
				for _, a := range dbAgents {
					if !a.Enabled {
						continue
					}
					// Single Agent Facade: when @cuan is active, hide internal legacy agents from contacts list
					if hasCuanAgent && a.ID != db.AgentIDCuan {
						continue
					}
					roomID := ""
					lastMsg := "No conversations yet."
					lastAt := a.UpdatedAt
					unread := 0
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
								observeResolution(roomID, msgs)
								msgs = visibleInboxMessages(msgs)
								if len(msgs) > 0 {
									lastMsg = msgs[len(msgs)-1].Content
									lastAt = msgs[len(msgs)-1].CreatedAt
									unread = countUnread(roomID, msgs, true)
								}
							}
						}
					}
					if roomID == "" {
						// Inbox lists existing conversations. Starting a chat belongs
						// to New Work; reading the list must not recreate deleted rooms.
						continue
					}
					tc, at, qt, dt, ft, rt, ct := getRoomTaskCounts(roomID, "contact_"+a.ID, a.ID)
					c := ChatContact{
						ID:      "contact_" + a.ID,
						AgentID: a.ID, AgentName: a.Name,
						Type:           "agent",
						Name:           a.Name,
						Avatar:         a.Avatar,
						Status:         "idle",
						ConversationID: roomID,
						UnreadCount:    unread,
						LastMessage:    attachmentPreview(lastMsg),
						LastMessageAt:  lastAt,
						UpdatedAt:      lastAt,
						HasMessages:    true,
						MessageCount:   1,
						IsOnline:       true,
						TaskCount:      tc,
						ActiveTasks:    at,
						QueuedTasks:    qt,
						DoneTasks:      dt,
						FailedTasks:    ft,
						CanceledTasks:  ct,
						ReviewTasks:    rt,
					}
					contacts = append(contacts, c)
				}
			}
		}
	}

	if typeFilter != "customers" && s.projectStudioRepo != nil && s.agentRepo != nil {
		if rooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil {
			for _, room := range rooms {
				if !strings.HasPrefix(room.Topic, "agent-topic:") {
					continue
				}
				a, err := s.agentRepo.GetAgentByID(r.Context(), roomAgentID(room.Topic))
				if err != nil || a == nil || !a.Enabled {
					continue
				}
				last := "No messages yet."
				at := room.UpdatedAt
				unread := 0
				if messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), room.ID); err == nil && len(messages) > 0 {
					observeResolution(room.ID, messages)
					messages = visibleInboxMessages(messages)
					if len(messages) > 0 {
						last = messages[len(messages)-1].Content
						at = messages[len(messages)-1].CreatedAt
						unread = countUnread(room.ID, messages, true)
					}
				}
				tc, activeCnt, queuedCnt, doneCnt, failedCnt, reviewCnt, canceledCnt := getRoomTaskCounts(room.ID, "topic_"+room.ID, room.Topic)
				contacts = append(contacts, ChatContact{ID: "topic_" + room.ID, AgentID: a.ID, AgentName: a.Name, Type: "agent", Name: room.Title, Avatar: a.Avatar, Status: "idle", ConversationID: room.ID, UnreadCount: unread, LastMessage: attachmentPreview(last), LastMessageAt: at, UpdatedAt: at, HasMessages: true, MessageCount: 1, IsOnline: true, TaskCount: tc, ActiveTasks: activeCnt, QueuedTasks: queuedCnt, DoneTasks: doneCnt, FailedTasks: failedCnt, ReviewTasks: reviewCnt, CanceledTasks: canceledCnt})
			}
		}
	}

	// 3. Customer Live Chat Sessions (from Dojo Live Chat visitors)
	if typeFilter != "agents" {
		sessionList := []*LiveChatSession{}
		contactIDs := map[string]string{}
		saved, err := s.crmList(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Could not load customer conversations")
			return
		}
		for _, c := range saved {
			if c.IsManual {
				continue
			}
			contactIDs[c.SessionUUID] = c.ID
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
					if !known[rm.ID] && rm.ID != defaultScrumMasterRoomID && rm.ID != defaultAntigravityRoomID && !strings.Contains(strings.ToLower(rm.Title), "scrum") && roomAgentID(rm.Topic) == "" && !strings.Contains(rm.Topic, "Interactive Agent Chat") {
						sessionList = append(sessionList, &LiveChatSession{UUID: rm.ID, GuestUUID: rm.ID, Product: rm.Topic, Name: rm.Title, Status: "offline", CreatedAt: rm.CreatedAt, UpdatedAt: rm.UpdatedAt})
					}
				}
			}
		}

		sort.Slice(sessionList, func(i, j int) bool {
			return sessionList[i].UpdatedAt.After(sessionList[j].UpdatedAt)
		})

		for _, sess := range sessionList {
			lastMsg := ""
			lastAt := sess.UpdatedAt
			unread := 0
			hasMessages := false
			msgCount := 0

			if s.projectStudioRepo != nil {
				if msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), sess.UUID); len(msgs) > 0 {
					observeResolution(sess.UUID, msgs)
					msgs = visibleInboxMessages(msgs)
					if len(msgs) > 0 {
						msgCount = len(msgs)
						hasMessages = true
						lastMsg = msgs[len(msgs)-1].Content
						lastAt = msgs[len(msgs)-1].CreatedAt
						unread = countUnread(sess.UUID, msgs, false)
					}
				}
			}

			globalChatManager.RLock()
			liveSess := globalChatManager.sessions[sess.UUID]
			isOnline := false
			currURL := ""
			pageTitle := ""
			if liveSess != nil {
				currURL = liveSess.CurrentURL
				pageTitle = liveSess.PageTitle
				if liveSess.Status == "active" && time.Since(liveSess.UpdatedAt) <= 5*time.Minute {
					isOnline = true
				}
			}
			globalChatManager.RUnlock()

			// Skip sessions that have no messages AND are no longer online to prevent clutter.
			if !hasMessages && !isOnline {
				continue
			}

			status := "offline"
			if isOnline {
				status = "online"
			}

			if lastMsg == "" {
				if isOnline {
					if pageTitle != "" {
						lastMsg = "Viewing: " + pageTitle
					} else if currURL != "" {
						lastMsg = "Viewing: " + currURL
					} else {
						lastMsg = "Online on website"
					}
				} else {
					lastMsg = "A new conversation session has started."
				}
			}

			tc, at, qt, dt, ft, rt, ct := getRoomTaskCounts(sess.UUID, "cust_"+sess.UUID)
			contactEmail := sess.Email
			if contactEmail == "" && liveSess != nil && liveSess.Email != "" {
				contactEmail = liveSess.Email
			}
			if contactEmail == "" {
				if crmS, err := s.crmSession(r.Context(), sess.UUID); err == nil && crmS != nil && crmS.Email != "" {
					contactEmail = crmS.Email
				}
			}

			custContact := ChatContact{
				ContactID:      contactIDs[sess.UUID],
				ID:             "cust_" + sess.UUID,
				Type:           "customer",
				Name:           sess.Name,
				Email:          contactEmail,
				Avatar:         "👤",
				Status:         status,
				Product:        sess.Product,
				GuestUUID:      sess.GuestUUID,
				ConversationID: sess.UUID,
				UnreadCount:    unread,
				LastMessage:    attachmentPreview(lastMsg),
				LastMessageAt:  lastAt,
				UpdatedAt:      lastAt,
				HasMessages:    hasMessages,
				MessageCount:   msgCount,
				IsOnline:       isOnline,
				CurrentURL:     currURL,
				PageTitle:      pageTitle,
				TaskCount:      tc,
				ActiveTasks:    at,
				QueuedTasks:    qt,
				DoneTasks:      dt,
				FailedTasks:    ft,
				CanceledTasks:  ct,
				ReviewTasks:    rt,
			}
			contacts = append(contacts, custContact)
		}
	}

	if s.agentRepo != nil {
		if cuan, err := s.agentRepo.GetAgentByID(r.Context(), db.AgentIDCuan); err == nil && cuan != nil && cuan.Enabled {
			for i := range contacts {
				if contacts[i].Type == "agent" {
					if strings.HasPrefix(contacts[i].ID, "contact_") {
						contacts[i].Name = cuan.Name
					}
					contacts[i].AgentID, contacts[i].AgentName, contacts[i].Avatar = cuan.ID, cuan.Name, cuan.Avatar
				}
			}
		}
	}
	// Filter by search query if specified
	if searchQuery != "" {
		filtered := make([]ChatContact, 0)
		for _, c := range contacts {
			if strings.Contains(strings.ToLower(c.Name+" "+c.AgentName+" "+c.Email), searchQuery) ||
				strings.Contains(strings.ToLower(c.LastMessage), searchQuery) ||
				strings.Contains(strings.ToLower(c.Product), searchQuery) {
				filtered = append(filtered, c)
			}
		}
		contacts = filtered
	}

	// Shared resolution is independent from each operator's read marker.
	if s.projectStudioRepo != nil {
		rooms, err := s.projectStudioRepo.ListAllRooms(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Could not load conversation status")
			return
		}
		byID := make(map[string]db.ProjectRoom, len(rooms))
		for _, room := range rooms {
			byID[room.ID] = room
		}
		for i := range contacts {
			room := byID[contacts[i].ConversationID]
			contacts[i].ResolvedAt = room.ResolvedAt
			observed := resolutionObserved[contacts[i].ConversationID]
			if observed.IsZero() {
				observed = contacts[i].LastMessageAt
			}
			contacts[i].ResolutionObservedAt = observed
			contacts[i].Resolved = room.IsResolved(observed) && contacts[i].ActiveTasks == 0 && contacts[i].QueuedTasks == 0 && contacts[i].ReviewTasks == 0 && contacts[i].FailedTasks == 0 && contacts[i].TaskCount == contacts[i].DoneTasks+contacts[i].CanceledTasks
		}
	}
	for i := range contacts {
		if preferences["conversation:"+contacts[i].ConversationID].Unread && contacts[i].UnreadCount == 0 {
			contacts[i].UnreadCount = 1
		}
	}
	sort.SliceStable(contacts, func(i, j int) bool { return contacts[i].UpdatedAt.After(contacts[j].UpdatedAt) })
	// UI consumers need one representation, not a duplicate compatibility alias.
	// Keep the legacy envelope for existing CLI/integration consumers.
	if r.URL.Query().Get("view") == "list" {
		respondJSON(w, http.StatusOK, map[string]any{"contacts": contacts, "total": len(contacts)})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"contacts":      contacts,
		"conversations": contacts,
		"total":         len(contacts),
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
func (s *Server) resolveConversationContext(ctx context.Context, convID string, messageText string) (result ResolvedConversation) {
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()

	// Legacy rooms keep their IDs/history, while new turns use the public identity.
	defer func() {
		if result.IsAgent && s.agentRepo != nil {
			if a, err := s.agentRepo.GetAgentByID(ctx, db.AgentIDCuan); err == nil && a != nil && a.Enabled {
				result.TargetAgentID, result.TargetAgentName = a.ID, a.Name
				result.TargetHarness = db.NormalizeHarness(a.Harness, a.Provider)
			}
		}
	}()
	s.ensureDefaultAgentRooms(ctx)

	result = ResolvedConversation{
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
		result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
		return result
	}

	// 5. If convID is a room UUID in projectStudioRepo
	if s.projectStudioRepo != nil {
		if rm, err := s.projectStudioRepo.GetRoomByID(ctx, trimmedID); err == nil && rm != nil {
			result.RoomID = rm.ID
			if roomAgentID(rm.Topic) != "" && s.agentRepo != nil {
				result.IsAgent = true
				result.TargetAgentID = roomAgentID(rm.Topic)
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
					result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
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
				var cuanAgent *db.Agent
				for _, a := range ags {
					if a.Enabled && (a.ID == db.AgentIDCuan || strings.EqualFold(a.Name, db.DefaultAgentHandle)) {
						cuanCopy := a
						cuanAgent = &cuanCopy
						break
					}
				}

				mLower := strings.ToLower(mention)
				// If cuanAgent is active, legacy team/bot handles map to cuanAgent
				if cuanAgent != nil && (mLower == "@cuan" || mLower == "cuan" || mLower == "@team" || mLower == "@sm" || mLower == "@dev" || mLower == "@support") {
					result.TargetAgentID = cuanAgent.ID
					result.TargetAgentName = cuanAgent.Name
					result.TargetHarness = cuanAgent.Harness
					result.IsAgent = true
				}

				if !result.IsAgent {
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

				// Special reserved handles: @team or @sm (Scrum Master) or @dev (Developer) when cuanAgent is not present
				if !result.IsAgent {
					if mLower == "@team" || mLower == "@sm" {
						for _, a := range ags {
							roleLower := strings.ToLower(a.Role)
							nameLower := strings.ToLower(a.Name)
							if strings.Contains(roleLower, "scrum") || strings.Contains(nameLower, "scrum") || strings.Contains(nameLower, "sm") {
								result.TargetAgentID = a.ID
								result.TargetAgentName = a.Name
								result.TargetHarness = a.Harness
								result.IsAgent = true
								break
							}
						}
						// If still not matched, pick first active enabled agent for @team
						if !result.IsAgent {
							for _, a := range ags {
								if a.Enabled {
									result.TargetAgentID = a.ID
									result.TargetAgentName = a.Name
									result.TargetHarness = a.Harness
									result.IsAgent = true
									break
								}
							}
						}
					} else if mLower == "@dev" {
						for _, a := range ags {
							roleLower := strings.ToLower(a.Role)
							nameLower := strings.ToLower(a.Name)
							if strings.Contains(roleLower, "dev") || strings.Contains(nameLower, "dev") || strings.Contains(nameLower, "worker") {
								result.TargetAgentID = a.ID
								result.TargetAgentName = a.Name
								result.TargetHarness = a.Harness
								result.IsAgent = true
								break
							}
						}
					} else if mLower == "@support" {
						for _, a := range ags {
							roleLower := strings.ToLower(a.Role)
							nameLower := strings.ToLower(a.Name)
							if strings.Contains(roleLower, "support") || strings.Contains(nameLower, "support") {
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
		}
		// Fallback for default @cuan or Scrum Master if not yet registered in agentRepo
		if !result.IsAgent {
			mLower := strings.ToLower(mention)
			if mLower == "@cuan" || mLower == "cuan" || mLower == "@team" || mLower == "@sm" {
				result.TargetAgentID = db.AgentIDCuan
				result.TargetAgentName = db.DefaultAgentHandle
				result.TargetHarness = "agy"
				result.IsAgent = true
			} else if mLower == "@dev" {
				result.TargetAgentID = db.AgentIDCuan
				result.TargetAgentName = db.DefaultAgentHandle
				result.TargetHarness = "agy"
				result.IsAgent = true
			} else if mLower == "@support" {
				result.TargetAgentID = db.AgentIDCuan
				result.TargetAgentName = db.DefaultAgentHandle
				result.TargetHarness = "agy"
				result.IsAgent = true
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

	result.TargetHarness = db.NormalizeHarness(result.TargetHarness, "")
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
			ID:      "contact_" + resolved.TargetAgentID,
			AgentID: resolved.TargetAgentID, AgentName: resolved.TargetAgentName,
			Type:           "agent",
			Name:           resolved.TargetAgentName,
			Avatar:         avatar,
			Status:         "online",
			ConversationID: resolved.RoomID,
		}
		if room, e := s.projectStudioRepo.GetRoomByID(r.Context(), resolved.RoomID); e == nil && room != nil && strings.HasPrefix(room.Topic, "agent-topic:") {
			contact.ID = "topic_" + room.ID
			contact.Name = room.Title
		}
	} else {
		// Customer contact
		globalChatManager.RLock()
		sess := globalChatManager.sessions[resolved.RoomID]
		globalChatManager.RUnlock()

		name := "Customer"
		product := ""
		guestUUID := ""
		email := ""
		if sess != nil {
			name = sess.Name
			product = sess.Product
			guestUUID = sess.GuestUUID
			email = sess.Email
		} else if s.projectStudioRepo != nil {
			if rm, err := s.projectStudioRepo.GetRoomByID(r.Context(), resolved.RoomID); err == nil && rm != nil {
				name = rm.Title
				product = rm.Topic
			}
		}

		contactID := ""
		if crmSess, err := s.crmContactForSession(r.Context(), resolved.RoomID); err == nil {
			contactID = crmSess.ID
			if name == "" || name == "Customer" {
				name = crmSess.Name
			}
			if product == "" {
				product = crmSess.Product
			}
			if guestUUID == "" {
				guestUUID = crmSess.GuestUUID
			}
			if email == "" {
				email = crmSess.Email
			}
		}

		contact = ChatContact{
			ID:             "cust_" + resolved.RoomID,
			ContactID:      contactID,
			Type:           "customer",
			Name:           name,
			Email:          email,
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
			respondJSONError(w, 503, "Could not read Inbox status. Refresh again.")
			return
		}
	}
	messages, err := s.getFormattedMessagesResult(r.Context(), resolved.RoomID, product)
	if err != nil {
		respondJSONError(w, 503, "Could not read Inbox history. Try again.")
		return
	}

	limit := 50
	if l := strings.TrimSpace(r.URL.Query().Get("limit")); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 200 {
				limit = 200
			}
		} else if l == "all" || l == "-1" {
			limit = 0
		}
	} else if r.URL.Query().Get("all") == "1" {
		limit = 0
	}
	beforeID := strings.TrimSpace(r.URL.Query().Get("before_id"))
	if beforeID == "" {
		beforeID = strings.TrimSpace(r.URL.Query().Get("before"))
	}

	hasMore := false
	if limit > 0 && len(messages) > limit {
		if beforeID != "" {
			idx := -1
			for i, m := range messages {
				if m.UUID == beforeID || strconv.FormatInt(m.ID, 10) == beforeID {
					idx = i
					break
				}
			}
			if idx > 0 {
				start := idx - limit
				if start < 0 {
					start = 0
				}
				hasMore = start > 0
				messages = messages[start:idx]
			} else {
				hasMore = false
				messages = []ChatMessageDTO{}
			}
		} else {
			hasMore = len(messages) > limit
			messages = messages[len(messages)-limit:]
		}
	}

	linked, err := s.inboxLinkedTasks(r.Context(), resolved.RoomID, requests)
	if err != nil {
		respondJSONError(w, 503, "Could not read conversation tasks")
		return
	}
	counts, err := s.readTaskCounts(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Could not read conversation task progress")
		return
	}
	c := counts.Rooms[resolved.RoomID]
	activeTasks, reviewTasks, queuedTasks, doneTasks, failedTasks := c.Active, c.Review, c.Queued, c.Done, c.Failed
	contact.TaskCount = c.Total
	contact.ActiveTasks = activeTasks
	contact.ReviewTasks = reviewTasks
	contact.QueuedTasks = queuedTasks
	contact.DoneTasks = doneTasks
	contact.FailedTasks = failedTasks
	contact.CanceledTasks = c.Canceled

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":           linked,
		"task_count":      c.Total,
		"active_tasks":    activeTasks,
		"review_tasks":    reviewTasks,
		"queued_tasks":    queuedTasks,
		"done_tasks":      doneTasks,
		"failed_tasks":    failedTasks,
		"canceled_tasks":  c.Canceled,
		"requests":        inboxRecoveryRequests(requests),
		"conversation_id": convID,
		"room_id":         resolved.RoomID,
		"contact":         contact,
		"messages":        messages,
		"has_more":        hasMore,
	})
}

// POST /api/v1/chat/conversations/{id}/messages
func (s *Server) handleInternalSendConversationMessage(w http.ResponseWriter, r *http.Request) {
	ownerID := s.accountOwner(w, r)
	if ownerID == "" {
		return
	}
	var payload struct {
		RegenerateReplyID string   `json:"regenerate_reply_id"`
		AttachmentIDs     []string `json:"attachment_ids"`
		AuthorType        string   `json:"author_type"`
		AuthorName        string   `json:"author_name"`
		Message           string   `json:"message"`
		Content           string   `json:"content"`
		RequestID         string   `json:"request_id"`
		ApproveMessageID  string   `json:"approve_message_id"`
		PlanAction        string   `json:"plan_action"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&payload) != nil {
		respondJSONError(w, 400, "Invalid request")
		return
	}
	if payload.PlanAction != "" && payload.PlanAction != "approve" && payload.PlanAction != "defer" {
		respondJSONError(w, 400, "Unknown plan action")
		return
	}
	if payload.PlanAction != "" && payload.ApproveMessageID == "" {
		respondJSONError(w, 400, "Choose a plan for this action")
		return
	}
	text := strings.TrimSpace(payload.Message)
	if text == "" {
		text = strings.TrimSpace(payload.Content)
	}
	if payload.RegenerateReplyID != "" {
		if payload.ApproveMessageID != "" || payload.PlanAction != "" || len(payload.AttachmentIDs) > 0 || !db.IsValidUUID(payload.RegenerateReplyID) {
			respondJSONError(w, 400, "Choose one reply draft to regenerate")
			return
		}
		text = db.DefaultAgentHandle + " Regenerate draft"
	}
	if text == "" && len(payload.AttachmentIDs) == 0 {
		respondJSONError(w, 400, "Message cannot be empty")
		return
	}
	if len([]rune(text)) > 6000 || strings.HasPrefix(text, attachmentEnvelope) {
		respondJSONError(w, 400, "Use a message up to 6000 characters")
		return
	}
	if err := validateAttachmentIDs(ownerID, payload.AttachmentIDs); err != nil {
		respondJSONError(w, 400, err.Error())
		return
	}
	text = encodeAttachmentMessage(text, payload.AttachmentIDs)
	legacy := payload.RequestID == ""
	if legacy {
		payload.RequestID = db.GenerateUUID()
	}
	if !db.IsValidUUID(payload.RequestID) {
		respondJSONError(w, 400, "request_id must be a UUID")
		return
	}
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		respondJSONError(w, 503, "Inbox storage is not ready")
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), text)
	if payload.RegenerateReplyID != "" {
		source, err := s.projectStudioRepo.GetMessageByID(r.Context(), payload.RegenerateReplyID)
		if err != nil || source == nil || source.RoomID != resolved.RoomID || source.AuthorType != "agent" {
			respondJSONError(w, 404, "Reply draft not found")
			return
		}
		_, draft := splitCustomerReplyDraft(source.Content)
		if draft == "" || !resolved.IsAgent {
			respondJSONError(w, 400, "Choose a customer reply draft")
			return
		}
		raw, _ := json.Marshal(attachmentMessage{Text: "Regenerate draft", RegenerateReplyID: source.ID})
		text = attachmentEnvelope + string(raw)
	}
	if payload.RegenerateReplyID == "" && payload.ApproveMessageID == "" && resolved.IsAgent && len(payload.AttachmentIDs) == 0 {
		payload.ApproveMessageID = s.conversationalApprovalID(r.Context(), resolved.RoomID, text)
	}
	if payload.ApproveMessageID != "" {
		if !resolved.IsAgent || len(payload.AttachmentIDs) > 0 {
			respondJSONError(w, 400, "Approve a plan in an agent conversation without new attachments")
			return
		}
		plan, err := s.persistedInboxPlan(r.Context(), resolved.RoomID, ownerID, resolved.TargetAgentID, payload.ApproveMessageID)
		if err != nil {
			respondJSONError(w, 409, err.Error())
			return
		}
		payload.RequestID = delegationMessageID("approve-plan:" + resolved.RoomID + ":" + plan.MessageID)
		decisionText := "Approved: " + plan.Plan.Title
		if payload.PlanAction == "defer" {
			messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), resolved.RoomID)
			if err != nil {
				respondJSONError(w, 503, "Could not verify the plan status")
				return
			}
			for _, m := range messages {
				if m.AuthorType == "user" {
					if decision := decodeApprovedInboxPlan(m.Content); decision != nil && decision.MessageID == plan.MessageID {
						respondJSONError(w, 409, "This plan is already approved. Use its work controls to stop active execution.")
						return
					}
				}
			}

			payload.RequestID = delegationMessageID("defer-plan:" + resolved.RoomID + ":" + plan.MessageID)
			plan.Action = "defer"
			decisionText = "Not now: " + plan.Plan.Title
		}
		raw, _ := json.Marshal(attachmentMessage{Text: decisionText, Approval: plan})
		text = attachmentEnvelope + string(raw)
	}
	now := time.Now().UTC()
	q := db.InboxRequest{RoomID: resolved.RoomID, RequestID: payload.RequestID, PayloadHash: fmt.Sprintf("%x", sha256.Sum256([]byte(text))), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: ownerID, AgentID: resolved.TargetAgentID, State: "queued", CreatedAt: now, Deadline: now.Add(24 * time.Hour)}
	user := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", AuthorID: "operator_user", AuthorName: "Owner", Content: text, CreatedAt: now, UpdatedAt: now}
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
		respondJSONError(w, 429, "Queue full (20 messages). Wait or cancel a queued message.")
		return
	}
	if errors.Is(err, db.ErrInboxRequestConflict) {
		respondJSONError(w, 409, "Request ID has already been used for a different message")
		return
	}
	if err != nil {
		respondJSONError(w, 503, "Message not accepted. Inbox storage is not ready. Try again using the same ID.")
		return
	}
	if fresh {
		if !resolved.IsAgent {
			s.OnOperatorChatMessageSent(q.RoomID)
		}
		s.nameAgentTopic(r.Context(), q.RoomID, text)
		s.publishInbox(q.RoomID)
		if legacy {
			s.drainInboxRoom(q.RoomID)
		} else {
			s.kickInboxRoom(q.RoomID)
		}
	}
	// Durable acceptance is the acknowledgement. History and receipt hydration
	// must not turn an already accepted message into a slow/failed POST.
	if !legacy {
		respondJSON(w, http.StatusAccepted, map[string]any{"request": q, "user_message_saved": true})
		return
	}
	requests, err := store.ListInboxRequests(r.Context(), q.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Message accepted. Refresh history to check the result. Do not create a new ID.")
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
	if ctxErr != nil || strings.Contains(lower, "timeout") {
		return "Provider timed out. Your message is saved, but the result is unknown. Refresh history. The request will not be rerun automatically."
	}
	if strings.Contains(lower, "user location is not supported for the api use") {
		return "The provider rejected AI execution from this environment because of a location restriction. Google sign-in may still be saved. Your message is saved; signing in again will not resolve this restriction. No automatic retry was started."
	}
	if strings.Contains(lower, "quota") || strings.Contains(lower, "429") || strings.Contains(lower, "resource_exhausted") {
		if dur, resetStr, ok := agent.ParseProviderQuotaResetDuration(raw); ok && dur > 0 {
			return fmt.Sprintf("Provider quota is limited (Resets in %s). Your message is saved. Wait for the quota to reset; accounts will not be switched automatically.", resetStr)
		}
		return "Provider quota is limited. Your message is saved. Wait for the quota to reset; accounts will not be switched automatically."
	}
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "login") || strings.Contains(lower, "401") || strings.Contains(lower, "invalid_grant") {
		return "Check the provider's sign-in status. Your message is saved. Open the agent's status. The request will not be rerun automatically."
	}
	return "The provider could not reply. Your message is saved. Check the agent's status and refresh history. The request will not be rerun automatically."
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

// isCustomerRoom returns true if the given room represents a visitor/customer session.
func (s *Server) isCustomerRoom(ctx context.Context, roomID string) bool {
	if roomID == "" {
		return false
	}
	if sess, err := s.crmSession(ctx, roomID); err == nil && sess != nil {
		return true
	}
	if s.projectStudioRepo != nil {
		if rm, err := s.projectStudioRepo.GetRoomByID(ctx, roomID); err == nil && rm != nil {
			if strings.HasPrefix(rm.Topic, "agent:") || strings.HasPrefix(rm.Topic, "agent-topic:") || rm.ID == defaultScrumMasterRoomID || rm.ID == defaultAntigravityRoomID {
				return false
			}
			return true
		}
	}
	return false
}

// extractCustomerReplyDraft parses a ready-to-use customer response draft from agent content.
func extractCustomerReplyDraft(content string) string {
	_, draft := splitCustomerReplyDraft(content)
	return draft
}

var internalDraftLink = regexp.MustCompile(`\[[^\]]*\]\(/(?:tasks|inbox|projects)/[^)]*\)`)

type customerReplyPayload struct {
	Draft string `json:"draft"`
}

func splitCustomerReplyDraft(content string) (string, string) {
	const marker = "```cuan-customer-reply\n"
	if start := strings.Index(content, marker); start >= 0 {
		body := start + len(marker)
		if end := strings.Index(content[body:], "```"); end >= 0 {
			var payload customerReplyPayload
			if strictCoordinatorJSON([]byte(content[body:body+end]), &payload) == nil {
				draft := strings.TrimSpace(internalDraftLink.ReplaceAllString(payload.Draft, ""))
				if draft != "" && len(draft) <= 6000 && !strings.Contains(draft, "```cuan-") {
					return strings.TrimSpace(content[:start] + content[body+end+3:]), draft
				}
			}
		}
		return content, ""
	}
	lower := strings.ToLower(content)
	markers := []string{
		"**draf balasan untuk pengunjung:**",
		"**draft balasan untuk pengunjung:**",
		"**draf balasan pengunjung:**",
		"**draft balasan pengunjung:**",
		"**draf balasan:**",
		"**draft balasan:**",
		"**customer reply draft:**",
		"draf balasan untuk pengunjung:",
		"draft balasan untuk pengunjung:",
		"draf balasan pengunjung:",
		"draft balasan pengunjung:",
		"balasan siap pakai:",
	}
	for _, marker := range markers {
		idx := strings.Index(lower, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		lines := strings.SplitAfter(content[start:], "\n")
		consumed := 0
		var draftLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" && len(draftLines) == 0 {
				consumed += len(line)
				continue
			}
			if trimmed == "" || strings.HasPrefix(trimmed, "**") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "[View work]") || strings.HasPrefix(trimmed, "[Tinjau rincian") {
				break
			}
			clean := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, ">")), "\"")
			clean = strings.TrimSpace(internalDraftLink.ReplaceAllString(clean, ""))
			if clean != "" {
				draftLines = append(draftLines, clean)
			}
			consumed += len(line)
		}
		if len(draftLines) > 0 {
			return strings.TrimSpace(content[:idx] + "\n\n" + content[start+consumed:]), strings.Join(draftLines, "\n")
		}
	}
	return content, ""
}

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

	isCustomer := s.isCustomerRoom(ctx, roomID)
	workUpdates := s.inboxWorkUpdates(ctx, roomID, rawMsgs)
	approved := map[string]bool{}
	deferred := map[string]bool{}
	for _, m := range rawMsgs {
		if m.AuthorType == "user" {
			if p := decodeInboxPlanDecision(m.Content); p != nil {
				if p.Action == "defer" {
					deferred[p.MessageID] = true
				} else {
					approved[p.MessageID] = true
				}
			}
		}
	}
	dtos := make([]ChatMessageDTO, 0, len(rawMsgs))
	emailDeliveries := map[string]*ChatEmailDelivery{}
	if isCustomer {
		emailDeliveries, err = s.chatEmailDeliveries(ctx, roomID)
		if err != nil {
			return nil, err
		}
	}
	receipts := map[string]db.MessageReceipt{}
	if store, ok := s.projectStudioRepo.(db.ChatReceiptRepository); ok && isCustomer {
		receipts, err = store.MessageReceipts(ctx, roomID)
		if err != nil {
			return nil, err
		}
	}
	for _, m := range rawMsgs {
		// Filter out internal synthetic review trigger prompts from client chat timeline
		if internalInboxReviewInput(m) {
			continue
		}

		senderType := "visitor"
		if isCustomer {
			if m.AuthorID == "operator_user" || m.AuthorName == "You" || m.AuthorName == "Owner" || m.AuthorType == "agent" || m.AuthorType == "system" {
				senderType = "operator"
			}
		} else {
			if m.AuthorType == "agent" || m.AuthorType == "system" {
				senderType = "operator"
			}
		}

		regenerateID := ""
		if m.AuthorType == "user" && m.AuthorID == "operator_user" {
			regenerateID = decodeRegenerateReplyID(m.Content)
		}
		mention := extractMentionHandle(m.Content)
		isInternal := false
		if isCustomer {
			isInternal = customerMessageInternal(&m)
		}
		messageContent := m.Content
		replyDraft := ""
		if m.AuthorType == "agent" {
			messageContent, replyDraft = splitCustomerReplyDraft(messageContent)
		}

		var actionCard *AgentActionCard
		content, draft := splitInboxTaskDraft(m.AuthorType, messageContent)
		content, approvalTitle := splitInboxApproval(m.AuthorType, content)
		content, promptChoices := splitInboxPromptChoices(m.AuthorType, content)
		var approvalPlan *inboxApprovalPlan
		approvalRequestID := ""
		if m.AuthorType == "agent" {
			approvalPlan = parseInboxApprovalPlan(m.Content)
			if approvalPlan != nil {
				approvalRequestID = delegationMessageID("approve-plan:" + roomID + ":" + m.ID)
				content = presentContinuationText(content, approvalPlan)
				approvalPlan = presentContinuationPlan(approvalPlan)
			}
		}
		var attached []conversationAttachment
		if m.AuthorType == "user" || m.AuthorType == "agent" {
			var ids []string
			content, ids = decodeAttachmentMessage(content)
			for _, id := range ids {
				// Owner-only APIs share the account context; files remain private.
				a, _, err := loadAttachmentForMessage(ctx, s, roomID, m.ID, id)
				if err != nil {
					a, _, err = loadAttachment("visitor", id)
				}
				if err != nil {
					a, _, err = findAttachmentAcrossOwners(id)
				}
				if err == nil {
					if isCustomer {
						a.URL = "https://cuan.dojo.cc/api/chat/attachments/" + a.ID
					}
					attached = append(attached, a)
				}
			}
		}

		var attURL, attType, attName string
		var attSize int64
		var hasAtt bool
		if len(attached) > 0 {
			attURL = attached[0].URL
			attType = attached[0].Type
			attName = attached[0].Name
			attSize = attached[0].Size
			hasAtt = true
		}

		var groundingTastes []GroundingTaste
		if m.AuthorType == "agent" {
			if recorded, ok := s.GetMessageGrounding(m.ID); ok && len(recorded) > 0 {
				groundingTastes = recorded
			} else {
				groundingTastes = s.activeBrandGroundingTastesForRoom(ctx, roomID)
			}
		}

		dtos = append(dtos, ChatMessageDTO{
			RegenerateReplyID: regenerateID,
			WorkUpdate:        workUpdates[m.ID],
			MessageReceipt:    receipts[m.ID],
			EmailDelivery:     emailDeliveries[m.ID],
			ID:                int64(len(dtos) + 1), // Monotonic numeric ID for Dojo widget
			UUID:              m.ID,
			ApprovalRequestID: approvalRequestID,
			ApprovalAccepted:  approved[m.ID],
			PlanDeferred:      deferred[m.ID],
			PlanDeferRequestID: func() string {
				if approvalPlan != nil {
					return delegationMessageID("defer-plan:" + roomID + ":" + m.ID)
				}
				return ""
			}(),
			ChatSessionUUID: roomID,
			RoomID:          roomID,
			SenderType:      senderType,
			AuthorType:      m.AuthorType,
			AuthorID:        m.AuthorID,
			AuthorName:      m.AuthorName,
			Message:         content,
			Content:         content,
			Product:         product,
			AttachmentURL:   attURL,
			AttachmentType:  attType,
			AttachmentName:  attName,
			AttachmentSize:  attSize,
			HasAttachment:   hasAtt,
			ActionCard:      actionCard,
			TaskDraft:       draft,
			IsInternal:      isInternal,
			MentionHandle:   mention,
			ReplyDraft:      replyDraft,
			ApprovalTitle:   approvalTitle,
			ApprovalPlan:    approvalPlan,
			PromptChoices:   promptChoices,
			Attachments:     attached,
			CreatedAt:       m.CreatedAt,
			GroundingTastes: groundingTastes,
		})
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
			{Text: "Investigate the root cause and isolate the affected module", Completed: false},
			{Text: "Implement the bug fix in the repository", Completed: false},
			{Text: "Targeted automated tests pass with no regressions", Completed: false},
		}
	} else if strings.Contains(lowerPrompt, "deploy") || strings.Contains(lowerPrompt, "vps") || strings.Contains(lowerPrompt, "rilis") || strings.Contains(lowerPrompt, "release") {
		dodList = []db.DoDItem{
			{Text: "Build the frontend bundle and production binary", Completed: false},
			{Text: "Sync the release to the target server and restart the service", Completed: false},
			{Text: "Public smoke test returns HTTP 200 and live verification passes", Completed: false},
		}
	} else {
		dodList = []db.DoDItem{
			{Text: "Write technical specifications and break down the repository tasks", Completed: false},
			{Text: "Implement the feature using the project architecture", Completed: false},
			{Text: "Targeted automated unit test PASS & audit evidence", Completed: false},
		}
	}

	draftReply := "Halo Kak, terima kasih sudah menghubungi kami. Tim kami sedang menindaklanjuti kendala ini dan segera memperbaikinya. Mohon ditunggu ya! 🙏"
	if strings.Contains(lowerPrompt, "fix") || strings.Contains(lowerPrompt, "perbaiki") || strings.Contains(lowerPrompt, "bug") || strings.Contains(lowerPrompt, "error") {
		draftReply = "Halo Kak, terima kasih laporannya. Tim kami sedang memeriksa dan memperbaiki kendala tersebut. Kami kabari segera setelah selesai ya!"
	}

	respText := fmt.Sprintf(`Here is a proposed plan for your request:

**Problem Statement:**
%s

**Execution plan & definition of done:**
1. %s
2. %s
3. %s

**Draf Balasan untuk Pengunjung:**
> "%s"

**Recommended worker agent:** %s (check quota before starting).
Select **Approve and start task** below to begin execution.`, title, dodList[0].Text, dodList[1].Text, dodList[2].Text, draftReply, worker)

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

// Only executing work belongs in the working count.
func inboxTaskIsWorking(status string) bool {
	switch status {
	case "doing", "in_progress", "running":
		return true
	}
	return false
}
