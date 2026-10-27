package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrBatchCampaignNotFound = errors.New("batch campaign not found")
	ErrEmptyCampaignTitle    = errors.New("campaign title cannot be empty")
	ErrInvalidCampaignStatus = errors.New("invalid campaign status: must be draft, pending, running, completed, failed, paused, or canceled")
	ErrDuplicateCampaign     = errors.New("batch campaign already exists")
	ErrInvalidJSONPayload    = errors.New("invalid JSON payload")
)

const (
	CampaignStatusDraft     = "draft"
	CampaignStatusPending   = "pending"
	CampaignStatusRunning   = "running"
	CampaignStatusCompleted = "completed"
	CampaignStatusFailed    = "failed"
	CampaignStatusPaused    = "paused"
	CampaignStatusCanceled  = "canceled"
)

// NormalizeCampaignStatus validates and returns lowercase campaign status.
func NormalizeCampaignStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return CampaignStatusDraft, nil
	}
	switch norm {
	case CampaignStatusDraft, CampaignStatusPending, CampaignStatusRunning,
		CampaignStatusCompleted, CampaignStatusFailed, CampaignStatusPaused, CampaignStatusCanceled:
		return norm, nil
	default:
		return "", ErrInvalidCampaignStatus
	}
}

// ValidateJSON ensures that the provided RawMessage contains valid JSON syntax.
func ValidateJSON(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var dummy interface{}
	if err := json.Unmarshal(raw, &dummy); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidJSONPayload, err)
	}
	return nil
}

// BatchCampaign represents a multi-project execution or rollout campaign entity.
type BatchCampaign struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	TemplatePayload json.RawMessage `json:"template_payload"`
	TargetCriteria  json.RawMessage `json:"target_criteria"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Clone creates a deep copy of BatchCampaign.
func (b *BatchCampaign) Clone() *BatchCampaign {
	if b == nil {
		return nil
	}
	var tPayload, tCriteria json.RawMessage
	if len(b.TemplatePayload) > 0 {
		tPayload = make(json.RawMessage, len(b.TemplatePayload))
		copy(tPayload, b.TemplatePayload)
	} else {
		tPayload = json.RawMessage("{}")
	}
	if len(b.TargetCriteria) > 0 {
		tCriteria = make(json.RawMessage, len(b.TargetCriteria))
		copy(tCriteria, b.TargetCriteria)
	} else {
		tCriteria = json.RawMessage("{}")
	}

	return &BatchCampaign{
		ID:              b.ID,
		Title:           b.Title,
		Description:     b.Description,
		TemplatePayload: tPayload,
		TargetCriteria:  tCriteria,
		Status:          b.Status,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
}

// BatchCampaignDTO is the public Data Transfer Object for BatchCampaign.
type BatchCampaignDTO struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	TemplatePayload json.RawMessage `json:"template_payload"`
	TargetCriteria  json.RawMessage `json:"target_criteria"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ToDTO converts BatchCampaign to its public DTO.
