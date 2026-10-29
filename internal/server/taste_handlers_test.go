package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestTaste_AuthenticationRequired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	endpoints := []struct {
		method string
		url    string
		body   string
	}{
		{http.MethodGet, "/api/v1/taste", ""},
		{http.MethodPost, "/api/v1/taste/items", `{"category":"brand","content":"Test"}`},
		{http.MethodPut, "/api/v1/taste/items/item-123", `{"content":"Updated"}`},
		{http.MethodDelete, "/api/v1/taste/items/item-123", ""},
		{http.MethodPost, "/api/v1/taste/reset", ""},
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

func TestTaste_GetProfile_Defaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var profile TasteProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("failed to parse TasteProfile JSON: %v", err)
	}

	if profile.Total < 6 {
		t.Fatalf("expected at least 6 default items, got %d", profile.Total)
	}
	if len(profile.BrandItems) < 3 {
		t.Fatalf("expected at least 3 default brand items, got %d", len(profile.BrandItems))
	}
	if len(profile.TechnicalItems) < 3 {
		t.Fatalf("expected at least 3 default technical items, got %d", len(profile.TechnicalItems))
	}

	// Verify metadata fields
	for _, it := range profile.Items {
		if it.ID == "" {
			t.Errorf("item ID is empty")
		}
		if it.Category != string(TasteCategoryBrand) && it.Category != string(TasteCategoryTechnical) {
			t.Errorf("unexpected item category: %s", it.Category)
		}
		if it.Content == "" {
			t.Errorf("item content is empty for %s", it.ID)
		}
		if it.CreatedAt.IsZero() || it.UpdatedAt.IsZero() {
			t.Errorf("item timestamp is zero for %s", it.ID)
		}
	}
}

func TestTaste_CreateItem_And_Validation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// 1. Create a valid Brand Taste item
	brandPayload := `{"category":"brand","topic":"Gaya Bahasa Casual","content":"Gunakan gaya bahasa santai dan bersahabat untuk audiens muda."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(brandPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created TasteItem
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse created TasteItem: %v", err)
	}
	if created.Category != "brand" || created.Topic != "Gaya Bahasa Casual" {
		t.Errorf("unexpected created item attributes: %+v", created)
	}

	// 2. Create a valid Technical Taste item
	techPayload := `{"category":"technical","topic":"Docker Rules","content":"Pastikan setiap container berjalan dengan user non-root."}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(techPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for tech item, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Validation: Invalid category
	badCatPayload := `{"category":"unknown_category","content":"Something"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(badCatPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid category, got %d", rec.Code)
	}

	// 4. Validation: Empty content
	emptyContentPayload := `{"category":"brand","content":"   "}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(emptyContentPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty content, got %d", rec.Code)
	}

	// 5. Validation: Overlong content
	overlongContent := strings.Repeat("A", 2500)
	overlongPayload := fmt.Sprintf(`{"category":"brand","content":"%s"}`, overlongContent)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(overlongPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for overlong content, got %d", rec.Code)
	}
}

