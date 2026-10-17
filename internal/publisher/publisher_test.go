package publisher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestWordPressConfig_ValidationAndSanitization(t *testing.T) {
	cfg := WordPressConfig{
		SiteURL:             "https://example.com/wp-json/",
		Username:            "admin",
		ApplicationPassword: "secret-password-123",
	}

	if err := cfg.Normalize(); err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if cfg.SiteURL != "https://example.com/wp-json" {
		t.Errorf("expected trimmed trailing slash, got %q", cfg.SiteURL)
	}

	str := cfg.String()
	if strings.Contains(str, "secret-password-123") {
		t.Errorf("String() leaked secret application password: %s", str)
	}
	if !strings.Contains(str, "[REDACTED]") {
		t.Errorf("String() missing [REDACTED]: %s", str)
	}

	goStr := cfg.GoString()
	if strings.Contains(goStr, "secret-password-123") {
		t.Errorf("GoString() leaked secret application password: %s", goStr)
	}
}

func TestWordPressClient_MockServer_Success(t *testing.T) {
	expectedUser := "editor"
	expectedPassword := "abcd efgh ijkl mnop"
	authHeaderValue := "Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", expectedUser, expectedPassword)))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Basic Auth header
		authHeader := r.Header.Get("Authorization")
		if authHeader != authHeaderValue {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"code":    "incorrect_password",
				"message": "The password you provided is incorrect.",
			})
			return
		}

		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/users/me"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":    1,
				"name":  "Site Editor",
				"slug":  "editor",
				"roles": []string{"editor", "administrator"},
			})

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/tags"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Return one matching tag and one empty
			if r.URL.Query().Get("search") == "technology" {
				json.NewEncoder(w).Encode([]map[string]interface{}{
					{"id": 101, "name": "technology", "slug": "technology"},
				})
			} else {
				json.NewEncoder(w).Encode([]map[string]interface{}{})
			}

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/tags"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   202,
				"name": body["name"],
				"slug": body["name"],
			})

		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/posts"):
			var payload wpPostPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       42,
				"date":     "2026-10-17T12:00:00",
				"date_gmt": "2026-10-17T12:00:00",
				"status":   payload.Status,
				"link":     fmt.Sprintf("https://example.com/2026/10/17/%s/", payload.Slug),
				"title": map[string]string{
					"raw":      payload.Title,
					"rendered": payload.Title,
				},
				"content": map[string]string{
					"raw":      payload.Content,
					"rendered": payload.Content,
				},
				"tags": payload.Tags,
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := WordPressConfig{
		SiteURL:             server.URL,
		Username:            expectedUser,
		ApplicationPassword: expectedPassword,
		HTTPClient:          server.Client(),
	}

	client, err := NewWordPressClient(cfg)
	if err != nil {
		t.Fatalf("NewWordPressClient failed: %v", err)
	}

	// 1. TestConnection
	ctx := context.Background()
	user, err := client.TestConnection(ctx)
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
	if user.ID != 1 || user.Name != "Site Editor" {
		t.Errorf("unexpected user: %+v", user)
	}

	// 2. CreatePost Draft
	postReq := CreatePostRequest{
		Title:           "Automated Micro-SaaS Guide",
		Content:         "# Introduction\n\nBuild fast with Cuan Studio.",
		Status:          "draft",
		Tags:            []string{"technology", "startup"},
		Slug:            "automated-micro-saas-guide",
		ConvertMarkdown: true,
	}

	post, err := client.CreatePost(ctx, postReq)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}
	if post.ID != 42 {
		t.Errorf("expected post id 42, got %d", post.ID)
	}
	if post.Status != "draft" {
		t.Errorf("expected draft status, got %s", post.Status)
	}
	if !strings.Contains(post.Link, "automated-micro-saas-guide") {
		t.Errorf("unexpected publication url: %s", post.Link)
	}
}

