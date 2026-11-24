package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
