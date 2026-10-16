package sponsorship

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type DBStore struct {
	db *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

// Batches
func (s *DBStore) CreateBatch(ctx context.Context, b *Batch) error {
	query := `
		INSERT INTO sponsorship_batches (id, batch_slug, name, status, created_at, closed_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, query, b.ID, b.BatchSlug, b.Name, b.Status, b.CreatedAt, b.ClosedAt)
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}
	return nil
}

func (s *DBStore) GetBatch(ctx context.Context, id string) (*Batch, error) {
	query := `
		SELECT id, batch_slug, name, status, created_at, closed_at
		FROM sponsorship_batches
		WHERE id = $1
	`
	var b Batch
	err := s.db.QueryRowContext(ctx, query, id).Scan(&b.ID, &b.BatchSlug, &b.Name, &b.Status, &b.CreatedAt, &b.ClosedAt)
	if err == sql.ErrNoRows {
		return nil, ErrBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get batch: %w", err)
	}
	return &b, nil
}

func (s *DBStore) GetBatchBySlug(ctx context.Context, slug string) (*Batch, error) {
	query := `
		SELECT id, batch_slug, name, status, created_at, closed_at
		FROM sponsorship_batches
		WHERE batch_slug = $1
	`
	var b Batch
	err := s.db.QueryRowContext(ctx, query, slug).Scan(&b.ID, &b.BatchSlug, &b.Name, &b.Status, &b.CreatedAt, &b.ClosedAt)
	if err == sql.ErrNoRows {
		return nil, ErrBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get batch by slug: %w", err)
	}
	return &b, nil
}

func (s *DBStore) ListBatches(ctx context.Context) ([]*Batch, error) {
	query := `
		SELECT id, batch_slug, name, status, created_at, closed_at
		FROM sponsorship_batches
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	defer rows.Close()

	var list []*Batch
	for rows.Next() {
		var b Batch
		if err := rows.Scan(&b.ID, &b.BatchSlug, &b.Name, &b.Status, &b.CreatedAt, &b.ClosedAt); err != nil {
			return nil, fmt.Errorf("scan batch: %w", err)
		}
		list = append(list, &b)
	}
	return list, nil
}

func (s *DBStore) UpdateBatch(ctx context.Context, b *Batch) error {
	query := `
		UPDATE sponsorship_batches
		SET batch_slug = $1, name = $2, status = $3, closed_at = $4
		WHERE id = $5
	`
	res, err := s.db.ExecContext(ctx, query, b.BatchSlug, b.Name, b.Status, b.ClosedAt, b.ID)
	if err != nil {
		return fmt.Errorf("update batch: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrBatchNotFound
	}
	return nil
}

// Sponsors
func (s *DBStore) CreateSponsor(ctx context.Context, sp *Sponsor) error {
	query := `
		INSERT INTO sponsors (id, batch_id, brand_name, website_url, slogan, logo_path, contact_email, contact_wa, status, rejection_reason, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	if sp.CreatedAt.IsZero() {
		sp.CreatedAt = time.Now().UTC()
	}
	sp.UpdatedAt = sp.CreatedAt
	_, err := s.db.ExecContext(ctx, query,
		sp.ID, sp.BatchID, sp.BrandName, sp.WebsiteURL, sp.Slogan, sp.LogoPath,
		sp.ContactEmail, sp.ContactWA, sp.Status, sp.RejectionReason, sp.CreatedAt, sp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create sponsor: %w", err)
	}
	return nil
}

func (s *DBStore) GetSponsor(ctx context.Context, id string) (*Sponsor, error) {
	query := `
		SELECT s.id, s.batch_id, s.brand_name, s.website_url, s.slogan, s.logo_path,
		       s.contact_email, s.contact_wa, s.status, COALESCE(s.rejection_reason, ''),
		       s.created_at, s.updated_at,
		       COALESCE(MAX(o.amount_idr), 0) as amount_idr,
		       MIN(o.paid_at) as paid_at
		FROM sponsors s
		LEFT JOIN sponsorship_orders o ON s.id = o.sponsor_id AND o.status = 'paid'
		WHERE s.id = $1
		GROUP BY s.id, s.batch_id, s.brand_name, s.website_url, s.slogan, s.logo_path,
		         s.contact_email, s.contact_wa, s.status, s.rejection_reason, s.created_at, s.updated_at
	`
	var sp Sponsor
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&sp.ID, &sp.BatchID, &sp.BrandName, &sp.WebsiteURL, &sp.Slogan, &sp.LogoPath,
		&sp.ContactEmail, &sp.ContactWA, &sp.Status, &sp.RejectionReason,
		&sp.CreatedAt, &sp.UpdatedAt, &sp.AmountIDR, &sp.PaidAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrSponsorNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sponsor: %w", err)
	}
	return &sp, nil
}

func (s *DBStore) ListSponsorsByBatch(ctx context.Context, batchID string, status string) ([]*Sponsor, error) {
	query := `
		SELECT s.id, s.batch_id, s.brand_name, s.website_url, s.slogan, s.logo_path,
		       s.contact_email, s.contact_wa, s.status, COALESCE(s.rejection_reason, ''),
		       s.created_at, s.updated_at,
		       COALESCE(MAX(o.amount_idr), 0) as amount_idr,
		       MIN(o.paid_at) as paid_at
		FROM sponsors s
		LEFT JOIN sponsorship_orders o ON s.id = o.sponsor_id AND o.status = 'paid'
		WHERE ($1 = '' OR s.batch_id = $1)
		  AND ($2 = '' OR s.status = $2)
		GROUP BY s.id, s.batch_id, s.brand_name, s.website_url, s.slogan, s.logo_path,
		         s.contact_email, s.contact_wa, s.status, s.rejection_reason, s.created_at, s.updated_at
		ORDER BY s.created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, batchID, status)
	if err != nil {
		return nil, fmt.Errorf("list sponsors: %w", err)
	}
	defer rows.Close()

	var list []*Sponsor
	for rows.Next() {
		var sp Sponsor
		if err := rows.Scan(
			&sp.ID, &sp.BatchID, &sp.BrandName, &sp.WebsiteURL, &sp.Slogan, &sp.LogoPath,
			&sp.ContactEmail, &sp.ContactWA, &sp.Status, &sp.RejectionReason,
			&sp.CreatedAt, &sp.UpdatedAt, &sp.AmountIDR, &sp.PaidAt,
		); err != nil {
			return nil, fmt.Errorf("scan sponsor: %w", err)
		}
		list = append(list, &sp)
	}
	return list, nil
}

// ListApprovedSponsors implements dynamic ranking: ORDER BY amount_idr DESC, paid_at ASC
func (s *DBStore) ListApprovedSponsors(ctx context.Context, batchSlug string) ([]*Sponsor, error) {
	query := `
		SELECT s.id, s.batch_id, s.brand_name, s.website_url, s.slogan, s.logo_path,
		       s.contact_email, s.contact_wa, s.status, COALESCE(s.rejection_reason, ''),
		       s.created_at, s.updated_at,
		       COALESCE(MAX(o.amount_idr), 0) as amount_idr,
		       MIN(o.paid_at) as paid_at
		FROM sponsors s
		JOIN sponsorship_batches b ON s.batch_id = b.id
		LEFT JOIN sponsorship_orders o ON s.id = o.sponsor_id AND o.status = 'paid'
		WHERE s.status = 'approved'
		  AND ($1 = '' OR b.batch_slug = $1)
		GROUP BY s.id, s.batch_id, s.brand_name, s.website_url, s.slogan, s.logo_path,
		         s.contact_email, s.contact_wa, s.status, s.rejection_reason, s.created_at, s.updated_at
		ORDER BY amount_idr DESC, paid_at ASC NULLS LAST, s.created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, batchSlug)
	if err != nil {
		return nil, fmt.Errorf("list approved sponsors: %w", err)
	}
	defer rows.Close()

	var list []*Sponsor
	for rows.Next() {
		var sp Sponsor
		if err := rows.Scan(
			&sp.ID, &sp.BatchID, &sp.BrandName, &sp.WebsiteURL, &sp.Slogan, &sp.LogoPath,
			&sp.ContactEmail, &sp.ContactWA, &sp.Status, &sp.RejectionReason,
			&sp.CreatedAt, &sp.UpdatedAt, &sp.AmountIDR, &sp.PaidAt,
		); err != nil {
			return nil, fmt.Errorf("scan approved sponsor: %w", err)
		}
		list = append(list, &sp)
	}
	return list, nil
}

func (s *DBStore) UpdateSponsor(ctx context.Context, sp *Sponsor) error {
	query := `
		UPDATE sponsors
		SET brand_name = $1, website_url = $2, slogan = $3, logo_path = $4,
		    contact_email = $5, contact_wa = $6, status = $7, rejection_reason = $8,
		    updated_at = $9
		WHERE id = $10
	`
	sp.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, query,
		sp.BrandName, sp.WebsiteURL, sp.Slogan, sp.LogoPath,
		sp.ContactEmail, sp.ContactWA, sp.Status, sp.RejectionReason,
		sp.UpdatedAt, sp.ID,
	)
	if err != nil {
		return fmt.Errorf("update sponsor: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSponsorNotFound
	}
	return nil
}

func (s *DBStore) UpdateSponsorStatus(ctx context.Context, id, status, rejectionReason string) error {
	query := `
		UPDATE sponsors
		SET status = $1, rejection_reason = $2, updated_at = $3
		WHERE id = $4
	`
	res, err := s.db.ExecContext(ctx, query, status, rejectionReason, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update sponsor status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrSponsorNotFound
	}
	return nil
}

// Orders
func (s *DBStore) CreateOrder(ctx context.Context, o *Order) error {
	query := `
		INSERT INTO sponsorship_orders (id, sponsor_id, dojo_order_id, amount_idr, status, paid_at, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now().UTC()
	}
	payloadBytes := o.Payload
	if len(payloadBytes) == 0 {
		payloadBytes = json.RawMessage("{}")
	}

	_, err := s.db.ExecContext(ctx, query,
		o.ID, o.SponsorID, o.DojoOrderID, o.AmountIDR, o.Status, o.PaidAt, string(payloadBytes), o.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

func (s *DBStore) GetOrder(ctx context.Context, id string) (*Order, error) {
	query := `
		SELECT id, sponsor_id, dojo_order_id, amount_idr, status, paid_at, payload, created_at
		FROM sponsorship_orders
		WHERE id = $1
	`
	var o Order
	var payloadStr sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&o.ID, &o.SponsorID, &o.DojoOrderID, &o.AmountIDR, &o.Status, &o.PaidAt, &payloadStr, &o.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	if payloadStr.Valid && payloadStr.String != "" {
		o.Payload = json.RawMessage(payloadStr.String)
	}
	return &o, nil
}

func (s *DBStore) GetOrderByDojoID(ctx context.Context, dojoOrderID string) (*Order, error) {
	query := `
		SELECT id, sponsor_id, dojo_order_id, amount_idr, status, paid_at, payload, created_at
		FROM sponsorship_orders
		WHERE dojo_order_id = $1
	`
	var o Order
	var payloadStr sql.NullString
	err := s.db.QueryRowContext(ctx, query, dojoOrderID).Scan(
		&o.ID, &o.SponsorID, &o.DojoOrderID, &o.AmountIDR, &o.Status, &o.PaidAt, &payloadStr, &o.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order by dojo_order_id: %w", err)
	}
	if payloadStr.Valid && payloadStr.String != "" {
		o.Payload = json.RawMessage(payloadStr.String)
	}
	return &o, nil
}

func (s *DBStore) UpdateOrder(ctx context.Context, o *Order) error {
	query := `
		UPDATE sponsorship_orders
		SET sponsor_id = $1, amount_idr = $2, status = $3, paid_at = $4, payload = $5
		WHERE id = $6
	`
	payloadBytes := o.Payload
	if len(payloadBytes) == 0 {
		payloadBytes = json.RawMessage("{}")
	}
	res, err := s.db.ExecContext(ctx, query, o.SponsorID, o.AmountIDR, o.Status, o.PaidAt, string(payloadBytes), o.ID)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrOrderNotFound
	}
	return nil
}

func (s *DBStore) ListOrdersBySponsor(ctx context.Context, sponsorID string) ([]*Order, error) {
	query := `
		SELECT id, sponsor_id, dojo_order_id, amount_idr, status, paid_at, payload, created_at
		FROM sponsorship_orders
		WHERE sponsor_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, sponsorID)
	if err != nil {
		return nil, fmt.Errorf("list orders by sponsor: %w", err)
	}
	defer rows.Close()

	var list []*Order
	for rows.Next() {
		var o Order
		var payloadStr sql.NullString
		if err := rows.Scan(
			&o.ID, &o.SponsorID, &o.DojoOrderID, &o.AmountIDR, &o.Status, &o.PaidAt, &payloadStr, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		if payloadStr.Valid && payloadStr.String != "" {
			o.Payload = json.RawMessage(payloadStr.String)
		}
		list = append(list, &o)
	}
	return list, nil
}

// Waiting List Leads
func (s *DBStore) CreateLead(ctx context.Context, l *WaitingListLead) error {
	query := `
		INSERT INTO waiting_list_leads (id, email, source, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, query, l.ID, l.Email, l.Source, l.Status, l.CreatedAt)
	if err != nil {
		return fmt.Errorf("create lead: %w", err)
	}
	return nil
}

func (s *DBStore) ListLeads(ctx context.Context) ([]*WaitingListLead, error) {
	query := `
		SELECT id, email, source, status, created_at
		FROM waiting_list_leads
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list leads: %w", err)
	}
	defer rows.Close()

	var list []*WaitingListLead
	for rows.Next() {
		var l WaitingListLead
		if err := rows.Scan(&l.ID, &l.Email, &l.Source, &l.Status, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan lead: %w", err)
		}
		list = append(list, &l)
	}
	return list, nil
}

// Activity Logs
func (s *DBStore) CreateActivity(ctx context.Context, a *ActivityLog) error {
	query := `
		INSERT INTO sponsorship_activities (sponsor_id, order_id, action, details, actor, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	err := s.db.QueryRowContext(ctx, query,
		a.SponsorID, a.OrderID, a.Action, a.Details, a.Actor, a.CreatedAt,
	).Scan(&a.ID)
	if err != nil {
		return fmt.Errorf("create activity: %w", err)
	}
	return nil
}

func (s *DBStore) ListActivities(ctx context.Context, sponsorID string) ([]*ActivityLog, error) {
	query := `
		SELECT id, sponsor_id, order_id, action, details, actor, created_at
		FROM sponsorship_activities
		WHERE ($1 = '' OR sponsor_id = $1)
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, sponsorID)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()

	var list []*ActivityLog
	for rows.Next() {
		var a ActivityLog
		if err := rows.Scan(&a.ID, &a.SponsorID, &a.OrderID, &a.Action, &a.Details, &a.Actor, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		list = append(list, &a)
	}
	return list, nil
}
