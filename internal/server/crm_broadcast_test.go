package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBroadcastAudienceResolver_HybridDeduplication(t *testing.T) {
	s, token := setupTestChatServer(t)
	call := func(method, path, body string, authed bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if authed {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}

	// 1. Create contacts in CRM
	// Contact A: Alice (#backer, VIP)
	sessionA := "session-alice"
	cA, err := s.crmUpsertWithSession(t.Context(), "guest-alice", "Shop", "Alice Smith", "alice@example.com", sessionA)
	if err != nil {
		t.Fatalf("failed creating contact A: %v", err)
	}
	wPatchA := call("PATCH", "/api/v1/contacts/"+cA.ID, `{"tags":["#backer", "VIP"]}`, true)
	if wPatchA.Code != http.StatusOK {
		t.Fatalf("failed patching contact A tags: %s", wPatchA.Body.String())
	}

	// Contact B: Bob (#leads)
	sessionB := "session-bob"
	cB, err := s.crmUpsertWithSession(t.Context(), "guest-bob", "Shop", "Bob Jones", "bob@example.com", sessionB)
	if err != nil {
		t.Fatalf("failed creating contact B: %v", err)
	}
	wPatchB := call("PATCH", "/api/v1/contacts/"+cB.ID, `{"tags":["leads"]}`, true)
	if wPatchB.Code != http.StatusOK {
		t.Fatalf("failed patching contact B tags: %s", wPatchB.Body.String())
	}

	// Contact C: Charlie (no email)
	sessionC := "session-charlie"
	cC, err := s.crmUpsertWithSession(t.Context(), "guest-charlie", "Shop", "Charlie Brown", "", sessionC)
	if err != nil {
		t.Fatalf("failed creating contact C: %v", err)
	}
	call("PATCH", "/api/v1/contacts/"+cC.ID, `{"tags":["#backer"]}`, true)

	// 2. Test GET /api/v1/contacts/broadcast/tags
	wTags := call("GET", "/api/v1/contacts/broadcast/tags", "", true)
	if wTags.Code != http.StatusOK {
		t.Fatalf("expected 200 for broadcast tags, got %d: %s", wTags.Code, wTags.Body.String())
	}
	var tagResp struct {
		Tags []BroadcastTagSummary `json:"tags"`
	}
	if err := json.Unmarshal(wTags.Body.Bytes(), &tagResp); err != nil {
		t.Fatalf("failed parsing broadcast tags response: %v", err)
	}
	// Alice has #backer (1 contact with email), Charlie has #backer but no email (count should be 1)
	foundBacker := false
	for _, ts := range tagResp.Tags {
		if ts.Tag == "backer" {
			foundBacker = true
			if ts.Count != 1 {
				t.Errorf("expected backer count 1, got %d", ts.Count)
			}
		}
	}
	if !foundBacker {
		t.Errorf("backer tag not found in tags summary: %+v", tagResp.Tags)
	}

	// 3. Test POST /api/v1/contacts/broadcast/audience with Hybrid resolution
	// Filter by tag "backer", plus custom emails:
	// - "ALICE@example.com" (same as Alice, case-insensitive duplicate)
	// - "david@example.com"
	// - "David Lee <david@example.com>" (duplicate in custom emails with display name)
	reqBody := `{"tags":["backer"],"custom_emails":["ALICE@example.com","david@example.com","David Lee <david@example.com>"]}`
	wAudience := call("POST", "/api/v1/contacts/broadcast/audience", reqBody, true)
	if wAudience.Code != http.StatusOK {
		t.Fatalf("expected 200 for broadcast audience, got %d: %s", wAudience.Code, wAudience.Body.String())
	}

	var audienceResp BroadcastAudienceResponse
	if err := json.Unmarshal(wAudience.Body.Bytes(), &audienceResp); err != nil {
		t.Fatalf("failed unmarshaling audience response: %v", err)
	}

	// Expected total unique: 2 (Alice and David). Charlie has no email, Bob is not in "backer"
	if audienceResp.TotalUnique != 2 {
		t.Fatalf("expected 2 total unique recipients, got %d (recipients: %+v)", audienceResp.TotalUnique, audienceResp.Recipients)
	}
	if audienceResp.CRMCount != 1 {
		t.Errorf("expected CRMCount 1, got %d", audienceResp.CRMCount)
	}
	if audienceResp.CustomCount != 2 {
		t.Errorf("expected CustomCount 2 (Alice was in custom, David in custom), got %d", audienceResp.CustomCount)
	}

	for _, rec := range audienceResp.Recipients {
		if rec.Email == "alice@example.com" {
			if rec.Source != "both" {
				t.Errorf("expected Alice source to be 'both', got %s", rec.Source)
			}
			if rec.ContactID != cA.ID {
				t.Errorf("expected Alice ContactID %s, got %s", cA.ID, rec.ContactID)
			}
		}
		if rec.Email == "david@example.com" {
			if rec.Source != "custom" {
				t.Errorf("expected David source to be 'custom', got %s", rec.Source)
			}
			if rec.Name != "David Lee" {
				t.Errorf("expected David Name 'David Lee', got %s", rec.Name)
			}
		}
	}

	// 4. Test preview alias endpoint
	wPreview := call("POST", "/api/v1/contacts/broadcast/preview", reqBody, true)
	if wPreview.Code != http.StatusOK {
		t.Fatalf("expected 200 for preview alias, got %d: %s", wPreview.Code, wPreview.Body.String())
	}
}

func TestBroadcastAudienceResolver_Validation(t *testing.T) {
	s, token := setupTestChatServer(t)
	call := func(method, path, body string, authed bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if authed {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}

	// 1. Unauthorized access without auth token must return 401
	wUnauth := call("POST", "/api/v1/contacts/broadcast/audience", `{"tags":["leads"]}`, false)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", wUnauth.Code)
	}

	// 2. Invalid email format must return 400
	wInvalidEmail := call("POST", "/api/v1/contacts/broadcast/audience", `{"custom_emails":["not-an-email"]}`, true)
	if wInvalidEmail.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got %d: %s", wInvalidEmail.Code, wInvalidEmail.Body.String())
	}

	// 3. Invalid domain format must return 400
	wInvalidDomain := call("POST", "/api/v1/contacts/broadcast/audience", `{"custom_emails":["user@localhost"]}`, true)
	if wInvalidDomain.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for domain without dot, got %d: %s", wInvalidDomain.Code, wInvalidDomain.Body.String())
	}

	// 4. Overly long tag (>40 characters) must return 400
	longTag := "this-tag-is-way-longer-than-forty-characters-and-should-fail"
	wLongTag := call("POST", "/api/v1/contacts/broadcast/audience", `{"tags":["`+longTag+`"]}`, true)
	if wLongTag.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for overly long tag, got %d", wLongTag.Code)
	}

	// 5. Empty audience request returns 200 with 0 recipients
	wEmpty := call("POST", "/api/v1/contacts/broadcast/audience", `{"tags":[],"custom_emails":[]}`, true)
	if wEmpty.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty request, got %d: %s", wEmpty.Code, wEmpty.Body.String())
	}
	var emptyResp BroadcastAudienceResponse
	if err := json.Unmarshal(wEmpty.Body.Bytes(), &emptyResp); err != nil {
		t.Fatalf("failed unmarshaling empty response: %v", err)
	}
	if emptyResp.TotalUnique != 0 || len(emptyResp.Recipients) != 0 {
		t.Fatalf("expected 0 recipients, got %d", emptyResp.TotalUnique)
	}

	// 6. Comma/newline separated string format for custom emails
	wText := call("POST", "/api/v1/contacts/broadcast/audience", `{"custom_emails":"user1@example.com, user2@example.com\nuser3@example.com"}`, true)
	if wText.Code != http.StatusOK {
		t.Fatalf("expected 200 for string custom emails, got %d: %s", wText.Code, wText.Body.String())
	}
	var textResp BroadcastAudienceResponse
	json.Unmarshal(wText.Body.Bytes(), &textResp)
	if textResp.TotalUnique != 3 {
		t.Fatalf("expected 3 recipients from text, got %d", textResp.TotalUnique)
	}
}
