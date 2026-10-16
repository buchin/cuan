package sponsorship

import (
	"encoding/json"
	"time"
)

// Batch status constants
const (
	BatchStatusOpen   = "open"
	BatchStatusClosed = "closed"
)

// Sponsor status constants
const (
	SponsorStatusPendingReview = "pending_review"
	SponsorStatusApproved      = "approved"
	SponsorStatusRejected      = "rejected"
)

// Order status constants
const (
	OrderStatusPending = "pending"
	OrderStatusPaid    = "paid"
	OrderStatusFailed  = "failed"
)

// Waiting list status constants
const (
	LeadStatusPending   = "pending"
	LeadStatusVerified  = "verified"
	LeadStatusConverted = "converted"
)

// Activity action constants
const (
	ActivityActionOrderCreated   = "order_created"
	ActivityActionOrderPaid      = "order_paid"
	ActivityActionStatusChange   = "status_change"
	ActivityActionSponsorCreated = "sponsor_created"
	ActivityActionSponsorApprove = "sponsor_approved"
	ActivityActionSponsorReject  = "sponsor_rejected"
)

// Batch represents a release sponsorship phase (e.g. v1.0 Founding Backers).
type Batch struct {
	ID        string     `json:"id"`
	BatchSlug string     `json:"batch_slug"`
	Name      string     `json:"name"`
	Status    string     `json:"status"` // open, closed
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

// Sponsor represents a sponsor brand profile.
type Sponsor struct {
	ID              string    `json:"id"`
	BatchID         string    `json:"batch_id"`
	BrandName       string    `json:"brand_name"`
	WebsiteURL      string    `json:"website_url"`
	Slogan          string    `json:"slogan"`
	LogoPath        string    `json:"logo_path"`
	ContactEmail    string    `json:"contact_email"`
	ContactWA       string    `json:"contact_wa"`
	Status          string    `json:"status"` // pending_review, approved, rejected
	RejectionReason string    `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Joined attributes for leaderboard & ranking
	AmountIDR int64      `json:"amount_idr,omitempty"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}

// Order represents a financial sponsorship transaction mediated by Dojo.
type Order struct {
	ID          string          `json:"id"`
	SponsorID   *string         `json:"sponsor_id,omitempty"`
	DojoOrderID string          `json:"dojo_order_id"`
	AmountIDR   int64           `json:"amount_idr"`
	Status      string          `json:"status"` // pending, paid, failed
	PaidAt      *time.Time      `json:"paid_at,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// WaitingListLead represents a prospective user signing up on the landing page.
type WaitingListLead struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Source    string    `json:"source"`
	Status    string    `json:"status"` // pending, verified, converted
	CreatedAt time.Time `json:"created_at"`
}

// ActivityLog represents an audit trail event for sponsorship modifications.
type ActivityLog struct {
	ID        int64     `json:"id"`
	SponsorID string    `json:"sponsor_id"`
	OrderID   *string   `json:"order_id,omitempty"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

// DojoWebhookPayload represents the JSON data received from Dojo checkout webhook.
type DojoWebhookPayload struct {
	Event       string          `json:"event,omitempty"`
	OrderID     string          `json:"order_id"`
	DojoOrderID string          `json:"dojo_order_id,omitempty"`
	Amount      int64           `json:"amount"`
	AmountIDR   int64           `json:"amount_idr,omitempty"`
	Status      string          `json:"status"` // paid, completed, success
	PaidAt      *time.Time      `json:"paid_at,omitempty"`
	SponsorID   string          `json:"sponsor_id,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// EffectiveOrderID returns the order ID from either OrderID or DojoOrderID.
func (p *DojoWebhookPayload) EffectiveOrderID() string {
	if p.DojoOrderID != "" {
		return p.DojoOrderID
	}
	return p.OrderID
}

// EffectiveAmountIDR returns the amount from either AmountIDR or Amount.
func (p *DojoWebhookPayload) EffectiveAmountIDR() int64 {
	if p.AmountIDR > 0 {
		return p.AmountIDR
	}
	return p.Amount
}
