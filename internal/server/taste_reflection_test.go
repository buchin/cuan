package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/auth"
)

func TestTaste_Reflection_AuthenticationRequired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	endpoints := []struct {
		method string
		url    string
		body   string
	}{
		{http.MethodPost, "/api/v1/taste/reflect", `{"text":"Gunakan bahasa santai"}`},
		{http.MethodGet, "/api/v1/taste/candidates", ""},
		{http.MethodPost, "/api/v1/taste/candidates/cand-123/accept", ""},
		{http.MethodPost, "/api/v1/taste/candidates/cand-123/dismiss", ""},
		{http.MethodDelete, "/api/v1/taste/candidates/cand-123", ""},
	}

	for _, ep := range endpoints {
		var req *http.Request
		if ep.body != "" {
			req = httptest.NewRequest(ep.method, ep.url, strings.NewReader(ep.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(ep.method, ep.url, nil)
		}

		rec := httptest.NewRecorder()
		srv.Router().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("[%s %s] expected 401 Unauthorized for unauthenticated request, got %d", ep.method, ep.url, rec.Code)
		}
	}
}

func TestTaste_Reflection_Extract_BrandAndTechnical(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Post chat context with both Brand and Technical preference statements
	reflectPayload := `{
		"text": "Pengguna: Tolong mulai sekarang gunakan bahasa yang santai dan bersahabat untuk audiens muda.\nKoreksi: Untuk backend selalu gunakan Go dan sertakan targeted unit test."
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /taste/reflect, got %d: %s", rec.Code, rec.Body.String())
	}

	var res ReflectTasteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode reflect response: %v", err)
	}

	if res.Count < 2 {
		t.Fatalf("expected at least 2 extracted candidates, got %d", res.Count)
	}

	foundBrand := false
	foundTech := false

	for _, c := range res.Candidates {
		if c.ID == "" {
			t.Errorf("candidate ID should not be empty")
		}
		if c.Status != string(TasteCandidateStatusSuggested) && c.Status != string(TasteCandidateStatusAccepted) {
			t.Errorf("expected candidate status 'suggested' or 'accepted', got '%s'", c.Status)
		}
		if c.SourceSnippet == "" {
			t.Errorf("expected source_snippet to be non-empty")
		}
		if c.Reason == "" {
			t.Errorf("expected reason to be non-empty")
		}
		if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
			t.Errorf("expected created_at and updated_at timestamps to be non-zero")
		}

		if c.Category == string(TasteCategoryBrand) {
			foundBrand = true
			if !strings.Contains(strings.ToLower(c.Text), "santai") {
				t.Errorf("expected brand candidate text to contain 'santai', got: %s", c.Text)
			}
			if c.Topic == "" {
				t.Errorf("expected non-empty topic for brand candidate")
			}
		} else if c.Category == string(TasteCategoryTechnical) {
			foundTech = true
			if !strings.Contains(strings.ToLower(c.Text), "go") && !strings.Contains(strings.ToLower(c.Text), "backend") {
				t.Errorf("expected tech candidate text to contain 'go' or 'backend', got: %s", c.Text)
			}
			if c.Topic == "" {
				t.Errorf("expected non-empty topic for technical candidate")
			}
		}
	}

	if !foundBrand {
		t.Errorf("expected to extract at least one Brand candidate")
	}
	if !foundTech {
		t.Errorf("expected to extract at least one Technical candidate")
	}
}

func TestTaste_Reflection_DuplicatePrevention(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	reflectPayload := `{
		"text": "Tolong mulai sekarang gunakan nada bicara yang santai dan bersahabat untuk audiens muda.",
		"auto_approve": false
	}`

	// 1. First reflection run: candidate is created
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first reflect failed with %d: %s", rec.Code, rec.Body.String())
	}

	var res1 ReflectTasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res1)
	if res1.Count != 1 {
		t.Fatalf("expected 1 candidate extracted on first run, got %d", res1.Count)
	}

	// 2. Second reflection run with the exact same text: must NOT create duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second reflect failed with %d: %s", rec2.Code, rec2.Body.String())
	}

	var res2 ReflectTasteResponse
	_ = json.Unmarshal(rec2.Body.Bytes(), &res2)
	if res2.Count != 0 {
		t.Fatalf("expected 0 new candidates on duplicate run, got %d: %+v", res2.Count, res2.Candidates)
	}

	// 3. Verify total candidates list has exactly 1 candidate
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	recList := httptest.NewRecorder()
	srv.Router().ServeHTTP(recList, reqList)

	var listRes struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &listRes)
	if listRes.Total != 1 {
		t.Fatalf("expected exactly 1 candidate in store, got %d", listRes.Total)
	}

	// 4. Try reflecting an active default taste item: should also be rejected as duplicate
	activeDuplicatePayload := `{
		"text": "Gunakan bahasa Indonesia yang profesional, ramah, lugas, dan mudah dipahami."
	}`
	reqActive := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(activeDuplicatePayload))
	reqActive.Header.Set("Authorization", "Bearer "+token)
	reqActive.Header.Set("Content-Type", "application/json")
	recActive := httptest.NewRecorder()
	srv.Router().ServeHTTP(recActive, reqActive)

	var resActive ReflectTasteResponse
	_ = json.Unmarshal(recActive.Body.Bytes(), &resActive)
	if resActive.Count != 0 {
		t.Fatalf("expected 0 new candidates when text matches existing active taste item, got %d", resActive.Count)
	}
}

func TestTaste_Candidates_ListAndFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Extract both brand and tech candidates
	payload := `{
		"text": "Tolong mulai sekarang gunakan bahasa yang santai dan bersahabat untuk audiens muda.\nKoreksi: Untuk backend selalu gunakan Go dan sertakan targeted unit test.",
		"auto_approve": false
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	// 1. Default GET /candidates returns all pending
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	recGet := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("GET /candidates failed with %d: %s", recGet.Code, recGet.Body.String())
	}

	var allResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recGet.Body.Bytes(), &allResp)
	if allResp.Total < 2 {
		t.Fatalf("expected at least 2 pending candidates, got %d", allResp.Total)
	}

	// 2. Filter by category=brand
	reqBrand := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates?category=brand", nil)
	reqBrand.Header.Set("Authorization", "Bearer "+token)
	recBrand := httptest.NewRecorder()
	srv.Router().ServeHTTP(recBrand, reqBrand)

	var brandResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recBrand.Body.Bytes(), &brandResp)
	if brandResp.Total == 0 {
		t.Fatalf("expected at least 1 brand candidate")
	}
	for _, c := range brandResp.Candidates {
		if c.Category != string(TasteCategoryBrand) {
			t.Errorf("expected category 'brand', got %s", c.Category)
		}
	}

	// 3. Filter by category=technical
	reqTech := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates?category=technical", nil)
	reqTech.Header.Set("Authorization", "Bearer "+token)
	recTech := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTech, reqTech)

	var techResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recTech.Body.Bytes(), &techResp)
	if techResp.Total == 0 {
		t.Fatalf("expected at least 1 technical candidate")
	}
	for _, c := range techResp.Candidates {
		if c.Category != string(TasteCategoryTechnical) {
			t.Errorf("expected category 'technical', got %s", c.Category)
		}
	}

	// 4. Filter by status=accepted: should be 0 before any are accepted
	reqAccepted := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates?status=accepted", nil)
	reqAccepted.Header.Set("Authorization", "Bearer "+token)
	recAccepted := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAccepted, reqAccepted)

	var accResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recAccepted.Body.Bytes(), &accResp)
	if accResp.Total != 0 {
		t.Errorf("expected 0 accepted candidates before acceptance, got %d", accResp.Total)
	}
}

