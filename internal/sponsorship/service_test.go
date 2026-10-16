package sponsorship

import (
	"context"
	"os"
	"testing"
	"time"
)

func setupTestService(t *testing.T) (*Service, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "cuan-sponsorship-svc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	storage, err := NewLogoStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to create logo storage: %v", err)
	}

	store := NewMemoryStore()
	svc := NewService(store, storage, "dojo-test-secret-12345")

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	return svc, cleanup
}

func TestSponsorshipBatches(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	batch, err := svc.GetActiveBatch(ctx)
	if err != nil {
		t.Fatalf("failed to get active batch: %v", err)
	}
	if batch.BatchSlug != "v1.0" {
		t.Fatalf("expected batch slug v1.0, got %s", batch.BatchSlug)
	}
	if batch.Status != BatchStatusOpen {
		t.Fatalf("expected batch status open, got %s", batch.Status)
	}
}

func TestSponsorIntakeAndModeration(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create sponsor
	sp, err := svc.CreateSponsor(ctx, CreateSponsorRequest{
		BrandName:    "Acme Cloud",
		WebsiteURL:   "https://acmecloud.com",
		Slogan:       "Deploy fast and scale",
		LogoPath:     "data/sponsors/logos/logo_acme.png",
		ContactEmail: "founder@acmecloud.com",
		ContactWA:    "081234567890",
	})
	if err != nil {
		t.Fatalf("failed to create sponsor: %v", err)
	}
	if sp.Status != SponsorStatusPendingReview {
		t.Fatalf("expected initial status pending_review, got %s", sp.Status)
	}

	// Validation checks
	_, err = svc.CreateSponsor(ctx, CreateSponsorRequest{
		BrandName:    "",
		WebsiteURL:   "https://valid.com",
		ContactEmail: "valid@email.com",
	})
	if err != ErrInvalidBrandName {
		t.Fatalf("expected ErrInvalidBrandName, got: %v", err)
	}

	_, err = svc.CreateSponsor(ctx, CreateSponsorRequest{
		BrandName:    "Valid",
		WebsiteURL:   "https://valid.com",
		ContactEmail: "not-an-email",
	})
	if err != ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got: %v", err)
	}

	// 2. Reject sponsor (Anti-Judol)
	err = svc.RejectSponsor(ctx, sp.ID, "Dilarang mempromosikan platform judol", "owner")
	if err != nil {
		t.Fatalf("failed to reject sponsor: %v", err)
	}
	updated, err := svc.GetSponsor(ctx, sp.ID)
	if err != nil {
		t.Fatalf("failed to get updated sponsor: %v", err)
	}
	if updated.Status != SponsorStatusRejected {
		t.Fatalf("expected status rejected, got %s", updated.Status)
	}
	if updated.RejectionReason != "Dilarang mempromosikan platform judol" {
		t.Fatalf("expected rejection reason match, got %s", updated.RejectionReason)
	}

	// 3. Approve sponsor
	err = svc.ApproveSponsor(ctx, sp.ID, "owner")
	if err != nil {
		t.Fatalf("failed to approve sponsor: %v", err)
	}
	updated, err = svc.GetSponsor(ctx, sp.ID)
	if err != nil || updated.Status != SponsorStatusApproved {
		t.Fatalf("expected status approved, got %s", updated.Status)
	}
}

