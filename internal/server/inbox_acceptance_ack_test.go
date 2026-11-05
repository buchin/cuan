package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

type unavailableAckHistory struct {
	db.ProjectStudioRepository
	db.InboxRequestRepository
	reads int
}

func (r *unavailableAckHistory) ListInboxRequests(context.Context, string) ([]db.InboxRequest, error) {
	r.reads++
	return nil, errors.New("history unavailable")
}
func (r *unavailableAckHistory) ListMessagesByRoomID(context.Context, string) ([]db.ProjectMessage, error) {
	r.reads++
	return nil, errors.New("history unavailable")
}

func TestInboxAcceptanceAcknowledgedWithoutHistoryHydration(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	s.cfg.InboxExecutionRole = "api"
	original := s.projectStudioRepo
	repo := &unavailableAckHistory{ProjectStudioRepository: original, InboxRequestRepository: original.(db.InboxRequestRepository)}
	s.projectStudioRepo = repo
	id := db.GenerateUUID()
	body := `{"request_id":"` + id + `","message":"Accepted despite unavailable history"}`
	var first db.InboxRequest
	for i := 0; i < 2; i++ {
		w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", body)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
		var ack struct {
			Request db.InboxRequest
			Saved   bool `json:"user_message_saved"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &ack); err != nil {
			t.Fatal(err)
		}
		if !ack.Saved || ack.Request.RequestID != id || ack.Request.MessageID == "" {
			t.Fatal("missing durable acknowledgement")
		}
		if i == 0 {
			first = ack.Request
		} else if ack.Request.MessageID != first.MessageID {
			t.Fatal("duplicate replay created another message")
		}
	}
	if repo.reads != 0 {
		t.Fatal("acceptance depended on history", repo.reads)
	}
	messages, err := original.ListMessagesByRoomID(context.Background(), first.RoomID)
	if err != nil || len(messages) != 1 {
		t.Fatal("message was not persisted exactly once", err)
	}
}
