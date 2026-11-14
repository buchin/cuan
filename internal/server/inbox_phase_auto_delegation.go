package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"regexp"
)

var (
	multiPhaseRegex  = regexp.MustCompile(`(?i)(?:Phase|Fase)\s*(\d+)[:\-\s]+([^|\n\r]+)`)
	activePhaseRegex = regexp.MustCompile(`(?i)(?:Active|Current)\s*Phase:\s*(\d+)`)
)

type multiPhaseTarget struct {
	CurrentPhase int
	NextPhase    int
	TotalPhases  int
	NextTitle    string
}

func (s *Server) detectNextMultiPhase(ctx context.Context, d *task.TaskDetailResponse) (*multiPhaseTarget, bool) {
	if d == nil {
		return nil, false
	}
	content := d.Instructions
	if s.projectStudioRepo != nil && d.SourceMessageID != "" {
		if m, err := s.projectStudioRepo.GetMessageByID(ctx, d.SourceMessageID); err == nil && m != nil {
			if plan := decodeApprovedInboxPlan(m.Content); plan != nil {
				content = content + "\n" + plan.Plan.Description
			}
		}
	}
	if strings.TrimSpace(content) == "" {
		return nil, false
	}
	phases := multiPhaseOutcomes(content)
	if len(phases) < 2 {
		return nil, false
	}

	currentPhase := 0
	if activeMatch := activePhaseRegex.FindStringSubmatch(d.Instructions); len(activeMatch) >= 2 {
		if num, err := strconv.Atoi(activeMatch[1]); err == nil {
			currentPhase = num
		}
	}
	if currentPhase == 0 {
		if titleMatch := multiPhaseRegex.FindStringSubmatch(d.Title); len(titleMatch) >= 2 {
			if num, err := strconv.Atoi(titleMatch[1]); err == nil {
				currentPhase = num
			}
		}
	}
	if currentPhase == 0 {
		return nil, false
	}

	nextNum := currentPhase + 1
	nextTitle, exists := phases[nextNum]
	if !exists {
		return nil, false
	}

	return &multiPhaseTarget{
		CurrentPhase: currentPhase,
		NextPhase:    nextNum,
		TotalPhases:  len(phases),
		NextTitle:    nextTitle,
	}, true
}

func (s *Server) enqueueInboxAutoDelegation(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, explanation string) string {
	text, _ := s.attemptInboxAutoDelegation(ctx, q, d, explanation)
	return text
}

