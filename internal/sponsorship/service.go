package sponsorship

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidWebhookSecret = errors.New("secret webhook Dojo tidak valid")
	ErrMissingOrderID        = errors.New("order_id tidak boleh kosong")
	ErrInvalidEmail          = errors.New("format email tidak valid")
	ErrInvalidBrandName      = errors.New("nama brand tidak boleh kosong")
	ErrInvalidWebsiteURL     = errors.New("URL website tidak valid")
)

type WebhookResult struct {
	Processed        bool   `json:"processed"`
	AlreadyProcessed bool   `json:"already_processed"`
	OrderID          string `json:"order_id"`
	DojoOrderID      string `json:"dojo_order_id"`
	AmountIDR        int64  `json:"amount_idr"`
	Status           string `json:"status"`
	SponsorStatus    string `json:"sponsor_status,omitempty"`
	Message          string `json:"message"`
}

type CreateSponsorRequest struct {
	BatchID      string `json:"batch_id"`
	BrandName    string `json:"brand_name"`
	WebsiteURL   string `json:"website_url"`
	Slogan       string `json:"slogan"`
	LogoPath     string `json:"logo_path"`
	ContactEmail string `json:"contact_email"`
	ContactWA    string `json:"contact_wa"`
}

type CreateOrderRequest struct {
	SponsorID   *string `json:"sponsor_id,omitempty"`
	DojoOrderID string  `json:"dojo_order_id"`
	AmountIDR   int64   `json:"amount_idr"`
	Status      string  `json:"status,omitempty"`
}

type Service struct {
	store         Store
	logoStorage   *LogoStorage
	webhookSecret string
}

func NewService(store Store, logoStorage *LogoStorage, webhookSecret string) *Service {
	return &Service{
		store:         store,
		logoStorage:   logoStorage,
		webhookSecret: strings.TrimSpace(webhookSecret),
	}
}

func (s *Service) WebhookSecret() string {
	return s.webhookSecret
}

func (s *Service) SetWebhookSecret(secret string) {
	s.webhookSecret = strings.TrimSpace(secret)
}

func (s *Service) LogoStorage() *LogoStorage {
	return s.logoStorage
}

// Batches
func (s *Service) GetActiveBatch(ctx context.Context) (*Batch, error) {
	b, err := s.store.GetBatchBySlug(ctx, "v1.0")
	if err == nil {
		return b, nil
	}
	// Fallback to first batch
	batches, err := s.store.ListBatches(ctx)
	if err != nil {
		return nil, err
	}
	if len(batches) == 0 {
		return nil, ErrBatchNotFound
	}
	return batches[0], nil
}

func (s *Service) ListBatches(ctx context.Context) ([]*Batch, error) {
	return s.store.ListBatches(ctx)
}

// Sponsors
func (s *Service) CreateSponsor(ctx context.Context, req CreateSponsorRequest) (*Sponsor, error) {
	brandName := strings.TrimSpace(req.BrandName)
	if brandName == "" {
		return nil, ErrInvalidBrandName
	}

	websiteURL := strings.TrimSpace(req.WebsiteURL)
	if websiteURL == "" {
		return nil, ErrInvalidWebsiteURL
	}
	if !strings.HasPrefix(websiteURL, "http://") && !strings.HasPrefix(websiteURL, "https://") {
		websiteURL = "https://" + websiteURL
	}
	if _, err := url.ParseRequestURI(websiteURL); err != nil {
		return nil, ErrInvalidWebsiteURL
	}

	email := strings.TrimSpace(req.ContactEmail)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}

	batchID := strings.TrimSpace(req.BatchID)
	if batchID == "" {
		activeBatch, err := s.GetActiveBatch(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve active batch: %w", err)
		}
		batchID = activeBatch.ID
	}

	now := time.Now().UTC()
	sp := &Sponsor{
		ID:           generateID("sp"),
		BatchID:      batchID,
		BrandName:    brandName,
		WebsiteURL:   websiteURL,
		Slogan:       strings.TrimSpace(req.Slogan),
		LogoPath:     strings.TrimSpace(req.LogoPath),
		ContactEmail: email,
		ContactWA:    strings.TrimSpace(req.ContactWA),
		Status:       SponsorStatusPendingReview,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.CreateSponsor(ctx, sp); err != nil {
		return nil, fmt.Errorf("create sponsor: %w", err)
	}

	_ = s.store.CreateActivity(ctx, &ActivityLog{
		SponsorID: sp.ID,
		Action:    ActivityActionSponsorCreated,
		Details:   fmt.Sprintf("Sponsor brand %s mendaftar", sp.BrandName),
		Actor:     "intake_form",
		CreatedAt: now,
	})

	return sp, nil
}