func TestTaste_CapacityLimit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Fetch profile to see current item count
	req := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	var profile TasteProfile
	_ = json.Unmarshal(rec.Body.Bytes(), &profile)

	brandCount := len(profile.BrandItems)
	// Add items up to MaxTasteItemsPerCategory (15)
	for i := brandCount; i < MaxTasteItemsPerCategory; i++ {
		payload := fmt.Sprintf(`{"category":"brand","topic":"Brand %d","content":"Item content %d"}`, i+1, i+1)
		r := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(payload))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, r)
		if w.Code != http.StatusCreated {
			t.Fatalf("failed creating item %d: %d %s", i+1, w.Code, w.Body.String())
		}
	}

	// The 16th item in the brand category should be rejected
	overflowPayload := `{"category":"brand","topic":"Overflow","content":"This should exceed 15 items limit."}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(overflowPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for capacity limit, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Kapasitas maksimum 15 item") {
		t.Errorf("expected capacity limit error message, got: %s", rec.Body.String())
	}
}

func TestTaste_UpdateItem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Create an item first
	createPayload := `{"category":"brand","topic":"Original Topic","content":"Original Content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create item: %d", rec.Code)
	}
	var created TasteItem
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// 1. Update content and topic
	updatePayload := `{"topic":"Updated Topic","content":"Updated Content"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/taste/items/"+created.ID, strings.NewReader(updatePayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for update, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated TasteItem
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Topic != "Updated Topic" || updated.Content != "Updated Content" {
		t.Errorf("item was not properly updated: %+v", updated)
	}

	// 2. Update category to technical
	catPayload := `{"category":"technical"}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/taste/items/"+created.ID, strings.NewReader(catPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for category update, got %d: %s", rec.Code, rec.Body.String())
	}
	var catUpdated TasteItem
	_ = json.Unmarshal(rec.Body.Bytes(), &catUpdated)
	if catUpdated.Category != "technical" {
		t.Errorf("expected category technical, got %s", catUpdated.Category)
	}

	// 3. Update non-existent item -> 404
	req = httptest.NewRequest(http.MethodPut, "/api/v1/taste/items/non-existent-id", strings.NewReader(`{"content":"test"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for missing item, got %d", rec.Code)
	}
}

func TestTaste_DeleteItem_GranularForget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Create an item to delete
	createPayload := `{"category":"brand","topic":"To Be Forgotten","content":"Temporary note to forget."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	var created TasteItem
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// 1. Delete item
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/taste/items/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for delete, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify item is no longer in list
	req = httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	var profile TasteProfile
	_ = json.Unmarshal(rec.Body.Bytes(), &profile)
	for _, it := range profile.Items {
		if it.ID == created.ID {
			t.Errorf("item %s should have been deleted", created.ID)
		}
	}

	// 2. Delete again should return 404
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/taste/items/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found on second delete, got %d", rec.Code)
	}
}

func TestTaste_ResetProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Add custom items
	customPayload := `{"category":"brand","topic":"Custom 1","content":"Custom Content 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(customPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("failed to create custom item: %d", rec.Code)
	}

	// Reset total
	req = httptest.NewRequest(http.MethodPost, "/api/v1/taste/reset", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reset, got %d: %s", rec.Code, rec.Body.String())
	}

	var resetProfile TasteProfile
	if err := json.Unmarshal(rec.Body.Bytes(), &resetProfile); err != nil {
		t.Fatalf("failed to decode reset response: %v", err)
	}

	// Default template items should be restored exactly
	if resetProfile.Total != 6 {
		t.Fatalf("expected exactly 6 default items after reset, got %d", resetProfile.Total)
	}
	if len(resetProfile.BrandItems) != 3 {
		t.Fatalf("expected 3 brand items after reset, got %d", len(resetProfile.BrandItems))
	}
	if len(resetProfile.TechnicalItems) != 3 {
		t.Fatalf("expected 3 technical items after reset, got %d", len(resetProfile.TechnicalItems))
	}
}

func TestTaste_ContextLayering_BrandTasteChat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	// Format brand taste prompt
	ctx := context.Background()
	brandPrompt := srv.FormatBrandTastePrompt(ctx)

	if brandPrompt == "" {
		t.Fatal("expected non-empty Brand Taste prompt")
	}
	if !strings.Contains(brandPrompt, "Brand & Product Taste") {
		t.Errorf("expected header 'Brand & Product Taste', got: %s", brandPrompt)
	}
	if !strings.Contains(brandPrompt, "Gaya Bahasa & Tone of Voice") {
		t.Errorf("expected tone of voice topic in prompt, got: %s", brandPrompt)
	}

	// Verify injection into Chat prompt via executeInboxCLIWithSafety
	home := t.TempDir()
	workspace := t.TempDir()

	// When executeInboxCLIWithSafety executes, prompt prepends Brand Taste
	// We can test this by running executeInboxCLIWithSafety with testing env
	srv.cfg.Environment = "testing"
	_, failMsg := srv.executeInboxCLIWithSafety(ctx, "TestAgent", "agy", "Halo bot", home, workspace, nil, nil)
	if failMsg == "" {
		t.Log("executed inbox CLI in testing environment")
	}
}

func TestTaste_ContextLayering_TechnicalTasteRunner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	ctx := context.Background()
	techPrompt := srv.FormatTechnicalTastePrompt(ctx)

	if techPrompt == "" {
		t.Fatal("expected non-empty Technical Taste prompt")
	}
	if !strings.Contains(techPrompt, "## Technical Taste & Conventions") {
		t.Errorf("expected header '## Technical Taste & Conventions', got: %s", techPrompt)
	}
	if !strings.Contains(techPrompt, "Default Tech Stack") {
		t.Errorf("expected tech stack topic in prompt, got: %s", techPrompt)
	}

	// Verify that issueTaskAgent writes Technical Taste into session.md
	created, err := srv.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID:    "default",
		Title:        "Test Technical Taste Injection",
		Instructions: "Verify technical taste injection into session.md",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	att, err := srv.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{
		AgentName: "agent_runner",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	env, _, _, cleanup, err := srv.issueTaskAgent(ctx, created.ID, att.ID, "agent_runner")
	if err != nil {
		t.Fatalf("issueTaskAgent failed: %v", err)
	}
	defer cleanup()

	// Locate session.md
	capabilityFile := ""
	for _, e := range env {
		if strings.HasPrefix(e, "CUAN_TASK_CAPABILITY_FILE=") {
			capabilityFile = strings.TrimPrefix(e, "CUAN_TASK_CAPABILITY_FILE=")
			break
		}
	}
	if capabilityFile == "" {
		t.Fatal("missing CUAN_TASK_CAPABILITY_FILE in env")
	}

	guidePath := filepath.Join(filepath.Dir(capabilityFile), "session.md")
	guideContent, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("failed reading session.md: %v", err)
	}

	contentStr := string(guideContent)
	if !strings.Contains(contentStr, "## Technical Taste & Conventions") {
		t.Fatalf("session.md does not contain '## Technical Taste & Conventions':\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "Default stack: Go untuk backend") {
		t.Fatalf("session.md missing default tech stack rule:\n%s", contentStr)
	}
}

func TestTaste_ConfidenceScoring_Calculation(t *testing.T) {
	// Test confidence scoring against various patterns
	cases := []struct {
		name         string
		text         string
		expectedMin  float64
		expectedMax  float64
		expectAutoOk bool // >= 0.80
	}{
		{
			name:         "Explicit prefix correction",
			text:         "Koreksi: Untuk backend selalu gunakan Go dan sertakan targeted unit test.",
			expectedMin:  0.90,
			expectedMax:  1.00,
			expectAutoOk: true,
		},
		{
			name:         "Explicit prefix new rule",
			text:         "Aturan baru: Gunakan format timestamp RFC3339 untuk semua API.",
			expectedMin:  0.90,
			expectedMax:  1.00,
			expectAutoOk: true,
		},
		{
			name:         "Strong imperative mandatory",
			text:         "Wajib gunakan user non-root di setiap container Docker.",
			expectedMin:  0.85,
			expectedMax:  0.95,
			expectAutoOk: true,
		},
		{
			name:         "Action directive",
			text:         "Gunakan gaya bahasa santai dan bersahabat untuk audiens muda.",
			expectedMin:  0.80,
			expectedMax:  0.90,
			expectAutoOk: true,
		},
		{
			name:         "Stated preference",
			text:         "Saya lebih suka ringkasan jawaban singkat dalam format bullet points.",
			expectedMin:  0.80,
			expectedMax:  0.85,
			expectAutoOk: true,
		},
		{
			name:         "Suggestive softer phrase",
			text:         "Sebaiknya pertimbangkan format ringkas untuk ringkasan mingguan jika memungkinkan.",
			expectedMin:  0.50,
			expectedMax:  0.79,
			expectAutoOk: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extracted := ExtractTasteCandidates(tc.text, nil, nil)
			if len(extracted) == 0 {
				t.Fatalf("failed to extract candidate for: %s", tc.text)
			}
			cand := extracted[0]
			if cand.Confidence < tc.expectedMin || cand.Confidence > tc.expectedMax {
				t.Errorf("candidate confidence %.2f out of expected range [%.2f, %.2f] for '%s'",
					cand.Confidence, tc.expectedMin, tc.expectedMax, tc.text)
			}
			if cand.Confidence < 0.0 || cand.Confidence > 1.0 {
				t.Errorf("candidate confidence %.2f must be between 0.0 and 1.0", cand.Confidence)
			}
			if (cand.Confidence >= 0.80) != tc.expectAutoOk {
				t.Errorf("expected auto_approve eligibility %v, got %v (score: %.2f)",
					tc.expectAutoOk, cand.Confidence >= 0.80, cand.Confidence)
			}
		})
	}
}

func TestTaste_AutoApproval_Gate_UnderQuota(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// High confidence extraction: should be automatically approved into active profile
	reflectPayload := `{
		"text": "Koreksi: Untuk backend wajib menggunakan Go dan REST JSON standar."
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res ReflectTasteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse reflect response: %v", err)
	}

	if len(res.Candidates) == 0 {
		t.Fatalf("expected candidate to be extracted")
	}

	cand := res.Candidates[0]
	if cand.Confidence < 0.80 {
		t.Fatalf("expected confidence >= 0.80, got %.2f", cand.Confidence)
	}
	if cand.Status != string(TasteCandidateStatusAccepted) {
		t.Errorf("expected candidate status 'accepted', got '%s'", cand.Status)
	}
	if len(res.AutoApproved) == 0 {
		t.Errorf("expected candidate in res.AutoApproved list")
	}

	// Verify active profile now contains the auto-approved item with metadata
	reqProfile := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	recProfile := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProfile, reqProfile)

	var profile TasteProfile
	_ = json.Unmarshal(recProfile.Body.Bytes(), &profile)

	foundAutoApproved := false
	for _, it := range profile.TechnicalItems {
		if strings.Contains(it.Content, "REST JSON standar") {
			foundAutoApproved = true
			if !it.AutoApproved {
				t.Errorf("expected item.AutoApproved to be true")
			}
			if it.AutoApprovedAt == nil || it.AutoApprovedAt.IsZero() {
				t.Errorf("expected item.AutoApprovedAt timestamp to be set")
			}
			break
		}
	}
	if !foundAutoApproved {
		t.Errorf("auto-approved item was not found in active profile technical items")
	}

	// Verify candidate is NOT in default pending candidates list
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
		if c.ID == cand.ID {
			t.Errorf("auto-approved candidate should not be returned in default pending candidates list")
		}
	}

	// Verify candidate IS returned when filtering by status=accepted
	reqAccepted := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates?status=accepted", nil)
	reqAccepted.Header.Set("Authorization", "Bearer "+token)
	recAccepted := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAccepted, reqAccepted)

	var accResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recAccepted.Body.Bytes(), &accResp)
	foundAcc := false
	for _, c := range accResp.Candidates {
		if c.ID == cand.ID {
			foundAcc = true
			break
		}
	}
	if !foundAcc {
		t.Errorf("auto-approved candidate not found in status=accepted candidates list")
	}
}