func (b *BatchCampaign) ToDTO() BatchCampaignDTO {
	var tPayload, tCriteria json.RawMessage
	if len(b.TemplatePayload) > 0 {
		tPayload = make(json.RawMessage, len(b.TemplatePayload))
		copy(tPayload, b.TemplatePayload)
	} else {
		tPayload = json.RawMessage("{}")
	}
	if len(b.TargetCriteria) > 0 {
		tCriteria = make(json.RawMessage, len(b.TargetCriteria))
		copy(tCriteria, b.TargetCriteria)
	} else {
		tCriteria = json.RawMessage("{}")
	}

	return BatchCampaignDTO{
		ID:              b.ID,
		Title:           b.Title,
		Description:     b.Description,
		TemplatePayload: tPayload,
		TargetCriteria:  tCriteria,
		Status:          b.Status,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
}

// CreateBatchCampaignInput defines payload parameters for creating a new batch campaign.
type CreateBatchCampaignInput struct {
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	TemplatePayload json.RawMessage `json:"template_payload,omitempty"`
	TargetCriteria  json.RawMessage `json:"target_criteria,omitempty"`
	Status          string          `json:"status,omitempty"`
}

// Validate verifies the constraints on CreateBatchCampaignInput.
func (in *CreateBatchCampaignInput) Validate() error {
	if strings.TrimSpace(in.Title) == "" {
		return ErrEmptyCampaignTitle
	}
	if in.Status != "" {
		if _, err := NormalizeCampaignStatus(in.Status); err != nil {
			return err
		}
	}
	if err := ValidateJSON(in.TemplatePayload); err != nil {
		return fmt.Errorf("template_payload: %w", err)
	}
	if err := ValidateJSON(in.TargetCriteria); err != nil {
		return fmt.Errorf("target_criteria: %w", err)
	}
	return nil
}

// ToEntity converts CreateBatchCampaignInput into a BatchCampaign entity.
func (in *CreateBatchCampaignInput) ToEntity() (*BatchCampaign, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	status := CampaignStatusDraft
	if in.Status != "" {
		norm, err := NormalizeCampaignStatus(in.Status)
		if err != nil {
			return nil, err
		}
		status = norm
	}

	templatePayload := in.TemplatePayload
	if len(templatePayload) == 0 {
		templatePayload = json.RawMessage("{}")
	}
	targetCriteria := in.TargetCriteria
	if len(targetCriteria) == 0 {
		targetCriteria = json.RawMessage("{}")
	}

	return &BatchCampaign{
		Title:           strings.TrimSpace(in.Title),
		Description:     strings.TrimSpace(in.Description),
		TemplatePayload: templatePayload,
		TargetCriteria:  targetCriteria,
		Status:          status,
	}, nil
}

// UpdateBatchCampaignInput defines fields that can be updated on an existing campaign.
type UpdateBatchCampaignInput struct {
	Title           *string          `json:"title,omitempty"`
	Description     *string          `json:"description,omitempty"`
	TemplatePayload *json.RawMessage `json:"template_payload,omitempty"`
	TargetCriteria  *json.RawMessage `json:"target_criteria,omitempty"`
	Status          *string          `json:"status,omitempty"`
}

// Validate verifies the constraints on UpdateBatchCampaignInput.
func (in *UpdateBatchCampaignInput) Validate() error {
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return ErrEmptyCampaignTitle
	}
	if in.Status != nil {
		if _, err := NormalizeCampaignStatus(*in.Status); err != nil {
			return err
		}
	}
	if in.TemplatePayload != nil {
		if err := ValidateJSON(*in.TemplatePayload); err != nil {
			return fmt.Errorf("template_payload: %w", err)
		}
	}
	if in.TargetCriteria != nil {
		if err := ValidateJSON(*in.TargetCriteria); err != nil {
			return fmt.Errorf("target_criteria: %w", err)
		}
	}
	return nil
}

// ApplyTo modifies the provided BatchCampaign with non-nil fields from the input.
func (in *UpdateBatchCampaignInput) ApplyTo(campaign *BatchCampaign) error {
	if err := in.Validate(); err != nil {
		return err
	}
	if in.Title != nil {
		campaign.Title = strings.TrimSpace(*in.Title)
	}
	if in.Description != nil {
		campaign.Description = strings.TrimSpace(*in.Description)
	}
	if in.TemplatePayload != nil {
		campaign.TemplatePayload = *in.TemplatePayload
		if len(campaign.TemplatePayload) == 0 {
			campaign.TemplatePayload = json.RawMessage("{}")
		}
	}
	if in.TargetCriteria != nil {
		campaign.TargetCriteria = *in.TargetCriteria
		if len(campaign.TargetCriteria) == 0 {
			campaign.TargetCriteria = json.RawMessage("{}")
		}
	}
	if in.Status != nil {
		norm, _ := NormalizeCampaignStatus(*in.Status)
		campaign.Status = norm
	}
	return nil
}

