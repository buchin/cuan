package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type taskReviewCompletionKey struct{}
type taskReviewRoomIDKey struct{}

func matchesTaskSourceMessage(replyTo *string, sourceMessageID string) bool {
	if sourceMessageID == "" {
		return replyTo == nil || *replyTo == ""
	}
	return replyTo != nil && *replyTo == sourceMessageID
}

// taskDeliveredImageAttachmentIDs collects unique image attachment IDs delivered by
// this task's attempts (newest first, up to 5) so Inbox completion messages render
// inline image previews directly in the chat bubble.
func (s *Server) taskDeliveredImageAttachmentIDs(ctx context.Context, t *task.Task, detail *task.TaskDetailResponse) []string {
	if s.projectStudioRepo == nil {
		return nil
	}
	if detail == nil && t != nil && s.taskSvc != nil {
		detail, _ = s.taskSvc.GetTaskDetail(ctx, t.ID)
	}
	if detail == nil || detail.SourceRoomID == "" {
		return nil
	}
	owner, _ := s.taskArtifactOwner(ctx, detail)

	attempts := make([]*task.TaskAttempt, 0, len(detail.Attempts)+1)
	if detail.LatestAttempt != nil {
		attempts = append(attempts, detail.LatestAttempt)
	}
	sortedAttempts := append([]*task.TaskAttempt(nil), detail.Attempts...)
	sort.SliceStable(sortedAttempts, func(i, j int) bool {
		if sortedAttempts[i] == nil {
			return false
		}
		if sortedAttempts[j] == nil {
			return true
		}
		return sortedAttempts[i].CreatedAt.After(sortedAttempts[j].CreatedAt)
	})
	for _, a := range sortedAttempts {
		if a == nil {
			continue
		}
		if detail.LatestAttempt != nil && a.ID == detail.LatestAttempt.ID {
			continue
		}
		attempts = append(attempts, a)
	}

	var imageIDs []string
	seen := map[string]bool{}
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		for _, prefix := range []string{"task-visual-delivery:", "task-delivery:"} {
			m, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID(prefix+attempt.ID))
			if err != nil || m == nil || m.RoomID != detail.SourceRoomID || m.AuthorType != "agent" || !matchesTaskSourceMessage(m.ReplyToID, detail.SourceMessageID) {
				continue
			}
			_, ids := decodeAttachmentMessage(m.Content)
			for _, id := range ids {
				if seen[id] {
					continue
				}
				var att conversationAttachment
				var loadErr error
				if owner != "" {
					att, _, loadErr = loadAttachment(owner, id)
				} else {
					loadErr = errors.New("no owner")
				}
				if loadErr != nil {
					att, _, loadErr = findAttachmentAcrossOwners(id)
				}
				if loadErr != nil || !isImageAttachmentNameOrType(att.Name, att.Type) {
					continue
				}
				seen[id] = true
				imageIDs = append(imageIDs, id)
				if len(imageIDs) >= 5 {
					return imageIDs
				}
			}
		}
	}
	return imageIDs
}

func isDevNullShipLogTask(title, summary string) bool {
	combined := strings.ToLower(title + "\n" + summary)
	return strings.Contains(combined, "devnull daily ship log") ||
		strings.Contains(combined, "dev-null.sh") ||
		strings.Contains(combined, "--- draft narasi siap posting ---")
}

func sanitizeDevNullDraftText(raw string) string {
	text := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	text = strings.ReplaceAll(text, "`scripts/post-to-devnull.sh`", "dev-null.sh")
	text = strings.ReplaceAll(text, "scripts/post-to-devnull.sh", "dev-null.sh")
	text = strings.ReplaceAll(text, "`short_sha`", "ringkasan commit")
	text = strings.ReplaceAll(text, "short_sha", "ringkasan commit")
	text = strings.ReplaceAll(text, "chained auto-recovery", "pemulihan otomatis berantai")
	return strings.TrimSpace(text)
}

