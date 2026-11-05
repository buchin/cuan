package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
)

var (
	chatIdleDetectorMu  sync.RWMutex
	chatIdleHistory     = make(map[string]chatIdleNotificationRecord)
	chatIdleTimers      = make(map[string]*time.Timer)
	chatIdleRoomLocksMu sync.Mutex
	chatIdleRoomLocks   = make(map[string]*sync.Mutex)
)

type chatIdleNotificationRecord struct {
	RoomID     string
	MessageID  string
	NotifiedAt time.Time
	ResendID   string
}

func getRoomLock(roomID string) *sync.Mutex {
	chatIdleRoomLocksMu.Lock()
	defer chatIdleRoomLocksMu.Unlock()
	lock, exists := chatIdleRoomLocks[roomID]
	if !exists {
		lock = &sync.Mutex{}
		chatIdleRoomLocks[roomID] = lock
	}
	return lock
}

// OnCustomerChatMessageReceived is called when a visitor or customer sends a message.
// It arms or resets the background idle timer for the conversation.
func (s *Server) OnCustomerChatMessageReceived(roomID, messageID string) {
	if roomID == "" {
		return
	}

	settings, err := s.GetResendSettings(context.Background())
	if err != nil || !settings.Enabled || settings.APIKey == "" {
		return
	}

	thresholdMinutes := settings.IdleThresholdMinutes
	if thresholdMinutes <= 0 {
		thresholdMinutes = 5
	}
	delay := time.Duration(thresholdMinutes) * time.Minute

	s.ScheduleChatIdleTimer(roomID, delay)
}

// OnOperatorChatMessageSent is called when an operator replies to a conversation.
// It cancels pending idle timers and marks the chat as responded.
func (s *Server) OnOperatorChatMessageSent(roomID string) {
	s.CancelChatIdleTimer(roomID)
}

// ScheduleChatIdleTimer sets a debounce timer to trigger idle notification after delay.
func (s *Server) ScheduleChatIdleTimer(roomID string, delay time.Duration) {
	chatIdleDetectorMu.Lock()
	defer chatIdleDetectorMu.Unlock()

	// Cancel existing timer for this room if active (debounce)
	if t, exists := chatIdleTimers[roomID]; exists && t != nil {
		t.Stop()
	}

	chatIdleTimers[roomID] = time.AfterFunc(delay, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = s.TriggerRoomIdleNotification(ctx, roomID)
	})
}

// CancelChatIdleTimer cancels any pending idle timer for the specified room.
func (s *Server) CancelChatIdleTimer(roomID string) {
	chatIdleDetectorMu.Lock()
	defer chatIdleDetectorMu.Unlock()

	if t, exists := chatIdleTimers[roomID]; exists && t != nil {
		t.Stop()
		delete(chatIdleTimers, roomID)
	}
}

