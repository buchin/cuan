package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
)

func setupTestServerWithAuth(t *testing.T) (*Server, string, *httptest.Server) {
	// Mock GitHub API
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/buchin/cuan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"full_name": "buchin/cuan",
			"default_branch": "main",
			"private": false,
			"clone_url": "https://github.com/buchin/cuan.git"
		}`))
	})
	mux.HandleFunc("/repos/buchin/private-app", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer ghp_valid_super_secret_pat_987654" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"full_name": "buchin/private-app",
			"default_branch": "main",
			"private": true,
			"clone_url": "https://github.com/buchin/private-app.git"
		}`))
	})
	mux.HandleFunc("/repos/buchin/denied-repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message": "Access forbidden"}`))
	})
	mux.HandleFunc("/repos/buchin/missing-repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	})

	mockGH := httptest.NewServer(mux)
	t.Cleanup(mockGH.Close)

	gitClient := git.NewClient()
	gitClient.SetAPIBaseURL(mockGH.URL)

	cfg := &config.Config{
		AuthSecret: "server-test-secret-32-bytes-long",
	}
	memAuth := auth.NewMemoryStore()
	authSvc := auth.NewService(memAuth)

	memProject := project.NewMemoryStore()
	projectSvc := project.NewService(memProject, gitClient, cfg.AuthSecret, t.TempDir())

	srv := NewServer(cfg, nil, authSvc)
	srv.SetProjectService(projectSvc)

	// Create owner and login to get token
	setupResp, err := authSvc.Setup(context.Background(), "owner@example.com", "validPassword123")
	if err != nil {
		t.Fatalf("failed to setup owner: %v", err)
	}

	return srv, setupResp.Token, mockGH
}

func TestProjectsAPI_Unauthenticated(t *testing.T) {
	srv, _, _ := setupTestServerWithAuth(t)

	// GET /api/v1/projects -> 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// POST /api/v1/projects -> 401
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"repo_url":"buchin/cuan"}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestProjectsAPI_ConnectAndListAndGet(t *testing.T) {
	srv, token, _ := setupTestServerWithAuth(t)

	// 1. Connect public repo
	payload := []byte(`{
		"name": "Cuan Project",
		"repo_url": "https://github.com/buchin/cuan",
		"default_branch": "main"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var created project.ProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse created project response: %v", err)
	}

	if created.ID == "" || created.Name != "Cuan Project" || created.DefaultBranch != "main" {
		t.Fatalf("unexpected project details: %+v", created)
	}

	// 2. List projects
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var listResp struct {
		Projects []project.ProjectResponse `json:"projects"`
		Total    int                       `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}

	if listResp.Total != 1 || len(listResp.Projects) != 1 {
		t.Fatalf("expected 1 project in list, got %d", listResp.Total)
	}

	// 3. Get Project Detail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for project detail, got %d", rr.Code)
	}

	// 4. Get Branches
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.ID+"/branches", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for branches, got %d", rr.Code)
	}

	// 5. Delete project from Cuan
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d", rr.Code)
	}

	// Verify deleted from Cuan
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found after delete, got %d", rr.Code)
	}
}

func TestProjectsAPI_PrivateRepo_TokenMaskedAndEncrypted(t *testing.T) {
	srv, token, _ := setupTestServerWithAuth(t)

	rawSecretToken := "ghp_valid_super_secret_pat_987654"
	payload := []byte(`{
		"name": "Private App",
		"repo_url": "buchin/private-app",
		"token": "` + rawSecretToken + `"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// Assert response body does NOT contain the plaintext token
	respBody := rr.Body.String()
	if strings.Contains(respBody, rawSecretToken) {
		t.Fatalf("CRITICAL SECURITY FLAW: API response leaked raw plaintext PAT: %s", respBody)
	}

	var pResp project.ProjectResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &pResp)
	if !pResp.HasToken {
		t.Fatalf("expected HasToken true")
	}
	if pResp.TokenPreview != "ghp_***7654" {
		t.Fatalf("expected preview 'ghp_***7654', got %q", pResp.TokenPreview)
	}
}

func TestProjectsAPI_ErrorHandling_Informative(t *testing.T) {
	srv, token, _ := setupTestServerWithAuth(t)

	// 1. Repo not found (404)
	payload := []byte(`{"repo_url": "buchin/missing-repo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Repository not found") {
		t.Fatalf("expected informative message in body, got %s", rr.Body.String())
	}

	// 2. Access denied / bad token (403/401)
	payload = []byte(`{"repo_url": "buchin/private-app", "token": "invalid_pat"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for bad token, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "could not grant access to this repository") {
		t.Fatalf("expected informative access denied message, got %s", rr.Body.String())
	}

	// 3. Forbidden repo directly
	payload = []byte(`{"repo_url": "buchin/denied-repo"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestProjectsAPI_CreateManagedWorkspace_EmptyRepoURL(t *testing.T) {
	srv, token, _ := setupTestServerWithAuth(t)

	// 1. Missing name with empty repo_url -> 400 Bad Request
	payload := []byte(`{"name": ""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty name, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "project name is required") {
		t.Fatalf("expected 'project name is required' error, got %s", rr.Body.String())
	}

	// 2. Valid name with empty repo_url -> 201 Created (Managed Local Workspace)
	payload = []byte(`{"name": "Toko Online Budi"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var created project.ProjectResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.HasPrefix(created.ID, "proj_") || len(created.ID) != 37 {
		t.Fatalf("expected managed project ID of length 37 starting with proj_, got %q", created.ID)
	}
	if created.Name != "Toko Online Budi" {
		t.Fatalf("expected name 'Toko Online Budi', got %q", created.Name)
	}

	// 3. Verify project is retrievable via GET /api/v1/projects/{id}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK retrieving created managed project, got %d", rr.Code)
	}
}
