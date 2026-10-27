package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBatchCampaign_ValidationAndNormalization(t *testing.T) {
	// Status normalization tests
	testCases := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{"", CampaignStatusDraft, false},
		{"draft", CampaignStatusDraft, false},
		{"Draft", CampaignStatusDraft, false},
		{"PENDING", CampaignStatusPending, false},
		{"Running", CampaignStatusRunning, false},
		{"completed", CampaignStatusCompleted, false},
		{"FAILED", CampaignStatusFailed, false},
		{"paused", CampaignStatusPaused, false},
		{"canceled", CampaignStatusCanceled, false},
		{"invalid_status", "", true},
		{"xyz123", "", true},
	}

	for _, tc := range testCases {
		norm, err := NormalizeCampaignStatus(tc.input)
		if tc.expectError {
			if err == nil {
				t.Errorf("NormalizeCampaignStatus(%q) expected error, got nil", tc.input)
			}
			if !errors.Is(err, ErrInvalidCampaignStatus) {
				t.Errorf("NormalizeCampaignStatus(%q) expected ErrInvalidCampaignStatus, got %v", tc.input, err)
			}
		} else {
			if err != nil {
				t.Errorf("NormalizeCampaignStatus(%q) unexpected error: %v", tc.input, err)
			}
			if norm != tc.expected {
				t.Errorf("NormalizeCampaignStatus(%q) expected %s, got %s", tc.input, tc.expected, norm)
			}
		}
	}

	// JSON payload validation tests
	validJSON := json.RawMessage(`{"tags": ["wordpress", "adsense"], "action": "update_code"}`)
	if err := ValidateJSON(validJSON); err != nil {
		t.Errorf("ValidateJSON() expected valid, got error: %v", err)
	}

	emptyJSON := json.RawMessage("")
	if err := ValidateJSON(emptyJSON); err != nil {
		t.Errorf("ValidateJSON() expected valid for empty, got error: %v", err)
	}

	invalidJSON := json.RawMessage(`{"tags": [unquoted]}`)
	if err := ValidateJSON(invalidJSON); err == nil {
		t.Errorf("ValidateJSON() expected error for invalid JSON, got nil")
	} else if !errors.Is(err, ErrInvalidJSONPayload) {
		t.Errorf("ValidateJSON() expected ErrInvalidJSONPayload, got %v", err)
	}
}

func TestBatchCampaign_DTOConversion(t *testing.T) {
	// CreateBatchCampaignInput validation & conversion
	inputValid := CreateBatchCampaignInput{
		Title:           "AdSense Code Mass Rollout",
		Description:     "Rollout new AdSense tracking script to 10 WordPress projects",
		TemplatePayload: json.RawMessage(`{"script": "<script async src=\"ads.js\"></script>"}`),
		TargetCriteria:  json.RawMessage(`{"tags": ["wordpress", "adsense"]}`),
		Status:          "pending",
	}

	if err := inputValid.Validate(); err != nil {
		t.Fatalf("inputValid.Validate() unexpected error: %v", err)
	}

	entity, err := inputValid.ToEntity()
	if err != nil {
		t.Fatalf("inputValid.ToEntity() unexpected error: %v", err)
	}
	if entity.Title != "AdSense Code Mass Rollout" {
		t.Errorf("expected Title %q, got %q", "AdSense Code Mass Rollout", entity.Title)
	}
	if entity.Status != CampaignStatusPending {
		t.Errorf("expected Status %q, got %q", CampaignStatusPending, entity.Status)
	}

	// Empty title check
	inputEmptyTitle := CreateBatchCampaignInput{
		Title: "   ",
	}
	if err := inputEmptyTitle.Validate(); !errors.Is(err, ErrEmptyCampaignTitle) {
		t.Errorf("expected ErrEmptyCampaignTitle, got %v", err)
	}

	// Default status and payload fallback
	inputMinimal := CreateBatchCampaignInput{
		Title: "Minimal Campaign",
	}
	entityMin, err := inputMinimal.ToEntity()
	if err != nil {
		t.Fatalf("inputMinimal.ToEntity() error: %v", err)
	}
	if entityMin.Status != CampaignStatusDraft {
		t.Errorf("expected default Status draft, got %s", entityMin.Status)
	}
	if string(entityMin.TemplatePayload) != "{}" {
		t.Errorf("expected empty payload {}, got %s", string(entityMin.TemplatePayload))
	}
	if string(entityMin.TargetCriteria) != "{}" {
		t.Errorf("expected empty criteria {}, got %s", string(entityMin.TargetCriteria))
	}

	// UpdateBatchCampaignInput
	newTitle := "Updated Campaign Title"
	newStatus := "running"
	updateInput := UpdateBatchCampaignInput{
		Title:  &newTitle,
		Status: &newStatus,
	}
	if err := updateInput.ApplyTo(entityMin); err != nil {
		t.Fatalf("updateInput.ApplyTo() error: %v", err)
	}
	if entityMin.Title != newTitle {
		t.Errorf("expected updated title %s, got %s", newTitle, entityMin.Title)
	}
	if entityMin.Status != CampaignStatusRunning {
		t.Errorf("expected updated status running, got %s", entityMin.Status)
	}

	// ToDTO and Clone deep copy isolation
	dto := entity.ToDTO()
	if dto.Title != entity.Title {
		t.Errorf("expected DTO title %s, got %s", entity.Title, dto.Title)
	}

	clone := entity.Clone()
	if clone.Title != entity.Title {
		t.Errorf("expected clone title %s, got %s", entity.Title, clone.Title)
	}
	clone.Title = "Mutated Title"
	if entity.Title == "Mutated Title" {
		t.Errorf("mutating clone modified original entity")
	}
}

func TestMemoryBatchCampaignRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryBatchCampaignRepository()

	// 1. Create with Auto UUID
	c1 := &BatchCampaign{
		Title:           "S08.1 Verification Campaign",
		Description:     "Test automated runner and aggregator",
		TemplatePayload: json.RawMessage(`{"check": true}`),
		TargetCriteria:  json.RawMessage(`{"limit": 5}`),
		Status:          CampaignStatusDraft,
	}

	if err := repo.Create(ctx, c1); err != nil {
		t.Fatalf("repo.Create(c1) unexpected error: %v", err)
	}
	if c1.ID == "" {
		t.Fatalf("expected repo.Create to assign UUID, got empty")
	}
	if !IsValidUUID(c1.ID) {
		t.Errorf("expected valid UUID, got %s", c1.ID)
	}
	if c1.CreatedAt.IsZero() || c1.UpdatedAt.IsZero() {
		t.Errorf("expected timestamps to be set, got zero")
	}

	// 2. Create Duplicate Error
	cDuplicate := &BatchCampaign{
		ID:    c1.ID,
		Title: "Duplicate Campaign",
	}
	if err := repo.Create(ctx, cDuplicate); !errors.Is(err, ErrDuplicateCampaign) {
		t.Errorf("expected ErrDuplicateCampaign, got %v", err)
	}

	// 3. Create with Invalid UUID Error
	cBadUUID := &BatchCampaign{
		ID:    "not-a-uuid",
		Title: "Bad UUID Campaign",
	}
	if err := repo.Create(ctx, cBadUUID); !errors.Is(err, ErrInvalidUUID) {
		t.Errorf("expected ErrInvalidUUID, got %v", err)
	}

	// 4. Create with Empty Title Error
	cEmptyTitle := &BatchCampaign{
		Title: "   ",
	}
	if err := repo.Create(ctx, cEmptyTitle); !errors.Is(err, ErrEmptyCampaignTitle) {
		t.Errorf("expected ErrEmptyCampaignTitle, got %v", err)
	}

	// 5. GetByID
	fetched, err := repo.GetByID(ctx, c1.ID)
	if err != nil {
		t.Fatalf("repo.GetByID(%s) unexpected error: %v", c1.ID, err)
	}
	if fetched.Title != c1.Title {
		t.Errorf("expected Title %s, got %s", c1.Title, fetched.Title)
	}

	// GetByID not found
	nonExistentID := GenerateUUID()
	if _, err := repo.GetByID(ctx, nonExistentID); !errors.Is(err, ErrBatchCampaignNotFound) {
		t.Errorf("expected ErrBatchCampaignNotFound for ID %s, got %v", nonExistentID, err)
	}

	// 6. Update
	fetched.Title = "Renamed S08.1 Campaign"
	fetched.Status = CampaignStatusRunning
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("repo.Update() unexpected error: %v", err)
	}

	updated, err := repo.GetByID(ctx, c1.ID)
	if err != nil {
		t.Fatalf("repo.GetByID() after update error: %v", err)
	}
	if updated.Title != "Renamed S08.1 Campaign" {
		t.Errorf("expected updated title %s, got %s", "Renamed S08.1 Campaign", updated.Title)
	}
	if updated.Status != CampaignStatusRunning {
		t.Errorf("expected status running, got %s", updated.Status)
	}

	// 7. UpdateStatus
	if err := repo.UpdateStatus(ctx, c1.ID, CampaignStatusCompleted); err != nil {
		t.Fatalf("repo.UpdateStatus() unexpected error: %v", err)
	}
	afterStatus, _ := repo.GetByID(ctx, c1.ID)
	if afterStatus.Status != CampaignStatusCompleted {
		t.Errorf("expected status completed, got %s", afterStatus.Status)
	}

	// UpdateStatus invalid UUID & invalid status
	if err := repo.UpdateStatus(ctx, "invalid-uuid", CampaignStatusCompleted); !errors.Is(err, ErrInvalidUUID) {
		t.Errorf("expected ErrInvalidUUID on UpdateStatus, got %v", err)
	}
	if err := repo.UpdateStatus(ctx, c1.ID, "unknown-status"); !errors.Is(err, ErrInvalidCampaignStatus) {
		t.Errorf("expected ErrInvalidCampaignStatus, got %v", err)
	}

	// 8. Delete
	if err := repo.Delete(ctx, c1.ID); err != nil {
		t.Fatalf("repo.Delete(%s) unexpected error: %v", c1.ID, err)
	}

	// Verify deleted
	if _, err := repo.GetByID(ctx, c1.ID); !errors.Is(err, ErrBatchCampaignNotFound) {
		t.Errorf("expected ErrBatchCampaignNotFound after delete, got %v", err)
	}
	if err := repo.Delete(ctx, c1.ID); !errors.Is(err, ErrBatchCampaignNotFound) {
		t.Errorf("expected ErrBatchCampaignNotFound on second delete, got %v", err)
	}
}

