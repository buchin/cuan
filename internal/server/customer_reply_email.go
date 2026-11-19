package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
	"github.com/go-chi/chi/v5"
)

var (
	customerEmailMu sync.Mutex
	inFlightEmails  sync.Map // key: string -> struct{}
	sentEmailCache  sync.Map // messageID: string -> cachedEmailDelivery
)

type cachedEmailDelivery struct {
	EmailID   string
	Status    string
	Recipient string
	SentAt    time.Time
}

type sendCustomerReplyEmailRequest struct {
	MessageID      string `json:"message_id,omitempty"`
	Recipient      string `json:"recipient,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Draft          string `json:"draft,omitempty"`
	Content        string `json:"content,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type sendCustomerReplyEmailResponse struct {
	Status         string `json:"status"`
	EmailID        string `json:"email_id"`
	DeliveryStatus string `json:"delivery_status"`
	Recipient      string `json:"recipient,omitempty"`
	Message        string `json:"message"`
	AlreadySent    bool   `json:"already_sent,omitempty"`
}

// validateEmailAddress strictly validates RFC 5322 address without CRLF injection vulnerabilities.
func validateEmailAddress(address string) (string, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return "", errors.New("Email penerima wajib diisi")
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return "", errors.New("Format email tidak boleh mengandung karakter baris baru")
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed {
		return "", errors.New("Format email penerima tidak valid")
	}
	parts := strings.Split(trimmed, "@")
	if len(parts) != 2 || !strings.Contains(parts[1], ".") {
		return "", errors.New("Domain email penerima tidak valid")
	}
	return trimmed, nil
}

// handleSendCustomerReplyEmail handles POST /api/v1/chat/conversations/{id}/messages/{messageID}/email
func (s *Server) handleSendCustomerReplyEmail(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	convID := chi.URLParam(r, "id")
	messageID := chi.URLParam(r, "messageID")
	if !db.IsValidUUID(messageID) {
		respondJSONError(w, http.StatusBadRequest, "ID pesan tidak valid")
		return
	}

	resolved := s.resolveConversationContext(r.Context(), convID, "")

	var req sendCustomerReplyEmailRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err == nil && len(body) > 0 {
		_ = json.Unmarshal(body, &req)
	}

	s.processCustomerReplyEmail(w, r, resolved, messageID, req)
}

// handleSendConversationEmail handles POST /api/v1/chat/conversations/{id}/send-email
func (s *Server) handleSendConversationEmail(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	convID := chi.URLParam(r, "id")
	resolved := s.resolveConversationContext(r.Context(), convID, "")

	var req sendCustomerReplyEmailRequest
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Permintaan tidak valid")
		return
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "Format payload JSON tidak valid")
			return
		}
	}

	messageID := strings.TrimSpace(req.MessageID)
	if messageID == "" {
		messageID = db.GenerateUUID()
	} else if !db.IsValidUUID(messageID) {
		respondJSONError(w, http.StatusBadRequest, "ID pesan tidak valid")
		return
	}

	s.processCustomerReplyEmail(w, r, resolved, messageID, req)
}

