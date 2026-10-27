package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/domain"
)

// mockTestDeployDriver implements deployer.DeployDriver for server handler unit tests.
type mockTestDeployDriver struct {
	deployments map[string]*deployer.Deployment
}

func newMockTestDeployDriver() *mockTestDeployDriver {
	return &mockTestDeployDriver{
		deployments: make(map[string]*deployer.Deployment),
	}
}

func (m *mockTestDeployDriver) Deploy(ctx context.Context, req deployer.DeploymentRequest) (*deployer.DeploymentResult, error) {
	return nil, nil
}
func (m *mockTestDeployDriver) Start(ctx context.Context, id string) error   { return nil }
func (m *mockTestDeployDriver) Stop(ctx context.Context, id string) error    { return nil }
func (m *mockTestDeployDriver) Restart(ctx context.Context, id string) error { return nil }
func (m *mockTestDeployDriver) Health(ctx context.Context, id string) (*deployer.HealthStatus, error) {
	return &deployer.HealthStatus{Alive: true}, nil
}
func (m *mockTestDeployDriver) GetLogs(ctx context.Context, id string, limit int) ([]deployer.LogEntry, error) {
	return nil, nil
}
func (m *mockTestDeployDriver) List(ctx context.Context) ([]deployer.Deployment, error) {
	var list []deployer.Deployment
	for _, d := range m.deployments {
		list = append(list, *d)
	}
	return list, nil
}
func (m *mockTestDeployDriver) Get(ctx context.Context, id string) (*deployer.Deployment, error) {
	if d, ok := m.deployments[id]; ok {
		return d, nil
	}
	return nil, deployer.ErrDeploymentNotFound
}
func (m *mockTestDeployDriver) Delete(ctx context.Context, id string) error { return nil }

func setupTestServerWithDomain(t *testing.T) (*Server, *domain.MockDNSResolver, cuanDB.DomainRepository, *deployer.CaddyRouter) {
	t.Helper()

	cfg := &config.Config{
		AppMode:    "studio",
		BaseDomain: "cuan.app",
	}

	srv := NewServer(cfg, nil)

	// Replace with memory store and mock drivers for fast, isolated test
	domRepo := cuanDB.NewMemoryDomainRepository()
	srv.SetDomainRepository(domRepo)

	mockDNS := domain.NewMockDNSResolver()
	verifier := domain.NewDNSVerifier(domain.DNSVerifierConfig{
		CanonicalHosts: []string{"cuan.app", "cuan.dojo.cc"},
		TargetIPs:      []string{"127.0.0.1"},
	}, domain.WithResolver(mockDNS))
	srv.SetDNSVerifier(verifier)

	caddyRouter := deployer.NewCaddyRouter(deployer.CaddyConfig{
		BaseDomain: "cuan.app",
	})
	srv.SetCaddyRouter(caddyRouter)

	mockDriver := newMockTestDeployDriver()
	mockDriver.deployments["dep_test_100"] = &deployer.Deployment{
		ID:           "dep_test_100",
		ProjectName:  "Test SaaS App",
		Slug:         "saas-app",
		InternalPort: 3456,
		Status:       "running",
	}
	srv.SetDeployerDriver(mockDriver)

	return srv, mockDNS, domRepo, caddyRouter
}

func TestDomainHandlers_CreateDeploymentDomain(t *testing.T) {
	srv, _, _, _ := setupTestServerWithDomain(t)

	// 1. Success case: register custom domain
	reqBody := []byte(`{"domain": "app.mystartup.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/deployments/dep_test_100/domains", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status       string                 `json:"status"`
		Domain       cuanDB.CustomDomainDTO `json:"domain"`
		Instructions map[string]interface{} `json:"instructions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Fatalf("expected status success, got %s", resp.Status)
	}
	if resp.Domain.Domain != "app.mystartup.com" {
		t.Fatalf("expected domain app.mystartup.com, got %s", resp.Domain.Domain)
	}
	if resp.Domain.DNSStatus != "pending" {
		t.Fatalf("expected dns_status pending, got %s", resp.Domain.DNSStatus)
	}
	if resp.Domain.VerificationToken == "" {
		t.Fatalf("expected verification token to be generated")
	}
	if resp.Instructions["cname"] == nil || resp.Instructions["txt"] == nil {
		t.Fatalf("expected cname and txt instructions in response")
	}

	// 2. Duplicate registration -> 409 Conflict
	dupReq := httptest.NewRequest(http.MethodPost, "/api/deployments/dep_test_100/domains", bytes.NewReader(reqBody))
	dupReq.Header.Set("Content-Type", "application/json")
	dupW := httptest.NewRecorder()
	srv.Router().ServeHTTP(dupW, dupReq)
	if dupW.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict for duplicate domain, got %d", dupW.Code)
	}

	// 3. Nonexistent deployment -> 404 Not Found
	notFoundReq := httptest.NewRequest(http.MethodPost, "/api/deployments/dep_nonexistent/domains", bytes.NewReader(reqBody))
	notFoundReq.Header.Set("Content-Type", "application/json")
	notFoundW := httptest.NewRecorder()
	srv.Router().ServeHTTP(notFoundW, notFoundReq)
	if notFoundW.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found for nonexistent deployment, got %d", notFoundW.Code)
	}

	// 4. Invalid domain name syntax -> 400 Bad Request
	badReqBody := []byte(`{"domain": "http://invalid-format.com"}`)
	badReq := httptest.NewRequest(http.MethodPost, "/api/deployments/dep_test_100/domains", bytes.NewReader(badReqBody))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	srv.Router().ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request for invalid domain, got %d", badW.Code)
	}
}

