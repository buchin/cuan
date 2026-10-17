package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrDomainNotFound    = errors.New("custom domain not found")
	ErrDuplicateDomain   = errors.New("custom domain already exists")
	ErrInvalidDomainName = errors.New("invalid domain name format")
	ErrEmptyDeploymentID = errors.New("deployment id cannot be empty")
)

var (
	// domainLabelRegex validates standard DNS hostname labels (RFC 1123 / RFC 952).
	domainLabelRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
)

// NormalizeDomain trims whitespace and converts the domain to lowercase.
func NormalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

// ValidateDomain checks whether a domain string conforms to valid DNS hostname standards.
func ValidateDomain(domain string) error {
	d := NormalizeDomain(domain)
	if d == "" {
		return fmt.Errorf("%w: domain cannot be empty", ErrInvalidDomainName)
	}

	if len(d) > 253 {
		return fmt.Errorf("%w: domain length exceeds 253 characters", ErrInvalidDomainName)
	}

	if strings.HasPrefix(d, "http://") || strings.HasPrefix(d, "https://") {
		return fmt.Errorf("%w: domain must not include http/https protocol", ErrInvalidDomainName)
	}

	if strings.Contains(d, "/") || strings.Contains(d, ":") || strings.Contains(d, "@") || strings.Contains(d, " ") {
		return fmt.Errorf("%w: domain contains invalid characters (slash, colon, at, or whitespace)", ErrInvalidDomainName)
	}

	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return fmt.Errorf("%w: domain must contain at least a name and a top-level domain", ErrInvalidDomainName)
	}

	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Errorf("%w: label '%s' must be between 1 and 63 characters", ErrInvalidDomainName, label)
		}
		if !domainLabelRegex.MatchString(label) {
			return fmt.Errorf("%w: label '%s' contains invalid characters or leading/trailing hyphens", ErrInvalidDomainName, label)
		}
	}

	// TLD should not be purely numeric
	tld := labels[len(labels)-1]
	allDigits := true
	for _, r := range tld {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		return fmt.Errorf("%w: top-level domain cannot be purely numeric", ErrInvalidDomainName)
	}

	return nil
}