func TestDojoWebhookReceiverAndIdempotency(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Register sponsor first
	sp, err := svc.CreateSponsor(ctx, CreateSponsorRequest{
		BrandName:    "Dojo Backer",
		WebsiteURL:   "https://dojobacker.dev",
		ContactEmail: "backer@dojo.dev",
	})
	if err != nil {
		t.Fatalf("failed to create sponsor: %v", err)
	}

	// 1. Invalid secret -> rejected
	_, err = svc.ProcessDojoWebhook(ctx, "wrong-secret", &DojoWebhookPayload{
		OrderID: "dojo_ord_1001",
		Amount:  500000,
	})
	if err != ErrInvalidWebhookSecret {
		t.Fatalf("expected ErrInvalidWebhookSecret, got %v", err)
	}

	// 2. Missing order_id -> rejected
	_, err = svc.ProcessDojoWebhook(ctx, "dojo-test-secret-12345", &DojoWebhookPayload{
		OrderID: "",
		Amount:  500000,
	})
	if err != ErrMissingOrderID {
		t.Fatalf("expected ErrMissingOrderID, got %v", err)
	}

	// 3. First webhook delivery: order success
	res1, err := svc.ProcessDojoWebhook(ctx, "dojo-test-secret-12345", &DojoWebhookPayload{
		OrderID:   "dojo_ord_1001",
		AmountIDR: 2500000,
		Status:    "paid",
		SponsorID: sp.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error on first webhook delivery: %v", err)
	}
	if !res1.Processed || res1.AlreadyProcessed {
		t.Fatalf("expected processed=true, already_processed=false, got %+v", res1)
	}
	if res1.AmountIDR != 2500000 {
		t.Fatalf("expected amount 2500000, got %d", res1.AmountIDR)
	}
	if res1.SponsorStatus != SponsorStatusPendingReview {
		t.Fatalf("expected sponsor status pending_review, got %s", res1.SponsorStatus)
	}

	// Verify order in store
	ord, err := svc.GetOrderByDojoID(ctx, "dojo_ord_1001")
	if err != nil || ord.Status != OrderStatusPaid {
		t.Fatalf("expected order in DB with status paid: %v", err)
	}

	// 4. Second webhook delivery (Network retry / duplicate delivery): Idempotency check
	res2, err := svc.ProcessDojoWebhook(ctx, "dojo-test-secret-12345", &DojoWebhookPayload{
		OrderID:   "dojo_ord_1001",
		AmountIDR: 2500000,
		Status:    "paid",
		SponsorID: sp.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error on retry delivery: %v", err)
	}
	if res2.Processed || !res2.AlreadyProcessed {
		t.Fatalf("expected idempotent result: processed=false, already_processed=true, got %+v", res2)
	}
	if res2.DojoOrderID != "dojo_ord_1001" {
		t.Fatalf("expected matching dojo order id, got %s", res2.DojoOrderID)
	}

	// 5. Existing Pending Order Transition
	pendingOrd, err := svc.CreateOrder(ctx, CreateOrderRequest{
		SponsorID:   &sp.ID,
		DojoOrderID: "dojo_ord_precreated",
		AmountIDR:   100000,
		Status:      OrderStatusPending,
	})
	if err != nil {
		t.Fatalf("failed to create pending order: %v", err)
	}
	if pendingOrd.Status != OrderStatusPending {
		t.Fatalf("expected pending order status, got %s", pendingOrd.Status)
	}

	res3, err := svc.ProcessDojoWebhook(ctx, "dojo-test-secret-12345", &DojoWebhookPayload{
		OrderID:   "dojo_ord_precreated",
		AmountIDR: 100000,
		Status:    "paid",
	})
	if err != nil {
		t.Fatalf("failed to process pending order webhook: %v", err)
	}
	if !res3.Processed || res3.AlreadyProcessed {
		t.Fatalf("expected processed=true on pending order update, got %+v", res3)
	}

	// Verify pending order is now paid
	ordUpdated, err := svc.GetOrderByDojoID(ctx, "dojo_ord_precreated")
	if err != nil || ordUpdated.Status != OrderStatusPaid {
		t.Fatalf("expected order updated to paid, got %+v", ordUpdated)
	}
}

func TestLeaderboardOrdering(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create 3 sponsors
	spA, _ := svc.CreateSponsor(ctx, CreateSponsorRequest{BrandName: "Brand A", WebsiteURL: "https://a.com", ContactEmail: "a@a.com"})
	spB, _ := svc.CreateSponsor(ctx, CreateSponsorRequest{BrandName: "Brand B", WebsiteURL: "https://b.com", ContactEmail: "b@b.com"})
	spC, _ := svc.CreateSponsor(ctx, CreateSponsorRequest{BrandName: "Brand C", WebsiteURL: "https://c.com", ContactEmail: "c@c.com"})

	// Orders:
	// spA: 500,000 paid at T1
	// spB: 1,000,000 paid at T2 (Higher amount -> Rank 1)
	// spC: 500,000 paid at T3 (Same amount as spA, but later -> Rank 3)
	t1 := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 16, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	_ = svc.store.CreateOrder(ctx, &Order{
		ID:          "ord_a",
		SponsorID:   &spA.ID,
		DojoOrderID: "dojo_a",
		AmountIDR:   500000,
		Status:      OrderStatusPaid,
		PaidAt:      &t1,
	})
	_ = svc.store.CreateOrder(ctx, &Order{
		ID:          "ord_b",
		SponsorID:   &spB.ID,
		DojoOrderID: "dojo_b",
		AmountIDR:   1000000,
		Status:      OrderStatusPaid,
		PaidAt:      &t2,
	})
	_ = svc.store.CreateOrder(ctx, &Order{
		ID:          "ord_c",
		SponsorID:   &spC.ID,
		DojoOrderID: "dojo_c",
		AmountIDR:   500000,
		Status:      OrderStatusPaid,
		PaidAt:      &t3,
	})

	// Non-approved sponsors must NOT appear on public leaderboard
	board, err := svc.ListApprovedSponsors(ctx, "v1.0")
	if err != nil {
		t.Fatalf("failed to list approved sponsors: %v", err)
	}
	if len(board) != 0 {
		t.Fatalf("expected 0 approved sponsors before approval, got %d", len(board))
	}

	// Approve all 3
	_ = svc.ApproveSponsor(ctx, spA.ID, "owner")
	_ = svc.ApproveSponsor(ctx, spB.ID, "owner")
	_ = svc.ApproveSponsor(ctx, spC.ID, "owner")

	board, err = svc.ListApprovedSponsors(ctx, "v1.0")
	if err != nil {
		t.Fatalf("failed to list approved sponsors: %v", err)
	}
	if len(board) != 3 {
		t.Fatalf("expected 3 approved sponsors, got %d", len(board))
	}

	// Expected ranking:
	// 1: Brand B (1,000,000)
	// 2: Brand A (500,000, paid earlier at t1)
	// 3: Brand C (500,000, paid later at t3)
	if board[0].BrandName != "Brand B" {
		t.Errorf("expected Rank 1 to be Brand B, got %s", board[0].BrandName)
	}
	if board[1].BrandName != "Brand A" {
		t.Errorf("expected Rank 2 to be Brand A, got %s", board[1].BrandName)
	}
	if board[2].BrandName != "Brand C" {
		t.Errorf("expected Rank 3 to be Brand C, got %s", board[2].BrandName)
	}
}

func TestWaitingListLeads(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Valid lead
	lead, err := svc.SubmitWaitingListLead(ctx, "prospect@example.com", "hero_cta")
	if err != nil {
		t.Fatalf("failed to submit lead: %v", err)
	}
	if lead.Email != "prospect@example.com" {
		t.Fatalf("expected email match, got %s", lead.Email)
	}
	if lead.Status != LeadStatusPending {
		t.Fatalf("expected status pending, got %s", lead.Status)
	}

	// Invalid email
	_, err = svc.SubmitWaitingListLead(ctx, "invalid-email", "hero_cta")
	if err != ErrInvalidEmail {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}

	// List leads
	leads, err := svc.ListWaitingListLeads(ctx)
	if err != nil || len(leads) != 1 {
		t.Fatalf("expected 1 lead in store, got %d, err: %v", len(leads), err)
	}
}