// BatchCampaignFilter specifies criteria for filtering and paginating campaigns.
type BatchCampaignFilter struct {
	Status string `json:"status,omitempty"`
	Search string `json:"search,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// BatchCampaignRepository defines persistent storage operations for batch campaigns.
type BatchCampaignRepository interface {
	Create(ctx context.Context, campaign *BatchCampaign) error
	GetByID(ctx context.Context, id string) (*BatchCampaign, error)
	List(ctx context.Context, filter BatchCampaignFilter) ([]BatchCampaign, error)
	Update(ctx context.Context, campaign *BatchCampaign) error
	UpdateStatus(ctx context.Context, id string, status string) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, filter BatchCampaignFilter) (int, error)
}

// SQLBatchCampaignRepository implements BatchCampaignRepository with PostgreSQL.
type SQLBatchCampaignRepository struct {
	db *sql.DB
}

// NewSQLBatchCampaignRepository creates a new SQLBatchCampaignRepository instance.
func NewSQLBatchCampaignRepository(db *sql.DB) *SQLBatchCampaignRepository {
	return &SQLBatchCampaignRepository{db: db}
}

// Create inserts a new BatchCampaign record into PostgreSQL.
func (r *SQLBatchCampaignRepository) Create(ctx context.Context, campaign *BatchCampaign) error {
	if campaign == nil {
		return errors.New("campaign cannot be nil")
	}

	if strings.TrimSpace(campaign.Title) == "" {
		return ErrEmptyCampaignTitle
	}

	normStatus, err := NormalizeCampaignStatus(campaign.Status)
	if err != nil {
		return err
	}
	campaign.Status = normStatus

	if strings.TrimSpace(campaign.ID) == "" {
		campaign.ID = GenerateUUID()
	} else if !IsValidUUID(campaign.ID) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, campaign.ID)
	}

	if len(campaign.TemplatePayload) == 0 {
		campaign.TemplatePayload = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TemplatePayload); err != nil {
		return fmt.Errorf("template_payload: %w", err)
	}

	if len(campaign.TargetCriteria) == 0 {
		campaign.TargetCriteria = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TargetCriteria); err != nil {
		return fmt.Errorf("target_criteria: %w", err)
	}

	now := time.Now().UTC()
	if campaign.CreatedAt.IsZero() {
		campaign.CreatedAt = now
	}
	if campaign.UpdatedAt.IsZero() {
		campaign.UpdatedAt = now
	}

	query := `
		INSERT INTO batch_campaigns (
			id, title, description, template_payload, target_criteria, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = r.db.ExecContext(
		ctx,
		query,
		campaign.ID,
		campaign.Title,
		campaign.Description,
		[]byte(campaign.TemplatePayload),
		[]byte(campaign.TargetCriteria),
		campaign.Status,
		campaign.CreatedAt,
		campaign.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrDuplicateCampaign
		}
		return fmt.Errorf("create batch campaign: %w", err)
	}
	return nil
}

// GetByID retrieves a BatchCampaign by its UUID.
func (r *SQLBatchCampaignRepository) GetByID(ctx context.Context, id string) (*BatchCampaign, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, id)
	}

	query := `
		SELECT id, title, description, template_payload, target_criteria, status, created_at, updated_at
		FROM batch_campaigns
		WHERE id = $1
	`
	var campaign BatchCampaign
	var templateBytes, targetBytes []byte

	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&campaign.ID,
		&campaign.Title,
		&campaign.Description,
		&templateBytes,
		&targetBytes,
		&campaign.Status,
		&campaign.CreatedAt,
		&campaign.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBatchCampaignNotFound
		}
		return nil, fmt.Errorf("get batch campaign by id: %w", err)
	}

	if len(templateBytes) > 0 {
		campaign.TemplatePayload = json.RawMessage(templateBytes)
	} else {
		campaign.TemplatePayload = json.RawMessage("{}")
	}

	if len(targetBytes) > 0 {
		campaign.TargetCriteria = json.RawMessage(targetBytes)
	} else {
		campaign.TargetCriteria = json.RawMessage("{}")
	}

	return &campaign, nil
}

// List retrieves batch campaigns matching the filter.
func (r *SQLBatchCampaignRepository) List(ctx context.Context, filter BatchCampaignFilter) ([]BatchCampaign, error) {
	query := `
		SELECT id, title, description, template_payload, target_criteria, status, created_at, updated_at
		FROM batch_campaigns
	`
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if strings.TrimSpace(filter.Status) != "" {
		normStatus, err := NormalizeCampaignStatus(filter.Status)
		if err == nil {
			whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
			args = append(args, normStatus)
			argIdx++
		}
	}

	if strings.TrimSpace(filter.Search) != "" {
		searchPattern := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(LOWER(title) LIKE $%d OR LOWER(description) LIKE $%d)", argIdx, argIdx))
		args = append(args, searchPattern)
		argIdx++
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
		argIdx++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list batch campaigns: %w", err)
	}
	defer rows.Close()

	list := make([]BatchCampaign, 0)
	for rows.Next() {
		var campaign BatchCampaign
		var templateBytes, targetBytes []byte
		if err := rows.Scan(
			&campaign.ID,
			&campaign.Title,
			&campaign.Description,
			&templateBytes,
			&targetBytes,
			&campaign.Status,
			&campaign.CreatedAt,
			&campaign.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan batch campaign row: %w", err)
		}

		if len(templateBytes) > 0 {
			campaign.TemplatePayload = json.RawMessage(templateBytes)
		} else {
			campaign.TemplatePayload = json.RawMessage("{}")
		}

		if len(targetBytes) > 0 {
			campaign.TargetCriteria = json.RawMessage(targetBytes)
		} else {
			campaign.TargetCriteria = json.RawMessage("{}")
		}

		list = append(list, campaign)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return list, nil
}

// Count returns total number of campaigns matching filter criteria.
func (r *SQLBatchCampaignRepository) Count(ctx context.Context, filter BatchCampaignFilter) (int, error) {
	query := "SELECT COUNT(*) FROM batch_campaigns"
	var whereClauses []string
	var args []interface{}
	argIdx := 1

	if strings.TrimSpace(filter.Status) != "" {
		normStatus, err := NormalizeCampaignStatus(filter.Status)
		if err == nil {
			whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIdx))
			args = append(args, normStatus)
			argIdx++
		}
	}

	if strings.TrimSpace(filter.Search) != "" {
		searchPattern := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf("(LOWER(title) LIKE $%d OR LOWER(description) LIKE $%d)", argIdx, argIdx))
		args = append(args, searchPattern)
		argIdx++
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count batch campaigns: %w", err)
	}
	return count, nil
}

