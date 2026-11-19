package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/resend"
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

type mockBroadcastResendClient struct {
	mu           sync.Mutex
	sentRequests []resend.EmailRequest
	sendDelay    time.Duration
	failEmail    string
}

func (m *mockBroadcastResendClient) SendEmail(ctx context.Context, req resend.EmailRequest) (*resend.EmailResponse, error) {
	if m.sendDelay > 0 {
		time.Sleep(m.sendDelay)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failEmail != "" && len(req.To) > 0 && req.To[0] == m.failEmail {
		return nil, errors.New("simulated provider rejection for " + m.failEmail)
	}
	m.sentRequests = append(m.sentRequests, req)
	return &resend.EmailResponse{ID: fmt.Sprintf("re_msg_%d", len(m.sentRequests))}, nil
}

func (m *mockBroadcastResendClient) ValidateAPIKey(ctx context.Context) error {
	return nil
}

func TestBroadcastSend_SuccessAndIdempotency(t *testing.T) {
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

	// 1. Create a contact with tag "community"
	c1, err := s.crmUpsertWithSession(t.Context(), "guest-alex", "Store", "Alex Turner", "alex@example.com", "session-alex")
	if err != nil {
		t.Fatalf("failed creating contact: %v", err)
	}
	call("PATCH", "/api/v1/contacts/"+c1.ID, `{"tags":["community"]}`, true)

	// Inject mock Resend client
	mockResend := &mockBroadcastResendClient{}
	s.resendClient = mockResend

	// 2. Dispatch broadcast send
	idempotencyKey := "test-idemp-" + db.GenerateUUID()
	sendPayload := map[string]any{
		"tags":            []string{"community"},
		"custom_emails":   []string{"external@partner.com"},
		"subject":         "Update Komunitas Terkini",
		"content":         "Halo semuanya,\n\nIni adalah pengumuman rilis broadcast.\n\nSalam hangat.",
		"idempotency_key": idempotencyKey,
	}
	payloadBytes, _ := json.Marshal(sendPayload)

	wSend := call("POST", "/api/v1/contacts/broadcast/send", string(payloadBytes), true)
	if wSend.Code != http.StatusOK {
		t.Fatalf("expected 200 for broadcast send, got %d: %s", wSend.Code, wSend.Body.String())
	}

	var sendResp BroadcastSendResponse
	if err := json.Unmarshal(wSend.Body.Bytes(), &sendResp); err != nil {
		t.Fatalf("failed parsing send response: %v", err)
	}

	if sendResp.Status != "success" {
		t.Errorf("expected status 'success', got %q", sendResp.Status)
	}
	if sendResp.SentCount != 2 {
		t.Errorf("expected sent_count 2, got %d", sendResp.SentCount)
	}
	if sendResp.FailedCount != 0 {
		t.Errorf("expected failed_count 0, got %d", sendResp.FailedCount)
	}
	if sendResp.TotalRecipients != 2 {
		t.Errorf("expected total_recipients 2, got %d", sendResp.TotalRecipients)
	}
	if sendResp.AlreadySent {
		t.Errorf("expected already_sent false on first execution")
	}

	// Verify Mock Client Sent Requests
	mockResend.mu.Lock()
	if len(mockResend.sentRequests) != 2 {
		mockResend.mu.Unlock()
		t.Fatalf("expected 2 emails sent via provider, got %d", len(mockResend.sentRequests))
	}

	for _, req := range mockResend.sentRequests {
		// RFC 8058 List-Unsubscribe headers
		listUnsub := req.Headers["List-Unsubscribe"]
		listUnsubPost := req.Headers["List-Unsubscribe-Post"]

		if !strings.HasPrefix(listUnsub, "<http") || !strings.HasSuffix(listUnsub, ">") {
			t.Errorf("expected List-Unsubscribe header to be wrapped in <URL>, got %q", listUnsub)
		}
		if !strings.Contains(listUnsub, "/api/v1/contacts/broadcast/unsubscribe?token=") {
			t.Errorf("expected List-Unsubscribe header to contain unsubscribe endpoint and token, got %q", listUnsub)
		}
		if listUnsubPost != "List-Unsubscribe=One-Click" {
			t.Errorf("expected List-Unsubscribe-Post to be 'List-Unsubscribe=One-Click', got %q", listUnsubPost)
		}

		// Body standard unsubscribe link
		if !strings.Contains(req.Text, "Berhenti berlangganan") && !strings.Contains(req.Text, "berhenti berlangganan") {
			t.Errorf("expected text body to contain standard unsubscribe note, got %q", req.Text)
		}
		if !strings.Contains(req.HTML, "Berhenti berlangganan") {
			t.Errorf("expected html body to contain unsubscribe link, got %q", req.HTML)
		}
		if !strings.Contains(req.IdempotencyKey, idempotencyKey) {
			t.Errorf("expected per-recipient idempotency key to contain %s, got %s", idempotencyKey, req.IdempotencyKey)
		}
	}
	mockResend.mu.Unlock()

	// 3. Test Idempotency Guard (re-sending same idempotency key)
	wDuplicate := call("POST", "/api/v1/contacts/broadcast/send", string(payloadBytes), true)
	if wDuplicate.Code != http.StatusOK {
		t.Fatalf("expected 200 for idempotent re-send, got %d: %s", wDuplicate.Code, wDuplicate.Body.String())
	}

	var dupResp BroadcastSendResponse
	if err := json.Unmarshal(wDuplicate.Body.Bytes(), &dupResp); err != nil {
		t.Fatalf("failed parsing duplicate send response: %v", err)
	}

	if !dupResp.AlreadySent {
		t.Errorf("expected already_sent true on duplicate send")
	}
	if dupResp.SentCount != 2 {
		t.Errorf("expected duplicate response to return cached sent count 2, got %d", dupResp.SentCount)
	}

	// Verify provider did NOT receive any new emails
	mockResend.mu.Lock()
	if len(mockResend.sentRequests) != 2 {
		t.Errorf("expected provider sent requests to remain 2 after duplicate send, got %d", len(mockResend.sentRequests))
	}
	mockResend.mu.Unlock()
}

func TestBroadcastSend_InFlightConflict(t *testing.T) {
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

	// Artificially acquire in-flight lock
	testKey := "inflight-test-key"
	inFlightBroadcasts.Store(testKey, struct{}{})
	defer inFlightBroadcasts.Delete(testKey)

	payload := fmt.Sprintf(`{"tags":[],"custom_emails":["user@test.id"],"subject":"Test","content":"Content","idempotency_key":%q}`, testKey)
	wConflict := call("POST", "/api/v1/contacts/broadcast/send", payload, true)

	if wConflict.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for concurrent in-flight send, got %d: %s", wConflict.Code, wConflict.Body.String())
	}
}