// TriggerRoomIdleNotification checks if a single room is idle and unread, sending an email if due.
func (s *Server) TriggerRoomIdleNotification(ctx context.Context, roomID string) (bool, error) {
	if roomID == "" {
		return false, nil
	}

	settings, err := s.GetResendSettings(ctx)
	if err != nil || !settings.Enabled || strings.TrimSpace(settings.APIKey) == "" {
		return false, nil
	}

	roomLock := getRoomLock(roomID)
	roomLock.Lock()
	defer roomLock.Unlock()

	thresholdMinutes := settings.IdleThresholdMinutes
	if thresholdMinutes <= 0 {
		thresholdMinutes = 5
	}
	thresholdDuration := time.Duration(thresholdMinutes) * time.Minute

	// Fetch messages for the room
	if s.projectStudioRepo == nil {
		return false, nil
	}

	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil || len(messages) == 0 {
		return false, nil
	}

	// 1. Identify latest customer and operator messages
	var latestCustomerMsg *cuanDB.ProjectMessage
	var latestOperatorTime time.Time
	var unreadCustomerMsgs []cuanDB.ProjectMessage

	for i := range messages {
		msg := messages[i]
		isOperator := msg.AuthorID == "operator_user" || msg.AuthorName == "You" || msg.AuthorName == "Owner" || msg.AuthorType == "agent" || msg.AuthorType == "system"
		if isOperator {
			if msg.CreatedAt.After(latestOperatorTime) {
				latestOperatorTime = msg.CreatedAt
			}
		} else if msg.AuthorType == "user" {
			if latestCustomerMsg == nil || msg.CreatedAt.After(latestCustomerMsg.CreatedAt) {
				m := messages[i]
				latestCustomerMsg = &m
			}
		}
	}

	if latestCustomerMsg == nil {
		return false, nil
	}

	// If operator has replied AFTER the latest customer message, the chat is active/answered
	if !latestOperatorTime.IsZero() && latestOperatorTime.After(latestCustomerMsg.CreatedAt) {
		return false, nil
	}

	// Check if message is old enough to be considered idle
	timeSinceLastMessage := time.Since(latestCustomerMsg.CreatedAt)
	if timeSinceLastMessage < thresholdDuration {
		return false, nil
	}

	// 2. Check read state from item preferences
	preferences := itemPreferences{}
	if s.authSvc != nil {
		if ownerID, err := s.authSvc.OwnerID(ctx); err == nil && ownerID != "" {
			itemPreferencesMu.Lock()
			saved, _ := readItemPreferences(ownerID)
			itemPreferencesMu.Unlock()
			preferences = saved
		}
	}
	state := preferences["conversation:"+roomID]
	if state.ReadAt != "" {
		cutoff, _ := time.Parse(time.RFC3339Nano, state.ReadAt)
		if !latestCustomerMsg.CreatedAt.After(cutoff) {
			// Already read by operator
			return false, nil
		}
	}

	// 3. Collect unread customer messages (since last operator reply or read cutoff)
	var cutoffTime time.Time
	if !latestOperatorTime.IsZero() {
		cutoffTime = latestOperatorTime
	}
	if state.ReadAt != "" {
		readCutoff, _ := time.Parse(time.RFC3339Nano, state.ReadAt)
		if readCutoff.After(cutoffTime) {
			cutoffTime = readCutoff
		}
	}

	for _, m := range messages {
		isCustomer := m.AuthorType == "user" && m.AuthorID != "operator_user" && m.AuthorName != "Owner" && m.AuthorName != "You"
		if isCustomer && (cutoffTime.IsZero() || m.CreatedAt.After(cutoffTime)) {
			unreadCustomerMsgs = append(unreadCustomerMsgs, m)
		}
	}
	if len(unreadCustomerMsgs) == 0 {
		return false, nil
	}

	// 4. Anti-inbox bombing cooldown check (minimum 5 minutes between notifications per room)
	chatIdleDetectorMu.RLock()
	lastNotif, hasNotif := chatIdleHistory[roomID]
	chatIdleDetectorMu.RUnlock()

	if hasNotif {
		// If last notification was for the exact same message, skip
		if lastNotif.MessageID == latestCustomerMsg.ID {
			return false, nil
		}
		// If notified recently within 5 minutes, enforce cooldown
		if time.Since(lastNotif.NotifiedAt) < 5*time.Minute {
			return false, nil
		}
	}

	// 5. Resolve recipient email
	recipient := strings.TrimSpace(settings.NotificationEmail)
	if recipient == "" && s.authSvc != nil {
		if info, err := s.authSvc.OwnerIdentity(ctx); err == nil && info.Email != "" {
			recipient = info.Email
		}
	}
	if recipient == "" {
		return false, fmt.Errorf("no recipient notification email found for idle alert")
	}

	// 6. Resolve customer identity & store details
	customerName := latestCustomerMsg.AuthorName
	if customerName == "" {
		customerName = "Pelanggan"
	}
	customerEmail := ""
	if s.crm != nil {
		if sess, err := s.crmSession(ctx, roomID); err == nil && sess != nil {
			if sess.Name != "" {
				customerName = sess.Name
			}
			customerEmail = sess.Email
		}
	}

	storeName := "Toko Cuan"

	baseDomain := "cuan.dojo.cc"
	if s.cfg != nil && s.cfg.BaseDomain != "" {
		baseDomain = s.cfg.BaseDomain
	}
	convoURL := fmt.Sprintf("https://%s/inbox/%s", baseDomain, roomID)

	var items []resend.ChatMessageItem
	for _, m := range unreadCustomerMsgs {
		items = append(items, resend.ChatMessageItem{
			SenderName: m.AuthorName,
			SenderRole: "customer",
			Content:    m.Content,
			SentAt:     m.CreatedAt,
		})
	}

	templateParams := resend.ChatIdleTemplateParams{
		StoreName:       storeName,
		CustomerName:    customerName,
		CustomerEmail:   customerEmail,
		RoomID:          roomID,
		ConversationURL: convoURL,
		IdleMinutes:     thresholdMinutes,
		Messages:        items,
	}

	htmlBody := resend.BuildChatIdleTranscriptHTML(templateParams)
	textBody := resend.BuildChatIdleTranscriptText(templateParams)

	fromName := settings.FromName
	if fromName == "" {
		fromName = "Cuan Support"
	}
	fromEmail := settings.FromEmail
	if fromEmail == "" {
		fromEmail = "onboarding@resend.dev"
	}
	fromHeader := fmt.Sprintf("%s <%s>", fromName, fromEmail)

	client, err := s.ResendClient(ctx)
	if err != nil {
		return false, fmt.Errorf("initialize resend client: %w", err)
	}

	subject := fmt.Sprintf("🔔 [Live Chat] Pesan Belum Terbaca dari %s (%d pesan, idle >%d menit)", customerName, len(items), thresholdMinutes)
	resp, err := client.SendEmail(ctx, resend.EmailRequest{
		From:    fromHeader,
		To:      []string{recipient},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	})
	if err != nil {
		return false, fmt.Errorf("send idle email via resend: %w", err)
	}

	// 7. Record notification event
	now := time.Now().UTC()
	chatIdleDetectorMu.Lock()
	chatIdleHistory[roomID] = chatIdleNotificationRecord{
		RoomID:     roomID,
		MessageID:  latestCustomerMsg.ID,
		NotifiedAt: now,
		ResendID:   resp.ID,
	}
	chatIdleDetectorMu.Unlock()

	// Persist to audit log table if DB is available
	messageIDs := make([]string, 0, len(unreadCustomerMsgs))
	for _, message := range unreadCustomerMsgs {
		messageIDs = append(messageIDs, message.ID)
	}
	// This email is an operator notification, never customer delivery or a chat read.
	if err := s.recordChatEmail(ctx, roomID, resp.ID, messageIDs, "operator"); err != nil {
		// Sending already succeeded. Do not replay the email to repair tracking.
		fmt.Printf("[chat] could not save email receipt mapping for room %s\n", roomID)
	}
	if s.db != nil {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO resend_email_logs (id, room_id, recipient_email, subject, resend_email_id, message_count, status, sent_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, cuanDB.GenerateUUID(), roomID, recipient, subject, resp.ID, len(items), "sent", now)
	}

	return true, nil
}

