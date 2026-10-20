package server

import (
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type ownerPreferences struct {
	PushSubject   string                  `json:"push_subject,omitempty"`
	DisplayName   string                  `json:"display_name"`
	Notifications notificationPreferences `json:"notifications"`
	Items         []ownerNotification     `json:"items"`
	Subscriptions []ownerPushSubscription `json:"subscriptions,omitempty"`
	PrivateKey    string                  `json:"vapid_private_key,omitempty"`
	PublicKey     string                  `json:"vapid_public_key,omitempty"`
}
type notificationPreferences struct {
	Approval  bool   `json:"approval"`
	Blocker   bool   `json:"blocker"`
	Completed bool   `json:"completed"`
	Customer  bool   `json:"customer"`
	Quiet     bool   `json:"quiet"`
	Start     int    `json:"start_hour"`
	End       int    `json:"end_hour"`
	Timezone  string `json:"timezone"`
}
type ownerNotification struct {
	ID    string    `json:"id"`
	Kind  string    `json:"kind"`
	Title string    `json:"title"`
	Href  string    `json:"href"`
	At    time.Time `json:"created_at"`
	Read  bool      `json:"read"`
}
type ownerPushSubscription struct {
	Subscription webpush.Subscription `json:"subscription"`
	Since        time.Time            `json:"since"`
	Sent         map[string]bool      `json:"sent"`
}

func defaultOwnerPreferences() ownerPreferences {
	return ownerPreferences{Notifications: notificationPreferences{Approval: true, Blocker: true, Completed: true, Customer: true, Start: 22, End: 8, Timezone: "UTC"}, Items: []ownerNotification{}}
}
func (s *Server) notificationCandidates(ctx context.Context, owner string) ([]ownerNotification, error) {
	items := []ownerNotification{}
	if s.taskSvc != nil {
		tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
		if err != nil {
			return nil, err
		}
		for _, t := range tasks {
			kind := ""
			title := ""
			switch t.Status {
			case "done":
				kind = "completed"
				title = "Work completed"
			case "review":
				kind = "approval"
				title = "Work needs your review"
			case "blocked":
				kind = "blocker"
				title = "Work needs attention"
			}
			if kind == "" {
				continue
			}
			attempt := ""
			if t.LatestAttempt != nil {
				attempt = t.LatestAttempt.ID
			}
			href := "/tasks/" + url.PathEscape(t.ID)
			if t.ProjectID != "" {
				href = "/projects/" + url.PathEscape(t.ProjectID) + "/tasks/" + url.PathEscape(t.ID)
			}
			if t.SourceRoomID != "" {
				href = "/inbox/" + url.PathEscape(t.SourceRoomID)
			}
			items = append(items, ownerNotification{ID: "task:" + t.ID + ":" + t.Status + ":" + attempt, Kind: kind, Title: title + ": " + truncateProjectText(t.Title, 160), Href: href, At: t.UpdatedAt})
		}
	}
	if s.db != nil {
		chat, err := s.notificationChatCandidatesSQL(ctx, owner)
		if err != nil {
			return nil, err
		}
		items = append(items, chat...)
		sort.SliceStable(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
		if len(items) > 100 {
			items = items[:100]
		}
		return items, nil
	}
	if s.projectStudioRepo != nil {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			rooms, err := s.projectStudioRepo.ListAllRooms(ctx)
			if err != nil {
				return nil, err
			}
			for _, room := range rooms {
				if roomAgentID(room.Topic) == "" {
					continue
				}
				requests, err := store.ListInboxRequests(ctx, room.ID)
				if err != nil {
					return nil, err
				}
				allowedReplies := map[string]bool{}
				for _, q := range requests {
					if q.OwnerID == owner {
						allowedReplies[q.ReplyID] = true
					}
				}
				for _, q := range requests {
					if q.OwnerID != owner {
						continue
					}
					if q.State == "failed" || q.State == "interrupted" {
						items = append(items, ownerNotification{ID: "request:" + room.ID + ":" + q.RequestID, Kind: "blocker", Title: "Agent response needs attention", Href: "/inbox/" + url.PathEscape(room.ID), At: q.CreatedAt})
					}
				}
				messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
				if err != nil {
					return nil, err
				}
				for _, m := range messages {
					if m.AuthorType == "agent" && allowedReplies[m.ID] {
						_, approval := splitInboxApproval(m.AuthorType, m.Content)
						if approval != "" {
							items = append(items, ownerNotification{ID: "approval:" + room.ID + ":" + m.ID, Kind: "approval", Title: approval, Href: "/inbox/" + url.PathEscape(room.ID), At: m.CreatedAt})
						}
					}
				}
			}
		}
		contacts, err := s.crmList(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range contacts {
			messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, c.SessionUUID)
			if err != nil {
				if errors.Is(err, db.ErrRoomNotFound) {
					continue
				}
				return nil, err
			}
			for _, m := range messages {
				if m.AuthorType == "user" && m.AuthorID != "operator_user" {
					items = append(items, ownerNotification{ID: "customer:" + c.SessionUUID + ":" + m.ID, Kind: "customer", Title: "New customer message", Href: "/inbox/" + url.PathEscape(c.SessionUUID), At: m.CreatedAt})
				}
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	if len(items) > 100 {
		items = items[:100]
	}
	return items, nil
}
func mergeNotifications(p *ownerPreferences, items []ownerNotification) {
	seen := map[string]bool{}
	for _, item := range p.Items {
		seen[item.ID] = true
	}
	for _, item := range items {
		if !seen[item.ID] {
			p.Items = append(p.Items, item)
			seen[item.ID] = true
		}
	}
	sort.SliceStable(p.Items, func(i, j int) bool { return p.Items[i].At.After(p.Items[j].At) })
	if len(p.Items) > 100 {
		p.Items = p.Items[:100]
	}
}
func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	items, err := s.notificationCandidates(r.Context(), owner)
	if err != nil {
		respondJSONError(w, 503, "Could not load notifications. Try again.")
		return
	}
	ownerPreferencesMu.Lock()
	defer ownerPreferencesMu.Unlock()
	p, err := readOwnerPreferences(owner)
	if err != nil {
		respondJSONError(w, 503, "Could not load notifications. Try again.")
		return
	}
	mergeNotifications(&p, items)
	if r.Method == "PATCH" {
		var req struct {
			IDs []string `json:"ids"`
			All bool     `json:"all"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32000)).Decode(&req) != nil {
			respondJSONError(w, 400, "Choose notifications to mark as read.")
			return
		}
		for i := range p.Items {
			if req.All {
				p.Items[i].Read = true
			}
			for _, id := range req.IDs {
				if id == p.Items[i].ID {
					p.Items[i].Read = true
				}
			}
		}
	}
	if err = writeOwnerPreferences(owner, p); err != nil {
		respondJSONError(w, 503, "Could not save notification state. Try again.")
		return
	}
	respondJSON(w, 200, map[string]any{"items": p.Items})
}
func validNotificationPreferences(p notificationPreferences) bool {
	_, err := time.LoadLocation(p.Timezone)
	return err == nil && p.Start >= 0 && p.Start <= 23 && p.End >= 0 && p.End <= 23 && (!p.Quiet || p.Start != p.End)
}
func quietAt(p notificationPreferences, now time.Time) bool {
	if !p.Quiet {
		return false
	}
	zone, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return true
	}
	hour := now.In(zone).Hour()
	if p.Start < p.End {
		return hour >= p.Start && hour < p.End
	}
	return hour >= p.Start || hour < p.End
}
func (s *Server) handleNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	ownerPreferencesMu.Lock()
	defer ownerPreferencesMu.Unlock()
	p, err := readOwnerPreferences(owner)
	if err != nil {
		respondJSONError(w, 503, "Could not load preferences.")
		return
	}
	if r.Method == "PATCH" {
		var req notificationPreferences
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil || !validNotificationPreferences(req) {
			respondJSONError(w, 400, "Choose valid quiet hours and a timezone.")
			return
		}
		p.Notifications = req
		if err = writeOwnerPreferences(owner, p); err != nil {
			respondJSONError(w, 503, "Could not save notification preferences.")
			return
		}
	}
	respondJSON(w, 200, map[string]any{"preferences": p.Notifications, "devices": len(p.Subscriptions)})
}
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

func sendOwnerPush(ctx context.Context, p ownerPreferences, sub webpush.Subscription, item ownerNotification) (int, error) {
	// Avoid exposing project titles or message contents on a locked device.
	raw, _ := json.Marshal(map[string]string{"title": "Cuan", "body": "There is an update that needs your attention.", "href": item.Href, "tag": item.ID})
	response, err := webpush.SendNotificationWithContext(ctx, raw, &sub, &webpush.Options{HTTPClient: notificationHTTPClient, Subscriber: p.PushSubject, VAPIDPublicKey: p.PublicKey, VAPIDPrivateKey: p.PrivateKey, TTL: 600})
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode, nil
}
func (s *Server) dispatchOwnerNotifications(ctx context.Context, owner string) {
	ownerPreferencesMu.Lock()
	p, err := readOwnerPreferences(owner)
	ownerPreferencesMu.Unlock()
	if err != nil || len(p.Subscriptions) == 0 {
		return
	}
	items, err := s.notificationCandidates(ctx, owner)
	if err != nil {
		return
	}
	ownerPreferencesMu.Lock()
	p, err = readOwnerPreferences(owner)
	if err == nil {
		mergeNotifications(&p, items)
		err = writeOwnerPreferences(owner, p)
	}
	ownerPreferencesMu.Unlock()
	if err != nil || quietAt(p.Notifications, time.Now()) {
		return
	}
	for _, sub := range p.Subscriptions {
		sent := 0
		for i := len(p.Items) - 1; i >= 0; i-- {
			item := p.Items[i]
			if ctx.Err() != nil {
				return
			}
			if sent >= 5 {
				break
			}
			if item.Read || sub.Sent[item.ID] || !item.At.After(sub.Since) || time.Since(item.At) > 24*time.Hour || !enabledNotification(p.Notifications, item.Kind) {
				continue
			}
			code, e := sendOwnerPush(ctx, p, sub.Subscription, item)
			if e != nil {
				break
			}
			ownerPreferencesMu.Lock()
			current, err := readOwnerPreferences(owner)
			if err == nil {
				for j := range current.Subscriptions {
					device := &current.Subscriptions[j]
					if device.Subscription.Endpoint != sub.Subscription.Endpoint {
						continue
					}
					if code == 404 || code == 410 {
						current.Subscriptions = append(current.Subscriptions[:j], current.Subscriptions[j+1:]...)
					} else if code >= 200 && code < 300 {
						if device.Sent == nil {
							device.Sent = map[string]bool{}
						}
						device.Sent[item.ID] = true
						valid := map[string]bool{}
						for _, n := range current.Items {
							if device.Sent[n.ID] {
								valid[n.ID] = true
							}
						}
						device.Sent = valid
					}
					break
				}
				_ = writeOwnerPreferences(owner, current)
			}
			ownerPreferencesMu.Unlock()
			if code < 200 || code >= 300 {
				break
			}
			sent++
		}
	}
}
func (s *Server) StartNotificationDispatcher(ctx context.Context) {
	go func() {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if s.authSvc != nil {
					owner, err := s.authSvc.OwnerID(ctx)
					if err == nil {
						s.dispatchOwnerNotifications(ctx, owner)
					}
				}
			}
		}
	}()
}

func (s *Server) notificationChatCandidatesSQL(ctx context.Context, owner string) ([]ownerNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
 SELECT kind,id,room_id,created_at,content FROM (
 SELECT 'request' AS kind,q.request_id::text AS id,q.room_id::text AS room_id,q.created_at,'' AS content FROM inbox_requests q WHERE q.owner_id=$1 AND q.state IN ('failed','interrupted')
 UNION ALL
 SELECT 'approval',m.id::text,m.room_id::text,m.created_at,m.content FROM project_messages m JOIN inbox_requests q ON q.reply_id=m.id WHERE q.owner_id=$1 AND m.author_type='agent' AND m.content LIKE '%cuan-approval%'
 UNION ALL
 SELECT 'customer',m.id::text,m.room_id::text,m.created_at,'' FROM project_messages m JOIN crm_contacts c ON c.session_uuid=m.room_id WHERE m.author_type='user' AND COALESCE(m.author_id,'')<>'operator_user'
 ) candidates ORDER BY created_at DESC LIMIT 100`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ownerNotification{}
	for rows.Next() {
		var kind, id, room, content string
		var at time.Time
		if err = rows.Scan(&kind, &id, &room, &at, &content); err != nil {
			return nil, err
		}
		title := ""
		category := kind
		switch kind {
		case "request":
			category = "blocker"
			title = "Agent response needs attention"
		case "customer":
			title = "New customer message"
		case "approval":
			_, title = splitInboxApproval("agent", content)
			if title == "" {
				continue
			}
		}
		items = append(items, ownerNotification{ID: kind + ":" + room + ":" + id, Kind: category, Title: title, Href: "/inbox/" + url.PathEscape(room), At: at})
	}
	return items, rows.Err()
}
