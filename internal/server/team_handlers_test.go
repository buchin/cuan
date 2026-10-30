package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func setupTeamTestServer(t *testing.T) (*Server, string, string) {
	t.Helper()

	cfg := &config.Config{
		AppMode:    "production",
		AuthSecret: "test-auth-secret-phrase-for-team-testing",
	}

	authStore := auth.NewMemoryStore()
	authSvc := auth.NewService(authStore)
	repo := db.NewMemoryAgentRepository()
	registry := agent.NewProviderRegistry()
	agent.RegisterBYOKProviders(registry)

	srv := NewServer(cfg, nil, authSvc)
	srv.SetAgentRepository(repo)
	srv.SetProviderRegistry(registry)

	ctx := context.Background()
	// Setup owner
	ownerSetup, err := authSvc.Setup(ctx, "boss@company.local", "bossPassword123")
	if err != nil {
		t.Fatalf("failed to setup owner: %v", err)
	}

	// Create member invite & accept to get member token
	inviteResp, err := authSvc.CreateInvite(ctx, ownerSetup.Owner.ID, "staff@company.local", auth.RoleMember)
	if err != nil {
		t.Fatalf("failed to create member invite: %v", err)
	}

	memberResp, err := authSvc.AcceptInvite(ctx, inviteResp.Token, "Staff Member", "staffPassword123")
	if err != nil {
		t.Fatalf("failed to accept member invite: %v", err)
	}

	return srv, ownerSetup.Token, memberResp.Token
}

func TestTeam_RBAC_FuralModelProtection(t *testing.T) {
	srv, ownerToken, memberToken := setupTeamTestServer(t)

	// 1. Member attempting to create an Agent -> 403 Forbidden
	createAgentPayload := map[string]interface{}{
		"name":          "@custom-agent",
		"harness":       "custom",
		"connection_id": "conn_test_123",
		"enabled":       true,
	}
	agentBytes, _ := json.Marshal(createAgentPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewReader(agentBytes))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when member creates agent, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Member attempting to save AI Provider -> 403 Forbidden
	providerPayload := map[string]interface{}{
		"provider":      "openai",
		"mode":          "byok",
		"api_key":       "sk-proj-test123456789",
		"default_model": "gpt-4o",
	}
	provBytes, _ := json.Marshal(providerPayload)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers", bytes.NewReader(provBytes))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when member saves AI provider, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Member attempting to install AI provider binary -> 403 Forbidden
	installPayload := map[string]string{
		"binary": "codex",
	}
	instBytes, _ := json.Marshal(installPayload)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/ai/providers/install-binary", bytes.NewReader(instBytes))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when member installs AI binary, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Member attempting to invite team member -> 403 Forbidden
	invitePayload := map[string]string{
		"email": "another@company.local",
		"role":  "member",
	}
	invBytes, _ := json.Marshal(invitePayload)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/team/invites", bytes.NewReader(invBytes))
	req.Header.Set("Authorization", "Bearer "+memberToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when member creates invite, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Owner attempting to invite team member -> 201 Created
	req = httptest.NewRequest(http.MethodPost, "/api/v1/team/invites", bytes.NewReader(invBytes))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created when owner creates invite, got %d: %s", w.Code, w.Body.String())
	}

	var inviteResp auth.CreateInviteResponse
	if err := json.NewDecoder(w.Body).Decode(&inviteResp); err != nil {
		t.Fatalf("failed to decode invite response: %v", err)
	}
	if inviteResp.Token == "" || inviteResp.InviteURL == "" {
		t.Fatalf("expected token and invite_url in response")
	}

	// 6. Member can view team members -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/v1/team/members", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for member viewing team members, got %d: %s", w.Code, w.Body.String())
	}

	var teamResp struct {
		Status      string               `json:"status"`
		Members     []auth.TeamMemberDTO `json:"members"`
		Invitations []auth.UserInvitation `json:"invitations"`
	}
	if err := json.NewDecoder(w.Body).Decode(&teamResp); err != nil {
		t.Fatalf("failed to decode team members response: %v", err)
	}
	if len(teamResp.Members) < 2 {
		t.Errorf("expected at least 2 members (owner and member), got %d", len(teamResp.Members))
	}
	if len(teamResp.Invitations) < 1 {
		t.Errorf("expected at least 1 pending invitation, got %d", len(teamResp.Invitations))
	}
}

func TestTeam_InviteAcceptAndLoginFlow(t *testing.T) {
	srv, ownerToken, _ := setupTeamTestServer(t)

	// 1. Owner creates invitation for new staff
	invitePayload := map[string]string{
		"email": "newbie@company.local",
		"role":  "member",
	}
	invBytes, _ := json.Marshal(invitePayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/team/invites", bytes.NewReader(invBytes))
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on invite creation, got %d: %s", w.Code, w.Body.String())
	}

	var inviteResp auth.CreateInviteResponse
	_ = json.Unmarshal(w.Body.Bytes(), &inviteResp)
	rawToken := inviteResp.Token
	inviteID := inviteResp.Invitation.ID

	// 2. Verify invite endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/invites/verify?token="+rawToken, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on verify invite, got %d: %s", w.Code, w.Body.String())
	}

	var verifyResp struct {
		Valid bool   `json:"valid"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &verifyResp)
	if !verifyResp.Valid || verifyResp.Email != "newbie@company.local" {
		t.Errorf("unexpected verify response: %+v", verifyResp)
	}

	// 3. Public accept invite endpoint
	acceptPayload := map[string]string{
		"token":    rawToken,
		"name":     "Newbie Developer",
		"password": "newbiePassword123",
	}
	accBytes, _ := json.Marshal(acceptPayload)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/invites/accept", bytes.NewReader(accBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on accept invite, got %d: %s", w.Code, w.Body.String())
	}

	var authResp auth.AuthResponse
	if err := json.NewDecoder(w.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to decode accept invite auth response: %v", err)
	}

	if authResp.Token == "" {
		t.Errorf("expected session token in accept invite response")
	}
	if authResp.User.Role != auth.RoleMember {
		t.Errorf("expected member role, got %s", authResp.User.Role)
	}

	// Check cookie was set
	cookies := w.Result().Cookies()
	foundCookie := false
	for _, c := range cookies {
		if c.Name == "cuan_session" && c.Value == authResp.Token {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Errorf("expected cuan_session cookie to be set")
	}

	// 4. Verify /api/v1/auth/me returns the role and identity
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on auth me, got %d: %s", w.Code, w.Body.String())
	}

	var meResp struct {
		User map[string]interface{} `json:"user"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &meResp)
	if meResp.User["role"] != "member" {
		t.Errorf("expected role 'member' in /auth/me, got %v", meResp.User["role"])
	}

	// 5. Delete invite
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/team/invites/"+inviteID, nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete invite, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Delete member
	newbieID := authResp.User.ID
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/team/members/"+newbieID, nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete member, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeam_MemberOperationalAccessAllowed(t *testing.T) {
	srv, _, memberToken := setupTeamTestServer(t)

	// Member can view tasks (protected route) -> 200 OK
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for member viewing tasks, got %d: %s", w.Code, w.Body.String())
	}

	// Member can view projects (protected route) -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for member viewing projects, got %d: %s", w.Code, w.Body.String())
	}

	// Member can view agents list (read-only) -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for member listing agents, got %d: %s", w.Code, w.Body.String())
	}

	// Member can view AI providers list (read-only) -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ai/providers", nil)
	req.Header.Set("Authorization", "Bearer "+memberToken)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for member listing AI providers, got %d: %s", w.Code, w.Body.String())
	}
}

