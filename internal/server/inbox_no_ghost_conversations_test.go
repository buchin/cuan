package server

import (
	"context"
	"testing"
)

func TestListingInboxDoesNotRecreateDeletedConversations(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	before, err := s.projectStudioRepo.ListAllRooms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		w := call("GET", "/api/v1/chat/contacts", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		after, err := s.projectStudioRepo.ListAllRooms(context.Background())
		if err != nil || len(after) != len(before) {
			t.Fatalf("listing Inbox created a new conversation: before=%d after=%d err=%v", len(before), len(after), err)
		}
	}
}
