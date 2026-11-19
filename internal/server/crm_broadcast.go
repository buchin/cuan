package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
)

var (
	broadcastSendMu    sync.Mutex
	inFlightBroadcasts sync.Map // key: string -> struct{}
	sentBroadcastCache sync.Map // key: string -> BroadcastSendResponse
)

// BroadcastRecipient represents a single resolved recipient for email broadcasting.
type BroadcastRecipient struct {
	Email     string   `json:"email"`
	Name      string   `json:"name,omitempty"`
	Source    string   `json:"source"` // "crm", "custom", or "both"
	ContactID string   `json:"contact_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// BroadcastAudienceRequest is the payload sent to resolve the broadcast recipient audience.
type BroadcastAudienceRequest struct {
	Tags         []string `json:"tags"`
	CustomEmails any      `json:"custom_emails"`
}

// BroadcastAudienceResponse is the result of resolving the broadcast audience.
type BroadcastAudienceResponse struct {
	TotalUnique int                  `json:"total_unique"`
	CRMCount    int                  `json:"crm_count"`
	CustomCount int                  `json:"custom_count"`
	Recipients  []BroadcastRecipient `json:"recipients"`
}

// BroadcastTagSummary represents a CRM tag and the number of email-ready contacts having that tag.
type BroadcastTagSummary struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// BroadcastSendRequest is the payload sent to dispatch a batch broadcast email.
type BroadcastSendRequest struct {
	Tags           []string `json:"tags"`
	CustomEmails   any      `json:"custom_emails"`
	Subject        string   `json:"subject"`
	Content        string   `json:"content"`
	IdempotencyKey string   `json:"idempotency_key"`
}

// BroadcastSendResponse is the result returned after dispatching an email broadcast.
type BroadcastSendResponse struct {
	Status          string   `json:"status"` // "success", "partial", "failed"
	TotalRecipients int      `json:"total_recipients"`
	SentCount       int      `json:"sent_count"`
	FailedCount     int      `json:"failed_count"`
	Errors          []string `json:"errors,omitempty"`
	IdempotencyKey  string   `json:"idempotency_key"`
	AlreadySent     bool     `json:"already_sent,omitempty"`
	Message         string   `json:"message"`
}

func normalizeTagKey(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	return strings.ToLower(tag)
}

func parseAndValidateEmail(raw string) (email string, name string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", errors.New("Email address cannot be empty")
	}
	if len(raw) > 254 {
		return "", "", errors.New("Email address exceeds maximum length of 254 characters")
	}

	addr, parseErr := mail.ParseAddress(raw)
	if parseErr != nil {
		return "", "", fmt.Errorf("Invalid email address: %s", raw)
	}

	email = strings.ToLower(strings.TrimSpace(addr.Address))
	name = strings.TrimSpace(addr.Name)

	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("Invalid email address: %s", raw)
	}
	if !strings.Contains(parts[1], ".") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".") {
		return "", "", fmt.Errorf("Invalid email domain: %s", raw)
	}
	return email, name, nil
}

func extractCustomEmailStrings(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		fields := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r'
		})
		out := make([]string, 0, len(fields))
		for _, f := range fields {
			s := strings.TrimSpace(f)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			fields := strings.FieldsFunc(item, func(r rune) bool {
				return r == ',' || r == ';' || r == '\n' || r == '\r'
			})
			for _, f := range fields {
				s := strings.TrimSpace(f)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				fields := strings.FieldsFunc(str, func(r rune) bool {
					return r == ',' || r == ';' || r == '\n' || r == '\r'
				})
				for _, f := range fields {
					s := strings.TrimSpace(f)
					if s != "" {
						out = append(out, s)
					}
				}
			} else {
				return nil, errors.New("Each custom email must be a string")
			}
		}
		return out, nil
	default:
		return nil, errors.New("Invalid format for custom_emails: must be a string or array of strings")
	}
}

// broadcastSecret retrieves server secret for signing unsubscribe HMAC tokens.
func (s *Server) broadcastSecret() []byte {
	sec := "cuan-broadcast-unsubscribe-secret-salt"
	if s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.AuthSecret) != "" {
		sec = s.cfg.AuthSecret
	}
	return []byte(sec)
}

// generateUnsubscribeToken generates a tamper-proof HMAC-signed token for the given email.
func (s *Server) generateUnsubscribeToken(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	mac := hmac.New(sha256.New, s.broadcastSecret())
	mac.Write([]byte(email))
	sig := hex.EncodeToString(mac.Sum(nil))
	raw := email + ":" + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// validateUnsubscribeToken validates the HMAC token and extracts the authentic recipient email.
func (s *Server) validateUnsubscribeToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("token cannot be empty")
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid token encoding")
	}
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return "", errors.New("malformed token payload")
	}
	email, sig := parts[0], parts[1]
	expectedMAC := hmac.New(sha256.New, s.broadcastSecret())
	expectedMAC.Write([]byte(email))
	expectedSig := hex.EncodeToString(expectedMAC.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", errors.New("invalid token signature")
	}
	return email, nil
}

// markEmailUnsubscribed marks an email address as unsubscribed across the system.
func (s *Server) markEmailUnsubscribed(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email cannot be empty")
	}
	s.broadcastUnsubscribes.Store(email, time.Now())

	// Update tags in CRM contacts if present
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `SELECT id, tags FROM crm_contacts WHERE LOWER(email) = LOWER($1)`, email)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id string
				var tagsBytes []byte
				if err := rows.Scan(&id, &tagsBytes); err == nil {
					var tags []string
					_ = json.Unmarshal(tagsBytes, &tags)
					hasUnsub := false
					for _, t := range tags {
						if strings.EqualFold(normalizeTagKey(t), "unsubscribed") {
							hasUnsub = true
							break
						}
					}
					if !hasUnsub {
						tags = append(tags, "unsubscribed")
						updatedBytes, _ := json.Marshal(tags)
						_, _ = s.db.ExecContext(ctx, `UPDATE crm_contacts SET tags=$1, updated_at=NOW() WHERE id=$2`, string(updatedBytes), id)
					}
				}
			}
		}
	} else if s.crm != nil {
		s.crm.Lock()
		for id, c := range s.crm.contacts {
			if strings.EqualFold(c.Email, email) {
				hasUnsub := false
				for _, t := range c.Tags {
					if strings.EqualFold(normalizeTagKey(t), "unsubscribed") {
						hasUnsub = true
						break
					}
				}
				if !hasUnsub {
					c.Tags = append(c.Tags, "unsubscribed")
					c.UpdatedAt = time.Now()
					s.crm.contacts[id] = c
				}
			}
		}
		s.crm.Unlock()
	}
	return nil
}

// isEmailUnsubscribed checks if an email or contact has unsubscribed.
func (s *Server) isEmailUnsubscribed(email string, tags []string) bool {
	clean := strings.ToLower(strings.TrimSpace(email))
	if _, ok := s.broadcastUnsubscribes.Load(clean); ok {
		return true
	}
	for _, t := range tags {
		if strings.EqualFold(normalizeTagKey(t), "unsubscribed") {
			return true
		}
	}
	return false
}

// broadcastBaseURL determines public base URL for standard RFC 8058 unsubscribe links.
func (s *Server) broadcastBaseURL(r *http.Request) string {
	if value := strings.TrimSpace(os.Getenv("CUAN_PUBLIC_APP_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	if s != nil && s.cfg != nil {
		if domain := strings.TrimSpace(s.cfg.BaseDomain); domain != "" && domain != "localhost" {
			return "https://" + domain
		}
	}
	if r != nil {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := r.Host
		if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
			host = fwd
		}
		if host != "" {
			return scheme + "://" + host
		}
	}
	return "https://app.cuan.work"
}

func formatBroadcastText(content, unsubURL string) string {
	return fmt.Sprintf("%s\n\n---\nAnda menerima email ini karena terdaftar pada kontak kami.\nUntuk berhenti berlangganan, klik tautan berikut:\n%s\n", strings.TrimSpace(content), unsubURL)
}

func formatBroadcastHTML(content, unsubURL string) string {
	paragraphs := strings.Split(strings.TrimSpace(content), "\n\n")
	var sb strings.Builder
	sb.WriteString(`<div style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; font-size: 15px; line-height: 1.6; color: #18181b; max-width: 600px; margin: 0 auto; padding: 20px 0;">`)
	for _, p := range paragraphs {
		lines := strings.Split(p, "\n")
		escapedLines := make([]string, 0, len(lines))
		for _, l := range lines {
			escapedLines = append(escapedLines, html.EscapeString(l))
		}
		sb.WriteString(fmt.Sprintf(`<p style="margin: 0 0 16px 0;">%s</p>`, strings.Join(escapedLines, "<br/>")))
	}
	sb.WriteString(fmt.Sprintf(`<hr style="border: none; border-top: 1px solid #e4e4e7; margin: 32px 0 16px 0;" /><p style="font-size: 12px; color: #71717a; margin: 0;">Anda menerima email ini karena terdaftar pada kontak kami. <a href="%s" style="color: #71717a; text-decoration: underline;">Berhenti berlangganan</a></p></div>`, html.EscapeString(unsubURL)))
	return sb.String()
}