// generateVerificationToken produces a secure 32-character hexadecimal verification token.
func generateVerificationToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("tok_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// CustomDomain represents a custom domain mapping linked to a deployment.
type CustomDomain struct {
	ID                string     `json:"id"`
	DeploymentID      string     `json:"deployment_id"`
	Domain            string     `json:"domain"`
	DNSStatus         string     `json:"dns_status"` // 'pending', 'verified', 'failed'
	SSLStatus         string     `json:"ssl_status"` // 'pending', 'active', 'failed'
	VerificationToken string     `json:"verification_token"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// CustomDomainDTO is a public Data Transfer Object for custom domain API responses.
type CustomDomainDTO struct {
	ID                string     `json:"id"`
	DeploymentID      string     `json:"deployment_id"`
	Domain            string     `json:"domain"`
	DNSStatus         string     `json:"dns_status"`
	SSLStatus         string     `json:"ssl_status"`
	VerificationToken string     `json:"verification_token,omitempty"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// ToDTO converts a CustomDomain entity to a CustomDomainDTO.
func (cd *CustomDomain) ToDTO() CustomDomainDTO {
	return CustomDomainDTO{
		ID:                cd.ID,
		DeploymentID:      cd.DeploymentID,
		Domain:            cd.Domain,
		DNSStatus:         cd.DNSStatus,
		SSLStatus:         cd.SSLStatus,
		VerificationToken: cd.VerificationToken,
		LastCheckedAt:     cd.LastCheckedAt,
		CreatedAt:         cd.CreatedAt,
		UpdatedAt:         cd.UpdatedAt,
	}
}

// ToSanitizedDTO converts a CustomDomain entity to a sanitized DTO masking sensitive parts of the verification token.
func (cd *CustomDomain) ToSanitizedDTO() CustomDomainDTO {
	token := cd.VerificationToken
	if len(token) > 8 {
		token = token[:4] + "..." + token[len(token)-4:]
	}
	return CustomDomainDTO{
		ID:                cd.ID,
		DeploymentID:      cd.DeploymentID,
		Domain:            cd.Domain,
		DNSStatus:         cd.DNSStatus,
		SSLStatus:         cd.SSLStatus,
		VerificationToken: token,
		LastCheckedAt:     cd.LastCheckedAt,
		CreatedAt:         cd.CreatedAt,
		UpdatedAt:         cd.UpdatedAt,
	}
}

// DomainRepository defines the data access contract for custom domain management.
type DomainRepository interface {
	CreateCustomDomain(ctx context.Context, domain *CustomDomain) error
	GetCustomDomainByID(ctx context.Context, id string) (*CustomDomain, error)
	GetCustomDomainByDomain(ctx context.Context, domainName string) (*CustomDomain, error)
	ListCustomDomainsByDeploymentID(ctx context.Context, deploymentID string) ([]CustomDomain, error)
	ListAllCustomDomains(ctx context.Context) ([]CustomDomain, error)
	UpdateCustomDomainStatus(ctx context.Context, id string, dnsStatus, sslStatus string, lastCheckedAt *time.Time) error
	DeleteCustomDomain(ctx context.Context, id string) error
}

// SQLDomainRepository implements DomainRepository backed by PostgreSQL.
type SQLDomainRepository struct {
	db *sql.DB
}

// NewSQLDomainRepository creates a new SQLDomainRepository instance.
func NewSQLDomainRepository(db *sql.DB) *SQLDomainRepository {
	return &SQLDomainRepository{db: db}
}

func (r *SQLDomainRepository) CreateCustomDomain(ctx context.Context, domain *CustomDomain) error {
	if domain == nil {
		return errors.New("custom domain cannot be nil")
	}

	if strings.TrimSpace(domain.DeploymentID) == "" {
		return ErrEmptyDeploymentID
	}

	if err := ValidateDomain(domain.Domain); err != nil {
		return err
	}
	domain.Domain = NormalizeDomain(domain.Domain)

	if strings.TrimSpace(domain.ID) == "" {
		domain.ID = fmt.Sprintf("dom_%s", generateVerificationToken()[:16])
	}

	if strings.TrimSpace(domain.DNSStatus) == "" {
		domain.DNSStatus = "pending"
	}
	if strings.TrimSpace(domain.SSLStatus) == "" {
		domain.SSLStatus = "pending"
	}
	if strings.TrimSpace(domain.VerificationToken) == "" {
		domain.VerificationToken = generateVerificationToken()
	}

	now := time.Now().UTC()
	if domain.CreatedAt.IsZero() {
		domain.CreatedAt = now
	}
	domain.UpdatedAt = now

	query := `
		INSERT INTO custom_domains (
			id, deployment_id, domain, dns_status, ssl_status,
			verification_token, last_checked_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		domain.ID,
		domain.DeploymentID,
		domain.Domain,
		domain.DNSStatus,
		domain.SSLStatus,
		domain.VerificationToken,
		domain.LastCheckedAt,
		domain.CreatedAt,
		domain.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(err.Error(), "23505") {
			return ErrDuplicateDomain
		}
		return fmt.Errorf("create custom domain: %w", err)
	}

	return nil
}

func (r *SQLDomainRepository) GetCustomDomainByID(ctx context.Context, id string) (*CustomDomain, error) {
	query := `
		SELECT id, deployment_id, domain, dns_status, ssl_status,
		       COALESCE(verification_token, ''), last_checked_at, created_at, updated_at
		FROM custom_domains
		WHERE id = $1
	`
	var cd CustomDomain
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&cd.ID,
		&cd.DeploymentID,
		&cd.Domain,
		&cd.DNSStatus,
		&cd.SSLStatus,
		&cd.VerificationToken,
		&cd.LastCheckedAt,
		&cd.CreatedAt,
		&cd.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDomainNotFound
		}
		return nil, fmt.Errorf("get custom domain by id: %w", err)
	}
	return &cd, nil
}

func (r *SQLDomainRepository) GetCustomDomainByDomain(ctx context.Context, domainName string) (*CustomDomain, error) {
	normalized := NormalizeDomain(domainName)
	query := `
		SELECT id, deployment_id, domain, dns_status, ssl_status,
		       COALESCE(verification_token, ''), last_checked_at, created_at, updated_at
		FROM custom_domains
		WHERE domain = $1
	`
	var cd CustomDomain
	row := r.db.QueryRowContext(ctx, query, normalized)
	err := row.Scan(
		&cd.ID,
		&cd.DeploymentID,
		&cd.Domain,
		&cd.DNSStatus,
		&cd.SSLStatus,
		&cd.VerificationToken,
		&cd.LastCheckedAt,
		&cd.CreatedAt,
		&cd.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDomainNotFound
		}
		return nil, fmt.Errorf("get custom domain by domain: %w", err)
	}
	return &cd, nil
}

func (r *SQLDomainRepository) ListCustomDomainsByDeploymentID(ctx context.Context, deploymentID string) ([]CustomDomain, error) {
	query := `
		SELECT id, deployment_id, domain, dns_status, ssl_status,
		       COALESCE(verification_token, ''), last_checked_at, created_at, updated_at
		FROM custom_domains
		WHERE deployment_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("list custom domains by deployment id: %w", err)
	}
	defer rows.Close()

	var list []CustomDomain
	for rows.Next() {
		var cd CustomDomain
		if err := rows.Scan(
			&cd.ID,
			&cd.DeploymentID,
			&cd.Domain,
			&cd.DNSStatus,
			&cd.SSLStatus,
			&cd.VerificationToken,
			&cd.LastCheckedAt,
			&cd.CreatedAt,
			&cd.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan custom domain row: %w", err)
		}
		list = append(list, cd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate custom domains: %w", err)
	}
	return list, nil
}

func (r *SQLDomainRepository) ListAllCustomDomains(ctx context.Context) ([]CustomDomain, error) {
	query := `
		SELECT id, deployment_id, domain, dns_status, ssl_status,
		       COALESCE(verification_token, ''), last_checked_at, created_at, updated_at
		FROM custom_domains
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all custom domains: %w", err)
	}
	defer rows.Close()

	var list []CustomDomain
	for rows.Next() {
		var cd CustomDomain
		if err := rows.Scan(
			&cd.ID,
			&cd.DeploymentID,
			&cd.Domain,
			&cd.DNSStatus,
			&cd.SSLStatus,
			&cd.VerificationToken,
			&cd.LastCheckedAt,
			&cd.CreatedAt,
			&cd.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan custom domain row: %w", err)
		}
		list = append(list, cd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all custom domains: %w", err)
	}
	return list, nil
}

func (r *SQLDomainRepository) UpdateCustomDomainStatus(ctx context.Context, id string, dnsStatus, sslStatus string, lastCheckedAt *time.Time) error {
	now := time.Now().UTC()
	query := `
		UPDATE custom_domains
		SET dns_status = $2, ssl_status = $3, last_checked_at = $4, updated_at = $5
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id, dnsStatus, sslStatus, lastCheckedAt, now)
	if err != nil {
		return fmt.Errorf("update custom domain status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update custom domain status rows: %w", err)
	}
	if rows == 0 {
		return ErrDomainNotFound
	}
	return nil
}

func (r *SQLDomainRepository) DeleteCustomDomain(ctx context.Context, id string) error {
	query := `DELETE FROM custom_domains WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete custom domain: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete custom domain rows: %w", err)
	}
	if rows == 0 {
		return ErrDomainNotFound
	}
	return nil
}

// MemoryDomainRepository provides a thread-safe in-memory implementation of DomainRepository
// for unit tests, local development, and mock suites.
type MemoryDomainRepository struct {
	mu      sync.RWMutex
	domains map[string]*CustomDomain
}

// NewMemoryDomainRepository instantiates an in-memory DomainRepository.
func NewMemoryDomainRepository() *MemoryDomainRepository {
	return &MemoryDomainRepository{
		domains: make(map[string]*CustomDomain),
	}
}

func (m *MemoryDomainRepository) CreateCustomDomain(ctx context.Context, domain *CustomDomain) error {
	if domain == nil {
		return errors.New("custom domain cannot be nil")
	}
	if strings.TrimSpace(domain.DeploymentID) == "" {
		return ErrEmptyDeploymentID
	}
	if err := ValidateDomain(domain.Domain); err != nil {
		return err
	}
	domain.Domain = NormalizeDomain(domain.Domain)

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(domain.ID) == "" {
		domain.ID = fmt.Sprintf("dom_%s", generateVerificationToken()[:16])
	}

	if _, exists := m.domains[domain.ID]; exists {
		return ErrDuplicateDomain
	}

	// Check domain uniqueness
	for _, existing := range m.domains {
		if NormalizeDomain(existing.Domain) == domain.Domain {
			return ErrDuplicateDomain
		}
	}

	if strings.TrimSpace(domain.DNSStatus) == "" {
		domain.DNSStatus = "pending"
	}
	if strings.TrimSpace(domain.SSLStatus) == "" {
		domain.SSLStatus = "pending"
	}
	if strings.TrimSpace(domain.VerificationToken) == "" {
		domain.VerificationToken = generateVerificationToken()
	}

	now := time.Now().UTC()
	if domain.CreatedAt.IsZero() {
		domain.CreatedAt = now
	}
	domain.UpdatedAt = now

	copied := *domain
	m.domains[domain.ID] = &copied
	return nil
}

func (m *MemoryDomainRepository) GetCustomDomainByID(ctx context.Context, id string) (*CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cd, exists := m.domains[id]
	if !exists {
		return nil, ErrDomainNotFound
	}
	copied := *cd
	return &copied, nil
}

func (m *MemoryDomainRepository) GetCustomDomainByDomain(ctx context.Context, domainName string) (*CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalized := NormalizeDomain(domainName)
	for _, cd := range m.domains {
		if NormalizeDomain(cd.Domain) == normalized {
			copied := *cd
			return &copied, nil
		}
	}
	return nil, ErrDomainNotFound
}

func (m *MemoryDomainRepository) ListCustomDomainsByDeploymentID(ctx context.Context, deploymentID string) ([]CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []CustomDomain
	for _, cd := range m.domains {
		if cd.DeploymentID == deploymentID {
			list = append(list, *cd)
		}
	}
	return list, nil
}

func (m *MemoryDomainRepository) ListAllCustomDomains(ctx context.Context) ([]CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]CustomDomain, 0, len(m.domains))
	for _, cd := range m.domains {
		list = append(list, *cd)
	}
	return list, nil
}

func (m *MemoryDomainRepository) UpdateCustomDomainStatus(ctx context.Context, id string, dnsStatus, sslStatus string, lastCheckedAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cd, exists := m.domains[id]
	if !exists {
		return ErrDomainNotFound
	}

	cd.DNSStatus = dnsStatus
	cd.SSLStatus = sslStatus
	cd.LastCheckedAt = lastCheckedAt
	cd.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *MemoryDomainRepository) DeleteCustomDomain(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.domains[id]; !exists {
		return ErrDomainNotFound
	}
	delete(m.domains, id)
	return nil
}