func TestBroadcastSend_Validation(t *testing.T) {
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

	// 1. Unauthenticated request must return 401
	wUnauth := call("POST", "/api/v1/contacts/broadcast/send", `{"subject":"A","content":"B"}`, false)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", wUnauth.Code)
	}

	// 2. Empty subject must return 400
	wEmptySub := call("POST", "/api/v1/contacts/broadcast/send", `{"subject":"","content":"Isi pesan","custom_emails":["u@example.com"]}`, true)
	if wEmptySub.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty subject, got %d", wEmptySub.Code)
	}

	// 3. Subject with CRLF injection must return 400
	wCRLF := call("POST", "/api/v1/contacts/broadcast/send", `{"subject":"Sub\r\nBcc: evil@test.com","content":"Isi","custom_emails":["u@example.com"]}`, true)
	if wCRLF.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for subject with newline, got %d", wCRLF.Code)
	}

	// 4. Empty content must return 400
	wEmptyContent := call("POST", "/api/v1/contacts/broadcast/send", `{"subject":"Subjek","content":"","custom_emails":["u@example.com"]}`, true)
	if wEmptyContent.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty content, got %d", wEmptyContent.Code)
	}

	// 5. Zero matching audience must return 400
	wZeroRec := call("POST", "/api/v1/contacts/broadcast/send", `{"subject":"Subjek","content":"Isi","tags":["non-existent-tag"]}`, true)
	if wZeroRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 0 recipients, got %d: %s", wZeroRec.Code, wZeroRec.Body.String())
	}
}

