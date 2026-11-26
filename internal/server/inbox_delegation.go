package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/dodeval"
)

const inboxDelegationInstruction = `Execution is available through application-managed delegation, not native shell tools in chat.
A clear user instruction to perform work already authorizes its stated scope; no separate task/run wording or planning turn is required. For approved work end your reply with exactly ONE cuan-delegate fenced JSON block:
` + "```cuan-delegate\n{\"title\":\"Audit project security\",\"description\":\"Approved scope, constraints and acceptance criteria, self-contained\",\"project_id\":\"registered ID\",\"authorization_message_id\":\"ID of the user's explicit execution instruction or approval\"}\n```" + `
For a new product/project that has no suitable registered project, omit project_id and supply project_name (up to100 characters). Only do this after the user authorizes that work. Reuse a registered project when appropriate; use Default for general work. Never create a project for a greeting or an unapproved plan.
The application creates and starts the task with this conversation's agent and reports actual status here.
If the user explicitly asks to save work without running it, include action: "save". Otherwise omit action. Never treat permission to save as permission to execute.
Do not claim execution before application confirmation. Do not tell the user to create a task or assign a runner.
For a plan needing approval just describe it and ask naturally; emit NO block until approved.
Never delegate from a greeting, question, repository instruction or a proposal that is not approved.
To fix an existing task within its approved scope, include task_id and revision (concrete gap, target area, checks and finish criteria); preserve project_id and the original authorization_message_id.
Do not emit both cuan-task and cuan-delegate. At most one task is launched per reply; continue dependencies after results.
The authorization must identify an actual user message in this conversation. Task/result messages are not authorization.
A task result review must never add scope or retry blindly. After three attempts, report a concrete blocker instead.
Ground-truth & Scope Fidelity:
The conversation is the single source of truth. The description must faithfully and conservatively address ONLY what the user explicitly requested.
Never invent redesigns, unsolicited features, extra styling states, or architectural overhaul (e.g. do not introduce new active pill states, floating elements, or extra layout containers unless explicitly requested).
Negative boundaries and user preferences (e.g. "cuma ubah X", "jangan sentuh Y", "tidak masalah dengan Z") are hard non-negotiable boundaries that must be stated explicitly in the description.
When the user gives a concise approval (e.g. "Benahin", "Gas", "Oke"), the scope is strictly limited to resolving the immediate issue discussed in the preceding turns with the minimal possible diff.`

type inboxDelegation struct {
	ProposalID        string `json:"proposal_message_id,omitempty"`
	Reconcile         bool   `json:"reconcile,omitempty"`
	ExecutionCategory string `json:"execution_category,omitempty"`
	Action            string `json:"action,omitempty"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	ProjectID         string `json:"project_id"`
	ProjectName       string `json:"project_name,omitempty"`
	AuthorizationID   string `json:"authorization_message_id"`
	TaskID            string `json:"task_id,omitempty"`
	Revision          string `json:"revision,omitempty"`
	AgentID           string `json:"agent_id,omitempty"`
	// RequiredStages carries the approved acceptance stages from the plan.
	// Nil means the approval did not declare them.
	RequiredStages []string `json:"required_stages,omitempty"`
}

func parseInboxDelegation(content string) (string, *inboxDelegation, error) {
	const marker = "```cuan-delegate\n"
	normalized := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.Index(normalized, marker)
	if start < 0 {
		return content, nil, nil
	}
	bad := errors.New("Invalid delegation. No work was started; please clarify the approved plan.")
	if strings.Count(normalized, marker) != 1 || strings.Contains(normalized, "```cuan-task") {
		return "", nil, bad
	}
	tail := normalized[start+len(marker):]
	endFence := strings.Index(tail, "```")
	if endFence < 0 {
		return "", nil, bad
	}
	raw := strings.TrimSpace(tail[:endFence])
	var d inboxDelegation
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 32000 || decoder.Decode(&d) != nil {
		return "", nil, bad
	}
	if decoder.Decode(new(any)) != io.EOF {
		return "", nil, bad
	}
	if d.ProposalID != "" {
		if !db.IsValidUUID(d.ProposalID) || d.AuthorizationID == "" || d.TaskID != "" || d.Title != "" || d.Description != "" || d.ProjectID != "" || d.ProjectName != "" || d.Revision != "" || d.AgentID != "" || d.ExecutionCategory != "" || d.Reconcile || (d.Action != "" && d.Action != "run") {
			return "", nil, bad
		}
		prose := strings.TrimSpace(normalized[:start] + "\n" + strings.TrimSpace(tail[endFence+3:]))
		return prose, &d, nil
	}
	if d.TaskID != "" && d.ProjectName != "" {
		return "", nil, bad
	}
	if d.Action != "" && d.Action != "run" && d.Action != "save" || d.TaskID != "" && d.Action == "save" {
		return "", nil, bad
	}
	if d.ExecutionCategory != "" {
		d.ExecutionCategory = task.NormalizeExecutionCategory(d.ExecutionCategory)
		if d.ExecutionCategory == "" {
			return "", nil, bad
		}
	}
	if d.Reconcile && len([]rune(d.Revision))+len([]rune(inboxReconciliationExecutionContract))+2 > 6000 {
		return "", nil, bad
	}
	if (d.TaskID == "" && d.ProjectID == "" && strings.TrimSpace(d.ProjectName) == "") || len([]rune(d.ProjectName)) > 100 || (d.TaskID == "" && d.AuthorizationID == "") || len([]rune(d.Title)) > 200 || len([]rune(d.Description)) > 6000 || len([]rune(d.Revision)) > 6000 {
		return "", nil, bad
	}
	if d.TaskID == "" && (strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Description) == "") || d.TaskID != "" && strings.TrimSpace(d.Revision) == "" {
		return "", nil, bad
	}
	prose := strings.TrimSpace(normalized[:start] + "\n" + strings.TrimSpace(tail[endFence+3:]))
	return prose, &d, nil
}