// CheckAndNotifyIdleChats scans all active chat rooms and sends email alerts for idle unread conversations.
func (s *Server) CheckAndNotifyIdleChats(ctx context.Context) (int, error) {
	settings, err := s.GetResendSettings(ctx)
	if err != nil || !settings.Enabled || strings.TrimSpace(settings.APIKey) == "" {
		return 0, nil
	}

	if s.projectStudioRepo == nil {
		return 0, nil
	}

	rooms, err := s.projectStudioRepo.ListRoomsByProjectID(ctx, "default")
	if err != nil || len(rooms) == 0 {
		rooms, err = s.projectStudioRepo.ListAllRooms(ctx)
		if err != nil {
			return 0, fmt.Errorf("list chat rooms: %w", err)
		}
	}

	notifiedCount := 0
	for _, room := range rooms {
		sent, err := s.TriggerRoomIdleNotification(ctx, room.ID)
		if err == nil && sent {
			notifiedCount++
		}
	}

	return notifiedCount, nil
}

// handleTriggerChatIdleCheck exposes an endpoint to trigger or verify the idle detection run on demand.
// Route: POST /api/v1/chat/idle/check, POST /api/chat/idle/check
func (s *Server) handleTriggerChatIdleCheck(w http.ResponseWriter, r *http.Request) {
	count, err := s.CheckAndNotifyIdleChats(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memindai chat idle: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"status":         "success",
		"scanned":        true,
		"notified_count": count,
		"message":        fmt.Sprintf("Pemeriksaan chat idle selesai. %d notifikasi email dikirimkan.", count),
	})
}