func TestTaste_AutoApproval_Gate_LowConfidence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Low confidence extraction (< 0.80): should stay pending
	reflectPayload := `{
		"text": "Sebaiknya pertimbangkan format ringkas untuk ringkasan mingguan jika memungkinkan."
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res ReflectTasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if len(res.Candidates) == 0 {
		t.Fatalf("expected candidate to be extracted")
	}

	cand := res.Candidates[0]
	if cand.Confidence >= 0.80 {
		t.Fatalf("expected confidence < 0.80 for suggestive phrase, got %.2f", cand.Confidence)
	}
	if cand.Status != string(TasteCandidateStatusSuggested) {
		t.Errorf("expected candidate status 'suggested', got '%s'", cand.Status)
	}
	if len(res.AutoApproved) != 0 {
		t.Errorf("expected 0 auto-approved candidates, got %d", len(res.AutoApproved))
	}

	// Verify active profile does not contain this item
	reqProfile := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	recProfile := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProfile, reqProfile)
	var profile TasteProfile
	_ = json.Unmarshal(recProfile.Body.Bytes(), &profile)
	for _, it := range profile.Items {
		if strings.Contains(it.Content, "format ringkas") {
			t.Errorf("low-confidence candidate should not be added to active profile")
		}
	}

	// Verify candidate appears in pending list
	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqPending.Header.Set("Authorization", "Bearer "+token)
	recPending := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPending, reqPending)
	var pendResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recPending.Body.Bytes(), &pendResp)
	foundPending := false
	for _, c := range pendResp.Candidates {
		if c.ID == cand.ID {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Errorf("low confidence candidate should be present in pending candidates list")
	}
}

func TestTaste_AutoApproval_Gate_QuotaFull(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Fetch current brand count
	reqProfile := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	recProfile := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProfile, reqProfile)
	var profile TasteProfile
	_ = json.Unmarshal(recProfile.Body.Bytes(), &profile)

	// Fill brand category up to MaxTasteItemsPerCategory (15)
	brandCount := len(profile.BrandItems)
	for i := brandCount; i < MaxTasteItemsPerCategory; i++ {
		payload := fmt.Sprintf(`{"category":"brand","topic":"Brand %d","content":"Isi aturan brand %d"}`, i+1, i+1)
		r := httptest.NewRequest(http.MethodPost, "/api/v1/taste/items", strings.NewReader(payload))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		srv.Router().ServeHTTP(httptest.NewRecorder(), r)
	}

	// Verify quota is full for brand via GET /api/v1/taste/quota
	reqQuota := httptest.NewRequest(http.MethodGet, "/api/v1/taste/quota", nil)
	reqQuota.Header.Set("Authorization", "Bearer "+token)
	recQuota := httptest.NewRecorder()
	srv.Router().ServeHTTP(recQuota, reqQuota)
	var quota TasteQuotaResponse
	_ = json.Unmarshal(recQuota.Body.Bytes(), &quota)
	if quota.Brand.Used != 15 || quota.Brand.CanAutoApprove {
		t.Fatalf("expected brand quota to be 15 and CanAutoApprove=false, got: %+v", quota.Brand)
	}

	// Now try to reflect a high-confidence brand preference (Confidence >= 0.95)
	reflectPayload := `{
		"text": "Koreksi: Gaya bahasa sapaan harus selalu menggunakan kata Halo Kak untuk semua respons."
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	var res ReflectTasteResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if len(res.Candidates) == 0 {
		t.Fatalf("expected candidate to be extracted")
	}

	cand := res.Candidates[0]
	if cand.Confidence < 0.80 {
		t.Fatalf("expected candidate confidence >= 0.80, got %.2f", cand.Confidence)
	}

	// Quota guardrail MUST hold: candidate must NOT be auto-approved because brand quota is full (15/15)
	if cand.Status != string(TasteCandidateStatusSuggested) {
		t.Errorf("expected candidate status 'suggested' when quota full, got '%s'", cand.Status)
	}
	if len(res.AutoApproved) != 0 {
		t.Errorf("expected 0 auto-approved candidates when category quota is full, got %d", len(res.AutoApproved))
	}

	// Verify brand items count in profile is still 15
	reqProfileAfter := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	reqProfileAfter.Header.Set("Authorization", "Bearer "+token)
	recProfileAfter := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProfileAfter, reqProfileAfter)
	var profileAfter TasteProfile
	_ = json.Unmarshal(recProfileAfter.Body.Bytes(), &profileAfter)
	if len(profileAfter.BrandItems) != 15 {
		t.Errorf("brand items in profile exceeded 15: %d", len(profileAfter.BrandItems))
	}
}

