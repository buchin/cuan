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
	r.Get("/api/v1/projects/{id}/deployment-config", s.handleGetProjectDeploymentConfig)
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

	// 6b. Test reading deployment configuration
	reqGetCfg := httptest.NewRequest("GET", "/api/v1/projects/"+projID+"/deployment-config", nil)
	recGetCfg := httptest.NewRecorder()
	r.ServeHTTP(recGetCfg, reqGetCfg)
	if recGetCfg.Code != http.StatusOK {
		t.Fatalf("expected 200 OK getting deployment config, got %d: %s", recGetCfg.Code, recGetCfg.Body.String())
	}
	var getCfgResp map[string]interface{}
	_ = json.NewDecoder(recGetCfg.Body).Decode(&getCfgResp)
	if getCfgResp["auto_deploy"] != false || getCfgResp["deploy_branch"] != "main" {
		t.Errorf("unexpected deployment config response: %+v", getCfgResp)
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

func TestProjectDeploy_SmartFallbackPipeline(t *testing.T) {
	// 1. Setup in-memory deployment repository
	depRepo := cuanDB.NewMemoryProjectDeploymentRepository()
	srv := &Server{
		projectDeploymentRepo: depRepo,
	}

	ctx := context.Background()
	projID := "proj-fallback-test"

	// Case A: Repo has scripts/deploy.sh
	tempRepoA := t.TempDir()
	scriptsDir := filepath.Join(tempRepoA, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	deploySh := filepath.Join(scriptsDir, "deploy.sh")
	if err := os.WriteFile(deploySh, []byte("#!/bin/bash\necho 'DEPLOY_VIA_REPO_SCRIPT_SUCCESS'\n"), 0755); err != nil {
		t.Fatal(err)
	}

	depA := &cuanDB.ProjectDeployment{
		ID:        "dep-fallback-a",
		ProjectID: projID,
		CommitSHA: "abcdef1234567890",
		ShortSHA:  "abcdef1",
		Status:    cuanDB.DeploymentStatusBuilding,
		Steps:     generateStandardDeploymentSteps("main", "abcdef1234567890", "feat: test", cuanDB.DeploymentStatusBuilding),
	}
	if err := depRepo.Create(ctx, depA); err != nil {
		t.Fatal(err)
	}

	projA := &project.ProjectResponse{
		ID:            projID,
		DefaultBranch: "main",
	}

	// Run pipeline synchronously for test
	srv.runProjectDeploymentPipeline(depA.ID, projID, tempRepoA, projA, "main", "abcdef1234567890")

	updatedA, err := depRepo.GetByID(ctx, depA.ID)
	if err != nil || updatedA == nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}
	if !strings.Contains(updatedA.Steps[1].Log, "DEPLOY_VIA_REPO_SCRIPT_SUCCESS") {
		t.Errorf("expected Step 2 log to contain script execution output, got: %s", updatedA.Steps[1].Log)
	}

	// Case B: Static HTML repo without deployment script
	tempRepoB := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempRepoB, "index.html"), []byte("<html><body>Hello</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
	depB := &cuanDB.ProjectDeployment{
		ID:        "dep-fallback-b",
		ProjectID: projID,
		CommitSHA: "1111222333444555",
		ShortSHA:  "1111222",
		Status:    cuanDB.DeploymentStatusBuilding,
		Steps:     generateStandardDeploymentSteps("main", "1111222333444555", "feat: static site", cuanDB.DeploymentStatusBuilding),
	}
	if err := depRepo.Create(ctx, depB); err != nil {
		t.Fatal(err)
	}

	srv.runProjectDeploymentPipeline(depB.ID, projID, tempRepoB, projA, "main", "1111222333444555")

	updatedB, err := depRepo.GetByID(ctx, depB.ID)
	if err != nil || updatedB == nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}
	if !strings.Contains(updatedB.Steps[1].Log, "Auto-detected static web project") {
		t.Errorf("expected Step 2 log to contain static detection, got: %s", updatedB.Steps[1].Log)
	}
}