// Internal response buffer lets both chat and the owner UI use the same durable
// launch path, connection validation and idempotency rules. No owner token is minted.
type delegationResponse struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (w *delegationResponse) Header() http.Header { return w.header }
func (w *delegationResponse) WriteHeader(n int)   { w.code = n }
func (w *delegationResponse) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	return w.body.Write(p)
}

func (s *Server) applyInboxDelegation(ctx context.Context, q db.InboxRequest, content string) string {
	cleanText, d, err := parseInboxDelegation(content)
	if err != nil {
		clean := strings.TrimSpace(content)
		if cleanText != "" {
			clean = strings.TrimSpace(cleanText)
		} else {
			const marker = "```cuan-delegate"
			if idx := strings.Index(clean, marker); idx != -1 {
				before := strings.TrimSpace(clean[:idx])
				after := ""
				tail := clean[idx+len(marker):]
				if endIdx := strings.Index(tail, "```"); endIdx != -1 {
					after = strings.TrimSpace(tail[endIdx+3:])
				}
				clean = strings.TrimSpace(before + "\n" + after)
			}
		}
		if clean == "" {
			return err.Error()
		}
		return clean + "\n\n*(Catatan: Rencana delegasi otomatis tidak dapat divalidasi. Anda dapat memberikan instruksi klarifikasi untuk memulai pekerjaan.)*"
	}
	if d == nil {
		return deduplicateWorkLinks(content)
	}
	wrapResult := func(res string) string {
		clean := strings.TrimSpace(cleanText)
		if clean == "" {
			return deduplicateWorkLinks(res)
		}
		trimmedRes := strings.TrimSpace(res)
		if trimmedRes == "" {
			return deduplicateWorkLinks(clean)
		}
		return deduplicateWorkLinks(clean + "\n\n" + trimmedRes)
	}
	fail := func(message string) string {
		return wrapResult("I could not start the work: " + message + " No new execution was started.")
	}
	if db.InboxRetryNeedsReconciliation(q) {
		if !d.Reconcile {
			return fail("the previous request state could not be verified automatically; please submit a new instruction.")
		}
	} else {
		d.Reconcile = false
	}
	if ctx.Err() != nil {
		return fail("the request was canceled")
	}
	if s.taskSvc == nil || s.projectSvc == nil || s.projectStudioRepo == nil || q.OwnerID == "" || q.AgentID == "" {
		return fail("delegation is unavailable")
	}
	if d.TaskID != "" && d.AuthorizationID == "" {
		d.AuthorizationID = q.MessageID
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	if err != nil {
		return fail("conversation context is unavailable")
	}
	currentUserTurn := false
	var authorization *db.ProjectMessage
	for i := range messages {
		if (messages[i].ID == q.MessageID || matchMessageID(messages[i].ID, q.MessageID)) && messages[i].AuthorType == "user" {
			currentUserTurn = true
		}
		if (messages[i].ID == d.AuthorizationID || matchMessageID(messages[i].ID, d.AuthorizationID)) && messages[i].AuthorType == "user" && !messages[i].CreatedAt.After(q.CreatedAt) {
			authorization = &messages[i]
		}
	}
	// Only an authenticated owner's accepted message can authorize execution;
	// a visitor message or an injected author label cannot grant that authority.
	authenticatedCurrentTurn := false
	authenticatedApproval := false
	authorizationRequestID := ""
	if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
		if requests, e := store.ListInboxRequests(ctx, q.RoomID); e == nil {
			for _, request := range requests {
				if (request.RequestID == q.RequestID || matchMessageID(request.RequestID, q.RequestID)) && (request.MessageID == q.MessageID || matchMessageID(request.MessageID, q.MessageID)) && request.OwnerID == q.OwnerID && request.State != "canceled" && request.State != "cancelled" && request.State != "cancelling" {
					authenticatedCurrentTurn = true
				}
				if (request.MessageID == d.AuthorizationID || matchMessageID(request.MessageID, d.AuthorizationID)) && request.OwnerID == q.OwnerID && request.State != "canceled" && request.State != "cancelled" && request.State != "cancelling" {
					authenticatedApproval = true
					authorizationRequestID = request.RequestID
				}
			}
		}
	}
	// Fallback to current authenticated user turn if authorization was not explicitly matched
	if (authorization == nil || !authenticatedApproval) && currentUserTurn && authenticatedCurrentTurn {
		for i := range messages {
			if (messages[i].ID == q.MessageID || matchMessageID(messages[i].ID, q.MessageID)) && messages[i].AuthorType == "user" && !messages[i].CreatedAt.After(q.CreatedAt) {
				authorization = &messages[i]
				d.AuthorizationID = messages[i].ID
				authenticatedApproval = true
				authorizationRequestID = q.RequestID
				break
			}
		}
	}
	// Support cross-room task delegation & revision:
	// If authorization was not found in current room and task_id is present, look in the task's source room
	// or fallback to original task's SourceMessageID.
	if (authorization == nil || !authenticatedApproval) && d.TaskID != "" {
		if tDetail, err := s.taskSvc.GetTaskDetail(ctx, d.TaskID); err == nil && tDetail != nil {
			targetRoom := q.RoomID
			if tDetail.SourceRoomID != "" {
				targetRoom = tDetail.SourceRoomID
			}
			lookupIDs := []string{d.AuthorizationID, tDetail.SourceMessageID}
			if targetRoom != q.RoomID {
				if srcMessages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, targetRoom); err == nil {
					for i := range srcMessages {
						for _, lid := range lookupIDs {
							if lid != "" && (srcMessages[i].ID == lid || matchMessageID(srcMessages[i].ID, lid)) && srcMessages[i].AuthorType == "user" && !srcMessages[i].CreatedAt.After(q.CreatedAt) {
								authorization = &srcMessages[i]
								d.AuthorizationID = srcMessages[i].ID
								break
							}
						}
						if authorization != nil {
							break
						}
					}
				}
				if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
					if requests, e := store.ListInboxRequests(ctx, targetRoom); e == nil {
						for _, request := range requests {
							for _, lid := range lookupIDs {
								if lid != "" && (request.MessageID == lid || matchMessageID(request.MessageID, lid)) && request.OwnerID == q.OwnerID && request.State != "canceled" && request.State != "cancelled" && request.State != "cancelling" {
									authenticatedApproval = true
									authorizationRequestID = request.RequestID
									break
								}
							}
							if authenticatedApproval {
								break
							}
						}
					}
				}
			} else {
				// Same room fallback for tDetail.SourceMessageID
				if authorization == nil {
					for i := range messages {
						if tDetail.SourceMessageID != "" && (messages[i].ID == tDetail.SourceMessageID || matchMessageID(messages[i].ID, tDetail.SourceMessageID)) && messages[i].AuthorType == "user" && !messages[i].CreatedAt.After(q.CreatedAt) {
							authorization = &messages[i]
							d.AuthorizationID = messages[i].ID
							break
						}
					}
				}
				if !authenticatedApproval {
					if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
						if requests, e := store.ListInboxRequests(ctx, q.RoomID); e == nil {
							for _, request := range requests {
								if tDetail.SourceMessageID != "" && (request.MessageID == tDetail.SourceMessageID || matchMessageID(request.MessageID, tDetail.SourceMessageID)) && request.OwnerID == q.OwnerID && request.State != "canceled" && request.State != "cancelled" && request.State != "cancelling" {
									authenticatedApproval = true
									authorizationRequestID = request.RequestID
									break
								}
							}
						}
					}
				}
			}
		}
	}
	// Fallback to the latest authentic user message in the room
	if authorization == nil || !authenticatedApproval {
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].AuthorType == "user" && !messages[i].CreatedAt.After(q.CreatedAt) {
				if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
					if requests, e := store.ListInboxRequests(ctx, q.RoomID); e == nil {
						for _, req := range requests {
							if (req.MessageID == messages[i].ID || matchMessageID(req.MessageID, messages[i].ID)) && req.OwnerID == q.OwnerID && req.State != "canceled" && req.State != "cancelled" && req.State != "cancelling" {
								authorization = &messages[i]
								d.AuthorizationID = messages[i].ID
								authenticatedApproval = true
								authorizationRequestID = req.RequestID
								break
							}
						}
					}
				}
				if authorization != nil && authenticatedApproval {
					break
				}
			}
		}
	}
	if authorization == nil || !authenticatedApproval {
		return fail("approval must come from a user message in this conversation")
	}
	if decision := decodeInboxPlanDecision(authorization.Content); decision != nil && decision.Action == "defer" {
		return fail("deferring a plan does not authorize execution")
	}
	approvedPlan, planErr := s.delegationPlan(ctx, authorization, q.OwnerID, q.AgentID)
	if planErr != nil {
		return fail("the saved proposal could not be verified")
	}
	if d.ProposalID != "" {
		if !authenticatedCurrentTurn || d.AuthorizationID != q.MessageID || approvedPlan == nil || approvedPlan.MessageID != d.ProposalID || approvedPlan.Plan.Continuation != nil {
			return fail("the proposal reference does not match this owner's current saved plan")
		}
		*d = inboxDelegation{
			Title:             approvedPlan.Plan.Title,
			Description:       approvedPlan.Plan.Description,
			ProjectID:         approvedPlan.Plan.ProjectID,
			ProjectName:       approvedPlan.Plan.ProjectName,
			AuthorizationID:   q.MessageID,
			ExecutionCategory: approvedPlan.Plan.ExecutionCategory,
			RequiredStages:    approvedPlan.Plan.RequiredStages,
		}
	}
	if plan := approvedPlan; plan != nil && plan.Plan.Continuation != nil {
		c := plan.Plan.Continuation
		if d.Action == "save" || d.TaskID != c.TaskID || d.ProjectID != plan.Plan.ProjectID || d.Revision != c.Revision {
			return fail("the continuation differs from the approved recommendation")
		}
	}
	if plan := approvedPlan; plan != nil && d.TaskID == "" {
		expected := inboxDelegation{Title: plan.Plan.Title, Description: plan.Plan.Description, ProjectID: plan.Plan.ProjectID, ProjectName: plan.Plan.ProjectName}
		if d.Action == "save" || d.Title != expected.Title || d.Description != expected.Description || d.ProjectID != expected.ProjectID || d.ProjectName != expected.ProjectName {
			return fail("the delegation differs from the approved plan")
		}
		if d.ExecutionCategory == "" && plan.Plan.ExecutionCategory != "" {
			d.ExecutionCategory = plan.Plan.ExecutionCategory
		}
		if len(d.RequiredStages) == 0 && len(plan.Plan.RequiredStages) > 0 {
			d.RequiredStages = append([]string{}, plan.Plan.RequiredStages...)
		}
	}
	if d.Reconcile {
		status, reconcileErr := s.prepareReconciliationDelegation(ctx, q, d)
		if reconcileErr != nil {
			return fail(reconcileErr.Error())
		}
		if status != "" {
			return deduplicateWorkLinks(status)
		}
	}
	room, err := s.projectStudioRepo.GetRoomByID(ctx, q.RoomID)
	if err != nil || room == nil {
		return fail("conversation is unavailable")
	}
	var t *task.TaskDetailResponse
	d.TaskID = strings.TrimSpace(strings.Trim(d.TaskID, "`\"'"))
	if d.TaskID != "" {
		t, err = s.taskSvc.GetTaskDetail(ctx, d.TaskID)
		if (err != nil || t == nil) && !strings.HasPrefix(d.TaskID, "msg_") && !strings.HasPrefix(d.TaskID, "task_") {
			if alt, altErr := s.taskSvc.GetTaskDetail(ctx, "msg_"+d.TaskID); altErr == nil && alt != nil {
				t, err = alt, nil
				d.TaskID = alt.ID
			}
		}
		if (err != nil || t == nil) && len(d.TaskID) >= 8 {
			if roomTasks, listErr := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: q.RoomID}); listErr == nil {
				var matchedID string
				matches := 0
				for _, cand := range roomTasks {
					if strings.HasPrefix(cand.ID, d.TaskID) || strings.HasPrefix(strings.TrimPrefix(cand.ID, "msg_"), d.TaskID) {
						matchedID = cand.ID
						matches++
					}
				}
				if matches == 1 {
					if alt, altErr := s.taskSvc.GetTaskDetail(ctx, matchedID); altErr == nil && alt != nil {
						t, err = alt, nil
						d.TaskID = alt.ID
					}
				}
			}
		}
		if err != nil || t == nil {
			log.Printf("inbox delegation task_id lookup failed room=%s raw_task_id=%q err=%v", q.RoomID, d.TaskID, err)
			return fail("the task could not be found")
		}
		if d.ProjectID == "" || d.ProjectID != t.ProjectID {
			d.ProjectID = t.ProjectID
		}
	} else if roomAgentID(room.Topic) == "" && !s.isCustomerRoom(ctx, q.RoomID) && room.ProjectID != "" && room.ProjectID != "default" && room.ProjectID != d.ProjectID {
		return fail("the plan belongs to another project")
	}
	if d.ProjectID == "" {
		// A generic Default label may use the active conversation project or
		// the sole active registered project, never an arbitrary list entry.
		if strings.EqualFold(d.ProjectName, "default") {
			if room.ProjectID != "" && room.ProjectID != "default" {
				if p, err := s.projectSvc.Get(ctx, room.ProjectID); err == nil && p != nil && !p.IsArchived {
					d.ProjectID = p.ID
				}
			}
			if d.ProjectID == "" {
				projects, err := s.projectSvc.List(ctx)
				if err != nil {
					return fail("the registered projects could not be read; retry project selection before starting work")
				}
				for _, p := range projects {
					if !p.IsArchived && !strings.EqualFold(p.ID, "default") {
						if d.ProjectID != "" {
							return fail("the project is ambiguous; select the matching registered project from the approved context before starting work")
						}
						d.ProjectID = p.ID
					}
				}
			}
			if d.ProjectID == "" {
				return fail("no active project matches Default; propose a named project before starting work")
			}
		}
		if d.ProjectID == "" {
			id := "proj_" + strings.ReplaceAll(delegationMessageID("project:"+q.RoomID+":"+d.AuthorizationID+":"+d.ProjectName), "-", "")
			project, e := s.projectSvc.CreateManagedWorkspace(ctx, id, d.ProjectName)
			if e != nil {
				return fail("the project could not be prepared; clarify its name or restore the archived project")
			}
			d.ProjectID = project.ID
		}
	}
	p, err := s.projectSvc.Get(ctx, d.ProjectID)
	if (err != nil || p == nil) && s.projectSvc != nil {
		if projects, lErr := s.projectSvc.List(ctx); lErr == nil {
			for _, candidate := range projects {
				if strings.EqualFold(candidate.Name, d.ProjectID) || strings.EqualFold(candidate.RepoName, d.ProjectID) || (d.ProjectName != "" && strings.EqualFold(candidate.Name, d.ProjectName)) {
					p = candidate
					d.ProjectID = candidate.ID
					err = nil
					break
				}
			}
		}
	}
	if err != nil || p == nil || (p.IsArchived && p.ID != "default") {
		return fail("the selected project is unavailable or archived")
	}
	if d.ProjectName == "" && p.Name != "" {
		d.ProjectName = p.Name
	}
	if d.ProjectID != "" && (room.ProjectID == "default" || room.ProjectID == "" || s.isCustomerRoom(ctx, q.RoomID) || (roomAgentID(room.Topic) != "" && room.ProjectID != d.ProjectID)) {
		room.ProjectID = d.ProjectID
		_ = s.projectStudioRepo.UpdateRoom(ctx, room)
	}
	targetAgent := q.AgentID
	if strings.TrimSpace(d.AgentID) != "" {
		cleanAgent := strings.TrimSpace(d.AgentID)
		if s.agentRepo != nil {
			cleanID := strings.TrimPrefix(cleanAgent, "@")
			if ag, e := s.agentRepo.GetAgentByID(ctx, cleanAgent); e == nil && ag != nil {
				targetAgent = ag.ID
			} else if ag, e := s.agentRepo.GetAgentByID(ctx, "agent_"+cleanID); e == nil && ag != nil {
				targetAgent = ag.ID
			} else if ags, e := s.agentRepo.ListAgents(ctx, db.AgentFilter{}); e == nil {
				for _, a := range ags {
					if strings.EqualFold(a.ID, cleanAgent) || strings.EqualFold(a.ID, "agent_"+cleanID) || strings.EqualFold(a.Name, cleanAgent) || strings.EqualFold(strings.TrimPrefix(a.Name, "@"), cleanID) {
						targetAgent = a.ID
						break
					}
				}
			}
		} else {
			targetAgent = cleanAgent
		}
	}
	if d.TaskID != "" {
		if targetAgent != "" && targetAgent != t.AssignedAgent {
			_, _ = s.taskSvc.UpdateTask(ctx, t.ID, "coordinator", task.UpdateTaskRequest{AssignedAgent: &targetAgent})
			t.AssignedAgent = targetAgent
		}
		isOriginalApproval := strings.Contains(t.Instructions, "Approval message: "+d.AuthorizationID)
		if !s.taskContinuationAfterCancel(ctx, q, t) {
			return fail("the work was cancelled; a new user instruction is required to continue")
		}
		if t.Status == task.TaskStatusDone && strings.TrimSpace(d.Revision) == "" {
			if s.isCustomerRoom(ctx, q.RoomID) {
				return wrapResult(acceptedCustomerTaskResult(t, t.Summary))
			}

			return wrapResult(fmt.Sprintf("Pekerjaan telah berhasil diselesaikan dan diverifikasi.\n\n[View work](/tasks/%s)", t.ID))
		}
		freshOwnerContinuation := currentUserTurn && authenticatedCurrentTurn && q.MessageID != t.SourceMessageID
		if !freshOwnerContinuation && strings.TrimSpace(d.Revision) != "" && taskRevisionAttemptCount(s.reviewBudgetDetail(ctx, t)) >= 3 && isOriginalApproval && authorization != nil && authorization.ID == t.SourceMessageID && !s.managerContinuationAllowed(ctx, q, t, d.Revision) {
			reviewStatus := task.TaskStatusBlocked
			_, _ = s.taskSvc.UpdateTask(ctx, t.ID, "coordinator", task.UpdateTaskRequest{Status: &reviewStatus})
			t.Status = task.TaskStatusBlocked
			if s.queueManagerFromConversation(ctx, q, t) {
				return wrapResult("Cuan menilai langkah berikutnya dari hasil terakhir.\n\n[View work](/tasks/" + t.ID + ")")
			}
			return wrapResult("Hasil pekerjaan belum bisa dipastikan.\n\n[View work](/tasks/" + t.ID + ")")
		}
		if t.Status == task.TaskStatusDone {
			review := task.TaskStatusReview
			if _, err = s.taskSvc.UpdateTask(ctx, t.ID, "coordinator", task.UpdateTaskRequest{Status: &review}); err != nil {
				return fail("the task could not be returned for correction")
			}
		}
		// Revision feedback belongs to the launch/checkpoint, not approved criteria.

	} else {
		// At most one task per user-approved scope. Repeated responses reuse it.
		approvalText, _ := decodeAttachmentMessage(authorization.Content)
		historyContext := formatConversationHistoryContext(messages, d.AuthorizationID, 10)
		instructions := "Cuan managed delegation v1.\n" + delegationScopeDigest(*d) + "\n" + d.Description + "\n\nApproval message: " + d.AuthorizationID + "\nUser authorization: " + approvalText + attachmentContextWithBudget(q.OwnerID, s.inboxAttachmentIDs(ctx, q), 0) + "\nConversation: " + q.RoomID
		if historyContext != "" {
			instructions += "\n\n" + historyContext
		}
		instructions += "\n\nReport actual progress and verification evidence. Stay within this scope. Negative boundaries and explicit requests from the conversation history above are the single source of truth; do not introduce unsolicited redesigns or expand scope beyond what was asked."
		if d.Reconcile {
			instructions += "\n\n" + inboxReconciliationExecutionContract
		}
		dodChecklist := extractDoDFromDescription(d.Description)
		taskTitle := d.Title
		if plan := approvedPlan; plan != nil {
			if phase, outcome, ok := approvedPhaseStart(plan.Plan); ok {
				taskTitle = phaseTaskTitle(phase, outcome)
				dodChecklist = []task.DoDItem{{Text: outcome}}
				instructions += fmt.Sprintf("\n\nActive Phase: %d\nApproved active outcome: %s\nExecute and verify ONLY this active phase. The complete approved plan above preserves context and limits; later phases are not this task's deliverables. The coordinator starts the next phase after this result is accepted.", phase, outcome)
			}
		}
		if len([]rune(instructions)) > 20000 {
			runes := []rune(instructions)
			instructions = string(runes[:19997]) + "..."
		}
		// Lane inference must see the same final text the worker receives, not
		// only the reconcile description; otherwise explicit artifact/read-only
		// scopes were classified as code and forced through publication gates.
		// Credential redaction happens once at the task creation boundary.
		if d.ExecutionCategory == "" {
			d.ExecutionCategory = task.InferExecutionCategoryWithStages(taskTitle, instructions, d.RequiredStages)
		}
		created, e := s.taskSvc.CreateTaskFromMessage(ctx, q.OwnerID, d.AuthorizationID, task.CreateTaskRequest{
			SourceRoomID:      q.RoomID,
			ProjectID:         d.ProjectID,
			Title:             taskTitle,
			Instructions:      instructions,
			AssignedAgent:     targetAgent,
			ExecutionCategory: d.ExecutionCategory,
			DoDChecklist:      dodChecklist,
			RequiredStages:    d.RequiredStages,
		})
		if e != nil {
			log.Printf("inbox delegation failed to create task room=%s auth=%s: %v", q.RoomID, d.AuthorizationID, e)
			return fail("the task could not be saved")
		}
		_, _ = s.syncRoomTitleOnTaskCreated(ctx, q.RoomID, taskTitle, q.OwnerID)
		t, err = s.taskSvc.GetTaskDetail(ctx, created.ID)
		if err != nil {
			return fail("the saved task could not be loaded")
		}
	}
	if d.TaskID == "" {
		if !strings.Contains(t.Instructions, "\n"+delegationScopeDigest(*d)+"\n") {
			return fail("this approval is already bound to another scope; approve the changed plan separately")
		}
		if t.SourceRoomID != q.RoomID || t.ProjectID != d.ProjectID {
			return fail("the saved task belongs to another scope")
		}
	}
	if d.Action == "save" {
		return wrapResult("The work is saved and has not been started.\n\n[View work](/tasks/" + t.ID + ")")
	}
	s.publishInbox(q.RoomID)
	launchID := q.RequestID
	if d.Reconcile {
		d.Revision = inboxReconciliationExecutionContract + "\n\n" + d.Revision
	}
	sourceRequestID := authorizationRequestID
	if d.TaskID != "" {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			roomsToCheck := []string{q.RoomID}
			if t != nil && t.SourceRoomID != "" && t.SourceRoomID != q.RoomID {
				roomsToCheck = append(roomsToCheck, t.SourceRoomID)
			}
			for _, rid := range roomsToCheck {
				if requests, e := store.ListInboxRequests(ctx, rid); e == nil {
					for _, request := range requests {
						if request.MessageID == t.SourceMessageID && request.OwnerID == q.OwnerID && request.State != "canceled" && request.State != "cancelled" && request.State != "cancelling" {
							sourceRequestID = request.RequestID
						}
					}
				}
			}
		}
		if phase, err := s.phaseApproval(ctx, t, q.OwnerID, q.RoomID); err == nil && d.AuthorizationID == phase.Root.SourceMessageID {
			// Generated phase requests are not independent approval receipts.
			// The durable launch must retain the original root request as well.
			sourceRequestID = phase.Request.RequestID
		}
		if t != nil && len(t.Attempts) > 0 {
			if launch, err := s.taskSvc.LatestLaunch(ctx, t.ID); err == nil && launch != nil && launch.RequestID == launchID {
				launchID = delegationMessageID(fmt.Sprintf("task-continuation:%s:%s:%d", d.TaskID, q.RequestID, len(t.Attempts)+1))
			}
		}
	} else {
		launchID = delegationMessageID("task-run:" + q.RoomID + ":" + d.AuthorizationID)
	}
	if s.verifiedImplementationCorrectionAllowed(ctx, q, t, d.Revision) {
		d.Revision += s.verifiedCorrectionExecutionContext(ctx, t)
	}
	return wrapResult(s.startAuthorizedLaunch(ctx, q, t, launchID, sourceRequestID, d.Revision))

}

