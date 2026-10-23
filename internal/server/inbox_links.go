package server

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	cuanURLRegex      = regexp.MustCompile(`https?://[^\s]+`)
	cuanRoomLinkRegex = regexp.MustCompile(`(?:https?://[^/\s]+)?/(?:inbox|chat|conversations|studio)/([a-fA-F0-9-]{36})`)
	cuanTaskLinkRegex = regexp.MustCompile(`(?:https?://[^/\s]+)?(?:/projects/[^/\s]+)?/tasks/([a-zA-Z0-9_-]{8,})`)
	cuanBareTaskRegex = regexp.MustCompile(`\b(task_[a-zA-Z0-9]{8,}|task-[a-zA-Z0-9]{8,})\b`)
)

func (s *Server) resolveInternalCuanLinks(ctx context.Context, text string, currentRoomID string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	var b strings.Builder
	// 1. Check for room / conversation links
	roomMatches := cuanRoomLinkRegex.FindAllStringSubmatch(text, -1)
	seenRooms := map[string]bool{currentRoomID: true}
	var processedRooms int
	for _, m := range roomMatches {
		if len(m) < 2 {
			continue
		}
		roomID := m[1]
		if seenRooms[roomID] {
			continue
		}
		seenRooms[roomID] = true
		processedRooms++
		if processedRooms > 3 {
			break
		}
		if s.projectStudioRepo == nil {
			continue
		}
		room, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
		if err != nil || room == nil {
			continue
		}
		messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID)
		if err != nil || len(messages) == 0 {
			continue
		}
		sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreatedAt.Before(messages[j].CreatedAt) })
		fmt.Fprintf(&b, "\n[Referenced Internal Cuan Conversation: Room %s, Title: %q]\n", roomID, room.Title)
		b.WriteString("(This is an internal Cuan conversation from this application, NOT an external link. Analyze it directly):\n")
		if len(messages) <= 25 {
			for _, msg := range messages {
				clean, _ := decodeAttachmentMessage(msg.Content)
				clean = truncateProjectText(clean, 600)
				fmt.Fprintf(&b, "- %s: %s\n", strings.ToUpper(msg.AuthorType), clean)
			}
		} else {
			for _, msg := range messages[:2] {
				clean, _ := decodeAttachmentMessage(msg.Content)
				clean = truncateProjectText(clean, 600)
				fmt.Fprintf(&b, "- %s: %s\n", strings.ToUpper(msg.AuthorType), clean)
			}
			omitted := len(messages) - 22
			fmt.Fprintf(&b, "  [... %d pesan obrolan tengah disingkat ...]\n", omitted)
			for _, msg := range messages[len(messages)-20:] {
				clean, _ := decodeAttachmentMessage(msg.Content)
				clean = truncateProjectText(clean, 600)
				fmt.Fprintf(&b, "- %s: %s\n", strings.ToUpper(msg.AuthorType), clean)
			}
		}
	}

	// 2. Check for task links or task IDs
	taskMatches := cuanTaskLinkRegex.FindAllStringSubmatch(text, -1)
	for _, bm := range cuanBareTaskRegex.FindAllStringSubmatch(text, -1) {
		taskMatches = append(taskMatches, bm)
	}
	seenTasks := map[string]bool{}
	var processedTasks int
	for _, m := range taskMatches {
		if len(m) < 2 {
			continue
		}
		taskID := m[1]
		if seenTasks[taskID] {
			continue
		}
		seenTasks[taskID] = true
		processedTasks++
		if processedTasks > 3 {
			break
		}
		if s.taskSvc == nil {
			continue
		}
		detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
		if err != nil || detail == nil {
			continue
		}
		fmt.Fprintf(&b, "\n[Referenced Internal Cuan Task: ID %s, Title: %q, Status: %s]\n", detail.ID, detail.Title, detail.Status)
		b.WriteString("(This is an internal Cuan task from this application, NOT an external link. Analyze it directly):\n")
		fmt.Fprintf(&b, "Summary: %s\n", truncateProjectText(detail.Summary, 600))
		if detail.LatestAttempt != nil {
			fmt.Fprintf(&b, "Latest Attempt Status: %s, Error: %s\n", detail.LatestAttempt.Status, truncateProjectText(detail.LatestAttempt.ErrorMessage, 500))
			if logs := s.formatAttemptTerminalLogs(ctx, detail.LatestAttempt.ID, 50); logs != "" {
				fmt.Fprintf(&b, "Recent Attempt Terminal Output (up to 50 lines):\n%s\n", indentLines(truncateProjectText(logs, 2500), "  | "))
			}
		}
		for i := len(detail.Activities) - 1; i >= 0; i-- {
			act := detail.Activities[i]
			if act != nil && strings.TrimSpace(act.Message) != "" {
				fmt.Fprintf(&b, "Latest Activity: %s\n", truncateProjectText(act.Message, 500))
				break
			}
		}
	}

	return b.String()
}
