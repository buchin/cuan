package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

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

var inboxRoomResolutionMu sync.Mutex

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

	// 0. Ensure neutral default project placeholder exists for foreign key constraints on project_rooms
	if s.db != nil {
		_, _ = s.db.ExecContext(ctx, `
			INSERT INTO projects (id, name, repo_url, default_branch, clone_status, project_type, is_archived, created_at, updated_at)
			VALUES ('default', 'Default', '', '', 'pending', 'custom', TRUE, NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET is_archived = TRUE, default_branch = '' WHERE projects.id = 'default' AND (projects.repo_url = '' OR projects.repo_url IS NULL);
		`)
	}
	if s.projectRepo != nil {
		if proj, err := s.projectRepo.GetProjectByID(ctx, "default"); err != nil || proj == nil {
			_ = s.projectRepo.CreateProject(ctx, &db.Project{
				ID:            "default",
				Name:          "Default",
				RepoURL:       "",
				DefaultBranch: "",
				CloneStatus:   "pending",
				ProjectType:   "custom",
			})
		}
	}

	if s.db != nil && s.agentRepo != nil {
		_ = db.EnsureBuiltinAgents(ctx, s.agentRepo)
	}
}

func isLegacyInternalAgent(id string) bool {
	switch id {
	case "", "agent_main", "agent_sm", "agent_dev", "agent_support", "agent_lifecycle", "agent_queued", db.AgentIDCuan:
		return true
	default:
		return false
	}
}

// ============================================================================
// INTERNAL DIRECT MESSENGER API
// ============================================================================

// Note: handleInternalListContacts is moved to chat_contacts_handlers.go

// Note: resolveConversationContext is moved to chat_conversation_context.go

// GET /api/v1/chat/conversations/{id}/messages
func (s *Server) handleInternalGetConversationMessages(w http.ResponseWriter, r *http.Request) {
	owner := s.authenticateOwner(w, r)
	if owner == nil {
		return
	}
	r = r.WithContext(auth.ContextWithOwner(r.Context(), owner))
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
			Avatar:         "",
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
	limit := 50
	if l := strings.TrimSpace(r.URL.Query().Get("limit")); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
			if limit > 500 {
				limit = 500
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

	messages, hasMore, err := s.getFormattedMessagesResultPaged(r.Context(), resolved.RoomID, product, limit, beforeID)
	if err != nil {
		respondJSONError(w, 503, "Could not read Inbox history. Try again.")
		return
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

	visibleIDs := make(map[string]bool, len(messages))
	for _, m := range messages {
		if m.UUID != "" {
			visibleIDs[m.UUID] = true
		}
	}
	prunedRequests := make([]db.InboxRequest, len(requests))
	copy(prunedRequests, requests)
	for i := range prunedRequests {
		q := &prunedRequests[i]
		if q.State == "completed" || q.State == "cancelled" {
			if !visibleIDs[q.MessageID] && !visibleIDs[q.ReplyID] {
				q.Steps = nil
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks":           linked,
		"task_count":      c.Total,
		"active_tasks":    activeTasks,
		"review_tasks":    reviewTasks,
		"queued_tasks":    queuedTasks,
		"done_tasks":      doneTasks,
		"failed_tasks":    failedTasks,
		"canceled_tasks":  c.Canceled,
		"requests":        inboxRecoveryRequests(prunedRequests),
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

// Note: customerAttachmentBaseURL, isCustomerRoom, splitCustomerReplyDraft,
// getFormattedMessages, and triage helpers are moved to chat_formatting.go.

