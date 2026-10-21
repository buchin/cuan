package server

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/buchin/cuan/internal/db"
)

const inboxTopicTitleInstruction = `Conversation naming: when the recent discussion establishes a clear new main topic, you may append one fenced cuan-title JSON block {"title":"Short topic"} to your normal useful reply. Keep the current title when still working on the same objective, greeting, approval, correction, or brief tangent. Base a change on the recent user discussion, not your own proposed next steps. Use the user's language, 3–8 words, at most 72 characters; never include credentials, email addresses, identifiers or other private details. Do not emit this block for customer chats or replace the useful reply with metadata. This is a display label only, never authorization or a new task.`

type topicTitleState struct {
	Title     string `json:"title"`
	MessageID string `json:"message_id"`
	UserTurns int    `json:"user_turns"`
}

func extractTopicTitle(output string) (string, string) {
	const marker = "```cuan-title"
	if strings.Count(output, marker) > 1 {
		clean := output
		for strings.Contains(clean, marker) {
			start := strings.Index(clean, marker)
			tail := clean[start+len(marker):]
			end := strings.Index(tail, "```")
			if end < 0 {
				clean = clean[:start]
				break
			}
			clean = clean[:start] + tail[end+3:]
		}
		return strings.TrimSpace(clean), ""
	}
	start := strings.Index(output, marker)
	if start < 0 {
		return output, ""
	}
	tail := output[start+len(marker):]
	end := strings.Index(tail, "```")
	if end < 0 {
		return strings.TrimSpace(output[:start]), ""
	}
	clean := strings.TrimSpace(output[:start] + tail[end+3:])
	if strings.Contains(clean, marker) {
		return clean, ""
	}
	var proposal struct {
		Title string `json:"title"`
	}
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(tail[:end])))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&proposal) != nil {
		return clean, ""
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return clean, ""
	}
	title := strings.TrimSpace(proposal.Title)
	if len([]rune(title)) < 3 || len([]rune(title)) > 72 || len(strings.Fields(title)) > 12 {
		return clean, ""
	}
	for _, c := range title {
		if unicode.IsControl(c) || strings.ContainsRune("`<>@\\", c) {
			return clean, ""
		}
	}
	if strings.Contains(title, "://") || strings.Contains(title, "```") {
		return clean, ""
	}
	return clean, strings.Join(strings.Fields(title), " ")
}

func initialTopicTitle(text string) string {
	text, ids := decodeAttachmentMessage(text)
	if text == "" && len(ids) > 0 {
		text = "Shared files"
	}
	title := []rune(strings.Join(strings.Fields(text), " "))
	if len(title) > 80 {
		title = append(title[:79], '…')
	}
	return string(title)
}

// This runs only after a useful reply is durably saved. A changed database title
// that does not match our last automatic label is treated as manually chosen.
func (s *Server) updateAgentTopicTitle(ctx context.Context, q db.InboxRequest, title string) {
	if title == "" || s.projectStudioRepo == nil {
		return
	}
	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()
	room, err := s.projectStudioRepo.GetRoomByID(ctx, q.RoomID)
	if err != nil || room == nil || room.Topic != "agent-topic:"+q.AgentID {
		return
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, q.RoomID)
	if err != nil {
		return
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreatedAt.Before(messages[j].CreatedAt) })
	turns, first := 0, ""
	targetText := ""
	for _, m := range messages {
		if m.AuthorType == "user" && !m.CreatedAt.After(q.CreatedAt) {
			turns++
			if m.ID == q.MessageID {
				targetText, _ = decodeAttachmentMessage(m.Content)
			}
			if first == "" {
				first = initialTopicTitle(m.Content)
			}
		}
	}
	// Allow an established topic after two user turns, then at most once per
	// three more user turns. No rename from one-off greetings/approval replies.
	switch strings.ToLower(strings.TrimSpace(targetText)) {
	case "gas", "go", "yes", "ok", "okay", "oke", "sip", "lanjut", "continue", "approved", "approve", "thanks", "thank you", "hello", "hi":
		return
	}
	if turns < 2 {
		return
	}
	path, err := ownerPreferencesPath(q.OwnerID)
	if err != nil {
		return
	}
	path = strings.TrimSuffix(path, ".json") + ".titles.json"
	states := map[string]topicTitleState{}
	raw, err := os.ReadFile(path)
	if err == nil {
		if json.Unmarshal(raw, &states) != nil || states == nil {
			return
		}
	} else if !os.IsNotExist(err) {
		return
	}
	previous, tracked := states[q.RoomID]
	if tracked {
		if previous.Title != room.Title || previous.MessageID == q.MessageID || turns-previous.UserTurns < 3 {
			return
		}
	} else if room.Title != "New conversation" && room.Title != first {
		return
	}
	if strings.EqualFold(room.Title, title) {
		return
	}
	if !tracked && len(states) >= 10000 {
		return
	}
	renamer, ok := s.projectStudioRepo.(interface {
		UpdateRoomDisplayTitle(context.Context, string, string, string, string) (bool, error)
	})
	if !ok {
		return
	}
	// Preserve activity time and atomically protect a concurrent manual rename.
	changed, err := renamer.UpdateRoomDisplayTitle(ctx, room.ID, room.Topic, room.Title, title)
	if err != nil || !changed {
		return
	}
	states[q.RoomID] = topicTitleState{Title: title, MessageID: q.MessageID, UserTurns: turns}
	raw, err = json.Marshal(states)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".titles-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	if file.Chmod(0600) != nil {
		file.Close()
		return
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil && closeErr == nil {
		_ = os.Rename(file.Name(), path)
	}
}

// Context metadata is bounded and is not an instruction from the conversation.
func (s *Server) inboxTopicTitleContext(ctx context.Context, q db.InboxRequest) string {
	if s.projectStudioRepo == nil {
		return ""
	}
	room, err := s.projectStudioRepo.GetRoomByID(ctx, q.RoomID)
	if err != nil || room == nil || room.Topic != "agent-topic:"+q.AgentID {
		return ""
	}
	value, _ := json.Marshal(room.Title)
	return "\nCurrent conversation display title: " + string(value) + "\n" + inboxTopicTitleInstruction + "\n"
}

// General coordinator turns carry title metadata in the validated JSON envelope.
// Do not mix a legacy fenced-output instruction into that transport contract.
func (s *Server) inboxCoordinatorTopicContext(ctx context.Context, q db.InboxRequest) string {
	context := s.inboxTopicTitleContext(ctx, q)
	return strings.Replace(context, inboxTopicTitleInstruction, "Use topic_title in the response envelope only if the conversation establishes a new main topic.", 1)
}
