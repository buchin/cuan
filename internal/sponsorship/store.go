package sponsorship

import (
	"context"
	"errors"
)

var (
	ErrBatchNotFound        = errors.New("Sponsorship batch not found")
	ErrSponsorNotFound      = errors.New("Sponsor not found")
	ErrOrderNotFound        = errors.New("Sponsorship order not found")
	ErrLeadNotFound         = errors.New("Waiting list lead not found")
	ErrDuplicateDojoOrderID = errors.New("dojo_order_id is already registered")
	ErrInvalidOrderInput    = errors.New("Invalid order input")
	ErrInvalidSponsorInput  = errors.New("Invalid sponsor input")
)

type Store interface {
	// Batches
	CreateBatch(ctx context.Context, b *Batch) error
	GetBatch(ctx context.Context, id string) (*Batch, error)
	GetBatchBySlug(ctx context.Context, slug string) (*Batch, error)
	ListBatches(ctx context.Context) ([]*Batch, error)
	UpdateBatch(ctx context.Context, b *Batch) error

	// Sponsors
	CreateSponsor(ctx context.Context, s *Sponsor) error
	GetSponsor(ctx context.Context, id string) (*Sponsor, error)
	ListSponsorsByBatch(ctx context.Context, batchID string, status string) ([]*Sponsor, error)
	ListApprovedSponsors(ctx context.Context, batchSlug string) ([]*Sponsor, error)
	UpdateSponsor(ctx context.Context, s *Sponsor) error
	UpdateSponsorStatus(ctx context.Context, id, status, rejectionReason string) error
	ModerateSponsor(ctx context.Context, id, status, rejectionReason string, activity *ActivityLog) error

	// Orders
	CreateOrder(ctx context.Context, o *Order) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	GetOrderByDojoID(ctx context.Context, dojoOrderID string) (*Order, error)
	UpdateOrder(ctx context.Context, o *Order) error
	ListOrdersBySponsor(ctx context.Context, sponsorID string) ([]*Order, error)

	// Waiting List Leads
	CreateLead(ctx context.Context, l *WaitingListLead) error
	ListLeads(ctx context.Context) ([]*WaitingListLead, error)

	// Activity Logs
	CreateActivity(ctx context.Context, a *ActivityLog) error
	ListActivities(ctx context.Context, sponsorID string) ([]*ActivityLog, error)
}
