package server

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/buchin/cuan/internal/db"
)

// customerAttachmentBaseURL is the public origin widget visitors should use to
// fetch their own attachments. It follows the configured public app URL, then
// the configured base domain, so no retired host is baked into responses.
func customerAttachmentBaseURL(s *Server) string {
	if value := strings.TrimSpace(os.Getenv("CUAN_PUBLIC_APP_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	if s != nil && s.cfg != nil {
		if domain := strings.TrimSpace(s.cfg.BaseDomain); domain != "" && domain != "localhost" {
			return "https://" + domain
		}
	}
	return ""
}

// isCustomerRoom returns true if the given room represents a visitor/customer session.
func (s *Server) isCustomerRoom(ctx context.Context, roomID string) bool {
	if roomID == "" {
		return false
	}
	if sess, err := s.crmSession(ctx, roomID); err == nil && sess != nil {
		return true
	}
	if s.projectStudioRepo != nil {
		if rm, err := s.projectStudioRepo.GetRoomByID(ctx, roomID); err == nil && rm != nil {
			if strings.HasPrefix(rm.Topic, "agent:") || strings.HasPrefix(rm.Topic, "agent-topic:") || rm.ID == defaultScrumMasterRoomID || rm.ID == defaultAntigravityRoomID {
				return false
			}
			return true
		}
	}
	return false
}

// extractCustomerReplyDraft parses a ready-to-use customer response draft from agent content.
func extractCustomerReplyDraft(content string) string {
	_, draft := splitCustomerReplyDraft(content)
	return draft
}

var internalDraftLink = regexp.MustCompile(`\[[^\]]*\]\(/(?:tasks|inbox|projects)/[^)]*\)`)

type customerReplyPayload struct {
	Draft string `json:"draft"`
}

func splitCustomerReplyDraft(content string) (string, string) {
	const marker = "```cuan-customer-reply\n"
	if start := strings.Index(content, marker); start >= 0 {
		body := start + len(marker)
		if end := strings.Index(content[body:], "```"); end >= 0 {
			var payload customerReplyPayload
			if strictCoordinatorJSON([]byte(content[body:body+end]), &payload) == nil {
				draft := strings.TrimSpace(internalDraftLink.ReplaceAllString(payload.Draft, ""))
				if draft != "" && len(draft) <= 6000 && !strings.Contains(draft, "```cuan-") {
					return strings.TrimSpace(content[:start] + content[body+end+3:]), draft
				}
			}
		}
		return content, ""
	}
	lower := strings.ToLower(content)
	markers := []string{
		"**draf balasan untuk pengunjung:**",
		"**draft balasan untuk pengunjung:**",
		"**draf balasan pengunjung:**",
		"**draft balasan pengunjung:**",
		"**draf balasan:**",
		"**draft balasan:**",
		"**customer reply draft:**",
		"draf balasan untuk pengunjung:",
		"draft balasan untuk pengunjung:",
		"draf balasan pengunjung:",
		"draft balasan pengunjung:",
		"balasan siap pakai:",
	}
	for _, marker := range markers {
		idx := strings.Index(lower, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		lines := strings.SplitAfter(content[start:], "\n")
		consumed := 0
		var draftLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" && len(draftLines) == 0 {
				consumed += len(line)
				continue
			}
			if trimmed == "" || strings.HasPrefix(trimmed, "**") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "[View work]") || strings.HasPrefix(trimmed, "[Tinjau rincian") {
				break
			}
			clean := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, ">")), "\"")
			clean = strings.TrimSpace(internalDraftLink.ReplaceAllString(clean, ""))
			if clean != "" {
				draftLines = append(draftLines, clean)
			}
			consumed += len(line)
		}
		if len(draftLines) > 0 {
			return strings.TrimSpace(content[:idx] + "\n\n" + content[start+consumed:]), strings.Join(draftLines, "\n")
		}
	}
	return content, ""
}

func (s *Server) getFormattedMessages(ctx context.Context, roomID string, product string) []ChatMessageDTO {
	messages, _ := s.getFormattedMessagesResult(ctx, roomID, product)
	return messages
}