func TestTaste_Candidate_Accept_And_CapacityLimit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// 1. Extract a brand candidate
	payload := `{"text":"Tolong mulai sekarang gunakan gaya bahasa yang santai dan bersahabat untuk audiens muda."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var refResp ReflectTasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &refResp)
	if len(refResp.Candidates) == 0 {
		t.Fatalf("expected candidate to be extracted")
	}
	candidateID := refResp.Candidates[0].ID

	// 2. Accept the candidate via POST /api/v1/taste/candidates/:id/accept
	reqAccept := httptest.NewRequest(http.MethodPost, "/api/v1/taste/candidates/"+candidateID+"/accept", nil)
	reqAccept.Header.Set("Authorization", "Bearer "+token)
	recAccept := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAccept, reqAccept)

	if recAccept.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on accept, got %d: %s", recAccept.Code, recAccept.Body.String())
	}

	var acceptResult struct {
		Message   string         `json:"message"`
		Candidate TasteCandidate `json:"candidate"`
		Item      TasteItem      `json:"item"`
		ID        string         `json:"id"`
	}
	if err := json.Unmarshal(recAccept.Body.Bytes(), &acceptResult); err != nil {
		t.Fatalf("failed to parse accept response: %v", err)
	}

	if acceptResult.Candidate.Status != string(TasteCandidateStatusAccepted) {
		t.Errorf("expected candidate status 'accepted', got %s", acceptResult.Candidate.Status)
	}
	if acceptResult.Item.ID == "" || acceptResult.Item.Category != string(TasteCategoryBrand) {
		t.Errorf("invalid promoted taste item: %+v", acceptResult.Item)
	}

	// 3. Verify that the promoted item is now in the active TasteProfile
	reqProfile := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	recProfile := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProfile, reqProfile)

	var profile TasteProfile
	_ = json.Unmarshal(recProfile.Body.Bytes(), &profile)
	foundInProfile := false
	for _, it := range profile.BrandItems {
		if it.ID == acceptResult.Item.ID {
			foundInProfile = true
			break
		}
	}
	if !foundInProfile {
		t.Errorf("promoted item %s was not found in active profile brand items", acceptResult.Item.ID)
	}

	// 4. Verify candidate is no longer in pending list
	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqPending.Header.Set("Authorization", "Bearer "+token)
	recPending := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPending, reqPending)

	var pendResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recPending.Body.Bytes(), &pendResp)
	for _, c := range pendResp.Candidates {
		if c.ID == candidateID {
			t.Errorf("accepted candidate should not be returned in default pending list")
		}
	}

	// 5. Test capacity limit: fill Brand category to MaxTasteItemsPerCategory (15)
	brandCount := len(profile.BrandItems)
	for i := brandCount; i < MaxTasteItemsPerCategory; i++ {
		addPayload := fmt.Sprintf(`{"category":"brand","content":"Brand rule %d"}`, i)
		r := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(addPayload))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		srv.Router().ServeHTTP(httptest.NewRecorder(), r)
	}

	// Now extract another brand candidate
	candPayload := `{"text":"Tolong selalu gunakan sapaan ramah kepada pelanggan baru."}`
	rRef := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(candPayload))
	rRef.Header.Set("Authorization", "Bearer "+token)
	rRef.Header.Set("Content-Type", "application/json")
	wRef := httptest.NewRecorder()
	srv.Router().ServeHTTP(wRef, rRef)

	var refResp2 ReflectTasteResponse
	_ = json.Unmarshal(wRef.Body.Bytes(), &refResp2)
	if len(refResp2.Candidates) == 0 {
		t.Fatalf("expected second candidate to be extracted")
	}

	// Try to accept when category is full: must return 400 Bad Request
	rFullAccept := httptest.NewRequest(http.MethodPost, "/api/v1/taste/candidates/"+refResp2.Candidates[0].ID+"/accept", nil)
	rFullAccept.Header.Set("Authorization", "Bearer "+token)
	wFullAccept := httptest.NewRecorder()
	srv.Router().ServeHTTP(wFullAccept, rFullAccept)

	if wFullAccept.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when category capacity is exceeded, got %d: %s", wFullAccept.Code, wFullAccept.Body.String())
	}
	if !strings.Contains(wFullAccept.Body.String(), "Kapasitas maksimum") {
		t.Errorf("expected error message to mention capacity limit, got: %s", wFullAccept.Body.String())
	}
}

func TestTaste_Candidate_Dismiss(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// 1. Extract a candidate
	payload := `{"text":"Tolong mulai sekarang gunakan kata gaul untuk semua respons."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var refResp ReflectTasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &refResp)
	if len(refResp.Candidates) == 0 {
		t.Fatalf("expected candidate to be extracted")
	}
	candidateID := refResp.Candidates[0].ID

	// 2. Dismiss candidate via POST /api/v1/taste/candidates/:id/dismiss
	reqDismiss := httptest.NewRequest(http.MethodPost, "/api/v1/taste/candidates/"+candidateID+"/dismiss", nil)
	reqDismiss.Header.Set("Authorization", "Bearer "+token)
	recDismiss := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDismiss, reqDismiss)

	if recDismiss.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on dismiss, got %d: %s", recDismiss.Code, recDismiss.Body.String())
	}

	var dismissResult struct {
		Message   string         `json:"message"`
		Candidate TasteCandidate `json:"candidate"`
		ID        string         `json:"id"`
	}
	_ = json.Unmarshal(recDismiss.Body.Bytes(), &dismissResult)
	if dismissResult.Candidate.Status != string(TasteCandidateStatusDismissed) {
		t.Errorf("expected candidate status 'dismissed', got %s", dismissResult.Candidate.Status)
	}

	// 3. Verify dismissed candidate is NOT in default pending list
	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqPending.Header.Set("Authorization", "Bearer "+token)
	recPending := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPending, reqPending)

	var pendResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recPending.Body.Bytes(), &pendResp)
	for _, c := range pendResp.Candidates {
		if c.ID == candidateID {
			t.Errorf("dismissed candidate should not appear in default pending list")
		}
	}

	// 4. Verify candidate is in status=dismissed list
	reqDismissed := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates?status=dismissed", nil)
	reqDismissed.Header.Set("Authorization", "Bearer "+token)
	recDismissed := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDismissed, reqDismissed)

	var disResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recDismissed.Body.Bytes(), &disResp)
	foundDismissed := false
	for _, c := range disResp.Candidates {
		if c.ID == candidateID {
			foundDismissed = true
			break
		}
	}
	if !foundDismissed {
		t.Errorf("dismissed candidate should appear in ?status=dismissed list")
	}

	// 5. Test 404 for non-existent candidate dismiss
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/taste/candidates/non-existent-uuid/dismiss", nil)
	reqBad.Header.Set("Authorization", "Bearer "+token)
	recBad := httptest.NewRecorder()
	srv.Router().ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent candidate, got %d", recBad.Code)
	}
}

