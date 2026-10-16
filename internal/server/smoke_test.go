package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
)

// TestSmoke_InstallLoginRestart verifies the end-to-end DoD criteria:
// 1. Clean installation & first-time owner setup
// 2. Protection against takeover (second setup rejected with 403 Forbidden)
// 3. Unauthenticated access blocked (401 Unauthorized on protected routes)
// 4. Authenticated access granted with session token and cookie
// 5. Logout properly invalidates session
// 6. Persistence across server restart (state survives server reboot)
func TestSmoke_InstallLoginRestart(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		ServerPort:  "8080",
		AuthSecret:  "test-secret-key-for-smoke-test-32chars",
	}

	// Shared persistent store simulating database volume across server restarts
	persistentStore := auth.NewMemoryStore()

	// -------------------------------------------------------------------------
	// PHASE 1: Clean Install & Fresh Boot
	// -------------------------------------------------------------------------
	srv1 := NewServer(cfg, nil, auth.NewService(persistentStore))

	// Initial status should require setup
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr := httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("[Phase 1] expected 200 for auth status, got %d", rr.Code)
	}

	var statusResp auth.StatusResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &statusResp)
	if !statusResp.SetupRequired || statusResp.OwnerConfigured {
		t.Fatalf("[Phase 1] expected setup_required=true, got %+v", statusResp)
	}

	// Protected routes MUST return 401 Unauthorized
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("[Phase 1] unauthenticated /api/v1/tasks must return 401, got %d", rr.Code)
	}

	// -------------------------------------------------------------------------
	// PHASE 2: First-Time Owner Setup
	// -------------------------------------------------------------------------
	setupBody := []byte(`{"email":"owner@cuan.dev","password":"MasterPassword123!"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(setupBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("[Phase 2] setup failed: code=%d, body=%s", rr.Code, rr.Body.String())
	}

	var setupAuth auth.AuthResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &setupAuth)
	if setupAuth.Token == "" || setupAuth.Owner.Email != "owner@cuan.dev" {
		t.Fatalf("[Phase 2] invalid setup response: %+v", setupAuth)
	}

	// -------------------------------------------------------------------------
	// PHASE 3: Takeover Protection (Second Setup Attempt MUST FAIL)
	// -------------------------------------------------------------------------
	hackerBody := []byte(`{"email":"attacker@evil.com","password":"HackerPassword999!"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(hackerBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("[Phase 3] CRITICAL: Second setup MUST return 403 Forbidden, got %d", rr.Code)
	}

	// -------------------------------------------------------------------------
	// PHASE 4: Authenticated Operations with Owner Session
	// -------------------------------------------------------------------------
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+setupAuth.Token)
	rr = httptest.NewRecorder()
	srv1.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("[Phase 4] authenticated tasks request failed, got %d", rr.Code)
	}

	var tasksResp map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &tasksResp)
	total, ok := tasksResp["total"].(float64)
	if !ok || total < 1 {
		t.Fatalf("[Phase 4] expected at least 1 task, got %+v", tasksResp)
	}

	// -------------------------------------------------------------------------
	// PHASE 5: Server Restart Simulation (Persistent Storage Retention)
	// -------------------------------------------------------------------------
	// Simulating container restart: srv1 is shutdown, srv2 boots with the same persistent volume
	srv2 := NewServer(cfg, nil, auth.NewService(persistentStore))

	// Status after reboot: setup must NOT be required, owner MUST remain configured
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)
	_ = json.Unmarshal(rr.Body.Bytes(), &statusResp)

	if statusResp.SetupRequired || !statusResp.OwnerConfigured {
		t.Fatalf("[Phase 5] after restart, expected owner_configured=true, got %+v", statusResp)
	}

	// Login with established owner credentials after restart
	loginBody := []byte(`{"email":"owner@cuan.dev","password":"MasterPassword123!"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("[Phase 5] login after restart failed: code=%d, body=%s", rr.Code, rr.Body.String())
	}

	var loginAuth auth.AuthResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &loginAuth)
	if loginAuth.Token == "" {
		t.Fatalf("[Phase 5] expected valid token after login")
	}

	// Existing session token from before restart also remains valid!
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+setupAuth.Token)
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("[Phase 5] session from before reboot should remain valid, got %d", rr.Code)
	}

	// Takeover protection still holds after restart
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(hackerBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("[Phase 5] takeover attempt after restart must be 403 Forbidden, got %d", rr.Code)
	}

	// -------------------------------------------------------------------------
	// PHASE 6: Logout and Revocation
	// -------------------------------------------------------------------------
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginAuth.Token)
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("[Phase 6] logout failed, got %d", rr.Code)
	}

	// Logged out token is immediately revoked
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+loginAuth.Token)
	rr = httptest.NewRecorder()
	srv2.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("[Phase 6] revoked token must return 401 Unauthorized, got %d", rr.Code)
	}
}
