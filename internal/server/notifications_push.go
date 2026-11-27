package server

import (
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func validatePushSubscription(sub webpush.Subscription) bool {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || len(sub.Endpoint) > 4096 {
		return false
	}
	host := strings.ToLower(u.Hostname())
	allowed := host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" || host == "push.services.mozilla.com" || host == "web.push.apple.com" || strings.HasSuffix(host, ".notify.windows.com")
	if !allowed {
		return false
	}
	key, e := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.P256dh, "="))
	if e != nil {
		return false
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), key)
	if x == nil || y == nil {
		return false
	}
	secret, e := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.Auth, "="))
	return e == nil && len(secret) == 16
}

func (s *Server) handlePushSubscription(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	ownerPreferencesMu.Lock()
	defer ownerPreferencesMu.Unlock()
	p, err := readOwnerPreferences(owner)
	if err != nil {
		respondJSONError(w, 503, "Could not load push settings.")
		return
	}
	if r.Method == "GET" {
		if p.PushSubject == "" {
			p.PushSubject = "https://" + r.Host
			if p.PrivateKey != "" {
				err = writeOwnerPreferences(owner, p)
			}
		}
		if p.PrivateKey == "" {
			p.PrivateKey, p.PublicKey, err = webpush.GenerateVAPIDKeys()
			if err == nil {
				err = writeOwnerPreferences(owner, p)
			}
		}
		if err != nil {
			respondJSONError(w, 503, "Could not prepare push notifications.")
			return
		}
		respondJSON(w, 200, map[string]string{"public_key": p.PublicKey})
		return
	}
	var sub webpush.Subscription
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&sub) != nil || !validatePushSubscription(sub) {
		respondJSONError(w, 400, "Invalid or unsupported push subscription.")
		return
	}
	found := false
	remaining := []ownerPushSubscription{}
	for _, old := range p.Subscriptions {
		if old.Subscription.Endpoint == sub.Endpoint {
			found = true
			if r.Method == "POST" {
				old.Subscription = sub
				remaining = append(remaining, old)
			}
		} else {
			remaining = append(remaining, old)
		}
	}
	if r.Method == "POST" && !found {
		if len(remaining) >= 10 {
			respondJSONError(w, 400, "Up to 10 devices can receive push notifications.")
			return
		}
		if p.PrivateKey == "" {
			respondJSONError(w, 409, "Prepare push settings before subscribing.")
			return
		}
		remaining = append(remaining, ownerPushSubscription{Subscription: sub, Since: time.Now().UTC(), Sent: map[string]bool{}})
	}
	p.Subscriptions = remaining
	if err = writeOwnerPreferences(owner, p); err != nil {
		respondJSONError(w, 503, "Could not save push subscription.")
		return
	}
	respondJSON(w, 200, map[string]bool{"ok": true})
}

func enabledNotification(p notificationPreferences, kind string) bool {
	switch kind {
	case "approval":
		return p.Approval
	case "blocker":
		return p.Blocker
	case "completed":
		return p.Completed
	case "customer":
		return p.Customer
	}
	return false
}

var notificationHTTPClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

func formatTaskPushTitle(rawTitle string, kind string) string {
	raw := strings.TrimSpace(rawTitle)
	if raw == "" || raw == "Pekerjaan Selesai" || raw == "Tugas Selesai" || strings.HasPrefix(raw, "@") {
		switch kind {
		case "completed":
			return "✓ Selesai"
		case "approval":
			return "Butuh Persetujuan"
		case "blocker":
			return "Perhatian Diperlukan"
		default:
			return "Pekerjaan Selesai"
		}
	}
	raw = humanizeTaskTitle(raw)
	raw = strings.Join(strings.Fields(raw), " ")

	if kind == "completed" {
		clean := raw
		for _, pfx := range []string{"✓ Selesai:", "✓ Selesai", "Selesai:", "Pekerjaan Selesai:", "Pekerjaan Selesai"} {
			if strings.HasPrefix(clean, pfx) {
				clean = strings.TrimSpace(strings.TrimPrefix(clean, pfx))
				break
			}
		}
		if clean == "" {
			return "✓ Selesai"
		}
		shortName := clean
		runes := []rune(clean)
		if len(runes) > 42 {
			shortName = strings.TrimSpace(string(runes[:39])) + "..."
		}
		return "✓ Selesai: " + shortName
	}

	runes := []rune(raw)
	if len(runes) > 60 {
		return strings.TrimSpace(string(runes[:57])) + "..."
	}
	return raw
}