func (s *Service) GetSponsor(ctx context.Context, id string) (*Sponsor, error) {
	return s.store.GetSponsor(ctx, id)
}

func (s *Service) ListSponsors(ctx context.Context, batchID, status string) ([]*Sponsor, error) {
	return s.store.ListSponsorsByBatch(ctx, batchID, status)
}

func (s *Service) ListApprovedSponsors(ctx context.Context, batchSlug string) ([]*Sponsor, error) {
	return s.store.ListApprovedSponsors(ctx, batchSlug)
}

func (s *Service) ApproveSponsor(ctx context.Context, id, actor string) error {
	sp, err := s.store.GetSponsor(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.UpdateSponsorStatus(ctx, id, SponsorStatusApproved, ""); err != nil {
		return err
	}
	if actor == "" {
		actor = "owner"
	}
	_ = s.store.CreateActivity(ctx, &ActivityLog{
		SponsorID: id,
		Action:    ActivityActionSponsorApprove,
		Details:   fmt.Sprintf("Sponsor %s di-approve oleh %s (tampil di leaderboard)", sp.BrandName, actor),
		Actor:     actor,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

func (s *Service) RejectSponsor(ctx context.Context, id, reason, actor string) error {
	sp, err := s.store.GetSponsor(ctx, id)
	if err != nil {
		return err
	}
	if reason == "" {
		reason = "Melanggar etika dan pedoman anti-judol Cuan"
	}
	if err := s.store.UpdateSponsorStatus(ctx, id, SponsorStatusRejected, reason); err != nil {
		return err
	}
	if actor == "" {
		actor = "owner"
	}
	_ = s.store.CreateActivity(ctx, &ActivityLog{
		SponsorID: id,
		Action:    ActivityActionSponsorReject,
		Details:   fmt.Sprintf("Sponsor %s ditolak oleh %s. Alasan: %s", sp.BrandName, actor, reason),
		Actor:     actor,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

// Orders & Dojo Webhook
func (s *Service) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	dojoOrderID := strings.TrimSpace(req.DojoOrderID)
	if dojoOrderID == "" {
		return nil, ErrMissingOrderID
	}
	if req.AmountIDR <= 0 {
		return nil, errors.New("nominal order harus lebih besar dari 0")
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = OrderStatusPending
	}

	now := time.Now().UTC()
	ord := &Order{
		ID:          generateID("ord"),
		SponsorID:   req.SponsorID,
		DojoOrderID: dojoOrderID,
		AmountIDR:   req.AmountIDR,
		Status:      status,
		CreatedAt:   now,
	}
	if status == OrderStatusPaid {
		ord.PaidAt = &now
	}

	if err := s.store.CreateOrder(ctx, ord); err != nil {
		return nil, err
	}

	if ord.SponsorID != nil && *ord.SponsorID != "" {
		_ = s.store.CreateActivity(ctx, &ActivityLog{
			SponsorID: *ord.SponsorID,
			OrderID:   &ord.ID,
			Action:    ActivityActionOrderCreated,
			Details:   fmt.Sprintf("Order %s dibuat senilai Rp %d (status: %s)", ord.DojoOrderID, ord.AmountIDR, ord.Status),
			Actor:     "system",
			CreatedAt: now,
		})
	}

	return ord, nil
}

func (s *Service) GetOrderByDojoID(ctx context.Context, dojoOrderID string) (*Order, error) {
	return s.store.GetOrderByDojoID(ctx, dojoOrderID)
}

// ProcessDojoWebhook verifies secret, performs idempotency check, and transitions order and sponsor status.
func (s *Service) ProcessDojoWebhook(ctx context.Context, incomingSecret string, payload *DojoWebhookPayload) (*WebhookResult, error) {
	if payload == nil {
		return nil, errors.New("payload webhook kosong")
	}

	// 1. Verify Secret using constant-time comparison
	expectedSecret := s.webhookSecret
	if expectedSecret != "" {
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(incomingSecret)), []byte(expectedSecret)) != 1 {
			return nil, ErrInvalidWebhookSecret
		}
	}

	orderID := strings.TrimSpace(payload.EffectiveOrderID())
	if orderID == "" {
		return nil, ErrMissingOrderID
	}

	amountIDR := payload.EffectiveAmountIDR()

	// 2. Idempotency Check: lookup existing order
	existingOrder, err := s.store.GetOrderByDojoID(ctx, orderID)
	if err == nil && existingOrder != nil {
		// Idempotency: order already exists
		if existingOrder.Status == OrderStatusPaid {
			// Already successfully paid, safe return without re-triggering side effects
			return &WebhookResult{
				Processed:        false,
				AlreadyProcessed: true,
				OrderID:          existingOrder.ID,
				DojoOrderID:      existingOrder.DojoOrderID,
				AmountIDR:        existingOrder.AmountIDR,
				Status:           existingOrder.Status,
				SponsorStatus:    SponsorStatusPendingReview,
				Message:          "Order sudah pernah diproses sebelumnya (idempotent)",
			}, nil
		}

		// Transition pending order to paid
		now := time.Now().UTC()
		existingOrder.Status = OrderStatusPaid
		if amountIDR > 0 {
			existingOrder.AmountIDR = amountIDR
		}
		existingOrder.PaidAt = &now
		if err := s.store.UpdateOrder(ctx, existingOrder); err != nil {
			return nil, fmt.Errorf("update existing order to paid: %w", err)
		}

		// Transition sponsor to pending_review
		if existingOrder.SponsorID != nil && *existingOrder.SponsorID != "" {
			_ = s.store.UpdateSponsorStatus(ctx, *existingOrder.SponsorID, SponsorStatusPendingReview, "")
			_ = s.store.CreateActivity(ctx, &ActivityLog{
				SponsorID: *existingOrder.SponsorID,
				OrderID:   &existingOrder.ID,
				Action:    ActivityActionOrderPaid,
				Details:   fmt.Sprintf("Order %s berhasil dibayar Rp %d via Dojo webhook", existingOrder.DojoOrderID, existingOrder.AmountIDR),
				Actor:     "dojo_webhook",
				CreatedAt: now,
			})
		}

		return &WebhookResult{
			Processed:        true,
			AlreadyProcessed: false,
			OrderID:          existingOrder.ID,
			DojoOrderID:      existingOrder.DojoOrderID,
			AmountIDR:        existingOrder.AmountIDR,
			Status:           existingOrder.Status,
			SponsorStatus:    SponsorStatusPendingReview,
			Message:          "Order berhasil diperbarui ke status paid",
		}, nil
	}

	// 3. New order via webhook
	now := time.Now().UTC()
	var spID *string
	if strings.TrimSpace(payload.SponsorID) != "" {
		trimmed := strings.TrimSpace(payload.SponsorID)
		spID = &trimmed
	}

	newOrder := &Order{
		ID:          generateID("ord"),
		SponsorID:   spID,
		DojoOrderID: orderID,
		AmountIDR:   amountIDR,
		Status:      OrderStatusPaid,
		PaidAt:      &now,
		CreatedAt:   now,
	}

	if err := s.store.CreateOrder(ctx, newOrder); err != nil {
		return nil, fmt.Errorf("create order from webhook: %w", err)
	}

	// Transition sponsor to pending_review
	if spID != nil && *spID != "" {
		_ = s.store.UpdateSponsorStatus(ctx, *spID, SponsorStatusPendingReview, "")
		_ = s.store.CreateActivity(ctx, &ActivityLog{
			SponsorID: *spID,
			OrderID:   &newOrder.ID,
			Action:    ActivityActionOrderPaid,
			Details:   fmt.Sprintf("Order baru %s lunas senilai Rp %d via Dojo webhook", newOrder.DojoOrderID, newOrder.AmountIDR),
			Actor:     "dojo_webhook",
			CreatedAt: now,
		})
	}

	return &WebhookResult{
		Processed:        true,
		AlreadyProcessed: false,
		OrderID:          newOrder.ID,
		DojoOrderID:      newOrder.DojoOrderID,
		AmountIDR:        newOrder.AmountIDR,
		Status:           newOrder.Status,
		SponsorStatus:    SponsorStatusPendingReview,
		Message:          "Order berhasil dibuat dan ditandai paid",
	}, nil
}

// Waiting List
func (s *Service) SubmitWaitingListLead(ctx context.Context, email, source string) (*WaitingListLead, error) {
	trimmedEmail := strings.TrimSpace(email)
	if _, err := mail.ParseAddress(trimmedEmail); err != nil {
		return nil, ErrInvalidEmail
	}

	if source == "" {
		source = "landing_page"
	}

	lead := &WaitingListLead{
		ID:        generateID("lead"),
		Email:     trimmedEmail,
		Source:    source,
		Status:    LeadStatusPending,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.store.CreateLead(ctx, lead); err != nil {
		return nil, fmt.Errorf("save waiting list lead: %w", err)
	}

	return lead, nil
}

func (s *Service) ListWaitingListLeads(ctx context.Context) ([]*WaitingListLead, error) {
	return s.store.ListLeads(ctx)
}

// Logo saving helper
func (s *Service) SaveLogo(data []byte, filename string) (string, string, error) {
	if s.logoStorage == nil {
		return "", "", errors.New("penyimpanan logo belum terkonfigurasi")
	}
	return s.logoStorage.SaveLogo(data, filename)
}

func generateID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
