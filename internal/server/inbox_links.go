package server

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	cuanURLRegex          = regexp.MustCompile(`https?://[^\s]+`)
	cuanRoomLinkRegex     = regexp.MustCompile(`(?:https?://[^/\s]+)?/(?:inbox|chat|conversations|studio)/([a-zA-Z0-9_-]{6,36})`)
	cuanBareRoomRegex     = regexp.MustCompile(`\b(room_[a-zA-Z0-9_-]{6,}|room-[a-zA-Z0-9_-]{6,})\b`)
	cuanExplicitRoomRegex = regexp.MustCompile(`(?i)\b(?:room|inbox|chat)\s*[:=]?\s*([a-fA-F0-9-]{36})\b`)
	cuanTaskLinkRegex     = regexp.MustCompile(`(?:https?://[^/\s]+)?(?:/projects/[^/\s]+)?/tasks/([a-zA-Z0-9_-]{6,})`)
	cuanBareTaskRegex     = regexp.MustCompile(`\b(task_[a-zA-Z0-9_-]{6,}|task-[a-zA-Z0-9_-]{6,}|msg_[a-zA-Z0-9_-]{8,})\b`)
	viewWorkLinkRegex     = regexp.MustCompile(`(?i)\[(?:View work|Lihat rincian pekerjaan)\]\(\s*(?:https?://[^/\s]+)?/tasks/([a-zA-Z0-9_-]+)/?\s*\)`)
)

// deduplicateWorkLinks ensures that any [View work](/tasks/<id>) link appears at most once
// per unique task ID in an agent reply, preserving the first occurrence and retaining links
// to different tasks.
func deduplicateWorkLinks(content string) string {
	matches := viewWorkLinkRegex.FindAllStringSubmatchIndex(content, -1)
	if len(matches) <= 1 {
		return content
	}

	seen := make(map[string]bool)
	type span struct {
		start int
		end   int
	}
	var duplicates []span

	for _, m := range matches {
		taskID := content[m[2]:m[3]]
		if seen[taskID] {
			duplicates = append(duplicates, span{start: m[0], end: m[1]})
		} else {
			seen[taskID] = true
		}
	}

	if len(duplicates) == 0 {
		return content
	}

	result := content
	for i := len(duplicates) - 1; i >= 0; i-- {
		d := duplicates[i]
		start := d.start
		end := d.end

		lineStart := start
		for lineStart > 0 && result[lineStart-1] != '\n' {
			lineStart--
		}
		lineEnd := end
		for lineEnd < len(result) && result[lineEnd] != '\n' {
			lineEnd++
		}

		isPrefixWS := strings.TrimSpace(result[lineStart:start]) == ""
		isSuffixWS := strings.TrimSpace(result[end:lineEnd]) == ""

		if isPrefixWS && isSuffixWS {
			removeStart := lineStart
			if removeStart > 0 && result[removeStart-1] == '\n' {
				removeStart--
				if removeStart > 0 && result[removeStart-1] == '\r' {
					removeStart--
				}
				if removeStart > 0 && result[removeStart-1] == '\n' {
					removeStart--
					if removeStart > 0 && result[removeStart-1] == '\r' {
						removeStart--
					}
				}
			}
			removeEnd := lineEnd
			if removeStart == 0 && removeEnd < len(result) && result[removeEnd] == '\n' {
				removeEnd++
				if removeEnd < len(result) && result[removeEnd] == '\r' {
					removeEnd++
				}
			}
			result = result[:removeStart] + result[removeEnd:]
		} else {
			removeStart := start
			if removeStart > 0 && result[removeStart-1] == ' ' {
				removeStart--
			}
			result = result[:removeStart] + result[end:]
		}
	}

	return strings.TrimRight(result, " \t\r\n")
}

func cleanCuanReferenceID(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.IndexAny(raw, "?#"); idx != -1 {
		raw = raw[:idx]
	}
	return strings.TrimRight(raw, "/.,:;!?)]}>\"'")
}

func (s *Server) resolveInternalCuanLinks(ctx context.Context, text string, currentRoomID string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	var b strings.Builder
	// 1. Check for room / conversation links
	var roomMatches [][]string
	for _, m := range cuanRoomLinkRegex.FindAllStringSubmatch(text, -1) {
		roomMatches = append(roomMatches, m)
	}
	for _, bm := range cuanBareRoomRegex.FindAllStringSubmatch(text, -1) {
		roomMatches = append(roomMatches, bm)
	}
	for _, em := range cuanExplicitRoomRegex.FindAllStringSubmatch(text, -1) {
		roomMatches = append(roomMatches, em)
	}

	seenRooms := map[string]bool{currentRoomID: true}
	var processedRooms int
	for _, m := range roomMatches {
		if len(m) < 2 {
			continue
		}
		roomID := cleanCuanReferenceID(m[1])
		if roomID == "" || seenRooms[roomID] {
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
		if err != nil {
			messages = nil
		}
		sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreatedAt.Before(messages[j].CreatedAt) })
		fmt.Fprintf(&b, "\n[Referenced Internal Cuan Conversation: Room %s, Title: %q]\n", roomID, room.Title)
		b.WriteString("(This is an internal Cuan conversation from this application, NOT an external link. Analyze it directly):\n")
		if len(messages) == 0 {
			b.WriteString("(No messages recorded in this room yet)\n")
		} else if len(messages) <= 25 {
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
		taskID := cleanCuanReferenceID(m[1])
		if taskID == "" || seenTasks[taskID] {
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
