package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
)

func setupTestDeployServer(t *testing.T) (*Server, string) {
	t.Helper()
	tempDir := t.TempDir()

	cfg := &config.Config{
		ProjectsDir: tempDir,
		AuthSecret:  "test-secret-key-deployments-32byte",
	}

	pStore := project.NewMemoryStore()
	pSvc := project.NewService(pStore, git.NewClient(), cfg.AuthSecret, tempDir)

	// Create test project
	proj := &project.Project{
		ID:            "proj-test-1",
		Name:          "Test Deployment Project",
		DefaultBranch: "main",
		LocalPath:     filepath.Join(tempDir, "proj-test-1"),
		ProjectType:   "custom",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	_ = os.MkdirAll(proj.LocalPath, 0755)
	if err := pStore.CreateProject(context.Background(), proj); err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	depRepo := cuanDB.NewMemoryProjectDeploymentRepository()

	s := &Server{
		cfg:                   cfg,
		projectSvc:            pSvc,
		projectDeploymentRepo: depRepo,
	}

	return s, proj.ID
}

func TestProjectDeployHandlers_GetDeploymentsEmptyAndSeed(t *testing.T) {
	s, projID := setupTestDeployServer(t)

	r := chi.NewRouter()
	r.Get("/api/v1/projects/{id}/deployments", s.handleGetProjectDeployments)

	req := httptest.NewRequest("GET", "/api/v1/projects/"+projID+"/deployments", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ProjectDeploymentsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ProjectID != projID {
		t.Errorf("expected project ID %s, got %s", projID, resp.ProjectID)
	}
}

func TestProjectDeployHandlers_TriggerAndGetDetail(t *testing.T) {
	s, projID := setupTestDeployServer(t)

	r := chi.NewRouter()
	r.Post("/api/v1/projects/{id}/deployments", s.handleTriggerProjectDeployment)
	r.Get("/api/v1/projects/{id}/deployments/{dep_id}", s.handleGetProjectDeploymentDetail)
	r.Post("/api/v1/projects/{id}/deployments/{dep_id}/cancel", s.handleCancelProjectDeployment)

	// 1. Trigger deployment
	body := bytes.NewBufferString(`{"branch":"main","commit_sha":"abcdef1234567","trigger_source":"manual"}`)
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/deployments", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var triggerResp struct {
		Deployment ProjectDeploymentItem `json:"deployment"`
		Message    string                `json:"message"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed to decode trigger response: %v", err)
	}

	depID := triggerResp.Deployment.ID
	if depID == "" {
		t.Fatal("expected non-empty deployment ID")
	}
	if triggerResp.Deployment.ShortSHA != "abcdef1" {
		t.Errorf("expected short SHA abcdef1, got %s", triggerResp.Deployment.ShortSHA)
	}
	if len(triggerResp.Deployment.Steps) != 5 {
		t.Errorf("expected 5 steps, got %d", len(triggerResp.Deployment.Steps))
	}

	// 2. Get Detail
	reqDetail := httptest.NewRequest("GET", "/api/v1/projects/"+projID+"/deployments/"+depID, nil)
	recDetail := httptest.NewRecorder()
	r.ServeHTTP(recDetail, reqDetail)

	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on detail, got %d: %s", recDetail.Code, recDetail.Body.String())
	}

	var detailResp struct {
		Deployment ProjectDeploymentItem `json:"deployment"`
	}
	if err := json.NewDecoder(recDetail.Body).Decode(&detailResp); err != nil {
		t.Fatalf("failed to decode detail response: %v", err)
	}
	if detailResp.Deployment.ID != depID {
		t.Errorf("expected deployment ID %s, got %s", depID, detailResp.Deployment.ID)
	}

	// 3. Cancel
	reqCancel := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/deployments/"+depID+"/cancel", nil)
	recCancel := httptest.NewRecorder()
	r.ServeHTTP(recCancel, reqCancel)

	if recCancel.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on cancel, got %d: %s", recCancel.Code, recCancel.Body.String())
	}
}

func TestProjectDeployWebhook_UnauthorizedAndSuccess(t *testing.T) {
	s, projID := setupTestDeployServer(t)

	r := chi.NewRouter()
	r.Post("/api/v1/projects/{id}/webhooks/deploy", s.handleProjectDeployWebhook)
	r.Get("/api/v1/projects/{id}/deployments", s.handleGetProjectDeployments)
	r.Put("/api/v1/projects/{id}/deployment-config", s.handleUpdateProjectDeploymentConfig)

	proj, err := s.projectSvc.Get(context.Background(), projID)
	if err != nil {
		t.Fatalf("failed to get project: %v", err)
	}
	secret := s.resolveProjectDeployWebhookSecret(proj)

	// 1. Missing secret -> 401
	payload := `{"ref":"refs/heads/main","after":"1111222233334444555566667777888899990000","head_commit":{"id":"1111222233334444555566667777888899990000","message":"feat: add queue","author":{"name":"Buchin"}}}`
	reqUnauth := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy", bytes.NewBufferString(payload))
	recUnauth := httptest.NewRecorder()
	r.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without secret, got %d", recUnauth.Code)
	}

	// 2. Unmatched branch -> 200 ignored
	wrongBranchPayload := `{"ref":"refs/heads/feature/awesome","after":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	reqWrong := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy?secret="+secret, bytes.NewBufferString(wrongBranchPayload))
	recWrong := httptest.NewRecorder()
	r.ServeHTTP(recWrong, reqWrong)
	if recWrong.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on ignored branch, got %d", recWrong.Code)
	}
	var ignoredResp map[string]interface{}
	_ = json.NewDecoder(recWrong.Body).Decode(&ignoredResp)
	if ignoredResp["status"] != "ignored" {
		t.Errorf("expected status 'ignored', got %v", ignoredResp["status"])
	}

	// 3. Valid secret with matching branch -> 201 Created & building
	reqValid := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy?secret="+secret, bytes.NewBufferString(payload))
	recValid := httptest.NewRecorder()
	r.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on valid webhook, got %d: %s", recValid.Code, recValid.Body.String())
	}
	var createdResp struct {
		Deployment ProjectDeploymentItem `json:"deployment"`
		Message    string                `json:"message"`
	}
	if err := json.NewDecoder(recValid.Body).Decode(&createdResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if createdResp.Deployment.Status != cuanDB.DeploymentStatusBuilding {
		t.Errorf("expected initial deployment status building, got %s", createdResp.Deployment.Status)
	}
	if createdResp.Deployment.TriggerSource != cuanDB.TriggerSourceWebhook {
		t.Errorf("expected trigger source webhook, got %s", createdResp.Deployment.TriggerSource)
	}

	// 4. Second push while first is building -> enqueued as queued
	payload2 := `{"ref":"refs/heads/main","after":"2222333344445555666677778888999900001111","head_commit":{"id":"2222333344445555666677778888999900001111","message":"feat: second push","author":{"name":"Buchin"}}}`
	reqSecond := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy?secret="+secret, bytes.NewBufferString(payload2))
	recSecond := httptest.NewRecorder()
	r.ServeHTTP(recSecond, reqSecond)
	if recSecond.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on second push, got %d: %s", recSecond.Code, recSecond.Body.String())
	}
	var secondResp struct {
		Deployment ProjectDeploymentItem `json:"deployment"`
		Message    string                `json:"message"`
	}
	if err := json.NewDecoder(recSecond.Body).Decode(&secondResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if secondResp.Deployment.Status != cuanDB.DeploymentStatusQueued {
		t.Errorf("expected second deployment status queued, got %s", secondResp.Deployment.Status)
	}

	// 5. GitHub HMAC Signature validation
	payload3 := `{"ref":"refs/heads/main","after":"3333444455556666777788889999000011112222"}`
	bodyBytes := []byte(payload3)
	sig := "sha256=" + computeHmacHex([]byte(secret), bodyBytes)

	reqHMAC := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy", bytes.NewReader(bodyBytes))
	reqHMAC.Header.Set("X-Hub-Signature-256", sig)
	recHMAC := httptest.NewRecorder()
	r.ServeHTTP(recHMAC, reqHMAC)
	if recHMAC.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created with valid GitHub HMAC, got %d: %s", recHMAC.Code, recHMAC.Body.String())
	}

	// Verify webhook URL and secret in GET deployments response
	reqGet := httptest.NewRequest("GET", "/api/v1/projects/"+projID+"/deployments", nil)
	recGet := httptest.NewRecorder()
	r.ServeHTTP(recGet, reqGet)
	var getResp ProjectDeploymentsResponse
	_ = json.NewDecoder(recGet.Body).Decode(&getResp)
	if getResp.WebhookSecret == "" {
		t.Error("expected non-empty webhook secret in deployments response")
	}
	if !strings.Contains(getResp.WebhookURL, "/webhooks/deploy") {
		t.Errorf("expected valid webhook URL, got %s", getResp.WebhookURL)
	}

	// 6. Test updating deployment configuration (auto_deploy = false & custom deploy script)
	cfgPayload := `{"auto_deploy":false,"deploy_branch":"main","deploy_script":"./scripts/deploy.sh --branch main"}`
	reqCfg := httptest.NewRequest("PUT", "/api/v1/projects/"+projID+"/deployment-config", bytes.NewReader([]byte(cfgPayload)))
	recCfg := httptest.NewRecorder()
	r.ServeHTTP(recCfg, reqCfg)
	if recCfg.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating deployment config, got %d: %s", recCfg.Code, recCfg.Body.String())
	}

	// 7. Verify Webhook push is ignored when auto_deploy is false
	reqDisabled := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy", bytes.NewReader([]byte(`{"ref":"refs/heads/main","after":"4444555566667777888899990000111122223333"}`)))
	reqDisabled.Header.Set("X-Cuan-Deploy-Secret", secret)
	recDisabled := httptest.NewRecorder()
	r.ServeHTTP(recDisabled, reqDisabled)
	if recDisabled.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for disabled auto deploy, got %d: %s", recDisabled.Code, recDisabled.Body.String())
	}
	var disabledResp map[string]interface{}
	_ = json.NewDecoder(recDisabled.Body).Decode(&disabledResp)
	if disabledResp["status"] != "ignored" {
		t.Errorf("expected status 'ignored' when auto deploy is disabled, got %v", disabledResp["status"])
	}

	// 8. GitHub Ping event handling
	pingPayload := []byte(`{"zen":"Speak like a human.","hook_id":12345}`)
	pingSig := "sha256=" + computeHmacHex([]byte(secret), pingPayload)
	reqPing := httptest.NewRequest("POST", "/api/v1/projects/"+projID+"/webhooks/deploy", bytes.NewReader(pingPayload))
	reqPing.Header.Set("X-Hub-Signature-256", pingSig)
	reqPing.Header.Set("X-Github-Event", "ping")
	recPing := httptest.NewRecorder()
	r.ServeHTTP(recPing, reqPing)
	if recPing.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GitHub ping event, got %d: %s", recPing.Code, recPing.Body.String())
	}
	var pingResp map[string]interface{}
	_ = json.NewDecoder(recPing.Body).Decode(&pingResp)
	if pingResp["event"] != "ping" || pingResp["status"] != "ok" {
		t.Errorf("expected status 'ok' and event 'ping', got %+v", pingResp)
	}
}
