package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthService_SetupAndLockout(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	// Initial status should require setup
	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("unexpected error on status: %v", err)
	}
	if !status.SetupRequired || status.OwnerConfigured {
		t.Fatalf("expected SetupRequired=true, got %v", status)
	}

	// Invalid email
	_, err = svc.Setup(ctx, "invalid-email", "password123")
	if err == nil {
		t.Fatalf("expected error for invalid email, got nil")
	}

	// Short password
	_, err = svc.Setup(ctx, "owner@example.com", "short")
	if err == nil {
		t.Fatalf("expected error for short password, got nil")
	}

	// Valid first-time setup
	authResp, err := svc.Setup(ctx, "owner@example.com", "securePassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if authResp.Token == "" {
		t.Fatalf("expected token, got empty")
	}
	if authResp.Owner.Email != "owner@example.com" {
		t.Fatalf("expected email owner@example.com, got %s", authResp.Owner.Email)
	}

	// Status should now be configured
	status, err = svc.Status(ctx)
	if err != nil {
		t.Fatalf("unexpected error on status: %v", err)
	}
	if status.SetupRequired || !status.OwnerConfigured {
		t.Fatalf("expected SetupRequired=false, OwnerConfigured=true, got %v", status)
	}

	// CRITICAL SECURITY DOD CHECK:
	// Second setup attempt must be blocked permanently to prevent third-party takeover!
	_, err = svc.Setup(ctx, "attacker@example.com", "hackerPassword999")
	if err != ErrSetupAlreadyCompleted {
		t.Fatalf("expected ErrSetupAlreadyCompleted on second setup, got %v", err)
	}
}

func TestAuthService_LoginAndSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	// Attempt login before setup
	_, err := svc.Login(ctx, "owner@example.com", "pass")
	if err != ErrSetupRequired {
		t.Fatalf("expected ErrSetupRequired, got %v", err)
	}

	// Setup owner
	setupResp, err := svc.Setup(ctx, "owner@example.com", "correctPassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Validate setup session token
	owner, err := svc.ValidateToken(ctx, setupResp.Token)
	if err != nil {
		t.Fatalf("validate token failed: %v", err)
	}
	if owner.Email != "owner@example.com" {
		t.Fatalf("expected owner email owner@example.com, got %s", owner.Email)
	}

	// Login with wrong password
	_, err = svc.Login(ctx, "owner@example.com", "wrongPassword")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// Login with wrong email
	_, err = svc.Login(ctx, "stranger@example.com", "correctPassword123")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// Successful login
	loginResp, err := svc.Login(ctx, "owner@example.com", "correctPassword123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatalf("expected login token")
	}

	// Validate login token
	owner, err = svc.ValidateToken(ctx, loginResp.Token)
	if err != nil {
		t.Fatalf("validate login token failed: %v", err)
	}
	if owner.ID != setupResp.Owner.ID {
		t.Fatalf("owner ID mismatch")
	}

	// Logout invalidates session
	if err := svc.Logout(ctx, loginResp.Token); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// Subsequent validation of logged out token must fail
	_, err = svc.ValidateToken(ctx, loginResp.Token)
	if err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized after logout, got %v", err)
	}

	// First setup token should still be valid (independent session)
	_, err = svc.ValidateToken(ctx, setupResp.Token)
	if err != nil {
		t.Fatalf("expected setup token to still be valid, got %v", err)
	}
}

func TestAuthMiddleware(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	authResp, err := svc.Setup(ctx, "owner@example.com", "mySecretPassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	handler := Middleware(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		owner := GetOwnerFromContext(r.Context())
		if owner == nil {
			http.Error(w, "missing owner in context", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok:" + owner.Email))
	}))

	// 1. Unauthenticated request -> 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", rr.Code)
	}

	// 2. Request with Bearer Token -> 200
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bearer token request, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "ok:owner@example.com" {
		t.Fatalf("unexpected body: %s", rr.Body.String())
	}

	// 3. Request with Cookie -> 200
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.AddCookie(&http.Cookie{
		Name:  "cuan_session",
		Value: authResp.Token,
	})
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for cookie request, got %d", rr.Code)
	}

	// 4. Request with invalid token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", rr.Code)
	}
}

func TestSessionExpiration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	now := time.Now().UTC()
	owner := &Owner{
		ID:           "owner_test",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		IsSetup:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_ = store.CreateOwner(ctx, owner)

	rawToken := "expired-token-12345"
	tokenHash := HashToken(rawToken)
	session := &Session{
		TokenHash: tokenHash,
		OwnerID:   owner.ID,
		ExpiresAt: now.Add(-1 * time.Hour), // Expired 1 hour ago
		CreatedAt: now.Add(-2 * time.Hour),
	}
	_ = store.CreateSession(ctx, session)

	_, err := svc.ValidateToken(ctx, rawToken)
	if err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized for expired session, got %v", err)
	}
}