func TestWordPressClient_ZeroCredentialLeakage_OnErrors(t *testing.T) {
	sensitivePassword := "super-secret-top-secret-token-xyz-12345"

	// Mock server that returns 401 Unauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code":"incorrect_password","message":"Invalid credentials."}`))
	}))
	defer server.Close()

	cfg := WordPressConfig{
		SiteURL:             server.URL,
		Username:            "testuser",
		ApplicationPassword: sensitivePassword,
		HTTPClient:          server.Client(),
	}

	client, err := NewWordPressClient(cfg)
	if err != nil {
		t.Fatalf("NewWordPressClient failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.TestConnection(ctx)
	if err == nil {
		t.Fatal("expected error on 401 Unauthorized, got nil")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, sensitivePassword) {
		t.Fatalf("ZERO CREDENTIAL LEAKAGE VIOLATION: password found in error message: %s", errMsg)
	}

	// Negative post creation
	_, err = client.CreatePost(ctx, CreatePostRequest{
		Title:   "Test Post",
		Content: "Test Content",
	})
	if err == nil {
		t.Fatal("expected error on 401 Unauthorized during post creation, got nil")
	}
	if strings.Contains(err.Error(), sensitivePassword) {
		t.Fatalf("ZERO CREDENTIAL LEAKAGE VIOLATION: password found in create post error: %s", err.Error())
	}
}

func TestEngine_WordPressIntegration_And_PublishTask(t *testing.T) {
	// Setup mock WordPress server
	wpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/users/me"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   2,
				"name": "Buchin Author",
				"slug": "buchin",
			})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/posts"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     99,
				"date":   time.Now().UTC().Format(time.RFC3339),
				"status": "draft",
				"link":   "https://cuan.dojo.cc/blog/agentic-scrum-article/",
				"title": map[string]string{
					"rendered": "Agentic Scrum in Action",
				},
				"content": map[string]string{
					"rendered": "<p>Content</p>",
				},
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer wpServer.Close()

	ctx := context.Background()
	studioRepo := db.NewMemoryProjectStudioRepository()
	integrationRepo := db.NewMemoryProjectIntegrationRepository()
	secretKey := "test-secret-encryption-key-32-b"

	engine := NewEngine(
		WithStudioRepo(studioRepo),
		WithIntegrationRepo(integrationRepo),
		WithSecretKey(secretKey),
		WithClientFactory(func(cfg WordPressConfig) (Client, error) {
			cfg.HTTPClient = wpServer.Client()
			return NewWordPressClient(cfg)
		}),
	)

	projectID := "proj-wp-test"
	taskID := db.GenerateUUID()

	// 1. Create task in studioRepo with AgentOutput
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Draft Agentic Scrum Article",
		Description: "Initial brief",
		Status:      "doing",
		Priority:    "high",
		Tags:        []string{"agile", "ai"},
		AgentOutput: "```markdown\n# Agentic Scrum in Action\n\nPair programming with autonomous agents.\n```",
	}
	if err := studioRepo.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// 2. Setup WordPress integration
	cfg := WordPressConfig{
		SiteURL:             wpServer.URL,
		Username:            "buchin",
		ApplicationPassword: "test-application-password",
	}

	intDTO, err := engine.SetupWordPressIntegration(ctx, projectID, cfg, true)
	if err != nil {
		t.Fatalf("SetupWordPressIntegration failed: %v", err)
	}
	if !intDTO.HasConfig || intDTO.IntegrationType != "wordpress" {
		t.Errorf("unexpected integration DTO: %+v", intDTO)
	}

	// 3. Publish Task
	pubRes, err := engine.PublishTask(ctx, taskID, PublishTaskOptions{
		Status:          "draft",
		ConvertMarkdown: true,
	})
	if err != nil {
		t.Fatalf("PublishTask failed: %v", err)
	}
	if !pubRes.Success || pubRes.PostID != 99 {
		t.Errorf("unexpected publish result: %+v", pubRes)
	}
	if pubRes.PublicationURL != "https://cuan.dojo.cc/blog/agentic-scrum-article/" {
		t.Errorf("unexpected publication url: %s", pubRes.PublicationURL)
	}

	// 4. Verify task in database has updated publication_url and published_at
	updatedTask, err := studioRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if updatedTask.PublicationURL != "https://cuan.dojo.cc/blog/agentic-scrum-article/" {
		t.Errorf("task.PublicationURL not updated: %s", updatedTask.PublicationURL)
	}
	if updatedTask.PublishedAt == nil {
		t.Errorf("task.PublishedAt should not be nil")
	}

	// 5. Test ExportTask
	mdExport, err := engine.ExportTask(ctx, taskID, "md")
	if err != nil {
		t.Fatalf("ExportTask(md) failed: %v", err)
	}
	if !strings.Contains(mdExport.Content, "Pair programming with autonomous agents.") {
		t.Errorf("exported markdown missing content: %s", mdExport.Content)
	}

	htmlExport, err := engine.ExportTask(ctx, taskID, "html")
	if err != nil {
		t.Fatalf("ExportTask(html) failed: %v", err)
	}
	if !strings.Contains(htmlExport.Content, "<p>Pair programming with autonomous agents.</p>") {
		t.Errorf("exported html missing paragraph: %s", htmlExport.Content)
	}

	txtExport, err := engine.ExportTask(ctx, taskID, "txt")
	if err != nil {
		t.Fatalf("ExportTask(txt) failed: %v", err)
	}
	if !strings.Contains(txtExport.Content, "Pair programming with autonomous agents.") {
		t.Errorf("exported text missing content: %s", txtExport.Content)
	}

	// 6. Test Fallback to Description when AgentOutput is empty
	task2ID := db.GenerateUUID()
	task2 := &db.ProjectTask{
		ID:          task2ID,
		ProjectID:   projectID,
		Title:       "Task Without Agent Output",
		Description: "Fallback description used as content",
		Status:      "backlog",
	}
	_ = studioRepo.CreateTask(ctx, task2)
	pubRes2, err := engine.PublishTask(ctx, task2ID, PublishTaskOptions{Status: "publish"})
	if err != nil {
		t.Fatalf("PublishTask fallback to description failed: %v", err)
	}
	if !pubRes2.Success {
		t.Errorf("expected success for fallback to description")
	}

	// 7. Test Empty Content Error
	task3ID := db.GenerateUUID()
	task3 := &db.ProjectTask{
		ID:        task3ID,
		ProjectID: projectID,
		Title:     "Completely Empty Task",
		Status:    "backlog",
	}
	_ = studioRepo.CreateTask(ctx, task3)
	_, err = engine.PublishTask(ctx, task3ID, PublishTaskOptions{})
	if err == nil || !strings.Contains(err.Error(), "no deliverable content") {
		t.Errorf("expected ErrEmptyTaskContent, got %v", err)
	}

	// 8. Test Missing Integration
	task4ID := db.GenerateUUID()
	task4 := &db.ProjectTask{
		ID:          task4ID,
		ProjectID:   "proj-without-wp",
		Title:       "Unlinked Project Task",
		Description: "Some content",
	}
	_ = studioRepo.CreateTask(ctx, task4)
	_, err = engine.PublishTask(ctx, task4ID, PublishTaskOptions{})
	if err == nil || !strings.Contains(err.Error(), "wordpress integration not found") {
		t.Errorf("expected ErrIntegrationNotFound, got %v", err)
	}
}

