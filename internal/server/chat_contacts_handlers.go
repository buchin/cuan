package server

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

// ChatContact represents a contact in the direct messenger hub (agent or customer).
type ChatContact struct {
	ResolutionObservedAt time.Time  `json:"resolution_observed_at"`
	Resolved             bool       `json:"resolved"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	OnlySystemNotices    bool       `json:"only_system_notices,omitempty"`
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

// ============================================================================
// INTERNAL DIRECT MESSENGER API - CONTACTS
// ============================================================================

// GET /api/v1/chat/contacts
func (s *Server) handleInternalListContacts(w http.ResponseWriter, r *http.Request) {
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
			if internalInboxReviewInput(message) || isSystemLifecycleNotice(message) || !message.CreatedAt.After(cutoff) {
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

	// Single batch load of rooms and recent messages to eliminate N+1 query storms
	var allRooms []db.ProjectRoom
	var allMessages map[string][]db.ProjectMessage
	roomsByTopic := make(map[string]string)
	roomsByTitle := make(map[string]string)
	roomsByID := make(map[string]db.ProjectRoom)
	if s.projectStudioRepo != nil {
		allRooms, _ = s.projectStudioRepo.ListAllRooms(r.Context())
		allMessages, _ = s.projectStudioRepo.ListRecentMessages(r.Context(), 50)
		for i := range allRooms {
			roomsByID[allRooms[i].ID] = allRooms[i]
			if allRooms[i].Topic != "" && roomsByTopic[allRooms[i].Topic] == "" {
				roomsByTopic[allRooms[i].Topic] = allRooms[i].ID
			}
			if allRooms[i].Title != "" && roomsByTitle[allRooms[i].Title] == "" {
				roomsByTitle[allRooms[i].Title] = allRooms[i].ID
			}
		}
	}
	if allMessages == nil {
		allMessages = make(map[string][]db.ProjectMessage)
	}

	// Count active tasks and total tasks per conversation room
	counts, err := s.readTaskCounts(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Could not load conversation task progress. Try again.")
		return
	}
	resolutionObserved := map[string]time.Time{}
	observeResolution := func(id string, messages []db.ProjectMessage) {
		for _, message := range messages {
			if isSystemLifecycleNotice(message) || internalInboxReviewInput(message) {
				continue
			}
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

	var activeAgentsByID map[string]*db.Agent
	var cuanAgent *db.Agent
	var dbAgents []db.Agent
	if typeFilter != "customers" && s.agentRepo != nil {
		if loaded, err := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{Status: db.AgentStatusActive}); err == nil {
			dbAgents = loaded
			activeAgentsByID = make(map[string]*db.Agent, len(dbAgents))
			for i := range dbAgents {
				a := &dbAgents[i]
				activeAgentsByID[a.ID] = a
				if a.Enabled && (a.ID == db.AgentIDCuan || a.Name == db.DefaultAgentHandle) {
					cuanAgent = a
				}
			}
		}
	}

	if typeFilter != "customers" && len(dbAgents) > 0 {
		hasCuanAgent := cuanAgent != nil
		for _, a := range dbAgents {
			if !a.Enabled {
				continue
			}
			// Single Agent Facade: when @cuan is active, hide internal legacy agents from contacts list
			if hasCuanAgent && isLegacyInternalAgent(a.ID) && a.ID != db.AgentIDCuan {
				continue
			}
			roomID := ""
			lastMsg := "No conversations yet."
			lastAt := a.UpdatedAt
			unread := 0
			if id, ok := roomsByTopic["agent:"+a.ID]; ok {
				roomID = id
			} else if id, ok := roomsByTitle[a.Name]; ok {
				roomID = id
			}
			if roomID == "" {
				deterministicID := db.GenerateDeterministicUUID("agent", a.ID)
				for _, candidateID := range []string{deterministicID, "contact_" + a.ID, a.ID} {
					if len(allMessages[candidateID]) > 0 {
						roomID = candidateID
						break
					}
				}
			}
			if roomID != "" {
				if msgs := allMessages[roomID]; len(msgs) > 0 {
					observeResolution(roomID, msgs)
					msgs = visibleInboxMessages(msgs)
					if len(msgs) > 0 {
						if p := clientPreviewMessage(msgs); p != nil {
							lastMsg = p.Content
							lastAt = p.CreatedAt
						} else {
							lastMsg = msgs[len(msgs)-1].Content
							lastAt = msgs[len(msgs)-1].CreatedAt
						}
						unread = countUnread(roomID, msgs, true)
					}
				}
			}
			hasRoom := roomID != ""
			if roomID == "" {
				// Deterministic room UUID for registered agent without pre-creating empty rooms
				roomID = db.GenerateDeterministicUUID("agent", a.ID)
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
				HasMessages:    hasRoom,
				MessageCount:   0,
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

	if typeFilter != "customers" && len(allRooms) > 0 && activeAgentsByID != nil {
		for _, room := range allRooms {
			if !strings.HasPrefix(room.Topic, "agent-topic:") {
				continue
			}
			a := activeAgentsByID[roomAgentID(room.Topic)]
			if a == nil || !a.Enabled {
				a = cuanAgent
				if a == nil || !a.Enabled {
					continue
				}
			}
			last := "No messages yet."
			at := room.UpdatedAt
			unread := 0
			if messages := allMessages[room.ID]; len(messages) > 0 {
				observeResolution(room.ID, messages)
				messages = visibleInboxMessages(messages)
				if len(messages) > 0 {
					if p := clientPreviewMessage(messages); p != nil {
						last = p.Content
						at = p.CreatedAt
					} else {
						last = messages[len(messages)-1].Content
						at = messages[len(messages)-1].CreatedAt
					}
					unread = countUnread(room.ID, messages, true)
				}
			}
			tc, activeCnt, queuedCnt, doneCnt, failedCnt, reviewCnt, canceledCnt := getRoomTaskCounts(room.ID, "topic_"+room.ID, room.Topic)
			contacts = append(contacts, ChatContact{ID: "topic_" + room.ID, AgentID: a.ID, AgentName: a.Name, Type: "agent", Name: room.Title, Avatar: a.Avatar, Status: "idle", ConversationID: room.ID, UnreadCount: unread, LastMessage: attachmentPreview(last), LastMessageAt: at, UpdatedAt: at, HasMessages: true, MessageCount: 1, IsOnline: true, TaskCount: tc, ActiveTasks: activeCnt, QueuedTasks: queuedCnt, DoneTasks: doneCnt, FailedTasks: failedCnt, ReviewTasks: reviewCnt, CanceledTasks: canceledCnt})
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
		crmBySession := make(map[string]*CRMContact, len(saved))
		for i := range saved {
			crmBySession[saved[i].SessionUUID] = &saved[i]
			if saved[i].IsManual {
				continue
			}
			contactIDs[saved[i].SessionUUID] = saved[i].ID
			name := saved[i].Name
			if name == "" {
				name = "Visitor #" + saved[i].ID[:6]
			}
			sessionList = append(sessionList, &LiveChatSession{UUID: saved[i].SessionUUID, GuestUUID: saved[i].GuestUUID, Product: saved[i].Product, Name: name, Email: saved[i].Email, Status: "offline", CreatedAt: saved[i].CreatedAt, UpdatedAt: saved[i].UpdatedAt})
		}

		if len(allRooms) > 0 {
			known := map[string]bool{}
			for _, v := range sessionList {
				known[v.UUID] = true
			}
			for _, rm := range allRooms {
				if !known[rm.ID] && rm.ID != defaultScrumMasterRoomID && rm.ID != defaultAntigravityRoomID && !strings.Contains(strings.ToLower(rm.Title), "scrum") && roomAgentID(rm.Topic) == "" && !strings.Contains(rm.Topic, "Interactive Agent Chat") {
					sessionList = append(sessionList, &LiveChatSession{UUID: rm.ID, GuestUUID: rm.ID, Product: rm.Topic, Name: rm.Title, Status: "offline", CreatedAt: rm.CreatedAt, UpdatedAt: rm.UpdatedAt})
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

			if msgs := allMessages[sess.UUID]; len(msgs) > 0 {
				observeResolution(sess.UUID, msgs)
				msgs = visibleInboxMessages(msgs)
				if len(msgs) > 0 {
					msgCount = len(msgs)
					hasMessages = true
					if p := clientPreviewMessage(msgs); p != nil {
						lastMsg = p.Content
						lastAt = p.CreatedAt
					} else {
						lastMsg = msgs[len(msgs)-1].Content
						lastAt = msgs[len(msgs)-1].CreatedAt
					}
					unread = countUnread(sess.UUID, msgs, false)
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
				if crmC := crmBySession[sess.UUID]; crmC != nil && crmC.Email != "" {
					contactEmail = crmC.Email
				}
			}

			custContact := ChatContact{
				ContactID:      contactIDs[sess.UUID],
				ID:             "cust_" + sess.UUID,
				Type:           "customer",
				Name:           sess.Name,
				Email:          contactEmail,
				Avatar:         "",
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

	if cuanAgent == nil && s.agentRepo != nil {
		cuanAgent, _ = s.agentRepo.GetAgentByID(r.Context(), db.AgentIDCuan)
	}
	if cuanAgent != nil && cuanAgent.Enabled {
		for i := range contacts {
			if contacts[i].Type == "agent" {
				if strings.HasPrefix(contacts[i].ID, "contact_") {
					contacts[i].Name = cuanAgent.Name
				}
				contacts[i].AgentID, contacts[i].AgentName, contacts[i].Avatar = cuanAgent.ID, cuanAgent.Name, cuanAgent.Avatar
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
	for i := range contacts {
		room := roomsByID[contacts[i].ConversationID]
		contacts[i].ResolvedAt = room.ResolvedAt
		observed := resolutionObserved[contacts[i].ConversationID]
		if observed.IsZero() {
			observed = contacts[i].LastMessageAt
		}
		contacts[i].ResolutionObservedAt = observed
		onlySystemNotices := hasOnlySystemLifecycleNotices(allMessages[contacts[i].ConversationID])
		contacts[i].OnlySystemNotices = onlySystemNotices
		hasNoUnfinishedTasks := contacts[i].ActiveTasks == 0 && contacts[i].QueuedTasks == 0 && contacts[i].ReviewTasks == 0 && contacts[i].FailedTasks == 0 && contacts[i].TaskCount == contacts[i].DoneTasks+contacts[i].CanceledTasks
		if onlySystemNotices && hasNoUnfinishedTasks {
			contacts[i].Resolved = true
			contacts[i].UnreadCount = 0
		} else {
			contacts[i].Resolved = room.IsResolved(observed) && hasNoUnfinishedTasks
		}
		if contacts[i].Resolved {
			contacts[i].UnreadCount = 0
		} else if preferences["conversation:"+contacts[i].ConversationID].Unread && contacts[i].UnreadCount == 0 {
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