func (s *Server) processCustomerReplyEmail(
	w http.ResponseWriter,
	r *http.Request,
	resolved ResolvedConversation,
	messageID string,
	req sendCustomerReplyEmailRequest,
) {
	ctx := r.Context()

	// 1. Resolve content/draft
	content := strings.TrimSpace(req.Draft)
	if content == "" {
		content = strings.TrimSpace(req.Content)
	}

	// If content not explicitly provided, load from message
	if content == "" {
		var sourceContent string
		if s.db != nil {
			_ = s.db.QueryRowContext(ctx, `SELECT content FROM project_messages WHERE id=$1::uuid AND room_id=$2::uuid`, messageID, resolved.RoomID).Scan(&sourceContent)
		}
		if sourceContent == "" && s.projectStudioRepo != nil {
			msgs, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, resolved.RoomID)
			for _, m := range msgs {
				if m.ID == messageID {
					sourceContent = m.Content
					break
				}
			}
		}
		if sourceContent != "" {
			_, draft := splitCustomerReplyDraft(sourceContent)
			if draft != "" {
				content = draft
			} else {
				content = strings.TrimSpace(sourceContent)
			}
		}
	}

	if content == "" {
		respondJSONError(w, http.StatusBadRequest, "Konten draf balasan email tidak boleh kosong")
		return
	}

	// 2. Resolve recipient
	recipient := strings.TrimSpace(req.Recipient)
	if recipient == "" && s.db != nil {
		_ = s.db.QueryRowContext(ctx, `SELECT email FROM crm_contacts WHERE session_uuid=$1::uuid AND email <> '' LIMIT 1`, resolved.RoomID).Scan(&recipient)
		if recipient == "" {
			_ = s.db.QueryRowContext(ctx, `SELECT email FROM crm_contacts WHERE id::text=$1 AND email <> '' LIMIT 1`, resolved.RoomID).Scan(&recipient)
		}
	}

	validRecipient, err := validateEmailAddress(recipient)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 3. Resolve subject
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = "Balasan untuk percakapan Anda"
	}
	if strings.ContainsAny(subject, "\r\n") {
		respondJSONError(w, http.StatusBadRequest, "Subjek email tidak boleh mengandung baris baru")
		return
	}

	// 4. Idempotency Guard
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("cuan-reply/%s/%s", resolved.RoomID, messageID)
	}

	// Check database for existing delivery
	if s.db != nil {
		var existingEmailID, existingStatus string
		err := s.db.QueryRowContext(ctx, `SELECT e.email_id, r.status
FROM chat_message_emails e
JOIN chat_email_receipts r ON r.email_id = e.email_id
WHERE e.message_id = $1::uuid
ORDER BY e.linked_at DESC
LIMIT 1`, messageID).Scan(&existingEmailID, &existingStatus)
		if err == nil && existingEmailID != "" && (existingStatus == "sent" || existingStatus == "delivered") {
			respondJSON(w, http.StatusOK, sendCustomerReplyEmailResponse{
				Status:         "success",
				EmailID:        existingEmailID,
				DeliveryStatus: existingStatus,
				Recipient:      validRecipient,
				Message:        "Email sudah pernah dikirim sebelumnya (idempotent)",
				AlreadySent:    true,
			})
			return
		}
	}

	// Check in-memory cache
	if cached, ok := sentEmailCache.Load(messageID); ok {
		rec := cached.(cachedEmailDelivery)
		respondJSON(w, http.StatusOK, sendCustomerReplyEmailResponse{
			Status:         "success",
			EmailID:        rec.EmailID,
			DeliveryStatus: rec.Status,
			Recipient:      rec.Recipient,
			Message:        "Email sudah pernah dikirim sebelumnya (idempotent)",
			AlreadySent:    true,
		})
		return
	}

	// In-flight concurrency lock
	flightKey := messageID + ":" + idempotencyKey
	customerEmailMu.Lock()
	if _, loaded := inFlightEmails.LoadOrStore(flightKey, struct{}{}); loaded {
		customerEmailMu.Unlock()
		respondJSONError(w, http.StatusConflict, "Pengiriman email untuk pesan ini sedang berlangsung")
		return
	}
	customerEmailMu.Unlock()
	defer inFlightEmails.Delete(flightKey)

	// 5. Resend settings & client
	settings, err := s.GetResendSettings(ctx)
	if err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Gagal memuat pengaturan email: "+err.Error())
		return
	}

	from := strings.TrimSpace(settings.FromEmail)
	fromName := strings.TrimSpace(settings.FromName)
	if fromName == "" {
		fromName = "Cuan"
	}
	if from == "" {
		if s.resendClient != nil {
			from = "notifications@cuan.dev"
		} else {
			respondJSONError(w, http.StatusBadRequest, "Alamat email pengirim (From Email) belum dikonfigurasi di pengaturan Resend")
			return
		}
	}

	sender, err := mail.ParseAddress(from)
	if err != nil || sender.Address != from || strings.ContainsAny(from+fromName, "\r\n") {
		respondJSONError(w, http.StatusBadRequest, "Format alamat pengirim email tidak valid")
		return
	}

	client, err := s.ResendClient(ctx)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Layanan pengiriman email (Resend) belum aktif: "+err.Error())
		return
	}

	// 6. Execute Provider Send
	emailReq := resend.EmailRequest{
		IdempotencyKey: idempotencyKey,
		From:           (&mail.Address{Name: fromName, Address: from}).String(),
		To:             []string{validRecipient},
		Subject:        subject,
		Text:           content,
	}

	resp, sendErr := client.SendEmail(ctx, emailReq)
	if sendErr != nil {
		respondJSONError(w, http.StatusBadGateway, "Gagal mengirim email via provider: "+sendErr.Error())
		return
	}
	if resp == nil || strings.TrimSpace(resp.ID) == "" {
		respondJSONError(w, http.StatusBadGateway, "Provider tidak mengembalikan konfirmasi ID email")
		return
	}

	// 7. Persist receipt & link message
	if err := s.recordChatEmail(ctx, resolved.RoomID, resp.ID, []string{messageID}, "visitor"); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal mencatat receipt email: "+err.Error())
		return
	}

	// Update in-memory cache
	sentEmailCache.Store(messageID, cachedEmailDelivery{
		EmailID:   resp.ID,
		Status:    "sent",
		Recipient: validRecipient,
		SentAt:    time.Now(),
	})

	respondJSON(w, http.StatusOK, sendCustomerReplyEmailResponse{
		Status:         "success",
		EmailID:        resp.ID,
		DeliveryStatus: "sent",
		Recipient:      validRecipient,
		Message:        "Email balasan berhasil dikirim ke " + validRecipient,
	})
}
