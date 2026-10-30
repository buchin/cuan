package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRBAC_RequireRole(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	ownerUser := &Owner{ID: "usr_owner", Role: RoleOwner, Email: "owner@test.local"}
	memberUser := &Owner{ID: "usr_member", Role: RoleMember, Email: "member@test.local"}

	// 1. Unauthenticated request -> 401
	handler := RequireRole(RoleOwner)(dummyHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", w.Code)
	}

	// 2. Member accessing Owner-only route -> 403
	reqMember := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqMember = reqMember.WithContext(ContextWithOwner(reqMember.Context(), memberUser))
	wMember := httptest.NewRecorder()
	handler.ServeHTTP(wMember, reqMember)
	if wMember.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for member on owner-only route, got %d", wMember.Code)
	}

	// 3. Owner accessing Owner-only route -> 200
	reqOwner := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqOwner = reqOwner.WithContext(ContextWithOwner(reqOwner.Context(), ownerUser))
	wOwner := httptest.NewRecorder()
	handler.ServeHTTP(wOwner, reqOwner)
	if wOwner.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for owner, got %d", wOwner.Code)
	}

	// 4. RequireOwner helper
	ownerHandler := RequireOwner()(dummyHandler)
	wMember2 := httptest.NewRecorder()
	ownerHandler.ServeHTTP(wMember2, reqMember)
	if wMember2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden via RequireOwner for member, got %d", wMember2.Code)
	}

	wOwner2 := httptest.NewRecorder()
	ownerHandler.ServeHTTP(wOwner2, reqOwner)
	if wOwner2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK via RequireOwner for owner, got %d", wOwner2.Code)
	}

	// 5. Route allowing both owner and member
	dualHandler := RequireRole(RoleOwner, RoleMember)(dummyHandler)
	wDualMember := httptest.NewRecorder()
	dualHandler.ServeHTTP(wDualMember, reqMember)
	if wDualMember.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for member on dual route, got %d", wDualMember.Code)
	}
}

func TestRBAC_RequireOwnerWithService(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store)
	ctx := context.Background()

	// Setup owner
	ownerResp, err := svc.Setup(ctx, "boss@test.local", "securepassword123")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Create member
	inviteResp, err := svc.CreateInvite(ctx, ownerResp.Owner.ID, "staff@test.local", RoleMember)
	if err != nil {
		t.Fatalf("create invite failed: %v", err)
	}
	memberResp, err := svc.AcceptInvite(ctx, inviteResp.Token, "Staff Member", "securepassword123")
	if err != nil {
		t.Fatalf("accept invite failed: %v", err)
	}

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	guard := RequireOwnerWithService(svc)(dummyHandler)

	// Unauthenticated
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	guard.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}

	// Member Bearer token -> 403 Forbidden
	reqMember := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqMember.Header.Set("Authorization", "Bearer "+memberResp.Token)
	wMember := httptest.NewRecorder()
	guard.ServeHTTP(wMember, reqMember)
	if wMember.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member token, got %d: %s", wMember.Code, wMember.Body.String())
	}

	// Owner Bearer token -> 200 OK
	reqOwner := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqOwner.Header.Set("Authorization", "Bearer "+ownerResp.Token)
	wOwner := httptest.NewRecorder()
	guard.ServeHTTP(wOwner, reqOwner)
	if wOwner.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner token, got %d: %s", wOwner.Code, wOwner.Body.String())
	}
}
