package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
)

const (
	StatusHealthy   = "healthy"
	StatusDegraded  = "degraded"
	StatusUnhealthy = "unhealthy"

	DefaultPollInterval = 30 * time.Second
	DefaultProbeTimeout = 5 * time.Second
	DefaultHistoryLimit = 50
	MaxLatencyThreshold = 2000 // 2 seconds in milliseconds
	FailureThreshold    = 3    // consecutive failures threshold for unhealthy
)

// ProbeResult stores the outcome of a single HTTP probe attempt.
type ProbeResult struct {
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	Error          string    `json:"error,omitempty"`
	MatchedKeyword bool      `json:"matched_keyword"`
	CheckedAt      time.Time `json:"checked_at"`
}

// ProbeRecord represents a historical probe sample in rolling history.
type ProbeRecord struct {
	Timestamp      time.Time `json:"timestamp"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
}

// UptimeReport represents computed rolling uptime statistics.
type UptimeReport struct {
	UptimePercent    float64       `json:"uptime_percent"`
	AverageLatencyMs float64       `json:"avg_latency_ms"`
	TotalProbes      int           `json:"total_probes"`
	SuccessfulProbes int           `json:"successful_probes"`
	FailedProbes     int           `json:"failed_probes"`
	RecentProbes     []ProbeRecord `json:"recent_probes"`
}

// RollingHistory maintains a fixed-capacity sliding window of probe results.
type RollingHistory struct {
	mu      sync.RWMutex
	limit   int
	records []ProbeRecord
}

// NewRollingHistory creates a new rolling history window with the given maximum capacity.
func NewRollingHistory(limit int) *RollingHistory {
	if limit <= 0 {
		limit = DefaultHistoryLimit
	}
	return &RollingHistory{
		limit:   limit,
		records: make([]ProbeRecord, 0, limit),
	}
}

// Add appends a new probe record to the rolling window, evicting the oldest if capacity is exceeded.
func (h *RollingHistory) Add(rec ProbeRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.records) >= h.limit {
		h.records = h.records[1:]
	}
	h.records = append(h.records, rec)
}

// Report calculates the rolling uptime percentage and average latency.
func (h *RollingHistory) Report() UptimeReport {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.records) == 0 {
		return UptimeReport{
			UptimePercent:    100.0,
			AverageLatencyMs: 0,
			TotalProbes:      0,
			SuccessfulProbes: 0,
			FailedProbes:     0,
			RecentProbes:     []ProbeRecord{},
		}
	}

	var totalLatency int64
	var successful, failed int
	recent := make([]ProbeRecord, len(h.records))
	copy(recent, h.records)

	for _, r := range h.records {
		totalLatency += r.ResponseTimeMs
		if r.Status == StatusUnhealthy {
			failed++
		} else {
			successful++
		}
	}

	uptime := float64(successful) / float64(len(h.records)) * 100.0
	avgLatency := float64(totalLatency) / float64(len(h.records))

	return UptimeReport{
		UptimePercent:    uptime,
		AverageLatencyMs: avgLatency,
		TotalProbes:      len(h.records),
		SuccessfulProbes: successful,
		FailedProbes:     failed,
		RecentProbes:     recent,
	}
}

// Prober defines the contract for executing an HTTP/HTTPS health probe.
type Prober interface {
	Probe(ctx context.Context, targetURL string, keyword string) (*ProbeResult, error)
}

// HTTPProber executes standard HTTP/HTTPS probes with a timeout and keyword verification.
type HTTPProber struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPProber creates an HTTPProber with the specified probe timeout.
func NewHTTPProber(timeout time.Duration) *HTTPProber {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	return &HTTPProber{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("stopped after 5 redirects")
				}
				return nil
			},
		},
		timeout: timeout,
	}
}

// Probe performs an HTTP GET request to targetURL and measures latency and keyword match.
func (p *HTTPProber) Probe(ctx context.Context, targetURL string, keyword string) (*ProbeResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return &ProbeResult{
			StatusCode:     0,
			ResponseTimeMs: 0,
			Error:          err.Error(),
			MatchedKeyword: false,
			CheckedAt:      time.Now().UTC(),
		}, nil
	}
	req.Header.Set("User-Agent", "Cuan-HealthCheck/1.0")

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return &ProbeResult{
			StatusCode:     0,
			ResponseTimeMs: latency,
			Error:          err.Error(),
			MatchedKeyword: false,
			CheckedAt:      time.Now().UTC(),
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	matched := true
	if keyword != "" {
		matched = strings.Contains(string(bodyBytes), keyword)
	}

	return &ProbeResult{
		StatusCode:     resp.StatusCode,
		ResponseTimeMs: latency,
		MatchedKeyword: matched,
		CheckedAt:      time.Now().UTC(),
	}, nil
}

// EvaluateStatus determines health status based on previous consecutive failures and probe result.
//
// Rules:
//   - Failure (Error != "", StatusCode < 200 || StatusCode >= 400, or keyword mismatch):
//     consecutiveFailures increases. If consecutiveFailures >= 3 -> "unhealthy", else "degraded".
//   - Success (2xx/3xx, keyword matched, no error):
//     consecutiveFailures reset to 0. If ResponseTimeMs > 2000 -> "degraded", else "healthy".
func EvaluateStatus(prevConsecutiveFailures int, result *ProbeResult) (string, int) {
	isFailure := result.Error != "" || result.StatusCode < 200 || result.StatusCode >= 400 || !result.MatchedKeyword

	if isFailure {
		newFailures := prevConsecutiveFailures + 1
		if newFailures >= FailureThreshold {
			return StatusUnhealthy, newFailures
		}
		return StatusDegraded, newFailures
	}

	// Success
	newFailures := 0
	if result.ResponseTimeMs > MaxLatencyThreshold {
		return StatusDegraded, newFailures
	}
	return StatusHealthy, newFailures
}

// HealthPollerOption is a functional configuration option for HealthPoller.
type HealthPollerOption func(*HealthPoller)

// WithPollInterval sets the background polling interval.
func WithPollInterval(d time.Duration) HealthPollerOption {
	return func(p *HealthPoller) {
		if d > 0 {
			p.interval = d
		}
	}
}

// WithProbeTimeout sets the timeout for individual probe requests.
func WithProbeTimeout(d time.Duration) HealthPollerOption {
	return func(p *HealthPoller) {
		if d > 0 {
			p.timeout = d
		}
	}
}

// WithProber overrides the default HTTP prober with a custom implementation.
func WithProber(prober Prober) HealthPollerOption {
	return func(p *HealthPoller) {
		if prober != nil {
			p.prober = prober
		}
	}
}

// WithHistoryLimit sets the maximum rolling history length per deployment.
func WithHistoryLimit(limit int) HealthPollerOption {
	return func(p *HealthPoller) {
		if limit > 0 {
			p.historyLimit = limit
		}
	}
}

// WithProbeCallback sets an optional callback executed after every probe.
func WithProbeCallback(cb func(hc *db.HealthCheck, res *ProbeResult)) HealthPollerOption {
	return func(p *HealthPoller) {
		p.onProbeCallback = cb
	}
}

// HealthPoller runs automated periodic health checks and maintains rolling uptime metrics.
type HealthPoller struct {
	repo            db.ObservabilityRepository
	prober          Prober
	interval        time.Duration
	timeout         time.Duration
	historyLimit    int
	history         map[string]*RollingHistory // deploymentID -> RollingHistory
	onProbeCallback func(hc *db.HealthCheck, res *ProbeResult)

	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

// NewHealthPoller creates a new HealthPoller instance.
func NewHealthPoller(repo db.ObservabilityRepository, opts ...HealthPollerOption) *HealthPoller {
	p := &HealthPoller{
		repo:         repo,
		interval:     DefaultPollInterval,
		timeout:      DefaultProbeTimeout,
		historyLimit: DefaultHistoryLimit,
		history:      make(map[string]*RollingHistory),
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.prober == nil {
		p.prober = NewHTTPProber(p.timeout)
	}

	return p
}

// Start begins the automated background polling loop.
func (p *HealthPoller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.done = make(chan struct{})
	p.running = true
	p.mu.Unlock()

	go p.pollLoop()
	return nil
}

// Stop gracefully stops the background polling loop and waits for completion.
func (p *HealthPoller) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.cancel()
	p.running = false
	done := p.done
	p.mu.Unlock()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// IsRunning reports whether the background poller is currently active.
func (p *HealthPoller) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// SetProbeCallback dynamically sets or updates the probe callback.
func (p *HealthPoller) SetProbeCallback(cb func(hc *db.HealthCheck, res *ProbeResult)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onProbeCallback = cb
}

func (p *HealthPoller) pollLoop() {
	defer close(p.done)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Initial poll on startup
	_, _ = p.PollAll(p.ctx)

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			_, _ = p.PollAll(p.ctx)
		}
	}
}

// PollAll executes a health check probe for all registered health checks.
func (p *HealthPoller) PollAll(ctx context.Context) ([]*db.HealthCheck, error) {
	checks, err := p.repo.ListHealthChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list health checks: %w", err)
	}

	var results []*db.HealthCheck
	for _, check := range checks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		hcCopy := check
		updated, _, err := p.probeSingle(ctx, &hcCopy)
		if err == nil && updated != nil {
			results = append(results, updated)
		}
	}

	return results, nil
}

// ProbeNow executes an immediate on-demand probe for the given deployment.
func (p *HealthPoller) ProbeNow(ctx context.Context, deploymentID string) (*db.HealthCheck, *UptimeReport, error) {
	hc, err := p.repo.GetHealthCheckByDeploymentID(ctx, deploymentID)
	if err != nil {
		return nil, nil, err
	}

	updated, report, err := p.probeSingle(ctx, hc)
	if err != nil {
		return nil, nil, err
	}

	return updated, report, nil
}

func (p *HealthPoller) probeSingle(ctx context.Context, hc *db.HealthCheck) (*db.HealthCheck, *UptimeReport, error) {
	res, err := p.prober.Probe(ctx, hc.URL, "")
	if err != nil && res == nil {
		res = &ProbeResult{
			StatusCode:     0,
			ResponseTimeMs: 0,
			Error:          err.Error(),
			MatchedKeyword: false,
			CheckedAt:      time.Now().UTC(),
		}
	}

	newStatus, newFailures := EvaluateStatus(hc.ConsecutiveFailures, res)

	now := res.CheckedAt
	if err := p.repo.UpdateHealthCheckResult(ctx, hc.ID, newStatus, res.StatusCode, res.ResponseTimeMs, newFailures, now); err != nil {
		return nil, nil, fmt.Errorf("update health check result: %w", err)
	}

	hc.Status = newStatus
	hc.LastStatusCode = res.StatusCode
	hc.ResponseTimeMs = res.ResponseTimeMs
	hc.ConsecutiveFailures = newFailures
	hc.LastCheckedAt = &now
	hc.UpdatedAt = now

	// Update rolling history
	p.recordHistory(hc.DeploymentID, ProbeRecord{
		Timestamp:      now,
		StatusCode:     res.StatusCode,
		ResponseTimeMs: res.ResponseTimeMs,
		Status:         newStatus,
		Error:          res.Error,
	})

	if p.onProbeCallback != nil {
		p.onProbeCallback(hc, res)
	}

	report := p.GetUptimeReport(hc.DeploymentID)
	return hc, &report, nil
}

func (p *HealthPoller) recordHistory(deploymentID string, record ProbeRecord) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h, ok := p.history[deploymentID]
	if !ok {
		h = NewRollingHistory(p.historyLimit)
		p.history[deploymentID] = h
	}
	h.Add(record)
}

// GetUptimeReport returns the rolling uptime report for a deployment.
func (p *HealthPoller) GetUptimeReport(deploymentID string) UptimeReport {
	p.mu.RLock()
	h, ok := p.history[deploymentID]
	p.mu.RUnlock()

	if !ok {
		return UptimeReport{
			UptimePercent:    100.0,
			AverageLatencyMs: 0,
			TotalProbes:      0,
			SuccessfulProbes: 0,
			FailedProbes:     0,
			RecentProbes:     []ProbeRecord{},
		}
	}

	return h.Report()
}

// EnsureHealthCheck creates a health check record if one does not already exist for the deployment.
func (p *HealthPoller) EnsureHealthCheck(ctx context.Context, deploymentID, targetURL string) (*db.HealthCheck, error) {
	existing, err := p.repo.GetHealthCheckByDeploymentID(ctx, deploymentID)
	if err == nil && existing != nil {
		return existing, nil
	}

	now := time.Now().UTC()
	hc := &db.HealthCheck{
		DeploymentID:        deploymentID,
		URL:                 targetURL,
		Status:              StatusHealthy,
		LastStatusCode:      200,
		ResponseTimeMs:      0,
		ConsecutiveFailures: 0,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := p.repo.CreateHealthCheck(ctx, hc); err != nil {
		// Could be duplicate race
		if errors.Is(err, db.ErrDuplicateHealthCheck) {
			return p.repo.GetHealthCheckByDeploymentID(ctx, deploymentID)
		}
		return nil, err
	}

	return hc, nil
}