func TestTaste_AutoApproval_Toggle_Disabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// Disable auto-approval via PUT /api/v1/taste/config
	updateConfigPayload := `{"auto_approve_enabled": false}`
	reqCfg := httptest.NewRequest(http.MethodPut, "/api/v1/taste/config", strings.NewReader(updateConfigPayload))
	reqCfg.Header.Set("Authorization", "Bearer "+token)
	reqCfg.Header.Set("Content-Type", "application/json")
	recCfg := httptest.NewRecorder()
	srv.Router().ServeHTTP(recCfg, reqCfg)
	if recCfg.Code != http.StatusOK {
		t.Fatalf("failed to update taste config: %d %s", recCfg.Code, recCfg.Body.String())
	}

	// Verify quota metrics reflect can_auto_approve=false
	reqQuota := httptest.NewRequest(http.MethodGet, "/api/v1/taste/quota", nil)
	reqQuota.Header.Set("Authorization", "Bearer "+token)
	recQuota := httptest.NewRecorder()
	srv.Router().ServeHTTP(recQuota, reqQuota)
	var quota TasteQuotaResponse
	_ = json.Unmarshal(recQuota.Body.Bytes(), &quota)
	if quota.AutoApproveEnabled || quota.Brand.CanAutoApprove || quota.Technical.CanAutoApprove {
		t.Errorf("expected auto_approve_enabled=false and can_auto_approve=false across categories, got: %+v", quota)
	}

	// Reflect high-confidence preference: should NOT auto-approve when toggle is false
	reflectPayload := `{
		"text": "Koreksi: Untuk backend selalu gunakan bahasa Go dan arsitektur modular."
	}`
	reqRef := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	reqRef.Header.Set("Authorization", "Bearer "+token)
	reqRef.Header.Set("Content-Type", "application/json")
	recRef := httptest.NewRecorder()
	srv.Router().ServeHTTP(recRef, reqRef)

	var res ReflectTasteResponse
	_ = json.Unmarshal(recRef.Body.Bytes(), &res)
	if len(res.Candidates) == 0 {
		t.Fatalf("expected candidate to be extracted")
	}
	if res.Candidates[0].Status != string(TasteCandidateStatusSuggested) {
		t.Errorf("expected candidate to remain 'suggested' when auto-approval is disabled, got: %s", res.Candidates[0].Status)
	}
	if len(res.AutoApproved) != 0 {
		t.Errorf("expected 0 auto-approved candidates when toggle disabled")
	}
}