func TestReconcileActiveProjectDeployments_MatchesAndCompletesSteps(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	activeSHA := "0123456789abcdef0123456789abcdef01234567"
	releaseDir := filepath.Join(tempDir, "releases", activeSHA)
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	currentLink := filepath.Join(tempDir, "current")
	if err := os.Symlink(releaseDir, currentLink); err != nil {
		t.Fatal(err)
	}

	pStore := project.NewMemoryStore()
	pSvc := project.NewService(pStore, git.NewClient(), "test-secret-key-deployments-32byte", tempDir)
	proj := &project.Project{
		ID:            "default",
		Name:          "Cuan",
		RepoName:      "cuan",
		DefaultBranch: "dev",
		LocalPath:     filepath.Join(tempDir, "cuan"),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := pStore.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}

	depRepo := cuanDB.NewMemoryProjectDeploymentRepository()
	now := time.Now().UTC()
	s1Start := now.Add(-30 * time.Second)
	s1End := now.Add(-27 * time.Second)
	s2End := now.Add(-10 * time.Second)
	s3End := now.Add(-5 * time.Second)
	s4Start := now.Add(-4 * time.Second)

	depBuilding := &cuanDB.ProjectDeployment{
		ID:            "dep_interrupted_step4",
		ProjectID:     proj.ID,
		CommitSHA:     activeSHA,
		ShortSHA:      activeSHA[:7],
		CommitMessage: "fix(deploy): reconcile interrupted self-deploy",
		Branch:        "dev",
		Status:        cuanDB.DeploymentStatusBuilding,
		TriggerSource: cuanDB.TriggerSourceWebhook,
		StartedAt:     s1Start,
		CreatedAt:     s1Start,
		UpdatedAt:     s4Start,
		Steps: []cuanDB.ProjectDeploymentStep{
			{ID: "step-1", Name: "Source Checkout", Status: cuanDB.StepStatusSuccess, Duration: "3s", StartedAt: &s1Start, CompletedAt: &s1End, Summary: "Checked out commit 0123456"},
			{ID: "step-2", Name: "Compile & Build", Status: cuanDB.StepStatusSuccess, Duration: "17s", StartedAt: &s1End, CompletedAt: &s2End, Summary: "Application compiled successfully"},
			{ID: "step-3", Name: "Database Migration", Status: cuanDB.StepStatusSuccess, Duration: "5s", StartedAt: &s2End, CompletedAt: &s3End, Summary: "Database schema synchronized and up to date"},
			{ID: "step-4", Name: "Atomic Service Switch", Status: cuanDB.StepStatusBuilding, StartedAt: &s4Start, Summary: "Performing atomic service switch..."},
			{ID: "step-5", Name: "Health Verification", Status: cuanDB.StepStatusPending, Summary: "Pending service switch"},
		},
	}
	if err := depRepo.Create(ctx, depBuilding); err != nil {
		t.Fatal(err)
	}

	depPending := &cuanDB.ProjectDeployment{
		ID:        "dep_pending_active",
		ProjectID: proj.ID,
		CommitSHA: activeSHA,
		ShortSHA:  activeSHA[:7],
		Branch:    "dev",
		Status:    "pending",
		StartedAt: s1Start.Add(-time.Minute),
		CreatedAt: s1Start.Add(-time.Minute),
		UpdatedAt: s1Start.Add(-time.Minute),
	}
	if err := depRepo.Create(ctx, depPending); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		projectSvc:            pSvc,
		projectDeploymentRepo: depRepo,
		cuanCurrentPath:       currentLink,
	}

	srv.ReconcileActiveProjectDeployments(ctx)

	got, err := depRepo.GetByID(ctx, depBuilding.ID)
	if err != nil || got == nil {
		t.Fatalf("failed to get reconciled deployment: %v", err)
	}
	if got.Status != cuanDB.DeploymentStatusSuccess {
		t.Fatalf("expected status success, got %s", got.Status)
	}
	if got.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}
	if got.DurationSeconds < 1 {
		t.Errorf("expected DurationSeconds >= 1, got %d", got.DurationSeconds)
	}
	if len(got.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(got.Steps))
	}
	for i, step := range got.Steps {
		if step.Status != cuanDB.StepStatusSuccess {
			t.Errorf("step %d (%s) expected status success, got %s", i+1, step.ID, step.Status)
		}
		if step.StartedAt == nil {
			t.Errorf("step %d (%s) expected non-nil StartedAt", i+1, step.ID)
		}
		if step.CompletedAt == nil {
			t.Errorf("step %d (%s) expected non-nil CompletedAt", i+1, step.ID)
		}
		if step.Duration == "" {
			t.Errorf("step %d (%s) expected non-empty Duration", i+1, step.ID)
		}
	}
	if got.Steps[3].Summary != "Release symlink updated and daemon reloaded" {
		t.Errorf("unexpected step-4 summary: %q", got.Steps[3].Summary)
	}
	if got.Steps[3].Log == "" {
		t.Errorf("expected step-4 log to be populated")
	}
	if got.Steps[4].Summary != "Live service health check passed" {
		t.Errorf("unexpected step-5 summary: %q", got.Steps[4].Summary)
	}
	if got.Steps[4].Log == "" {
		t.Errorf("expected step-5 log to be populated")
	}

	gotPending, err := depRepo.GetByID(ctx, depPending.ID)
	if err != nil || gotPending == nil {
		t.Fatalf("failed to get pending deployment: %v", err)
	}
	if gotPending.Status != cuanDB.DeploymentStatusSuccess {
		t.Fatalf("expected pending deployment for active commit to become success, got %s", gotPending.Status)
	}
	if len(gotPending.Steps) != 5 {
		t.Errorf("expected 5 generated steps for pending deployment, got %d", len(gotPending.Steps))
	}
}