func extractDevNullPostDraft(summary string) string {
	normalized := strings.ReplaceAll(summary, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	startIdx := -1
	endIdx := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if startIdx < 0 && (strings.Contains(lower, "draft narasi siap posting") || strings.Contains(lower, "--- draft")) {
			startIdx = i + 1
			continue
		}
		if strings.Contains(lower, "--- rekomendasi gambar") ||
			strings.HasPrefix(lower, "balas \"setuju") ||
			strings.HasPrefix(lower, "balas 'setuju") ||
			strings.HasPrefix(lower, "pilih \"setuju") {
			if startIdx >= 0 || i > 0 {
				endIdx = i
				break
			}
		}
	}
	if startIdx < 0 {
		startIdx = 0
		for i, line := range lines {
			lower := strings.ToLower(strings.TrimSpace(line))
			if strings.HasPrefix(lower, "draft daily ship log") ||
				strings.Contains(lower, "siap ditinjau sebagai approval proposal") ||
				strings.Contains(lower, "belum diposting ke dev-null.sh") ||
				lower == "" {
				startIdx = i + 1
				continue
			}
			break
		}
	}
	if startIdx >= endIdx {
		return sanitizeDevNullDraftText(summary)
	}
	extracted := strings.TrimSpace(strings.Join(lines[startIdx:endIdx], "\n"))
	if extracted == "" {
		return sanitizeDevNullDraftText(summary)
	}
	return sanitizeDevNullDraftText(extracted)
}

func formatDevNullProposalForInbox(d *task.TaskDetailResponse, hasImages bool) string {
	if d == nil {
		return ""
	}
	summary := strings.TrimSpace(d.Summary)
	if plan := parseInboxApprovalPlan(summary); plan != nil {
		prose, _ := splitInboxApproval("agent", summary)
		prose = sanitizeDevNullDraftText(prose)
		rawPlan, _ := json.Marshal(plan)
		out := strings.TrimSpace(prose) + "\n\n```cuan-approval\n" + string(rawPlan) + "\n```"
		if d.ID != "" && !strings.Contains(out, "/tasks/"+d.ID) {
			out += fmt.Sprintf("\n\n[Lihat rincian](/tasks/%s)", d.ID)
		}
		return strings.TrimSpace(out)
	}

	draft := extractDevNullPostDraft(summary)
	if len([]rune(draft)) > 5800 {
		runes := []rune(draft)
		draft = string(runes[:5800]) + "..."
	}

	intro := "Berikut preview postingan **Daily Ship Log** untuk **dev-null.sh** yang siap ditinjau:"
	if hasImages {
		intro = "Berikut preview postingan **Daily Ship Log** untuk **dev-null.sh** beserta gambar lampirannya:"
	}

	plan := inboxApprovalPlan{
		Title:             "Posting Daily Ship Log ke dev-null.sh",
		Description:       draft,
		ExecutionCategory: task.ExecutionCategoryOps,
		RequiredStages:    []string{},
	}
	if strings.TrimSpace(d.ProjectID) != "" {
		plan.ProjectID = strings.TrimSpace(d.ProjectID)
	} else if strings.TrimSpace(d.ProjectName) != "" {
		plan.ProjectName = strings.TrimSpace(d.ProjectName)
	} else {
		plan.ProjectID = "default"
	}
	rawPlan, _ := json.Marshal(plan)

	var sb strings.Builder
	sb.WriteString(intro)
	sb.WriteString("\n\n```cuan-approval\n")
	sb.Write(rawPlan)
	sb.WriteString("\n```")
	if d.ID != "" {
		sb.WriteString(fmt.Sprintf("\n\n[Lihat rincian](/tasks/%s)", d.ID))
	}
	return strings.TrimSpace(sb.String())
}

func (s *Server) attachReviewedTaskImages(ctx context.Context, q db.InboxRequest, output string) string {
	if strings.HasPrefix(output, attachmentEnvelope) {
		return output
	}
	if reviewed := s.inboxReviewedTask(ctx, q); reviewed != nil && reviewed.Status == task.TaskStatusDone {
		if imageIDs := s.taskDeliveredImageAttachmentIDs(ctx, &reviewed.Task, reviewed); len(imageIDs) > 0 {
			return encodeAttachmentMessage(output, imageIDs)
		}
	}
	return output
}

