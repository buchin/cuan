package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func setupArchiveTestServer(t *testing.T) (*Server, string, *project.Service, db.ProjectRepository, db.TaskProjectRepository) {
	cfg := &config.Config{
		AuthSecret: "super-secret-auth-key-32-bytes-long",
	}

	authMem := auth.NewMemoryStore()
	authSvc := auth.NewService(authMem)
	setupResp, err := authSvc.Setup(context.Background(), "owner@cuan.dev", "superpassword123")
	if err != nil {
		t.Fatalf("auth setup failed: %v", err)
	}

	projMem := project.NewMemoryStore()
	projSvc := project.NewService(projMem, git.NewClient(), cfg.AuthSecret, t.TempDir())

	projRepo := db.NewMemoryProjectRepository()
	taskProjRepo := db.NewMemoryTaskProjectRepository()
	taskMem := task.NewMemoryStore()
	taskSvc := task.NewService(taskMem, projSvc)

	srv := NewServer(cfg, nil, authSvc)
	srv.SetProjectService(projSvc)
	srv.SetProjectRepository(projRepo)
	srv.SetTaskProjectRepository(taskProjRepo)
	srv.SetTaskService(taskSvc)

	return srv, setupResp.Token, projSvc, projRepo, taskProjRepo
}

func TestProjectArchiveAndUnarchiveEndpoints(t *testing.T) {
	srv, token, projSvc, _, _ := setupArchiveTestServer(t)

	// Seed 2 projects directly into store
	now := time.Now().UTC()
	p1 := &project.Project{
		ID:            "proj_alpha",
		Name:          "Project Alpha",
		RepoURL:       "https://github.com/buchin/alpha",
		DefaultBranch: "main",
		CloneStatus:   "synced",
		IsArchived:    false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	p2 := &project.Project{
		ID:            "proj_beta",
		Name:          "Project Beta",
		RepoURL:       "https://github.com/buchin/beta",
		DefaultBranch: "main",
		CloneStatus:   "synced",
		IsArchived:    false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_ = projSvc.Store().CreateProject(context.Background(), p1)
	_ = projSvc.Store().CreateProject(context.Background(), p2)

	// 1. Initial list: both must be returned as active
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var listResp struct {
		Projects []*project.ProjectResponse `json:"projects"`
		Total    int                        `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if listResp.Total != 2 {
		t.Fatalf("expected 2 active projects, got %d", listResp.Total)
	}

	// 2. Archive proj_alpha via PATCH /api/v1/projects/proj_alpha/archive
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/projects/proj_alpha/archive", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from archive, got %d: %s", rr.Code, rr.Body.String())
	}
	var archivedResp project.ProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &archivedResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if !archivedResp.IsArchived {
		t.Fatalf("expected is_archived to be true")
	}

	// 3. GET /api/v1/projects (default) -> should now only return proj_beta (is_archived = false)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &listResp)
	if listResp.Total != 1 || listResp.Projects[0].ID != "proj_beta" {
		t.Fatalf("expected 1 active project (proj_beta), got %d: %+v", listResp.Total, listResp.Projects)
	}

	// 4. GET /api/v1/projects?archived=true -> should return proj_alpha only
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects?archived=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &listResp)
	if listResp.Total != 1 || listResp.Projects[0].ID != "proj_alpha" {
		t.Fatalf("expected 1 archived project (proj_alpha), got %d: %+v", listResp.Total, listResp.Projects)
	}

	// 5. GET /api/v1/projects?include_archived=true -> should return both (2)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects?include_archived=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &listResp)
	if listResp.Total != 2 {
		t.Fatalf("expected 2 projects with include_archived=true, got %d", listResp.Total)
	}

	// 6. Unarchive proj_alpha via PATCH /api/v1/projects/proj_alpha/unarchive
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/projects/proj_alpha/unarchive", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from unarchive, got %d: %s", rr.Code, rr.Body.String())
	}
	var unarchivedResp project.ProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &unarchivedResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if unarchivedResp.IsArchived {
		t.Fatalf("expected is_archived to be false after unarchive")
	}

	// 7. GET /api/v1/projects (default) -> both must be back in active list
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	_ = json.Unmarshal(rr.Body.Bytes(), &listResp)
	if listResp.Total != 2 {
		t.Fatalf("expected 2 active projects after unarchive, got %d", listResp.Total)
	}
}

func TestMultiProjectTaskLinking(t *testing.T) {
	srv, token, projSvc, _, taskProjRepo := setupArchiveTestServer(t)

	// Seed 3 projects
	now := time.Now().UTC()
	for _, id := range []string{"proj_a", "proj_b", "proj_c"} {
		_ = projSvc.Store().CreateProject(context.Background(), &project.Project{
			ID:          id,
			Name:        "Project " + id,
			RepoURL:     "https://github.com/buchin/" + id,
			CloneStatus: "synced",
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Create task linking proj_a, proj_b, and proj_c
	createBody := map[string]interface{}{
		"title":       "Mass Rollout Auto-Ads 2026",
		"description": "Deploy adsense script across network",
		"project_id":  "proj_a",
		"project_ids": []string{"proj_a", "proj_b", "proj_c"},
	}
	raw, _ := json.Marshal(createBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rr.Code, rr.Body.String())
	}

	var created task.TaskItemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode task: %v", err)
	}

	if len(created.ProjectIDs) != 3 {
		t.Fatalf("expected 3 linked project IDs, got: %+v", created.ProjectIDs)
	}

	// Verify relational store task_projects has the links
	linked, err := taskProjRepo.GetProjectsByTask(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("failed to query taskProjRepo: %v", err)
	}
	if len(linked) != 3 {
		t.Fatalf("expected 3 projects in task_projects repo, got: %+v", linked)
	}

	// Filter tasks by project_ids: ?project_ids=proj_b
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks?project_ids=proj_b", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var filterResp struct {
		Tasks []*task.TaskItemResponse `json:"tasks"`
		Total int                      `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &filterResp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if filterResp.Total != 1 || filterResp.Tasks[0].ID != created.ID {
		t.Fatalf("expected 1 task matching proj_b, got %d", filterResp.Total)
	}
}