func TestReconcileActiveProjectDeployments_SkipsMismatchCommit(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	activeSHA := "0123456789abcdef0123456789abcdef01234567"
	otherSHA := "ffffffffffffffffffffffffffffffffffffffff"
	releaseDir := filepath.Join(tempDir, "releases", activeSHA)
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	currentLink := filepath.Join(tempDir, "current")
	if err := os.Symlink(releaseDir, currentLink); err != nil {
		t.Fatal(err)
	}

	pStore := project.NewMemoryStore()
	pSvc := project.NewService(pStore, git.NewClient(), "test-secret-key-deployments-32byte", tempDir)
	proj := &project.Project{
		ID:            "default",
		Name:          "Cuan",
		RepoName:      "cuan",
		DefaultBranch: "dev",
		LocalPath:     filepath.Join(tempDir, "cuan"),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := pStore.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}

	depRepo := cuanDB.NewMemoryProjectDeploymentRepository()
	now := time.Now().UTC()
	depMismatch := &cuanDB.ProjectDeployment{
		ID:        "dep_mismatch_building",
		ProjectID: proj.ID,
		CommitSHA: otherSHA,
		ShortSHA:  otherSHA[:7],
		Branch:    "dev",
		Status:    cuanDB.DeploymentStatusBuilding,
		StartedAt: now.Add(-10 * time.Second),
		CreatedAt: now.Add(-10 * time.Second),
		UpdatedAt: now.Add(-10 * time.Second),
		Steps:     generateStandardDeploymentSteps("dev", otherSHA, "feat: other", cuanDB.DeploymentStatusBuilding),
	}
	if err := depRepo.Create(ctx, depMismatch); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		projectSvc:            pSvc,
		projectDeploymentRepo: depRepo,
		cuanCurrentPath:       currentLink,
	}

	srv.ReconcileActiveProjectDeployments(ctx)

	got, err := depRepo.GetByID(ctx, depMismatch.ID)
	if err != nil || got == nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if got.Status != cuanDB.DeploymentStatusBuilding {
		t.Fatalf("expected mismatch commit deployment to remain building, got %s", got.Status)
	}
	if got.CompletedAt != nil {
		t.Fatalf("expected mismatch commit deployment CompletedAt to remain nil")
	}
}

func TestIsCuanProject_ExcludesSiblingProjects(t *testing.T) {
	cases := []struct {
		name string
		proj *project.ProjectResponse
		want bool
	}{
		{name: "nil", proj: nil, want: false},
		{name: "default id", proj: &project.ProjectResponse{ID: "default", Name: "Other"}, want: true},
		{name: "exact repo", proj: &project.ProjectResponse{ID: "proj_1", Name: "My App", RepoName: "cuan"}, want: true},
		{name: "exact name", proj: &project.ProjectResponse{ID: "proj_2", Name: "Cuan"}, want: true},
		{name: "cuan-landing", proj: &project.ProjectResponse{ID: "proj_556a", Name: "cuan-landing", RepoName: "cuan-landing"}, want: false},
		{name: "cuan-docs", proj: &project.ProjectResponse{ID: "proj_docs", Name: "cuan-docs", RepoName: "cuan-docs"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCuanProject(tc.proj); got != tc.want {
				t.Fatalf("isCuanProject(%+v) = %v, want %v", tc.proj, got, tc.want)
			}
		})
	}
}

func TestRunDeployBuildCommand_ActivityExtendsBeyondIdleAndKillsStalled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Active output every 50ms for 250ms must survive a 120ms idle timeout.
	out, err := runDeployBuildCommand(ctx, "bash", []string{"-c", "for i in 1 2 3 4 5; do sleep 0.05; echo tick-$i; done"}, t.TempDir(), nil, 120*time.Millisecond)
	if err != nil {
		t.Fatalf("expected active build command to succeed beyond idle timeout, got err=%v out=%s", err, string(out))
	}
	if !strings.Contains(string(out), "tick-5") {
		t.Fatalf("expected full streamed output, got %q", string(out))
	}

	// Silent build exceeding idle timeout must be killed with stalled error.
	_, stallErr := runDeployBuildCommand(ctx, "bash", []string{"-c", "echo starting; sleep 2"}, t.TempDir(), nil, 100*time.Millisecond)
	if stallErr == nil || !strings.Contains(stallErr.Error(), "build stalled") {
		t.Fatalf("expected build stalled error, got %v", stallErr)
	}
}
