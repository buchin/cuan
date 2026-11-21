package project

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/git"
)

func setupMockGitHub(t *testing.T) (*httptest.Server, *git.Client) {
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

	mux.HandleFunc("/repos/buchin/private-repo", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid_pat_token_123456789" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"full_name": "buchin/private-repo",
			"default_branch": "main",
			"private": true,
			"clone_url": "https://github.com/buchin/private-repo.git"
		}`))
	})

	mux.HandleFunc("/repos/buchin/forbidden-repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message": "Resource not accessible by integration"}`))
	})

	mux.HandleFunc("/repos/buchin/nonexistent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := git.NewClient()
	client.SetAPIBaseURL(server.URL)

	return server, client
}

func TestProjectService_ConnectPublicRepo_Success(t *testing.T) {
	_, client := setupMockGitHub(t)
	store := NewMemoryStore()
	secret := "test-secret-key-for-encryption"
	tempDir := t.TempDir()

	svc := NewService(store, client, secret, tempDir)
	ctx := context.Background()

	req := ConnectProjectRequest{
		Name:    "Cuan Public",
		RepoURL: "https://github.com/buchin/cuan",
	}

	res, err := svc.Connect(ctx, req)
	if err != nil {
		t.Fatalf("unexpected connect error: %v", err)
	}

	if res.ID == "" {
		t.Fatalf("expected non-empty project ID")
	}
	if res.Name != "Cuan Public" {
		t.Fatalf("expected name 'Cuan Public', got %q", res.Name)
	}
	if res.DefaultBranch != "main" {
		t.Fatalf("expected default branch 'main', got %q", res.DefaultBranch)
	}
	if res.HasToken {
		t.Fatalf("expected HasToken false for public repo without token")
	}

	// Verify project in store
	saved, err := store.GetProject(ctx, res.ID)
	if err != nil {
		t.Fatalf("failed to retrieve project from store: %v", err)
	}
	if saved.RepoOwner != "buchin" || saved.RepoName != "cuan" {
		t.Fatalf("unexpected saved repo fields: %+v", saved)
	}
}

func TestProjectService_ConnectPrivateRepo_SafeTokenStorage(t *testing.T) {
	_, client := setupMockGitHub(t)
	store := NewMemoryStore()
	secret := "test-secret-key-for-encryption"
	tempDir := t.TempDir()

	svc := NewService(store, client, secret, tempDir)
	ctx := context.Background()

	rawToken := "valid_pat_token_123456789"
	req := ConnectProjectRequest{
		Name:    "Cuan Private",
		RepoURL: "buchin/private-repo",
		Token:   rawToken,
	}

	res, err := svc.Connect(ctx, req)
	if err != nil {
		t.Fatalf("unexpected connect error: %v", err)
	}

	if !res.HasToken {
		t.Fatalf("expected HasToken true")
	}
	// Plaintext token MUST NOT be exposed in response
	if strings.Contains(res.TokenPreview, rawToken) {
		t.Fatalf("response leaked plaintext token: %s", res.TokenPreview)
	}
	if !strings.HasPrefix(res.TokenPreview, "***") && !strings.HasPrefix(res.TokenPreview, "ghp_") {
		t.Fatalf("unexpected token preview format: %s", res.TokenPreview)
	}

	// Verify stored entity
	saved, err := store.GetProject(ctx, res.ID)
	if err != nil {
		t.Fatalf("failed to retrieve project from store: %v", err)
	}

	// Encrypted token must NOT match raw token
	if saved.EncryptedToken == rawToken {
		t.Fatalf("encrypted token was stored in plain text!")
	}

	// Decrypting stored token must recover original raw token
	decrypted, err := crypto.Decrypt(saved.EncryptedToken, secret)
	if err != nil {
		t.Fatalf("failed to decrypt stored token: %v", err)
	}
	if decrypted != rawToken {
		t.Fatalf("expected decrypted token %q, got %q", rawToken, decrypted)
	}
}

func TestProjectService_Connect_ErrorsHandled(t *testing.T) {
	_, client := setupMockGitHub(t)
	store := NewMemoryStore()
	secret := "test-secret"
	svc := NewService(store, client, secret, t.TempDir())
	ctx := context.Background()

	// 1. Invalid URL
	_, err := svc.Connect(ctx, ConnectProjectRequest{RepoURL: "invalid://url"})
	if err != git.ErrInvalidRepoURL {
		t.Fatalf("expected ErrInvalidRepoURL, got %v", err)
	}

	// 2. 404 Repo Not Found
	_, err = svc.Connect(ctx, ConnectProjectRequest{RepoURL: "buchin/nonexistent"})
	if err != git.ErrRepoNotFound {
		t.Fatalf("expected ErrRepoNotFound, got %v", err)
	}

	// 3. 401 Access Denied (invalid token)
	_, err = svc.Connect(ctx, ConnectProjectRequest{
		RepoURL: "buchin/private-repo",
		Token:   "wrong_token",
	})
	if err != git.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}

	// 4. 403 Forbidden
	_, err = svc.Connect(ctx, ConnectProjectRequest{
		RepoURL: "buchin/forbidden-repo",
		Token:   "any_token",
	})
	if err != git.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}

	// 5. Duplicate connect
	_, err = svc.Connect(ctx, ConnectProjectRequest{RepoURL: "buchin/cuan"})
	if err != nil {
		t.Fatalf("initial connect failed: %v", err)
	}
	_, err = svc.Connect(ctx, ConnectProjectRequest{RepoURL: "https://github.com/buchin/cuan.git"})
	if err != ErrProjectAlreadyExists {
		t.Fatalf("expected ErrProjectAlreadyExists on duplicate, got %v", err)
	}
}