// handleBroadcastTags returns all tags in CRM with the count of contacts that have a valid email.
func (s *Server) handleBroadcastTags(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	contacts, err := s.crmList(r.Context())
	if err != nil {
		respondJSONError(w, 503, "Could not load contacts")
		return
	}

	tagCounts := make(map[string]int)
	for _, c := range contacts {
		if strings.TrimSpace(c.Email) == "" {
			continue
		}
		seenInContact := make(map[string]bool)
		for _, t := range c.Tags {
			clean := strings.TrimSpace(t)
			clean = strings.TrimPrefix(clean, "#")
			k := strings.ToLower(clean)
			if k != "" && !seenInContact[k] {
				seenInContact[k] = true
				tagCounts[clean]++
			}
		}
	}

	var tags []BroadcastTagSummary
	for tag, count := range tagCounts {
		tags = append(tags, BroadcastTagSummary{Tag: tag, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return strings.ToLower(tags[i].Tag) < strings.ToLower(tags[j].Tag)
	})

	respondJSON(w, http.StatusOK, map[string]any{"tags": tags})
}

// resolveBroadcastAudience resolves unique email recipients from CRM tags and custom emails with strict deduplication and unsubscribe checks.
func (s *Server) resolveBroadcastAudience(ctx context.Context, req BroadcastAudienceRequest) (*BroadcastAudienceResponse, error) {
	if len(req.Tags) > 50 {
		return nil, errors.New("Up to 50 tags are allowed")
	}

	selectedTagMap := make(map[string]bool)
	for _, t := range req.Tags {
		t = strings.TrimSpace(t)
		if len(t) == 0 || utf8.RuneCountInString(t) > 40 {
			return nil, errors.New("Tags must contain 1–40 characters")
		}
		k := normalizeTagKey(t)
		if k != "" {
			selectedTagMap[k] = true
		}
	}

	customList, err := extractCustomEmailStrings(req.CustomEmails)
	if err != nil {
		return nil, err
	}
	if len(customList) > 500 {
		return nil, errors.New("Up to 500 custom emails are allowed")
	}

	recipientMap := make(map[string]*BroadcastRecipient)
	var order []string

	// 1. Resolve CRM contacts matching selected tags
	if len(selectedTagMap) > 0 {
		contacts, err := s.crmList(ctx)
		if err != nil {
			return nil, errors.New("Could not load contacts")
		}

		for _, c := range contacts {
			if strings.TrimSpace(c.Email) == "" {
				continue
			}
			cEmail, _, parseErr := parseAndValidateEmail(c.Email)
			if parseErr != nil {
				continue
			}

			// Exclude unsubscribed contacts
			if s.isEmailUnsubscribed(cEmail, c.Tags) {
				continue
			}

			var matchingTags []string
			for _, ct := range c.Tags {
				if selectedTagMap[normalizeTagKey(ct)] {
					matchingTags = append(matchingTags, ct)
				}
			}
			if len(matchingTags) == 0 {
				continue
			}

			if existing, ok := recipientMap[cEmail]; ok {
				// Contact already present; merge tags
				for _, mt := range matchingTags {
					found := false
					for _, et := range existing.Tags {
						if strings.EqualFold(mt, et) {
							found = true
							break
						}
					}
					if !found {
						existing.Tags = append(existing.Tags, mt)
					}
				}
				if existing.ContactID == "" {
					existing.ContactID = c.ID
				}
			} else {
				displayName := strings.TrimSpace(c.Name)
				if displayName == "" {
					displayName = cEmail
				}
				recipientMap[cEmail] = &BroadcastRecipient{
					Email:     cEmail,
					Name:      displayName,
					Source:    "crm",
					ContactID: c.ID,
					Tags:      matchingTags,
				}
				order = append(order, cEmail)
			}
		}
	}

	// 2. Resolve custom emails with validation & deduplication
	for _, rawEmail := range customList {
		email, name, valErr := parseAndValidateEmail(rawEmail)
		if valErr != nil {
			return nil, valErr
		}

		// Exclude unsubscribed emails
		if s.isEmailUnsubscribed(email, nil) {
			continue
		}

		if existing, ok := recipientMap[email]; ok {
			// Deduplicated with existing entry
			if existing.Source == "crm" {
				existing.Source = "both"
			}
			if (existing.Name == "" || existing.Name == email) && name != "" {
				existing.Name = name
			}
		} else {
			displayName := name
			if displayName == "" {
				displayName = email
			}
			recipientMap[email] = &BroadcastRecipient{
				Email:  email,
				Name:   displayName,
				Source: "custom",
			}
			order = append(order, email)
		}
	}

	recipients := make([]BroadcastRecipient, 0, len(order))
	crmCount := 0
	customCount := 0

	for _, emailKey := range order {
		rec := *recipientMap[emailKey]
		if rec.Source == "crm" || rec.Source == "both" {
			crmCount++
		}
		if rec.Source == "custom" || rec.Source == "both" {
			customCount++
		}
		recipients = append(recipients, rec)
	}

	// Sort alphabetically by name then email
	sort.Slice(recipients, func(i, j int) bool {
		nameI := strings.ToLower(recipients[i].Name)
		nameJ := strings.ToLower(recipients[j].Name)
		if nameI != nameJ {
			return nameI < nameJ
		}
		return recipients[i].Email < recipients[j].Email
	})

	return &BroadcastAudienceResponse{
		TotalUnique: len(recipients),
		CRMCount:    crmCount,
		CustomCount: customCount,
		Recipients:  recipients,
	}, nil
}

// handleBroadcastAudience handles POST /api/v1/contacts/broadcast/audience and /preview.
func (s *Server) handleBroadcastAudience(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}

	var req BroadcastAudienceRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Invalid JSON payload for audience resolution")
		return
	}

	resp, err := s.resolveBroadcastAudience(r.Context(), req)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleBroadcastSend handles POST /api/v1/contacts/broadcast/send with strict idempotency and RFC 8058 headers.