func TestMemoryBatchCampaignRepository_ListAndFilter(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryBatchCampaignRepository()

	// Seed campaigns
	items := []*BatchCampaign{
		{
			Title:          "WordPress Plugin Batch Rollout",
			Description:    "Mass upgrade WooCommerce on 15 client sites",
			TargetCriteria: json.RawMessage(`{"tags": ["wordpress", "woocommerce"]}`),
			Status:         CampaignStatusDraft,
			CreatedAt:      time.Now().UTC().Add(-30 * time.Minute),
		},
		{
			Title:          "AdSense Header Tag Integration",
			Description:    "Inject responsive ad units into 10 niche websites",
			TargetCriteria: json.RawMessage(`{"tags": ["wordpress", "adsense"]}`),
			Status:         CampaignStatusRunning,
			CreatedAt:      time.Now().UTC().Add(-20 * time.Minute),
		},
		{
			Title:          "SSL Certificate Auto-Renewal Check",
			Description:    "Trigger ACME challenge check on all custom domains",
			TargetCriteria: json.RawMessage(`{"tags": ["custom_domain", "ssl"]}`),
			Status:         CampaignStatusCompleted,
			CreatedAt:      time.Now().UTC().Add(-10 * time.Minute),
		},
		{
			Title:          "WordPress Core Security Hotfix",
			Description:    "Emergency security patch rollout across all wordpress servers",
			TargetCriteria: json.RawMessage(`{"tags": ["wordpress", "security"]}`),
			Status:         CampaignStatusRunning,
			CreatedAt:      time.Now().UTC().Add(-5 * time.Minute),
		},
	}

	for _, item := range items {
		if err := repo.Create(ctx, item); err != nil {
			t.Fatalf("seed create error: %v", err)
		}
	}

	// 1. List all (no filter)
	all, err := repo.List(ctx, BatchCampaignFilter{})
	if err != nil {
		t.Fatalf("repo.List(all) error: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 campaigns, got %d", len(all))
	}
	// Verify descending order by CreatedAt
	if all[0].Title != "WordPress Core Security Hotfix" {
		t.Errorf("expected newest campaign first, got %s", all[0].Title)
	}

	// Count all
	countAll, err := repo.Count(ctx, BatchCampaignFilter{})
	if err != nil {
		t.Fatalf("repo.Count(all) error: %v", err)
	}
	if countAll != 4 {
		t.Errorf("expected count 4, got %d", countAll)
	}

	// 2. Filter by status: running
	running, err := repo.List(ctx, BatchCampaignFilter{Status: "running"})
	if err != nil {
		t.Fatalf("repo.List(running) error: %v", err)
	}
	if len(running) != 2 {
		t.Fatalf("expected 2 running campaigns, got %d", len(running))
	}

	countRunning, _ := repo.Count(ctx, BatchCampaignFilter{Status: "running"})
	if countRunning != 2 {
		t.Errorf("expected countRunning 2, got %d", countRunning)
	}

	// 3. Search substring: "adsense"
	adsenseList, err := repo.List(ctx, BatchCampaignFilter{Search: "AdSense"})
	if err != nil {
		t.Fatalf("repo.List(search adsense) error: %v", err)
	}
	if len(adsenseList) != 1 {
		t.Fatalf("expected 1 campaign matching AdSense, got %d", len(adsenseList))
	}
	if adsenseList[0].Title != "AdSense Header Tag Integration" {
		t.Errorf("expected title AdSense Header Tag Integration, got %s", adsenseList[0].Title)
	}

	// 4. Search in description: "patch rollout"
	patchList, err := repo.List(ctx, BatchCampaignFilter{Search: "patch rollout"})
	if err != nil {
		t.Fatalf("repo.List(search patch) error: %v", err)
	}
	if len(patchList) != 1 {
		t.Fatalf("expected 1 campaign matching patch rollout, got %d", len(patchList))
	}

	// 5. Pagination: Limit 2, Offset 1
	paged, err := repo.List(ctx, BatchCampaignFilter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("repo.List(paged) error: %v", err)
	}
	if len(paged) != 2 {
		t.Fatalf("expected 2 items paged, got %d", len(paged))
	}
	if paged[0].Title != all[1].Title || paged[1].Title != all[2].Title {
		t.Errorf("paged slice does not match expected offset/limit items")
	}
}

