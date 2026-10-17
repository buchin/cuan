package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/creator"
	"github.com/buchin/cuan/internal/db"
)

func setupTestServerForCreator(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-chars-long!",
	}
	srv := NewServer(cfg, nil)
	return srv
}

func TestCreatorTemplatesEndpoints(t *testing.T) {
	srv := setupTestServerForCreator(t)

	endpoints := []string{
		"/api/v1/creator/templates",
		"/api/creator/templates",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
			}

			var resp struct {
				Status    string             `json:"status"`
				Count     int                `json:"count"`
				Templates []creator.Template `json:"templates"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response json: %v", err)
			}

			if resp.Status != "success" {
				t.Errorf("expected status 'success', got %q", resp.Status)
			}
			if resp.Count != 4 || len(resp.Templates) != 4 {
				t.Fatalf("expected 4 templates, got count=%d, len=%d", resp.Count, len(resp.Templates))
			}

			// Verify templates by ID
			ids := make(map[string]creator.Template)
			for _, tmpl := range resp.Templates {
				ids[tmpl.ID] = tmpl
			}

			for _, expectedID := range []string{creator.BlueprintBlog, creator.BlueprintYouTube, creator.BlueprintOlshop, creator.BlueprintAffiliate} {
				tmpl, exists := ids[expectedID]
				if !exists {
					t.Errorf("expected template %s to exist", expectedID)
					continue
				}
				if tmpl.Name == "" || tmpl.Description == "" {
					t.Errorf("[%s] expected name and description", expectedID)
				}
				if tmpl.Room.Title == "" {
					t.Errorf("[%s] expected room title", expectedID)
				}
				if tmpl.Agent.Name == "" || tmpl.Agent.Role == "" {
					t.Errorf("[%s] expected agent name and role", expectedID)
				}
				if len(tmpl.Tasks) < 3 {
					t.Errorf("[%s] expected >= 3 tasks, got %d", expectedID, len(tmpl.Tasks))
				}
				for _, task := range tmpl.Tasks {
					if len(task.DoDChecklist) < 3 {
						t.Errorf("[%s task %s] expected >= 3 DoD items, got %d", expectedID, task.Title, len(task.DoDChecklist))
					}
				}
			}
		})
	}
}

func TestGetCreatorTemplateByIDAndAliases(t *testing.T) {
	srv := setupTestServerForCreator(t)

	testCases := []struct {
		url          string
		expectedCode int
		expectedID   string
		expectedName string
	}{
		{"/api/v1/creator/templates/blog", http.StatusOK, creator.BlueprintBlog, "Blog Publisher"},
		{"/api/creator/templates/blog", http.StatusOK, creator.BlueprintBlog, "Blog Publisher"},
		{"/api/v1/creator/templates/blog_publisher", http.StatusOK, creator.BlueprintBlog, "Blog Publisher"},
		{"/api/v1/creator/templates/youtube", http.StatusOK, creator.BlueprintYouTube, "YouTube Channel"},
		{"/api/creator/templates/youtube_channel", http.StatusOK, creator.BlueprintYouTube, "YouTube Channel"},
		{"/api/v1/creator/templates/olshop", http.StatusOK, creator.BlueprintOlshop, "Online Shop / Digital Product"},
		{"/api/creator/templates/online_shop", http.StatusOK, creator.BlueprintOlshop, "Online Shop / Digital Product"},
		{"/api/v1/creator/templates/affiliate", http.StatusOK, creator.BlueprintAffiliate, "Affiliate Site"},
		{"/api/creator/templates/affiliate_site", http.StatusOK, creator.BlueprintAffiliate, "Affiliate Site"},
		{"/api/v1/creator/templates/nonexistent_template", http.StatusNotFound, "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != tc.expectedCode {
				t.Fatalf("expected code %d, got %d. Body: %s", tc.expectedCode, w.Code, w.Body.String())
			}

			if tc.expectedCode == http.StatusOK {
				var resp struct {
					Status   string           `json:"status"`
					Template creator.Template `json:"template"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode json: %v", err)
				}
				if resp.Template.ID != tc.expectedID {
					t.Errorf("expected template ID %q, got %q", tc.expectedID, resp.Template.ID)
				}
				if resp.Template.Name != tc.expectedName {
					t.Errorf("expected template Name %q, got %q", tc.expectedName, resp.Template.Name)
				}
			}
		})
	}
}