func (s *Server) handleBroadcastSend(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}

	var req BroadcastSendRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "Format payload broadcast tidak valid")
		return
	}

	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		respondJSONError(w, http.StatusBadRequest, "Subjek broadcast tidak boleh kosong")
		return
	}
	if strings.ContainsAny(subject, "\r\n") {
		respondJSONError(w, http.StatusBadRequest, "Subjek broadcast tidak boleh mengandung baris baru")
		return
	}
	if utf8.RuneCountInString(subject) > 200 {
		respondJSONError(w, http.StatusBadRequest, "Subjek broadcast melebihi batas 200 karakter")
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondJSONError(w, http.StatusBadRequest, "Konten pesan broadcast tidak boleh kosong")
		return
	}
	if len(content) > 50000 {
		respondJSONError(w, http.StatusBadRequest, "Konten pesan broadcast melebihi batas 50.000 karakter")
		return
	}

	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = "bc-" + db.GenerateUUID()
	}

	// 1. Idempotency Check: Already sent
	if cached, ok := sentBroadcastCache.Load(idempotencyKey); ok {
		resp := cached.(BroadcastSendResponse)
		resp.AlreadySent = true
		resp.Message = "Broadcast sudah pernah dikirim sebelumnya (idempotent)"
		respondJSON(w, http.StatusOK, resp)
		return
	}

	// 2. Concurrency Lock: In-flight check
	broadcastSendMu.Lock()
	if _, loaded := inFlightBroadcasts.LoadOrStore(idempotencyKey, struct{}{}); loaded {
		broadcastSendMu.Unlock()
		respondJSONError(w, http.StatusConflict, "Pengiriman broadcast dengan kunci ini sedang berlangsung")
		return
	}
	broadcastSendMu.Unlock()
	defer inFlightBroadcasts.Delete(idempotencyKey)

	// 3. Resolve audience
	audience, err := s.resolveBroadcastAudience(r.Context(), BroadcastAudienceRequest{
		Tags:         req.Tags,
		CustomEmails: req.CustomEmails,
	})
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if audience.TotalUnique == 0 {
		respondJSONError(w, http.StatusBadRequest, "Tidak ada penerima yang cocok untuk pengiriman broadcast")
		return
	}

	// 4. Resolve Resend client and sender settings
	settings, err := s.GetResendSettings(r.Context())
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

	client, err := s.ResendClient(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "Layanan pengiriman email (Resend) belum aktif: "+err.Error())
		return
	}

	// 5. Batch dispatch loop
	baseURL := s.broadcastBaseURL(r)
	sentCount := 0
	failedCount := 0
	var sendErrors []string

	for _, recipient := range audience.Recipients {
		token := s.generateUnsubscribeToken(recipient.Email)
		unsubURL := fmt.Sprintf("%s/api/v1/contacts/broadcast/unsubscribe?token=%s", baseURL, token)
		perRecipientKey := fmt.Sprintf("bc/%s/%s", idempotencyKey, recipient.Email)

		emailReq := resend.EmailRequest{
			IdempotencyKey: perRecipientKey,
			From:           (&mail.Address{Name: fromName, Address: from}).String(),
			To:             []string{recipient.Email},
			Subject:        subject,
			Text:           formatBroadcastText(content, unsubURL),
			HTML:           formatBroadcastHTML(content, unsubURL),
			Headers: map[string]string{
				"List-Unsubscribe":      fmt.Sprintf("<%s>", unsubURL),
				"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			},
		}

		_, sendErr := client.SendEmail(r.Context(), emailReq)
		if sendErr != nil {
			failedCount++
			sendErrors = append(sendErrors, fmt.Sprintf("%s: %s", recipient.Email, sendErr.Error()))
		} else {
			sentCount++
		}
	}

	status := "success"
	message := fmt.Sprintf("Berhasil mengirim broadcast ke %d penerima", sentCount)
	if failedCount > 0 {
		if sentCount == 0 {
			status = "failed"
			message = fmt.Sprintf("Gagal mengirim broadcast ke seluruh penerima (%d gagal)", failedCount)
		} else {
			status = "partial"
			message = fmt.Sprintf("Terkirim ke %d penerima, %d gagal", sentCount, failedCount)
		}
	}

	resp := BroadcastSendResponse{
		Status:          status,
		TotalRecipients: audience.TotalUnique,
		SentCount:       sentCount,
		FailedCount:     failedCount,
		Errors:          sendErrors,
		IdempotencyKey:  idempotencyKey,
		AlreadySent:     false,
		Message:         message,
	}

	if status == "success" || status == "partial" {
		sentBroadcastCache.Store(idempotencyKey, resp)
	}

	if status == "failed" {
		respondJSON(w, http.StatusBadGateway, resp)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleBroadcastUnsubscribe handles both GET (browser link) and POST (RFC 8058 One-Click).
func (s *Server) handleBroadcastUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" && r.Method == http.MethodPost {
		_ = r.ParseForm()
		token = r.Form.Get("token")
	}

	email, err := s.validateUnsubscribeToken(token)
	if err != nil {
		if r.Method == http.MethodPost {
			respondJSONError(w, http.StatusBadRequest, "Invalid or expired unsubscribe token: "+err.Error())
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `<!DOCTYPE html><html lang="id"><head><meta charset="utf-8"><title>Tautan Tidak Valid</title></head><body style="font-family:sans-serif;text-align:center;padding:40px;"><h2>Tautan Berhenti Berlangganan Tidak Valid</h2><p>Token tautan tidak valid atau telah kedaluwarsa.</p></body></html>`)
		}
		return
	}

	if err := s.markEmailUnsubscribed(r.Context(), email); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal memproses pembatalan langganan: "+err.Error())
		return
	}

	if r.Method == http.MethodPost {
		// RFC 8058 One-Click response
		respondJSON(w, http.StatusOK, map[string]any{
			"status":       "success",
			"unsubscribed": true,
			"email":        email,
			"message":      "Berhasil berhenti berlangganan",
		})
		return
	}

	// Browser GET confirmation page (Clean Bear Notes / Things 3 design)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	escapedEmail := html.EscapeString(email)
	pageHTML := strings.Replace(unsubscribeHTMLTemplate, "{{EMAIL}}", escapedEmail, 1)
	_, _ = w.Write([]byte(pageHTML))
}