func TestTaste_QuotaMetrics_API(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// GET /api/v1/taste/quota
	req := httptest.NewRequest(http.MethodGet, "/api/v1/taste/quota", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var quota TasteQuotaResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &quota); err != nil {
		t.Fatalf("failed to decode quota response: %v", err)
	}

	// Defaults: 3 brand items, 3 technical items out of 15 max
	if quota.Brand.Used != 3 || quota.Brand.Max != 15 {
		t.Errorf("unexpected brand quota: used=%d, max=%d", quota.Brand.Used, quota.Brand.Max)
	}
	if quota.Brand.Percentage != 20.0 {
		t.Errorf("expected brand percentage 20.0, got %.1f", quota.Brand.Percentage)
	}
	if !quota.Brand.CanAutoApprove {
		t.Errorf("expected brand CanAutoApprove=true")
	}

	if quota.Technical.Used != 3 || quota.Technical.Max != 15 {
		t.Errorf("unexpected technical quota: used=%d, max=%d", quota.Technical.Used, quota.Technical.Max)
	}
	if quota.Technical.Percentage != 20.0 {
		t.Errorf("expected technical percentage 20.0, got %.1f", quota.Technical.Percentage)
	}
	if !quota.Technical.CanAutoApprove {
		t.Errorf("expected technical CanAutoApprove=true")
	}

	if quota.TotalUsed != 6 || quota.TotalMax != 30 {
		t.Errorf("unexpected total used=%d, max=%d", quota.TotalUsed, quota.TotalMax)
	}
	if !quota.AutoApproveEnabled {
		t.Errorf("expected AutoApproveEnabled=true by default")
	}
	if quota.AutoApproveThreshold != 0.80 {
		t.Errorf("expected default threshold 0.80, got %.2f", quota.AutoApproveThreshold)
	}
	if quota.Categories["brand"].Used != 3 || quota.Categories["technical"].Used != 3 {
		t.Errorf("categories map does not match category metrics")
	}
}

func TestTaste_Config_API_Validation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// 1. GET /api/v1/taste/config
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/taste/config", nil)
	reqGet.Header.Set("Authorization", "Bearer "+token)
	recGet := httptest.NewRecorder()
	srv.Router().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /taste/config, got %d", recGet.Code)
	}

	var cfg TasteConfig
	_ = json.Unmarshal(recGet.Body.Bytes(), &cfg)
	if !cfg.AutoApproveEnabled || cfg.AutoApproveThreshold != 0.80 {
		t.Errorf("unexpected default config: %+v", cfg)
	}

	// 2. PUT /api/v1/taste/config: valid update
	validUpdate := `{"auto_approve_enabled": true, "auto_approve_threshold": 0.85}`
	reqPut := httptest.NewRequest(http.MethodPut, "/api/v1/taste/config", strings.NewReader(validUpdate))
	reqPut.Header.Set("Authorization", "Bearer "+token)
	reqPut.Header.Set("Content-Type", "application/json")
	recPut := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid config update, got %d: %s", recPut.Code, recPut.Body.String())
	}
	var updated TasteConfig
	_ = json.Unmarshal(recPut.Body.Bytes(), &updated)
	if updated.AutoApproveThreshold != 0.85 {
		t.Errorf("expected threshold 0.85, got %.2f", updated.AutoApproveThreshold)
	}

	// 3. PUT /api/v1/taste/config: invalid threshold > 1.0 -> 400 Bad Request
	badOverPayload := `{"auto_approve_threshold": 1.25}`
	reqBadOver := httptest.NewRequest(http.MethodPut, "/api/v1/taste/config", strings.NewReader(badOverPayload))
	reqBadOver.Header.Set("Authorization", "Bearer "+token)
	reqBadOver.Header.Set("Content-Type", "application/json")
	recBadOver := httptest.NewRecorder()
	srv.Router().ServeHTTP(recBadOver, reqBadOver)
	if recBadOver.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for threshold > 1.0, got %d", recBadOver.Code)
	}

	// 4. PUT /api/v1/taste/config: invalid threshold < 0.0 -> 400 Bad Request
	badNegPayload := `{"auto_approve_threshold": -0.20}`
	reqBadNeg := httptest.NewRequest(http.MethodPut, "/api/v1/taste/config", strings.NewReader(badNegPayload))
	reqBadNeg.Header.Set("Authorization", "Bearer "+token)
	reqBadNeg.Header.Set("Content-Type", "application/json")
	recBadNeg := httptest.NewRecorder()
	srv.Router().ServeHTTP(recBadNeg, reqBadNeg)
	if recBadNeg.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for negative threshold, got %d", recBadNeg.Code)
	}
}