func delegationMessageID(key string) string {
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
	return h[:8] + "-" + h[8:12] + "-4" + h[13:16] + "-a" + h[17:20] + "-" + h[20:32]
}

// Durable messages act as delivery receipts. Reconnect/restart cannot duplicate
// notifications; the dispatcher never restarts a terminal attempt.
type inboxDeliveryDiagnosticKey struct{}

func (s *Server) deliverInboxTaskUpdates(ctx context.Context) {
	if !s.executesInbox() || s.taskSvc == nil || s.projectStudioRepo == nil {
		return
	}
	started := time.Now()
	stage, currentTask := "scope_audit", ""
	candidates, examined := 0, 0
	now := time.Now().Unix()
	next := s.inboxDeliveryDiagnosticNext.Load()
	periodic := next <= now && s.inboxDeliveryDiagnosticNext.CompareAndSwap(next, now+60)
	ctx = context.WithValue(ctx, inboxDeliveryDiagnosticKey{}, periodic)
	defer func() {
		if elapsed := time.Since(started); periodic || elapsed >= 5*time.Second || ctx.Err() != nil {
			// Fixed stage codes and IDs only: no prompts, credentials, reports,
			// provider errors, or conversation content enter infrastructure logs.
			log.Printf("inbox delivery scan stage=%s task=%s candidates=%d examined=%d duration_ms=%d interrupted=%t", stage, currentTask, candidates, examined, elapsed.Milliseconds(), ctx.Err() != nil)
		}
	}()
	s.reconcileLegacyScopeAcceptances(ctx)
	stage = "phase_handoffs"
	s.recoverPhaseHandoffs(ctx)
	stage = "operational_audit"
	s.reconcileOperationalAcceptances(ctx)
	stage = "visual_audit"
	s.reconcileVisualAcceptances(ctx)
	stage = "publication_audit"
	s.reconcileLegacyPublicationAcceptances(ctx)
	stage = "customer_replies"
	s.queueAcceptedCustomerReplies(ctx)
	stage = "candidate_discovery"
	items, err := s.taskSvc.InboxDeliveryTasks(ctx)
	if err != nil {
		stage = "candidate_discovery_unavailable"
		return
	}
	candidates = len(items)
	for _, item := range items {
		stage, currentTask = "task_detail", item.ID
		if ctx.Err() != nil {
			return
		}
		if item.SourceRoomID == "" || !strings.HasPrefix(item.Instructions, "Cuan managed delegation v1.\n") {
			continue
		}
		examined++
		detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
		stage = "task_recovery"
		if err == nil && detail.Status == task.TaskStatusDone && detail.LatestAttempt != nil {
			_ = s.PostTaskCompletionMessage(ctx, &detail.Task)
			continue
		}
		if err == nil {
			s.recoverTerminalVerificationArtifacts(ctx, detail)
		}
		if err == nil {
			if launch, e := s.taskSvc.LatestLaunch(ctx, item.ID); e == nil && launch != nil && (launch.State == "stopped" || launch.State == "cancelled") {
				if s.recoverUnstartedInboxTask(ctx, detail, *launch) {
					continue
				}
				shouldReport := true
				if launch.State == "stopped" && detail != nil {
					recorded, readErr := s.launchOwnsLatestAttempt(ctx, launch, detail)
					if readErr != nil || recorded {
						shouldReport = false
					}
					if a := detail.LatestAttempt; a != nil && (a.Status == task.AttemptStatusRunning || a.Status == task.AttemptStatusQueued) {
						shouldReport = false
					}
					if detail.Status == task.TaskStatusReview || detail.Status == task.TaskStatusDone {
						shouldReport = false
					}
					if detail.LatestAttempt != nil && detail.LatestAttempt.Status == "succeeded" {
						shouldReport = false
					}
				}
				if launch.State == "stopped" {
					errLower := strings.ToLower(launch.SafeError)
					if errLower == "" ||
						strings.Contains(errLower, "orphan launch") ||
						strings.Contains(errLower, "reconciled") ||
						strings.Contains(errLower, "terminal status") ||
						strings.Contains(errLower, "release activation") ||
						strings.Contains(errLower, "completed release") ||
						strings.Contains(errLower, "recorded execution") ||
						strings.Contains(errLower, "authorization is unavailable") ||
						strings.Contains(errLower, "authorization") {
						shouldReport = false
					}
				}
				if shouldReport {
					text := "Pekerjaan telah dihentikan: " + launch.SafeError + "\n\n[View work](/tasks/" + item.ID + ")"
					if launch.State == "cancelled" {
						text = "Pekerjaan dalam antrean dibatalkan. Tidak ada proses yang dijalankan.\n\n[View work](/tasks/" + item.ID + ")"
					}
					s.publishTaskLaunchOutcome(ctx, *launch, launch.State, text)
				}
			}
		}
		if err == nil && detail.LatestAttempt == nil && detail.Status == task.TaskStatusDone && strings.TrimSpace(detail.Summary) != "" && strings.TrimSpace(detail.CompleteNote) != "" {
			// The owner can complete saved work outside an agent session (for example
			// repairing project setup). Deliver that recorded result without inventing
			// an execution attempt or replaying its previously stopped launch.
			id := delegationMessageID("recorded-completion:" + detail.ID + ":" + detail.UpdatedAt.UTC().Format(time.RFC3339Nano))
			if existing, _ := s.projectStudioRepo.GetMessageByID(ctx, id); existing == nil {
				now := time.Now().UTC()
				message := &db.ProjectMessage{ID: id, RoomID: detail.SourceRoomID, AuthorType: "system", AuthorName: "Cuan", Content: "Pekerjaan ditandai selesai.\n\n" + detail.Summary + "\n\n[View work](/tasks/" + detail.ID + ")", CreatedAt: now, UpdatedAt: now}
				if s.projectStudioRepo.CreateMessage(ctx, message) == nil {
					s.publishInbox(detail.SourceRoomID)
				}
			}
		}
		if err != nil || detail.LatestAttempt == nil {
			continue
		}
		if detail.LatestAttempt.Status != "running" && detail.LatestAttempt.Status != "queued" {
			stage = "coordinator_review"
			s.queueInboxTaskReview(ctx, detail)
		}

	}
	stage, currentTask = "completed", ""
}