const unsubscribeHTMLTemplate = `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Berhenti Berlangganan</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #fafafa; color: #18181b; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; padding: 24px; }
    .card { background: #ffffff; border: 1px solid #e4e4e7; border-radius: 12px; padding: 36px 32px; max-width: 440px; width: 100%; box-shadow: 0 1px 3px rgba(0,0,0,0.04); text-align: center; }
    .icon { width: 44px; height: 44px; border-radius: 50%; background: #f0fdf4; color: #16a34a; display: inline-flex; align-items: center; justify-content: center; font-size: 20px; margin-bottom: 16px; }
    h1 { font-size: 18px; font-weight: 600; margin: 0 0 8px 0; color: #09090b; }
    p { font-size: 13px; line-height: 1.5; color: #71717a; margin: 0 0 16px 0; }
    .email { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 12px; background: #f4f4f5; padding: 6px 12px; border-radius: 6px; color: #27272a; display: inline-block; margin-bottom: 20px; word-break: break-all; }
    .footer { font-size: 12px; color: #a1a1aa; border-top: 1px solid #f4f4f5; padding-top: 16px; margin-top: 8px; }
  </style>
</head>
<body>
  <div class="card">
    <div class="icon">✓</div>
    <h1>Berhenti Berlangganan</h1>
    <p>Alamat email berikut telah dihapus dari daftar penerima broadcast:</p>
    <div class="email">{{EMAIL}}</div>
    <div class="footer">Anda tidak akan menerima email pengumuman atau broadcast lagi di alamat ini.</div>
  </div>
</body>
</html>`
