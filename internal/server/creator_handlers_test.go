package server

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestExecuteTaskAgent_MissingGeminiKey_GracefulError(t *testing.T) {
	srv := setupTestServerForCreator(t)

	// 1. Instantiate a Blog Project
	instPayload := map[string]interface{}{
		"template_id":  creator.BlueprintBlog,
		"project_name": "SEO Blog Without Key",
	}
	body, _ := json.Marshal(instPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on instantiate, got %d: %s", w.Code, w.Body.String())
	}

	var instResp struct {
		Tasks []db.ProjectTaskDTO `json:"tasks"`
	}
	json.Unmarshal(w.Body.Bytes(), &instResp)
	if len(instResp.Tasks) == 0 {
		t.Fatalf("expected tasks created")
	}
	targetTask := instResp.Tasks[0]

	// 2. Call POST /api/v1/tasks/{id}/execute-agent without configured key
	execReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/execute-agent", targetTask.ID), nil)
	execW := httptest.NewRecorder()
	srv.Router().ServeHTTP(execW, execReq)

	if execW.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity, got %d: %s", execW.Code, execW.Body.String())
	}

	var errResp struct {
		Status  string `json:"status"`
		Code    string `json:"code"`
		Error   string `json:"error"`
		HelpURL string `json:"help_url"`
	}
	if err := json.Unmarshal(execW.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error json: %v", err)
	}

	if errResp.Code != "MISSING_GEMINI_KEY" {
		t.Errorf("expected code 'MISSING_GEMINI_KEY', got %q", errResp.Code)
	}
	if !strings.Contains(errResp.Error, "Agent Hub") {
		t.Errorf("expected error message to direct user to Agent Hub, got: %s", errResp.Error)
	}
	if errResp.HelpURL != "/studio/agents" {
		t.Errorf("expected help_url '/studio/agents', got %q", errResp.HelpURL)
	}
}

func TestExecuteTaskAgent_Success_FullWorkflow(t *testing.T) {
	srv := setupTestServerForCreator(t)

	// 1. Register Gemini Key via POST /api/agent-connections
	connPayload := map[string]interface{}{
		"provider":   "google",
		"api_key":    "mock-gemini-pro-creator-key-vault",
		"verify_now": false,
	}
	connBody, _ := json.Marshal(connPayload)
	connReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-connections", bytes.NewReader(connBody))
	connW := httptest.NewRecorder()
	srv.Router().ServeHTTP(connW, connReq)
	if connW.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create connection, got %d: %s", connW.Code, connW.Body.String())
	}

	// 2. Instantiate YouTube Channel project
	instPayload := map[string]interface{}{
		"template_id":  creator.BlueprintYouTube,
		"project_name": "Tech Reviews Channel",
	}
	instBody, _ := json.Marshal(instPayload)
	instReq := httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader(instBody))
	instW := httptest.NewRecorder()
	srv.Router().ServeHTTP(instW, instReq)
	if instW.Code != http.StatusCreated {
		t.Fatalf("expected 201 on instantiate, got %d", instW.Code)
	}

	var instResp struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
		Tasks []db.ProjectTaskDTO `json:"tasks"`
	}
	json.Unmarshal(instW.Body.Bytes(), &instResp)

	// Pick "10-min scriptwriting" task (index 1)
	var scriptTask db.ProjectTaskDTO
	for _, tsk := range instResp.Tasks {
		if strings.Contains(strings.ToLower(tsk.Title), "scriptwriting") {
			scriptTask = tsk
			break
		}
	}
	if scriptTask.ID == "" {
		scriptTask = instResp.Tasks[0]
	}

	// 3. Execute agent on task: POST /api/v1/tasks/{id}/execute-agent
	execPayload := map[string]interface{}{
		"instructions": "Incorporate tech review benchmark angles",
	}
	execBody, _ := json.Marshal(execPayload)
	execReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/execute-agent", scriptTask.ID), bytes.NewReader(execBody))
	execW := httptest.NewRecorder()
	srv.Router().ServeHTTP(execW, execReq)

	if execW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on execute agent, got %d: %s", execW.Code, execW.Body.String())
	}

	var execResp struct {
		Status            string            `json:"status"`
		ExecutionStatus   string            `json:"execution_status"`
		Task              db.ProjectTaskDTO `json:"task"`
		Agent             *db.AgentDTO      `json:"agent"`
		AgentOutput       string            `json:"agent_output"`
		DoDCompletedCount int               `json:"dod_completed_count"`
		DoDTotalCount     int               `json:"dod_total_count"`
		RoomMessageID     string            `json:"room_message_id"`
	}
	if err := json.Unmarshal(execW.Body.Bytes(), &execResp); err != nil {
		t.Fatalf("failed to parse execute response json: %v", err)
	}

	if execResp.Status != "success" || execResp.ExecutionStatus != "completed" {
		t.Errorf("expected success/completed, got %s / %s", execResp.Status, execResp.ExecutionStatus)
	}
	if execResp.AgentOutput == "" {
		t.Errorf("expected non-empty agent output")
	}
	if execResp.DoDCompletedCount == 0 || execResp.DoDCompletedCount != execResp.DoDTotalCount {
		t.Errorf("expected all DoD checklist items completed, got %d/%d", execResp.DoDCompletedCount, execResp.DoDTotalCount)
	}
	if execResp.RoomMessageID == "" {
		t.Errorf("expected RoomMessageID to be returned")
	}

	// 4. Verify Task State via GET /api/tasks/{id}
	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tasks/%s", scriptTask.ID), nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 on get task, got %d", getW.Code)
	}

	var getResp struct {
		Task db.ProjectTaskDTO `json:"task"`
	}
	json.Unmarshal(getW.Body.Bytes(), &getResp)
	if getResp.Task.ExecutionStatus != "completed" {
		t.Errorf("expected persisted execution_status 'completed', got %s", getResp.Task.ExecutionStatus)
	}
	if getResp.Task.AgentOutput == "" {
		t.Errorf("expected persisted agent_output")
	}
	for _, item := range getResp.Task.DoDChecklist {
		if !item.Completed {
			t.Errorf("expected DoD item %q to be completed in DB", item.Text)
		}
	}

	// 5. Verify Room Messages via GET /api/projects/rooms/{id}/messages
	roomMsgReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/rooms/%s/messages", instResp.Room.ID), nil)
	roomMsgW := httptest.NewRecorder()
	srv.Router().ServeHTTP(roomMsgW, roomMsgReq)
	if roomMsgW.Code != http.StatusOK {
		t.Fatalf("expected 200 on get room messages, got %d", roomMsgW.Code)
	}

	var msgsResp struct {
		Messages []db.ProjectMessageDTO `json:"messages"`
	}
	json.Unmarshal(roomMsgW.Body.Bytes(), &msgsResp)
	if len(msgsResp.Messages) < 2 { // 1 welcome + 1 task completion message
		t.Errorf("expected at least 2 messages in room, got %d", len(msgsResp.Messages))
	}
	foundTaskMsg := false
	for _, m := range msgsResp.Messages {
		if strings.Contains(m.Content, "Task Execution Completed") {
			foundTaskMsg = true
			break
		}
	}
	if !foundTaskMsg {
		t.Errorf("expected completion notification message in room messages")
	}
}