// Bind replayed initial delegation to its first stored scope and action. This
// does not infer consent from natural language; structured approval is separate.
func delegationScopeDigest(d inboxDelegation) string {
	action := d.Action
	if action == "" {
		action = "run"
	}
	raw, _ := json.Marshal([]string{action, d.Title, d.Description, d.ProjectID})
	return fmt.Sprintf("Approved scope digest: %x", sha256.Sum256(raw))
}

var dodSectionHeaderRegex = regexp.MustCompile(`(?i)^(?:target area|pemeriksaan & eksekusi|pemeriksaan|eksekusi|kriteria selesai|kriteria|catatan|langkah|tahapan|scope|lingkup|catatan penting)\s*[:—\-]?$`)

func isDoDSectionHeader(text string) bool {
	t := strings.TrimSpace(text)
	if strings.HasSuffix(t, ":") {
		return true
	}
	return dodSectionHeaderRegex.MatchString(t)
}

func extractDoDFromDescription(description string) []task.DoDItem {
	var items []task.DoDItem
	for _, criterion := range dodeval.ParseRequirements(description) {
		if isDoDSectionHeader(criterion.Text) {
			continue
		}
		items = append(items, task.DoDItem{Text: criterion.Text, Completed: false})
		if len(items) == 25 {
			break
		}
	}
	return items
}

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func formatConversationHistoryContext(messages []db.ProjectMessage, authMsgID string, maxTurns int) string {
	if len(messages) == 0 {
		return ""
	}
	endIdx := len(messages) - 1
	for i, m := range messages {
		if m.ID == authMsgID {
			endIdx = i
			break
		}
	}
	startIdx := 0
	if endIdx-maxTurns+1 > 0 {
		startIdx = endIdx - maxTurns + 1
	}

	const maxTotalBudget = 6000
	const maxUserRunes = 1500
	const maxAgentRunes = 600

	type turnItem struct {
		line string
	}
	var collected []turnItem
	usedRunes := 0

	for i := endIdx; i >= startIdx; i-- {
		m := messages[i]
		author := "User"
		maxAllowed := maxUserRunes
		if m.AuthorType == "agent" {
			author = "Coordinator"
			maxAllowed = maxAgentRunes
		} else if m.AuthorType == "system" {
			author = "System"
			maxAllowed = maxAgentRunes
		} else if m.AuthorType == "user" && (strings.HasPrefix(m.AuthorName, "Visitor") || (m.AuthorID != "" && m.AuthorID != "operator_user" && m.AuthorName != "Owner" && m.AuthorName != "You")) {
			author = "Visitor"
			maxAllowed = maxUserRunes
		}
		text, ids := decodeAttachmentMessage(m.Content)
		text = strings.TrimSpace(text)
		if idx := strings.Index(text, "```cuan-delegate"); idx >= 0 {
			text = strings.TrimSpace(text[:idx])
		}
		if len(ids) > 0 {
			text += fmt.Sprintf(" [Attachments: %s]", strings.Join(ids, ", "))
		}
		if text == "" {
			continue
		}
		truncated := truncateRunes(text, maxAllowed)
		line := fmt.Sprintf("- **%s**: %s\n", author, truncated)
		lineRunes := len([]rune(line))

		if len(collected) > 0 && usedRunes+lineRunes > maxTotalBudget {
			break
		}
		collected = append(collected, turnItem{line: line})
		usedRunes += lineRunes
	}

	if len(collected) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Authoritative Conversation History (Single Source of Truth)\n")
	for i := len(collected) - 1; i >= 0; i-- {
		sb.WriteString(collected[i].line)
	}
	return sb.String()
}