func TestWordPressClient_HTTP403And500_ZeroLeakage(t *testing.T) {
	secretPass := "my-secret-app-pass-5678"

	// Mock server returning 403
	server403 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"code":"rest_cannot_view","message":"Sorry, you are not allowed to view users."}`))
	}))
	defer server403.Close()

	client403, _ := NewWordPressClient(WordPressConfig{
		SiteURL:             server403.URL,
		Username:            "subscriber",
		ApplicationPassword: secretPass,
		HTTPClient:          server403.Client(),
	})

	_, err := client403.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error on 403 Forbidden")
	}
	if strings.Contains(err.Error(), secretPass) {
		t.Fatalf("ZERO CREDENTIAL LEAKAGE VIOLATION: password leaked on 403: %s", err.Error())
	}

	// Mock server returning 500
	server500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Fatal error in /var/www/html/wp-includes/rest-api.php`))
	}))
	defer server500.Close()

	client500, _ := NewWordPressClient(WordPressConfig{
		SiteURL:             server500.URL,
		Username:            "admin",
		ApplicationPassword: secretPass,
		HTTPClient:          server500.Client(),
	})

	_, err = client500.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error on 500 Internal Server Error")
	}
	if strings.Contains(err.Error(), secretPass) {
		t.Fatalf("ZERO CREDENTIAL LEAKAGE VIOLATION: password leaked on 500: %s", err.Error())
	}
}

