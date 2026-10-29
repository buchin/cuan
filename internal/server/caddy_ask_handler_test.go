package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
)

func setupTestServerWithDomainRepo(t *testing.T) (*Server, *cuanDB.MemoryDomainRepository) {
	t.Helper()
	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-chars-long!",
	}
	s := NewServer(cfg, nil)
	memRepo := cuanDB.NewMemoryDomainRepository()
	s.SetDomainRepository(memRepo)
	return s, memRepo
}

func TestCaddyAskHandler_AuthorizedDomain(t *testing.T) {
	s, repo := setupTestServerWithDomainRepo(t)
	ctx := context.Background()

	// Seed a verified custom domain
	err := repo.CreateCustomDomain(ctx, &cuanDB.CustomDomain{
		ID:           "dom_verified_01",
		DeploymentID: "dep_verified_01",
		Domain:       "app.clientbrand.com",
		DNSStatus:    "verified",
		SSLStatus:    "pending",
	})
	if err != nil {
		t.Fatalf("failed to create verified custom domain: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=app.clientbrand.com", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for verified domain, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp CaddyAskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response json: %v", err)
	}

	if resp.Status != "authorized" {
		t.Errorf("expected status 'authorized', got %q", resp.Status)
	}
	if resp.Domain != "app.clientbrand.com" {
		t.Errorf("expected domain 'app.clientbrand.com', got %q", resp.Domain)
	}
}

func TestCaddyAskHandler_ActiveSSLDomain(t *testing.T) {
	s, repo := setupTestServerWithDomainRepo(t)
	ctx := context.Background()

	// Seed an active SSL custom domain (even if dns_status was previously active)
	err := repo.CreateCustomDomain(ctx, &cuanDB.CustomDomain{
		ID:           "dom_active_ssl_02",
		DeploymentID: "dep_active_ssl_02",
		Domain:       "store.mycompany.io",
		DNSStatus:    "active",
		SSLStatus:    "active",
	})
	if err != nil {
		t.Fatalf("failed to create custom domain: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=store.mycompany.io", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for active SSL domain, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestCaddyAskHandler_UnauthorizedDomain_NotFound(t *testing.T) {
	s, _ := setupTestServerWithDomainRepo(t)

	req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=unregistered.clientbrand.com", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for unregistered domain, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp CaddyAskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response json: %v", err)
	}

	if resp.Status != "forbidden" {
		t.Errorf("expected status 'forbidden', got %q", resp.Status)
	}
}

func TestCaddyAskHandler_UnverifiedDomain_PendingOrFailed(t *testing.T) {
	s, repo := setupTestServerWithDomainRepo(t)
	ctx := context.Background()

	// 1. Pending domain
	err := repo.CreateCustomDomain(ctx, &cuanDB.CustomDomain{
		ID:           "dom_pending_03",
		DeploymentID: "dep_pending_03",
		Domain:       "pending.example.org",
		DNSStatus:    "pending",
		SSLStatus:    "pending",
	})
	if err != nil {
		t.Fatalf("failed to create pending domain: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=pending.example.org", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for pending domain, got %d. Body: %s", w.Code, w.Body.String())
	}

	// 2. Failed DNS status domain
	err = repo.CreateCustomDomain(ctx, &cuanDB.CustomDomain{
		ID:           "dom_failed_04",
		DeploymentID: "dep_failed_04",
		Domain:       "failed.example.org",
		DNSStatus:    "failed",
		SSLStatus:    "failed",
	})
	if err != nil {
		t.Fatalf("failed to create failed domain: %v", err)
	}

	reqFailed := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=failed.example.org", nil)
	wFailed := httptest.NewRecorder()
	s.Router().ServeHTTP(wFailed, reqFailed)

	if wFailed.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for failed DNS domain, got %d. Body: %s", wFailed.Code, wFailed.Body.String())
	}
}

func TestCaddyAskHandler_MissingOrEmptyDomainParam(t *testing.T) {
	s, _ := setupTestServerWithDomainRepo(t)

	// Missing parameter completely
	req1 := httptest.NewRequest(http.MethodGet, "/api/caddy/ask", nil)
	w1 := httptest.NewRecorder()
	s.Router().ServeHTTP(w1, req1)

	if w1.Code != http.StatusForbidden && w1.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 403/400 for missing domain param, got %d", w1.Code)
	}

	// Empty parameter
	req2 := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=", nil)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden && w2.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 403/400 for empty domain param, got %d", w2.Code)
	}

	// Whitespace only parameter
	req3 := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=%20%20%20", nil)
	w3 := httptest.NewRecorder()
	s.Router().ServeHTTP(w3, req3)

	if w3.Code != http.StatusForbidden && w3.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 403/400 for whitespace domain param, got %d", w3.Code)
	}
}

