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

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

const inboxTopicTitleInstruction = `Conversation naming: when the recent discussion establishes a clear new main topic, you may append one fenced cuan-title JSON block {"title":"Short topic"} to your normal useful reply. Keep the current title when still working on the same objective, greeting, approval, correction, or brief tangent. Base a change on the recent user discussion, not your own proposed next steps. Use the user's language, 3–8 words, at most 72 characters; never include credentials, email addresses, identifiers or other private details. Do not emit this block for customer chats or replace the useful reply with metadata. This is a display label only, never authorization or a new task.`

type topicTitleState struct {
	Title     string `json:"title"`
	MessageID string `json:"message_id,omitempty"`
	UserTurns int    `json:"user_turns,omitempty"`
	Manual    bool   `json:"manual,omitempty"`
}

func loadTopicTitleStates(ownerID string) (string, map[string]topicTitleState) {
	if ownerID == "" {
		ownerID = "owner"
	}
	path, err := ownerPreferencesPath(ownerID)
	if err != nil {
		return "", map[string]topicTitleState{}
	}
	path = strings.TrimSuffix(path, ".json") + ".titles.json"
	states := map[string]topicTitleState{}
	raw, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(raw, &states)
	}
	if states == nil {
		states = map[string]topicTitleState{}
	}
	return path, states
}

func saveTopicTitleStates(path string, states map[string]topicTitleState) error {
	if path == "" || states == nil {
		return nil
	}
	raw, err := json.Marshal(states)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".titles-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if file.Chmod(0600) != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil && closeErr == nil {
		return os.Rename(file.Name(), path)
	}
	return err
}

func (s *Server) markRoomTitleManual(ownerID, roomID, title string) {
	if roomID == "" {
		return
	}
	path, states := loadTopicTitleStates(ownerID)
	states[roomID] = topicTitleState{
		Title:  title,
		Manual: true,
	}
	_ = saveTopicTitleStates(path, states)
}

func (s *Server) isRoomTitleManual(ownerID, roomID string) bool {
	if roomID == "" {
		return false
	}
	_, states := loadTopicTitleStates(ownerID)
	if state, ok := states[roomID]; ok && state.Manual {
		return true
	}
	return false
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
	if s.taskSvc != nil && s.taskSvc.Store() != nil {
		if existingTasks, err := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{SourceRoomID: q.RoomID}); err == nil && len(existingTasks) > 0 {
			// Room has associated tasks; task titles have precedence over conversational LLM topic titling.
			return
		}
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
	path, states := loadTopicTitleStates(q.OwnerID)
	previous, tracked := states[q.RoomID]
	if tracked {
		if previous.Manual || previous.Title != room.Title || previous.MessageID == q.MessageID || turns-previous.UserTurns < 3 {
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
	_ = saveTopicTitleStates(path, states)
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

// syncRoomTitleOnTaskCreated updates the conversation title to match the newly created task,
// preserving custom titles if the user manually modified the title.
func (s *Server) syncRoomTitleOnTaskCreated(ctx context.Context, roomID, taskTitle string, explicitOwner ...string) (bool, error) {
	taskTitle = strings.TrimSpace(taskTitle)
	if roomID == "" || taskTitle == "" || s.projectStudioRepo == nil {
		return false, nil
	}
	if len([]rune(taskTitle)) > 200 {
		taskTitle = string([]rune(taskTitle)[:200])
	}

	inboxRoomResolutionMu.Lock()
	defer inboxRoomResolutionMu.Unlock()

	room, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if err != nil || room == nil {
		return false, err
	}
	if room.Title == taskTitle {
		return false, nil
	}

	ownerID := "owner"
	if len(explicitOwner) > 0 && explicitOwner[0] != "" {
		ownerID = explicitOwner[0]
	} else if s.authSvc != nil {
		if owner := auth.GetOwnerFromContext(ctx); owner != nil && owner.ID != "" {
			ownerID = owner.ID
		}
	}

	// If the user manually edited or chose a custom title, do NOT overwrite it!
	if s.isRoomTitleManual(ownerID, roomID) {
		return false, nil
	}

	// Determine if current title is default/auto-generated
	isDefault := room.Title == "New conversation"

	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID)
	if err == nil && len(messages) > 0 {
		sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreatedAt.Before(messages[j].CreatedAt) })
		for _, m := range messages {
			if m.AuthorType == "user" {
				first := initialTopicTitle(m.Content)
				if room.Title == first {
					isDefault = true
				}
				break
			}
		}
	}

	// Check against tracked automatic titles in .titles.json
	path, states := loadTopicTitleStates(ownerID)
	if prev, ok := states[roomID]; ok && prev.Title == room.Title {
		isDefault = true
	}

	// Check against any existing tasks in this room (Scope 3: multiple sequential tasks in one room)
	if s.taskSvc != nil && s.taskSvc.Store() != nil {
		if existingTasks, err := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{SourceRoomID: roomID}); err == nil {
			for _, t := range existingTasks {
				if room.Title == t.Title {
					isDefault = true
					break
				}
			}
		}
	}

	// If the user manually edited the title, do NOT overwrite it!
	if !isDefault {
		return false, nil
	}

	renamer, ok := s.projectStudioRepo.(interface {
		UpdateRoomDisplayTitle(context.Context, string, string, string, string) (bool, error)
	})
	var changed bool
	if ok {
		changed, err = renamer.UpdateRoomDisplayTitle(ctx, room.ID, room.Topic, room.Title, taskTitle)
	} else {
		room.Title = taskTitle
		err = s.projectStudioRepo.UpdateRoom(ctx, room)
		changed = (err == nil)
	}
	if err != nil || !changed {
		return false, err
	}

	states[roomID] = topicTitleState{
		Title:     taskTitle,
		UserTurns: len(messages),
	}
	_ = saveTopicTitleStates(path, states)

	if s.roomEventBroker != nil {
		s.roomEventBroker.Broadcast(room.ID, "room.updated", map[string]any{
			"id":    room.ID,
			"title": taskTitle,
			"topic": room.Topic,
		})
	}
	s.publishInbox(room.ID)
	return true, nil
}