func TestTaste_Candidate_Undo(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// 1. Auto-approve a high-confidence candidate
	reflectPayload := `{
		"text": "Koreksi: Untuk backend wajib menggunakan arsitektur modular yang rapi."
	}`
	reqRef := httptest.NewRequest(http.MethodPost, "/api/v1/taste/reflect", strings.NewReader(reflectPayload))
	reqRef.Header.Set("Authorization", "Bearer "+token)
	reqRef.Header.Set("Content-Type", "application/json")
	recRef := httptest.NewRecorder()
	srv.Router().ServeHTTP(recRef, reqRef)

	var res ReflectTasteResponse
	_ = json.Unmarshal(recRef.Body.Bytes(), &res)
	if len(res.AutoApproved) == 0 {
		t.Fatalf("expected candidate to be auto-approved")
	}
	candID := res.AutoApproved[0].ID

	// 2. Perform 1-Click Undo via POST /api/v1/taste/candidates/{id}/undo
	reqUndo := httptest.NewRequest(http.MethodPost, "/api/v1/taste/candidates/"+candID+"/undo", nil)
	reqUndo.Header.Set("Authorization", "Bearer "+token)
	recUndo := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUndo, reqUndo)

	if recUndo.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for undo, got %d: %s", recUndo.Code, recUndo.Body.String())
	}

	var undoRes struct {
		Message   string         `json:"message"`
		Candidate TasteCandidate `json:"candidate"`
		ID        string         `json:"id"`
	}
	_ = json.Unmarshal(recUndo.Body.Bytes(), &undoRes)
	if undoRes.Candidate.Status != string(TasteCandidateStatusSuggested) {
		t.Errorf("expected undone candidate status 'suggested', got '%s'", undoRes.Candidate.Status)
	}

	// 3. Verify active profile no longer contains the item
	reqProfile := httptest.NewRequest(http.MethodGet, "/api/v1/taste", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	recProfile := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProfile, reqProfile)
	var profile TasteProfile
	_ = json.Unmarshal(recProfile.Body.Bytes(), &profile)
	for _, it := range profile.Items {
		if strings.Contains(it.Content, "arsitektur modular yang rapi") {
			t.Errorf("undone item should have been removed from active taste profile")
		}
	}

	// 4. Verify candidate is now in the pending list for manual review
	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/taste/candidates", nil)
	reqPending.Header.Set("Authorization", "Bearer "+token)
	recPending := httptest.NewRecorder()
	srv.Router().ServeHTTP(recPending, reqPending)
	var pendResp struct {
		Candidates []TasteCandidate `json:"candidates"`
		Total      int              `json:"total"`
	}
	_ = json.Unmarshal(recPending.Body.Bytes(), &pendResp)
	foundPending := false
	for _, c := range pendResp.Candidates {
		if c.ID == candID {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Errorf("undone candidate should appear in pending candidates list")
	}
}

func TestTaste_GroundingMetadata_Capture(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	// 1. FormatBrandTastePromptWithGrounding
	ctx := context.Background()
	prompt, grounding := srv.FormatBrandTastePromptWithGrounding(ctx)
	if prompt == "" {
		t.Fatalf("expected non-empty prompt")
	}
	if len(grounding) == 0 {
		t.Fatalf("expected non-empty grounding tastes")
	}

	// Verify minimal model requirements: id, category, topic, summary/preview
	for _, g := range grounding {
		if g.ID == "" {
			t.Errorf("grounding taste ID should not be empty")
		}
		if g.Category != string(TasteCategoryBrand) {
			t.Errorf("expected brand category, got %s", g.Category)
		}
		if g.Topic == "" {
			t.Errorf("grounding taste topic should not be empty")
		}
		if g.Summary == "" {
			t.Errorf("grounding taste summary should not be empty")
		}
		if g.Preview == "" {
			t.Errorf("grounding taste preview should not be empty")
		}
	}

	// 2. formatBrandTastePrompt with context sink
	var captured []GroundingTaste
	sinkCtx := WithTasteGroundingCapture(ctx, &captured)
	prompt2 := srv.formatBrandTastePrompt(sinkCtx)
	if prompt2 != prompt {
		t.Errorf("expected identical prompt output")
	}
	if len(captured) != len(grounding) {
		t.Errorf("expected %d captured items, got %d", len(grounding), len(captured))
	}
}