func formatTaskCompletionForInbox(d *task.TaskDetailResponse, hasImages bool) string {
	if d == nil {
		return ""
	}
	summary := strings.TrimSpace(d.Summary)
	if isDevNullShipLogTask(d.Title, summary) {
		return formatDevNullProposalForInbox(d, hasImages)
	}
	if parseInboxApprovalPlan(summary) != nil {
		out := summary
		if d.ID != "" && !strings.Contains(out, "/tasks/"+d.ID) {
			out += fmt.Sprintf("\n\n[Lihat rincian](/tasks/%s)", d.ID)
		}
		return strings.TrimSpace(out)
	}

	var sb strings.Builder
	title := strings.TrimSpace(d.Title)
	if title != "" {
		sb.WriteString(fmt.Sprintf("Pekerjaan **%s** telah selesai dan diverifikasi.\n\n", title))
	} else {
		sb.WriteString("Pekerjaan telah selesai dan seluruh hasil telah diverifikasi.\n\n")
	}
	if summary != "" {
		sb.WriteString("### Ringkasan Hasil\n")
		sb.WriteString(summary)
		sb.WriteString("\n\n")
	}
	completeNote := strings.TrimSpace(d.CompleteNote)
	if completeNote != "" && completeNote != summary {
		sb.WriteString("### Bukti Verifikasi\n")
		sb.WriteString(completeNote)
		sb.WriteString("\n\n")
	}
	sb.WriteString(fmt.Sprintf("[Lihat rincian pekerjaan](/tasks/%s)", d.ID))
	return strings.TrimSpace(sb.String())
}

// PostTaskCompletionMessage delivers a closing delivery summary and verification note
// to the associated chat room when a task transitions to 'done' (closing the communication loop).
func (s *Server) PostTaskCompletionMessage(ctx context.Context, t *task.Task) error {
	if ctx != nil && ctx.Value(taskReviewCompletionKey{}) != nil {
		reviewRoom, _ := ctx.Value(taskReviewRoomIDKey{}).(string)
		if reviewRoom == "" || t == nil || t.SourceRoomID == reviewRoom {
			// Post-verify review in inbox is actively handling this task completion
			// and will emit the unified confirmation message. Avoid duplicate emitter.
			return nil
		}
	}
	if s.projectStudioRepo == nil || t == nil || t.SourceRoomID == "" || t.Status != task.TaskStatusDone {
		return nil
	}

	msgID := delegationMessageID("task-completed-closing:" + t.ID)

	// Idempotency: avoid creating duplicate message if this task's closing note already exists
	if existing, err := s.projectStudioRepo.GetMessageByID(ctx, msgID); err == nil && existing != nil {
		return nil
	}

	summary := strings.TrimSpace(t.Summary)

	// Single-bubble idempotency: avoid inserting duplicate message if already present in room
	if msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, t.SourceRoomID); err == nil && len(msgs) > 0 {
		for i := len(msgs) - 1; i >= 0 && i >= len(msgs)-10; i-- {
			content := msgs[i].Content
			if msgs[i].ID == msgID || (strings.Contains(content, "/tasks/"+t.ID) &&
				(strings.Contains(content, "telah selesai") ||
					strings.Contains(content, "sudah diterima") ||
					strings.Contains(content, "cuan-approval") ||
					(summary != "" && strings.Contains(content, summary)))) {
				return nil
			}
		}
	}

	var detail *task.TaskDetailResponse
	if s.taskSvc != nil {
		detail, _ = s.taskSvc.GetTaskDetail(ctx, t.ID)
	}
	if detail == nil {
		detail = &task.TaskDetailResponse{Task: *t}
	}
	imageIDs := s.taskDeliveredImageAttachmentIDs(ctx, t, detail)
	content := encodeAttachmentMessage(formatTaskCompletionForInbox(detail, len(imageIDs) > 0), imageIDs)

	authorID := "agent_cuan"
	authorName := "@cuan"
	if t.AssignedAgent != "" {
		authorID = t.AssignedAgent
		if s.agentRepo != nil {
			if ag, err := s.runtimeAgent(ctx, t.AssignedAgent); err == nil && ag != nil && ag.Name != "" {
				authorName = ag.Name
			}
		}
	}

	now := time.Now().UTC()
	var replyTo *string
	if t.SourceMessageID != "" {
		replyTo = &t.SourceMessageID
	}

	msg := &db.ProjectMessage{
		ID:         msgID,
		RoomID:     t.SourceRoomID,
		AuthorType: "agent",
		AuthorID:   authorID,
		AuthorName: authorName,
		Content:    content,
		ReplyToID:  replyTo,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err := s.projectStudioRepo.CreateMessage(ctx, msg)
	if err == nil || errors.Is(err, db.ErrDuplicateMessage) {
		s.publishInbox(t.SourceRoomID)
		if s.roomEventBroker != nil {
			s.roomEventBroker.Broadcast(t.SourceRoomID, "message.created", msg.ToDTO())
		}
		return nil
	}
	return err
}
