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
	if raw == "" || raw == "Pekerjaan Selesai" || raw == "Tugas Selesai" {
		switch kind {
		case "completed":
			return "✓ Selesai"
		case "approval":
			return "Tinjauan Tugas"
		case "blocker":
			return "Perhatian Diperlukan"
		default:
			return "Pekerjaan Selesai"
		}
	}
	raw = strings.Join(strings.Fields(raw), " ")

	if kind == "completed" {
		clean := raw
		for _, pfx := range []string{"✓ Selesai:", "✓ Selesai", "Selesai:"} {
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

func pushIconForNotification(item ownerNotification, tag string) string {
	isTask := strings.HasPrefix(tag, "task:") || strings.HasPrefix(item.ID, "task:") || strings.Contains(item.Href, "/tasks/")
	if isTask {
		switch {
		case item.Kind == "completed" || strings.Contains(item.ID, ":done") || item.Status == "Completed":
			return "/icons/task-done.png"
		case item.Kind == "approval" || strings.Contains(item.ID, ":review") || item.Status == "Review requested":
			return "/icons/task-review.png"
		case item.Kind == "blocker" || strings.Contains(item.ID, ":blocked") || item.Status == "Needs attention":
			return "/icons/task-blocker.png"
		default:
			return "/icons/task-done.png"
		}
	}

	switch item.Kind {
	case "approval":
		return "/icons/task-review.png"
	case "blocker":
		return "/icons/task-blocker.png"
	default:
		return "/icons/cuan-192.png"
	}
}

func buildPushPayload(item ownerNotification) map[string]string {
	tag := item.ID
	if strings.HasPrefix(item.ID, "approval:") {
		tag = "agent:" + strings.TrimPrefix(item.ID, "approval:")
	} else if strings.HasPrefix(item.ID, "request:") {
		tag = "agent:" + strings.TrimPrefix(item.ID, "request:")
	}

	isTask := strings.HasPrefix(tag, "task:") || strings.HasPrefix(item.ID, "task:") || strings.Contains(item.Href, "/tasks/")

	title := item.Title
	if item.Kind == "customer" {
		if title == "" || title == "New customer message" {
			if item.Context != "" {
				title = "Pesan Baru dari " + item.Context
			} else {
				title = "Pesan Baru dari Pengunjung"
			}
		}
	} else if isTask {
		title = formatTaskPushTitle(title, item.Kind)
	} else if item.Kind == "completed" {
		if title == "" {
			title = "Asisten AI"
		}
	} else if item.Kind == "approval" || strings.HasPrefix(tag, "agent:") {
		if title == "" {
			title = "Asisten AI"
		}
	} else if item.Kind == "blocker" {
		if title == "" {
			title = "Perhatian Diperlukan"
		}
	}
	if title == "" {
		title = "Cuan"
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
		} else if item.Title != "" && item.Title != title {
			body = item.Title
		} else if item.Kind == "customer" {
			body = "Pesan baru dari pelanggan di Live Chat."
		} else if item.Kind == "approval" {
			body = "Asisten AI membutuhkan persetujuan Anda."
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

	icon := pushIconForNotification(item, tag)

	return map[string]string{
		"title":  title,
		"body":   body,
		"href":   href,
		"tag":    tag,
		"badge":  "/icons/cuan-badge-96.png",
		"icon":   icon,
		"kind":   item.Kind,
		"status": item.Status,
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
