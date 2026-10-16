package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/sponsorship"
)

func TestReadmeSync_AdminApproveTriggersSync(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-server-sync-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	readmePath := filepath.Join(tempDir, "README.md")
	initialReadme := `# Cuan

> Agentic project management for self-hosted teams.

## License
Apache-2.0
`
	if err := os.WriteFile(readmePath, []byte(initialReadme), 0644); err != nil {
		t.Fatalf("failed to write initial readme: %v", err)
	}

	s, token, cleanup := setupTestServerWithAdminAuth(t)
	defer cleanup()

	// Configure syncer pointing to tempDir
	syncer := sponsorship.NewSyncer(s.SponsorshipService().Store(), sponsorship.SyncerConfig{
		RepoDir:            tempDir,
		ReadmePath:         readmePath,
		SponsorsArchiveDir: filepath.Join(tempDir, "sponsors"),
		BatchSlug:          "v1.0",
		AuthorName:         "Buchin",
		AuthorEmail:        "buchin@dropsugar.com",
	})
	s.SponsorshipService().SetSyncer(syncer)

	ctx := context.Background()

	// 1. Create a sponsor and paid order
	sp, err := s.SponsorshipService().CreateSponsor(ctx, sponsorship.CreateSponsorRequest{
		BrandName:    "SuperHost Pro",
		WebsiteURL:   "https://superhost.pro",
		Slogan:       "High Performance Cloud Hosting",
		ContactEmail: "contact@superhost.pro",
	})
	if err != nil {
		t.Fatalf("create sponsor failed: %v", err)
	}

	// Create paid order for sponsor
	_, err = s.SponsorshipService().CreateOrder(ctx, sponsorship.CreateOrderRequest{
		SponsorID:   &sp.ID,
		DojoOrderID: "dojo_order_superhost_999",
		AmountIDR:   5000000,
		Status:      sponsorship.OrderStatusPaid,
	})
	if err != nil {
		t.Fatalf("create paid order failed: %v", err)
	}

	// 2. Approve sponsor via HTTP endpoint (Single-owner auth)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/"+sp.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from approve, got %d: %s", rec.Code, rec.Body.String())
	}

	var approveResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &approveResp); err != nil {
		t.Fatalf("unmarshal approve response: %v", err)
	}
	if approveResp["readme_synced"] != true {
		t.Errorf("expected readme_synced: true, got %v", approveResp["readme_synced"])
	}

	// 3. Verify README.md on disk was automatically updated
	readmeBytes, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read updated readme failed: %v", err)
	}
	readmeStr := string(readmeBytes)

	if !strings.Contains(readmeStr, sponsorship.MarkerSponsorsStart) {
		t.Fatalf("README missing start marker")
	}
	if !strings.Contains(readmeStr, "SuperHost Pro") {
		t.Fatalf("README missing approved sponsor name")
	}
	if !strings.Contains(readmeStr, "Rp 5.000.000") {
		t.Fatalf("README missing formatted contribution amount")
	}
	if !strings.Contains(readmeStr, "## License") {
		t.Fatalf("original license section was corrupted or deleted")
	}

	// 4. Verify permanent archive directory /sponsors was generated
	archiveReadmePath := filepath.Join(tempDir, "sponsors", "README.md")
	if _, err := os.Stat(archiveReadmePath); err != nil {
		t.Fatalf("sponsors/README.md was not generated: %v", err)
	}
	batchArchivePath := filepath.Join(tempDir, "sponsors", "v1.0.md")
	batchBytes, err := os.ReadFile(batchArchivePath)
	if err != nil {
		t.Fatalf("sponsors/v1.0.md was not generated: %v", err)
	}
	if !strings.Contains(string(batchBytes), "SuperHost Pro") {
		t.Fatalf("sponsors/v1.0.md missing sponsor details")
	}
}

func TestReadmeSync_ManualSyncEndpointAndAuthGuard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-server-manual-sync-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	readmePath := filepath.Join(tempDir, "README.md")
	_ = os.WriteFile(readmePath, []byte("# Test Repo\n\n## License\nMIT\n"), 0644)

	s, token, cleanup := setupTestServerWithAdminAuth(t)
	defer cleanup()

	syncer := sponsorship.NewSyncer(s.SponsorshipService().Store(), sponsorship.SyncerConfig{
		RepoDir:            tempDir,
		ReadmePath:         readmePath,
		SponsorsArchiveDir: filepath.Join(tempDir, "sponsors"),
		BatchSlug:          "v1.0",
	})
	s.SponsorshipService().SetSyncer(syncer)

	// 1. Without Auth -> 401 Unauthorized
	reqNoAuth := httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/sync-readme", nil)
	recNoAuth := httptest.NewRecorder()
	s.Router().ServeHTTP(recNoAuth, reqNoAuth)
	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without auth token, got %d", recNoAuth.Code)
	}

	// 2. With Invalid Auth -> 401 Unauthorized
	reqBadAuth := httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/sync-readme", nil)
	reqBadAuth.Header.Set("Authorization", "Bearer invalid-token-xyz")
	recBadAuth := httptest.NewRecorder()
	s.Router().ServeHTTP(recBadAuth, reqBadAuth)
	if recBadAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized with bad token, got %d", recBadAuth.Code)
	}

	// 3. With Valid Owner Auth -> 200 OK
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/admin/sponsors/sync-readme", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+token)
	recAuth := httptest.NewRecorder()
	s.Router().ServeHTTP(recAuth, reqAuth)
	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid owner token, got %d: %s", recAuth.Code, recAuth.Body.String())
	}

	var syncRes sponsorship.SyncResult
	if err := json.Unmarshal(recAuth.Body.Bytes(), &syncRes); err != nil {
		t.Fatalf("failed to parse sync result JSON: %v", err)
	}
	if !syncRes.Success {
		t.Fatalf("expected sync success to be true")
	}

	// 4. Verify alias /api/v1/admin/sponsors/sync-readme
	reqV1 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/sponsors/sync-readme", nil)
	reqV1.Header.Set("Authorization", "Bearer "+token)
	recV1 := httptest.NewRecorder()
	s.Router().ServeHTTP(recV1, reqV1)
	if recV1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/v1/admin/sponsors/sync-readme, got %d: %s", recV1.Code, recV1.Body.String())
	}
}