func (s *Server) getFormattedMessagesResult(ctx context.Context, roomID, product string) ([]ChatMessageDTO, error) {
	if s.projectStudioRepo == nil {
		return []ChatMessageDTO{}, nil
	}

	rawMsgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if len(rawMsgs) == 0 {
		return []ChatMessageDTO{}, nil
	}

	isCustomer := s.isCustomerRoom(ctx, roomID)
	workUpdates := s.inboxWorkUpdates(ctx, roomID, rawMsgs)
	approved := map[string]bool{}
	deferred := map[string]bool{}
	for _, m := range rawMsgs {
		if m.AuthorType == "user" {
			if p := decodeInboxPlanDecision(m.Content); p != nil {
				if p.Action == "defer" {
					deferred[p.MessageID] = true
				} else {
					approved[p.MessageID] = true
				}
			}
		}
	}
	dtos := make([]ChatMessageDTO, 0, len(rawMsgs))
	emailDeliveries := map[string]*ChatEmailDelivery{}
	if isCustomer {
		emailDeliveries, err = s.chatEmailDeliveries(ctx, roomID)
		if err != nil {
			return nil, err
		}
	}
	receipts := map[string]db.MessageReceipt{}
	if store, ok := s.projectStudioRepo.(db.ChatReceiptRepository); ok && isCustomer {
		receipts, err = store.MessageReceipts(ctx, roomID)
		if err != nil {
			return nil, err
		}
	}
	for _, m := range rawMsgs {
		// Filter out internal synthetic review trigger prompts from client chat timeline
		if internalInboxReviewInput(m) {
			continue
		}

		senderType := "visitor"
		if isCustomer {
			if m.AuthorID == "operator_user" || m.AuthorName == "You" || m.AuthorName == "Owner" || m.AuthorType == "agent" || m.AuthorType == "system" {
				senderType = "operator"
			}
		} else {
			if m.AuthorType == "agent" || m.AuthorType == "system" {
				senderType = "operator"
			}
		}

		regenerateID := ""
		if m.AuthorType == "user" && m.AuthorID == "operator_user" {
			regenerateID = decodeRegenerateReplyID(m.Content)
		}
		mention := extractMentionHandle(m.Content)
		isInternal := false
		if isCustomer {
			isInternal = customerMessageInternal(&m)
		}
		messageContent := m.Content
		replyDraft := ""
		if m.AuthorType == "agent" {
			messageContent, replyDraft = splitCustomerReplyDraft(messageContent)
		}

		var actionCard *AgentActionCard
		content, draft := splitInboxTaskDraft(m.AuthorType, messageContent)
		content, approvalTitle := splitInboxApproval(m.AuthorType, content)
		content, promptChoices := splitInboxPromptChoices(m.AuthorType, content)
		var approvalPlan *inboxApprovalPlan
		approvalRequestID := ""
		if m.AuthorType == "agent" {
			approvalPlan = parseInboxApprovalPlan(m.Content)
			if approvalPlan != nil {
				approvalRequestID = delegationMessageID("approve-plan:" + roomID + ":" + m.ID)
				content = presentContinuationText(content, approvalPlan)
				approvalPlan = presentContinuationPlan(approvalPlan)
			}
		}
		var attached []conversationAttachment
		if m.AuthorType == "user" || m.AuthorType == "agent" {
			var ids []string
			content, ids = decodeAttachmentMessage(content)
			for _, id := range ids {
				// Owner-only APIs share the account context; files remain private.
				a, _, err := loadAttachmentForMessage(ctx, s, roomID, m.ID, id)
				if err != nil {
					a, _, err = loadAttachment("visitor", id)
				}
				if err != nil {
					a, _, err = findAttachmentAcrossOwners(id)
				}
				if err == nil {
					if isCustomer {
						// Widget visitors resolve attachments on the public app origin.
						a.URL = customerAttachmentBaseURL(s) + "/api/chat/attachments/" + a.ID
					}
					attached = append(attached, a)
				}
			}
		}

		var attURL, attType, attName string
		var attSize int64
		var hasAtt bool
		if len(attached) > 0 {
			attURL = attached[0].URL
			attType = attached[0].Type
			attName = attached[0].Name
			attSize = attached[0].Size
			hasAtt = true
		}

		var groundingTastes []GroundingTaste
		if m.AuthorType == "agent" {
			if recorded, ok := s.GetMessageGrounding(m.ID); ok && len(recorded) > 0 {
				groundingTastes = recorded
			} else {
				groundingTastes = s.activeBrandGroundingTastesForRoom(ctx, roomID)
			}
		}

		dtos = append(dtos, ChatMessageDTO{
			RegenerateReplyID: regenerateID,
			WorkUpdate:        workUpdates[m.ID],
			MessageReceipt:    receipts[m.ID],
			EmailDelivery:     emailDeliveries[m.ID],
			ID:                int64(len(dtos) + 1), // Monotonic numeric ID for Dojo widget
			UUID:              m.ID,
			ApprovalRequestID: approvalRequestID,
			ApprovalAccepted:  approved[m.ID],
			PlanDeferred:      deferred[m.ID],
			PlanDeferRequestID: func() string {
				if approvalPlan != nil {
					return delegationMessageID("defer-plan:" + roomID + ":" + m.ID)
				}
				return ""
			}(),
			ChatSessionUUID: roomID,
			RoomID:          roomID,
			SenderType:      senderType,
			AuthorType:      m.AuthorType,
			AuthorID:        m.AuthorID,
			AuthorName:      m.AuthorName,
			Message:         content,
			Content:         content,
			Product:         product,
			AttachmentURL:   attURL,
			AttachmentType:  attType,
			AttachmentName:  attName,
			AttachmentSize:  attSize,
			HasAttachment:   hasAtt,
			ActionCard:      actionCard,
			TaskDraft:       draft,
			IsInternal:      isInternal,
			MentionHandle:   mention,
			ReplyDraft:      replyDraft,
			ApprovalTitle:   approvalTitle,
			ApprovalPlan:    approvalPlan,
			PromptChoices:   promptChoices,
			Attachments:     attached,
			CreatedAt:       m.CreatedAt,
			GroundingTastes: groundingTastes,
		})
	}

	return dtos, nil
}