// Update updates an existing batch campaign record.
func (r *SQLBatchCampaignRepository) Update(ctx context.Context, campaign *BatchCampaign) error {
	if campaign == nil {
		return errors.New("campaign cannot be nil")
	}

	if !IsValidUUID(campaign.ID) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, campaign.ID)
	}

	if strings.TrimSpace(campaign.Title) == "" {
		return ErrEmptyCampaignTitle
	}

	normStatus, err := NormalizeCampaignStatus(campaign.Status)
	if err != nil {
		return err
	}
	campaign.Status = normStatus

	if len(campaign.TemplatePayload) == 0 {
		campaign.TemplatePayload = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TemplatePayload); err != nil {
		return fmt.Errorf("template_payload: %w", err)
	}

	if len(campaign.TargetCriteria) == 0 {
		campaign.TargetCriteria = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TargetCriteria); err != nil {
		return fmt.Errorf("target_criteria: %w", err)
	}

	campaign.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE batch_campaigns
		SET title = $2, description = $3, template_payload = $4, target_criteria = $5, status = $6, updated_at = $7
		WHERE id = $1
	`
	res, err := r.db.ExecContext(
		ctx,
		query,
		campaign.ID,
		campaign.Title,
		campaign.Description,
		[]byte(campaign.TemplatePayload),
		[]byte(campaign.TargetCriteria),
		campaign.Status,
		campaign.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update batch campaign: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrBatchCampaignNotFound
	}
	return nil
}

// UpdateStatus updates the status and updated_at of a campaign.
func (r *SQLBatchCampaignRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, id)
	}

	normStatus, err := NormalizeCampaignStatus(status)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	query := `
		UPDATE batch_campaigns
		SET status = $2, updated_at = $3
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id, normStatus, now)
	if err != nil {
		return fmt.Errorf("update campaign status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrBatchCampaignNotFound
	}
	return nil
}