func TestCaddyAskHandler_InvalidDomainFormat(t *testing.T) {
	s, _ := setupTestServerWithDomainRepo(t)

	invalidInputs := []string{
		"http://evil.com",
		"https://sub.domain.org",
		"domain-with-space .com",
		"domain:8080",
		"bad/path",
		"singleword",
		"evil..com",
	}

	for _, invalid := range invalidInputs {
		req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain="+url.QueryEscape(invalid), nil)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected HTTP 403 Forbidden for invalid domain %q, got %d. Body: %s", invalid, w.Code, w.Body.String())
		}
	}
}

func TestCaddyAskHandler_NormalizationAndAliases(t *testing.T) {
	s, repo := setupTestServerWithDomainRepo(t)
	ctx := context.Background()

	err := repo.CreateCustomDomain(ctx, &cuanDB.CustomDomain{
		ID:           "dom_alias_05",
		DeploymentID: "dep_alias_05",
		Domain:       "portal.enterprise.id",
		DNSStatus:    "verified",
		SSLStatus:    "pending",
	})
	if err != nil {
		t.Fatalf("failed to create custom domain: %v", err)
	}

	// Uppercase and trimmed input query
	reqUpper := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=%20%20PORTAL.Enterprise.ID%20", nil)
	wUpper := httptest.NewRecorder()
	s.Router().ServeHTTP(wUpper, reqUpper)

	if wUpper.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for normalized uppercase query, got %d. Body: %s", wUpper.Code, wUpper.Body.String())
	}

	// SSOT alias: /api/domains/check-ask
	reqAlias1 := httptest.NewRequest(http.MethodGet, "/api/domains/check-ask?domain=portal.enterprise.id", nil)
	wAlias1 := httptest.NewRecorder()
	s.Router().ServeHTTP(wAlias1, reqAlias1)

	if wAlias1.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for /api/domains/check-ask alias, got %d. Body: %s", wAlias1.Code, wAlias1.Body.String())
	}

	// v1 alias: /api/v1/caddy/ask
	reqAlias2 := httptest.NewRequest(http.MethodGet, "/api/v1/caddy/ask?domain=portal.enterprise.id", nil)
	wAlias2 := httptest.NewRecorder()
	s.Router().ServeHTTP(wAlias2, reqAlias2)

	if wAlias2.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for /api/v1/caddy/ask alias, got %d. Body: %s", wAlias2.Code, wAlias2.Body.String())
	}
}

func TestCaddyAskHandler_NilRepositorySafeguard(t *testing.T) {
	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-chars-long!",
	}
	s := NewServer(cfg, nil)
	s.SetDomainRepository(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=app.clientbrand.com", nil)
	w := httptest.NewRecorder()

	// Should not panic, but return 403 Forbidden
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden when repository is nil, got %d", w.Code)
	}
}

func TestCaddyAskHandler_BaseDomainsAuthorized(t *testing.T) {
	s, _ := setupTestServerWithDomainRepo(t)

	baseDomains := []string{
		"cuan.work",
		"CUAN.WORK",
		"cuan.dojo.cc",
		"cuan.chat",
		"CUAN.CHAT",
	}

	for _, d := range baseDomains {
		req := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain="+url.QueryEscape(d), nil)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected HTTP 200 OK for base domain %q, got %d. Body: %s", d, w.Code, w.Body.String())
		}

		var resp CaddyAskResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response json for %q: %v", d, err)
		}
		if resp.Status != "authorized" {
			t.Errorf("expected status 'authorized' for %q, got %q", d, resp.Status)
		}
	}
}
