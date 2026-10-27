package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLazyCRMContactCreationAndPrune(t *testing.T) {
	srv, token := setupTestChatServer(t)

	call := func(method, path, body string, authed bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if authed {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, r)
		return w
	}

	// 1. Visitor connects anonymously without name, email, or message
	initBody := `{"guest_uuid":"anon-visitor-1","product":"WebStore","current_url":"https://example.com/shop"}`
	recInit := call(http.MethodPost, "/api/chat/session", initBody, false)
	if recInit.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recInit.Code, recInit.Body.String())
	}
	var initResp struct {
		SessionUUID string `json:"session_uuid"`
	}
	if err := json.Unmarshal(recInit.Body.Bytes(), &initResp); err != nil || initResp.SessionUUID == "" {
		t.Fatalf("failed to decode session_uuid: %v", err)
	}
	anonSessionID := initResp.SessionUUID

	// 2. Contacts list must NOT contain this anonymous visitor because they haven't engaged
	recContacts := call(http.MethodGet, "/api/v1/contacts", "", true)
	if recContacts.Code != http.StatusOK {
		t.Fatalf("contacts list failed: %d", recContacts.Code)
	}
	var listResp struct {
		Contacts []CRMContact `json:"contacts"`
	}
	_ = json.Unmarshal(recContacts.Body.Bytes(), &listResp)
	for _, c := range listResp.Contacts {
		if c.GuestUUID == "anon-visitor-1" {
			t.Fatalf("anonymous visitor without messages should NOT be in CRM contacts: %+v", c)
		}
	}

	// 3. Telemetry and message polling work smoothly for the transient session
	recTelem := call(http.MethodPost, "/api/chat/sessions/"+anonSessionID+"/page", `{"current_url":"https://example.com/cart"}`, false)
	if recTelem.Code != http.StatusOK {
		t.Fatalf("telemetry failed: %d", recTelem.Code)
	}

	recPoll := call(http.MethodGet, "/api/chat/messages?session_uuid="+anonSessionID, "", false)
	if recPoll.Code != http.StatusOK {
		t.Fatalf("polling messages failed: %d", recPoll.Code)
	}

	// 4. Now visitor sends their first message -> promotes to CRM contact
	sendBody := `{"session_uuid":"` + anonSessionID + `","text":"Hello, is anyone there?"}`
	recSend := call(http.MethodPost, "/api/chat/messages", sendBody, false)
	if recSend.Code != http.StatusOK {
		t.Fatalf("sending message failed: %d: %s", recSend.Code, recSend.Body.String())
	}

	// 5. Contacts list must now contain the visitor because they sent a message
	recContactsAfter := call(http.MethodGet, "/api/v1/contacts", "", true)
	if recContactsAfter.Code != http.StatusOK {
		t.Fatalf("contacts list failed: %d", recContactsAfter.Code)
	}
	var listRespAfter struct {
		Contacts []CRMContact `json:"contacts"`
	}
	_ = json.Unmarshal(recContactsAfter.Body.Bytes(), &listRespAfter)
	found := false
	for _, c := range listRespAfter.Contacts {
		if c.GuestUUID == "anon-visitor-1" {
			found = true
			if c.SessionUUID != anonSessionID {
				t.Fatalf("expected sessionUUID %s, got %s", anonSessionID, c.SessionUUID)
			}
		}
	}
	if !found {
		t.Fatalf("expected visitor to be promoted to contacts after sending message")
	}

	// 6. Test prune job
	// A completely stale empty visitor in CRM
	srv.crm.Lock()
	srv.crm.contacts["test\x00stale-ghost"] = CRMContact{
		ID:          "ghost-1",
		Product:     "test",
		GuestUUID:   "stale-ghost",
		SessionUUID: "ghost-session-1",
		Name:        "",
		Email:       "",
		CreatedAt:   time.Now().UTC().Add(-48 * time.Hour),
		UpdatedAt:   time.Now().UTC().Add(-48 * time.Hour),
	}
	srv.crm.Unlock()

	deleted, err := srv.PruneStaleAnonymousContacts(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if deleted < 1 {
		t.Fatalf("expected at least 1 deleted ghost contact, got %d", deleted)
	}

	// Ensure the active visitor who sent a message was NOT deleted
	srv.crm.Lock()
	_, activeExists := srv.crm.contacts["WebStore\x00anon-visitor-1"]
	srv.crm.Unlock()
	if !activeExists {
		t.Fatalf("active contact with message should not have been pruned")
	}
}