// Delete removes a batch campaign by its UUID.
func (r *SQLBatchCampaignRepository) Delete(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, id)
	}

	query := `DELETE FROM batch_campaigns WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete batch campaign: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrBatchCampaignNotFound
	}
	return nil
}

// MemoryBatchCampaignRepository provides a thread-safe, in-memory implementation of BatchCampaignRepository.
type MemoryBatchCampaignRepository struct {
	mu        sync.RWMutex
	campaigns map[string]*BatchCampaign
}

// NewMemoryBatchCampaignRepository creates a new thread-safe in-memory campaign repository.
func NewMemoryBatchCampaignRepository() *MemoryBatchCampaignRepository {
	return &MemoryBatchCampaignRepository{
		campaigns: make(map[string]*BatchCampaign),
	}
}

// Create inserts a new BatchCampaign into memory.
func (m *MemoryBatchCampaignRepository) Create(ctx context.Context, campaign *BatchCampaign) error {
	if campaign == nil {
		return errors.New("campaign cannot be nil")
	}

	if strings.TrimSpace(campaign.Title) == "" {
		return ErrEmptyCampaignTitle
	}

	normStatus, err := NormalizeCampaignStatus(campaign.Status)
	if err != nil {
		return err
	}
	campaign.Status = normStatus

	if strings.TrimSpace(campaign.ID) == "" {
		campaign.ID = GenerateUUID()
	} else if !IsValidUUID(campaign.ID) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, campaign.ID)
	}

	if len(campaign.TemplatePayload) == 0 {
		campaign.TemplatePayload = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TemplatePayload); err != nil {
		return fmt.Errorf("template_payload: %w", err)
	}

	if len(campaign.TargetCriteria) == 0 {
		campaign.TargetCriteria = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TargetCriteria); err != nil {
		return fmt.Errorf("target_criteria: %w", err)
	}

	now := time.Now().UTC()
	if campaign.CreatedAt.IsZero() {
		campaign.CreatedAt = now
	}
	if campaign.UpdatedAt.IsZero() {
		campaign.UpdatedAt = now
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.campaigns[campaign.ID]; exists {
		return ErrDuplicateCampaign
	}

	m.campaigns[campaign.ID] = campaign.Clone()
	return nil
}

// GetByID retrieves a BatchCampaign by its UUID from memory.
func (m *MemoryBatchCampaignRepository) GetByID(ctx context.Context, id string) (*BatchCampaign, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, id)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.campaigns[id]
	if !exists {
		return nil, ErrBatchCampaignNotFound
	}

	return item.Clone(), nil
}

// List returns filtered and paginated batch campaigns from memory.
func (m *MemoryBatchCampaignRepository) List(ctx context.Context, filter BatchCampaignFilter) ([]BatchCampaign, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []*BatchCampaign
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))
	statusFilter := ""
	if strings.TrimSpace(filter.Status) != "" {
		if norm, err := NormalizeCampaignStatus(filter.Status); err == nil {
			statusFilter = norm
		}
	}

	for _, c := range m.campaigns {
		if statusFilter != "" && c.Status != statusFilter {
			continue
		}
		if searchLower != "" {
			titleMatch := strings.Contains(strings.ToLower(c.Title), searchLower)
			descMatch := strings.Contains(strings.ToLower(c.Description), searchLower)
			if !titleMatch && !descMatch {
				continue
			}
		}
		matched = append(matched, c.Clone())
	}

	// Sort created_at descending, tie break by ID
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID < matched[j].ID
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := len(matched)
	start := filter.Offset
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	end := total
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}

	result := make([]BatchCampaign, 0, end-start)
	for _, item := range matched[start:end] {
		result = append(result, *item)
	}

	return result, nil
}

// Count returns the number of campaigns matching the filter in memory.
func (m *MemoryBatchCampaignRepository) Count(ctx context.Context, filter BatchCampaignFilter) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))
	statusFilter := ""
	if strings.TrimSpace(filter.Status) != "" {
		if norm, err := NormalizeCampaignStatus(filter.Status); err == nil {
			statusFilter = norm
		}
	}

	for _, c := range m.campaigns {
		if statusFilter != "" && c.Status != statusFilter {
			continue
		}
		if searchLower != "" {
			titleMatch := strings.Contains(strings.ToLower(c.Title), searchLower)
			descMatch := strings.Contains(strings.ToLower(c.Description), searchLower)
			if !titleMatch && !descMatch {
				continue
			}
		}
		count++
	}

	return count, nil
}

// Update modifies an existing campaign in memory.
func (m *MemoryBatchCampaignRepository) Update(ctx context.Context, campaign *BatchCampaign) error {
	if campaign == nil {
		return errors.New("campaign cannot be nil")
	}

	if !IsValidUUID(campaign.ID) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, campaign.ID)
	}

	if strings.TrimSpace(campaign.Title) == "" {
		return ErrEmptyCampaignTitle
	}

	normStatus, err := NormalizeCampaignStatus(campaign.Status)
	if err != nil {
		return err
	}
	campaign.Status = normStatus

	if len(campaign.TemplatePayload) == 0 {
		campaign.TemplatePayload = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TemplatePayload); err != nil {
		return fmt.Errorf("template_payload: %w", err)
	}

	if len(campaign.TargetCriteria) == 0 {
		campaign.TargetCriteria = json.RawMessage("{}")
	} else if err := ValidateJSON(campaign.TargetCriteria); err != nil {
		return fmt.Errorf("target_criteria: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.campaigns[campaign.ID]
	if !exists {
		return ErrBatchCampaignNotFound
	}

	campaign.CreatedAt = existing.CreatedAt
	campaign.UpdatedAt = time.Now().UTC()
	m.campaigns[campaign.ID] = campaign.Clone()
	return nil
}

// UpdateStatus changes the status and updated_at of an in-memory campaign.
func (m *MemoryBatchCampaignRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, id)
	}

	normStatus, err := NormalizeCampaignStatus(status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.campaigns[id]
	if !exists {
		return ErrBatchCampaignNotFound
	}

	item.Status = normStatus
	item.UpdatedAt = time.Now().UTC()
	return nil
}

// Delete removes an in-memory campaign by its UUID.
func (m *MemoryBatchCampaignRepository) Delete(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: campaign id %s", ErrInvalidUUID, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.campaigns[id]; !exists {
		return ErrBatchCampaignNotFound
	}

	delete(m.campaigns, id)
	return nil
}