func TestExecuteTaskAgent_AliasesAndValidation(t *testing.T) {
	srv := setupTestServerForCreator(t)

	// Register key
	connPayload := map[string]interface{}{
		"provider": "google",
		"api_key":  "mock-gemini-key",
	}
	connBody, _ := json.Marshal(connPayload)
	connReq := httptest.NewRequest(http.MethodPost, "/api/agent-connections", bytes.NewReader(connBody))
	srv.Router().ServeHTTP(httptest.NewRecorder(), connReq)

	// Instantiate Olshop
	instPayload := map[string]interface{}{
		"template_id":  creator.BlueprintOlshop,
		"project_name": "Digital Products Store",
	}
	instBody, _ := json.Marshal(instPayload)
	instReq := httptest.NewRequest(http.MethodPost, "/api/creator/projects/instantiate", bytes.NewReader(instBody))
	instW := httptest.NewRecorder()
	srv.Router().ServeHTTP(instW, instReq)

	var instResp struct {
		Tasks []db.ProjectTaskDTO `json:"tasks"`
	}
	json.Unmarshal(instW.Body.Bytes(), &instResp)
	targetTaskID := instResp.Tasks[0].ID

	// Test aliases:
	aliases := []string{
		fmt.Sprintf("/api/tasks/%s/execute-agent", targetTaskID),
		fmt.Sprintf("/api/projects/tasks/%s/execute-agent", targetTaskID),
		fmt.Sprintf("/api/v1/projects/tasks/%s/execute-agent", targetTaskID),
		fmt.Sprintf("/api/v1/tasks/%s/execute-agent", targetTaskID),
	}

	for _, ep := range aliases {
		t.Run(ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, ep, nil)
			w := httptest.NewRecorder()
			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("[%s] expected 200 OK, got %d: %s", ep, w.Code, w.Body.String())
			}
		})
	}

	// Test invalid UUID
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/not-a-uuid/execute-agent", nil)
	badW := httptest.NewRecorder()
	srv.Router().ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid UUID, got %d", badW.Code)
	}

	// Test non-existent task UUID
	nonExistentID := db.GenerateUUID()
	notFoundReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/execute-agent", nonExistentID), nil)
	notFoundW := httptest.NewRecorder()
	srv.Router().ServeHTTP(notFoundW, notFoundReq)
	if notFoundW.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found on non-existent task, got %d", notFoundW.Code)
	}
}

func TestExecuteTaskAgent_ZeroCredentialLeakage(t *testing.T) {
	srv := setupTestServerForCreator(t)

	secretAPIKey := "mock-AIzaSyRealSecretKeyVault999"
	connPayload := map[string]interface{}{
		"provider": "google",
		"api_key":  secretAPIKey,
	}
	connBody, _ := json.Marshal(connPayload)
	srv.Router().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/agent-connections", bytes.NewReader(connBody)))

	// Instantiate Affiliate
	instPayload := map[string]interface{}{
		"template_id":  creator.BlueprintAffiliate,
		"project_name": "Affiliate Review Site",
	}
	instBody, _ := json.Marshal(instPayload)
	instW := httptest.NewRecorder()
	srv.Router().ServeHTTP(instW, httptest.NewRequest(http.MethodPost, "/api/v1/creator/projects/instantiate", bytes.NewReader(instBody)))

	var instResp struct {
		Tasks []db.ProjectTaskDTO `json:"tasks"`
	}
	json.Unmarshal(instW.Body.Bytes(), &instResp)
	targetTaskID := instResp.Tasks[0].ID

	// Execute
	execReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/execute-agent", targetTaskID), nil)
	execW := httptest.NewRecorder()
	srv.Router().ServeHTTP(execW, execReq)

	if execW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", execW.Code, execW.Body.String())
	}

	responseString := execW.Body.String()
	if strings.Contains(responseString, secretAPIKey) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: secret API key found in HTTP response!")
	}
}

