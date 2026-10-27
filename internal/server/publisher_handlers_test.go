package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/publisher"
)

func TestPublisherHandlers_FullWorkflow(t *testing.T) {
	// 1. Setup mock WordPress REST server
	wpMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"code": "unauthorized"})
			return
		}

		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/users/me"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":    10,
				"name":  "Buchin Admin",
				"slug":  "buchin",
				"roles": []string{"administrator"},
			})

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/posts"):
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)

			status, _ := body["status"].(string)
			if status == "" {
				status = "draft"
			}
			title, _ := body["title"].(string)
			content, _ := body["content"].(string)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     555,
				"date":   time.Now().UTC().Format(time.RFC3339),
				"status": status,
				"link":   "https://cuan.dojo.cc/posts/agentic-scrum-deep-dive/",
				"title": map[string]string{
					"rendered": title,
				},
				"content": map[string]string{
					"rendered": content,
				},
			})

		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer wpMockServer.Close()

	// 2. Setup Server with memory repositories
	cfg := &config.Config{
		AuthSecret: "super-secret-auth-key-32-bytes!",
	}
	srv := NewServer(cfg, nil)

	studioRepo := db.NewMemoryProjectStudioRepository()
	intRepo := db.NewMemoryProjectIntegrationRepository()
	srv.projectStudioRepo = studioRepo
	srv.projectIntegrationRepo = intRepo

	// Override publisher engine to route to wpMockServer
	engine := publisher.NewEngine(
		publisher.WithStudioRepo(studioRepo),
		publisher.WithIntegrationRepo(intRepo),
		publisher.WithSecretKey(cfg.AuthSecret),
		publisher.WithClientFactory(func(c publisher.WordPressConfig) (publisher.Client, error) {
			c.HTTPClient = wpMockServer.Client()
			return publisher.NewWordPressClient(c)
		}),
	)
	srv.SetPublisherEngine(engine)

	projectID := "proj-blog-123"
	secretPassword := "wp-application-pass-secret-xyz"

	// -------------------------------------------------------------------------
	// A. Setup WordPress Integration: POST /api/v1/projects/{id}/integrations/wordpress
	// -------------------------------------------------------------------------
	setupPayload := map[string]interface{}{
		"site_url":             wpMockServer.URL,
		"username":             "buchin",
		"application_password": secretPassword,
		"test_connection":      true,
	}
	setupBytes, _ := json.Marshal(setupPayload)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/integrations/wordpress", projectID), bytes.NewReader(setupBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST setup wordpress integration failed with status %d: %s", w.Code, w.Body.String())
	}

	var intDTO db.ProjectIntegrationDTO
	if err := json.Unmarshal(w.Body.Bytes(), &intDTO); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !intDTO.HasConfig || intDTO.IntegrationType != "wordpress" || intDTO.ProjectID != projectID {
		t.Errorf("unexpected integration DTO: %+v", intDTO)
	}
	// Zero credential leakage check
	if strings.Contains(w.Body.String(), secretPassword) {
		t.Fatalf("ZERO LEAKAGE VIOLATION: setup response leaked password: %s", w.Body.String())
	}

	// -------------------------------------------------------------------------
	// B. Get WordPress Integration: GET /api/v1/projects/{id}/integrations/wordpress
	// -------------------------------------------------------------------------
	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/integrations/wordpress", projectID), nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET wordpress integration failed with status %d: %s", getW.Code, getW.Body.String())
	}
	if strings.Contains(getW.Body.String(), secretPassword) {
		t.Fatalf("ZERO LEAKAGE VIOLATION: get response leaked password: %s", getW.Body.String())
	}

	// -------------------------------------------------------------------------
	// C. Seed Task with AgentOutput
	// -------------------------------------------------------------------------
	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Agentic Scrum Deep Dive",
		Description: "Research and outline on Agentic Scrum principles.",
		Status:      "doing",
		Priority:    "urgent",
		Tags:        []string{"scrum", "ai", "agile"},
		AgentOutput: "```markdown\n# Agentic Scrum Deep Dive\n\nAgentic Scrum merges Scrum practices with autonomous agent pair-programming.\n\n## Key Benefits\n- Velocity\n- Zero Friction\n```",
	}
	if err := studioRepo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// -------------------------------------------------------------------------
	// D. Publish Task: POST /api/v1/tasks/{id}/publish
	// -------------------------------------------------------------------------
	pubPayload := map[string]interface{}{
		"status":          "draft",
		"convert_markdown": true,
	}
	pubBytes, _ := json.Marshal(pubPayload)

	pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/publish", taskID), bytes.NewReader(pubBytes))
	pubReq.Header.Set("Content-Type", "application/json")
	pubW := httptest.NewRecorder()
	srv.Router().ServeHTTP(pubW, pubReq)

	if pubW.Code != http.StatusOK {
		t.Fatalf("POST publish task failed with status %d: %s", pubW.Code, pubW.Body.String())
	}

	var pubRes publisher.PublishResult
	if err := json.Unmarshal(pubW.Body.Bytes(), &pubRes); err != nil {
		t.Fatalf("failed to decode publish response: %v", err)
	}
	if !pubRes.Success || pubRes.PostID != 555 {
		t.Errorf("unexpected publish result: %+v", pubRes)
	}
	if pubRes.PublicationURL != "https://cuan.dojo.cc/posts/agentic-scrum-deep-dive/" {
		t.Errorf("unexpected publication url: %s", pubRes.PublicationURL)
	}

	// Verify task in DB was updated with publication_url and published_at
	persistedTask, err := studioRepo.GetTaskByID(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if persistedTask.PublicationURL != "https://cuan.dojo.cc/posts/agentic-scrum-deep-dive/" {
		t.Errorf("persisted publication_url not matching: %s", persistedTask.PublicationURL)
	}
	if persistedTask.PublishedAt == nil {
		t.Errorf("persisted published_at should not be nil")
	}

	// -------------------------------------------------------------------------
	// E. Export Task: GET /api/v1/tasks/{id}/export?format={md|html|txt}
	// -------------------------------------------------------------------------
	// 1. Export as Markdown
	expMDReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s/export?format=md", taskID), nil)
	expMDW := httptest.NewRecorder()
	srv.Router().ServeHTTP(expMDW, expMDReq)

	if expMDW.Code != http.StatusOK {
		t.Fatalf("export md failed with status %d: %s", expMDW.Code, expMDW.Body.String())
	}
	if expMDW.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
		t.Errorf("expected text/markdown header, got %s", expMDW.Header().Get("Content-Type"))
	}
	if !strings.Contains(expMDW.Body.String(), "Agentic Scrum Deep Dive") {
		t.Errorf("md export content missing title: %s", expMDW.Body.String())
	}

	// 2. Export as HTML
	expHTMLReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s/export?format=html", taskID), nil)
	expHTMLW := httptest.NewRecorder()
	srv.Router().ServeHTTP(expHTMLW, expHTMLReq)

	if expHTMLW.Code != http.StatusOK {
		t.Fatalf("export html failed with status %d: %s", expHTMLW.Code, expHTMLW.Body.String())
	}
	if expHTMLW.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html header, got %s", expHTMLW.Header().Get("Content-Type"))
	}
	if !strings.Contains(expHTMLW.Body.String(), "<h1>Agentic Scrum Deep Dive</h1>") {
		t.Errorf("html export content missing h1: %s", expHTMLW.Body.String())
	}
	if !strings.Contains(expHTMLW.Body.String(), "<li>Velocity</li>") {
		t.Errorf("html export content missing list item: %s", expHTMLW.Body.String())
	}

	// 3. Export as Plain Text (Teleprompter)
	expTxtReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s/export?format=txt", taskID), nil)
	expTxtW := httptest.NewRecorder()
	srv.Router().ServeHTTP(expTxtW, expTxtReq)

	if expTxtW.Code != http.StatusOK {
		t.Fatalf("export txt failed with status %d: %s", expTxtW.Code, expTxtW.Body.String())
	}
	if expTxtW.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("expected text/plain header, got %s", expTxtW.Header().Get("Content-Type"))
	}
	txtBody := expTxtW.Body.String()
	if strings.Contains(txtBody, "```") || strings.Contains(txtBody, "<h1>") {
		t.Errorf("txt export should have no markdown fences or html tags: %s", txtBody)
	}
	if !strings.Contains(txtBody, "• Velocity") {
		t.Errorf("txt export should have clean bullet: %s", txtBody)
	}

	// 4. Export with unsupported format
	expBadReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s/export?format=docx", taskID), nil)
	expBadW := httptest.NewRecorder()
	srv.Router().ServeHTTP(expBadW, expBadReq)
	if expBadW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on unsupported format, got %d", expBadW.Code)
	}

	// -------------------------------------------------------------------------
	// F. Negative Test: Zero credential leakage when setup fails
	// -------------------------------------------------------------------------
	badSetupPayload := map[string]interface{}{
		"site_url":             "http://127.0.0.1:54321", // dead port
		"username":             "admin",
		"application_password": "sensitive-leaked-password-9999",
		"test_connection":      true,
	}
	badBytes, _ := json.Marshal(badSetupPayload)
	badReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/integrations/wordpress", projectID), bytes.NewReader(badBytes))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	srv.Router().ServeHTTP(badW, badReq)

	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on connection failure, got %d", badW.Code)
	}
	if strings.Contains(badW.Body.String(), "sensitive-leaked-password-9999") {
		t.Fatalf("ZERO CREDENTIAL LEAKAGE VIOLATION: password leaked on setup error: %s", badW.Body.String())
	}
}
