package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/sponsorship"
)

func setupTestServerWithAdminAuth(t *testing.T) (*Server, string, func()) {
	t.Helper()
	s, cleanup := setupTestServerWithSponsorship(t)

	// Setup owner
	setupPayload := []byte(`{"email":"owner@cuan.dev","password":"MasterPassword123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBuffer(setupPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("setup owner failed: %d - %s", rr.Code, rr.Body.String())
	}

	var authResp auth.AuthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}

	return s, authResp.Token, cleanup
}

func TestAdminSponsors_AuthGuard(t *testing.T) {
	s, token, cleanup := setupTestServerWithAdminAuth(t)
	defer cleanup()

	// 1. Without Token -> 401 Unauthorized
	req := httptest.NewRequest(http.MethodGet, "/api/admin/sponsors", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without token on /api/admin/sponsors, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/fake_123/approve", nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without token on approve, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/fake_123/reject", nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without token on reject, got %d", rec.Code)
	}

	// 2. Invalid Token -> 401 Unauthorized
	req = httptest.NewRequest(http.MethodGet, "/api/admin/sponsors", nil)
	req.Header.Set("Authorization", "Bearer invalid-tampered-token")
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized with bad token, got %d", rec.Code)
	}

	// 3. Valid Owner Token -> 200 OK
	req = httptest.NewRequest(http.MethodGet, "/api/admin/sponsors", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid owner token, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Also works on /api/v1/admin/sponsors
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/sponsors", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/v1/admin/sponsors, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSponsors_AtomicApproveAndLeaderboardLive(t *testing.T) {
	s, token, cleanup := setupTestServerWithAdminAuth(t)
	defer cleanup()
	ctx := t.Context()

	// 1. Create a legitimate founding sponsor
	sp, err := s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "DevOps Studio Pro",
		WebsiteURL:   "https://devopsstudio.pro",
		Slogan:       "High Performance Deployment",
		ContactEmail: "founder@devopsstudio.pro",
		ContactWA:    "081234567890",
	})
	if err != nil {
		t.Fatalf("failed to create sponsor: %v", err)
	}

	// 2. Process paid order via Dojo webhook
	webhookBody := map[string]interface{}{
		"order_id":   "dojo_legit_001",
		"amount_idr": 2500000,
		"status":     "paid",
		"sponsor_id": sp.ID,
	}
	wb, _ := json.Marshal(webhookBody)
	reqW := httptest.NewRequest(http.MethodPost, "/api/webhooks/dojo", bytes.NewReader(wb))
	reqW.Header.Set("Content-Type", "application/json")
	reqW.Header.Set("X-Dojo-Secret", "test-dojo-secret-xyz")
	recW := httptest.NewRecorder()
	s.Router().ServeHTTP(recW, reqW)
	if recW.Code != http.StatusOK {
		t.Fatalf("webhook failed: %d - %s", recW.Code, recW.Body.String())
	}

	// 3. DoD Verification: Pending sponsor MUST NOT leak to public leaderboard
	reqPublic := httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	recPublic := httptest.NewRecorder()
	s.Router().ServeHTTP(recPublic, reqPublic)
	var pubResp map[string]interface{}
	_ = json.NewDecoder(recPublic.Body).Decode(&pubResp)
	if int(pubResp["total"].(float64)) != 0 {
		t.Fatalf("DoD VIOLATION: pending_review sponsor leaked to public endpoint! count: %v", pubResp["total"])
	}

	// 4. Verify admin list contains the sponsor with amount and paid date
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/admin/sponsors?status=pending_review", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+token)
	recAdmin := httptest.NewRecorder()
	s.Router().ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Fatalf("admin list failed: %d", recAdmin.Code)
	}

	var adminResp map[string]interface{}
	_ = json.NewDecoder(recAdmin.Body).Decode(&adminResp)
	if int(adminResp["pending_count"].(float64)) != 1 {
		t.Fatalf("expected pending_count 1, got %v", adminResp["pending_count"])
	}

	sponsorsList := adminResp["sponsors"].([]interface{})
	if len(sponsorsList) != 1 {
		t.Fatalf("expected 1 pending sponsor, got %d", len(sponsorsList))
	}
	spData := sponsorsList[0].(map[string]interface{})
	if spData["brand_name"] != "DevOps Studio Pro" {
		t.Errorf("expected brand DevOps Studio Pro, got %v", spData["brand_name"])
	}
	if int64(spData["amount_idr"].(float64)) != 2500000 {
		t.Errorf("expected amount_idr 2500000, got %v", spData["amount_idr"])
	}

	// 5. Owner approves sponsor
	reqApprove := httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/"+sp.ID+"/approve", nil)
	reqApprove.Header.Set("Authorization", "Bearer "+token)
	recApprove := httptest.NewRecorder()
	s.Router().ServeHTTP(recApprove, reqApprove)
	if recApprove.Code != http.StatusOK {
		t.Fatalf("approve failed: %d - %s", recApprove.Code, recApprove.Body.String())
	}

	var approveResp map[string]interface{}
	_ = json.NewDecoder(recApprove.Body).Decode(&approveResp)
	if approveResp["status"] != "approved" || approveResp["actor"] != "owner@cuan.dev" {
		t.Fatalf("unexpected approve response: %+v", approveResp)
	}

	// 6. DoD Verification: Now sponsor is live in public leaderboard!
	reqPublic2 := httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	recPublic2 := httptest.NewRecorder()
	s.Router().ServeHTTP(recPublic2, reqPublic2)
	var pubResp2 map[string]interface{}
	_ = json.NewDecoder(recPublic2.Body).Decode(&pubResp2)
	if int(pubResp2["total"].(float64)) != 1 {
		t.Fatalf("expected 1 sponsor on public leaderboard after approval, got %v", pubResp2["total"])
	}

	// 7. Verify audit activities trail
	reqAct := httptest.NewRequest(http.MethodGet, "/api/admin/sponsors/"+sp.ID+"/activities", nil)
	reqAct.Header.Set("Authorization", "Bearer "+token)
	recAct := httptest.NewRecorder()
	s.Router().ServeHTTP(recAct, reqAct)
	if recAct.Code != http.StatusOK {
		t.Fatalf("get activities failed: %d", recAct.Code)
	}

	var actResp map[string]interface{}
	_ = json.NewDecoder(recAct.Body).Decode(&actResp)
	acts := actResp["activities"].([]interface{})
	if len(acts) == 0 {
		t.Fatalf("expected audit activities recorded, got 0")
	}

	// Find the approve action
	foundApprove := false
	for _, a := range acts {
		item := a.(map[string]interface{})
		if item["action"] == sponsorship.ActivityActionSponsorApprove {
			foundApprove = true
			if item["actor"] != "owner@cuan.dev" {
				t.Errorf("expected actor owner@cuan.dev, got %v", item["actor"])
			}
		}
	}
	if !foundApprove {
		t.Errorf("sponsor_approved activity not found in audit trail")
	}
}

func TestAdminSponsors_AtomicRejectWithReasonAndJudolBlacklist(t *testing.T) {
	s, token, cleanup := setupTestServerWithAdminAuth(t)
	defer cleanup()
	ctx := t.Context()

	// 1. Create a suspicious judol sponsor
	sp, err := s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "Slot Gacor 888",
		WebsiteURL:   "https://judol888.com",
		Slogan:       "Menang Mudah Maxwin",
		ContactEmail: "admin@judol888.com",
	})
	if err != nil {
		t.Fatalf("failed to create judol sponsor: %v", err)
	}

	// 2. Webhook confirms payment
	webhookBody := map[string]interface{}{
		"order_id":   "dojo_judol_999",
		"amount_idr": 10000000,
		"status":     "paid",
		"sponsor_id": sp.ID,
	}
	wb, _ := json.Marshal(webhookBody)
	reqW := httptest.NewRequest(http.MethodPost, "/api/webhooks/dojo", bytes.NewReader(wb))
	reqW.Header.Set("Content-Type", "application/json")
	reqW.Header.Set("X-Dojo-Secret", "test-dojo-secret-xyz")
	recW := httptest.NewRecorder()
	s.Router().ServeHTTP(recW, reqW)

	// 3. DoD Verification: Pending judol sponsor MUST NOT leak to public leaderboard
	reqPub := httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	recPub := httptest.NewRecorder()
	s.Router().ServeHTTP(recPub, reqPub)
	var pubResp map[string]interface{}
	_ = json.NewDecoder(recPub.Body).Decode(&pubResp)
	if int(pubResp["total"].(float64)) != 0 {
		t.Fatalf("DoD VIOLATION: judol sponsor leaked to public endpoint!")
	}

	// 4. Owner rejects with anti-judol violation reason
	rejectBody := map[string]string{
		"reason": "Terindikasi situs judi online (judol/slot). Karantina blacklist anti-judol.",
	}
	rb, _ := json.Marshal(rejectBody)
	reqReject := httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/"+sp.ID+"/reject", bytes.NewReader(rb))
	reqReject.Header.Set("Authorization", "Bearer "+token)
	reqReject.Header.Set("Content-Type", "application/json")
	recReject := httptest.NewRecorder()
	s.Router().ServeHTTP(recReject, reqReject)

	if recReject.Code != http.StatusOK {
		t.Fatalf("reject failed: %d - %s", recReject.Code, recReject.Body.String())
	}

	var rejectResp map[string]interface{}
	_ = json.NewDecoder(recReject.Body).Decode(&rejectResp)
	if rejectResp["status"] != "rejected" {
		t.Errorf("expected status rejected, got %v", rejectResp["status"])
	}
	if rejectResp["reason"] != rejectBody["reason"] {
		t.Errorf("expected reason %s, got %v", rejectBody["reason"], rejectResp["reason"])
	}

	// 5. DoD Verification: Rejected sponsor STILL 100% excluded from public leaderboard
	reqPub2 := httptest.NewRequest(http.MethodGet, "/api/sponsors/current", nil)
	recPub2 := httptest.NewRecorder()
	s.Router().ServeHTTP(recPub2, reqPub2)
	var pubResp2 map[string]interface{}
	_ = json.NewDecoder(recPub2.Body).Decode(&pubResp2)
	if int(pubResp2["total"].(float64)) != 0 {
		t.Fatalf("DoD VIOLATION: rejected judol sponsor appeared on public leaderboard!")
	}

	// 6. Verify audit activity recorded with reason and actor
	reqAct := httptest.NewRequest(http.MethodGet, "/api/admin/sponsors/"+sp.ID+"/activities", nil)
	reqAct.Header.Set("Authorization", "Bearer "+token)
	recAct := httptest.NewRecorder()
	s.Router().ServeHTTP(recAct, reqAct)

	var actResp map[string]interface{}
	_ = json.NewDecoder(recAct.Body).Decode(&actResp)
	acts := actResp["activities"].([]interface{})
	foundReject := false
	for _, a := range acts {
		item := a.(map[string]interface{})
		if item["action"] == sponsorship.ActivityActionSponsorReject {
			foundReject = true
			if item["actor"] != "owner@cuan.dev" {
				t.Errorf("expected actor owner@cuan.dev, got %v", item["actor"])
			}
		}
	}
	if !foundReject {
		t.Errorf("expected sponsor_rejected in audit trail")
	}
}

func TestAdminSponsors_FilterTabsAndCounters(t *testing.T) {
	s, token, cleanup := setupTestServerWithAdminAuth(t)
	defer cleanup()
	ctx := t.Context()

	// Create 1 approved, 1 pending, 1 rejected
	sp1, _ := s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "Brand Approved",
		WebsiteURL:   "https://approved.com",
		ContactEmail: "a@approved.com",
	})
	_ = s.SponsorshipService().ApproveSponsor(ctx, sp1.ID, "owner@cuan.dev")

	_, _ = s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "Brand Pending",
		WebsiteURL:   "https://pending.com",
		ContactEmail: "p@pending.com",
	})

	sp3, _ := s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "Brand Rejected",
		WebsiteURL:   "https://rejected.com",
		ContactEmail: "r@rejected.com",
	})
	_ = s.SponsorshipService().RejectSponsor(ctx, sp3.ID, "Judol violation", "owner@cuan.dev")

	// 1. Query All
	req := httptest.NewRequest(http.MethodGet, "/api/admin/sponsors?status=all", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	var respAll map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&respAll)
	if int(respAll["total_count"].(float64)) != 3 {
		t.Errorf("expected total_count 3, got %v", respAll["total_count"])
	}
	if int(respAll["pending_count"].(float64)) != 1 {
		t.Errorf("expected pending_count 1, got %v", respAll["pending_count"])
	}
	if int(respAll["approved_count"].(float64)) != 1 {
		t.Errorf("expected approved_count 1, got %v", respAll["approved_count"])
	}
	if int(respAll["rejected_count"].(float64)) != 1 {
		t.Errorf("expected rejected_count 1, got %v", respAll["rejected_count"])
	}
	if int(respAll["count"].(float64)) != 3 {
		t.Errorf("expected count 3 for status=all, got %v", respAll["count"])
	}

	// 2. Query Pending only
	req = httptest.NewRequest(http.MethodGet, "/api/admin/sponsors?status=pending_review", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	var respPending map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&respPending)
	if int(respPending["count"].(float64)) != 1 {
		t.Errorf("expected count 1 for pending_review, got %v", respPending["count"])
	}
	pSponsors := respPending["sponsors"].([]interface{})
	if pSponsors[0].(map[string]interface{})["brand_name"] != "Brand Pending" {
		t.Errorf("expected Brand Pending, got %v", pSponsors[0])
	}
}
