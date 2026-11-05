package server

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

// Learn only durable, explicit instructions from the authenticated request's
// own message. Visitor speech and generated replies are never personal tastes.
var durablePreferenceCue = regexp.MustCompile(`(?i)\b(?:mulai sekarang|(?:saya|aku|ane|gue|kami)\s+(?:lebih suka|prefer)|preferensi|catatan gaya|aturan baru|gaya bahasa|nada bicara|from now on|always|never|preference|correction)\b`)

func (s *Server) learnInboxPreferences(ctx context.Context, q db.InboxRequest) {
	if q.OwnerID == "" || s.projectStudioRepo == nil || !s.getTasteConfig(q.OwnerID).AutoApproveEnabled {
		return
	}
	message, err := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
	if err != nil || message == nil || message.RoomID != q.RoomID || message.AuthorType != "user" || message.AuthorID != "operator_user" || decodeRegenerateReplyID(message.Content) != "" {
		return
	}
	text := operatorPreferenceText(message.Content)
	if !durablePreferenceCue.MatchString(text) {
		return
	}
	if _, err := s.reflectTaste(ctx, q.OwnerID, ReflectTasteRequest{Text: agent.MaskSecrets(text, nil)}); err != nil {
		log.Printf("Inbox personal preference learning could not persist (request %s)", q.RequestID)
	}
}

func operatorPreferenceText(content string) string {
	text, _ := decodeAttachmentMessage(content)
	var ownLines []string
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if !inCode && !strings.HasPrefix(trimmed, ">") {
			ownLines = append(ownLines, line)
		}
	}
	return strings.Join(ownLines, "\n")
}
