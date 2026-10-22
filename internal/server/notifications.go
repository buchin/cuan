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
	"github.com/buchin/cuan/internal/agent"
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
	Context     string    `json:"context,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Status      string    `json:"status,omitempty"`
	ActionLabel string    `json:"action_label,omitempty"`
	Resolved    bool      `json:"resolved,omitempty"`
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Href        string    `json:"href"`
	At          time.Time `json:"created_at"`
	Read        bool      `json:"read"`
}
type ownerPushSubscription struct {
	Subscription webpush.Subscription `json:"subscription"`
	Since        time.Time            `json:"since"`
	Sent         map[string]bool      `json:"sent"`
}

func defaultOwnerPreferences() ownerPreferences {
	return ownerPreferences{Notifications: notificationPreferences{Approval: true, Blocker: true, Completed: true, Customer: true, Start: 22, End: 8, Timezone: "UTC"}, Items: []ownerNotification{}}
}

type notificationTaskState struct{ Status, Attempt, Title, Context, Href string }

func notificationText(text string, limit int) string {
	return truncateProjectText(agent.MaskSecrets(strings.Join(strings.Fields(text), " "), nil), limit)
}
func notificationContext(title, project string) string {
	if value := strings.TrimSpace(title); value != "" && value != "New conversation" {
		return notificationText(value, 120)
	}
	return notificationText(project, 120)
}
func replyNotification(q db.InboxRequest, context, nextState string) ownerNotification {
	n := ownerNotification{ID: "request:" + q.RoomID + ":" + q.RequestID, Kind: "blocker", Title: "Reply could not finish", Context: context, Summary: "Your message is saved. Open the conversation for recovery options.", Status: "Needs attention", ActionLabel: "Check reply", Href: "/inbox/" + url.PathEscape(q.RoomID), At: q.CreatedAt}
	lower := strings.ToLower(q.SafeError)
	switch {
	case nextState == "completed":
		n.Title = "Reply recovered"
		n.Status = "Recovered"
		n.Summary = "Cuan completed this reply after retrying."
		n.Resolved = true
		n.ActionLabel = "View conversation"
	case nextState == "queued" || nextState == "running" || nextState == "cancelling":
		n.Title = "Cuan is trying again"
		n.Status = "Recovering"
		n.Summary = "Your message is saved. Cuan is continuing this reply automatically."
		n.Resolved = true
		n.ActionLabel = "View conversation"
	case nextState != "":
		n.Status = "Updated"
		n.Summary = "A newer reply attempt is available in this conversation."
		n.Resolved = true
		n.ActionLabel = "View conversation"
	case q.State == "interrupted":
		n.Title = "Reply interrupted"
	case strings.Contains(lower, "quota") || strings.Contains(lower, "no allowed account/model route"):
		n.Title = "AI quota unavailable"
		n.Summary = "Your message is saved. No available AI route could handle this reply."
	case strings.Contains(lower, "sign-in") || strings.Contains(lower, "agent connection"):
		n.Title = "AI connection needs attention"
		n.Summary = "Open the conversation to check the AI connection and continue."
	}
	return n
}
func proposalNotification(room, id, title, context, decision string, at time.Time) ownerNotification {
	n := ownerNotification{ID: "approval:" + room + ":" + id, Kind: "approval", Title: notificationText(title, 160), Context: context, Status: "Needs approval", ActionLabel: "Review proposal", Href: "/inbox/" + url.PathEscape(room), At: at}
	if decision != "" {
		n.Resolved = true
		n.ActionLabel = "View conversation"
		n.Status = "Approved"
		if decision == "defer" {
			n.Status = "Saved for later"
		}
	}
	return n
}
func (s *Server) notificationCandidates(ctx context.Context, owner string) ([]ownerNotification, error) {
	items, _, err := s.notificationSnapshot(ctx, owner)
	return items, err
}
func (s *Server) notificationSnapshot(ctx context.Context, owner string) ([]ownerNotification, map[string]notificationTaskState, error) {
	retained, err := readOwnerPreferences(owner)
	if err != nil {
		return nil, nil, err
	}
	retainedIDs := make([]string, 0, len(retained.Items))
	known := map[string]bool{}
	for _, item := range retained.Items {
		retainedIDs = append(retainedIDs, item.ID)
		known[item.ID] = true
	}
	items := []ownerNotification{}
	states := map[string]notificationTaskState{}
	if s.taskSvc != nil {
		tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
		if err != nil {
			return nil, nil, err
		}
		for _, t := range tasks {
			attempt := ""
			if t.LatestAttempt != nil {
				attempt = t.LatestAttempt.ID
			}
			href := "/tasks/" + url.PathEscape(t.ID)
			if t.ProjectID != "" {
				href = "/projects/" + url.PathEscape(t.ProjectID) + "/tasks/" + url.PathEscape(t.ID)
			}
			action := "View work"
			if t.SourceRoomID != "" {
				href = "/inbox/" + url.PathEscape(t.SourceRoomID)
				action = "Open conversation"
			}
			state := notificationTaskState{Status: t.Status, Attempt: attempt, Title: notificationText(t.Title, 160), Context: notificationText(t.ProjectName, 120), Href: href}
			states[t.ID] = state
			kind, status := "", ""
			switch t.Status {
			case "done":
				kind = "completed"
				status = "Completed"
			case "review":
				kind = "approval"
				status = "Review requested"
			case "blocked":
				kind = "blocker"
				status = "Needs attention"
			}
			if kind != "" {
				items = append(items, ownerNotification{ID: "task:" + t.ID + ":" + t.Status + ":" + attempt, Kind: kind, Title: state.Title, Context: state.Context, Status: status, ActionLabel: action, Href: href, At: t.UpdatedAt})
			}
		}
	}
	if s.db != nil {
		chat, err := s.notificationChatCandidatesSQL(ctx, owner, retainedIDs)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, chat...)
	} else if s.projectStudioRepo != nil {
		chat, err := s.notificationChatCandidatesMemory(ctx, owner)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, chat...)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	if len(items) > 100 {
		extra := []ownerNotification{}
		for _, item := range items[100:] {
			if known[item.ID] {
				extra = append(extra, item)
			}
		}
		items = append(items[:100:100], extra...)
	}
	return items, states, nil
}
func (s *Server) notificationChatCandidatesMemory(ctx context.Context, owner string) ([]ownerNotification, error) {
	items := []ownerNotification{}
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
			allowed := map[string]bool{}
			ownedMessages := map[string]bool{}
			latest := map[string]db.InboxRequest{}
			for _, q := range requests {
				if q.OwnerID == owner {
					allowed[q.ReplyID] = true
					ownedMessages[q.MessageID] = true
					old, ok := latest[q.MessageID]
					if !ok || q.CreatedAt.After(old.CreatedAt) || (q.CreatedAt.Equal(old.CreatedAt) && q.RequestID > old.RequestID) {
						latest[q.MessageID] = q
					}
				}
			}
			for _, q := range requests {
				if q.OwnerID == owner && (q.State == "failed" || q.State == "interrupted") {
					next := ""
					if current := latest[q.MessageID]; current.RequestID != q.RequestID {
						next = current.State
					}
					items = append(items, replyNotification(q, notificationContext(room.Title, ""), next))
				}
			}
			messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
			if err != nil {
				return nil, err
			}
			decisions := map[string]string{}
			for _, m := range messages {
				if m.AuthorType == "user" && ownedMessages[m.ID] {
					if d := decodeInboxPlanDecision(m.Content); d != nil {
						decision := "approve"
						if d.Action == "defer" {
							decision = "defer"
						}
						decisions[d.MessageID] = decision
					}
				}
			}
			for _, m := range messages {
				if m.AuthorType == "agent" && allowed[m.ID] {
					_, title := splitInboxApproval(m.AuthorType, m.Content)
					if title != "" {
						items = append(items, proposalNotification(room.ID, m.ID, title, notificationContext(room.Title, ""), decisions[m.ID], m.CreatedAt))
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
				items = append(items, ownerNotification{ID: "customer:" + c.SessionUUID + ":" + m.ID, Kind: "customer", Title: "New customer message", Context: notificationContext(c.Name, c.Product), Status: "Customer message", ActionLabel: "View message", Href: "/inbox/" + url.PathEscape(c.SessionUUID), At: m.CreatedAt})
			}
		}
	}
	return items, nil
}
func mergeNotifications(p *ownerPreferences, items []ownerNotification, snapshots ...map[string]notificationTaskState) {
	index := map[string]int{}
	for i, item := range p.Items {
		index[item.ID] = i
	}
	for _, item := range items {
		if i, ok := index[item.ID]; ok {
			item.Read = p.Items[i].Read
			p.Items[i] = item
		} else if !item.Resolved {
			index[item.ID] = len(p.Items)
			p.Items = append(p.Items, item)
		}
	}
	if len(snapshots) > 0 {
		for i := range p.Items {
			n := &p.Items[i]
			if n.Kind != "approval" && n.Kind != "blocker" {
				continue
			}
			parts := strings.SplitN(n.ID, ":", 4)
			if len(parts) != 4 || parts[0] != "task" {
				continue
			}
			state, ok := snapshots[0][parts[1]]
			if !ok {
				continue
			}
			if n.ID != "task:"+parts[1]+":"+state.Status+":"+state.Attempt {
				n.Resolved = true
				n.Status = "Updated"
				if state.Status == "done" {
					n.Status = "Completed"
				} else if state.Status == "doing" {
					n.Status = "In progress"
				}
				n.Title = state.Title
				n.Context = state.Context
				n.Href = state.Href
				n.Summary = "This work has a newer update."
				n.ActionLabel = "View work"
			}
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
	items, states, err := s.notificationSnapshot(r.Context(), owner)
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
	mergeNotifications(&p, items, states)
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
	items, states, err := s.notificationSnapshot(ctx, owner)
	if err != nil {
		return
	}
	ownerPreferencesMu.Lock()
	p, err = readOwnerPreferences(owner)
	if err == nil {
		mergeNotifications(&p, items, states)
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
			if item.Read || item.Resolved || sub.Sent[item.ID] || !item.At.After(sub.Since) || time.Since(item.At) > 24*time.Hour || !enabledNotification(p.Notifications, item.Kind) {
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

func (s *Server) notificationChatCandidatesSQL(ctx context.Context, owner string, retainedIDs []string) ([]ownerNotification, error) {
	rows, err := s.db.QueryContext(ctx, `
 SELECT c.kind,c.id,c.room_id,c.created_at,c.content,c.state,c.safe_error,c.next_state,COALESCE(NULLIF(c.context,''),NULLIF(r.title,''),p.name,'')
 FROM (
 SELECT 'request' AS kind,q.request_id::text AS id,q.room_id,q.created_at,'' AS content,q.state,q.safe_error,
 COALESCE((SELECT child.state FROM inbox_requests child WHERE child.owner_id=q.owner_id AND child.room_id=q.room_id AND child.message_id=q.message_id AND (child.created_at,child.request_id)>(q.created_at,q.request_id) ORDER BY child.created_at DESC,child.request_id DESC LIMIT 1),'') AS next_state,'' AS context
 FROM inbox_requests q WHERE q.owner_id=$1 AND q.state IN ('failed','interrupted')
 UNION ALL
 SELECT 'approval',m.id::text,m.room_id,m.created_at,m.content,'','','','' FROM project_messages m JOIN inbox_requests q ON q.reply_id=m.id WHERE q.owner_id=$1 AND m.author_type='agent' AND m.content LIKE '%cuan-approval%'
 UNION ALL
 SELECT 'customer',m.id::text,m.room_id,m.created_at,'','','','',c.name FROM project_messages m JOIN crm_contacts c ON c.session_uuid=m.room_id WHERE m.author_type='user' AND COALESCE(m.author_id,'')<>'operator_user'
 ) c LEFT JOIN project_rooms r ON r.id=c.room_id LEFT JOIN projects p ON p.id=r.project_id ORDER BY COALESCE((c.kind||':'||c.room_id::text||':'||c.id)=ANY($2::text[]),false) DESC,c.created_at DESC LIMIT 200`, owner, retainedIDs)
	if err != nil {
		return nil, err
	}
	items := []ownerNotification{}
	approvalRooms := map[string]bool{}
	for rows.Next() {
		var kind, id, room, content, state, safe, context string
		var at time.Time
		var nextState string
		if err = rows.Scan(&kind, &id, &room, &at, &content, &state, &safe, &nextState, &context); err != nil {
			rows.Close()
			return nil, err
		}
		context = notificationText(context, 120)
		switch kind {
		case "request":
			items = append(items, replyNotification(db.InboxRequest{RoomID: room, RequestID: id, State: state, SafeError: safe, CreatedAt: at}, context, nextState))
		case "approval":
			_, title := splitInboxApproval("agent", content)
			if title != "" {
				items = append(items, proposalNotification(room, id, title, context, "", at))
				approvalRooms[room] = true
			}
		case "customer":
			items = append(items, ownerNotification{ID: "customer:" + room + ":" + id, Kind: "customer", Title: "New customer message", Context: context, Status: "Customer message", ActionLabel: "View message", Href: "/inbox/" + url.PathEscape(room), At: at})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(approvalRooms) > 0 {
		rooms := make([]string, 0, len(approvalRooms))
		for room := range approvalRooms {
			rooms = append(rooms, room)
		}
		decisions, err := s.db.QueryContext(ctx, `SELECT m.room_id::text,m.content FROM project_messages m JOIN inbox_requests q ON q.message_id=m.id WHERE q.owner_id=$1 AND m.author_type='user' AND m.room_id::text=ANY($2) AND m.content LIKE $3 AND m.content LIKE '%"approval":%' ORDER BY m.created_at`, owner, rooms, attachmentEnvelope+"%")
		if err != nil {
			return nil, err
		}
		defer decisions.Close()
		handled := map[string]string{}
		for decisions.Next() {
			var room, content string
			if err = decisions.Scan(&room, &content); err != nil {
				return nil, err
			}
			if decision := decodeInboxPlanDecision(content); decision != nil {
				status := "Approved"
				if decision.Action == "defer" {
					status = "Saved for later"
				}
				handled["approval:"+room+":"+decision.MessageID] = status
			}
		}
		if err = decisions.Err(); err != nil {
			return nil, err
		}
		for i := range items {
			if status, ok := handled[items[i].ID]; ok {
				items[i].Resolved = true
				items[i].Status = status
				items[i].ActionLabel = "View conversation"
			}
		}
	}
	return items, nil
}