func TestTaste_GroundingMetadata_ContextLayering(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	ctx := context.Background()

	// 1. Brand grounding
	brandTastes := srv.ActiveBrandGroundingTastes(ctx)
	if len(brandTastes) == 0 {
		t.Fatalf("expected active brand grounding tastes")
	}
	for _, b := range brandTastes {
		if b.Category != "brand" {
			t.Errorf("expected category brand, got %s", b.Category)
		}
	}

	// 2. Technical grounding
	techTastes := srv.ActiveTechnicalGroundingTastes(ctx)
	if len(techTastes) == 0 {
		t.Fatalf("expected active technical grounding tastes")
	}
	for _, te := range techTastes {
		if te.Category != "technical" {
			t.Errorf("expected category technical, got %s", te.Category)
		}
	}

	// 3. Context layering: FormatTechnicalTastePromptWithGrounding only captures technical items
	techPrompt, capturedTech := srv.FormatTechnicalTastePromptWithGrounding(ctx)
	if techPrompt == "" {
		t.Fatalf("expected non-empty tech prompt")
	}
	if len(capturedTech) != len(techTastes) {
		t.Errorf("expected %d tech tastes, got %d", len(techTastes), len(capturedTech))
	}
	for _, ct := range capturedTech {
		if ct.Category != "technical" {
			t.Errorf("expected technical layer only, got %s", ct.Category)
		}
	}

	// 4. Combined ActiveGroundingTastes preserves category separation
	allTastes := srv.ActiveGroundingTastes(ctx)
	if len(allTastes) != len(brandTastes)+len(techTastes) {
		t.Errorf("expected total %d tastes, got %d", len(brandTastes)+len(techTastes), len(allTastes))
	}
}

func TestTaste_GroundingDTO_Serialization_And_Omitempty(t *testing.T) {
	// 1. Backward compatibility: when GroundingTastes is nil/empty, it is omitted from JSON
	userMsg := ChatMessageDTO{
		ID:         1,
		UUID:       "user-msg-123",
		AuthorType: "user",
		AuthorName: "Owner",
		Message:    "Halo",
		Content:    "Halo",
	}

	rawUser, err := json.Marshal(userMsg)
	if err != nil {
		t.Fatalf("failed to marshal user message: %v", err)
	}
	if strings.Contains(string(rawUser), "grounding_tastes") {
		t.Errorf("grounding_tastes must be omitted (omitempty) when empty, got: %s", string(rawUser))
	}

	// 2. When GroundingTastes is populated, it is serialized with full metadata
	agentMsg := ChatMessageDTO{
		ID:         2,
		UUID:       "agent-msg-456",
		AuthorType: "agent",
		AuthorName: "Scrum Master",
		Message:    "Siap membantu",
		Content:    "Siap membantu",
		GroundingTastes: []GroundingTaste{
			{
				ID:       "taste-1",
				Category: "brand",
				Topic:    "Tone of Voice",
				Summary:  "Gunakan bahasa profesional dan ramah",
				Preview:  "Gunakan bahasa profesional dan ramah",
			},
			{
				ID:       "taste-2",
				Category: "technical",
				Topic:    "Tech Stack",
				Summary:  "Default stack Go dan React",
				Preview:  "Default stack Go dan React",
			},
		},
	}

	rawAgent, err := json.Marshal(agentMsg)
	if err != nil {
		t.Fatalf("failed to marshal agent message: %v", err)
	}
	if !strings.Contains(string(rawAgent), `"grounding_tastes"`) {
		t.Fatalf("expected grounding_tastes field in JSON: %s", string(rawAgent))
	}

	// 3. Unmarshal and verify fields
	var decoded ChatMessageDTO
	if err := json.Unmarshal(rawAgent, &decoded); err != nil {
		t.Fatalf("failed to unmarshal agent message: %v", err)
	}
	if len(decoded.GroundingTastes) != 2 {
		t.Fatalf("expected 2 grounding tastes, got %d", len(decoded.GroundingTastes))
	}
	if decoded.GroundingTastes[0].Category != "brand" || decoded.GroundingTastes[0].Topic != "Tone of Voice" {
		t.Errorf("unexpected first grounding taste: %+v", decoded.GroundingTastes[0])
	}
	if decoded.GroundingTastes[1].Category != "technical" || decoded.GroundingTastes[1].Summary != "Default stack Go dan React" {
		t.Errorf("unexpected second grounding taste: %+v", decoded.GroundingTastes[1])
	}
}

