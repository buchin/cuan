package sponsorship

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidWebhookSecret = errors.New("Invalid Dojo webhook secret")
	ErrMissingOrderID       = errors.New("order_id cannot be empty")
	ErrInvalidEmail         = errors.New("Invalid email format")
	ErrInvalidBrandName     = errors.New("Brand name cannot be empty")
	ErrInvalidWebsiteURL    = errors.New("Invalid website URL")
	ErrAmountTooLow         = errors.New("Pay-What-You-Want amount must be at least IDR 10,000")
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
	AmountIDR    int64  `json:"amount_idr,omitempty"`
	ReturnURL    string `json:"return_url,omitempty"`
}

type CreateOrderRequest struct {
	SponsorID   *string `json:"sponsor_id,omitempty"`
	DojoOrderID string  `json:"dojo_order_id"`
	AmountIDR   int64   `json:"amount_idr"`
	Status      string  `json:"status,omitempty"`
}

// DojoDispatcher defines interface for forwarding waiting list signups to Dojo.
type DojoDispatcher interface {
	DispatchWaitingListSignup(ctx context.Context, lead *WaitingListLead) error
}

// HTTPDojoDispatcher dispatches waiting list signups to Dojo webhook over HTTP.
type HTTPDojoDispatcher struct {
	WebhookURL    string
	WebhookSecret string
	Client        *http.Client
}

func (d *HTTPDojoDispatcher) DispatchWaitingListSignup(ctx context.Context, lead *WaitingListLead) error {
	if d.WebhookURL == "" {
		return nil
	}
	payload := map[string]interface{}{
		"event":         "waiting_list.signup",
		"lead_id":       lead.ID,
		"email":         lead.Email,
		"source":        lead.Source,
		"timestamp":     lead.CreatedAt.UTC().Format(time.RFC3339),
		"action":        "send_welcome_email",
		"trigger_email": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Cuan-WaitingList/1.0")
	if d.WebhookSecret != "" {
		req.Header.Set("X-Dojo-Secret", d.WebhookSecret)
		req.Header.Set("Authorization", "Bearer "+d.WebhookSecret)
	}

	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dojo webhook error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("dojo webhook rejected with status %d", resp.StatusCode)
	}
	return nil
}

type Service struct {
	store           Store
	logoStorage     *LogoStorage
	webhookSecret   string
	dojoWebhookURL  string
	checkoutBaseURL string
	dojoDispatcher  DojoDispatcher
	syncer          *Syncer
}

func NewService(store Store, logoStorage *LogoStorage, webhookSecret string) *Service {
	return &Service{
		store:           store,
		logoStorage:     logoStorage,
		webhookSecret:   strings.TrimSpace(webhookSecret),
		checkoutBaseURL: "https://dojo.cc/checkout/cuan-founding-backer",
	}
}

func (s *Service) Store() Store {
	return s.store
}

func (s *Service) CheckoutBaseURL() string {
	if s.checkoutBaseURL != "" {
		return s.checkoutBaseURL
	}
	return "https://dojo.cc/checkout/cuan-founding-backer"
}

func (s *Service) SetCheckoutBaseURL(u string) {
	s.checkoutBaseURL = strings.TrimSpace(u)
}

func (s *Service) WebhookSecret() string {
	return s.webhookSecret
}

func (s *Service) SetWebhookSecret(secret string) {
	s.webhookSecret = strings.TrimSpace(secret)
	if s.dojoWebhookURL != "" {
		s.dojoDispatcher = &HTTPDojoDispatcher{
			WebhookURL:    s.dojoWebhookURL,
			WebhookSecret: s.webhookSecret,
		}
	}
}

func (s *Service) SetDojoDispatcher(dispatcher DojoDispatcher) {
	s.dojoDispatcher = dispatcher
}

func (s *Service) DojoDispatcher() DojoDispatcher {
	return s.dojoDispatcher
}