func TestBroadcastUnsubscribe_RFC8058AndWeb(t *testing.T) {
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

	// 1. Create contact with tag "newsletter"
	cEmail := "subscriber@domain.com"
	c, err := s.crmUpsertWithSession(t.Context(), "guest-sub", "Shop", "Subscriber User", cEmail, "session-sub")
	if err != nil {
		t.Fatalf("failed creating contact: %v", err)
	}
	call("PATCH", "/api/v1/contacts/"+c.ID, `{"tags":["newsletter"]}`, true)

	// Verify audience initially has 1 recipient
	wAudienceBefore := call("POST", "/api/v1/contacts/broadcast/audience", `{"tags":["newsletter"]}`, true)
	var audBefore BroadcastAudienceResponse
	json.Unmarshal(wAudienceBefore.Body.Bytes(), &audBefore)
	if audBefore.TotalUnique != 1 {
		t.Fatalf("expected 1 recipient before unsubscribe, got %d", audBefore.TotalUnique)
	}

	// 2. Generate valid unsubscribe token
	validToken := s.generateUnsubscribeToken(cEmail)

	// 3. Test GET unsubscribe link (browser click)
	wUnsubGET := call("GET", "/api/v1/contacts/broadcast/unsubscribe?token="+validToken, "", false)
	if wUnsubGET.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET unsubscribe, got %d: %s", wUnsubGET.Code, wUnsubGET.Body.String())
	}
	bodyHTML := wUnsubGET.Body.String()
	if !strings.Contains(bodyHTML, "Berhenti Berlangganan") {
		t.Errorf("expected confirmation page to contain 'Berhenti Berlangganan'")
	}
	if !strings.Contains(bodyHTML, cEmail) {
		t.Errorf("expected confirmation page to display unsubscribed email %s", cEmail)
	}

	// 4. Verify audience resolution now EXCLUDES the unsubscribed recipient
	wAudienceAfter := call("POST", "/api/v1/contacts/broadcast/audience", `{"tags":["newsletter"]}`, true)
	var audAfter BroadcastAudienceResponse
	json.Unmarshal(wAudienceAfter.Body.Bytes(), &audAfter)
	if audAfter.TotalUnique != 0 {
		t.Fatalf("expected 0 recipients after unsubscribe, got %d", audAfter.TotalUnique)
	}

	// 5. Test RFC 8058 One-Click POST unsubscribe
	cEmail2 := "another@domain.com"
	token2 := s.generateUnsubscribeToken(cEmail2)
	wUnsubPOST := call("POST", "/api/v1/contacts/broadcast/unsubscribe?token="+token2, "List-Unsubscribe=One-Click", false)
	if wUnsubPOST.Code != http.StatusOK {
		t.Fatalf("expected 200 for RFC 8058 POST unsubscribe, got %d: %s", wUnsubPOST.Code, wUnsubPOST.Body.String())
	}
	var postResp struct {
		Status       string `json:"status"`
		Unsubscribed bool   `json:"unsubscribed"`
		Email        string `json:"email"`
	}
	if err := json.Unmarshal(wUnsubPOST.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("failed parsing POST unsubscribe response: %v", err)
	}
	if !postResp.Unsubscribed || postResp.Email != cEmail2 {
		t.Errorf("unexpected POST unsubscribe response: %+v", postResp)
	}

	// 6. Test invalid / tampered token
	wInvalid := call("GET", "/api/v1/contacts/broadcast/unsubscribe?token=malformed-invalid-token", "", false)
	if wInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d", wInvalid.Code)
	}
}
