package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
)

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
				s.publishInbox(sessionUUID)
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
		s.publishInbox(sessionUUID)
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
			IsInternal:      false,
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