func (s *Service) SetDojoWebhookURL(url string) {
	s.dojoWebhookURL = strings.TrimSpace(url)
	if s.dojoWebhookURL != "" {
		s.dojoDispatcher = &HTTPDojoDispatcher{
			WebhookURL:    s.dojoWebhookURL,
			WebhookSecret: s.webhookSecret,
		}
	} else {
		s.dojoDispatcher = nil
	}
}

func (s *Service) DojoWebhookURL() string {
	return s.dojoWebhookURL
}

func (s *Service) LogoStorage() *LogoStorage {
	return s.logoStorage
}

func (s *Service) SetSyncer(syncer *Syncer) {
	s.syncer = syncer
}

func (s *Service) Syncer() *Syncer {
	return s.syncer
}

func (s *Service) SyncReadmeAndArchive(ctx context.Context, batchSlug string) (*SyncResult, error) {
	if s.syncer == nil {
		return &SyncResult{
			Success: false,
			Message: "Sync service is not initialized",
		}, nil
	}
	return s.syncer.Sync(ctx, batchSlug)
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
	brandName := SanitizeText(req.BrandName)
	if brandName == "" {
		return nil, ErrInvalidBrandName
	}

	websiteURL, err := ValidateAndFormatURL(req.WebsiteURL)
	if err != nil {
		return nil, err
	}

	email := strings.TrimSpace(req.ContactEmail)
	if err := ValidateContactEmail(email); err != nil {
		return nil, err
	}

	slogan := SanitizeText(req.Slogan)
	contactWA := SanitizeText(req.ContactWA)

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
		Slogan:       slogan,
		LogoPath:     strings.TrimSpace(req.LogoPath),
		ContactEmail: email,
		ContactWA:    contactWA,
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

// CreateCheckoutSession handles PWYW checkout intake, generating an order and Dojo redirect URL.
func (s *Service) CreateCheckoutSession(ctx context.Context, req CreateCheckoutRequest) (*CheckoutResult, error) {
	if req.AmountIDR < MinSponsorAmountIDR {
		return nil, ErrAmountTooLow
	}

	sp, err := s.CreateSponsor(ctx, CreateSponsorRequest{
		BatchID:      req.BatchID,
		BrandName:    req.BrandName,
		WebsiteURL:   req.WebsiteURL,
		Slogan:       req.Slogan,
		LogoPath:     req.LogoPath,
		ContactEmail: req.ContactEmail,
		ContactWA:    req.ContactWA,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	dojoOrderID := fmt.Sprintf("dojo_%s_%d", sp.ID, now.Unix())
	cuanOrderID := generateID("ord")

	order := &Order{
		ID:          cuanOrderID,
		SponsorID:   &sp.ID,
		DojoOrderID: dojoOrderID,
		AmountIDR:   req.AmountIDR,
		Status:      OrderStatusPending,
		CreatedAt:   now,
	}

	if err := s.store.CreateOrder(ctx, order); err != nil {
		return nil, fmt.Errorf("create checkout order: %w", err)
	}

	_ = s.store.CreateActivity(ctx, &ActivityLog{
		SponsorID: sp.ID,
		OrderID:   &order.ID,
		Action:    ActivityActionOrderCreated,
		Details:   fmt.Sprintf("PWYW checkout order created for IDR %d (Dojo: %s)", order.AmountIDR, dojoOrderID),
		Actor:     "checkout_flow",
		CreatedAt: now,
	})

	baseURL := s.CheckoutBaseURL()
	returnURL := strings.TrimSpace(req.ReturnURL)
	if returnURL == "" {
		returnURL = "/sponsor/pending?order_id=" + dojoOrderID
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		u, _ = url.Parse("https://dojo.cc/checkout/cuan-founding-backer")
	}
	q := u.Query()
	q.Set("order_id", dojoOrderID)
	q.Set("cuan_order_id", order.ID)
	q.Set("sponsor_id", sp.ID)
	q.Set("amount", strconv.FormatInt(req.AmountIDR, 10))
	q.Set("brand", sp.BrandName)
	q.Set("return_url", returnURL)
	u.RawQuery = q.Encode()

	return &CheckoutResult{
		Status:       "success",
		CheckoutURL:  u.String(),
		DojoOrderID:  dojoOrderID,
		OrderID:      order.ID,
		AmountIDR:    req.AmountIDR,
		Sponsor:      sp,
		Order:        order,
		MinAmountIDR: MinSponsorAmountIDR,
		PendingURL:   "/sponsor/pending?order_id=" + dojoOrderID,
		Message:      "Dojo checkout session is ready",
	}, nil
}

// GetSponsorshipStatus checks current payment and curation status for an order or sponsor.
func (s *Service) GetSponsorshipStatus(ctx context.Context, orderID, sponsorID string) (*SponsorshipStatusResult, error) {
	var ord *Order
	var sp *Sponsor
	var err error

	orderID = strings.TrimSpace(orderID)
	sponsorID = strings.TrimSpace(sponsorID)

	if orderID != "" {
		ord, err = s.store.GetOrderByDojoID(ctx, orderID)
		if err != nil || ord == nil {
			ord, err = s.store.GetOrder(ctx, orderID)
		}
	}

	if ord != nil && ord.SponsorID != nil && *ord.SponsorID != "" {
		sp, _ = s.store.GetSponsor(ctx, *ord.SponsorID)
	} else if sponsorID != "" {
		sp, _ = s.store.GetSponsor(ctx, sponsorID)
	}

	if ord == nil && sp == nil {
		return &SponsorshipStatusResult{
			Found:   false,
			Status:  "not_found",
			Message: "Order or sponsor not found",
		}, nil
	}

	status := "pending_payment"
	message := "Waiting for payment confirmation from Dojo"

	if ord != nil && ord.Status == OrderStatusPaid {
		status = "pending_review"
		message = "Payment verified. The sponsor profile is awaiting owner moderation."
	}
	if sp != nil {
		if sp.Status == SponsorStatusApproved {
			status = "approved"
			message = "Sponsor profile approved and published on the public leaderboard."
		} else if sp.Status == SponsorStatusRejected {
			status = "rejected"
			message = "Sponsor profile rejected: " + sp.RejectionReason
		}
	}

	return &SponsorshipStatusResult{
		Found:          true,
		Status:         status,
		Message:        message,
		Order:          ord,
		Sponsor:        sp,
		LeaderboardURL: "/sponsor",
	}, nil
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
	if actor == "" {
		actor = "owner"
	}
	now := time.Now().UTC()
	act := &ActivityLog{
		SponsorID: id,
		Action:    ActivityActionSponsorApprove,
		Details:   fmt.Sprintf("Sponsor %s di-approve oleh %s (tampil di leaderboard)", sp.BrandName, actor),
		Actor:     actor,
		CreatedAt: now,
	}
	if err := s.store.ModerateSponsor(ctx, id, SponsorStatusApproved, "", act); err != nil {
		return err
	}

	// Trigger automatic sync of README.md and permanent archive directory
	if s.syncer != nil {
		batchSlug := DefaultBatchSlug
		if b, err := s.store.GetBatch(ctx, sp.BatchID); err == nil && b != nil && b.BatchSlug != "" {
			batchSlug = b.BatchSlug
		}
		_, _ = s.SyncReadmeAndArchive(ctx, batchSlug)
	}

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
	if actor == "" {
		actor = "owner"
	}
	now := time.Now().UTC()
	act := &ActivityLog{
		SponsorID: id,
		Action:    ActivityActionSponsorReject,
		Details:   fmt.Sprintf("Sponsor %s rejected by %s. Reason: %s", sp.BrandName, actor, reason),
		Actor:     actor,
		CreatedAt: now,
	}
	if err := s.store.ModerateSponsor(ctx, id, SponsorStatusRejected, reason, act); err != nil {
		return err
	}

	// Trigger automatic sync of README.md and permanent archive directory to remove rejected sponsor
	if s.syncer != nil {
		batchSlug := DefaultBatchSlug
		if b, err := s.store.GetBatch(ctx, sp.BatchID); err == nil && b != nil && b.BatchSlug != "" {
			batchSlug = b.BatchSlug
		}
		_, _ = s.SyncReadmeAndArchive(ctx, batchSlug)
	}

	return nil
}

func (s *Service) ListActivities(ctx context.Context, sponsorID string) ([]*ActivityLog, error) {
	return s.store.ListActivities(ctx, sponsorID)
}

// Orders & Dojo Webhook
func (s *Service) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
	dojoOrderID := strings.TrimSpace(req.DojoOrderID)
	if dojoOrderID == "" {
		return nil, ErrMissingOrderID
	}
	if req.AmountIDR <= 0 {
		return nil, errors.New("Order amount must be greater than 0")
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
			Details:   fmt.Sprintf("Order %s created for IDR %d (status: %s)", ord.DojoOrderID, ord.AmountIDR, ord.Status),
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
		return nil, errors.New("Webhook payload is empty")
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
				Message:          "Order has already been processed (idempotent)",
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

		// Transition sponsor to pending_review or auto-approve
		sponsorStatus := SponsorStatusPendingReview
		if existingOrder.SponsorID != nil && *existingOrder.SponsorID != "" {
			spID := *existingOrder.SponsorID
			if payload.AutoApprove {
				_ = s.ApproveSponsor(ctx, spID, "dojo_webhook_auto")
				sponsorStatus = SponsorStatusApproved
			} else {
				_ = s.store.UpdateSponsorStatus(ctx, spID, SponsorStatusPendingReview, "")
				_ = s.store.CreateActivity(ctx, &ActivityLog{
					SponsorID: spID,
					OrderID:   &existingOrder.ID,
					Action:    ActivityActionOrderPaid,
					Details:   fmt.Sprintf("Order %s paid IDR %d through the Dojo webhook", existingOrder.DojoOrderID, existingOrder.AmountIDR),
					Actor:     "dojo_webhook",
					CreatedAt: now,
				})
			}
		}

		return &WebhookResult{
			Processed:        true,
			AlreadyProcessed: false,
			OrderID:          existingOrder.ID,
			DojoOrderID:      existingOrder.DojoOrderID,
			AmountIDR:        existingOrder.AmountIDR,
			Status:           existingOrder.Status,
			SponsorStatus:    sponsorStatus,
			Message:          "Order status updated to paid",
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

	// Transition sponsor to pending_review or auto-approve
	sponsorStatus := SponsorStatusPendingReview
	if spID != nil && *spID != "" {
		if payload.AutoApprove {
			_ = s.ApproveSponsor(ctx, *spID, "dojo_webhook_auto")
			sponsorStatus = SponsorStatusApproved
		} else {
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
	}

	return &WebhookResult{
		Processed:        true,
		AlreadyProcessed: false,
		OrderID:          newOrder.ID,
		DojoOrderID:      newOrder.DojoOrderID,
		AmountIDR:        newOrder.AmountIDR,
		Status:           newOrder.Status,
		SponsorStatus:    sponsorStatus,
		Message:          "Order created and marked as paid",
	}, nil
}

// Waiting List
func (s *Service) SubmitWaitingListLead(ctx context.Context, email, source string) (*WaitingListLead, error) {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" || len(trimmedEmail) > 254 {
		return nil, ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(trimmedEmail)
	if err != nil || addr.Address != trimmedEmail {
		return nil, ErrInvalidEmail
	}
	parts := strings.Split(trimmedEmail, "@")
	if len(parts) != 2 || !strings.Contains(parts[1], ".") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".") {
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

	// Forward to Dojo webhook if configured
	if s.dojoDispatcher != nil {
		_ = s.dojoDispatcher.DispatchWaitingListSignup(ctx, lead)
	}

	return lead, nil
}

func (s *Service) ListWaitingListLeads(ctx context.Context) ([]*WaitingListLead, error) {
	return s.store.ListLeads(ctx)
}

// Logo saving helper
func (s *Service) SaveLogo(data []byte, filename string) (string, string, error) {
	if s.logoStorage == nil {
		return "", "", errors.New("Logo storage is not configured")
	}
	return s.logoStorage.SaveLogo(data, filename)
}

func generateID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