func TestInstantiateCreatorProjects_AllFourBlueprints(t *testing.T) {
	srv := setupTestServerForCreator(t)

	blueprints := []struct {
		templateID      string
		expectedProject string
		expectedRoom    string
		expectedAgent   string
		expectedTask    string
	}{
		{
			templateID:      creator.BlueprintBlog,
			expectedProject: "Tech Insights Blog",
			expectedRoom:    "SEO Research",
			expectedAgent:   "SEO Specialist",
			expectedTask:    "Keyword research",
		},
		{
			templateID:      creator.BlueprintYouTube,
			expectedProject: "Tech Explainer Channel",
			expectedRoom:    "Channel Production",
			expectedAgent:   "YouTube Scriptwriter",
			expectedTask:    "Ideation & CTR hook",
		},
		{
			templateID:      creator.BlueprintOlshop,
			expectedProject: "Digital Growth Toolkit",
			expectedRoom:    "Sales Funnel",
			expectedAgent:   "Conversion Copywriter",
			expectedTask:    "Sales page copy",
		},
		{
			templateID:      creator.BlueprintAffiliate,
			expectedProject: "Audio Gear Comparisons",
			expectedRoom:    "Niche Affiliate",
			expectedAgent:   "Affiliate Strategist",
			expectedTask:    "Product comparison matrix",
		},
	}

	for _, bp := range blueprints {
		t.Run(bp.templateID, func(t *testing.T) {
			payload := map[string]interface{}{
				"template_id":  bp.templateID,
				"project_name": bp.expectedProject,
			}
			bodyBytes, _ := json.Marshal(payload)

			// Test both /api/v1/creator/projects/instantiate and /api/creator/projects/instantiate
			ep := "/api/v1/creator/projects/instantiate"
			req := httptest.NewRequest(http.MethodPost, ep, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
			}

			var resp creator.InstantiateResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Status != "success" {
				t.Errorf("expected status 'success', got %q", resp.Status)
			}
			if resp.TemplateID != bp.templateID {
				t.Errorf("expected template_id %q, got %q", bp.templateID, resp.TemplateID)
			}

			// Verify Project
			if resp.Project == nil || resp.Project.ID == "" {
				t.Fatal("expected non-empty project ID")
			}
			if resp.Project.Name != bp.expectedProject {
				t.Errorf("expected project name %q, got %q", bp.expectedProject, resp.Project.Name)
			}
			if resp.Project.ProjectType != bp.templateID {
				t.Errorf("expected project_type %q, got %q", bp.templateID, resp.Project.ProjectType)
			}

			// Verify Room
			if resp.Room.ID == "" {
				t.Fatal("expected non-empty room ID")
			}
			if resp.Room.Title != bp.expectedRoom {
				t.Errorf("expected room title %q, got %q", bp.expectedRoom, resp.Room.Title)
			}
			if resp.Room.ProjectID != resp.Project.ID {
				t.Errorf("room project_id mismatch: %q vs %q", resp.Room.ProjectID, resp.Project.ID)
			}

			// Verify Agent Persona
			if resp.Agent.ID == "" {
				t.Fatal("expected non-empty agent ID")
			}
			if resp.Agent.Name != bp.expectedAgent {
				t.Errorf("expected agent name %q, got %q", bp.expectedAgent, resp.Agent.Name)
			}

			// Verify Tasks
			if len(resp.Tasks) < 3 {
				t.Fatalf("expected >= 3 tasks, got %d", len(resp.Tasks))
			}

			var foundExpectedTask bool
			for _, task := range resp.Tasks {
				if task.ProjectID != resp.Project.ID {
					t.Errorf("task project_id %q mismatch", task.ProjectID)
				}
				if task.RoomID == nil || *task.RoomID != resp.Room.ID {
					t.Errorf("task room_id %v mismatch with room %q", task.RoomID, resp.Room.ID)
				}
				if task.AssigneeID == nil || *task.AssigneeID != resp.Agent.ID {
					t.Errorf("task assignee_id %v mismatch with agent %q", task.AssigneeID, resp.Agent.ID)
				}
				if len(task.DoDChecklist) < 3 {
					t.Errorf("task %q DoD items < 3: got %d", task.Title, len(task.DoDChecklist))
				}
				if strings.Contains(task.Title, bp.expectedTask) {
					foundExpectedTask = true
				}
			}
			if !foundExpectedTask {
				t.Errorf("did not find expected task %q in generated tasks", bp.expectedTask)
			}

			// 5. Query Project Studio endpoints to verify data is accessible via GET APIs
			// GET /api/projects/{id}/rooms
			roomReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+resp.Project.ID+"/rooms", nil)
			roomRec := httptest.NewRecorder()
			srv.Router().ServeHTTP(roomRec, roomReq)
			if roomRec.Code != http.StatusOK {
				t.Errorf("GET rooms failed: code %d, body: %s", roomRec.Code, roomRec.Body.String())
			}

			// GET /api/projects/{id}/tasks
			taskReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+resp.Project.ID+"/tasks", nil)
			taskRec := httptest.NewRecorder()
			srv.Router().ServeHTTP(taskRec, taskReq)
			if taskRec.Code != http.StatusOK {
				t.Errorf("GET tasks failed: code %d, body: %s", taskRec.Code, taskRec.Body.String())
			}

			// GET /api/agents
			agentReq := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
			agentRec := httptest.NewRecorder()
			srv.Router().ServeHTTP(agentRec, agentReq)
			if agentRec.Code != http.StatusOK {
				t.Errorf("GET agents failed: code %d, body: %s", agentRec.Code, agentRec.Body.String())
			}
		})
	}
}

func TestInstantiateCreatorProjects_AliasAndValidation(t *testing.T) {
	srv := setupTestServerForCreator(t)

	// 1. Test alias endpoint /api/creator/projects/instantiate
	t.Run("alias_endpoint", func(t *testing.T) {
		payload := map[string]interface{}{
			"blueprint_id": "blog_publisher",
			"name":         "Aliased Blog",
		}
		bodyBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/creator/projects/instantiate", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created from alias endpoint, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// 2. Test missing template_id
	t.Run("missing_template_id", func(t *testing.T) {
		payload := map[string]interface{}{
			"project_name": "No Template",
		}
		bodyBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	// 3. Test invalid template_id
	t.Run("invalid_template_id", func(t *testing.T) {
		payload := map[string]interface{}{
			"template_id":  "totally_fake_template",
			"project_name": "Fake Project",
		}
		bodyBytes, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	// 4. Test invalid json body
	t.Run("invalid_json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader([]byte("{invalid-json")))
		w := httptest.NewRecorder()

		srv.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestInstantiateCreatorProjects_ThreadSafety(t *testing.T) {
	srv := setupTestServerForCreator(t)

	concurrency := 6
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			payload := map[string]interface{}{
				"template_id":  creator.BlueprintYouTube,
				"project_name": "Concurrent Channel " + string(rune('A'+idx)),
			}
			bodyBytes, _ := json.Marshal(payload)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader(bodyBytes))
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				errCh <- db.ErrDuplicateRoom
				return
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent instantiate failed: %v", err)
		}
	}
}