func TestDomainHandlers_ListDomains(t *testing.T) {
	srv, _, repo, _ := setupTestServerWithDomain(t)

	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:           "dom_1",
		DeploymentID: "dep_test_100",
		Domain:       "one.example.com",
	})
	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:           "dom_2",
		DeploymentID: "dep_test_100",
		Domain:       "two.example.com",
	})
	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:           "dom_3",
		DeploymentID: "dep_other_200",
		Domain:       "three.example.com",
	})

	// 1. List by deployment: GET /api/deployments/dep_test_100/domains
	req1 := httptest.NewRequest(http.MethodGet, "/api/deployments/dep_test_100/domains", nil)
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", w1.Code, w1.Body.String())
	}

	var resp1 struct {
		Status  string                   `json:"status"`
		Domains []cuanDB.CustomDomainDTO `json:"domains"`
		Count   int                      `json:"count"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.Count != 2 {
		t.Fatalf("expected 2 domains for dep_test_100, got %d", resp1.Count)
	}

	// 2. List all: GET /api/domains
	req2 := httptest.NewRequest(http.MethodGet, "/api/domains", nil)
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", w2.Code)
	}
	var resp2 struct {
		Domains []cuanDB.CustomDomainDTO `json:"domains"`
		Count   int                      `json:"count"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Count != 3 {
		t.Fatalf("expected 3 total domains, got %d", resp2.Count)
	}

	// 3. List with filter: GET /api/domains?deployment_id=dep_other_200
	req3 := httptest.NewRequest(http.MethodGet, "/api/domains?deployment_id=dep_other_200", nil)
	w3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", w3.Code)
	}
	var resp3 struct {
		Domains []cuanDB.CustomDomainDTO `json:"domains"`
		Count   int                      `json:"count"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &resp3)
	if resp3.Count != 1 {
		t.Fatalf("expected 1 domain for dep_other_200, got %d", resp3.Count)
	}
}

func TestDomainHandlers_GetDomain(t *testing.T) {
	srv, _, repo, _ := setupTestServerWithDomain(t)

	_ = repo.CreateCustomDomain(context.Background(), &cuanDB.CustomDomain{
		ID:                "dom_lookup_1",
		DeploymentID:      "dep_test_100",
		Domain:            "lookup.test.com",
		VerificationToken: "tok_test_lookup",
	})

	// 1. Success lookup
	req := httptest.NewRequest(http.MethodGet, "/api/domains/dom_lookup_1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	var resp struct {
		Status string                 `json:"status"`
		Domain cuanDB.CustomDomainDTO `json:"domain"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Domain.Domain != "lookup.test.com" {
		t.Fatalf("expected lookup.test.com, got %s", resp.Domain.Domain)
	}

	// 2. Not found lookup
	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/domains/dom_unknown", nil)
	wNotFound := httptest.NewRecorder()
	srv.Router().ServeHTTP(wNotFound, reqNotFound)
	if wNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", wNotFound.Code)
	}
}

func TestDomainHandlers_VerifyDomain_SuccessAndFailure(t *testing.T) {
	srv, mockDNS, repo, caddyRouter := setupTestServerWithDomain(t)

	// Create domain for verification
	cd := &cuanDB.CustomDomain{
		ID:                "dom_verify_test",
		DeploymentID:      "dep_test_100",
		Domain:            "portal.customer.io",
		DNSStatus:         "pending",
		VerificationToken: "verify_token_abc_123",
	}
	_ = repo.CreateCustomDomain(context.Background(), cd)

	// Case 1: Verification probe fails (no DNS record configured yet)
	verifyReq1 := httptest.NewRequest(http.MethodPost, "/api/domains/dom_verify_test/verify", nil)
	w1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w1, verifyReq1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with failed verification status, got %d", w1.Code)
	}
	var resp1 struct {
		Status    string `json:"status"`
		Verified  bool   `json:"verified"`
		DNSStatus string `json:"dns_status"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.Verified {
		t.Fatalf("expected verified to be false when DNS has no records")
	}
	if resp1.DNSStatus != "failed" {
		t.Fatalf("expected dns_status failed, got %s", resp1.DNSStatus)
	}

	// Case 2: User sets DNS CNAME record -> verification probe succeeds
	mockDNS.SetCNAME("portal.customer.io", "cuan.app.")

	verifyReq2 := httptest.NewRequest(http.MethodPost, "/api/domains/dom_verify_test/verify", nil)
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, verifyReq2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Status          string `json:"status"`
		Verified        bool   `json:"verified"`
		DNSStatus       string `json:"dns_status"`
		RouteRegistered bool   `json:"route_registered"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if !resp2.Verified {
		t.Fatalf("expected verified to be true after CNAME is set")
	}
	if resp2.DNSStatus != "verified" {
		t.Fatalf("expected dns_status verified, got %s", resp2.DNSStatus)
	}
	if !resp2.RouteRegistered {
		t.Fatalf("expected Caddy route to be registered on verification success")
	}

	// Verify Caddy router actually has the custom domain route registered
	hasRoute, err := caddyRouter.GetCustomDomainRouteStatus(context.Background(), "portal.customer.io")
	if err != nil || !hasRoute {
		t.Fatalf("expected caddy router to have route registered for portal.customer.io, err: %v", err)
	}
}

