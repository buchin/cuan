package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/buchin/cuan/internal/db"
)

const inboxApprovalInstruction = "When a concrete plan is ready and you need the user's approval before starting work, present its outcome, scope, limits and acceptance criteria in plain language, then end the reply with exactly one cuan-approval fenced JSON block, for example:\n```cuan-approval\n{\"title\":\"Navigation audit\",\"description\":\"Read-only navigation audit. Report findings; do not modify or deploy.\",\"project_id\":\"registered project ID\"}\n```\nFor an executable plan include description (the exact scope, limits and acceptance criteria, up to 6000 characters), and either project_id or project_name in the same JSON. Use only title, description, and either project_id or project_name. Do not include authorization_message_id, action, task_id or revision: user approval is recorded separately after this proposal. The application displays that description and a button to approve that exact plan. Never hide extra scope in the description. The title is at most 200 characters. This marker creates a proposal and approval notification; it does not authorize or run anything until the user approves. Do not use it for greetings, ordinary questions, clarification, already approved work or a completed result. Do not combine it with cuan-delegate or cuan-task blocks."

func splitInboxApproval(author, content string) (string, string) {
	if author != "agent" {
		return content, ""
	}
	const marker = "```cuan-approval\n"
	text := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	index := strings.LastIndex(text, marker)
	if index < 0 || strings.Count(text, marker) != 1 || strings.Contains(text, "```cuan-delegate") || strings.Contains(text, "```cuan-task") {
		return content, ""
	}
	tail := text[index+len(marker):]
	endFence := strings.Index(tail, "```")
	if endFence < 0 {
		return content, ""
	}
	raw := []byte(strings.TrimSpace(tail[:endFence]))
	if len(raw) > 32000 {
		return content, ""
	}
	// Some native providers echo delegation bookkeeping into a proposal. Accept
	// this one known non-executable field, but discard it: it can never authorize
	// work, select an owner, or enter the immutable approval snapshot.
	var value struct {
		inboxApprovalPlan
		AuthorizationMessageID string `json:"authorization_message_id,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || json.Valid(raw) == false {
		return content, ""
	}
	if len(value.AuthorizationMessageID) > 200 {
		return content, ""
	}
	value.Title = strings.TrimSpace(value.Title)
	if value.Title == "" || len([]rune(value.Title)) > 200 {
		return content, ""
	}
	prose := strings.TrimSpace(text[:index] + "\n" + strings.TrimSpace(tail[endFence+3:]))
	return prose, value.Title
}

// The exact structured description is rendered beside the approval action.
// Client messages can select a persisted proposal, never supply its contents.
type inboxApprovalPlan struct {
	Title        string                 `json:"title"`
	Description  string                 `json:"description,omitempty"`
	ProjectID    string                 `json:"project_id,omitempty"`
	ProjectName  string                 `json:"project_name,omitempty"`
	Continuation *inboxContinuationPlan `json:"continuation,omitempty"`
}
type approvedInboxPlan struct {
	Action    string            `json:"action,omitempty"`
	MessageID string            `json:"message_id"`
	Plan      inboxApprovalPlan `json:"plan"`
}

func parseInboxApprovalPlan(content string) *inboxApprovalPlan {
	_, title := splitInboxApproval("agent", content)
	if title == "" {
		return nil
	}
	text := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.LastIndex(text, "```cuan-approval\n")
	if start < 0 {
		return nil
	}
	tail := text[start+len("```cuan-approval\n"):]
	endFence := strings.Index(tail, "```")
	if endFence < 0 {
		return nil
	}
	raw := []byte(strings.TrimSpace(tail[:endFence]))
	var p inboxApprovalPlan
	if json.Unmarshal(raw, &p) != nil {
		return nil
	}
	if strings.TrimSpace(p.Description) == "" || len([]rune(p.Description)) > 6000 || (p.ProjectID == "") == (strings.TrimSpace(p.ProjectName) == "") || len([]rune(p.ProjectName)) > 100 {
		return nil
	}
	return &p
}
func (s *Server) persistedInboxPlan(ctx context.Context, room, owner, agent, id string) (*approvedInboxPlan, error) {
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return nil, errors.New("Approval storage is unavailable")
	}
	requests, err := store.ListInboxRequests(ctx, room)
	if err != nil {
		return nil, err
	}
	trusted := false
	for _, q := range requests {
		if q.ReplyID == id && q.OwnerID == owner && q.AgentID == agent && q.State == "completed" {
			trusted = true
		}
	}
	if !trusted {
		return nil, errors.New("Choose a completed plan from this conversation")
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
	if err != nil {
		return nil, err
	}
	for _, m := range messages {
		if m.ID == id && m.AuthorType == "agent" && m.AuthorID == agent {
			if p := parseInboxApprovalPlan(m.Content); p != nil {
				// Replaying an accepted decision reads its existing receipt. Current
				// task state guards fresh approvals, not delivery retries after work
				// already advanced. Match the saved owner envelope and exact plan;
				// AcceptInboxRequest still enforces its canonical payload hash.
				accepted := false
				for _, q := range requests {
					if q.OwnerID != owner || q.AgentID != agent || q.RequestID != delegationMessageID("approve-plan:"+room+":"+id) {
						continue
					}
					message, readErr := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
					if readErr != nil {
						return nil, readErr
					}
					if message == nil || message.RoomID != room || message.AuthorType != "user" {
						continue
					}
					if previous := decodeApprovedInboxPlan(message.Content); previous != nil && previous.MessageID == id {
						a, _ := json.Marshal(previous.Plan)
						b, _ := json.Marshal(p)
						accepted = bytes.Equal(a, b)
					}
				}
				if !accepted {
					if err := s.validateInboxContinuation(ctx, room, p); err != nil {
						return nil, err
					}
				}
				return &approvedInboxPlan{MessageID: id, Plan: *p}, nil
			}
		}
	}
	return nil, errors.New("This message has no executable plan; ask the agent to clarify it")
}
func decodeInboxPlanDecision(content string) *approvedInboxPlan {
	if !strings.HasPrefix(content, attachmentEnvelope) {
		return nil
	}
	var m attachmentMessage
	if json.Unmarshal([]byte(strings.TrimPrefix(content, attachmentEnvelope)), &m) != nil {
		return nil
	}
	return m.Approval
}
func decodeApprovedInboxPlan(content string) *approvedInboxPlan {
	p := decodeInboxPlanDecision(content)
	if p != nil && p.Action != "" && p.Action != "approve" {
		return nil
	}
	return p
}

func approvedPlanDelegation(plan *approvedInboxPlan, authorization string) string {
	d := inboxDelegation{Title: plan.Plan.Title, Description: plan.Plan.Description, ProjectID: plan.Plan.ProjectID, ProjectName: plan.Plan.ProjectName, AuthorizationID: authorization}
	if c := plan.Plan.Continuation; c != nil {
		d = inboxDelegation{TaskID: c.TaskID, ProjectID: plan.Plan.ProjectID, Revision: c.Revision, AuthorizationID: authorization}
	}
	raw, _ := json.Marshal(d)
	return fmt.Sprintf("```cuan-delegate\n%s\n```", raw)
}

func (s *Server) inboxApprovedPlan(ctx context.Context, q db.InboxRequest) (*approvedInboxPlan, error) {
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	if err != nil {
		return nil, err
	}
	for _, m := range messages {
		if m.ID == q.MessageID && m.AuthorType == "user" {
			plan := decodeInboxPlanDecision(m.Content)
			if plan != nil && plan.Action != "defer" {
				if err := s.validateInboxContinuation(ctx, q.RoomID, &plan.Plan); err != nil {
					return nil, err
				}
			}
			return plan, nil
		}
	}
	return nil, nil
}

// Short, unambiguous replies can approve the immediately preceding plan.
// Questions, attachments and replies after an intervening message remain chat.
func (s *Server) conversationalApprovalID(ctx context.Context, room, text string) string {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(text), ".!")) {
	case "approve", "approved", "approve this plan", "yes", "yes please", "go ahead", "go", "proceed", "gas", "gass", "gaskeun", "gas pol", "setuju", "lanjut", "lanjutkan", "oke", "ok", "oke gas", "siap", "jalankan", "jalan":
	default:
		return ""
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
	if err != nil {
		return ""
	}
	var latest *db.ProjectMessage
	for i := range messages {
		m := &messages[i]
		if latest == nil || m.CreatedAt.After(latest.CreatedAt) || (m.CreatedAt.Equal(latest.CreatedAt) && m.ID > latest.ID) {
			latest = m
		}
	}
	if latest != nil && latest.AuthorType == "agent" && parseInboxApprovalPlan(latest.Content) != nil {
		return latest.ID
	}
	return ""
}