func TestAuthService_LoginWithUsername(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	// Setup owner
	_, err := svc.Setup(ctx, "admin@example.com", "correctPassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Login with username "admin" (derived from email prefix)
	loginResp, err := svc.Login(ctx, "admin", "correctPassword123")
	if err != nil {
		t.Fatalf("login with username failed: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatalf("expected login token")
	}
	if loginResp.Owner.Username != "admin" {
		t.Fatalf("expected username admin, got %s", loginResp.Owner.Username)
	}
	if loginResp.User.Username != "admin" {
		t.Fatalf("expected user username admin, got %s", loginResp.User.Username)
	}

	// Login with uppercase username "ADMIN" (case-insensitive)
	loginRespUpper, err := svc.Login(ctx, "ADMIN", "correctPassword123")
	if err != nil {
		t.Fatalf("login with uppercase username failed: %v", err)
	}
	if loginRespUpper.Token == "" {
		t.Fatalf("expected login token")
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	setupResp, err := svc.Setup(ctx, "admin@example.com", "oldPassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// 1. Wrong current password -> fail
	err = svc.ChangePassword(ctx, setupResp.Owner.ID, "wrongCurrent", "newPassword456")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for wrong current password, got %v", err)
	}

	// 2. Short new password -> fail
	err = svc.ChangePassword(ctx, setupResp.Owner.ID, "oldPassword123", "short")
	if err == nil {
		t.Fatalf("expected error for short new password, got nil")
	}

	// 3. Valid change password -> success
	err = svc.ChangePassword(ctx, setupResp.Owner.ID, "oldPassword123", "newPassword456")
	if err != nil {
		t.Fatalf("expected success on change password, got %v", err)
	}

	// 4. Old password can no longer log in
	_, err = svc.Login(ctx, "admin@example.com", "oldPassword123")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials when using old password, got %v", err)
	}

	// 5. New password can log in successfully
	loginResp, err := svc.Login(ctx, "admin@example.com", "newPassword456")
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}
	if loginResp.Token == "" {
		t.Fatalf("expected token after new password login")
	}
}

func TestAuthService_AutoSeedDefaultAdmin(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	// 1. Auto seed default admin
	if err := svc.AutoSeedDefaultAdmin(ctx); err != nil {
		t.Fatalf("AutoSeedDefaultAdmin failed: %v", err)
	}

	// 2. Status should be configured
	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status check failed: %v", err)
	}
	if !status.OwnerConfigured || status.SetupRequired {
		t.Fatalf("expected OwnerConfigured=true, got %+v", status)
	}

	// 3. Login with default credentials: username 'admin', password 'whosyourdaddy'
	loginResp, err := svc.Login(ctx, "admin", "whosyourdaddy")
	if err != nil {
		t.Fatalf("login with default credentials failed: %v", err)
	}
	if loginResp.Owner.Username != "admin" {
		t.Fatalf("expected username admin, got %s", loginResp.Owner.Username)
	}
	if loginResp.Token == "" {
		t.Fatalf("expected token")
	}

	// 4. Second auto-seed should be a no-op
	if err := svc.AutoSeedDefaultAdmin(ctx); err != nil {
		t.Fatalf("expected second AutoSeedDefaultAdmin to succeed without error: %v", err)
	}
}

func TestAuthGuardMiddleware(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store)

	authResp, err := svc.Setup(ctx, "admin@example.com", "whosyourdaddy")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	guard := AuthGuard(svc)
	testHandler := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// 1. Bypass public paths without token: /healthz
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypass /healthz, got %d", rr.Code)
	}

	// 2. Bypass public paths: /readyz
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypass /readyz, got %d", rr.Code)
	}

	// 3. Bypass public paths: /api/info
	req = httptest.NewRequest(http.MethodGet, "/api/info", nil)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypass /api/info, got %d", rr.Code)
	}

	// 4. Bypass public paths: /api/mode
	req = httptest.NewRequest(http.MethodGet, "/api/mode", nil)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypass /api/mode, got %d", rr.Code)
	}

	// 5. Bypass public paths: /api/caddy/ask
	req = httptest.NewRequest(http.MethodGet, "/api/caddy/ask", nil)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypass /api/caddy/ask, got %d", rr.Code)
	}

	// 6. Bypass public paths: /api/v1/auth/login
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for bypass /api/v1/auth/login, got %d", rr.Code)
	}

	// 7. Protected path without token -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated private path, got %d", rr.Code)
	}

	// 8. Protected path with token -> 200
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	rr = httptest.NewRecorder()
	testHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated private path, got %d", rr.Code)
	}
}