func TestDomainHandlers_DeleteDomain(t *testing.T) {
	srv, _, repo, caddyRouter := setupTestServerWithDomain(t)

	cd := &cuanDB.CustomDomain{
		ID:                "dom_del_1",
		DeploymentID:      "dep_test_100",
		Domain:            "delete-me.app.com",
		DNSStatus:         "verified",
		VerificationToken: "tok_del",
	}
	_ = repo.CreateCustomDomain(context.Background(), cd)
	// Register Caddy route
	_ = caddyRouter.AddCustomDomainRoutePort(context.Background(), "delete-me.app.com", 3456)

	// Verify route exists
	hasRoute, _ := caddyRouter.GetCustomDomainRouteStatus(context.Background(), "delete-me.app.com")
	if !hasRoute {
		t.Fatalf("precondition failed: caddy route should exist")
	}

	// Delete domain
	req := httptest.NewRequest(http.MethodDelete, "/api/domains/dom_del_1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on deletion, got %d: %s", w.Code, w.Body.String())
	}

	// Check domain removed from repo
	_, err := repo.GetCustomDomainByID(context.Background(), "dom_del_1")
	if err == nil {
		t.Fatalf("expected domain to be removed from repository")
	}

	// Check Caddy route removed
	hasRouteAfter, _ := caddyRouter.GetCustomDomainRouteStatus(context.Background(), "delete-me.app.com")
	if hasRouteAfter {
		t.Fatalf("expected caddy route to be removed after domain deletion")
	}
}

func TestDomainHandlers_EndToEndWithCaddyAsk(t *testing.T) {
	srv, mockDNS, _, _ := setupTestServerWithDomain(t)

	// Step 1: Register domain
	regBody := []byte(`{"domain": "secure.myagency.com"}`)
	regReq := httptest.NewRequest(http.MethodPost, "/api/deployments/dep_test_100/domains", bytes.NewReader(regBody))
	regW := httptest.NewRecorder()
	srv.Router().ServeHTTP(regW, regReq)
	if regW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", regW.Code)
	}

	var regResp struct {
		Domain cuanDB.CustomDomainDTO `json:"domain"`
	}
	_ = json.Unmarshal(regW.Body.Bytes(), &regResp)
	domainID := regResp.Domain.ID

	// Step 2: Query /api/caddy/ask while domain is pending -> Must be HTTP 403 Forbidden
	askReq1 := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=secure.myagency.com", nil)
	askW1 := httptest.NewRecorder()
	srv.Router().ServeHTTP(askW1, askReq1)
	if askW1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for pending domain, got %d", askW1.Code)
	}

	// Step 3: Configure DNS TXT challenge token and trigger verification
	mockDNS.SetTXT("_cuan-challenge.secure.myagency.com", regResp.Domain.VerificationToken)

	verifyReq := httptest.NewRequest(http.MethodPost, "/api/domains/"+domainID+"/verify", nil)
	verifyW := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyW, verifyReq)
	if verifyW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on verify, got %d", verifyW.Code)
	}

	var verifyResp struct {
		Verified  bool   `json:"verified"`
		DNSStatus string `json:"dns_status"`
	}
	_ = json.Unmarshal(verifyW.Body.Bytes(), &verifyResp)
	if !verifyResp.Verified || verifyResp.DNSStatus != "verified" {
		t.Fatalf("expected verified domain, got %+v", verifyResp)
	}

	// Step 4: Query /api/caddy/ask now that domain is verified -> Must be HTTP 200 OK!
	askReq2 := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=secure.myagency.com", nil)
	askW2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(askW2, askReq2)
	if askW2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for verified domain, got %d: %s", askW2.Code, askW2.Body.String())
	}

	// Step 5: Delete domain -> /api/caddy/ask must return 403 Forbidden again
	delReq := httptest.NewRequest(http.MethodDelete, "/api/domains/"+domainID, nil)
	delW := httptest.NewRecorder()
	srv.Router().ServeHTTP(delW, delReq)
	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d", delW.Code)
	}

	askReq3 := httptest.NewRequest(http.MethodGet, "/api/caddy/ask?domain=secure.myagency.com", nil)
	askW3 := httptest.NewRecorder()
	srv.Router().ServeHTTP(askW3, askReq3)
	if askW3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden after domain was deleted, got %d", askW3.Code)
	}
}