func TestMemoryBatchCampaignRepository_ConcurrencyThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryBatchCampaignRepository()

	const numGoroutines = 40
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	createdIDs := make([]string, numGoroutines)
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		idx := i
		go func() {
			defer wg.Done()
			campaign := &BatchCampaign{
				Title:           fmt.Sprintf("Concurrent Campaign %03d", idx),
				Description:     fmt.Sprintf("Testing race condition safety for worker %d", idx),
				TemplatePayload: json.RawMessage(`{"worker": true}`),
				TargetCriteria:  json.RawMessage(`{"concurrency": true}`),
				Status:          CampaignStatusDraft,
			}

			if err := repo.Create(ctx, campaign); err != nil {
				t.Errorf("concurrent repo.Create failed: %v", err)
				return
			}

			mu.Lock()
			createdIDs[idx] = campaign.ID
			mu.Unlock()

			// Concurrently read it
			if _, err := repo.GetByID(ctx, campaign.ID); err != nil {
				t.Errorf("concurrent repo.GetByID failed: %v", err)
			}

			// Concurrently update status
			if err := repo.UpdateStatus(ctx, campaign.ID, CampaignStatusRunning); err != nil {
				t.Errorf("concurrent repo.UpdateStatus failed: %v", err)
			}

			// Concurrently list
			if _, err := repo.List(ctx, BatchCampaignFilter{Limit: 10}); err != nil {
				t.Errorf("concurrent repo.List failed: %v", err)
			}

			// Concurrently count
			if _, err := repo.Count(ctx, BatchCampaignFilter{Status: "running"}); err != nil {
				t.Errorf("concurrent repo.Count failed: %v", err)
			}
		}()
	}

	wg.Wait()

	count, err := repo.Count(ctx, BatchCampaignFilter{})
	if err != nil {
		t.Fatalf("repo.Count() error after concurrency: %v", err)
	}
	if count != numGoroutines {
		t.Errorf("expected count %d, got %d", numGoroutines, count)
	}

	// Verify all items are now running
	runningCount, _ := repo.Count(ctx, BatchCampaignFilter{Status: CampaignStatusRunning})
	if runningCount != numGoroutines {
		t.Errorf("expected all %d campaigns to be in running status, got %d", numGoroutines, runningCount)
	}
}

func TestRepositories_BatchCampaignIntegration(t *testing.T) {
	memRepos := NewMemoryRepositories()
	if memRepos.BatchCampaigns == nil {
		t.Fatalf("expected memRepos.BatchCampaigns to be non-nil")
	}

	sqlRepos := NewSQLRepositories(nil)
	if sqlRepos.BatchCampaigns == nil {
		t.Fatalf("expected sqlRepos.BatchCampaigns to be non-nil")
	}
}