// SMTriageResult holds distilled problem analysis and DoD.
type SMTriageResult struct {
	ProblemTitle string
	WorkerAgent  string
	DoDItems     []db.DoDItem
	Text         string
}

func generateSMTriageResponse(userMessage string) SMTriageResult {
	raw := strings.TrimSpace(userMessage)
	title := raw
	lower := strings.ToLower(raw)
	for _, prefix := range []string{
		"halo scrum master, tolong siapkan sprint backlog untuk ",
		"halo scrum master, tolong buatkan ",
		"halo scrum master, tolong perbaiki ",
		"halo scrum master, ",
		"scrum master, tolong ",
		"tolong siapkan sprint backlog untuk ",
		"tolong buatkan ",
		"tolong perbaiki ",
		"tolong ",
		"buatkan ",
		"perbaiki ",
	} {
		if strings.HasPrefix(lower, prefix) {
			title = strings.TrimSpace(raw[len(prefix):])
			break
		}
	}

	if len(title) > 60 {
		title = title[:57] + "..."
	}
	if title == "" {
		title = raw
		if len(title) > 60 {
			title = title[:57] + "..."
		}
	}

	worker := "antigravity-flash"

	// Dynamic DoD based on user prompt topic
	lowerPrompt := strings.ToLower(raw)
	var dodList []db.DoDItem

	if strings.Contains(lowerPrompt, "bug") || strings.Contains(lowerPrompt, "error") || strings.Contains(lowerPrompt, "perbaiki") || strings.Contains(lowerPrompt, "fix") {
		dodList = []db.DoDItem{
			{Text: "Investigate the root cause and isolate the affected module", Completed: false},
			{Text: "Implement the bug fix in the repository", Completed: false},
			{Text: "Targeted automated tests pass with no regressions", Completed: false},
		}
	} else if strings.Contains(lowerPrompt, "deploy") || strings.Contains(lowerPrompt, "vps") || strings.Contains(lowerPrompt, "rilis") || strings.Contains(lowerPrompt, "release") {
		dodList = []db.DoDItem{
			{Text: "Build the frontend bundle and production binary", Completed: false},
			{Text: "Sync the release to the target server and restart the service", Completed: false},
			{Text: "Public smoke test returns HTTP 200 and live verification passes", Completed: false},
		}
	} else {
		dodList = []db.DoDItem{
			{Text: "Write technical specifications and break down the repository tasks", Completed: false},
			{Text: "Implement the feature using the project architecture", Completed: false},
			{Text: "Targeted automated unit test PASS & audit evidence", Completed: false},
		}
	}

	draftReply := "Halo Kak, terima kasih sudah menghubungi kami. Tim kami sedang menindaklanjuti kendala ini dan segera memperbaikinya. Mohon ditunggu ya! 🙏"
	if strings.Contains(lowerPrompt, "fix") || strings.Contains(lowerPrompt, "perbaiki") || strings.Contains(lowerPrompt, "bug") || strings.Contains(lowerPrompt, "error") {
		draftReply = "Halo Kak, terima kasih laporannya. Tim kami sedang memeriksa dan memperbaiki kendala tersebut. Kami kabari segera setelah selesai ya!"
	}

	respText := fmt.Sprintf(`Here is a proposed plan for your request:

**Problem Statement:**
%s

**Execution plan & definition of done:**
1. %s
2. %s
3. %s

**Draf Balasan untuk Pengunjung:**
> "%s"

**Recommended worker agent:** %s (check quota before starting).
Select **Approve and start task** below to begin execution.`, title, dodList[0].Text, dodList[1].Text, dodList[2].Text, draftReply, worker)

	return SMTriageResult{
		ProblemTitle: title,
		WorkerAgent:  worker,
		DoDItems:     dodList,
		Text:         respText,
	}
}

// Only executing work belongs in the working count.
func inboxTaskIsWorking(status string) bool {
	switch status {
	case "doing", "in_progress", "running":
		return true
	}
	return false
}