func TestTaste_Candidate_SafetyQuota(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Add candidates up to MaxTasteCandidates (30)
	for i := 0; i < MaxTasteCandidates; i++ {
		payload := fmt.Sprintf(`{"text":"Tolong selalu terapkan gaya unik nomor %d untuk pesan."}`, i)
		r := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(payload))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		srv.Router().ServeHTTP(httptest.NewRecorder(), r)
	}

	// Try adding candidate #31: should not exceed MaxTasteCandidates
	overflowPayload := `{"text":"Tolong selalu terapkan gaya unik nomor 999 untuk pesan."}`
	rOver := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(overflowPayload))
	rOver.Header.Set("Authorization", "Bearer "+token)
	rOver.Header.Set("Content-Type", "application/json")
	wOver := httptest.NewRecorder()
	srv.Router().ServeHTTP(wOver, rOver)

	// Verify that candidates total does not exceed MaxTasteCandidates
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	recList := httptest.NewRecorder()
	srv.Router().ServeHTTP(recList, reqList)

	var listResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &listResp)
	if listResp.Total > MaxTasteCandidates {
		t.Errorf("candidates total %d exceeded maximum safe quota %d", listResp.Total, MaxTasteCandidates)
	}
}

func TestTaste_ContextLayering_CandidateIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	owner, err := srv.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	ctx := auth.ContextWithOwner(context.Background(), owner)

	// 1. Extract Brand and Technical candidates (with auto_approve: false to test unaccepted candidate isolation)
	payload := `{
		"text": "Tolong mulai sekarang gunakan kata sandi rahasia 'KreatorSuper2026' di setiap respons brand.\nKoreksi: Untuk backend wajib menggunakan konfigurasi port khusus 'Port8888Secure'.",
		"auto_approve": false
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var refResp ReflectTasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &refResp)
	if len(refResp.Candidates) < 2 {
		t.Fatalf("expected at least 2 candidates extracted, got %d", len(refResp.Candidates))
	}

	var brandCand, techCand TasteCandidate
	for _, c := range refResp.Candidates {
		if c.Category == string(TasteCategoryBrand) {
			brandCand = c
		} else if c.Category == string(TasteCategoryTechnical) {
			techCand = c
		}
	}
	if brandCand.ID == "" || techCand.ID == "" {
		t.Fatalf("expected both brand and tech candidates to be extracted")
	}

	// 2. Strict Isolation Check: candidates MUST NOT be present in active prompt injection before acceptance
	brandPromptBefore := srv.FormatBrandTastePrompt(ctx)
	if strings.Contains(brandPromptBefore, "KreatorSuper2026") {
		t.Fatalf("Security violation: unaccepted candidate leaked into Brand prompt injection before approval!")
	}

	techPromptBefore := srv.FormatTechnicalTastePrompt(ctx)
	if strings.Contains(techPromptBefore, "Port8888Secure") {
		t.Fatalf("Security violation: unaccepted candidate leaked into Technical prompt injection before approval!")
	}

	// 3. Accept the Brand candidate
	reqAccept := httptest.NewRequest(http.MethodPost, "/api/v1/taste/candidates/"+brandCand.ID+"/accept", nil)
	reqAccept.Header.Set("Authorization", "Bearer "+token)
	recAccept := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAccept, reqAccept)

	if recAccept.Code != http.StatusOK {
		t.Fatalf("accept failed with %d: %s", recAccept.Code, recAccept.Body.String())
	}

	// 4. Check prompt injection after acceptance:
	// Brand prompt MUST now contain the approved taste item
	brandPromptAfter := srv.FormatBrandTastePrompt(ctx)
	if !strings.Contains(brandPromptAfter, "KreatorSuper2026") {
		t.Errorf("expected approved brand taste item to appear in Brand prompt, got:\n%s", brandPromptAfter)
	}

	// Technical prompt MUST STILL NOT contain the unaccepted technical candidate
	techPromptAfter := srv.FormatTechnicalTastePrompt(ctx)
	if strings.Contains(techPromptAfter, "Port8888Secure") {
		t.Errorf("unaccepted technical candidate leaked into Technical prompt!")
	}
}
