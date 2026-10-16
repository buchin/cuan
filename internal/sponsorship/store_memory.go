package sponsorship

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu         sync.RWMutex
	batches    map[string]*Batch
	sponsors   map[string]*Sponsor
	orders     map[string]*Order
	leads      []*WaitingListLead
	activities []*ActivityLog
	actSeq     int64
}

func NewMemoryStore() *MemoryStore {
	m := &MemoryStore{
		batches:  make(map[string]*Batch),
		sponsors: make(map[string]*Sponsor),
		orders:   make(map[string]*Order),
	}
	// Seed default v1.0 batch
	m.batches["batch_v1_founding"] = &Batch{
		ID:        "batch_v1_founding",
		BatchSlug: "v1.0",
		Name:      "v1.0 Founding Backers",
		Status:    BatchStatusOpen,
		CreatedAt: time.Now().UTC(),
	}
	return m
}

// Batches
func (m *MemoryStore) CreateBatch(ctx context.Context, b *Batch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.batches {
		if existing.BatchSlug == b.BatchSlug {
			return errors.New("batch slug already exists")
		}
	}
	m.batches[b.ID] = b
	return nil
}

func (m *MemoryStore) GetBatch(ctx context.Context, id string) (*Batch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.batches[id]
	if !ok {
		return nil, ErrBatchNotFound
	}
	return b, nil
}

func (m *MemoryStore) GetBatchBySlug(ctx context.Context, slug string) (*Batch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, b := range m.batches {
		if b.BatchSlug == slug {
			return b, nil
		}
	}
	return nil, ErrBatchNotFound
}