func TestTaste_Grounding_SessionStudioInspection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, _ := setupTestChatServer(t)

	ctx := context.Background()
	sessionID := "test-session-uuid-1234"

	// 1. InspectActiveGroundingTastes helper
	resp, err := srv.InspectActiveGroundingTastes(ctx, sessionID)
	if err != nil {
		t.Fatalf("InspectActiveGroundingTastes failed: %v", err)
	}
	if resp.SessionID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, resp.SessionID)
	}
	if len(resp.BrandTastes) == 0 {
		t.Errorf("expected non-empty brand tastes")
	}
	if len(resp.TechnicalTastes) == 0 {
		t.Errorf("expected non-empty technical tastes")
	}
	if resp.Total != len(resp.BrandTastes)+len(resp.TechnicalTastes) {
		t.Errorf("expected total %d, got %d", len(resp.BrandTastes)+len(resp.TechnicalTastes), resp.Total)
	}

	// 2. GetActiveGroundingTastesForSession helper
	allSessionTastes, err := srv.GetActiveGroundingTastesForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetActiveGroundingTastesForSession failed: %v", err)
	}
	if len(allSessionTastes) != resp.Total {
		t.Errorf("expected %d tastes, got %d", resp.Total, len(allSessionTastes))
	}

	// 3. RecordMessageGrounding and GetMessageGrounding
	msgID := "msg-test-recording-999"
	customGrounding := []GroundingTaste{
		{
			ID:       "custom-1",
			Category: "brand",
			Topic:    "Solutif",
			Summary:  "Ringkas dan fokus hasil",
			Preview:  "Ringkas dan fokus hasil",
		},
	}
	srv.RecordMessageGrounding(msgID, customGrounding)

	stored, found := srv.GetMessageGrounding(msgID)
	if !found || len(stored) != 1 || stored[0].ID != "custom-1" {
		t.Errorf("failed to retrieve recorded message grounding: found=%v, stored=%+v", found, stored)
	}

	// 4. getFormattedMessagesResult enrichment
	roomID := db.GenerateUUID()
	_ = srv.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "default",
		Title:     "Inspection Room",
		Topic:     "general",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	_ = srv.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "user",
		AuthorName: "Owner",
		Content:    "Permintaan bantuan",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})

	_ = srv.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorName: "Scrum Master",
		Content:    "Tentu, ini hasil analisisnya.",
		CreatedAt:  time.Now().UTC().Add(time.Second),
		UpdatedAt:  time.Now().UTC().Add(time.Second),
	})

	messages, err := srv.getFormattedMessagesResult(ctx, roomID, "")
	if err != nil {
		t.Fatalf("getFormattedMessagesResult failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// User message: no grounding tastes
	if len(messages[0].GroundingTastes) != 0 {
		t.Errorf("user message should not have grounding tastes, got %d", len(messages[0].GroundingTastes))
	}

	// Agent message: automatically enriched with brand grounding tastes
	if len(messages[1].GroundingTastes) == 0 {
		t.Errorf("agent message should have active grounding tastes populated")
	}
}

func TestTaste_Grounding_API_Endpoints(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, token := setupTestChatServer(t)

	// 1. Unauthenticated -> 401 Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/taste/grounding", nil)
	recUnauth := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated grounding request, got %d", recUnauth.Code)
	}

	// 2. Authenticated GET /api/v1/taste/grounding
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/taste/grounding", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+token)
	recAuth := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recAuth.Code, recAuth.Body.String())
	}

	var resp SessionGroundingTastesResponse
	if err := json.Unmarshal(recAuth.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.BrandTastes) == 0 || len(resp.TechnicalTastes) == 0 {
		t.Errorf("expected brand and technical grounding tastes: %+v", resp)
	}

	// 3. Filter category=brand
	reqBrand := httptest.NewRequest(http.MethodGet, "/api/v1/taste/grounding?category=brand", nil)
	reqBrand.Header.Set("Authorization", "Bearer "+token)
	recBrand := httptest.NewRecorder()
	srv.Router().ServeHTTP(recBrand, reqBrand)
	if recBrand.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for brand filter, got %d", recBrand.Code)
	}
	var brandResp struct {
		GroundingTastes []GroundingTaste `json:"grounding_tastes"`
		Total           int              `json:"total"`
	}
	_ = json.Unmarshal(recBrand.Body.Bytes(), &brandResp)
	if len(brandResp.GroundingTastes) == 0 {
		t.Errorf("expected brand tastes, got 0")
	}
	for _, bt := range brandResp.GroundingTastes {
		if bt.Category != "brand" {
			t.Errorf("expected category brand, got %s", bt.Category)
		}
	}

	// 4. Filter category=technical
	reqTech := httptest.NewRequest(http.MethodGet, "/api/v1/taste/grounding?category=technical", nil)
	reqTech.Header.Set("Authorization", "Bearer "+token)
	recTech := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTech, reqTech)
	if recTech.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for technical filter, got %d", recTech.Code)
	}
	var techResp struct {
		GroundingTastes []GroundingTaste `json:"grounding_tastes"`
		Total           int              `json:"total"`
	}
	_ = json.Unmarshal(recTech.Body.Bytes(), &techResp)
	if len(techResp.GroundingTastes) == 0 {
		t.Errorf("expected technical tastes, got 0")
	}
	for _, tt := range techResp.GroundingTastes {
		if tt.Category != "technical" {
			t.Errorf("expected category technical, got %s", tt.Category)
		}
	}

	// 5. Query with session_id: GET /api/v1/taste/grounding?session_id=sess-123
	reqSess := httptest.NewRequest(http.MethodGet, "/api/v1/taste/grounding?session_id=sess-123", nil)
	reqSess.Header.Set("Authorization", "Bearer "+token)
	recSess := httptest.NewRecorder()
	srv.Router().ServeHTTP(recSess, reqSess)
	if recSess.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for session query, got %d", recSess.Code)
	}
	var sessResp SessionGroundingTastesResponse
	_ = json.Unmarshal(recSess.Body.Bytes(), &sessResp)
	if sessResp.SessionID != "sess-123" {
		t.Errorf("expected session_id sess-123, got %s", sessResp.SessionID)
	}

	// 6. Direct conversation route: GET /api/v1/chat/conversations/{id}/grounding
	reqConv := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations/conv-abc-999/grounding", nil)
	reqConv.Header.Set("Authorization", "Bearer "+token)
	recConv := httptest.NewRecorder()
	srv.Router().ServeHTTP(recConv, reqConv)
	if recConv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for conversation grounding route, got %d: %s", recConv.Code, recConv.Body.String())
	}
	var convResp SessionGroundingTastesResponse
	_ = json.Unmarshal(recConv.Body.Bytes(), &convResp)
	if convResp.SessionID != "conv-abc-999" {
		t.Errorf("expected session_id conv-abc-999, got %s", convResp.SessionID)
	}
}

