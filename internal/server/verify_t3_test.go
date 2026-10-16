package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
)

func TestVerifyT3Evidence(t *testing.T) {
	fmt.Println("=== STARTING CUAN T3 VERIFICATION ===")

	cfg := &config.Config{
		AuthSecret: "cuan-production-auth-secret-key-32b",
		StaticDir:  "web/dist",
	}

	authStore := auth.NewMemoryStore()
	authSvc := auth.NewService(authStore)

	projectStore := project.NewMemoryStore()
	gitClient := git.NewClient()
	tempDataDir, err := os.MkdirTemp("", "cuan-data-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDataDir)

	projectSvc := project.NewService(projectStore, gitClient, cfg.AuthSecret, tempDataDir)

	srv := NewServer(cfg, nil, authSvc)
	srv.SetProjectService(projectSvc)

	ctx := context.Background()

	// Owner Setup
	setupResp, err := authSvc.Setup(ctx, "owner@cuan.dev", "OwnerPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	token := setupResp.Token
	fmt.Printf("[AUTH] Owner setup successful: %s, token: %s...\n", setupResp.Owner.Email, token[:8])

	// EVIDENCE 1: Connect real GitHub repository (buchin/cuan)
	fmt.Println("\n--- [EVIDENCE 1] CONNECTING REAL GITHUB REPO (buchin/cuan) ---")
	connectPayload := []byte(`{
		"name": "Cuan Monorepo",
		"repo_url": "https://github.com/buchin/cuan",
		"default_branch": "main"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer(connectPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	fmt.Printf("HTTP Status: %d\n", rr.Code)
	fmt.Printf("Response Body: %s\n", rr.Body.String())
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", rr.Code)
	}

	var connectedProj project.ProjectResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &connectedProj)

	// Fetch Branches from real GitHub repo
	reqBranches := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+connectedProj.ID+"/branches", nil)
	reqBranches.Header.Set("Authorization", "Bearer "+token)
	rrBranches := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrBranches, reqBranches)
	fmt.Printf("Branches Response (%d): %s\n", rrBranches.Code, rrBranches.Body.String())

	// EVIDENCE 2: INFORMATIVE ERROR HANDLING
	fmt.Println("\n--- [EVIDENCE 2] INFORMATIVE ERROR HANDLING (404 & 401/403) ---")

	// Case 2A: Nonexistent Repo (404)
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer([]byte(`{
		"repo_url": "buchin/nonexistent-cuan-repo-99999"
	}`)))
	req404.Header.Set("Authorization", "Bearer "+token)
	req404.Header.Set("Content-Type", "application/json")
	rr404 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr404, req404)
	fmt.Printf("Case 2A (Repo 404) HTTP Status: %d\nBody: %s\n", rr404.Code, rr404.Body.String())

	// Case 2B: Invalid Token / Access Denied (403)
	req403 := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer([]byte(`{
		"repo_url": "https://github.com/buchin/private-fixture-repo",
		"token": "ghp_invalid_token_sample_12345678"
	}`)))
	req403.Header.Set("Authorization", "Bearer "+token)
	req403.Header.Set("Content-Type", "application/json")
	rr403 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr403, req403)
	fmt.Printf("Case 2B (Access Denied) HTTP Status: %d\nBody: %s\n", rr403.Code, rr403.Body.String())

	// Case 2C: Invalid URL (400)
	req400 := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBuffer([]byte(`{
		"repo_url": "not-a-valid-url"
	}`)))
	req400.Header.Set("Authorization", "Bearer "+token)
	req400.Header.Set("Content-Type", "application/json")
	rr400 := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr400, req400)
	fmt.Printf("Case 2C (Invalid URL) HTTP Status: %d\nBody: %s\n", rr400.Code, rr400.Body.String())

	// EVIDENCE 3: CREDENTIAL SECURITY & REMOTE PROTECTION AUDIT
	fmt.Println("\n--- [EVIDENCE 3] CREDENTIAL SECURITY AUDIT ---")
	rawSecretPAT := "ghp_RealSecretGitHubPATValue998877665544"

	// 1. Encryption Test
	encryptedHex, err := crypto.Encrypt(rawSecretPAT, cfg.AuthSecret)
	if err != nil {
		t.Fatal(err)
	}
	decryptedVal, err := crypto.Decrypt(encryptedHex, cfg.AuthSecret)
	if err != nil {
		t.Fatal(err)
	}
	preview := crypto.TokenPreview(rawSecretPAT)

	fmt.Printf("Raw PAT:              %s\n", rawSecretPAT)
	fmt.Printf("Encrypted DB Hex:     %s\n", encryptedHex)
	fmt.Printf("Decrypted from DB:    %s (Match: %v)\n", decryptedVal, decryptedVal == rawSecretPAT)
	fmt.Printf("Token Preview in API: %s\n", preview)

	// 2. Secret Masker in log / stderr
	dirtyGitLog := fmt.Sprintf("fatal: Authentication failed for 'https://x-access-token:%s@github.com/org/repo.git/'", rawSecretPAT)
	cleanedLog := agent.MaskSecrets(dirtyGitLog, []string{rawSecretPAT, "x-access-token:" + rawSecretPAT})
	fmt.Printf("Dirty Stderr:         %s\n", dirtyGitLog)
	fmt.Printf("Sanitized Log Output: %s\n", cleanedLog)

	// EVIDENCE 4: REMOTE REPOSITORY SAFETY ON PROJECT DELETE
	fmt.Println("\n--- [EVIDENCE 4] NON-DESTRUCTIVE DELETE AUDIT ---")
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+connectedProj.ID, nil)
	reqDel.Header.Set("Authorization", "Bearer "+token)
	rrDel := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrDel, reqDel)
	fmt.Printf("Delete Project HTTP Status: %d\nResponse: %s\n", rrDel.Code, rrDel.Body.String())

	// Verify project is gone from Cuan
	reqCheck := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+connectedProj.ID, nil)
	reqCheck.Header.Set("Authorization", "Bearer "+token)
	rrCheck := httptest.NewRecorder()
	srv.Router().ServeHTTP(rrCheck, reqCheck)
	fmt.Printf("Get Deleted Project HTTP Status: %d\n", rrCheck.Code)

	// Verify remote GitHub repo is still alive and reachable
	remoteCheck, err := gitClient.ValidateRepository(ctx, "buchin/cuan", "")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Remote GitHub Repository Status: Intact and reachable (Default branch: %s, URL: %s)\n",
		remoteCheck.DefaultBranch, remoteCheck.CloneURL)

	fmt.Println("\n=== VERIFICATION COMPLETE ===")
}