func extractRoomID(item ownerNotification) string {
	for _, prefix := range []string{"customer:", "agent:", "approval:", "request:", "room:"} {
		if strings.HasPrefix(item.ID, prefix) {
			rest := strings.TrimPrefix(item.ID, prefix)
			parts := strings.Split(rest, ":")
			if len(parts) > 0 && parts[0] != "" {
				return parts[0]
			}
		}
	}
	if strings.HasPrefix(item.Href, "/inbox/") {
		trimmed := strings.TrimPrefix(item.Href, "/inbox/")
		trimmed = strings.Split(trimmed, "?")[0]
		trimmed = strings.Split(trimmed, "#")[0]
		trimmed = strings.Split(trimmed, "/")[0]
		if unescaped, err := url.PathUnescape(trimmed); err == nil && unescaped != "" {
			return unescaped
		}
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func pushNotificationTag(item ownerNotification) string {
	tag := item.ID
	isTask := strings.HasPrefix(tag, "task:") || strings.HasPrefix(item.ID, "task:") || strings.Contains(item.Href, "/tasks/")
	if isTask {
		if strings.HasPrefix(item.ID, "task:") {
			parts := strings.SplitN(item.ID, ":", 4)
			if len(parts) >= 3 && parts[1] != "" && parts[2] == "done" {
				return "task:" + parts[1] + ":done"
			}
		}
		return tag
	}
	if roomID := extractRoomID(item); roomID != "" {
		return "room:" + roomID
	}
	if strings.HasPrefix(item.ID, "approval:") {
		return "agent:" + strings.TrimPrefix(item.ID, "approval:")
	}
	if strings.HasPrefix(item.ID, "request:") {
		return "agent:" + strings.TrimPrefix(item.ID, "request:")
	}
	return tag
}

// shouldSuppressInboxTaskPush filters out intermediate/internal runner task progress
// on inbox-linked tasks, while preserving terminal deploy/release failure blockers.
func shouldSuppressInboxTaskPush(item ownerNotification) bool {
	if !strings.HasPrefix(item.ID, "task:") || !strings.Contains(item.Href, "/inbox/") {
		return false
	}
	if strings.Contains(item.ID, ":review:") || strings.HasSuffix(item.ID, ":review") {
		return true
	}
	if strings.Contains(item.ID, ":blocked:") || strings.HasSuffix(item.ID, ":blocked") {
		lower := strings.ToLower(item.Summary)
		if strings.Contains(item.Summary, "⚠️ Deploy production gagal") ||
			strings.Contains(lower, "deploy") ||
			strings.Contains(lower, "rilis") {
			return false
		}
		return true
	}
	return false
}

func buildPushPayload(item ownerNotification) map[string]string {
	tag := pushNotificationTag(item)
	isTask := strings.HasPrefix(tag, "task:") || strings.HasPrefix(item.ID, "task:") || strings.Contains(item.Href, "/tasks/")

	title := strings.TrimSpace(item.Title)
	if item.Kind == "customer" {
		if title == "" || title == "New customer message" || title == "Cuan" || strings.HasPrefix(title, "@") {
			if item.Context != "" && item.Context != "Customer" && item.Context != "Pengunjung" {
				title = "Pesan Baru dari " + item.Context
			} else {
				title = "Pesan Baru"
			}
		}
	} else if item.Kind == "approval" || strings.HasPrefix(item.ID, "approval:") || item.Status == "Needs approval" {
		if !isTask || title == "" || title == "@cuan" || strings.HasPrefix(title, "@") || title == "Asisten AI" || title == "Tinjauan Tugas" {
			title = "Butuh Persetujuan"
		} else {
			title = formatTaskPushTitle(title, item.Kind)
		}
	} else if item.Kind == "blocker" || item.Status == "Needs attention" {
		if !isTask || title == "" || title == "@cuan" || strings.HasPrefix(title, "@") || title == "Asisten AI" {
			title = "Perhatian Diperlukan"
		} else {
			title = formatTaskPushTitle(title, item.Kind)
		}
	} else if isTask {
		title = formatTaskPushTitle(title, item.Kind)
	} else if strings.HasPrefix(tag, "agent:") || strings.HasPrefix(item.ID, "agent:") {
		if title == "" || strings.HasPrefix(title, "@") || title == "Asisten AI" || title == "Cuan" {
			title = "Pesan Baru"
		}
	} else if item.Kind == "completed" || item.Status == "Completed" {
		if title == "" || strings.HasPrefix(title, "@") || title == "Asisten AI" || title == "Cuan" {
			title = "Pekerjaan Selesai"
		}
	}
	if title == "" || strings.HasPrefix(title, "@") || title == "Cuan" {
		title = "Pesan Baru"
	}

	body := item.Summary
	if body == "" {
		if isTask {
			if item.Kind == "approval" {
				body = "Hasil pekerjaan siap ditinjau."
			} else if item.Kind == "completed" {
				body = "Pekerjaan telah selesai dan aktif berjalan."
			} else if item.Kind == "blocker" {
				body = "Pekerjaan tertahan dan memerlukan perhatian Anda."
			} else {
				body = "Ada pembaruan pada tugas Anda."
			}
		} else if item.Title != "" && item.Title != title && !strings.HasPrefix(item.Title, "@") {
			body = humanizeTaskTitle(item.Title)
		} else if item.Kind == "customer" {
			body = "Pesan baru dari pelanggan di Live Chat."
		} else if item.Kind == "approval" {
			body = "Rencana pekerjaan baru telah disiapkan dan siap ditinjau."
		} else if item.Kind == "completed" {
			body = "Pekerjaan telah selesai dan aktif berjalan."
		} else if item.Kind == "blocker" {
			body = "Pekerjaan tertahan dan memerlukan perhatian Anda."
		} else {
			body = "Ada pembaruan yang memerlukan perhatian Anda."
		}
	}
	body = cleanNotificationSnippet(body)

	href := strings.TrimSpace(item.Href)
	if href == "" {
		parts := strings.Split(tag, ":")
		if len(parts) >= 2 && parts[1] != "" {
			if strings.HasPrefix(tag, "task:") {
				href = "/tasks/" + url.PathEscape(parts[1])
			} else {
				href = "/inbox/" + url.PathEscape(parts[1])
			}
		} else {
			href = "/inbox"
		}
	}

	return map[string]string{
		"title":    title,
		"body":     body,
		"href":     href,
		"tag":      tag,
		"badge":    "/icons/cuan-badge-96.png",
		"kind":     item.Kind,
		"status":   item.Status,
		"renotify": "false",
	}
}

func sendOwnerPush(ctx context.Context, p ownerPreferences, sub webpush.Subscription, item ownerNotification) (int, error) {
	subject := p.PushSubject
	if subject == "" {
		subject = "mailto:cuan-push@cuan.local"
	}
	payload := buildPushPayload(item)
	raw, _ := json.Marshal(payload)
	response, err := webpush.SendNotificationWithContext(ctx, raw, &sub, &webpush.Options{
		HTTPClient:      notificationHTTPClient,
		Subscriber:      subject,
		VAPIDPublicKey:  p.PublicKey,
		VAPIDPrivateKey: p.PrivateKey,
		TTL:             600,
	})
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode, nil
}
