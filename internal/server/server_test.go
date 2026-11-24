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
)

func TestHealthz(t *testing.T) {
	cfg := &config.Config{
		AppVersion: "0.9.0",
		CommitSHA:  "616f3840dc2dcdbbe962d639937da250b1d694c4",
	}
	srv := NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", body["status"])
	}
	if body["version"] != "0.9.0" {
		t.Errorf("expected version '0.9.0', got %v", body["version"])
	}
	if body["commit_sha"] != "616f3840dc2dcdbbe962d639937da250b1d694c4" {
		t.Errorf("expected commit_sha '616f3840dc2dcdbbe962d639937da250b1d694c4', got %v", body["commit_sha"])
	}
}

func TestReadyz_InMemory(t *testing.T) {
	cfg := &config.Config{DatabaseURL: ""}
	srv := NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if body["status"] != "ready" {
		t.Errorf("expected status ready, got %v", body["status"])
	}
}

func TestInstallScript(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/install.sh", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/x-shellscript") {
		t.Errorf("expected text/x-shellscript content-type, got %s", contentType)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "#!/usr/bin/env bash") {
		t.Errorf("expected shell script shebang, got %s", body[:min(50, len(body))])
	}
}

func TestApiInfo(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if body["license"] != "Apache-2.0" {
		t.Errorf("expected license Apache-2.0, got %v", body["license"])
	}
	if body["status"] != "running" {
		t.Errorf("expected status running, got %v", body["status"])
	}
	if body["repository"] != "https://github.com/buchin/cuan" {
		t.Errorf("expected repository https://github.com/buchin/cuan, got %v", body["repository"])
	}
	if body["release_stage"] != "release-candidate" {
		t.Errorf("expected release_stage release-candidate, got %v", body["release_stage"])
	}
}

func TestAuthFlow_SetupLoginProtectedEndpoints(t *testing.T) {
	cfg := &config.Config{}
	memStore := auth.NewMemoryStore()
	authSvc := auth.NewService(memStore)
	srv := NewServer(cfg, nil, authSvc)

	// 1. Initial auth status
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var statusResp auth.StatusResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &statusResp)
	if !statusResp.SetupRequired || statusResp.OwnerConfigured {
		t.Fatalf("expected setup required, got %+v", statusResp)
	}

	// 2. Unauthenticated access to /api/v1/tasks must be 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for /api/v1/tasks, got %d", rr.Code)
	}

	// 3. Setup first owner
	setupPayload := []byte(`{"email":"admin@example.com","password":"secretPassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(setupPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on setup, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var setupResp auth.AuthResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &setupResp)
	if setupResp.Token == "" {
		t.Fatalf("expected token in setup response")
	}

	// Verify cookie is set
	cookies := rr.Result().Cookies()
	var foundCookie bool
	for _, c := range cookies {
		if c.Name == "cuan_session" && c.Value == setupResp.Token {
			foundCookie = true
			if !c.HttpOnly {
				t.Errorf("cookie should be HttpOnly")
			}
		}
	}
	if !foundCookie {
		t.Errorf("expected cuan_session cookie to be set")
	}

	// 4. CRITICAL SECURITY DoD: Second setup attempt MUST BE BLOCKED with 403 Forbidden
	attackerPayload := []byte(`{"email":"attacker@example.com","password":"evilPassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(attackerPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for duplicate setup attempt, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 5. Access /api/v1/tasks with Bearer token
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+setupResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated /api/v1/tasks, got %d", rr.Code)
	}

	// 6. Access /api/v1/auth/me
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+setupResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/auth/me, got %d", rr.Code)
	}

	// 7. Login with valid credentials
	loginPayload := []byte(`{"email":"admin@example.com","password":"secretPassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d", rr.Code)
	}
	var loginResp auth.AuthResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)

	// 8. Logout
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for logout, got %d", rr.Code)
	}

	// 9. Attempt access with logged out token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", rr.Code)
	}
}

func TestAuthFlow_UsernameLogin_PasswordChange_DefaultAdmin(t *testing.T) {
	cfg := &config.Config{}
	memStore := auth.NewMemoryStore()
	authSvc := auth.NewService(memStore)
	srv := NewServer(cfg, nil, authSvc)

	// 1. Auto seed default admin
	if err := authSvc.AutoSeedDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("auto-seed failed: %v", err)
	}

	// 2. Login using default admin credentials: username 'admin', password 'whosyourdaddy'
	loginPayload := []byte(`{"username":"admin","password":"whosyourdaddy"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for username login, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var loginResp auth.AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatalf("expected token in login response")
	}
	if loginResp.User.Username != "admin" {
		t.Fatalf("expected user.username 'admin', got %s", loginResp.User.Username)
	}

	// 3. Verify GET /api/v1/auth/me returns user & owner info
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/v1/auth/me, got %d", rr.Code)
	}
	var meResp map[string]map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("failed to decode me response: %v", err)
	}
	if meResp["user"]["username"] != "admin" {
		t.Fatalf("expected user username 'admin', got %v", meResp["user"]["username"])
	}

	// 4. Change Password via POST /api/v1/auth/password without auth -> 401
	pwPayload := []byte(`{"current_password":"whosyourdaddy","new_password":"newSecurePassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", bytes.NewBuffer(pwPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated password change, got %d", rr.Code)
	}

	// 5. Change Password with wrong current password -> 401
	badPwPayload := []byte(`{"current_password":"wrongPassword","new_password":"newSecurePassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", bytes.NewBuffer(badPwPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong current password, got %d", rr.Code)
	}

	// 6. Change Password with valid credentials -> 200
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", bytes.NewBuffer(pwPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid password change, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 7. Old password fails
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when logging in with old password, got %d", rr.Code)
	}

	// 8. New password succeeds
	newLoginPayload := []byte(`{"username":"admin","password":"newSecurePassword123"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(newLoginPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for login with new password, got %d", rr.Code)
	}
}

func TestDeployWebhookRoute_UnauthenticatedBypass(t *testing.T) {
	cfg := &config.Config{Environment: "testing"}
	memStore := auth.NewMemoryStore()
	authSvc := auth.NewService(memStore)
	srv := NewServer(cfg, nil, authSvc)

	// Call webhook without session auth
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/non-existent/webhooks/deploy", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	// Must NOT return session auth error ("unauthorized: authentication required")
	body := rr.Body.String()
	if strings.Contains(body, "authentication required") {
		t.Fatalf("expected webhook route to bypass session auth, got: %s", body)
	}
}
