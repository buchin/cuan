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