// Errors describe persistence/authorization outcomes independently of Inbox prose.
// A durable recovery worker must revalidate approval before retrying this attempt.
func (s *Server) attemptInboxAutoDelegation(ctx context.Context, q db.InboxRequest, d *task.TaskDetailResponse, explanation string) (string, error) {
	var handoff *db.ProjectMessage
	launchRecorded := false
	fail := func(reason string, cause error) (string, error) {
		if cause == nil {
			cause = phaseDenied(reason)
		}
		text := explanation + "\n\n" + reason
		if launchRecorded {
			text += " The saved launch remains responsible for execution."
		} else {
			text += " No new phase was started."
		}
		if handoff != nil {
			handoff.Content = text
			handoff.UpdatedAt = time.Now().UTC()
			_ = s.projectStudioRepo.UpdateMessage(ctx, handoff)
		}
		return text, cause
	}
	approval, err := s.phaseApproval(ctx, d, q.OwnerID, q.RoomID)
	if err != nil {
		return fail("The original plan approval could not be verified.", err)
	}
	if d.Status != task.TaskStatusDone || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil {
		return fail("The preceding phase has not been accepted.", nil)
	}
	approved := *d
	approved.Title = fmt.Sprintf("Phase %d: current", approval.Phase)
	approved.Instructions = approval.Plan.Plan.Description
	approved.SourceMessageID = ""
	target, ok := s.detectNextMultiPhase(ctx, &approved)
	if !ok {
		return explanation, nil
	}
	policy, err := s.taskPublicationPolicy(ctx, d)
	if err != nil {
		return fail("The approved execution limits could not be verified.", err)
	}
	limits := ""
	if policy.ReadOnlyRepository {
		limits += "Repository is read-only.\n"
	}
	if policy.DenyPush {
		limits += "Do not push.\n"
	}
	if policy.DenyDeploy {
		limits += "Do not deploy.\n"
	}
	phaseLabel := fmt.Sprintf("Phase %d: %s", target.NextPhase, target.NextTitle)
	inputs, err := s.acceptedPhaseInputs(ctx, q.OwnerID, d)
	if err != nil {
		return fail("The accepted preceding-phase inputs could not be prepared.", err)
	}
	nextTitle := phaseTaskTitle(target.NextPhase, target.NextTitle)
	key := fmt.Sprintf("auto-phase:%s:%d", approval.Root.ID, target.NextPhase)
	requestID, inputMsgID := delegationMessageID(key), delegationMessageID(key+":input")
	now := time.Now().UTC()
	store, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return fail("Conversation storage is unavailable.", nil)
	}
	nextReq := db.InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: q.RoomID, RequestID: requestID, MessageID: inputMsgID, ReplyID: inputMsgID, PayloadHash: requestID, State: "completed", CreatedAt: now, Deadline: now.Add(10 * time.Minute)}
	msg := &db.ProjectMessage{ID: inputMsgID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_auto_delegation", AuthorName: "Cuan", Content: fmt.Sprintf("Phase %d verified complete. Preparing %s.", approval.Phase, nextTitle), CreatedAt: now, UpdatedAt: now}
	accepted, fresh, err := store.AcceptInboxRequest(ctx, nextReq, msg)
	if err != nil {
		return fail("The phase handoff could not be saved.", err)
	}
	if accepted.OwnerID != q.OwnerID || accepted.RoomID != q.RoomID || accepted.MessageID != inputMsgID {
		return fail("The saved handoff identity differs.", nil)
	}
	if !fresh {
		stored, e := s.projectStudioRepo.GetMessageByID(ctx, inputMsgID)
		if e != nil || stored == nil || stored.AuthorType != "system" || stored.AuthorID != "cuan_auto_delegation" {
			return fail("The saved handoff message differs.", e)
		}
		msg = stored
	}
	handoff = msg
	if accepted.State == "cancelled" || accepted.State == "canceled" || accepted.State == "cancelling" {
		return fail("The phase handoff was cancelled.", nil)
	}
	inherited := phaseTaskInstructions(approval.Root.Instructions)
	if approval.Plan.Plan.Description != "" {
		inherited = strings.ReplaceAll(inherited, approval.Plan.Plan.Description, "[See the approved plan above.]")
	}
	nextInstructions := fmt.Sprintf("Cuan managed delegation v1.\nActive Phase: %d\n%s\n\nApproved phase: %s\n\nOriginal approved plan:\n%s\n\nHistorical root context (reference only; the active approved phase above controls):\n%s\n\nApproval message: %s\nConversation: %s\nExecute only the active phase and report actual verification evidence.", target.NextPhase, limits, phaseLabel, approval.Plan.Plan.Description, inherited, approval.Root.SourceMessageID, q.RoomID)
	// Each new phase must prove its own approved outcome. Never inherit a
	nextInstructions = strings.TrimSpace(nextInstructions + inputs)
	// predecessor's completed checklist or leave the next worker without DoD.
	criteria := []task.DoDItem{{Text: target.NextTitle}}
	created, err := s.taskSvc.CreateTaskFromMessage(ctx, q.OwnerID, inputMsgID, task.CreateTaskRequest{SourceRoomID: q.RoomID, ProjectID: d.ProjectID, Title: nextTitle, Instructions: nextInstructions, AssignedAgent: q.AgentID, ExecutionCategory: d.ExecutionCategory, DoDChecklist: criteria})
	if err != nil || created == nil {
		return fail("The next phase task could not be saved.", err)
	}
	_, _ = s.syncRoomTitleOnTaskCreated(ctx, q.RoomID, nextTitle, q.OwnerID)
	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil || detail == nil || detail.ProjectID != d.ProjectID || detail.SourceRoomID != q.RoomID || detail.SourceMessageID != inputMsgID || phaseTaskInstructions(detail.Instructions) != nextInstructions || detail.ExecutionCategory != d.ExecutionCategory {
		return fail("The saved phase scope differs from the approved handoff.", err)
	}
	if err := s.savePhaseLineage(ctx, detail, d, approval, q.OwnerID, q.RoomID); err != nil {
		return fail("The next phase authorization could not be saved.", err)
	}
	// Refresh the receipt-bearing detail before launch authorization reads it.
	detail, err = s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil || detail == nil {
		return fail("The saved phase authorization could not be read.", err)
	}
	outcome := s.startAuthorizedLaunch(ctx, accepted, detail, requestID, approval.Request.RequestID, "")
	// The launch text is presentation only. Recovery ownership transfers only
	// after the durable launch can actually be read.
	if _, launchErr := s.taskSvc.GetLaunch(ctx, task.LaunchID(detail.ID, requestID)); launchErr != nil {
		return explanation + "\n\n" + outcome, launchErr
	}
	launchRecorded = true
	status := fmt.Sprintf("Phase %d verified complete. Next phase: %s.\n\n%s", approval.Phase, nextTitle, outcome)
	msg.Content = status
	if !fresh {
		stored, e := s.projectStudioRepo.GetMessageByID(ctx, inputMsgID)
		if e != nil || stored == nil {
			return fail("The phase status could not be read.", e)
		}
		msg = stored
		msg.Content = status
	}
	msg.UpdatedAt = time.Now().UTC()
	if err := s.projectStudioRepo.UpdateMessage(ctx, msg); err != nil {
		return explanation + "\n\n" + outcome + "\n\nThe phase status could not be saved.", err
	}
	s.publishInbox(q.RoomID)
	return explanation + "\n\n" + status, nil
}

func (s *Server) inboxAutoDelegationPrompt(ctx context.Context, q db.InboxRequest) (string, bool) {
	if s.projectStudioRepo == nil {
		return "", false
	}
	m, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || m == nil || m.RoomID != q.RoomID || m.AuthorType != "system" || m.AuthorID != "cuan_auto_delegation" {
		return "", false
	}
	return m.Content, true
}