func (m *MemoryStore) ListBatches(ctx context.Context) ([]*Batch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*Batch
	for _, b := range m.batches {
		list = append(list, b)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

func (m *MemoryStore) UpdateBatch(ctx context.Context, b *Batch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.batches[b.ID]; !ok {
		return ErrBatchNotFound
	}
	m.batches[b.ID] = b
	return nil
}

// Sponsors
func (m *MemoryStore) CreateSponsor(ctx context.Context, s *Sponsor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	s.UpdatedAt = s.CreatedAt
	m.sponsors[s.ID] = s
	return nil
}

func (m *MemoryStore) GetSponsor(ctx context.Context, id string) (*Sponsor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sponsors[id]
	if !ok {
		return nil, ErrSponsorNotFound
	}
	cp := *s
	var highestAmount int64
	var earliestPaidAt *time.Time
	for _, o := range m.orders {
		if o.SponsorID != nil && *o.SponsorID == s.ID {
			if o.Status == OrderStatusPaid && o.AmountIDR > highestAmount {
				highestAmount = o.AmountIDR
			} else if highestAmount == 0 && o.AmountIDR > highestAmount {
				highestAmount = o.AmountIDR
			}
			if o.PaidAt != nil && (earliestPaidAt == nil || o.PaidAt.Before(*earliestPaidAt)) {
				earliestPaidAt = o.PaidAt
			}
		}
	}
	cp.AmountIDR = highestAmount
	cp.PaidAt = earliestPaidAt
	return &cp, nil
}

func (m *MemoryStore) ListSponsorsByBatch(ctx context.Context, batchID string, status string) ([]*Sponsor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*Sponsor
	for _, s := range m.sponsors {
		if batchID != "" && s.BatchID != batchID {
			continue
		}
		if status != "" && s.Status != status {
			continue
		}
		cp := *s
		var highestAmount int64
		var earliestPaidAt *time.Time
		for _, o := range m.orders {
			if o.SponsorID != nil && *o.SponsorID == s.ID {
				if o.Status == OrderStatusPaid && o.AmountIDR > highestAmount {
					highestAmount = o.AmountIDR
				} else if highestAmount == 0 && o.AmountIDR > highestAmount {
					highestAmount = o.AmountIDR
				}
				if o.PaidAt != nil && (earliestPaidAt == nil || o.PaidAt.Before(*earliestPaidAt)) {
					earliestPaidAt = o.PaidAt
				}
			}
		}
		cp.AmountIDR = highestAmount
		cp.PaidAt = earliestPaidAt
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (m *MemoryStore) ListApprovedSponsors(ctx context.Context, batchSlug string) ([]*Sponsor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var targetBatchID string
	if batchSlug != "" {
		for _, b := range m.batches {
			if b.BatchSlug == batchSlug {
				targetBatchID = b.ID
				break
			}
		}
		if targetBatchID == "" {
			return nil, nil
		}
	}

	var list []*Sponsor
	for _, s := range m.sponsors {
		if s.Status != SponsorStatusApproved {
			continue
		}
		if targetBatchID != "" && s.BatchID != targetBatchID {
			continue
		}

		cp := *s
		// Find highest paid order amount and earliest paid_at
		var highestAmount int64
		var earliestPaidAt *time.Time
		for _, o := range m.orders {
			if o.SponsorID != nil && *o.SponsorID == s.ID && o.Status == OrderStatusPaid {
				if o.AmountIDR > highestAmount {
					highestAmount = o.AmountIDR
				}
				if o.PaidAt != nil {
					if earliestPaidAt == nil || o.PaidAt.Before(*earliestPaidAt) {
						earliestPaidAt = o.PaidAt
					}
				}
			}
		}
		cp.AmountIDR = highestAmount
		cp.PaidAt = earliestPaidAt
		list = append(list, &cp)
	}

	// ORDER BY amount DESC, paid_at ASC
	sort.Slice(list, func(i, j int) bool {
		if list[i].AmountIDR != list[j].AmountIDR {
			return list[i].AmountIDR > list[j].AmountIDR
		}
		if list[i].PaidAt != nil && list[j].PaidAt != nil {
			return list[i].PaidAt.Before(*list[j].PaidAt)
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	return list, nil
}

func (m *MemoryStore) UpdateSponsor(ctx context.Context, s *Sponsor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sponsors[s.ID]; !ok {
		return ErrSponsorNotFound
	}
	s.UpdatedAt = time.Now().UTC()
	m.sponsors[s.ID] = s
	return nil
}

func (m *MemoryStore) UpdateSponsorStatus(ctx context.Context, id, status, rejectionReason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sponsors[id]
	if !ok {
		return ErrSponsorNotFound
	}
	s.Status = status
	s.RejectionReason = rejectionReason
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// ModerateSponsor atomically transitions sponsor status and records the audit log.
func (m *MemoryStore) ModerateSponsor(ctx context.Context, id, status, rejectionReason string, activity *ActivityLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sponsors[id]
	if !ok {
		return ErrSponsorNotFound
	}
	now := time.Now().UTC()
	s.Status = status
	s.RejectionReason = rejectionReason
	s.UpdatedAt = now

	if activity != nil {
		if activity.CreatedAt.IsZero() {
			activity.CreatedAt = now
		}
		m.actSeq++
		activity.ID = m.actSeq
		activity.SponsorID = id
		m.activities = append(m.activities, activity)
	}
	return nil
}

// Orders
func (m *MemoryStore) CreateOrder(ctx context.Context, o *Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.orders {
		if existing.DojoOrderID == o.DojoOrderID {
			return ErrDuplicateDojoOrderID
		}
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now().UTC()
	}
	m.orders[o.ID] = o
	return nil
}

func (m *MemoryStore) GetOrder(ctx context.Context, id string) (*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[id]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return o, nil
}

func (m *MemoryStore) GetOrderByDojoID(ctx context.Context, dojoOrderID string) (*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, o := range m.orders {
		if o.DojoOrderID == dojoOrderID {
			return o, nil
		}
	}
	return nil, ErrOrderNotFound
}

func (m *MemoryStore) UpdateOrder(ctx context.Context, o *Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.orders[o.ID]; !ok {
		return ErrOrderNotFound
	}
	m.orders[o.ID] = o
	return nil
}

func (m *MemoryStore) ListOrdersBySponsor(ctx context.Context, sponsorID string) ([]*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*Order
	for _, o := range m.orders {
		if o.SponsorID != nil && *o.SponsorID == sponsorID {
			list = append(list, o)
		}
	}
	return list, nil
}

// Waiting List Leads
func (m *MemoryStore) CreateLead(ctx context.Context, l *WaitingListLead) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	m.leads = append(m.leads, l)
	return nil
}

func (m *MemoryStore) ListLeads(ctx context.Context) ([]*WaitingListLead, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*WaitingListLead
	for _, l := range m.leads {
		list = append(list, l)
	}
	return list, nil
}

// Activity Logs
func (m *MemoryStore) CreateActivity(ctx context.Context, a *ActivityLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actSeq++
	a.ID = m.actSeq
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	m.activities = append(m.activities, a)
	return nil
}

func (m *MemoryStore) ListActivities(ctx context.Context, sponsorID string) ([]*ActivityLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*ActivityLog
	for _, a := range m.activities {
		if sponsorID == "" || a.SponsorID == sponsorID {
			list = append(list, a)
		}
	}
	return list, nil
}