func TestProjectService_Delete_DoesNotDamageRemoteRepo(t *testing.T) {
	_, client := setupMockGitHub(t)
	store := NewMemoryStore()
	tempDir := t.TempDir()
	svc := NewService(store, client, "secret", tempDir)
	ctx := context.Background()

	res, err := svc.Connect(ctx, ConnectProjectRequest{
		RepoURL: "buchin/cuan",
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	// Create a dummy local path file
	dummyLocal := filepath.Join(tempDir, res.ID)
	_ = os.MkdirAll(dummyLocal, 0755)
	_ = os.WriteFile(filepath.Join(dummyLocal, "local.txt"), []byte("local data"), 0644)

	// Perform project deletion in Cuan
	err = svc.Delete(ctx, res.ID)
	if err != nil {
		t.Fatalf("delete project failed: %v", err)
	}

	// Ensure removed from Cuan store
	_, err = svc.Get(ctx, res.ID)
	if err != ErrProjectNotFound {
		t.Fatalf("expected ErrProjectNotFound, got %v", err)
	}

	// Local cache should be removed
	if _, err := os.Stat(dummyLocal); !os.IsNotExist(err) {
		t.Fatalf("expected local path to be deleted")
	}

	// Remote repository on GitHub was NOT deleted or touched
	// Mock server will still return OK for buchin/cuan
	info, err := client.ValidateRepository(ctx, "buchin/cuan", "")
	if err != nil || info == nil {
		t.Fatalf("remote repo should remain accessible after project delete, got err: %v", err)
	}
}

func TestProject_ModularWorkspaceAndIntegrations(t *testing.T) {
	_, client := setupMockGitHub(t)
	store := NewMemoryStore()
	tempDir := t.TempDir()
	svc := NewService(store, client, "secret", tempDir)
	ctx := context.Background()

	// 1. Connect project with modular fields
	res, err := svc.Connect(ctx, ConnectProjectRequest{
		RepoURL:        "buchin/cuan",
		SourceType:     "template",
		EnabledModules: []byte(`["tasks", "discussions", "servers"]`),
	})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	if res.SourceType != "template" {
		t.Errorf("expected source_type template, got %s", res.SourceType)
	}
	if !strings.Contains(string(res.EnabledModules), "servers") {
		t.Errorf("expected enabled_modules to contain servers, got %s", string(res.EnabledModules))
	}

	// 2. Integration model and response check
	in := &ProjectIntegration{
		ID:              "integ-wp-1",
		ProjectID:       res.ID,
		IntegrationType: "wordpress",
		ConfigEncrypted: "encrypted-aes-gcm-data",
		Status:          "active",
	}
	inResp := in.ToResponse()
	if inResp.ID != in.ID {
		t.Errorf("expected id %s, got %s", in.ID, inResp.ID)
	}
	if inResp.IntegrationType != "wordpress" {
		t.Errorf("expected type wordpress, got %s", inResp.IntegrationType)
	}
	if !inResp.HasConfig {
		t.Errorf("expected HasConfig true")
	}
}

func TestProjectService_ListFiltered_SecurityAndFiltering(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, nil, "secret", t.TempDir())
	ctx := context.Background()

	pActive := &Project{
		ID:             "proj-active-1",
		Name:           "Active Project",
		RepoURL:        "https://github.com/buchin/active",
		Metadata:       []byte(`{"env":{"SENSITIVE_KEY":"super-secret-token"}}`),
		SourceMetadata: []byte(`{"raw_credential":"secret-password"}`),
		IsArchived:     false,
	}
	pArchived := &Project{
		ID:         "proj-archived-1",
		Name:       "Archived Project",
		RepoURL:    "https://github.com/buchin/archived",
		IsArchived: true,
	}

	if err := store.CreateProject(ctx, pActive); err != nil {
		t.Fatalf("create active project: %v", err)
	}
	if err := store.CreateProject(ctx, pArchived); err != nil {
		t.Fatalf("create archived project: %v", err)
	}

	// 1. Filter Active (default)
	activeItems, err := svc.ListFiltered(ctx, ArchivedFilterActive)
	if err != nil {
		t.Fatalf("ListFiltered active failed: %v", err)
	}
	if len(activeItems) != 1 || activeItems[0].ID != "proj-active-1" {
		t.Fatalf("expected only active project, got %d items", len(activeItems))
	}

	// 2. Security check: overfetching & sensitive env/token leak prevention
	data, err := json.Marshal(activeItems[0])
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	dataStr := string(data)
	if strings.Contains(dataStr, "super-secret-token") || strings.Contains(dataStr, "secret-password") {
		t.Fatalf("security violation: sensitive payload leaked in ProjectListItem: %s", dataStr)
	}
	if strings.Contains(dataStr, "metadata") || strings.Contains(dataStr, "source_metadata") {
		t.Fatalf("overfetching violation: raw metadata exposed in ProjectListItem: %s", dataStr)
	}

	// 3. Filter Only Archived
	archivedItems, err := svc.ListFiltered(ctx, ArchivedFilterOnly)
	if err != nil {
		t.Fatalf("ListFiltered only archived failed: %v", err)
	}
	if len(archivedItems) != 1 || archivedItems[0].ID != "proj-archived-1" {
		t.Fatalf("expected only archived project, got %d items", len(archivedItems))
	}

	// 4. Filter All
	allItems, err := svc.ListFiltered(ctx, ArchivedFilterAll)
	if err != nil {
		t.Fatalf("ListFiltered all failed: %v", err)
	}
	if len(allItems) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(allItems))
	}
}


